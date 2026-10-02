import { App, Button, Checkbox, Drawer, Form, Input, InputNumber, Modal, Tooltip } from "antd";
import { Coins, Copy, RefreshCw } from "lucide-react";
import { Select } from "@/pages/admin/ui/controls";
import { AdminModal } from "@/pages/admin/ui/overlays";
import { useEffect, useState, type ChangeEvent } from "react";

import { useCopyText } from "@/hooks/use-copy-text";
import { normalizeUsername, usernameValidationMessage } from "@/lib/username";
import { formatCredits } from "@/constant/credits";
import { createAdminUser, updateAdminUser, type AdminManagedUser, type AdminUser, type LocalUser } from "@/services/api/auth";
import { adjustAdminUserCredits, type CreditAccount } from "@/services/api/wallet";
import { generateAdminPassword } from "./admin-password";
import { ADMIN_PERMISSION_GROUPS, hasAdminPermission, type AdminLevel, type AdminPermission } from "@/lib/admin-permissions";
import { useUserStore } from "@/stores/use-user-store";

type UserFormValues = Pick<LocalUser, "email" | "role" | "status"> & { remark: string; password?: string; adminLevel?: AdminLevel; permissions?: AdminPermission[] };
type CreditAdjustmentFormValues = { amount: number; note: string };

export function AdminUserEditModal({
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
    const [saving, setSaving] = useState(false);
    const [adjusting, setAdjusting] = useState(false);
    const [pendingAdjustment, setPendingAdjustment] = useState<CreditAdjustmentFormValues | null>(null);
    const [availableMicrocredits, setAvailableMicrocredits] = useState(0);
    const [reservedMicrocredits, setReservedMicrocredits] = useState(0);
    const [form] = Form.useForm<UserFormValues>();
    const [adjustmentForm] = Form.useForm<CreditAdjustmentFormValues>();
    const copyText = useCopyText();
    const actorAccess = useUserStore((state) => state.user?.adminAccess);
    const actorIsFull = actorAccess?.level === "full";
    const canAdjustCredits = hasAdminPermission(actorAccess, "admin.finance.credits");
    const editingSelf = user?.id === actorId;
    const selectedRole = Form.useWatch("role", form);
    const selectedAdminLevel = Form.useWatch("adminLevel", form);

    useEffect(() => {
        if (!user) return;
        form.resetFields();
        adjustmentForm.resetFields();
        setPendingAdjustment(null);
        setAvailableMicrocredits(user.availableMicrocredits);
        setReservedMicrocredits(user.reservedMicrocredits);
        form.setFieldsValue({
            email: user.email || "",
            remark: user.remark || "",
            password: "",
            role: user.role,
            status: user.status,
            adminLevel: user.adminAccess?.level,
            permissions: user.adminAccess?.permissions || [],
        });
    }, [adjustmentForm, form, user]);

    const close = () => {
        if (saving || adjusting || pendingAdjustment) return;
        if (!form.isFieldsTouched() && !adjustmentForm.isFieldsTouched()) {
            onClose();
            return;
        }
        modal.confirm({
            title: "放弃用户修改？",
            content: "尚未保存的账号、备注、密码、角色、状态或积分调整内容将丢失。",
            okText: "放弃修改",
            cancelText: "继续编辑",
            okButtonProps: { danger: true },
            onOk: onClose,
        });
    };

    const save = async () => {
        if (!user) return;
        const adjustmentDraft = adjustmentForm.getFieldsValue();
        if (adjustmentDraft.amount !== undefined || adjustmentDraft.note?.trim()) {
            message.warning("请先完成或清空积分调账内容，再保存用户信息");
            return;
        }
        const values = await form.validateFields();
        const password = values.password || "";
        setSaving(true);
        try {
            const result = await updateAdminUser(user.id, {
                email: values.email?.trim() || "",
                remark: values.remark?.trim() || "",
                role: values.role,
                status: values.status,
                ...(values.role === "admin" && !editingSelf ? { adminAccess: { level: values.adminLevel || "scoped", permissions: values.adminLevel === "full" ? [] : (values.permissions || []) } } : {}),
                ...(password ? { password } : {}),
            });
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

    const previewCreditAdjustment = async () => {
        const values = await adjustmentForm.validateFields();
        const amount = Number(values.amount);
        if (!Number.isFinite(amount) || amount === 0) {
            message.error("积分变化不能为 0");
            return;
        }
        let amountMicrocredits: number;
        try {
            amountMicrocredits = toMicrocredits(amount);
        } catch (error) {
            message.error(error instanceof Error ? error.message : "积分变化超出可处理范围");
            return;
        }
        if (amount < 0 && availableMicrocredits + amountMicrocredits < 0) {
            message.error("扣减后可用积分不能低于 0");
            return;
        }
        setPendingAdjustment({ amount, note: values.note.trim() });
    };

    const applyCreditAdjustment = async () => {
        if (!user || !pendingAdjustment) return;
        setAdjusting(true);
        try {
            const result = await adjustAdminUserCredits(user.id, {
                amountMicrocredits: toMicrocredits(pendingAdjustment.amount),
                note: pendingAdjustment.note,
            });
            setAvailableMicrocredits(result.account.availableMicrocredits);
            setReservedMicrocredits(result.account.reservedMicrocredits);
            onCreditsAdjusted(result.account);
            adjustmentForm.resetFields();
            setPendingAdjustment(null);
            message.success(`用户积分已调整，当前可用积分 ${formatCredits(result.account.availableMicrocredits)}`);
        } catch (error) {
            message.error(error instanceof Error ? error.message : "调整积分失败");
        } finally {
            setAdjusting(false);
        }
    };

    return (
        <>
        <AdminModal
            title={user ? `编辑用户 · ${user.username}` : "编辑用户"}
            open={Boolean(user)}
            centered
            width="min(620px, calc(100vw - 32px))"
            onCancel={close}
            mask={{ closable: !saving && !adjusting && !pendingAdjustment }}
            keyboard={!saving && !adjusting && !pendingAdjustment}
            closable={!saving && !adjusting && !pendingAdjustment}
            styles={{ body: { maxHeight: "min(72vh, 720px)", overflowX: "hidden", overflowY: "auto" } }}
            footer={(
                <div className="flex justify-end gap-2">
                    <Button disabled={saving || adjusting} onClick={close}>取消</Button>
                    <Button type="primary" loading={saving} disabled={adjusting} onClick={() => void save()}>保存</Button>
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
                <Form.Item name="role" label="角色" extra={editingSelf ? "不能在此修改当前管理员自己的角色。" : "角色变更会立即影响后台访问权限。"}>
                    <Select disabled={editingSelf || !actorIsFull} options={[{ label: "管理员", value: "admin" }, { label: "普通用户", value: "user" }]} />
                </Form.Item>
                <Form.Item name="status" label="账号状态" extra={editingSelf ? "不能停用当前登录账号。" : "停用后会清除登录态，但保留身份、任务和积分流水。"}>
                    <Select disabled={editingSelf || (user?.role === "admin" && !actorIsFull)} options={[{ label: "已启用", value: "active" }, { label: "已停用", value: "disabled" }]} />
                </Form.Item>
                {actorIsFull && selectedRole === "admin" && !editingSelf ? <AdminAccessFields level={selectedAdminLevel} /> : null}
            </Form>

            {canAdjustCredits ? <section className="mt-2 border-t border-border/60 pt-5" aria-labelledby="admin-user-credit-adjustment-heading">
                <div className="mb-4 flex flex-wrap items-start justify-between gap-2">
                    <div>
                        <h3 id="admin-user-credit-adjustment-heading" className="text-sm font-semibold text-foreground">人工调账</h3>
                        <p className="mt-1 text-xs text-foreground/55">提交后会立即写入积分流水和管理员审计记录。</p>
                    </div>
                    <div className="text-right text-xs text-foreground/55">
                        <div>可用积分 <strong className="tabular-nums text-foreground/80">{formatCredits(availableMicrocredits)}</strong></div>
                        <div className="mt-1">冻结积分 <strong className="tabular-nums text-foreground/80">{formatCredits(reservedMicrocredits)}</strong></div>
                    </div>
                </div>
                <Form form={adjustmentForm} layout="vertical" requiredMark={false} onFinish={() => void previewCreditAdjustment()}>
                    <Form.Item
                        name="amount"
                        label="积分变化"
                        extra="正数增加，负数扣减；扣减只能使用可用积分。"
                        rules={[
                            { required: true, message: "请填写积分变化" },
                            {
                                validator: (_, value) => typeof value === "number" && Number.isFinite(value) && value !== 0
                                    ? Promise.resolve()
                                    : Promise.reject(new Error("积分变化不能为 0")),
                            },
                        ]}
                    >
                        <InputNumber className="w-full" precision={2} prefix={<Coins className="size-3.5 text-foreground/45" />} placeholder="例如 10 或 -2.50" />
                    </Form.Item>
                    <Form.Item name="note" label="调整原因" rules={[{ required: true, whitespace: true, message: "请填写工单号或处理依据" }]}>
                        <Input.TextArea rows={4} maxLength={500} showCount placeholder="例如：工单 YC-20260828，补偿失败任务费用" />
                    </Form.Item>
                    <div className="flex justify-end">
                        <Button icon={<Coins className="size-4" />} loading={adjusting} disabled={saving} onClick={() => adjustmentForm.submit()}>
                            核对并调账
                        </Button>
                    </div>
                </Form>
            </section> : null}
        </AdminModal>

        <Modal
            title={pendingAdjustment?.amount && pendingAdjustment.amount < 0 ? "确认扣减用户积分" : "确认增加用户积分"}
            open={Boolean(pendingAdjustment)}
            okText={pendingAdjustment?.amount && pendingAdjustment.amount < 0 ? "确认扣减" : "确认增加"}
            cancelText="返回修改"
            onCancel={() => {
                if (!adjusting) setPendingAdjustment(null);
            }}
            onOk={() => void applyCreditAdjustment()}
            confirmLoading={adjusting}
            mask={{ closable: !adjusting }}
            closable={!adjusting}
            destroyOnHidden
            rootClassName="admin-modal-root"
            okButtonProps={{ danger: Boolean(pendingAdjustment && pendingAdjustment.amount < 0) }}
        >
            {pendingAdjustment && user ? (
                <div className="admin-operation-confirmation">
                    <p className="admin-operation-confirmation-copy">请再次核对用户、积分变化和处理依据。确认后将立即写入账务流水。</p>
                    <dl className="admin-operation-confirmation-grid">
                        <div>
                            <dt>目标用户</dt>
                            <dd>@{user.username}</dd>
                        </div>
                        <div>
                            <dt>积分变化</dt>
                            <dd className={pendingAdjustment.amount < 0 ? "is-negative" : "is-positive"}>
                                {pendingAdjustment.amount > 0 ? "+" : ""}
                                {formatCredits(toMicrocredits(pendingAdjustment.amount))}
                            </dd>
                        </div>
                        <div>
                            <dt>当前可用</dt>
                            <dd>{formatCredits(availableMicrocredits)}</dd>
                        </div>
                        <div>
                            <dt>预计可用</dt>
                            <dd>{formatCredits(availableMicrocredits + toMicrocredits(pendingAdjustment.amount))}</dd>
                        </div>
                        <div className="is-wide">
                            <dt>处理依据</dt>
                            <dd>{pendingAdjustment.note}</dd>
                        </div>
                    </dl>
                </div>
            ) : null}
        </Modal>
        </>
    );
}

function toMicrocredits(value: number) {
    const result = Math.round(Number(value) * 1_000_000);
    if (!Number.isSafeInteger(result)) throw new Error("积分变化超出可处理范围");
    return result;
}

function AdminAccessFields({ level }: { level?: AdminLevel }) {
	const form = Form.useFormInstance();
	const selectedPermissions = (Form.useWatch("permissions", form) || []) as AdminPermission[];
	const updateGroup = (permissions: AdminPermission[], select: boolean) => {
		const next = new Set(selectedPermissions);
		for (const permission of permissions) {
			if (select) next.add(permission);
			else next.delete(permission);
		}
		form.setFieldValue("permissions", Array.from(next));
	};

    return (
        <section className="rounded-lg border border-border/60 p-4">
            <Form.Item name="adminLevel" label="管理员级别" rules={[{ required: true, message: "请选择管理员级别" }]}>
                <Select options={[
                    { label: "模块管理员", value: "scoped" },
                    { label: "全权限管理员", value: "full" },
                ]} />
            </Form.Item>
            {level === "full" ? <p className="text-xs text-foreground/55">全权限管理员自动拥有当前及未来全部后台权限，也可以任命和配置其他管理员。</p> : (
                <Form.Item
                    name="permissions"
                    label="模块权限"
                    rules={[{ validator: (_, value?: AdminPermission[]) => value?.length ? Promise.resolve() : Promise.reject(new Error("至少选择一个模块权限")) }]}
                >
                    <Checkbox.Group className="w-full">
                        <div className="grid gap-4">
                            {ADMIN_PERMISSION_GROUPS.map((group) => (
                                <div key={group.label}>
                                    <div className="mb-2 flex items-center justify-between gap-2">
										<span className="text-xs font-semibold text-foreground/65">{group.label}</span>
										<Button
											type="link"
											size="small"
											className="h-auto p-0 text-xs"
											onClick={() => {
												const permissions = group.items.map((item) => item.permission);
												const allSelected = permissions.every((permission) => selectedPermissions.includes(permission));
												updateGroup(permissions, !allSelected);
											}}
										>
											{group.items.every((item) => selectedPermissions.includes(item.permission)) ? "清空本组" : "全选本组"}
										</Button>
									</div>
                                    <div className="grid gap-2 sm:grid-cols-2">
                                        {group.items.map((item) => <Checkbox key={item.permission} value={item.permission}>{item.label}</Checkbox>)}
                                    </div>
                                </div>
                            ))}
                        </div>
                    </Checkbox.Group>
                </Form.Item>
            )}
        </section>
    );
}

function AdminPasswordField({
    value = "",
    onChange,
    onCopy,
    onGenerate,
}: {
    value?: string;
    onChange?: (event: ChangeEvent<HTMLInputElement>) => void;
    onCopy: () => void;
    onGenerate: () => void;
}) {
    return (
        <div className="flex items-center gap-2">
            <Input.Password
                className="min-w-0 flex-1"
                value={value}
                onChange={onChange}
                placeholder="输入新密码，或随机生成 16 位密码"
                autoComplete="new-password"
                suffix={(
                    <Tooltip title="复制密码">
                        <Button type="text" size="small" aria-label="复制密码" icon={<Copy className="size-3.5" />} onClick={onCopy} />
                    </Tooltip>
                )}
            />
            <Tooltip title="随机生成 16 位密码">
                <Button aria-label="随机生成 16 位密码" icon={<RefreshCw className="size-4" />} onClick={onGenerate} />
            </Tooltip>
        </div>
    );
}

type CreateUserFormValues = {
    username: string;
    email?: string;
    remark?: string;
    password: string;
    role: LocalUser["role"];
    status: LocalUser["status"];
    adminLevel?: AdminLevel;
    permissions?: AdminPermission[];
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
    const actorAccess = useUserStore((state) => state.user?.adminAccess);
    const actorIsFull = actorAccess?.level === "full";
    const selectedRole = Form.useWatch("role", form);
    const selectedAdminLevel = Form.useWatch("adminLevel", form);

    useEffect(() => {
        if (!open) return;
        form.resetFields();
        form.setFieldsValue({ role: "user", status: "active" });
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
                role: values.role,
                status: values.status,
                ...(values.role === "admin" ? { adminAccess: { level: values.adminLevel || "scoped", permissions: values.adminLevel === "full" ? [] : (values.permissions || []) } } : {}),
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
                <Form.Item name="role" label={"\u89d2\u8272"}>
                    <Select options={actorIsFull ? [{ label: "\u7ba1\u7406\u5458", value: "admin" }, { label: "\u666e\u901a\u7528\u6237", value: "user" }] : [{ label: "\u666e\u901a\u7528\u6237", value: "user" }]} />
                </Form.Item>
                {actorIsFull && selectedRole === "admin" ? <AdminAccessFields level={selectedAdminLevel} /> : null}
                <Form.Item name="status" label={"\u8d26\u53f7\u72b6\u6001"}>
                    <Select options={[{ label: "\u5df2\u542f\u7528", value: "active" }, { label: "\u5df2\u505c\u7528", value: "disabled" }]} />
                </Form.Item>
            </Form>
        </Drawer>
    );
}
