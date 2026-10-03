import { App, Button, Checkbox, Form, Input, InputNumber, Modal, Tooltip } from "antd";
import { Coins, Copy, RefreshCw } from "lucide-react";
import { useEffect, useState, type ChangeEvent } from "react";

import { formatCredits } from "@/constant/credits";
import { ADMIN_PERMISSION_GROUPS, type AdminLevel, type AdminPermission } from "@/lib/admin-permissions";
import { Select } from "@/pages/admin/ui/controls";
import type { AdminUser } from "@/services/api/auth";
import { adjustAdminUserCredits, type CreditAccount } from "@/services/api/wallet";

export type CreditAdjustmentState = {
    dirty: boolean;
    busy: boolean;
    confirming: boolean;
};

type CreditAdjustmentFormValues = { amount: number; note: string };

export function AdminAccessFields({ level, onChange }: { level?: AdminLevel; onChange?: () => void }) {
    const form = Form.useFormInstance();
    const selectedPermissions = (Form.useWatch("permissions", form) || []) as AdminPermission[];
    const updateGroup = (permissions: AdminPermission[], select: boolean) => {
        const next = new Set(selectedPermissions);
        for (const permission of permissions) {
            if (select) next.add(permission);
            else next.delete(permission);
        }
        form.setFieldValue("permissions", Array.from(next));
        onChange?.();
    };

    return (
        <section className="rounded-lg border border-border/60 p-4">
            <Form.Item name="adminLevel" label="管理员级别" rules={[{ required: true, message: "请选择管理员级别" }]}>
                <Select
                    options={[
                        { label: "模块管理员", value: "scoped" },
                        { label: "全权限管理员", value: "full" },
                    ]}
                />
            </Form.Item>
            {level === "full" ? (
                <p className="text-xs text-foreground/55">全权限管理员自动拥有当前及未来全部后台权限，也可以任命和配置其他管理员。</p>
            ) : (
                <Form.Item name="permissions" label="模块权限" rules={[{ validator: (_, value?: AdminPermission[]) => (value?.length ? Promise.resolve() : Promise.reject(new Error("至少选择一个模块权限"))) }]}>
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
                                        {group.items.map((item) => (
                                            <Checkbox key={item.permission} value={item.permission}>
                                                {item.label}
                                            </Checkbox>
                                        ))}
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

export function AdminPasswordField({ value = "", onChange, onCopy, onGenerate }: { value?: string; onChange?: (event: ChangeEvent<HTMLInputElement>) => void; onCopy: () => void; onGenerate: () => void }) {
    return (
        <div className="flex items-center gap-2">
            <Input.Password
                className="min-w-0 flex-1"
                value={value}
                onChange={onChange}
                placeholder="输入新密码，或随机生成 16 位密码"
                autoComplete="new-password"
                suffix={
                    <Tooltip title="复制密码">
                        <Button type="text" size="small" aria-label="复制密码" icon={<Copy className="size-3.5" />} onClick={onCopy} />
                    </Tooltip>
                }
            />
            <Tooltip title="随机生成 16 位密码">
                <Button aria-label="随机生成 16 位密码" icon={<RefreshCw className="size-4" />} onClick={onGenerate} />
            </Tooltip>
        </div>
    );
}

export function AdminCreditAdjustmentPanel({
    user,
    disabled = false,
    withDivider = false,
    onCreditsAdjusted,
    onStateChange,
}: {
    user: AdminUser | null;
    disabled?: boolean;
    withDivider?: boolean;
    onCreditsAdjusted: (account: CreditAccount) => void;
    onStateChange?: (state: CreditAdjustmentState) => void;
}) {
    const { message } = App.useApp();
    const [form] = Form.useForm<CreditAdjustmentFormValues>();
    const [adjusting, setAdjusting] = useState(false);
    const [dirty, setDirty] = useState(false);
    const [pendingAdjustment, setPendingAdjustment] = useState<CreditAdjustmentFormValues | null>(null);
    const [availableMicrocredits, setAvailableMicrocredits] = useState(0);
    const [reservedMicrocredits, setReservedMicrocredits] = useState(0);

    useEffect(() => {
        form.resetFields();
        setDirty(false);
        setPendingAdjustment(null);
        setAvailableMicrocredits(user?.availableMicrocredits || 0);
        setReservedMicrocredits(user?.reservedMicrocredits || 0);
    }, [form, user?.id]);

    useEffect(() => {
        onStateChange?.({ dirty, busy: adjusting, confirming: Boolean(pendingAdjustment) });
    }, [adjusting, dirty, onStateChange, pendingAdjustment]);

    const preview = async () => {
        const values = await form.validateFields();
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

    const apply = async () => {
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
            form.resetFields();
            setDirty(false);
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
            <section className={withDivider ? "mt-2 border-t border-border/60 pt-5" : undefined} aria-labelledby="admin-user-credit-adjustment-heading">
                <div className="mb-4 flex flex-wrap items-start justify-between gap-2">
                    <div>
                        <h3 id="admin-user-credit-adjustment-heading" className="text-sm font-semibold text-foreground">
                            人工调账
                        </h3>
                        <p className="mt-1 text-xs text-foreground/55">提交后会立即写入积分流水和管理员审计记录。</p>
                    </div>
                    <div className="text-right text-xs text-foreground/55">
                        <div>
                            可用积分 <strong className="tabular-nums text-foreground/80">{formatCredits(availableMicrocredits)}</strong>
                        </div>
                        <div className="mt-1">
                            冻结积分 <strong className="tabular-nums text-foreground/80">{formatCredits(reservedMicrocredits)}</strong>
                        </div>
                    </div>
                </div>
                <Form
                    form={form}
                    layout="vertical"
                    requiredMark={false}
                    disabled={disabled || adjusting}
                    onFieldsChange={() => {
                        const values = form.getFieldsValue();
                        setDirty(values.amount !== undefined || Boolean(values.note?.trim()));
                    }}
                    onFinish={() => void preview()}
                >
                    <Form.Item
                        name="amount"
                        label="积分变化"
                        extra="正数增加，负数扣减；扣减只能使用可用积分。"
                        rules={[
                            { required: true, message: "请填写积分变化" },
                            {
                                validator: (_, value) => (typeof value === "number" && Number.isFinite(value) && value !== 0 ? Promise.resolve() : Promise.reject(new Error("积分变化不能为 0"))),
                            },
                        ]}
                    >
                        <InputNumber className="w-full" precision={2} prefix={<Coins className="size-3.5 text-foreground/45" />} placeholder="例如 10 或 -2.50" />
                    </Form.Item>
                    <Form.Item name="note" label="调整原因" rules={[{ required: true, whitespace: true, message: "请填写工单号或处理依据" }]}>
                        <Input.TextArea rows={4} maxLength={500} showCount placeholder="例如：工单 YC-20260828，补偿失败任务费用" />
                    </Form.Item>
                    <div className="flex justify-end">
                        <Button icon={<Coins className="size-4" />} loading={adjusting} disabled={disabled} onClick={() => form.submit()}>
                            核对并调账
                        </Button>
                    </div>
                </Form>
            </section>

            <Modal
                title={pendingAdjustment?.amount && pendingAdjustment.amount < 0 ? "确认扣减用户积分" : "确认增加用户积分"}
                open={Boolean(pendingAdjustment)}
                okText={pendingAdjustment?.amount && pendingAdjustment.amount < 0 ? "确认扣减" : "确认增加"}
                cancelText="返回修改"
                onCancel={() => {
                    if (!adjusting) setPendingAdjustment(null);
                }}
                onOk={() => void apply()}
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
