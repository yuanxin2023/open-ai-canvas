import { Button, Skeleton } from "antd";
import { ArrowRight, RefreshCw } from "lucide-react";
import { useState } from "react";
import { Area, AreaChart, CartesianGrid, Cell, Pie, PieChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";

import { getControlPreview, type PreviewModel, type PreviewPeriod } from "@/services/api/control-preview";
import { AdminPageFrame } from "../components/admin-shell";
import { AdminStatTile, ListToolbar } from "../components/admin-ui";
import { PreviewChoice, PreviewEmpty, PreviewError, PreviewPanel } from "./preview-controls";
import { PreviewDetails, type PreviewSelection } from "./preview-details";
import { previewCapabilities, previewColors, previewCredits, previewModelSlices, previewNumber, previewPercent, previewPeriods, previewTime, previewUserColor } from "./preview-utils";
import { usePreviewQuery } from "./use-preview-query";
import "./control-preview.css";

const refreshOptions = [
    { value: 0, label: "暂停" },
    { value: 5_000, label: "5s" },
    { value: 30_000, label: "30s" },
    { value: 60_000, label: "60s" },
];
const inflowLabels = { redeem: "兑换码兑换", payment: "在线充值", admin: "管理员增加", reward: "其他奖励" };

export default function ControlPreviewPage() {
    const [period, setPeriod] = useState<PreviewPeriod>("day");
    const [interval, setInterval] = useState(30_000);
    const [top, setTop] = useState(10);
    const [metric, setMetric] = useState<"generations" | "consumedMicrocredits">("generations");
    const [capability, setCapability] = useState("");
    const [stack, setStack] = useState<PreviewSelection[]>([]);
    const query = usePreviewQuery(["overview", period], (signal) => getControlPreview(period, signal), interval);
    const data = query.data;
    const kpi = data?.kpi;
    const periodLabel = previewPeriods.find((item) => item.value === period)!.label;
    const ranking = data?.ranking.slice(0, top) || [];
    const slices = previewModelSlices(data?.models || [], metric, capability);
    const modelTotal = slices.reduce((sum, row) => sum + row.value, 0);
    const sources = (Object.keys(inflowLabels) as Array<keyof typeof inflowLabels>).map((source, index) => ({
        source,
        label: inflowLabels[source],
        color: previewColors[index],
        amount: data?.inflowSources.find((row) => row.source === source)?.amountMicrocredits || 0,
    }));
    const openModel = (row?: PreviewModel) => setStack([row ? { kind: "model", row } : { kind: "models" }]);
    const successRate = kpi && kpi.succeeded + kpi.failed > 0 ? previewPercent(kpi.succeeded / (kpi.succeeded + kpi.failed)) : "--";
    return (
        <AdminPageFrame title="控制预览" description="积分流向、用户消费与模型使用的实时概况" scroll>
            <div className="admin-control-preview" aria-busy={query.isFetching}>
                <ListToolbar
                    trailing={
                        <>
                            <span className="admin-preview-note">{interval ? "自动更新" : "已暂停更新"}</span>
                            <PreviewChoice label="刷新间隔" options={refreshOptions} value={interval} onChange={setInterval} />
                            <Button aria-label="刷新控制预览" title="刷新控制预览" icon={<RefreshCw className="size-4" />} loading={query.isFetching} onClick={() => void query.refetch({ cancelRefetch: false })} />
                            <span className="admin-preview-note" aria-live="polite">
                                {data ? `更新于 ${previewTime(data.asOf)}` : "尚未更新"}
                            </span>
                        </>
                    }
                >
                    <PreviewChoice<PreviewPeriod>
                        label="统计周期"
                        options={previewPeriods}
                        value={period}
                        onChange={(value) => {
                            setPeriod(value);
                            setStack([]);
                        }}
                    />
                    <span className="admin-preview-note">北京时间</span>
                </ListToolbar>
                <PreviewError error={query.error} stale={Boolean(data)} retry={() => void query.refetch({ cancelRefetch: false })} />
                {query.isPending && <Skeleton active paragraph={{ rows: 8 }} />}
                {!query.isPending && (
                    <>
                        <div className="admin-preview-stats">
                            <AdminStatTile
                                label="用户剩余积分 · 当前"
                                value={previewCredits(kpi?.availableMicrocredits)}
                                detail={kpi ? `冻结 ${previewCredits(kpi.reservedMicrocredits)} · 总额 ${previewCredits(kpi.availableMicrocredits + kpi.reservedMicrocredits)}` : "可用积分实时快照"}
                            />
                            <AdminStatTile label={`${periodLabel}消耗积分`} value={previewCredits(kpi?.consumedMicrocredits)} detail={kpi ? `累计 ${previewCredits(kpi.lifetimeConsumedMicrocredits)} · 不含冻结` : "按实际结算统计"} />
                            <div className="admin-preview-inflow-stat">
                                <AdminStatTile label={`${periodLabel}新增到账`} value={previewCredits(kpi?.inflowMicrocredits)} detail="兑换 / 充值 / 管理员 / 奖励" />
                                <button
                                    type="button"
                                    className="admin-preview-link"
                                    onClick={() => document.getElementById("admin-preview-inflows")?.scrollIntoView({ block: "nearest", behavior: window.matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth" })}
                                >
                                    查看来源 <ArrowRight className="size-3" />
                                </button>
                            </div>
                            <AdminStatTile
                                label="在线充值实收（元）"
                                value={kpi ? `¥${(kpi.paidAmountFen / 100).toLocaleString("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}` : "--"}
                                detail={kpi ? `${previewNumber(kpi.payingUsers)} 人 · ${previewNumber(kpi.paidOrders)} 笔已入账订单` : "按成功入账时间 · 人民币"}
                            />
                            <AdminStatTile label="消费用户数" value={previewNumber(kpi?.consumingUsers)} detail={kpi ? `人均消费 ${previewCredits(kpi.consumingUsers ? kpi.consumedMicrocredits / kpi.consumingUsers : 0)} 积分` : "实际消费的去重用户"} />
                            <AdminStatTile label="模型有效调用" value={previewNumber(kpi?.generations)} detail={`生成成功率 ${successRate}${kpi ? ` · 进行中 ${previewNumber(kpi.pending)}` : ""}`} />
                        </div>
                        <div className="admin-preview-main-grid">
                            <PreviewPanel
                                title="用户积分消耗排行"
                                description={`${periodLabel}累计 · 实际结算积分`}
                                actions={
                                    <PreviewChoice
                                        label="排行人数"
                                        options={[
                                            { value: 10, label: "TOP 10" },
                                            { value: 20, label: "TOP 20" },
                                        ]}
                                        value={top}
                                        onChange={setTop}
                                    />
                                }
                            >
                                {ranking.length ? (
                                    <div className="admin-preview-race" style={{ height: ranking.length * 44 }}>
                                        {ranking.map((row, index) => (
                                            <button
                                                type="button"
                                                className="admin-preview-race-row"
                                                key={row.userId}
                                                style={{ transform: `translateY(${index * 44}px)` }}
                                                onClick={() => setStack([{ kind: "user", id: row.userId }])}
                                                aria-label={`${row.username}，排名 ${row.rank}，消费 ${previewCredits(row.consumedMicrocredits)} 积分，查看明细`}
                                            >
                                                <span className="admin-preview-rank">{String(row.rank).padStart(2, "0")}</span>
                                                <span className="admin-preview-user" title={row.username}>
                                                    {row.username}
                                                </span>
                                                <span className="admin-preview-bar-track">
                                                    <span className="admin-preview-bar" style={{ width: `${(row.consumedMicrocredits / ranking[0].consumedMicrocredits) * 100}%`, background: previewUserColor(row.userId) }} />
                                                </span>
                                                <strong>{previewCredits(row.consumedMicrocredits)}</strong>
                                                <span className="admin-preview-share">{previewPercent(row.share)}</span>
                                            </button>
                                        ))}
                                    </div>
                                ) : (
                                    <PreviewEmpty />
                                )}
                                <div className="admin-preview-panel-footer">
                                    <span>{data ? `前 ${ranking.length} 名贡献 ${previewPercent(ranking.reduce((sum, row) => sum + row.share, 0))} 消费` : "--"}</span>
                                    <button type="button" className="admin-preview-link" onClick={() => setStack([{ kind: "ranking" }])}>
                                        查看完整排行 <ArrowRight className="size-3" />
                                    </button>
                                </div>
                            </PreviewPanel>
                            <PreviewPanel
                                title="模型使用分布"
                                actions={
                                    <PreviewChoice<"generations" | "consumedMicrocredits">
                                        label="模型统计维度"
                                        options={[
                                            { value: "generations", label: "调用次数" },
                                            { value: "consumedMicrocredits", label: "消费积分" },
                                        ]}
                                        value={metric}
                                        onChange={setMetric}
                                    />
                                }
                            >
                                <PreviewChoice<string> label="模型能力" options={previewCapabilities} value={capability} onChange={setCapability} />
                                {slices.length ? (
                                    <div className="admin-preview-model-chart">
                                        <div className="admin-preview-donut" role="img" aria-label={`模型${metric === "generations" ? "调用次数" : "消费积分"}比例，明细可通过下方列表查看`}>
                                            <ResponsiveContainer width="100%" height="100%">
                                                <PieChart>
                                                    <Pie
                                                        data={slices}
                                                        dataKey="value"
                                                        nameKey="label"
                                                        innerRadius="67%"
                                                        outerRadius="90%"
                                                        paddingAngle={2}
                                                        stroke="var(--admin-surface)"
                                                        isAnimationActive={false}
                                                        onClick={(_, index) => openModel(slices[index].model)}
                                                    >
                                                        {slices.map((row) => (
                                                            <Cell key={row.key} fill={row.color} />
                                                        ))}
                                                    </Pie>
                                                    <Tooltip
                                                        formatter={(value) => (metric === "generations" ? previewNumber(Number(value)) : previewCredits(Number(value)))}
                                                        contentStyle={{ background: "var(--admin-surface)", borderColor: "var(--admin-border)", color: "var(--admin-text)" }}
                                                    />
                                                </PieChart>
                                            </ResponsiveContainer>
                                            <div className="admin-preview-donut-center">
                                                <strong>{metric === "generations" ? previewNumber(modelTotal) : previewCredits(modelTotal)}</strong>
                                                <span>{metric === "generations" ? "有效生成" : "消费积分"}</span>
                                            </div>
                                        </div>
                                        <div className="admin-preview-model-legend">
                                            {slices.map((row) => (
                                                <button type="button" key={row.key} onClick={() => openModel(row.model)}>
                                                    <span className="admin-preview-dot" style={{ background: row.color }} />
                                                    <span className="admin-preview-model-name" title={row.label}>
                                                        {row.label}
                                                        <small>{row.model ? previewCapabilities.find((item) => item.value === row.model!.capability)?.label : "其他模型"}</small>
                                                    </span>
                                                    <strong>{metric === "generations" ? previewNumber(row.value) : previewCredits(row.value)}</strong>
                                                    <span>{previewPercent(row.value / modelTotal)}</span>
                                                </button>
                                            ))}
                                        </div>
                                    </div>
                                ) : (
                                    <PreviewEmpty />
                                )}
                                <div className="admin-preview-panel-footer">
                                    <span>点击模型，查看用户调用明细</span>
                                    <button type="button" className="admin-preview-link" onClick={() => openModel()}>
                                        全部模型 <ArrowRight className="size-3" />
                                    </button>
                                </div>
                            </PreviewPanel>
                        </div>
                        <div className="admin-preview-bottom-grid">
                            <PreviewPanel
                                title="积分到账与消费趋势"
                                description={`${periodLabel} · ${period === "day" ? "按小时" : "按日"}统计`}
                                actions={
                                    <div className="admin-preview-chart-legend">
                                        <span>
                                            <i style={{ background: previewColors[1] }} />
                                            新增到账
                                        </span>
                                        <span>
                                            <i style={{ background: previewColors[0] }} />
                                            实际消费
                                        </span>
                                    </div>
                                }
                            >
                                {data ? (
                                    <div className="admin-preview-trend" role="img" aria-label="积分到账与消费趋势，按实际入账和结算时间统计">
                                        <ResponsiveContainer width="100%" height="100%">
                                            <AreaChart data={data.trend.map((row) => ({ label: row.label, inflow: row.inflowMicrocredits / 1_000_000, consumed: row.consumedMicrocredits / 1_000_000 }))} margin={{ top: 12, right: 12, left: 0, bottom: 0 }}>
                                                <CartesianGrid stroke="var(--admin-divider)" vertical={false} />
                                                <XAxis dataKey="label" tick={{ fill: "var(--admin-text-muted)", fontSize: 11 }} minTickGap={30} tickLine={false} axisLine={false} />
                                                <YAxis tick={{ fill: "var(--admin-text-muted)", fontSize: 11 }} tickLine={false} axisLine={false} width={64} tickFormatter={(value) => previewNumber(Number(value))} />
                                                <Tooltip formatter={(value) => `${previewNumber(Number(value))} 积分`} contentStyle={{ background: "var(--admin-surface)", borderColor: "var(--admin-border)", color: "var(--admin-text)" }} />
                                                <Area name="新增到账" type="linear" dataKey="inflow" stroke={previewColors[1]} fill={previewColors[1]} fillOpacity={0.1} strokeWidth={2} isAnimationActive={false} />
                                                <Area name="实际消费" type="linear" dataKey="consumed" stroke={previewColors[0]} fill={previewColors[0]} fillOpacity={0.03} strokeWidth={2} isAnimationActive={false} />
                                            </AreaChart>
                                        </ResponsiveContainer>
                                    </div>
                                ) : (
                                    <PreviewEmpty />
                                )}
                            </PreviewPanel>
                            <PreviewPanel
                                id="admin-preview-inflows"
                                title={`${periodLabel}到账来源`}
                                actions={
                                    <span className="admin-preview-note">
                                        合计 <strong>{previewCredits(kpi?.inflowMicrocredits)}</strong>
                                    </span>
                                }
                            >
                                <div className="admin-preview-source-bar" aria-hidden="true">
                                    {sources.map((row) => (
                                        <span key={row.source} style={{ width: `${kpi?.inflowMicrocredits ? (row.amount / kpi.inflowMicrocredits) * 100 : 0}%`, background: row.color }} />
                                    ))}
                                </div>
                                <div className="admin-preview-sources">
                                    {sources.map((row) => (
                                        <div key={row.source}>
                                            <span className="admin-preview-dot" style={{ background: row.color }} />
                                            <span>{row.label}</span>
                                            <strong>{data ? previewCredits(row.amount) : "--"}</strong>
                                            <span>{data ? previewPercent(kpi?.inflowMicrocredits ? row.amount / kpi.inflowMicrocredits : 0) : "--"}</span>
                                        </div>
                                    ))}
                                </div>
                                <p className="admin-preview-note">退款及冻结退回不计入新增到账；兑换包含管理员个人出资码。</p>
                            </PreviewPanel>
                        </div>
                        <p className="admin-preview-note">仅统计当前保留的普通用户数据 · 管理员及已永久注销用户不计入 · 消费按结算时间，生成按首次请求时间统计 · 成功率仅包含已结束生成</p>
                        {Boolean(kpi?.unlinkedGenerations) && <p className="admin-preview-note">有 {previewNumber(kpi?.unlinkedGenerations)} 条历史生成缺少任务或请求关联，已按独立日志计数。</p>}
                    </>
                )}
                <PreviewDetails stack={stack} onStackChange={setStack} period={period} interval={interval} overview={data} />
            </div>
        </AdminPageFrame>
    );
}
