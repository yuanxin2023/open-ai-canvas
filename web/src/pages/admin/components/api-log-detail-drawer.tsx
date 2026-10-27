import { useEffect, useState, type ReactNode } from "react";
import { Alert, App, Button, Descriptions, Input, Skeleton, Tabs, Typography } from "antd";
import { AdminEmpty, AdminStatusBadge } from "@/pages/admin/components/admin-ui";
import { Activity, Clipboard, Link2, RefreshCw, Save, Video } from "lucide-react";
import { AdminModal } from "@/pages/admin/ui/overlays";
import { formatCredits } from "@/constant/credits";
import { hasAdminPermission } from "@/lib/admin-permissions";
import { getAdminApiLog, getAdminApiLogByBillingOrder, queryAdminApiLogTask, recoverAdminApiLogVideoByURL, type ApiCallLog } from "@/services/api/auth";
import { useUserStore } from "@/stores/use-user-store";
import { AdminUserDetailModal } from "./admin-user-detail-drawer";

export function ApiLogDetailModal({ logId = null, billingOrderId = null, onClose, onLogUpdated }: { logId?: string | null; billingOrderId?: string | null; onClose: () => void; onLogUpdated?: (log: ApiCallLog) => void }) {
    const { message } = App.useApp();
    const adminAccess = useUserStore((state) => state.user?.adminAccess);
    const canViewUserDetails = hasAdminPermission(adminAccess, "admin.users.accounts");
    const [log, setLog] = useState<ApiCallLog | null>(null);
    const [selectedUserId, setSelectedUserId] = useState<string | null>(null);
    const [loading, setLoading] = useState(false);
    const [querying, setQuerying] = useState(false);
    const [providerRequestId, setProviderRequestId] = useState("");
    const [videoURL, setVideoURL] = useState("");
    const [resultText, setResultText] = useState("");

    const refresh = async (id: string) => {
        const refreshed = await getAdminApiLog(id);
        setLog(refreshed.log);
        setProviderRequestId(refreshed.log.providerRequestId || "");
        onLogUpdated?.(refreshed.log);
        return refreshed.log;
    };

    useEffect(() => {
        setSelectedUserId(null);
        if (!logId && !billingOrderId) {
            setLog(null);
            return;
        }
        let active = true;
        setLoading(true);
        setLog(null);
        setResultText("");
        setVideoURL("");
        const request = logId ? getAdminApiLog(logId) : getAdminApiLogByBillingOrder(billingOrderId!);
        void request
            .then((result) => {
                if (!active) return;
                setLog(result.log);
                setProviderRequestId(result.log.providerRequestId || "");
            })
            .catch((error) => active && message.error(error instanceof Error ? error.message : "读取请求详情失败"))
            .finally(() => active && setLoading(false));
        return () => {
            active = false;
        };
    }, [billingOrderId, logId, message]);

    const runQuery = async () => {
        if (!log) return;
        setQuerying(true);
        setResultText("");
        try {
            const result = await queryAdminApiLogTask(log.id, providerRequestId.trim() || undefined);
            await refresh(log.id);
            if (result.recovered) {
                window.dispatchEvent(new CustomEvent("wallet:updated"));
                setResultText(result.billingSettled ? "查询成功，视频已转存并完成结算。" : "视频已转存，计费状态待核对。");
                message.success("视频任务恢复成功");
            } else {
                setResultText(`上游仍在生成中${result.providerStatus ? `（${result.providerStatus}）` : ""}，暂不扣费。`);
            }
        } catch (error) {
            setResultText(error instanceof Error ? error.message : "查询上游任务失败");
        } finally {
            setQuerying(false);
        }
    };

    const runURLRecovery = async () => {
        if (!log || !videoURL.trim()) return;
        setQuerying(true);
        setResultText("");
        try {
            const result = await recoverAdminApiLogVideoByURL(log.id, videoURL.trim(), providerRequestId.trim() || undefined);
            await refresh(log.id);
            window.dispatchEvent(new CustomEvent("wallet:updated"));
            setResultText(result.billingSettled ? "视频已转存并完成结算。" : "视频已转存，计费状态待核对。");
            message.success("视频转存成功");
        } catch (error) {
            setResultText(error instanceof Error ? error.message : "视频转存失败");
        } finally {
            setQuerying(false);
        }
    };

    const title = log ? (
        <div className="flex min-w-0 items-center gap-3">
            <div className="grid size-9 shrink-0 place-items-center rounded-lg bg-blue-500/12 text-blue-500">
                <Activity className="size-[18px]" aria-hidden="true" />
            </div>
            <div className="min-w-0">
                <div className="truncate text-[15px] font-semibold text-foreground">请求日志详情</div>
                <div className="mt-0.5 truncate text-xs font-normal text-foreground/50">
                    {capabilityText(log.capability)} · {log.model || "未识别模型"} · {formatTime(log.startedAt || log.createdAt)}
                </div>
            </div>
        </div>
    ) : "请求日志详情";

    return (
        <>
            <AdminModal
                title={title}
                open={Boolean(logId || billingOrderId)}
                centered
                width="min(1200px, calc(100vw - 32px))"
                onCancel={() => {
                    setSelectedUserId(null);
                    onClose();
                }}
                footer={null}
                rootClassName="admin-api-log-detail-modal"
                styles={{ body: { maxHeight: "min(76vh, 760px)", overflowX: "hidden", overflowY: "auto" } }}
            >
                {loading ? <Skeleton active paragraph={{ rows: 12 }} /> : log ? <LogDetail log={log} providerRequestId={providerRequestId} videoURL={videoURL} resultText={resultText} querying={querying} onProviderRequestIdChange={setProviderRequestId} onVideoURLChange={setVideoURL} onQuery={runQuery} onRecoverURL={runURLRecovery} onViewUser={canViewUserDetails ? setSelectedUserId : undefined} /> : <AdminEmpty size="compact" title="没有请求详情" />}
            </AdminModal>
            <AdminUserDetailModal userId={canViewUserDetails ? selectedUserId : null} onClose={() => setSelectedUserId(null)} />
        </>
    );
}

type LogDetailProps = {
    log: ApiCallLog;
    onViewUser?: (userId: string) => void;
    providerRequestId: string;
    videoURL: string;
    resultText: string;
    querying: boolean;
    onProviderRequestIdChange: (value: string) => void;
    onVideoURLChange: (value: string) => void;
    onQuery: () => void;
    onRecoverURL: () => void;
};

function LogDetail({ log, providerRequestId, videoURL, resultText, querying, onProviderRequestIdChange, onVideoURLChange, onQuery, onRecoverURL, onViewUser }: LogDetailProps) {
    const providerStatus = log.providerStatus?.toLowerCase();
    const processing = ["queued", "pending", "processing", "running", "in_progress"].includes(providerStatus || "");
    const failed = log.status === "failed" || ["failed", "cancelled", "expired"].includes(providerStatus || "");
    const userLabel = log.userDisplayName || log.userAccount || "未知用户";
    const userContent = (
        <div>
            <div className="font-medium text-foreground/90 group-hover:underline group-hover:underline-offset-2">{userLabel}</div>
            {log.userAccount ? <div className="mt-0.5 text-xs text-foreground/50">@{log.userAccount}</div> : null}
        </div>
    );
    const items = [
        ["时间", new Date(log.startedAt || log.createdAt).toLocaleString("zh-CN", { hour12: false })],
        ["状态", <AdminStatusBadge label={failed ? "失败" : processing ? "处理中" : "成功"} tone={failed ? "error" : processing ? "warning" : "success"} />],
        [
            "用户",
            onViewUser && log.userId?.trim() ? (
                <button
                    type="button"
                    className="group block cursor-pointer rounded-sm text-left focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--admin-accent)]"
                    aria-label={`查看用户 ${userLabel} 详情`}
                    onClick={() => onViewUser(log.userId)}
                >
                    {userContent}
                </button>
            ) : userContent,
        ],
        [
            "渠道 / 模型",
            <div>
                <div className="text-foreground/85">{log.channelName || "未记录渠道"}</div>
                <div className="mt-0.5 font-mono text-xs text-foreground/55" title={log.model}>
                    {log.model || "未识别模型"}
                </div>
            </div>,
        ],
        ["能力", capabilityText(log.capability)],
        ["请求阶段", requestKindText(log.requestKind)],
        ["计费属性", log.billable ? "计费调用" : "不计费"],
        ["总耗时", <span className="tabular-nums">{formatDuration(log.durationMs)}</span>],
        ["视频轮询", log.capability === "video" ? <span className="tabular-nums">{log.pollCount || 0} 次</span> : <span className="text-foreground/35">--</span>],
        [
            "Token 用量",
            log.usageAvailable ? (
                <div className="space-y-1 font-mono text-sm tabular-nums">
                    <div>
                        <span className="text-foreground/55">输入</span> <span className="ml-2 text-foreground/85">{log.inputTokens.toLocaleString()}</span>
                    </div>
                    <div>
                        <span className="text-foreground/55">输出</span> <span className="ml-2 text-foreground/85">{log.outputTokens.toLocaleString()}</span>
                    </div>
                    {log.cachedTokens > 0 ? (
                        <div>
                            <span className="text-foreground/55">缓存</span> <span className="ml-2 text-foreground/85">{log.cachedTokens.toLocaleString()}</span>
                        </div>
                    ) : null}
                </div>
            ) : (
                <span className="text-foreground/35">未返回</span>
            ),
        ],
        ["销售价格（积分）", billingText(log)],
        ["成本价格（积分）", log.creditCostMicrocredits !== undefined ? `${formatCredits(log.creditCostMicrocredits)} 积分` : log.creditCostConfigured ? "待核算" : "未配置"],
        ["上游成本", log.costAvailable ? <span className="font-mono tabular-nums">{log.currency || "USD"} {(log.estimatedCostMicros / 1_000_000).toFixed(6)}</span> : <span className="text-foreground/35">未配置成本</span>],
        [
            "错误信息",
            [log.errorCode, log.error].filter(Boolean).length > 0 ? (
                <div>
                    {log.errorCode ? <div className="mb-1 font-mono text-sm font-medium text-red-500">{log.errorCode}</div> : null}
                    {log.error ? <div className="text-sm text-foreground/75">{log.error}</div> : null}
                </div>
            ) : (
                <span className="text-foreground/35">--</span>
            ),
        ],
        ["方法与路径", <code className="text-sm text-foreground/80">{log.method} {log.path}</code>],
        ["请求 Content-Type", log.requestContentType ? <code className="text-sm text-foreground/70">{log.requestContentType}</code> : <span className="text-foreground/35">--</span>],
        ["HTTP 状态", log.statusCode ? <span className="tabular-nums">{log.statusCode}</span> : <span className="text-foreground/35">--</span>],
        ["任务 ID", log.taskId ? <code className="font-mono text-sm text-foreground/70">{log.taskId}</code> : <span className="text-foreground/35">--</span>],
        ["供应商任务 ID", log.providerRequestId ? <code className="font-mono text-sm text-foreground/70">{log.providerRequestId}</code> : <span className="text-foreground/35">--</span>],
        ["上游地址", log.upstreamUrl ? <code className="break-all text-sm text-foreground/70">{log.upstreamUrl}</code> : <span className="text-foreground/35">--</span>],
    ].map(([label, children], index) => ({ key: String(index), label, children }));

    const status = failed ? { label: "失败", tone: "error" as const } : processing ? { label: "处理中", tone: "warning" as const } : { label: "成功", tone: "success" as const };

    return (
        <div className="api-log-detail-body">
            <section className="api-log-detail-section">
                <div className="api-log-detail-section-heading">
                    <div>
                        <div className="api-log-detail-eyebrow">REQUEST OVERVIEW</div>
                        <h2>请求概览</h2>
                    </div>
                    <AdminStatusBadge label={status.label} tone={status.tone} />
                </div>
                <div className="api-log-detail-summary-grid">
                    <InfoItem label="请求 ID" value={<CopyValue value={log.id} />} />
                    <InfoItem label="任务状态" value={mediaDeliveryText(log)} />
                </div>
                <Descriptions column={{ xs: 1, sm: 2 }} items={items} />
                {log.error || log.errorCode ? (
                    <div className="api-log-detail-error">
                        <div className="text-xs font-semibold text-red-500">{log.errorCode || `HTTP ${log.statusCode || "失败"}`}</div>
                        <div className="mt-1 text-sm text-foreground/70">{log.error || "上游未返回错误详情"}</div>
                    </div>
                ) : null}
            </section>

            {log.capability === "video" ? (
                <section className="api-log-detail-section api-log-recovery-section">
                    <div className="api-log-detail-section-heading">
                        <div className="flex items-center gap-2.5">
                            <div className="grid size-8 place-items-center rounded-md bg-violet-500/12 text-violet-500">
                                <Video className="size-4" aria-hidden="true" />
                            </div>
                            <div>
                                <div className="api-log-detail-eyebrow">VIDEO RECOVERY</div>
                                <h2>视频任务恢复</h2>
                            </div>
                        </div>
                        <span className="text-xs text-foreground/45">失败任务可用</span>
                    </div>
                    <div className="api-log-recovery-body">
                        <div className="api-log-recovery-row">
                            <div className="api-log-recovery-label">
                                <span>供应商任务 ID</span>
                                <small>默认使用当前记录，可手动覆盖</small>
                            </div>
                            <Input className="api-log-recovery-input" value={providerRequestId} onChange={(event) => onProviderRequestIdChange(event.target.value)} placeholder="输入供应商任务 ID" />
                            <Button icon={<RefreshCw className="size-3.5" />} loading={querying} onClick={onQuery}>自动查询</Button>
                            <Button type="primary" icon={<Save className="size-3.5" />} loading={querying} onClick={onQuery}>保存并查询</Button>
                        </div>
                        <div className="api-log-recovery-row">
                            <div className="api-log-recovery-label">
                                <span>视频 URL 转存</span>
                                <small>仅支持公网 HTTP / HTTPS 视频地址</small>
                            </div>
                            <Input className="api-log-recovery-input" prefix={<Link2 className="size-4 text-foreground/35" />} value={videoURL} onChange={(event) => onVideoURLChange(event.target.value)} placeholder="https://example.com/video.mp4" />
                            <Button icon={<Save className="size-3.5" />} loading={querying} disabled={!videoURL.trim()} onClick={onRecoverURL}>转存并回写</Button>
                        </div>
                        {resultText ? <Alert className="api-log-recovery-result" type={resultText.includes("失败") || resultText.includes("错误") ? "error" : resultText.includes("生成中") ? "warning" : "success"} showIcon message={resultText} /> : null}
                    </div>
                </section>
            ) : null}

            <section className="api-log-detail-section api-log-payload-section">
                <div className="api-log-detail-section-heading">
                    <div>
                        <div className="api-log-detail-eyebrow">RAW PAYLOAD</div>
                        <h2>请求报文</h2>
                    </div>
                    <span className="text-xs text-foreground/45">{log.requestBody || log.responseBody ? "可复制原始内容" : "暂无报文"}</span>
                </div>
                <Tabs items={[{ key: "request", label: "请求", children: <PayloadPanel value={log.requestBody} empty="该请求未记录请求报文" /> }, { key: "response", label: "响应", children: <PayloadPanel value={log.responseBody} empty="该请求未记录响应报文" /> }]} />
            </section>
        </div>
    );
}

function InfoItem({ label, value, detail }: { label: string; value: ReactNode; detail?: string }) {
    return (
        <div className="api-log-detail-info-item">
            <div className="api-log-detail-info-label">{label}</div>
            <div className="api-log-detail-info-value">{value}</div>
            {detail ? <div className="api-log-detail-info-detail">{detail}</div> : null}
        </div>
    );
}

function CopyValue({ value }: { value: string }) {
    return (
        <Typography.Text className="api-log-copy-value" copyable={{ text: value, tooltips: ["复制", "已复制"] }}>
            <code>{value}</code>
        </Typography.Text>
    );
}

function PayloadPanel({ value, empty }: { value?: string; empty: string }) {
    if (!value) return <AdminEmpty size="compact" title={empty} />;
    return (
        <div className="api-log-payload-wrap">
            <div className="api-log-payload-actions">
                <Typography.Text copyable={{ text: value, tooltips: ["复制报文", "已复制"] }}>
                    <Clipboard className="mr-1 inline size-3.5" />复制报文
                </Typography.Text>
            </div>
            <pre className="api-log-payload">{value}</pre>
        </div>
    );
}

function billingText(log: ApiCallLog) {
    if (!log.billable) return "不计费";
    if (!log.billingAvailable) return "未扣积分";
    const labels = { settled: "已结算", refunded: "已退回", uncertain: "待核对", running: "运行中", reserved: "已预授权" } as const;
    return `${formatCredits(log.billingAmountMicrocredits)} 积分 · ${labels[log.billingStatus || "reserved"]}`;
}

function mediaDeliveryText(log: ApiCallLog) {
    if (log.taskStatus === "succeeded") return log.mediaStage === "completed" ? "已保存并登记" : "任务成功";
    if (log.taskStatus === "failed") return "任务失败";
    return log.taskStatus || "未记录";
}

function requestKindText(value: ApiCallLog["requestKind"]) {
    const labels: Partial<Record<ApiCallLog["requestKind"], string>> = { create: "模型生成", poll: "状态查询", download: "结果下载", upload: "上传 OSS", local_save: "保存文件", register: "登记素材", repair: "结果修复" };
    return labels[value] || "上游请求";
}

function capabilityText(value: string) {
    return ({ text: "文本", image: "图片", video: "视频", audio: "音频" } as Record<string, string>)[value] || "未知";
}

function formatTime(value?: string) {
    return value ? new Date(value).toLocaleString("zh-CN", { hour12: false }) : "--";
}

function formatDuration(value: number) {
    if (value < 1_000) return `${value} ms`;
    if (value < 60_000) return `${(value / 1_000).toFixed(value < 10_000 ? 1 : 0)} 秒`;
    return `${Math.floor(value / 60_000)} 分 ${Math.round((value % 60_000) / 1_000)} 秒`;
}
