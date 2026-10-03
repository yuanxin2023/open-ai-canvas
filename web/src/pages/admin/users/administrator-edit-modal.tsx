import { App, Button, Form, Input, Modal, Tabs, type FormInstance } from "antd";
import { KeyRound, ShieldCheck, UserCog } from "lucide-react";
import { useEffect, useMemo, useState } from "react";

import { useCopyText } from "@/hooks/use-copy-text";
import { ADMIN_PERMISSION_GROUPS, ALL_ADMIN_PERMISSIONS, type AdminAccess, type AdminLevel, type AdminPermission } from "@/lib/admin-permissions";
import { AdminStatusBadge } from "@/pages/admin/components/admin-ui";
import { AdminModal } from "@/pages/admin/ui/overlays";
import { updateAdministrator, type AdminManagedUser, type AdminUser, type LocalUser } from "@/services/api/auth";
import type { CreditAccount } from "@/services/api/wallet";
import { generateAdminPassword } from "./admin-password";
import { AdminAccessFields, AdminCreditAdjustmentPanel, AdminPasswordField, type CreditAdjustmentState } from "./admin-user-editor-fields";

type AdministratorEditorTab = "profile" | "access" | "security" | "credits";
type ProfileFormValues = { email?: string; remark: string };
type AccessFormValues = { adminLevel: AdminLevel; permissions?: AdminPermission[] };
type PasswordFormValues = { password: string };

const permissionLabels = new Map<AdminPermission, string>(ADMIN_PERMISSION_GROUPS.flatMap((group) => group.items.map((item) => [item.permission, item.label] as const)));
const idleCreditState: CreditAdjustmentState = { dirty: false, busy: false, confirming: false };

export function AdministratorEditModal({
    user,
    actorId,
    onClose,
    onSaved,
    onCreditsAdjusted,
}: {
    user: AdminUser | null;
    actorId?: string;
    onClose: () => void;
    onSaved: (user: AdminManagedUser) => void;
    onCreditsAdjusted: (account: CreditAccount) => void;
}) {
    const { message, modal } = App.useApp();
    const copyText = useCopyText();
    const [activeTab, setActiveTab] = useState<AdministratorEditorTab>("profile");
    const [currentUser, setCurrentUser] = useState<AdminUser | null>(null);
    const [saving, setSaving] = useState<"profile" | "access" | "password" | "status" | null>(null);
    const [profileDirty, setProfileDirty] = useState(false);
    const [accessDirty, setAccessDirty] = useState(false);
    const [passwordDirty, setPasswordDirty] = useState(false);
    const [creditState, setCreditState] = useState<CreditAdjustmentState>(idleCreditState);
    const [pendingAccess, setPendingAccess] = useState<AdminAccess | null>(null);
    const [pendingStatus, setPendingStatus] = useState<LocalUser["status"] | null>(null);
    const [profileForm] = Form.useForm<ProfileFormValues>();
    const [accessForm] = Form.useForm<AccessFormValues>();
    const [passwordForm] = Form.useForm<PasswordFormValues>();

    const editingSelf = currentUser?.id === actorId;
    const selectedAdminLevel = Form.useWatch("adminLevel", accessForm);
    const currentAccess = useMemo<AdminAccess>(
        () => ({
            level: currentUser?.adminAccess?.level || "full",
            permissions: currentUser?.adminAccess?.permissions || [],
        }),
        [currentUser?.adminAccess],
    );
    const interactionLocked = Boolean(saving || pendingAccess || pendingStatus || creditState.busy || creditState.confirming);
    const syncAccessDirty = () => {
        const values = accessForm.getFieldsValue();
        setAccessDirty(
            !sameAccess(currentAccess, {
                level: values.adminLevel || "scoped",
                permissions: values.adminLevel === "full" ? [] : values.permissions || [],
            }),
        );
    };

    useEffect(() => {
        if (!user) return;
        setCurrentUser(user);
        setActiveTab("profile");
        setSaving(null);
        setPendingAccess(null);
        setPendingStatus(null);
        setCreditState(idleCreditState);
        resetProfileForm(profileForm, user);
        resetAccessForm(accessForm, user.adminAccess);
        passwordForm.resetFields();
        setProfileDirty(false);
        setAccessDirty(false);
        setPasswordDirty(false);
    }, [accessForm, passwordForm, profileForm, user?.id]);

    const mergeSavedUser = (next: AdminManagedUser) => {
        setCurrentUser((current) => (current ? { ...current, ...next } : current));
        onSaved(next);
    };

    const close = () => {
        if (interactionLocked) return;
        const dirtySections = [profileDirty ? "账号资料" : "", accessDirty ? "权限配置" : "", passwordDirty ? "安全与状态" : "", creditState.dirty ? "人工调账" : ""].filter(Boolean);
        if (!dirtySections.length) {
            onClose();
            return;
        }
        modal.confirm({
            title: "放弃管理员修改？",
            content: `以下分栏还有未保存内容：${dirtySections.join("、")}。关闭后这些内容将丢失。`,
            okText: "放弃修改",
            cancelText: "继续编辑",
            okButtonProps: { danger: true },
            onOk: onClose,
        });
    };

    const saveProfile = async () => {
        if (!currentUser) return;
        let values: ProfileFormValues;
        try {
            values = await profileForm.validateFields();
        } catch {
            return;
        }
        setSaving("profile");
        try {
            const result = await updateAdministrator(currentUser.id, {
                email: values.email?.trim() || "",
                remark: values.remark?.trim() || "",
            });
            mergeSavedUser(result.user);
            resetProfileForm(profileForm, result.user);
            setProfileDirty(false);
            message.success("管理员账号资料已保存");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "保存管理员资料失败");
        } finally {
            setSaving(null);
        }
    };

    const previewAccess = async () => {
        if (!currentUser || editingSelf) return;
        let values: AccessFormValues;
        try {
            values = await accessForm.validateFields();
        } catch {
            return;
        }
        const next: AdminAccess = {
            level: values.adminLevel || "scoped",
            permissions: values.adminLevel === "full" ? [] : values.permissions || [],
        };
        if (sameAccess(currentAccess, next)) {
            resetAccessForm(accessForm, currentAccess);
            setAccessDirty(false);
            message.info("管理员权限没有变化");
            return;
        }
        setPendingAccess(next);
    };

    const saveAccess = async () => {
        if (!currentUser || !pendingAccess) return;
        setSaving("access");
        try {
            const result = await updateAdministrator(currentUser.id, { adminAccess: pendingAccess });
            mergeSavedUser(result.user);
            resetAccessForm(accessForm, result.user.adminAccess);
            setAccessDirty(false);
            setPendingAccess(null);
            message.success("管理员权限已更新，目标账号的现有登录态已撤销");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "更新管理员权限失败");
        } finally {
            setSaving(null);
        }
    };

    const savePassword = async () => {
        if (!currentUser) return;
        let values: PasswordFormValues;
        try {
            values = await passwordForm.validateFields();
        } catch {
            return;
        }
        setSaving("password");
        try {
            const result = await updateAdministrator(currentUser.id, { password: values.password });
            mergeSavedUser(result.user);
            passwordForm.resetFields();
            setPasswordDirty(false);
            message.success("管理员密码已重置，目标账号的现有登录态已撤销");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "重置管理员密码失败");
        } finally {
            setSaving(null);
        }
    };

    const saveStatus = async () => {
        if (!currentUser || !pendingStatus || editingSelf) return;
        setSaving("status");
        try {
            const result = await updateAdministrator(currentUser.id, { status: pendingStatus });
            mergeSavedUser(result.user);
            setPendingStatus(null);
            message.success(pendingStatus === "disabled" ? "管理员账号已停用，现有登录态已撤销" : "管理员账号已重新启用");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "更新管理员状态失败");
        } finally {
            setSaving(null);
        }
    };

    const handleCreditsAdjusted = (account: CreditAccount) => {
        setCurrentUser((current) => (current ? { ...current, availableMicrocredits: account.availableMicrocredits, reservedMicrocredits: account.reservedMicrocredits } : current));
        onCreditsAdjusted(account);
    };

    const footer = (
        <div className="flex justify-end gap-2">
            <Button disabled={interactionLocked} onClick={close}>
                关闭
            </Button>
            {activeTab === "profile" ? (
                <Button type="primary" loading={saving === "profile"} disabled={interactionLocked || !profileDirty} onClick={() => void saveProfile()}>
                    保存账号资料
                </Button>
            ) : null}
            {activeTab === "access" && !editingSelf ? (
                <Button type="primary" loading={saving === "access"} disabled={interactionLocked || !accessDirty} onClick={() => void previewAccess()}>
                    核对权限变更
                </Button>
            ) : null}
        </div>
    );

    return (
        <>
            <AdminModal
                title="编辑管理员"
                open={Boolean(user)}
                centered
                width="min(840px, calc(100vw - 32px))"
                onCancel={close}
                mask={{ closable: !interactionLocked }}
                keyboard={!interactionLocked}
                closable={!interactionLocked}
                rootClassName="admin-administrator-editor-modal"
                footer={footer}
            >
                {currentUser ? (
                    <div className="admin-administrator-editor">
                        <header className="admin-administrator-editor-summary">
                            <span className="admin-administrator-editor-icon" aria-hidden="true">
                                <UserCog />
                            </span>
                            <div className="admin-administrator-editor-identity">
                                <strong>@{currentUser.username}</strong>
                                <span>管理员账号</span>
                            </div>
                            <div className="admin-administrator-editor-badges">
                                <AdminStatusBadge label={currentAccess.level === "full" ? "全权限管理员" : "模块管理员"} tone="info" />
                                <AdminStatusBadge label={currentUser.status === "active" ? "已启用" : "已停用"} tone={currentUser.status === "active" ? "success" : "neutral"} />
                            </div>
                        </header>

                        <Tabs
                            activeKey={activeTab}
                            animated={false}
                            onChange={(key) => {
                                if (!interactionLocked) setActiveTab(key as AdministratorEditorTab);
                            }}
                            items={[
                                {
                                    key: "profile",
                                    label: <TabLabel label="账号资料" dirty={profileDirty} />,
                                    children: (
                                        <div className="admin-administrator-editor-pane">
                                            <div className="admin-administrator-editor-intro">
                                                <strong>账号资料</strong>
                                                <p>维护联系邮箱与内部备注，不影响管理员权限和登录状态。</p>
                                            </div>
                                            <Form
                                                form={profileForm}
                                                layout="vertical"
                                                requiredMark={false}
                                                disabled={Boolean(saving)}
                                                onFieldsChange={() => {
                                                    const values = profileForm.getFieldsValue();
                                                    setProfileDirty(values.email !== (currentUser.email || "") || values.remark !== (currentUser.remark || ""));
                                                }}
                                            >
                                                <Form.Item label="用户名">
                                                    <Input value={`@${currentUser.username}`} disabled />
                                                </Form.Item>
                                                <Form.Item name="email" label="邮箱" rules={[{ type: "email", message: "请输入有效邮箱" }]}>
                                                    <Input placeholder="name@example.com" />
                                                </Form.Item>
                                                <Form.Item name="remark" label="备注" extra="仅管理员可见，可通过管理员列表搜索。">
                                                    <Input.TextArea rows={4} maxLength={500} showCount placeholder="例如：副站长管理员账号" />
                                                </Form.Item>
                                            </Form>
                                        </div>
                                    ),
                                },
                                {
                                    key: "access",
                                    label: <TabLabel label="权限配置" dirty={accessDirty} />,
                                    children: (
                                        <div className="admin-administrator-editor-pane">
                                            <div className="admin-administrator-editor-intro is-warning">
                                                <strong>权限与登录态</strong>
                                                <p>权限变化保存后立即生效，并会撤销目标账号的现有登录态。</p>
                                            </div>
                                            {editingSelf ? (
                                                <ReadOnlyAccess access={currentAccess} />
                                            ) : (
                                                <Form form={accessForm} layout="vertical" requiredMark={false} disabled={Boolean(saving)} onFieldsChange={syncAccessDirty}>
                                                    <AdminAccessFields level={selectedAdminLevel} onChange={syncAccessDirty} />
                                                </Form>
                                            )}
                                        </div>
                                    ),
                                },
                                {
                                    key: "security",
                                    label: <TabLabel label="安全与状态" dirty={passwordDirty} />,
                                    children: (
                                        <div className="admin-administrator-editor-pane admin-administrator-security-grid">
                                            <section className="admin-administrator-operation-card">
                                                <div className="admin-administrator-operation-heading">
                                                    <span aria-hidden="true">
                                                        <KeyRound />
                                                    </span>
                                                    <div>
                                                        <h3>重置密码</h3>
                                                        <p>保存新密码后会撤销该账号当前的全部登录状态。</p>
                                                    </div>
                                                </div>
                                                <Form form={passwordForm} layout="vertical" requiredMark={false} disabled={Boolean(saving)} onFieldsChange={() => setPasswordDirty(Boolean(passwordForm.getFieldValue("password")))}>
                                                    <Form.Item
                                                        name="password"
                                                        label="新密码"
                                                        rules={[
                                                            { required: true, message: "请输入或生成新密码" },
                                                            {
                                                                validator: (_, value?: string) => (value && Array.from(value).length >= 8 ? Promise.resolve() : Promise.reject(new Error("密码至少 8 位"))),
                                                            },
                                                        ]}
                                                    >
                                                        <AdminPasswordField
                                                            onCopy={() => {
                                                                const password = passwordForm.getFieldValue("password") || "";
                                                                if (!password) {
                                                                    message.warning("请先输入或生成新密码");
                                                                    return;
                                                                }
                                                                copyText(password, "密码已复制");
                                                            }}
                                                            onGenerate={() => {
                                                                passwordForm.setFields([{ name: "password", value: generateAdminPassword(16), touched: true, errors: [] }]);
                                                                setPasswordDirty(true);
                                                            }}
                                                        />
                                                    </Form.Item>
                                                    <div className="flex justify-end">
                                                        <Button type="primary" loading={saving === "password"} disabled={Boolean(saving) || !passwordDirty} onClick={() => void savePassword()}>
                                                            重置密码
                                                        </Button>
                                                    </div>
                                                </Form>
                                            </section>

                                            <section className="admin-administrator-operation-card">
                                                <div className="admin-administrator-operation-heading">
                                                    <span aria-hidden="true">
                                                        <ShieldCheck />
                                                    </span>
                                                    <div>
                                                        <h3>账号状态</h3>
                                                        <p>停用会撤销登录态，但保留身份、任务、积分和审计记录。</p>
                                                    </div>
                                                </div>
                                                <div className="admin-administrator-status-action">
                                                    <div>
                                                        <span>当前状态</span>
                                                        <AdminStatusBadge label={currentUser.status === "active" ? "已启用" : "已停用"} tone={currentUser.status === "active" ? "success" : "neutral"} />
                                                    </div>
                                                    <Button danger={currentUser.status === "active"} disabled={editingSelf || Boolean(saving)} onClick={() => setPendingStatus(currentUser.status === "active" ? "disabled" : "active")}>
                                                        {currentUser.status === "active" ? "停用账号" : "重新启用"}
                                                    </Button>
                                                </div>
                                                {editingSelf ? <p className="admin-administrator-self-note">不能停用当前登录的管理员账号。</p> : null}
                                            </section>
                                        </div>
                                    ),
                                },
                                {
                                    key: "credits",
                                    label: <TabLabel label="人工调账" dirty={creditState.dirty} />,
                                    children: (
                                        <div className="admin-administrator-editor-pane">
                                            <div className="admin-administrator-editor-intro is-warning">
                                                <strong>账务写入操作</strong>
                                                <p>调账独立提交，不会连带保存账号资料、权限或密码草稿。</p>
                                            </div>
                                            <AdminCreditAdjustmentPanel user={currentUser} disabled={Boolean(saving)} onCreditsAdjusted={handleCreditsAdjusted} onStateChange={setCreditState} />
                                        </div>
                                    ),
                                },
                            ]}
                        />
                    </div>
                ) : null}
            </AdminModal>

            <Modal
                title="核对管理员权限变更"
                open={Boolean(pendingAccess)}
                okText="确认保存权限"
                cancelText="返回修改"
                onCancel={() => {
                    if (saving !== "access") setPendingAccess(null);
                }}
                onOk={() => void saveAccess()}
                confirmLoading={saving === "access"}
                closable={saving !== "access"}
                mask={{ closable: saving !== "access" }}
                destroyOnHidden
                rootClassName="admin-modal-root"
            >
                {pendingAccess ? <AccessReview before={currentAccess} after={pendingAccess} /> : null}
            </Modal>

            <Modal
                title={pendingStatus === "disabled" ? "确认停用管理员账号" : "确认重新启用管理员账号"}
                open={Boolean(pendingStatus)}
                okText={pendingStatus === "disabled" ? "确认停用" : "确认启用"}
                cancelText="取消"
                onCancel={() => {
                    if (saving !== "status") setPendingStatus(null);
                }}
                onOk={() => void saveStatus()}
                confirmLoading={saving === "status"}
                closable={saving !== "status"}
                mask={{ closable: saving !== "status" }}
                destroyOnHidden
                rootClassName="admin-modal-root"
                okButtonProps={{ danger: pendingStatus === "disabled" }}
            >
                <p>{pendingStatus === "disabled" ? "停用后该管理员将立即失去后台访问能力，现有登录态会被撤销；身份、任务、积分和审计记录仍会保留。" : "重新启用后，该管理员可以再次登录；原有登录态不会恢复。"}</p>
            </Modal>
        </>
    );
}

function TabLabel({ label, dirty }: { label: string; dirty: boolean }) {
    return (
        <span className="admin-administrator-tab-label">
            {label}
            {dirty ? <span className="admin-administrator-tab-dirty" aria-label="有未保存修改" /> : null}
        </span>
    );
}

function ReadOnlyAccess({ access }: { access: AdminAccess }) {
    const labels = access.permissions.map((permission) => permissionLabels.get(permission) || permission);
    return (
        <section className="admin-administrator-readonly-access">
            <div>
                <span>管理员级别</span>
                <strong>{access.level === "full" ? "全权限管理员" : "模块管理员"}</strong>
            </div>
            {access.level === "scoped" ? (
                <div>
                    <span>模块权限</span>
                    <p>{labels.length ? labels.join("、") : "未配置"}</p>
                </div>
            ) : (
                <p>全权限管理员自动拥有当前及未来全部后台权限。</p>
            )}
            <p className="admin-administrator-self-note">为避免当前会话失去管理能力，不能在这里修改自己的管理员级别或权限。</p>
        </section>
    );
}

function AccessReview({ before, after }: { before: AdminAccess; after: AdminAccess }) {
    const beforePermissions = before.level === "full" ? ALL_ADMIN_PERMISSIONS : before.permissions;
    const afterPermissions = after.level === "full" ? ALL_ADMIN_PERMISSIONS : after.permissions;
    const beforeSet = new Set(beforePermissions);
    const afterSet = new Set(afterPermissions);
    const added = afterPermissions.filter((permission) => !beforeSet.has(permission));
    const removed = beforePermissions.filter((permission) => !afterSet.has(permission));
    return (
        <div className="admin-operation-confirmation">
            <p className="admin-operation-confirmation-copy">权限保存后立即生效，并会撤销目标管理员当前的全部登录状态。</p>
            <dl className="admin-operation-confirmation-grid">
                <div>
                    <dt>当前级别</dt>
                    <dd>{accessSummary(before)}</dd>
                </div>
                <div>
                    <dt>调整后级别</dt>
                    <dd>{accessSummary(after)}</dd>
                </div>
                <div className="is-wide">
                    <dt>新增权限</dt>
                    <dd>{added.length ? added.map((permission) => permissionLabels.get(permission) || permission).join("、") : "无"}</dd>
                </div>
                <div className="is-wide">
                    <dt>移除权限</dt>
                    <dd>{removed.length ? removed.map((permission) => permissionLabels.get(permission) || permission).join("、") : "无"}</dd>
                </div>
            </dl>
        </div>
    );
}

function accessSummary(access: AdminAccess) {
    return access.level === "full" ? "全权限管理员" : `模块管理员（${access.permissions.length} 项权限）`;
}

function sameAccess(left: AdminAccess, right: AdminAccess) {
    if (left.level !== right.level) return false;
    if (left.level === "full") return true;
    if (left.permissions.length !== right.permissions.length) return false;
    const rightSet = new Set(right.permissions);
    return left.permissions.every((permission) => rightSet.has(permission));
}

function resetProfileForm(form: FormInstance<ProfileFormValues>, user: Pick<AdminManagedUser, "email" | "remark">) {
    form.resetFields();
    form.setFieldsValue({ email: user.email || "", remark: user.remark || "" });
}

function resetAccessForm(form: FormInstance<AccessFormValues>, access?: AdminAccess) {
    form.resetFields();
    form.setFieldsValue({
        adminLevel: access?.level || "full",
        permissions: access?.permissions || [],
    });
}
