import { App, Button, Drawer, Form, Input } from "antd";
import { Select } from "@/pages/admin/ui/controls";
import { AdminModal } from "@/pages/admin/ui/overlays";
import { useEffect, useState } from "react";

import { useCopyText } from "@/hooks/use-copy-text";
import { normalizeUsername, usernameValidationMessage } from "@/lib/username";
import { createAdminUser, updateAdminUser, type AdminManagedUser, type AdminUser, type LocalUser } from "@/services/api/auth";
import type { CreditAccount } from "@/services/api/wallet";
import { generateAdminPassword } from "./admin-password";
import { hasAdminPermission } from "@/lib/admin-permissions";
import { useUserStore } from "@/stores/use-user-store";
import { AdminCreditAdjustmentPanel, AdminPasswordField, type CreditAdjustmentState } from "./admin-user-editor-fields";

type UserFormValues = Pick<LocalUser, "email" | "role" | "status"> & { remark: string; password?: string };

export function AdminUserEditModal({ user, actorId, onClose, onSaved, onCreditsAdjusted }: { user: AdminUser | null; actorId?: string; onClose: () => void; onSaved: (user: AdminManagedUser) => void; onCreditsAdjusted: (account: CreditAccount) => void }) {
    const { message, modal } = App.useApp();
    const [saving, setSaving] = useState(false);
    const [creditState, setCreditState] = useState<CreditAdjustmentState>({ dirty: false, busy: false, confirming: false });
    const [form] = Form.useForm<UserFormValues>();
    const copyText = useCopyText();
    const actorAccess = useUserStore((state) => state.user?.adminAccess);
    const canAdjustCredits = hasAdminPermission(actorAccess, "admin.finance.credits");
    const editingSelf = user?.id === actorId;

    useEffect(() => {
        if (!user) return;
        form.resetFields();
        setCreditState({ dirty: false, busy: false, confirming: false });
        form.setFieldsValue({
            email: user.email || "",
            remark: user.remark || "",
            password: "",
            role: user.role,
            status: user.status,
        });
    }, [form, user]);

    const close = () => {
        if (saving || creditState.busy || creditState.confirming) return;
        if (!form.isFieldsTouched() && !creditState.dirty) {
            onClose();
            return;
        }
        modal.confirm({
            title: "放弃用户修改？",
            content: "尚未保存的账号、备注、密码、状态或积分调整内容将丢失。",
            okText: "放弃修改",
            cancelText: "继续编辑",
            okButtonProps: { danger: true },
            onOk: onClose,
        });
    };

    const save = async () => {
        if (!user) return;
        if (creditState.dirty) {
            message.warning("请先完成或清空积分调账内容，再保存用户信息");
            return;
        }
        const values = await form.validateFields();
        const password = values.password || "";
        setSaving(true);
        try {
            const input = {
                email: values.email?.trim() || "",
                remark: values.remark?.trim() || "",
                status: values.status,
                ...(password ? { password } : {}),
            };
            const result = await updateAdminUser(user.id, input);
            onSaved(result.user);
            form.resetFields();
            onClose();
            message.success(password ? "用户信息已保存，密码已重置" : "用户信息已保存");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "保存用户失败");
        } finally {
            setSaving(false);
        }
    };

    return (
        <AdminModal
            title={user ? `编辑用户 · ${user.username}` : "编辑用户"}
            open={Boolean(user)}
            centered
            width="min(620px, calc(100vw - 32px))"
            onCancel={close}
            mask={{ closable: !saving && !creditState.busy && !creditState.confirming }}
            keyboard={!saving && !creditState.busy && !creditState.confirming}
            closable={!saving && !creditState.busy && !creditState.confirming}
            styles={{ body: { maxHeight: "min(72vh, 720px)", overflowX: "hidden", overflowY: "auto" } }}
            footer={(
                <div className="flex justify-end gap-2">
                    <Button disabled={saving || creditState.busy} onClick={close}>取消</Button>
                    <Button type="primary" loading={saving} disabled={creditState.busy} onClick={() => void save()}>保存</Button>
                </div>
            )}
        >
            <Form form={form} layout="vertical" requiredMark={false}>
                <Form.Item label="用户名">
                    <Input value={user ? `@${user.username}` : ""} disabled />
                </Form.Item>
                <Form.Item name="email" label="邮箱" rules={[{ type: "email", message: "请输入有效邮箱" }]}>
                    <Input placeholder="name@example.com" />
                </Form.Item>
                <Form.Item name="remark" label="备注" extra="仅管理员可见，可通过用户列表搜索。">
                    <Input.TextArea rows={3} maxLength={500} showCount placeholder="例如：企业客户、工单编号或跟进说明" />
                </Form.Item>
                <Form.Item
                    name="password"
                    label="修改密码"
                    extra="留空则保持原密码；修改后会清除该用户当前的全部登录状态。"
                    rules={[{
                        validator: (_, value?: string) => !value || Array.from(value).length >= 8
                            ? Promise.resolve()
                            : Promise.reject(new Error("密码至少 8 位")),
                    }]}
                >
                    <AdminPasswordField
                        onCopy={() => {
                            const password = form.getFieldValue("password") || "";
                            if (!password) {
                                message.warning("请先输入或生成新密码");
                                return;
                            }
                            copyText(password, "密码已复制");
                        }}
                        onGenerate={() => form.setFields([{ name: "password", value: generateAdminPassword(16), touched: true, errors: [] }])}
                    />
                </Form.Item>
                <Form.Item name="status" label="账号状态" extra={editingSelf ? "不能停用当前登录账号。" : "停用后会清除登录态，但保留身份、任务和积分流水。"}>
                    <Select disabled={editingSelf} options={[{ label: "已启用", value: "active" }, { label: "已停用", value: "disabled" }]} />
                </Form.Item>
            </Form>

            {canAdjustCredits ? <AdminCreditAdjustmentPanel user={user} disabled={saving} withDivider onCreditsAdjusted={onCreditsAdjusted} onStateChange={setCreditState} /> : null}
        </AdminModal>
    );
}

type CreateUserFormValues = {
    username: string;
    email?: string;
    remark?: string;
    password: string;
    status: LocalUser["status"];
};

export function AdminUserCreateDrawer({
    open,
    onClose,
    onCreated,
}: {
    open: boolean;
    onClose: () => void;
    onCreated: (user: AdminUser) => void;
}) {
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
            extra={<Button type="primary" loading={saving} onClick={() => void save()}>{"\u4fdd\u5b58"}</Button>}
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
                    <Select options={[{ label: "\u5df2\u542f\u7528", value: "active" }, { label: "\u5df2\u505c\u7528", value: "disabled" }]} />
                </Form.Item>
            </Form>
        </Drawer>
    );
}
