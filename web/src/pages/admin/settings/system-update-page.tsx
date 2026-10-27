import { App, Button, Input, Skeleton } from "antd";
import { AlertTriangle, Archive, BadgeCheck, Circle, ExternalLink, History, RefreshCw, RotateCcw, ServerCog, ShieldCheck } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";

import { checkSystemUpdate, getSystemUpdateStatus, rollbackSystemUpdate, startSystemUpdate, type SystemUpdateStatus, type UpdatePhase } from "@/services/api/system-update";
import { hasAdminPermission } from "@/lib/admin-permissions";
import { useUserStore } from "@/stores/use-user-store";
import { AdminPageFrame } from "../components/admin-shell";
import { AdminStatusBadge, SettingsSectionCard } from "../components/admin-ui";

const activePhases = new Set<UpdatePhase>(["checking", "preflight", "backing_up", "pulling", "draining", "migrating", "switching", "verifying", "rolling_back"]);

const phaseLabels: Record<UpdatePhase, string> = {
    idle: "等待检查",
    checking: "正在检查",
    ready: "可更新",
    no_update: "已是最新",
    preflight: "前置检查",
    backing_up: "正在备份",
    pulling: "拉取镜像",
    draining: "停止旧服务",
    migrating: "迁移数据库",
    switching: "切换版本",
    verifying: "健康验证",
    succeeded: "更新成功",
    rolling_back: "正在回退",
    rolled_back: "已回退",
    failed: "更新失败",
    manual_intervention: "需人工介入",
};

export default function SystemUpdatePage() {
    const user = useUserStore((state) => state.user);
    if (!user?.id || user.role !== "admin" || !hasAdminPermission(user.adminAccess, "admin.settings.system_update")) return <p role="alert">没有系统更新权限</p>;
    return <SystemUpdateContent key={user.id} actorId={user.id} />;
}

function SystemUpdateContent({ actorId }: { actorId: string }) {
    const { message, modal } = App.useApp();
    const [status, setStatus] = useState<SystemUpdateStatus | null>(null);
    const [loading, setLoading] = useState(true);
    const [checking, setChecking] = useState(false);
    const [starting, setStarting] = useState(false);
    const [reconnecting, setReconnecting] = useState(false);
    const [loadError, setLoadError] = useState("");
    const mountedRef = useRef(true);
    const confirmations = useRef<Array<{ destroy: () => void }>>([]);
    const current = useCallback(() => {
        const user = useUserStore.getState().user;
        return mountedRef.current && user?.id === actorId && user.role === "admin" && hasAdminPermission(user.adminAccess, "admin.settings.system_update");
    }, [actorId]);

    const load = useCallback(async (initial = false) => {
        if (initial) setLoading(true);
        try {
            const next = await getSystemUpdateStatus();
            if (!current()) return;
            setStatus(next);
            setLoadError("");
            setReconnecting(false);
        } catch (error) {
            if (!current()) return;
            const errorMessage = error instanceof Error ? error.message : "读取系统更新状态失败";
            setLoadError(errorMessage);
            if (status && activePhases.has(status.operation.phase)) setReconnecting(true);
        } finally {
            if (current() && initial) setLoading(false);
        }
    }, [status, current]);

    useEffect(() => {
        mountedRef.current = true;
        void load(true);
        return () => {
            mountedRef.current = false;
            confirmations.current.forEach((confirmation) => confirmation.destroy());
        };
    }, []); // eslint-disable-line react-hooks/exhaustive-deps

    useEffect(() => {
        if (!status || !activePhases.has(status.operation.phase)) return;
        const timer = window.setInterval(() => void load(false), 2500);
        return () => window.clearInterval(timer);
    }, [load, status]);

    const operationActive = Boolean(status && activePhases.has(status.operation.phase));
    const blockingCheckFailed = status?.checks.some((check) => check.blocking && check.status === "failed") ?? true;

    const requestCheck = async () => {
        if (!current() || checking || operationActive) return;
        setChecking(true);
        try {
            const next = await checkSystemUpdate(actorId);
            if (!current()) return;
            setStatus(next);
            message.success(next.updateAvailable ? `发现新版本 ${next.latestRelease?.version}` : "当前已是最新版本");
        } catch (error) {
            if (!current()) return;
            message.error(error instanceof Error ? error.message : "检查更新失败");
            await load(false);
        } finally {
            if (current()) setChecking(false);
        }
    };

    const requestStart = () => {
        if (!current() || !status?.latestRelease || status.latestRelease.prerelease || !status.updateAvailable || operationActive || starting || blockingCheckFailed) return;
        const target = status.latestRelease.version;
        confirmations.current.push(modal.confirm({
            title: `更新到 ${target}？`,
            width: 560,
            content: (
                <div className="space-y-3 text-sm text-foreground/70">
                    <p>系统会先创建并校验数据库 ZIP 与数据目录备份，再停止旧服务、执行迁移并切换镜像。</p>
                    <p>更新期间站点会短暂不可用。前置检查失败时停止升级；切换后验证失败会尝试按备份恢复，恢复未完成时会提示人工处理。</p>
                </div>
            ),
            okText: "确认并开始更新",
            cancelText: "取消",
            onOk: async () => {
                if (!current()) throw new Error("管理员账号已变化，请重新打开更新页面");
                setStarting(true);
                try {
                    const next = await startSystemUpdate(target, actorId);
                    if (!current()) return;
                    setStatus(next);
                    message.success("更新任务已启动，页面会自动重连并显示进度");
                } catch (error) {
                    if (current()) message.error(error instanceof Error ? error.message : "无法开始更新");
                    throw error;
                } finally {
                    if (current()) setStarting(false);
                }
            },
        }));
    };

    const requestRollback = () => {
        if (!current() || !status?.rollbackVersion || operationActive) return;
        let reason = "";
        confirmations.current.push(modal.confirm({
            title: `回退到 ${status.rollbackVersion}？`,
            width: 520,
            content: (
                <div className="space-y-3">
                    <p className="text-sm text-foreground/65">回退会停止当前服务、恢复最近一次已校验数据库备份并重新启动旧镜像。请先确认备份时间与业务影响。</p>
                    <Input.TextArea rows={3} maxLength={300} showCount placeholder="填写回退原因（必填）" onChange={(event) => (reason = event.target.value)} />
                </div>
            ),
            okText: "开始回退",
            okButtonProps: { danger: true },
            cancelText: "取消",
            onOk: async () => {
                if (!current()) throw new Error("管理员账号已变化，请重新打开更新页面");
                if (!reason.trim()) {
                    message.warning("请填写回退原因");
                    throw new Error("rollback reason required");
                }
                const next = await rollbackSystemUpdate(reason.trim(), actorId);
                if (!current()) return;
                setStatus(next);
                message.success("回退任务已启动");
            },
        }));
    };

    if (loading) {
        return (
            <AdminPageFrame title="系统更新" description="检查自有仓库的完整正式 Release，由管理员确认后执行升级。" scroll>
                <div className="admin-settings-stack admin-system-update"><Skeleton active paragraph={{ rows: 12 }} /></div>
            </AdminPageFrame>
        );
    }

    return (
        <AdminPageFrame
            title="系统更新"
            description="检查自有仓库的完整正式 Release，由管理员确认后执行升级。"
            scroll
            actions={
                <Button icon={<RefreshCw className="size-4" />} loading={checking} disabled={operationActive || !status?.supported} onClick={() => void requestCheck()}>
                    检查更新
                </Button>
            }
        >
            <div className="admin-settings-stack admin-system-update">
                {loadError && !status ? <UpdateAlert tone="error" title="无法读取更新状态" detail={loadError} /> : null}
                {!status?.supported ? <UpdateAlert tone="warning" title="当前部署不支持后台在线更新" detail="请先在服务器安装 Host Updater，并重建 backend 容器挂载 Unix Socket。此状态下不会执行任何更新操作。" /> : null}
                {reconnecting ? <UpdateAlert tone="warning" title="服务正在切换，等待重新连接" detail="更新器运行在宿主机，后台页面暂时断线不会中止更新。连接恢复后会继续显示最终结果。" /> : null}
                {status?.operation.phase === "manual_intervention" ? <UpdateAlert
                    tone="error"
                    title={status.operation.rollbackError ? "自动回退未完成，需要人工介入" : "应用已更新，Host Updater 需要人工处理"}
                    detail={status.operation.rollbackError || status.operation.error || "请检查 Host Updater 和容器日志。"}
                /> : null}

                <div className="admin-system-update-grid">
                    <SettingsSectionCard
                        className="admin-system-update-release"
                        layout="stacked"
                        icon={<ServerCog className="size-4" />}
                        title="可用版本"
                        description={status?.updateAvailable ? `当前运行 ${status.currentVersion}，可更新到 ${status.latestRelease?.version}。` : "只检查自有仓库的完整正式 Release；普通提交和预发布不会自动上线。"}
                        status={<PhaseBadge phase={status?.operation.phase ?? "idle"} />}
                        footer={
                            <>
                                <span className="text-xs leading-5 text-foreground/50">每次在线更新都会重新备份数据库，并校验镜像摘要、迁移结果和服务健康状态。</span>
                                <Button type="primary" loading={starting} disabled={!status?.connected || !status.updateAvailable || operationActive || blockingCheckFailed} onClick={requestStart}>
                                    开始更新
                                </Button>
                            </>
                        }
                    >
                        <div className="admin-system-update-release-body">
                            <div className="admin-system-update-versions">
                                <div>
                                    <span>当前运行</span>
                                    <strong>{status?.currentVersion || "未识别"}</strong>
                                    {status?.currentVersion && !isReleaseVersion(status.currentVersion) ? <AdminStatusBadge label="非正式版本" tone="warning" /> : null}
                                </div>
                                <div>
                                    <span>最新发布</span>
                                    <strong>{status?.latestRelease?.version || "尚未检查"}</strong>
                                    {status?.latestRelease?.prerelease ? <AdminStatusBadge label="预览版" tone="warning" /> : null}
                                    {status?.updateAvailable ? <AdminStatusBadge label="可更新" tone="success" /> : null}
                                </div>
                            </div>
                            {status?.updateAvailable && status.latestRelease ? (
                                <p className="admin-system-update-hint">
                                    {isReleaseVersion(status.currentVersion)
                                        ? `可以从 ${status.currentVersion} 更新到 ${status.latestRelease.version}。`
                                        : `当前镜像标签不是正式版本号，所以会显示成 ${status.currentVersion}。点开始更新后会切换到 ${status.latestRelease.version}。`}
                                </p>
                            ) : null}
                            <dl className="admin-system-update-facts">
                                <dt>部署方式</dt><dd>{status?.deployment || "未知"}</dd>
                                <dt>发布时间</dt><dd>{formatDate(status?.latestRelease?.publishedAt)}</dd>
                                <dt>代码仓库</dt><dd>{status?.repository || "尚未连接自有更新源"}</dd>
                            </dl>
                            {status?.latestRelease ? (
                                <div className="admin-system-update-notes">
                                    <div className="flex items-center justify-between gap-3"><h3>更新日志</h3><a href={status.latestRelease.url} target="_blank" rel="noreferrer">查看 Release <ExternalLink className="size-3" /></a></div>
                                    <pre>{status.latestRelease.body || "本版本未填写更新日志。"}</pre>
                                </div>
                            ) : null}
                        </div>
                    </SettingsSectionCard>

                    <SettingsSectionCard layout="stacked" icon={<ShieldCheck className="size-4" />} title="更新前检查" description="所有阻断项通过后才允许切换服务。" status={{ label: status?.connected ? "更新器已连接" : "更新器未连接", color: status?.connected ? "success" : "error" }}>
                        <div className="admin-system-update-checks">
                            {(status?.checks || []).map((check) => <UpdateCheckRow key={check.key} check={check} />)}
                        </div>
                    </SettingsSectionCard>
                </div>

                <SettingsSectionCard layout="stacked" icon={<History className="size-4" />} title="更新进度" description="操作状态和日志由宿主机持久化，服务重启后仍可读取。" status={<PhaseBadge phase={status?.operation.phase ?? "idle"} />}>
                    <div className="admin-system-update-progress">
                        {status?.operation.error ? <UpdateAlert tone="error" title="本次操作失败" detail={status.operation.error} compact /> : null}
                        {(status?.operation.logs || []).length ? (
                            <ol>
                                {status?.operation.logs.map((entry, index) => (
                                    <li key={`${entry.at}-${index}`}><span className="admin-system-update-dot" data-active={index === status.operation.logs.length - 1} /><div><strong>{phaseLabels[entry.phase]}</strong><p>{entry.message}</p></div><time>{formatTime(entry.at)}</time></li>
                                ))}
                            </ol>
                        ) : <p className="admin-system-update-empty">检查更新后，这里会显示完整操作记录。</p>}
                    </div>
                </SettingsSectionCard>

                <SettingsSectionCard
                    layout="stacked"
                    icon={<Archive className="size-4" />}
                    title="备份与回退"
                    description="只使用最近一次通过 ZIP 结构与 SHA-256 校验的备份。"
                    status={{ label: status?.lastBackup ? "备份已校验" : "暂无备份", color: status?.lastBackup ? "success" : "warning" }}
                    footer={
                        <>
                            <span className="text-xs text-foreground/50">人工回退同样会执行停服、数据库恢复、旧镜像启动和健康验证。</span>
                            <Button danger icon={<RotateCcw className="size-4" />} disabled={!status?.rollbackVersion || operationActive || !status?.connected} onClick={requestRollback}>回退到上一版本</Button>
                        </>
                    }
                >
                    <div className="admin-system-update-backup">
                        {status?.lastBackup ? <dl><dt>备份编号</dt><dd>{status.lastBackup.id}</dd><dt>来源版本</dt><dd>{status.lastBackup.version}</dd><dt>创建时间</dt><dd>{formatDate(status.lastBackup.createdAt)}</dd><dt>文件大小</dt><dd>{formatBytes(status.lastBackup.size)}</dd><dt>SHA-256</dt><dd className="admin-monospace">{status.lastBackup.checksum}</dd></dl> : <p>首次开始更新时会自动创建数据库与数据目录 ZIP；备份失败将直接中止更新。</p>}
                    </div>
                </SettingsSectionCard>
            </div>
        </AdminPageFrame>
    );
}

function PhaseBadge({ phase }: { phase: UpdatePhase }) {
    const tone = phase === "succeeded" || phase === "ready" ? "success" : phase === "failed" || phase === "manual_intervention" ? "error" : phase === "rolling_back" || phase === "rolled_back" ? "warning" : activePhases.has(phase) ? "info" : "neutral";
    return <AdminStatusBadge label={phaseLabels[phase]} tone={tone} />;
}

function UpdateCheckRow({ check }: { check: SystemUpdateStatus["checks"][number] }) {
    const Icon = check.status === "passed" ? BadgeCheck : check.status === "failed" ? AlertTriangle : Circle;
    return <div className="admin-system-update-check-row" data-status={check.status}><Icon className="size-4" /><div><strong>{check.label}</strong><p>{check.detail || "等待执行"}</p></div><span>{check.status === "passed" ? "通过" : check.status === "failed" ? "失败" : "待执行"}</span></div>;
}

function UpdateAlert({ tone, title, detail, compact = false }: { tone: "warning" | "error"; title: string; detail: string; compact?: boolean }) {
    return <div className={`admin-system-update-alert${compact ? " is-compact" : ""}`} data-tone={tone} role="alert"><AlertTriangle className="size-4" /><div><strong>{title}</strong><p>{detail}</p></div></div>;
}

function formatDate(value?: string) {
    if (!value) return "—";
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? "—" : date.toLocaleString("zh-CN", { hour12: false });
}

function formatTime(value: string) {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? "—" : date.toLocaleTimeString("zh-CN", { hour12: false });
}

function formatBytes(value: number) {
    if (!Number.isFinite(value) || value <= 0) return "—";
    const units = ["B", "KB", "MB", "GB", "TB"];
    const index = Math.min(Math.floor(Math.log(value) / Math.log(1024)), units.length - 1);
    return `${(value / 1024 ** index).toFixed(index > 1 ? 2 : 0)} ${units[index]}`;
}

function isReleaseVersion(value?: string) {
    return Boolean(value && /^v?\d+\.\d+\.\d+/.test(value));
}
