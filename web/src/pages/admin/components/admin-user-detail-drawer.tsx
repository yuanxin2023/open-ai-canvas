import { useEffect, useState } from "react";
import { App, Button, DatePicker, Descriptions, Input, Progress, Select, Skeleton, Tabs } from "antd";
import { AdminModal } from "@/pages/admin/ui/overlays";
import { ChevronLeft, ChevronRight, Search } from "lucide-react";
import type { Dayjs } from "dayjs";

import { formatCredits } from "@/constant/credits";
import { IconButton } from "@/pages/admin/ui/controls";
import { AdminDataTable, AdminEmpty, AdminStatusBadge, AdminTableEmpty, PaginationBar, type AdminStatusTone } from "./admin-ui";
import { getAdminUserDetail, listAdminUserAuditEvents, listAdminUserLedger, listAdminUserLoginEvents, listAdminUserTasks, type AdminAuditEvent, type AdminUserDetail, type AdminUserLedgerFilter, type AdminUserLoginEvent, type AdminUserLoginEventQuery, type AdminUserTask } from "@/services/api/auth";
import type { CreditLedgerEntry } from "@/services/api/wallet";

type LoginEventFilters = Pick<AdminUserLoginEventQuery, "startAt" | "endAt" | "loginMethod" | "ip">;

export function AdminUserDetailModal({ userId, onClose, previousUserId, nextUserId, onNavigate, accountKind = "user" }: { userId: string | null; onClose: () => void; previousUserId?: string; nextUserId?: string; onNavigate?: (userId: string) => void; accountKind?: "user" | "administrator" }) {
    const { message } = App.useApp();
    const [detail, setDetail] = useState<AdminUserDetail | null>(null);
    const [ledger, setLedger] = useState<CreditLedgerEntry[]>([]);
    const [tasks, setTasks] = useState<AdminUserTask[]>([]);
    const [events, setEvents] = useState<AdminAuditEvent[]>([]);
    const [loginEvents, setLoginEvents] = useState<AdminUserLoginEvent[]>([]);
    const [loading, setLoading] = useState(false);
    const [ledgerLoading, setLedgerLoading] = useState(false);
    const [loginLoading, setLoginLoading] = useState(false);
    const [ledgerFilter, setLedgerFilter] = useState<AdminUserLedgerFilter>("all");
    const [ledgerPage, setLedgerPage] = useState(1);
    const [ledgerTotal, setLedgerTotal] = useState(0);
    const [taskPage, setTaskPage] = useState(1);
    const [taskTotal, setTaskTotal] = useState(0);
    const [auditPage, setAuditPage] = useState(1);
    const [auditTotal, setAuditTotal] = useState(0);
    const [loginPage, setLoginPage] = useState(1);
    const [loginPageSize, setLoginPageSize] = useState(20);
    const [loginTotal, setLoginTotal] = useState(0);
    const [loginDateRange, setLoginDateRange] = useState<[Dayjs | null, Dayjs | null] | null>(null);
    const [loginMethod, setLoginMethod] = useState("all");
    const [loginIP, setLoginIP] = useState("");
    const [loginFilters, setLoginFilters] = useState<LoginEventFilters>({});
    const [expandedLoginEventId, setExpandedLoginEventId] = useState("");

    useEffect(() => {
        if (!userId) return;
        let active = true;
        setLoading(true);
        setDetail(null);
        setLedgerFilter("all");
        setLedgerPage(1);
        setTaskPage(1);
        setAuditPage(1);
        setLoginPage(1);
        setLoginPageSize(20);
        setLoginDateRange(null);
        setLoginMethod("all");
        setLoginIP("");
        setLoginFilters({});
        setExpandedLoginEventId("");
        void getAdminUserDetail(userId)
            .then((nextDetail) => {
                if (active) setDetail(nextDetail);
            })
            .catch((error) => active && message.error(error instanceof Error ? error.message : "读取用户详情失败"))
            .finally(() => active && setLoading(false));
        return () => {
            active = false;
        };
    }, [message, userId]);

    useEffect(() => {
        if (!userId) return;
        let active = true;
        setLedgerLoading(true);
        void listAdminUserLedger(userId, { page: ledgerPage, pageSize: 20, type: ledgerFilter })
            .then((result) => {
                if (active) {
                    setLedger(result.entries);
                    setLedgerTotal(result.total);
                }
            })
            .catch((error) => {
                if (!active) return;
                setLedger([]);
                setLedgerTotal(0);
                message.error(error instanceof Error ? error.message : "读取积分流水失败");
            })
            .finally(() => active && setLedgerLoading(false));
        return () => {
            active = false;
        };
    }, [ledgerFilter, ledgerPage, message, userId]);
    useEffect(() => {
        if (!userId) return;
        let active = true;
        void listAdminUserTasks(userId, { page: taskPage, pageSize: 20 })
            .then((result) => {
                if (active) {
                    setTasks(result.tasks);
                    setTaskTotal(result.total);
                }
            })
            .catch((error) => active && message.error(error instanceof Error ? error.message : "读取任务记录失败"));
        return () => {
            active = false;
        };
    }, [message, taskPage, userId]);
    useEffect(() => {
        if (!userId) return;
        const controller = new AbortController();
        setLoginLoading(true);
        void listAdminUserLoginEvents(userId, { page: loginPage, pageSize: loginPageSize, ...loginFilters }, controller.signal)
            .then((result) => {
                if (controller.signal.aborted) return;
                setLoginEvents(result.events);
                setLoginTotal(result.total);
                if (result.total > 0 && result.events.length === 0 && loginPage > 1) setLoginPage(1);
            })
            .catch((error) => {
                if (error instanceof DOMException && error.name === "AbortError") return;
                setLoginEvents([]);
                setLoginTotal(0);
                message.error(error instanceof Error ? error.message : "读取登录环境失败");
            })
            .finally(() => {
                if (!controller.signal.aborted) setLoginLoading(false);
            });
        return () => controller.abort();
    }, [loginFilters, loginPage, loginPageSize, message, userId]);
    useEffect(() => {
        if (!userId) return;
        let active = true;
        void listAdminUserAuditEvents(userId, { page: auditPage, pageSize: 20 })
            .then((result) => {
                if (active) {
                    setEvents(result.events);
                    setAuditTotal(result.total);
                }
            })
            .catch((error) => active && message.error(error instanceof Error ? error.message : "读取管理操作失败"));
        return () => {
            active = false;
        };
    }, [auditPage, message, userId]);

    const draftLoginFilters = loginEventFilters(loginDateRange, loginMethod, loginIP);
    const loginFiltersActive = hasLoginEventFilters(loginFilters);
    const loginFiltersDirty = !sameLoginEventFilters(draftLoginFilters, loginFilters);

    const applyLoginFilters = () => {
        setLoginFilters(draftLoginFilters);
        setLoginPage(1);
        setExpandedLoginEventId("");
    };

    const resetLoginFilters = () => {
        setLoginDateRange(null);
        setLoginMethod("all");
        setLoginIP("");
        setLoginFilters({});
        setLoginPage(1);
        setExpandedLoginEventId("");
    };

    return (
        <AdminModal
            title={(
                <div className="flex items-center justify-between gap-4">
                    <span className="min-w-0 truncate">{detail ? `${detail.user.username} · ${accountKind === "administrator" ? "管理员详情" : "用户详情"}` : (accountKind === "administrator" ? "管理员详情" : "用户详情")}</span>
                    {onNavigate ? (
                        <div className="flex shrink-0 items-center gap-1">
                            <IconButton size="sm" variant="ghost" aria-label={`上一条${accountKind === "administrator" ? "管理员" : "用户"}`} disabled={!previousUserId} icon={ChevronLeft} onClick={() => previousUserId && onNavigate(previousUserId)} />
                            <IconButton size="sm" variant="ghost" aria-label={`下一条${accountKind === "administrator" ? "管理员" : "用户"}`} disabled={!nextUserId} icon={ChevronRight} onClick={() => nextUserId && onNavigate(nextUserId)} />
                        </div>
                    ) : null}
                </div>
            )}
            open={Boolean(userId)}
            centered
            width="min(920px, calc(100vw - 32px))"
            onCancel={onClose}
            footer={null}
            styles={{ body: { maxHeight: "calc(100vh - 160px)", overflowY: "auto", paddingBottom: 20 } }}
        >
            {loading && !detail ? (
                <Skeleton active paragraph={{ rows: 10 }} />
            ) : detail ? (
                <Tabs
                    items={[
                        {
                            key: "overview",
                            label: "账号概览",
                            children: (
                                <div className="space-y-5">
                                    <Descriptions
                                        bordered
                                        size="small"
                                        column={{ xs: 1, sm: 2 }}
                                        items={[
                                            { key: "id", label: "用户 ID", children: <span className="break-all font-mono text-xs">{detail.user.id}</span> },
                                            { key: "registrationIp", label: "注册 IP", children: <span className="font-mono text-xs">{detail.registrationIp || "未记录"}</span> },
                                            { key: "username", label: "用户名", children: `@${detail.user.username}` },
                                            { key: "email", label: "邮箱", children: detail.user.email || "未填写" },
                                            { key: "remark", label: "备注", span: 2, children: <span className="whitespace-pre-wrap break-words">{detail.user.remark || "未备注"}</span> },
                                            { key: "role", label: "角色", children: detail.user.role === "admin" ? "管理员" : "普通用户" },
                                            { key: "status", label: "状态", children: <AdminStatusBadge label={detail.user.status === "active" ? "启用" : "停用"} tone={detail.user.status === "active" ? "success" : "neutral"} /> },
                                            { key: "available", label: "可用积分", children: formatCredits(detail.account.availableMicrocredits) },
                                            { key: "reserved", label: "冻结积分", children: formatCredits(detail.account.reservedMicrocredits) },
                                            { key: "created", label: "注册时间", children: formatTime(detail.user.createdAt) },
                                            { key: "login", label: "最后登录", children: formatTime(detail.user.lastLoginAt) },
                                        ]}
                                    />
                                    <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                                        {Object.entries({ 积分流水: detail.counts.ledgerEntries, 生成任务: detail.counts.tasks, 上游请求: detail.counts.apiCalls, 登录记录: detail.counts.loginEvents, 管理操作: detail.counts.auditEvents }).map(([label, value]) => (
                                            <div key={label} className="rounded-md border border-border p-3">
                                                <div className="text-xs text-foreground/50">{label}</div>
                                                <div className="mt-1 text-xl font-semibold tabular-nums">{value}</div>
                                            </div>
                                        ))}
                                    </div>
                                    <div>
                                        <div className="mb-3 text-sm font-medium">资源与配额占用</div>
                                        <div className="grid gap-x-6 gap-y-4 sm:grid-cols-2">
                                            {quotaUsageItems(detail).map((item) => (
                                                <div key={item.label}>
                                                    <div className="mb-1 flex items-center justify-between gap-3 text-xs">
                                                        <span className="text-foreground/60">{item.label}</span>
                                                        <span className="shrink-0 tabular-nums text-foreground/75">{item.display}</span>
                                                    </div>
                                                    <Progress percent={Math.min(100, item.limit > 0 ? Math.round(item.value / item.limit * 100) : 0)} size="small" showInfo={false} status={item.value >= item.limit ? "exception" : "normal"} />
                                                </div>
                                            ))}
                                        </div>
                                    </div>
                                </div>
                            ),
                        },
                        {
                            key: "ledger",
                            label: `积分流水 ${detail.counts.ledgerEntries}`,
                            children: (
                                <AdminDataTable
                                    toolbar={(
                                        <Select<AdminUserLedgerFilter>
                                            aria-label="筛选积分流水"
                                            className="w-40"
                                            value={ledgerFilter}
                                            options={[
                                                { label: "全部流水", value: "all" },
                                                { label: "增加积分", value: "increase" },
                                                { label: "消耗积分", value: "consume" },
                                                { label: "管理调整", value: "admin" },
                                            ]}
                                            onChange={(value) => {
                                                setLedgerFilter(value);
                                                setLedgerPage(1);
                                            }}
                                        />
                                    )}
                                    table={{
                                        rowKey: "id",
                                        size: "small",
                                        loading: ledgerLoading,
                                        dataSource: ledger,
                                        pagination: false,
                                        columns: [
                                        { title: "时间", dataIndex: "createdAt", width: 170, render: formatTime },
                                        { title: "类型", dataIndex: "type", width: 130 },
                                        { title: "变化", dataIndex: "amountMicrocredits", width: 120, align: "right", render: (value) => formatCredits(value) },
                                        { title: "说明", dataIndex: "note", ellipsis: true },
                                        ],
                                        scroll: { x: 720 },
                                    }}
                                    empty={<AdminTableEmpty filtered={ledgerFilter !== "all"} />}
                                    footer={<PaginationBar alwaysShow current={ledgerPage} pageSize={20} total={ledgerTotal} onChange={(page) => setLedgerPage(page)} pageSizeOptions={[20]} />}
                                />
                            ),
                        },
                        {
                            key: "tasks",
                            label: `生成任务 ${detail.counts.tasks}`,
                            children: (
                                <AdminDataTable
                                    table={{
                                        rowKey: "id",
                                        size: "small",
                                        dataSource: tasks,
                                        pagination: false,
                                        columns: [
                                        { title: "时间", dataIndex: "createdAt", width: 170, render: formatTime },
                                        { title: "类型", dataIndex: "type", width: 180 },
                                        { title: "模型", dataIndex: "model", width: 180, ellipsis: true },
                                        { title: "状态", dataIndex: "status", width: 100, render: (value) => <AdminStatusBadge label={value || "未知"} tone={taskStatusTone(value)} /> },
                                        { title: "阶段", dataIndex: "stage", ellipsis: true },
                                        ],
                                        scroll: { x: 820 },
                                    }}
                                    empty={<AdminTableEmpty />}
                                    footer={<PaginationBar alwaysShow current={taskPage} pageSize={20} total={taskTotal} onChange={(page) => setTaskPage(page)} pageSizeOptions={[20]} />}
                                />
                            ),
                        },
                        {
                            key: "login-environment",
                            label: `登录环境 ${detail.counts.loginEvents}`,
                            children: (
                                <AdminDataTable
                                    toolbar={(
                                        <Input
                                            allowClear
                                            className="app-list-search"
                                            prefix={<Search className="size-4 text-foreground/40" />}
                                            value={loginIP}
                                            placeholder="搜索 IP 地址"
                                            onChange={(event) => setLoginIP(event.target.value)}
                                            onPressEnter={applyLoginFilters}
                                        />
                                    )}
                                    toolbarActive={loginFiltersActive}
                                    toolbarFilters={(
                                        <>
                                            <Select
                                                aria-label="登录方式"
                                                className="w-36"
                                                value={loginMethod}
                                                options={[
                                                    { label: "全部登录方式", value: "all" },
                                                    { label: "邮箱注册", value: "email_register" },
                                                    { label: "密码登录", value: "password" },
                                                    { label: "Linux.do", value: "linuxdo" },
                                                ]}
                                                onChange={setLoginMethod}
                                            />
                                            <DatePicker.RangePicker
                                                showTime
                                                value={loginDateRange}
                                                format="YYYY-MM-DD HH:mm:ss"
                                                placeholder={["开始时间", "结束时间"]}
                                                onChange={setLoginDateRange}
                                            />
                                        </>
                                    )}
                                    trailing={(
                                        <div className="flex items-center gap-2">
                                            {loginFiltersDirty ? <span className="text-xs text-foreground/45">请先查询以应用筛选条件</span> : null}
                                            <Button type="text" disabled={!loginFiltersActive && !loginFiltersDirty} onClick={resetLoginFilters}>重置</Button>
                                            <Button loading={loginLoading} onClick={applyLoginFilters}>查询</Button>
                                        </div>
                                    )}
                                    table={{
                                        rowKey: "id",
                                        size: "small",
                                        loading: loginLoading,
                                        dataSource: loginEvents,
                                        pagination: false,
                                        tableLayout: "fixed",
                                        columns: [
                                            { title: "登录时间", dataIndex: "createdAt", width: 170, render: formatTime },
                                            { title: "登录方式", dataIndex: "loginMethod", width: 105, render: loginMethodLabel },
                                            { title: "IP 地址", dataIndex: "ipAddress", width: 145, ellipsis: true, render: (value) => <span className="font-mono text-xs" title={fallbackText(value)}>{fallbackText(value)}</span> },
                                            { title: "设备", dataIndex: "deviceType", width: 85, render: fallbackText },
                                            { title: "操作系统", width: 145, ellipsis: true, render: (_, event) => environmentName(event.os, event.osVersion) },
                                            { title: "浏览器", width: 150, ellipsis: true, render: (_, event) => environmentName(event.browser, event.browserVersion) },
                                            {
                                                title: "操作",
                                                width: 100,
                                                fixed: "right",
                                                render: (_, event) => (
                                                    <Button type="link" size="small" onClick={() => setExpandedLoginEventId((current) => current === event.id ? "" : event.id)}>
                                                        {expandedLoginEventId === event.id ? "收起" : "查看详情"}
                                                    </Button>
                                                ),
                                            },
                                        ],
                                        expandable: {
                                            showExpandColumn: false,
                                            expandedRowKeys: expandedLoginEventId ? [expandedLoginEventId] : [],
                                            expandedRowRender: (event) => <LoginEventDetails event={event} />,
                                            onExpand: (expanded, event) => setExpandedLoginEventId(expanded ? event.id : ""),
                                        },
                                        scroll: { x: 900 },
                                    }}
                                    empty={<AdminTableEmpty filtered={loginFiltersActive} title="没有登录环境记录" />}
                                    footer={(
                                        <PaginationBar
                                            alwaysShow
                                            current={loginPage}
                                            pageSize={loginPageSize}
                                            total={loginTotal}
                                            onChange={(page, pageSize) => {
                                                setLoginPage(pageSize !== loginPageSize ? 1 : page);
                                                setLoginPageSize(pageSize);
                                                setExpandedLoginEventId("");
                                            }}
                                        />
                                    )}
                                />
                            ),
                        },
                        {
                            key: "audit",
                            label: `管理操作 ${detail.counts.auditEvents}`,
                            children: (
                                <AdminDataTable
                                    table={{
                                        rowKey: "id",
                                        size: "small",
                                        dataSource: events,
                                        pagination: false,
                                        columns: [
                                        { title: "时间", dataIndex: "createdAt", width: 170, render: formatTime },
                                        { title: "管理员", dataIndex: "actorUserId", width: 160, ellipsis: true },
                                        { title: "动作", dataIndex: "action", width: 160 },
                                        { title: "摘要", dataIndex: "summary", ellipsis: true },
                                        ],
                                        scroll: { x: 720 },
                                    }}
                                    empty={<AdminTableEmpty />}
                                    footer={<PaginationBar alwaysShow current={auditPage} pageSize={20} total={auditTotal} onChange={(page) => setAuditPage(page)} pageSizeOptions={[20]} />}
                                />
                            ),
                        },
                    ]}
                />
            ) : (
                <AdminEmpty size="compact" title="没有用户详情" />
            )}
        </AdminModal>
    );
}

function LoginEventDetails({ event }: { event: AdminUserLoginEvent }) {
    return (
        <Descriptions
            bordered
            size="small"
            column={{ xs: 1, sm: 2 }}
            items={[
                { key: "createdAt", label: "登录时间", children: formatTime(event.createdAt) },
                { key: "loginMethod", label: "登录方式", children: loginMethodLabel(event.loginMethod) },
                { key: "ipAddress", label: "IP 地址", children: <span className="font-mono text-xs">{fallbackText(event.ipAddress)}</span> },
                { key: "deviceType", label: "设备", children: fallbackText(event.deviceType) },
                { key: "os", label: "操作系统", children: environmentName(event.os, event.osVersion) },
                { key: "browser", label: "浏览器", children: environmentName(event.browser, event.browserVersion) },
                { key: "userAgent", label: "User-Agent", span: 2, children: <span className="break-all font-mono text-xs">{fallbackText(event.userAgent)}</span> },
            ]}
        />
    );
}

function loginEventFilters(dateRange: [Dayjs | null, Dayjs | null] | null, method: string, ip: string): LoginEventFilters {
    return {
        startAt: dateRange?.[0]?.toISOString(),
        endAt: dateRange?.[1]?.toISOString(),
        loginMethod: method === "all" ? undefined : method,
        ip: ip.trim() || undefined,
    };
}

function hasLoginEventFilters(filters: LoginEventFilters) {
    return Boolean(filters.startAt || filters.endAt || filters.loginMethod || filters.ip);
}

function sameLoginEventFilters(left: LoginEventFilters, right: LoginEventFilters) {
    return left.startAt === right.startAt && left.endAt === right.endAt && left.loginMethod === right.loginMethod && left.ip === right.ip;
}

function formatTime(value?: string) {
    return value ? new Date(value).toLocaleString("zh-CN", { hour12: false }) : "--";
}

function fallbackText(value?: string) {
    return value || "--";
}

function environmentName(name?: string, version?: string) {
    return [name, version].filter(Boolean).join(" ") || "--";
}

function loginMethodLabel(value?: string) {
    return ({ email_register: "邮箱注册", password: "密码登录", linuxdo: "Linux.do" } as Record<string, string>)[value || ""] || value || "--";
}

function taskStatusTone(value?: string): AdminStatusTone {
    const normalized = String(value || "").toLowerCase();
    if (["completed", "succeeded", "success", "done"].includes(normalized)) return "success";
    if (["failed", "error", "cancelled", "canceled"].includes(normalized)) return "error";
    if (["running", "processing", "pending", "queued"].includes(normalized)) return "warning";
    return "neutral";
}

function quotaUsageItems(detail: AdminUserDetail) {
    const structuredBytes = detail.storageUsage.assetBytes + detail.storageUsage.canvasBytes;
    const bytes = (value: number) => value >= 1024 ** 3 ? `${(value / 1024 ** 3).toFixed(2)} GB` : `${(value / 1024 ** 2).toFixed(1)} MB`;
    const number = (value: number) => new Intl.NumberFormat("zh-CN").format(value);
    return [
        { label: "资源与附件", value: detail.storedFileBytes, limit: detail.quota.storedFileGB * 1024 ** 3, display: `${bytes(detail.storedFileBytes)} / ${detail.quota.storedFileGB} GB` },
        { label: "今日上传（UTC）", value: detail.dailyUploadBytes, limit: detail.quota.dailyUploadMB * 1024 ** 2, display: `${bytes(detail.dailyUploadBytes)} / ${detail.quota.dailyUploadMB} MB` },
        { label: "画布、素材与会话数据", value: structuredBytes, limit: detail.quota.structuredDataMB * 1024 ** 2, display: `${bytes(structuredBytes)} / ${detail.quota.structuredDataMB} MB` },
        { label: "任务与请求日志数据", value: detail.storageUsage.taskBytes, limit: detail.quota.taskDataGB * 1024 ** 3, display: `${bytes(detail.storageUsage.taskBytes)} / ${detail.quota.taskDataGB} GB` },
        { label: "素材数量", value: detail.storageUsage.assetCount, limit: detail.quota.assetCount, display: `${number(detail.storageUsage.assetCount)} / ${number(detail.quota.assetCount)}` },
        { label: "画布数量", value: detail.storageUsage.canvasCount, limit: detail.quota.canvasCount, display: `${number(detail.storageUsage.canvasCount)} / ${number(detail.quota.canvasCount)}` },
        { label: "任务历史数量", value: detail.storageUsage.taskCount, limit: detail.quota.taskCount, display: `${number(detail.storageUsage.taskCount)} / ${number(detail.quota.taskCount)}` },
        { label: "上游请求日志数量", value: detail.storageUsage.apiCallCount, limit: detail.quota.apiCallLogCount, display: `${number(detail.storageUsage.apiCallCount)} / ${number(detail.quota.apiCallLogCount)}` },
    ];
}
