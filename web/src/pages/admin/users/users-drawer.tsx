import { App, Button, Drawer, Form, Input, Modal, Tabs } from "antd";
import { KeyRound, ShieldCheck, UserCog } from "lucide-react";
import { Select } from "@/pages/admin/ui/controls";
import { AdminModal } from "@/pages/admin/ui/overlays";
import { useEffect, useRef, useState } from "react";

import { useCopyText } from "@/hooks/use-copy-text";
import { normalizeUsername, usernameValidationMessage } from "@/lib/username";
import { createAdminUser, updateAdminUser, type AdminManagedUser, type AdminUser, type LocalUser } from "@/services/api/auth";
import type { CreditAccount } from "@/services/api/wallet";
import { generateAdminPassword } from "./admin-password";
import { hasAdminPermission } from "@/lib/admin-permissions";
import { useUserStore } from "@/stores/use-user-store";
import { AdminCreditAdjustmentPanel, AdminPasswordField, type CreditAdjustmentState } from "./admin-user-editor-fields";
import { AdminStatusBadge } from "../components/admin-ui";
import { buildUserProfilePatch, type UserProfileValues } from "./user-profile-patch";
import { UserResourceQuotaPanel, type ResourceQuotaEditorState } from "./user-resource-quota-panel";

type UserEditorTab = "profile" | "security" | "credits" | "quota";
const idleCreditState: CreditAdjustmentState = { dirty: false, busy: false, confirming: false };

export function AdminUserEditModal({ user, actorId, onClose, onSaved, onCreditsAdjusted }: { user: AdminUser | null; actorId?: string; onClose: () => void; onSaved: (user: AdminManagedUser) => void; onCreditsAdjusted: (account: CreditAccount) => void }) {
    const { message, modal } = App.useApp();
    const [activeTab, setActiveTab] = useState<UserEditorTab>("profile");
    const [currentUser, setCurrentUser] = useState<AdminUser | null>(null);
    const [saving, setSaving] = useState<"profile" | "password" | "status" | null>(null);
    const [profileDirty, setProfileDirty] = useState(false);
    const [passwordDirty, setPasswordDirty] = useState(false);
    const [pendingStatus, setPendingStatus] = useState<LocalUser["status"] | null>(null);
    const [closeConfirming, setCloseConfirming] = useState(false);
    const [creditState, setCreditState] = useState<CreditAdjustmentState>(idleCreditState);
    const [quotaState, setQuotaState] = useState<ResourceQuotaEditorState>({ dirty: false, busy: false });
    const [profileForm] = Form.useForm<UserProfileValues>();
    const [passwordForm] = Form.useForm<{ password: string }>();
    const operationRef = useRef(false);
    const copyText = useCopyText();
    const actorAccess = useUserStore((state) => state.user?.adminAccess);
    const canAdjustCredits = hasAdminPermission(actorAccess, "admin.finance.credits");
    const editingSelf = currentUser?.id === actorId;
    const interactionLocked = Boolean(saving || pendingStatus || closeConfirming || creditState.busy || creditState.confirming || quotaState.busy);

    useEffect(() => {
        if (!user) return;
        setCurrentUser(user);
        setActiveTab("profile");
        setSaving(null);
        operationRef.current = false;
        setPendingStatus(null);
        setCloseConfirming(false);
        setCreditState(idleCreditState);
        setQuotaState({ dirty: false, busy: false });
        profileForm.resetFields();
        profileForm.setFieldsValue({ email: user.email || "", remark: user.remark || "" });
        passwordForm.resetFields();
        setProfileDirty(false);
        setPasswordDirty(false);
    }, [passwordForm, profileForm, user?.id]);

    useEffect(() => {
        if (!canAdjustCredits) {
            setCreditState(idleCreditState);
            if (activeTab === "credits") setActiveTab("profile");
        }
    }, [activeTab, canAdjustCredits]);

    const mergeSavedUser = (next: AdminManagedUser) => {
        setCurrentUser((current) => current ? { ...current, ...next } : current);
        onSaved(next);
    };

    const close = () => {
        if (interactionLocked || operationRef.current) return;
        const dirtySections = [profileDirty ? "账号资料" : "", passwordDirty ? "安全与状态" : "", canAdjustCredits && creditState.dirty ? "人工调账" : "", quotaState.dirty ? "资源配额" : ""].filter(Boolean);
        if (!dirtySections.length) {
            onClose();
            return;
        }
        setCloseConfirming(true);
        modal.confirm({
            title: "放弃用户修改？",
            content: `以下分栏还有未保存内容：${dirtySections.join("、")}。关闭后这些内容将丢失。`,
            okText: "放弃修改",
            cancelText: "继续编辑",
            okButtonProps: { danger: true },
            onOk: onClose,
            afterClose: () => setCloseConfirming(false),
        });
    };

    const saveProfile = async () => {
        if (!currentUser || interactionLocked || operationRef.current) return;
        operationRef.current = true;
        setSaving("profile");
        try {
            const values = await profileForm.validateFields();
            const input = buildUserProfilePatch(currentUser, values);
            if (!Object.keys(input).length) {
                setProfileDirty(false);
                return;
            }
            const result = await updateAdminUser(currentUser.id, input);
            mergeSavedUser(result.user);
            profileForm.resetFields();
            profileForm.setFieldsValue({ email: result.user.email || "", remark: result.user.remark || "" });
            setProfileDirty(false);
            message.success("用户账号资料已保存");
        } catch (error) {
            if (!(error && typeof error === "object" && "errorFields" in error)) message.error(error instanceof Error ? error.message : "保存用户资料失败");
        } finally {
            operationRef.current = false;
            setSaving(null);
        }
    };

    const savePassword = async () => {
        if (!currentUser || interactionLocked || operationRef.current) return;
        operationRef.current = true;
        setSaving("password");
        try {
            const values = await passwordForm.validateFields();
            const result = await updateAdminUser(currentUser.id, { password: values.password });
            mergeSavedUser(result.user);
            passwordForm.resetFields();
            setPasswordDirty(false);
            message.success("用户密码已重置，目标账号的现有登录态已撤销");
        } catch (error) {
            if (!(error && typeof error === "object" && "errorFields" in error)) message.error(error instanceof Error ? error.message : "重置用户密码失败");
        } finally {
            operationRef.current = false;
            setSaving(null);
        }
    };

    const saveStatus = async () => {
        if (!currentUser || !pendingStatus || editingSelf || operationRef.current) return;
        operationRef.current = true;
        setSaving("status");
        try {
            const result = await updateAdminUser(currentUser.id, { status: pendingStatus });
            mergeSavedUser(result.user);
            setPendingStatus(null);
            message.success(pendingStatus === "disabled" ? "用户账号已停用，现有登录态已撤销" : "用户账号已重新启用");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "更新用户状态失败");
        } finally {
            operationRef.current = false;
            setSaving(null);
        }
    };

    const handleCreditsAdjusted = (account: CreditAccount) => {
        setCurrentUser((current) => current ? { ...current, availableMicrocredits: account.availableMicrocredits, reservedMicrocredits: account.reservedMicrocredits } : current);
        onCreditsAdjusted(account);
    };

    return (
        <>
        <AdminModal
            title="编辑用户"
            open={Boolean(user)}
            centered
            width="min(840px, calc(100vw - 32px))"
            onCancel={close}
            mask={{ closable: !interactionLocked }}
            keyboard={!interactionLocked}
            closable={!interactionLocked}
            rootClassName="admin-administrator-editor-modal"
            footer={(
                <div className="flex justify-end gap-2">
                    <Button disabled={interactionLocked} onClick={close}>关闭</Button>
                    {activeTab === "profile" ? <Button type="primary" loading={saving === "profile"} disabled={interactionLocked || !profileDirty} onClick={() => void saveProfile()}>保存账号资料</Button> : null}
                </div>
            )}
        >
            {currentUser ? (
                <div className="admin-administrator-editor">
                    <header className="admin-administrator-editor-summary">
                        <span className="admin-administrator-editor-icon" aria-hidden="true"><UserCog /></span>
                        <div className="admin-administrator-editor-identity"><strong>@{currentUser.username}</strong><span>普通用户 · {currentUser.status === "active" ? "已启用" : "已停用"}</span></div>
                        <div className="admin-administrator-editor-badges"><AdminStatusBadge label={currentUser.status === "active" ? "已启用" : "已停用"} tone={currentUser.status === "active" ? "success" : "neutral"} /></div>
                    </header>
                    <Tabs
                        activeKey={activeTab}
                        animated={false}
                        onChange={(key) => { if (!interactionLocked) setActiveTab(key as UserEditorTab); }}
                        items={[
                            {
                                key: "profile",
                                forceRender: true,
                                disabled: interactionLocked,
                                label: <UserEditorTabLabel label="账号资料" dirty={profileDirty} />,
                                children: (
                                    <div className="admin-administrator-editor-pane">
                                        <div className="admin-administrator-editor-intro"><strong>账号资料</strong><p>维护联系邮箱与内部备注，不影响密码和登录状态。</p></div>
                                        <Form form={profileForm} layout="vertical" requiredMark={false} disabled={interactionLocked} onFieldsChange={() => setProfileDirty(Boolean(Object.keys(buildUserProfilePatch(currentUser, profileForm.getFieldsValue())).length))}>
                                            <Form.Item label="用户名"><Input value={`@${currentUser.username}`} disabled /></Form.Item>
                                            <Form.Item name="email" label="邮箱" rules={[{ type: "email", message: "请输入有效邮箱" }]}><Input placeholder="name@example.com" /></Form.Item>
                                            <Form.Item name="remark" label="备注" extra="仅管理员可见，可通过用户列表搜索。"><Input.TextArea rows={4} maxLength={500} showCount placeholder="例如：企业客户、工单编号或跟进说明" /></Form.Item>
                                        </Form>
                                    </div>
                                ),
                            },
                            {
                                key: "security",
                                forceRender: true,
                                disabled: interactionLocked,
                                label: <UserEditorTabLabel label="安全与状态" dirty={passwordDirty} />,
                                children: (
                                    <div className="admin-administrator-editor-pane admin-administrator-security-grid">
                                        <section className="admin-administrator-operation-card">
                                            <div className="admin-administrator-operation-heading"><span aria-hidden="true"><KeyRound /></span><div><h3>重置密码</h3><p>保存新密码后会清除该用户当前的全部登录状态。</p></div></div>
                                            <Form form={passwordForm} layout="vertical" requiredMark={false} disabled={interactionLocked} onFieldsChange={() => setPasswordDirty(Boolean(passwordForm.getFieldValue("password")))}>
                                                <Form.Item name="password" label="新密码" rules={[{ required: true, message: "请输入或生成新密码" }, { validator: (_, value?: string) => value && Array.from(value).length >= 8 ? Promise.resolve() : Promise.reject(new Error("密码至少 8 位")) }]}>
                                                    <AdminPasswordField
                                                        onCopy={() => {
                                                            const password = passwordForm.getFieldValue("password") || "";
                                                            if (!password) { message.warning("请先输入或生成新密码"); return; }
                                                            copyText(password, "密码已复制");
                                                        }}
                                                        onGenerate={() => {
                                                            passwordForm.setFields([{ name: "password", value: generateAdminPassword(16), touched: true, errors: [] }]);
                                                            setPasswordDirty(true);
                                                        }}
                                                    />
                                                </Form.Item>
                                                <div className="flex justify-end"><Button type="primary" loading={saving === "password"} disabled={interactionLocked || !passwordDirty} onClick={() => void savePassword()}>重置密码</Button></div>
                                            </Form>
                                        </section>
                                        <section className="admin-administrator-operation-card">
                                            <div className="admin-administrator-operation-heading"><span aria-hidden="true"><ShieldCheck /></span><div><h3>账号状态</h3><p>停用会清除登录态，但保留身份、任务、积分和审计记录。</p></div></div>
                                            <div className="admin-administrator-status-action">
                                                <div><span>当前状态</span><AdminStatusBadge label={currentUser.status === "active" ? "已启用" : "已停用"} tone={currentUser.status === "active" ? "success" : "neutral"} /></div>
                                                <Button danger={currentUser.status === "active"} disabled={editingSelf || interactionLocked} onClick={() => setPendingStatus(currentUser.status === "active" ? "disabled" : "active")}>{currentUser.status === "active" ? "停用账号" : "重新启用"}</Button>
                                            </div>
                                            {editingSelf ? <p className="admin-administrator-self-note">不能停用当前登录账号。</p> : null}
                                        </section>
                                    </div>
                                ),
                            },
                            {
                                key: "quota",
                                forceRender: true,
                                disabled: interactionLocked,
                                label: <UserEditorTabLabel label="资源配额" dirty={quotaState.dirty} />,
                                children: <div className="admin-administrator-editor-pane"><UserResourceQuotaPanel userId={currentUser.id} disabled={interactionLocked} onStateChange={setQuotaState} /></div>,
                            },
                            ...(canAdjustCredits ? [{
                                key: "credits",
                                forceRender: true,
                                disabled: interactionLocked,
                                label: <UserEditorTabLabel label="人工调账" dirty={creditState.dirty} />,
                                children: (
                                    <div className="admin-administrator-editor-pane">
                                        <div className="admin-administrator-editor-intro is-warning"><strong>账务写入操作</strong><p>调账独立提交，不会连带保存账号资料或密码草稿。</p></div>
                                        <AdminCreditAdjustmentPanel user={currentUser} disabled={interactionLocked} onCreditsAdjusted={handleCreditsAdjusted} onStateChange={setCreditState} />
                                    </div>
                                ),
                            }] : []),
                        ]}
                    />
                </div>
            ) : null}
        </AdminModal>
        <Modal
            title={pendingStatus === "disabled" ? "确认停用用户账号" : "确认重新启用用户账号"}
            open={Boolean(pendingStatus)}
            okText={pendingStatus === "disabled" ? "确认停用" : "确认启用"}
            cancelText="取消"
            onCancel={() => { if (!saving) setPendingStatus(null); }}
            onOk={() => void saveStatus()}
            confirmLoading={saving === "status"}
            closable={!saving}
            keyboard={!saving}
            mask={{ closable: !saving }}
            destroyOnHidden
            rootClassName="admin-modal-root"
            okButtonProps={{ danger: pendingStatus === "disabled" }}
        >
            <p>目标用户：@{currentUser?.username}</p>
            <p>{pendingStatus === "disabled" ? "停用后该用户将无法登录，现有登录态会被撤销；身份、任务、积分和审计记录仍会保留。" : "重新启用后，该用户可以再次登录；原有登录态不会恢复。"}</p>
        </Modal>
        </>
    );
}

function UserEditorTabLabel({ label, dirty }: { label: string; dirty: boolean }) {
    return <span className="admin-administrator-tab-label">{label}{dirty ? <span className="admin-administrator-tab-dirty" aria-label="有未保存修改" /> : null}</span>;
}

type CreateUserFormValues = {
    username: string;
    email?: string;
    remark?: string;
    password: string;
    status: LocalUser["status"];
};

export function AdminUserCreateDrawer({ open, onClose, onCreated }: { open: boolean; onClose: () => void; onCreated: (user: AdminUser) => void }) {
    const { message, modal } = App.useApp();
    const [saving, setSaving] = useState(false);
    const [form] = Form.useForm<CreateUserFormValues>();

    useEffect(() => {
        if (!open) return;
        form.resetFields();
        form.setFieldsValue({ status: "active" });
    }, [form, open]);

    const close = () => {
        if (saving) return;
        if (!form.isFieldsTouched()) {
            onClose();
            return;
        }
        modal.confirm({
            title: "\u653e\u5f03\u6dfb\u52a0\u7528\u6237\uff1f",
            content: "\u5c1a\u672a\u4fdd\u5b58\u7684\u7528\u6237\u4fe1\u606f\u5c06\u4e22\u5931\u3002",
            okText: "\u653e\u5f03\u5e76\u5173\u95ed",
            cancelText: "\u7ee7\u7eed\u7f16\u8f91",
            okButtonProps: { danger: true },
            onOk: onClose,
        });
    };

    const save = async () => {
        const values = await form.validateFields();
        setSaving(true);
        try {
            const result = await createAdminUser({
                username: normalizeUsername(values.username),
                email: values.email?.trim() || "",
                remark: values.remark?.trim() || "",
                password: values.password,
                status: values.status,
            });
            onCreated(result.user);
            form.resetFields();
            onClose();
            message.success("\u7528\u6237\u5df2\u521b\u5efa");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "\u521b\u5efa\u7528\u6237\u5931\u8d25");
        } finally {
            setSaving(false);
        }
    };

    return (
        <Drawer
            title={"\u6dfb\u52a0\u7528\u6237"}
            open={open}
            size="min(520px, 100vw)"
            onClose={close}
            mask={{ closable: !saving }}
            destroyOnHidden
            extra={
                <Button type="primary" loading={saving} onClick={() => void save()}>
                    {"\u4fdd\u5b58"}
                </Button>
            }
        >
            <Form form={form} layout="vertical" requiredMark={false}>
                <Form.Item name="username" label={"\u7528\u6237\u540d"} rules={[{ validator: (_, value?: string) => { const error = usernameValidationMessage(value || ""); return error ? Promise.reject(new Error(error)) : Promise.resolve(); } }]}>
                    <Input placeholder={"3-9 \u4f4d\u4e2d\u6587\u3001\u82f1\u6587\u5b57\u6bcd\u6216\u6570\u5b57"} />
                </Form.Item>
                <Form.Item name="email" label={"\u90ae\u7bb1"} rules={[{ type: "email", message: "\u8bf7\u8f93\u5165\u6709\u6548\u90ae\u7bb1" }]}>
                    <Input placeholder="name@example.com" />
                </Form.Item>
                <Form.Item name="remark" label="备注" extra="仅管理员可见，可通过用户列表搜索。">
                    <Input.TextArea rows={3} maxLength={500} showCount placeholder="例如：企业客户、工单编号或跟进说明" />
                </Form.Item>
                <Form.Item name="password" label={"\u521d\u59cb\u5bc6\u7801"} rules={[{ required: true, message: "\u8bf7\u8bbe\u7f6e\u521d\u59cb\u5bc6\u7801" }]}>
                    <Input.Password placeholder={"\u81f3\u5c11 8 \u4f4d"} />
                </Form.Item>
                <Form.Item name="status" label={"\u8d26\u53f7\u72b6\u6001"}>
                    <Select
                        options={[
                            { label: "\u5df2\u542f\u7528", value: "active" },
                            { label: "\u5df2\u505c\u7528", value: "disabled" },
                        ]}
                    />
                </Form.Item>
            </Form>
        </Drawer>
    );
}
