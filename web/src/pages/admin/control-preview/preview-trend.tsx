import { CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import type { PreviewOverview } from "@/services/api/control-preview";
import { PreviewEmpty } from "./preview-controls";
import { previewCredits, previewNumber, previewTrendTime } from "./preview-utils";

export function PreviewTrend({ trend }: { trend: PreviewOverview["trend"] }) {
    if (!trend.length) return <PreviewEmpty />;
    const empty = trend.every((row) => row.inflowMicrocredits === 0 && row.consumedMicrocredits === 0);
    return (
        <>
            <div className="admin-preview-trend" role="group" aria-label="积分到账与消费趋势，按实际入账和结算时间统计，单位为积分">
                <ResponsiveContainer width="100%" height="100%">
                    <LineChart data={trend} margin={{ top: 24, right: 16, left: 0, bottom: 0 }}>
                        <CartesianGrid stroke="var(--admin-divider)" strokeDasharray="3 4" vertical={false} />
                        <XAxis dataKey="label" tick={{ fill: "var(--admin-text-muted)", fontSize: 11 }} minTickGap={30} tickLine={false} axisLine={false} />
                        <YAxis
                            domain={[0, "auto"]}
                            tick={{ fill: "var(--admin-text-muted)", fontSize: 11 }}
                            tickLine={false}
                            axisLine={false}
                            width={64}
                            tickFormatter={(value) => previewNumber(Number(value) / 1_000_000)}
                            label={{ value: "积分", position: "top", offset: 10, fill: "var(--admin-text-muted)", fontSize: 11 }}
                        />
                        <Tooltip
                            isAnimationActive={false}
                            formatter={(value) => `${previewCredits(Number(value))} 积分`}
                            labelFormatter={(_, payload) => (payload[0] ? `${previewTrendTime(payload[0].payload.time)} · 北京时间` : "")}
                            contentStyle={{ background: "var(--admin-surface)", borderColor: "var(--admin-border)", borderRadius: "var(--admin-radius)", color: "var(--admin-text)" }}
                            labelStyle={{ color: "var(--admin-text)" }}
                        />
                        <Line
                            name="新增到账"
                            type="monotone"
                            dataKey="inflowMicrocredits"
                            stroke="var(--admin-preview-inflow)"
                            strokeDasharray="6 5"
                            strokeWidth={2.5}
                            dot={trend.length === 1 ? { r: 3 } : false}
                            activeDot={{ r: 4 }}
                            isAnimationActive={false}
                        />
                        <Line name="实际消费" type="monotone" dataKey="consumedMicrocredits" stroke="var(--admin-preview-consumed)" strokeWidth={2.5} dot={trend.length === 1 ? { r: 3 } : false} activeDot={{ r: 4 }} isAnimationActive={false} />
                    </LineChart>
                </ResponsiveContainer>
            </div>
            {empty && <p className="admin-preview-note">当前范围暂无到账与消费</p>}
        </>
    );
}
