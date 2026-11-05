import { Button, Form, InputNumber, Segmented, Select, Switch, Table } from "antd";
import { Check, Gift, Info, Plus, ShieldCheck, TrendingUp, Trash2 } from "lucide-react";
import { useLayoutEffect, useRef, type ReactNode } from "react";
import type { ReferralPolicy } from "@/services/api/referrals";
import { referralCredits, referralPolicySummary } from "@/lib/referral-policy";
import { formatCredits } from "@/constant/credits";
import "./policy-editor.css";

function LimitInput({ value = 0, onChange, scale = 1, max = 1_000_000, label, unit, disabled, id }: { value?: number; onChange?: (value: number) => void; scale?: number; max?: number; label: string; unit?: string; disabled?: boolean; id?: string }) {
    return (
        <div className={`referral-limit ${value > 0 ? "is-limited" : ""}`}>
            <Select
                id={id}
                aria-label={`${label}是否限制`}
                disabled={disabled}
                value={value > 0 ? "limited" : "unlimited"}
                onChange={(next) => onChange?.(next === "unlimited" ? 0 : scale)}
                options={[
                    { value: "unlimited", label: "不限" },
                    { value: "limited", label: "限制" },
                ]}
            />
            {value > 0 ? (
                <InputNumber
                    aria-label={`${label}数值`}
                    disabled={disabled}
                    min={scale === 1 ? 1 : 0.01}
                    max={max}
                    precision={scale === 1 ? 0 : 2}
                    value={value / scale}
                    suffix={unit}
                    onChange={(next) => {
                        if (next !== null) onChange?.(Math.round(next * scale));
                    }}
                />
            ) : null}
        </div>
    );
}

function PolicyCard({ title, icon, badge, actions, description, children }: { title: string; icon: ReactNode; badge?: string; actions?: ReactNode; description?: string; children: ReactNode }) {
    return (
        <section className="referral-policy-card" aria-label={title}>
            <header className="referral-policy-card-head">
                <div className="referral-policy-card-title">
                    <span className="referral-policy-icon" aria-hidden>
                        {icon}
                    </span>
                    <h2>{title}</h2>
                    {badge ? <span className="referral-policy-badge">{badge}</span> : null}
                </div>
                {actions}
            </header>
            {description ? <p className="referral-policy-card-description">{description}</p> : null}
            <div className="referral-policy-card-body">{children}</div>
        </section>
    );
}

export function PolicyEditor({ initial, saving, onSave, global = false }: { initial: ReferralPolicy; saving: boolean; onSave: (policy: ReferralPolicy) => void; global?: boolean }) {
    const [form] = Form.useForm<ReferralPolicy>();
    const root = useRef<HTMLDivElement>(null);
    const values = Form.useWatch([], form) as ReferralPolicy | undefined;
    const policy = { ...initial, ...values, stages: values?.stages ?? initial.stages };
    const stages = policy.stages;
    const timeMode = policy.mode === "time";
    const previewReady = stages.length > 0 && stages.every((stage) => stage && Number.isInteger(stage.until) && Number.isInteger(stage.rateBps));
    const stageName = (index: number) => {
        const start = index === 0 ? (timeMode ? 0 : 1) : (stages[index - 1]?.until ?? 0) + (timeMode ? 0 : 1);
        const end = stages[index]?.until;
        return timeMode ? `${start}—${end || "不限"}天` : `第${start}—${end || "不限"}单`;
    };
    const last = stages.at(-1);
    const firstReward = stages[0] ? referralCredits(1_200_000_000, stages[0].rateBps) : 0;
    const inviterExample = Math.min(firstReward, policy.perInviteeCapMicrocredits || Number.MAX_SAFE_INTEGER);
    const newcomerExample = policy.newcomerEnabled ? Math.min(referralCredits(1_200_000_000, policy.newcomerRateBps), policy.newcomerCapMicrocredits || Number.MAX_SAFE_INTEGER) : 0;

    useLayoutEffect(() => {
        const node = root.current;
        if (!node) return;
        const measure = () => {
            const rect = node.getBoundingClientRect();
            node.style.setProperty("--referral-footer-left", `${rect.left}px`);
            node.style.setProperty("--referral-footer-width", `${rect.width}px`);
        };
        measure();
        const observer = new ResizeObserver(measure);
        observer.observe(node);
        window.addEventListener("resize", measure);
        return () => {
            observer.disconnect();
            window.removeEventListener("resize", measure);
        };
    }, []);

    return (
        <div ref={root} className="referral-policy">
            <Form form={form} initialValues={initial} layout="vertical" disabled={saving} scrollToFirstError={{ block: "center", focus: true }} onFinish={(value) => onSave({ ...value, versionId: initial.versionId })}>
                <section className="referral-policy-status" aria-label={global ? "全局推广状态" : "专属规则状态"}>
                    <div>
                        <div className="referral-policy-status-title">
                            <h2>{global ? "全局推广状态" : "专属规则状态"}</h2>
                            <span className={`referral-policy-state ${policy.enabled ? "is-enabled" : ""}`}>
                                <i />
                                {policy.enabled ? "已启用" : "已关闭"}
                            </span>
                        </div>
                        <p>{global ? "关闭并发布后，将暂停所有新邀请绑定和奖励计提；已有待审核记录仍可处理。" : "专属方案整体替代全局方案；关闭并发布后不再接受该方案的新邀请绑定。"}</p>
                    </div>
                    <div className="referral-policy-status-control">
                        <span>修改后需发布生效</span>
                        <Form.Item name="enabled" valuePropName="checked" noStyle>
                            <Switch aria-label={global ? "全局推广总开关" : "专属规则开关"} />
                        </Form.Item>
                    </div>
                </section>
                <div className="referral-policy-layout">
                    <div className="referral-policy-fields">
                        <PolicyCard
                            title="邀请人返利"
                            icon={<TrendingUp />}
                            badge="审核后发放"
                            description={timeMode ? "按首次成功充值后的连续天数分阶段，各阶段范围自动衔接。" : "按套餐基础积分计算返利，各阶段范围自动衔接。"}
                            actions={
                                <Form.Item name="mode" noStyle>
                                    <Segmented
                                        aria-label="阶梯模式"
                                        options={[
                                            { value: "orders", label: "按充值单数" },
                                            { value: "time", label: "按时间" },
                                        ]}
                                    />
                                </Form.Item>
                            }
                        >
                            <div className="referral-stage-header" aria-hidden>
                                <span>奖励阶段</span>
                                <span>{timeMode ? "首次充值后时间范围" : "适用充值范围"}</span>
                                <span>返利比例</span>
                                <span />
                            </div>
                            <Form.List
                                name="stages"
                                rules={[
                                    {
                                        validator: async (_, list: ReferralPolicy["stages"]) => {
                                            if (!list?.length || list.length > 20) throw new Error("请设置 1—20 个阶段");
                                            let previous = 0;
                                            for (let i = 0; i < list.length; i++) {
                                                const end = list[i].until;
                                                if (!Number.isInteger(end) || end < 0 || end > (timeMode ? 36500 : 1_000_000) || (end === 0 && i !== list.length - 1) || (end !== 0 && end <= previous))
                                                    throw new Error("结束值必须递增，只有最后阶段可不限；时间最多 36500 天");
                                                previous = end;
                                            }
                                        },
                                    },
                                ]}
                            >
                                {(fields, { add, remove }, { errors }) => (
                                    <>
                                        {fields.map((field, index) => (
                                            <div key={field.key} className="referral-stage-row">
                                                <div className="referral-stage-name">
                                                    <span>{index + 1}</span>阶段 {index + 1}
                                                </div>
                                                <div className="referral-stage-range">
                                                    <span className="referral-stage-start">{timeMode ? `第 ${index === 0 ? 0 : (stages[index - 1]?.until ?? 0)} 天` : `第 ${index === 0 ? 1 : (stages[index - 1]?.until ?? 0) + 1} 单`}</span>
                                                    <span className="referral-stage-arrow" aria-hidden>
                                                        →
                                                    </span>
                                                    <Form.Item name={[field.name, "until"]} rules={[{ required: true, message: "请设置阶段结束值" }]} className="referral-stage-end">
                                                        <LimitInput label={`阶段${index + 1}截止${timeMode ? "天数" : "单数"}`} unit={timeMode ? "天" : "单"} max={timeMode ? 36500 : 1_000_000} />
                                                    </Form.Item>
                                                </div>
                                                <div className="referral-stage-rate">
                                                    <span className="referral-mobile-label">返利比例</span>
                                                    <Form.Item
                                                        name={[field.name, "rateBps"]}
                                                        rules={[{ required: true, message: "请输入返利比例" }]}
                                                        getValueProps={(value) => ({ value: typeof value === "number" ? value / 100 : undefined })}
                                                        normalize={(value: number | null) => (value === null ? null : Math.round(value * 100))}
                                                    >
                                                        <InputNumber aria-label={`阶段${index + 1}返利比例`} min={0} max={100} precision={2} suffix="%" />
                                                    </Form.Item>
                                                </div>
                                                <Button
                                                    className="referral-stage-delete"
                                                    type="text"
                                                    icon={<Trash2 size={15} />}
                                                    aria-label={`删除阶段${index + 1}`}
                                                    title="删除阶段"
                                                    disabled={fields.length === 1 || saving}
                                                    onClick={() => remove(field.name)}
                                                />
                                            </div>
                                        ))}
                                        <Form.ErrorList errors={errors} />
                                        <div className="referral-stage-actions">
                                            <Button
                                                icon={<Plus size={14} />}
                                                disabled={fields.length >= 20 || saving}
                                                onClick={() => {
                                                    const insertAt = stages.at(-1)?.until === 0 ? Math.max(0, fields.length - 1) : fields.length;
                                                    add({ until: (stages[insertAt - 1]?.until || 0) + 30, rateBps: 0 }, insertAt);
                                                }}
                                            >
                                                添加阶段
                                            </Button>
                                            <span>
                                                <Info size={13} />
                                                最后阶段可设不限；比例为 0% 时停止奖励
                                            </span>
                                        </div>
                                        {last?.until && previewReady ? <p className="referral-stage-endnote">{timeMode ? `首次充值满 ${last.until} 天后停止返利。` : `按当前配置，第 ${last.until + 1} 单起不再返利。`}</p> : null}
                                    </>
                                )}
                            </Form.List>
                        </PolicyCard>
                        <PolicyCard title="奖励限制" icon={<ShieldCheck />} actions={<span className="referral-policy-helper">独立限制，达到任一条件即停止邀请人奖励</span>}>
                            <div className="referral-policy-limits">
                                <div>
                                    <Form.Item name="maxDays" label="总有效天数">
                                        <LimitInput label="总有效天数" unit="天" max={36500} />
                                    </Form.Item>
                                    <p className="referral-field-note">从首次成功充值起计算</p>
                                </div>
                                <div>
                                    <Form.Item name="maxOrders" label="最多参与充值单数">
                                        <LimitInput label="最多参与充值单数" unit="单" />
                                    </Form.Item>
                                    <p className="referral-field-note">按每位新客户的成功充值累计</p>
                                </div>
                                <div>
                                    <Form.Item name="perInviteeCapMicrocredits" label="每位新客户累计返利上限">
                                        <LimitInput label="累计返利上限" unit="积分" scale={1_000_000} max={1_000_000_000} />
                                    </Form.Item>
                                    <p className="referral-field-note">待审核与已发放奖励共同占用额度</p>
                                </div>
                            </div>
                        </PolicyCard>
                        <PolicyCard
                            title="新人首充赠送"
                            icon={<Gift />}
                            badge="自动到账"
                            description="给被邀请的新客户一份首充福利，与邀请人返利分别计算。"
                            actions={
                                <div className="referral-policy-status-control">
                                    <span className={policy.newcomerEnabled ? "referral-policy-enabled-label" : ""}>{policy.newcomerEnabled ? "已开启" : "已关闭"}</span>
                                    <Form.Item name="newcomerEnabled" valuePropName="checked" noStyle>
                                        <Switch aria-label="新人首充赠送开关" />
                                    </Form.Item>
                                </div>
                            }
                        >
                            <div className="referral-policy-newcomer">
                                <Form.Item
                                    name="newcomerRateBps"
                                    label="首充赠送比例"
                                    getValueProps={(value) => ({ value: typeof value === "number" ? value / 100 : undefined })}
                                    normalize={(value: number | null) => (value === null ? null : Math.round(value * 100))}
                                    rules={[{ required: true, message: "请输入首充赠送比例" }]}
                                >
                                    <InputNumber aria-label="首充赠送比例" disabled={saving || !policy.newcomerEnabled} min={0} max={100} precision={2} suffix="%" />
                                </Form.Item>
                                <Form.Item name="newcomerCapMicrocredits" label="首充赠送积分上限">
                                    <LimitInput label="首充赠送积分上限" unit="积分" scale={1_000_000} max={1_000_000_000} disabled={saving || !policy.newcomerEnabled} />
                                </Form.Item>
                            </div>
                            <p className="referral-newcomer-note">
                                <Check size={13} />
                                仅首次成功充值赠送；赠送积分不计入邀请人返利基数。
                            </p>
                        </PolicyCard>
                    </div>
                    <aside className="referral-policy-aside">
                        <section className="referral-policy-card" aria-label="奖励预览">
                            <header className="referral-policy-card-head">
                                <h2>奖励预览</h2>
                                <span className="referral-policy-helper">随配置实时更新</span>
                            </header>
                            <div className="referral-policy-card-body">
                                <p className="referral-policy-summary" aria-live="polite">
                                    {previewReady ? referralPolicySummary(policy) : "请完善阶段配置后查看试算。"}
                                </p>
                                <div className="referral-preview-caption">不同套餐下，邀请人可获得的积分</div>
                                <Table
                                    className="referral-preview-table"
                                    size="small"
                                    pagination={false}
                                    rowKey="index"
                                    scroll={{ x: 300, y: stages.length > 5 ? 220 : undefined }}
                                    dataSource={previewReady ? stages.map((stage, index) => ({ ...stage, index })) : []}
                                    columns={[
                                        {
                                            title: "适用阶段",
                                            key: "stage",
                                            width: 116,
                                            render: (_, row: { index: number; rateBps: number }) => (
                                                <span>
                                                    {stageName(row.index)}
                                                    <small>{row.rateBps / 100}%</small>
                                                </span>
                                            ),
                                        },
                                        ...[100, 1200].map((base) => ({
                                            title: `${base.toLocaleString()} 积分`,
                                            key: String(base),
                                            align: "right" as const,
                                            render: (_: unknown, stage: { rateBps: number }) => formatCredits(referralCredits(base * 1_000_000, stage.rateBps)),
                                        })),
                                    ]}
                                />
                                {last?.until ? <p className="referral-preview-after">{timeMode ? `满 ${last.until} 天后` : `第 ${last.until + 1} 单起`}不再返利</p> : null}
                                <div className="referral-preview-example">
                                    <div className="referral-preview-example-head">
                                        <span>新客户首充示例</span>
                                        <strong>99 元 / 1,200 积分</strong>
                                    </div>
                                    <div className="referral-preview-rewards">
                                        <div>
                                            <span>邀请人获得</span>
                                            <p>
                                                {formatCredits(inviterExample)}
                                                <small>积分</small>
                                            </p>
                                            <span>审核后发放</span>
                                        </div>
                                        <div>
                                            <span>新客户额外获得</span>
                                            <p>
                                                {formatCredits(newcomerExample)}
                                                <small>积分</small>
                                            </p>
                                            <span>{policy.newcomerEnabled ? "随首充自动到账" : "首充赠送已关闭"}</span>
                                        </div>
                                    </div>
                                </div>
                                <p className="referral-preview-note">阶段试算为原始奖励，实际受有效期、单数及剩余额度限制。{!policy.enabled ? "当前开关已关闭，发布后不计提新奖励。" : ""}</p>
                            </div>
                        </section>
                        <section className="referral-policy-card" aria-label="发布与生效">
                            <header className="referral-policy-card-head">
                                <h2>发布与生效</h2>
                                <span className="referral-policy-badge">版本管理</span>
                            </header>
                            <div className="referral-policy-card-body">
                                <ul className="referral-policy-version-notes">
                                    <li>发布后生成新版本，仅适用于之后绑定的邀请关系。</li>
                                    <li>已绑定关系及已产生奖励，继续使用原规则版本。</li>
                                    {global ? <li>全局关闭并发布后，已有关系也暂停计提；恢复不延长有效期。</li> : <li>全局总开关或邀请人暂停状态，仍会影响已有关系的新奖励。</li>}
                                </ul>
                                <div className="referral-policy-version">
                                    <span>当前规则版本</span>
                                    <code title={initial.versionId || "尚未发布"}>{initial.versionId ? `${initial.versionId.slice(0, 8)}…${initial.versionId.slice(-5)}` : "尚未发布"}</code>
                                </div>
                            </div>
                        </section>
                    </aside>
                </div>
                <footer className="referral-policy-footer">
                    <div>
                        <strong>发布前请核对奖励预览</strong>
                        <span>配置修改需发布后生效</span>
                    </div>
                    <Button type="primary" htmlType="submit" loading={saving}>
                        发布规则版本
                    </Button>
                </footer>
            </Form>
        </div>
    );
}
