import { App, Button, Select, Table, Typography } from "antd";
import type { ColumnsType } from "antd/es/table";
import dayjs from "dayjs";
import { RefreshCw, X } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";

import { PaginationBar, TableSurface } from "@/components/layout/workspace-page";
import { StatusBadge } from "@/components/ui/base/badges";
import {
    closePaymentOrder,
    listPaymentOrders,
    type PaymentOrder,
    type PaymentOrderFilter,
    type PaymentOrderStatus,
} from "@/services/api/payments";

const filterOptions: Array<{ label: string; value: PaymentOrderFilter }> = [
    { label: "全部", value: "all" },
    { label: "待支付", value: "unpaid" },
    { label: "已完成", value: "completed" },
    { label: "失败", value: "failed" },
    { label: "已关闭", value: "closed" },
];

const statusMeta: Record<PaymentOrderStatus, { label: string; tone: "neutral" | "success" | "warning" | "error" | "loading" }> = {
    created: { label: "创建中", tone: "loading" },
    pending: { label: "待支付", tone: "warning" },
    closing: { label: "关闭中", tone: "loading" },
    closed: { label: "已关闭", tone: "neutral" },
    credited: { label: "已完成", tone: "success" },
    create_failed: { label: "失败", tone: "error" },
};

const cancellableStatuses = new Set<PaymentOrderStatus>(["created", "pending", "closing", "create_failed"]);

export function PaymentOrdersPane({ onOpenWallet }: { onOpenWallet: () => void }) {
    const { message, modal } = App.useApp();
    const [orders, setOrders] = useState<PaymentOrder[]>([]);
    const [status, setStatus] = useState<PaymentOrderFilter>("all");
    const [page, setPage] = useState(1);
    const [pageSize, setPageSize] = useState(20);
    const [total, setTotal] = useState(0);
    const [loading, setLoading] = useState(false);
    const [closingId, setClosingId] = useState("");

    const loadOrders = useCallback(async (nextPage = page, nextPageSize = pageSize, nextStatus = status) => {
        setLoading(true);
        try {
            const result = await listPaymentOrders({ status: nextStatus, page: nextPage, pageSize: nextPageSize });
            setOrders(result.orders);
            setTotal(result.total);
            setPage(result.page);
            setPageSize(result.pageSize);
        } catch (error) {
            message.error(error instanceof Error ? error.message : "读取订单失败");
        } finally {
            setLoading(false);
        }
    }, [message, page, pageSize, status]);

    useEffect(() => {
        void loadOrders();
    }, []);

    const cancelOrder = (order: PaymentOrder) => {
        modal.confirm({
            title: "取消待支付订单？",
            content: "系统会先查询支付渠道状态。若已经支付将正常入账，否则关闭该订单。",
            okText: "查单并取消",
            cancelText: "返回",
            okButtonProps: { danger: true },
            onOk: async () => {
                setClosingId(order.id);
                try {
                    const result = await closePaymentOrder(order.id);
                    message.success(result.order.status === "credited" ? "支付已确认，积分已到账" : "订单已取消");
                    await loadOrders();
                } catch (error) {
                    message.error(error instanceof Error ? error.message : "取消订单失败");
                    throw error;
                } finally {
                    setClosingId("");
                }
            },
        });
    };

    const columns = useMemo<ColumnsType<PaymentOrder>>(() => [
        {
            title: "订单 ID",
            dataIndex: "id",
            width: 130,
            render: (value: string) => <Typography.Text className="font-mono text-xs" title={value}>#{value.slice(0, 8)}</Typography.Text>,
        },
        {
            title: "订单编号",
            dataIndex: "merchantOrderNo",
            width: 260,
            render: (value: string) => <Typography.Text copyable={{ text: value }} className="font-mono text-xs">{value}</Typography.Text>,
        },
        {
            title: "实付",
            dataIndex: "amountFen",
            width: 120,
            render: (value: number) => <span className="font-medium tabular-nums">¥{(value / 100).toFixed(2)}</span>,
        },
        {
            title: "支付方式",
            dataIndex: "providerId",
            width: 140,
            render: (value: string) => paymentMethodLabel(value),
        },
        {
            title: "状态",
            dataIndex: "status",
            width: 120,
            render: (value: PaymentOrderStatus) => <StatusBadge {...statusMeta[value]} />,
        },
        {
            title: "创建时间",
            dataIndex: "createdAt",
            width: 190,
            render: (value: string) => dayjs(value).format("YYYY/MM/DD HH:mm:ss"),
        },
        {
            title: "操作",
            key: "actions",
            width: 130,
            fixed: "right",
            render: (_, order) => cancellableStatuses.has(order.status) ? (
                <Button type="link" danger size="small" icon={<X className="size-3.5" />} loading={closingId === order.id} disabled={Boolean(closingId) && closingId !== order.id} onClick={() => cancelOrder(order)}>
                    取消订单
                </Button>
            ) : <span className="text-xs text-foreground/35">—</span>,
        },
    ], [closingId, loadOrders]);

    return (
        <div className="min-w-0">
            <div className="settings-pane-header">
                <div className="min-w-0">
                    <h2>我的订单</h2>
                    <p>查看充值订单状态，或取消尚未完成支付的订单。</p>
                </div>
            </div>

            <div className="app-workspace-surface flex min-h-14 flex-wrap items-center justify-between gap-3 rounded-lg p-3">
                <Select<PaymentOrderFilter>
                    aria-label="订单状态"
                    value={status}
                    options={filterOptions}
                    className="w-36"
                    onChange={(value) => {
                        setStatus(value);
                        setPage(1);
                        void loadOrders(1, pageSize, value);
                    }}
                />
                <div className="flex items-center gap-2">
                    <Button aria-label="刷新订单" title="刷新订单" icon={<RefreshCw className="size-4" />} loading={loading} onClick={() => void loadOrders()} />
                    <Button type="primary" onClick={onOpenWallet}>返回充值</Button>
                </div>
            </div>

            <TableSurface className="mt-4 rounded-lg border-border/70 bg-transparent">
                <Table
                    className="app-data-table"
                    rowKey="id"
                    columns={columns}
                    dataSource={orders}
                    loading={loading}
                    pagination={false}
                    tableLayout="fixed"
                    scroll={{ x: 1090 }}
                    locale={{ emptyText: status === "all" ? "暂无充值订单" : "当前筛选下没有订单" }}
                />
            </TableSurface>
            <PaginationBar
                alwaysShow
                current={page}
                pageSize={pageSize}
                total={total}
                pageSizeOptions={[20, 50, 100]}
                onChange={(nextPage, nextPageSize) => void loadOrders(nextPageSize !== pageSize ? 1 : nextPage, nextPageSize)}
            />
        </div>
    );
}

function paymentMethodLabel(providerId: string) {
    const normalized = providerId.toLowerCase();
    if (normalized.includes("alipay")) return "支付宝";
    if (normalized.includes("wechat")) return "微信支付";
    return providerId;
}
