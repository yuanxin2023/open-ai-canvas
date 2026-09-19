import { Alert, Button, Select, Table } from "antd";
import type { ColumnsType } from "antd/es/table";
import dayjs from "dayjs";
import { RefreshCw } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { PaginationBar, TableSurface } from "@/components/layout/workspace-page";
import { StatusBadge } from "@/components/ui/base/badges";
import { formatCredits } from "@/constant/credits";
import { getWallet, type CreditLedgerEntry } from "@/services/api/wallet";
import { modelDisplayName, useEffectiveConfig } from "@/stores/use-config-store";

type LedgerFilter = "all" | "income" | "consume" | "refund";

const ledgerFilterOptions: Array<{ label: string; value: LedgerFilter }> = [
    { label: "全部", value: "all" },
    { label: "充值与调整", value: "income" },
    { label: "模型消费", value: "consume" },
    { label: "退款", value: "refund" },
];

const ledgerTypeMeta: Record<CreditLedgerEntry["type"], { label: string; tone: "neutral" | "success" | "warning" | "error" }> = {
    redeem: { label: "兑换充值", tone: "success" },
    payment_topup: { label: "在线充值", tone: "success" },
    admin_grant: { label: "管理员充值", tone: "success" },
    consume: { label: "模型消费", tone: "error" },
    refund: { label: "消费退款", tone: "warning" },
    admin_adjustment: { label: "管理员调账", tone: "neutral" },
    signup_bonus: { label: "注册奖励", tone: "success" },
    checkin_bonus: { label: "签到奖励", tone: "success" },
};

const sceneLabels: Record<string, string> = {
    image: "图片生成",
    text: "文本生成",
    video: "视频生成",
    audio: "音频生成",
    storyboard: "分镜生成",
};

export function CreditLedgerPane() {
    const config = useEffectiveConfig();
    const [entries, setEntries] = useState<CreditLedgerEntry[]>([]);
    const [filter, setFilter] = useState<LedgerFilter>("all");
    const [page, setPage] = useState(1);
    const [pageSize, setPageSize] = useState(20);
    const [total, setTotal] = useState(0);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState("");
    const requestSequence = useRef(0);

    const loadLedger = useCallback(async (targetPage = page, targetPageSize = pageSize, targetFilter = filter) => {
        const sequence = ++requestSequence.current;
        setLoading(true);
        setError("");
        try {
            const result = await getWallet(targetPage, targetPageSize, targetFilter);
            if (sequence !== requestSequence.current) return;
            setEntries(result.entries);
            setTotal(result.total);
            setPage(result.page);
            setPageSize(result.pageSize);
        } catch (loadError) {
            if (sequence === requestSequence.current) {
                setError(loadError instanceof Error ? loadError.message : "读取积分流水失败");
            }
        } finally {
            if (sequence === requestSequence.current) setLoading(false);
        }
    }, [filter, page, pageSize]);

    useEffect(() => {
        void loadLedger();
    }, []);

    const columns = useMemo<ColumnsType<CreditLedgerEntry>>(() => [
        {
            title: "发生时间",
            dataIndex: "createdAt",
            width: 180,
            render: (value: string) => dayjs(value).format("YYYY/MM/DD HH:mm:ss"),
        },
        {
            title: "类型",
            dataIndex: "type",
            width: 130,
            render: (type: CreditLedgerEntry["type"]) => {
                const meta = ledgerTypeMeta[type];
                return <StatusBadge variant="filled" tone={meta.tone} label={meta.label} />;
            },
        },
        {
            title: "明细",
            width: 400,
            ellipsis: true,
            render: (_, entry) => {
                const title = entry.model ? modelDisplayName(config, entry.model) : ledgerTypeMeta[entry.type].label;
                const description = [entry.scene ? sceneLabels[entry.scene] || "其他场景" : "", entry.note].filter(Boolean).join(" · ") || "积分账户变动";
                return (
                    <div className="min-w-0 max-w-full overflow-hidden" title={`${title}\n${description}`}>
                        <div className="truncate font-medium">{title}</div>
                        <div className="mt-1 truncate text-xs text-foreground/50">{description}</div>
                    </div>
                );
            },
        },
        {
            title: "积分变化",
            dataIndex: "amountMicrocredits",
            width: 145,
            align: "right",
            render: (value: number) => (
                <span className={`font-medium tabular-nums ${value > 0 ? "text-status-success" : value < 0 ? "text-status-error" : "text-foreground/60"}`}>
                    {value > 0 ? "+" : ""}{formatCredits(value)}
                </span>
            ),
        },
        {
            title: "变更后余额",
            dataIndex: "availableAfterMicrocredits",
            width: 145,
            align: "right",
            render: (value: number) => <span className="tabular-nums">{formatCredits(value)}</span>,
        },
    ], [config]);

    return (
        <div className="min-w-0">
            <div className="settings-pane-header">
                <div className="min-w-0">
                    <h2>积分流水</h2>
                    <p>查看积分收入、模型消费、退款与账户调整记录。</p>
                </div>
            </div>

            <div className="app-workspace-surface flex min-h-14 flex-wrap items-center justify-between gap-3 rounded-lg p-3">
                <Select<LedgerFilter>
                    aria-label="流水类型"
                    value={filter}
                    options={ledgerFilterOptions}
                    className="w-36"
                    onChange={(value) => {
                        setFilter(value);
                        setPage(1);
                        void loadLedger(1, pageSize, value);
                    }}
                />
                <Button aria-label="刷新积分流水" title="刷新积分流水" icon={<RefreshCw className="size-4" />} loading={loading} onClick={() => void loadLedger()} />
            </div>

            {error ? (
                <Alert
                    className="mt-4"
                    type="error"
                    showIcon
                    message="积分流水加载失败"
                    description={error}
                    action={<Button size="small" onClick={() => void loadLedger()}>重试</Button>}
                />
            ) : null}

            <TableSurface className="mt-4 rounded-lg border-border/70 bg-transparent">
                <Table
                    className="app-data-table"
                    rowKey="id"
                    columns={columns}
                    dataSource={entries}
                    loading={loading}
                    pagination={false}
                    tableLayout="fixed"
                    scroll={{ x: 1000 }}
                    locale={{ emptyText: filter === "all" ? "暂无积分流水" : "当前筛选下没有积分流水" }}
                />
            </TableSurface>
            <PaginationBar
                alwaysShow
                current={page}
                pageSize={pageSize}
                total={total}
                pageSizeOptions={[20, 50, 100]}
                onChange={(nextPage, nextPageSize) => void loadLedger(nextPageSize !== pageSize ? 1 : nextPage, nextPageSize)}
            />
        </div>
    );
}
