import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Alert, Button, Input, Skeleton } from "antd";
import type { ColumnsType } from "antd/es/table";
import { ArrowDown, ChevronRight, Info, RefreshCw } from "lucide-react";
import { useState } from "react";

import { useUserStore } from "@/stores/use-user-store";
import {
    getAdminReferralStatistics,
    listAdminReferralInvitees,
    listAdminReferralStatisticsUsers,
    type ReferralInviteeStatistics,
    type ReferralMetrics,
    type ReferralStatisticsQuery,
    type ReferralStatisticsSort,
    type ReferralUserStatistics,
} from "@/services/api/referrals";
import { AdminDataTable, AdminStatTile, AdminTableEmpty, PaginationBar } from "../components/admin-ui";
import { AdminDrawer } from "../ui/overlays";
import "./statistics.css";

const rankings: Array<{ value: ReferralStatisticsSort; label: string }> = [
    { value: "inviteeCount", label: "注册人数" },
    { value: "paidInviteeCount", label: "充值人数" },
    { value: "paymentOrderCount", label: "充值笔数" },
    { value: "paymentAmountFen", label: "充值金额" },
];
const number = (value: number) => value.toLocaleString("zh-CN");
const money = (fen: number) => `¥${(fen / 100).toLocaleString("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
const percent = (metrics: ReferralMetrics) => `${metrics.inviteeCount ? ((metrics.paidInviteeCount / metrics.inviteeCount) * 100).toFixed(2) : "0.00"}%`;
const time = (value: string) => new Date(value).toLocaleString("zh-CN", { hour12: false });

function useStatisticsQuery<T>(key: readonly unknown[], load: (signal: AbortSignal) => Promise<T>) {
    const userId = useUserStore((state) => state.user?.id);
    return useQuery({
        queryKey: ["admin-referral-statistics", userId, ...key],
        queryFn: ({ signal }) => load(signal),
        enabled: Boolean(userId),
        retry: false,
        staleTime: 0,
        gcTime: 0,
        refetchOnWindowFocus: false,
    });
}

function QueryError({ error, onRetry }: { error: Error; onRetry: () => void }) {
    return <Alert type="error" showIcon title="邀请统计加载失败" description={error.message} action={<Button onClick={onRetry}>重试</Button>} />;
}

function UserIdentity({ user }: { user: { username: string; userId: string } }) {
    return (
        <div className="referral-statistics-identity">
            <strong>{user.username}</strong>
            <small>{user.userId}</small>
        </div>
    );
}

function Metrics({ metrics }: { metrics: ReferralMetrics }) {
    return (
        <div className="referral-statistics-metrics">
            <AdminStatTile label="注册人数" value={number(metrics.inviteeCount)} />
            <AdminStatTile label="充值人数" value={number(metrics.paidInviteeCount)} />
            <AdminStatTile label="充值笔数" value={number(metrics.paymentOrderCount)} />
            <AdminStatTile label="充值金额" value={money(metrics.paymentAmountFen)} />
        </div>
    );
}

export function ReferralStatisticsPanel() {
    const userId = useUserStore((state) => state.user?.id);
    const queryClient = useQueryClient();
    const [filters, setFilters] = useState<ReferralStatisticsQuery>({ sortBy: "paidInviteeCount", page: 1, pageSize: 20, keyword: "" });
    const [selected, setSelected] = useState<string>();
    const overview = useStatisticsQuery(["overview"], getAdminReferralStatistics);
    const users = useStatisticsQuery(["users", filters], (signal) => listAdminReferralStatisticsUsers(filters, signal));
    const refresh = () => void queryClient.invalidateQueries({ queryKey: ["admin-referral-statistics", userId] });
    const metricTitle = (key: ReferralStatisticsSort, label: string) => (
        <span className={filters.sortBy === key ? "referral-statistics-sorted" : undefined}>
            {label}
            {filters.sortBy === key && <ArrowDown aria-hidden="true" />}
        </span>
    );
    const columns: ColumnsType<ReferralUserStatistics> = [
        { title: "排名", dataIndex: "rank", width: 65, render: (value: number) => <span className={`referral-statistics-rank${value <= 3 ? " is-top" : ""}`}>{value}</span> },
        { title: "邀请人", width: 190, render: (_, row) => <UserIdentity user={row} /> },
        { title: "推广码", dataIndex: "code", width: 100, render: (value: string) => <span className="referral-statistics-code">{value || "—"}</span> },
        ...rankings.map(({ value, label }) => ({
            title: metricTitle(value, label),
            dataIndex: value,
            width: value === "paymentAmountFen" ? 145 : 110,
            align: "right" as const,
            render: (amount: number) => (value === "paymentAmountFen" ? money(amount) : number(amount)),
        })),
        { title: "充值转化率", width: 110, align: "right", render: (_, row) => percent(row) },
        {
            title: "操作",
            width: 110,
            render: (_, row) => (
                <button type="button" className="referral-statistics-link" onClick={() => setSelected(row.userId)}>
                    查看明细 <ChevronRight aria-hidden="true" />
                </button>
            ),
        },
    ];
    return (
        <section className="referral-statistics">
            <div className="referral-statistics-heading">
                <h2>全站累计概览</h2>
                <span>历史累计 · 仅统计直接邀请</span>
            </div>
            {overview.isError ? (
                <QueryError error={overview.error} onRetry={() => void overview.refetch()} />
            ) : overview.data ? (
                <div className="referral-statistics-overview">
                    <AdminStatTile label="有效邀请人" value={number(overview.data.inviterCount)} detail="至少成功邀请过 1 人" />
                    <AdminStatTile label="邀请注册人数" value={number(overview.data.inviteeCount)} detail="已绑定的直接邀请用户" />
                    <AdminStatTile label="充值人数" value={number(overview.data.paidInviteeCount)} detail="被邀请人中已成功充值" />
                    <AdminStatTile label="充值笔数" value={number(overview.data.paymentOrderCount)} detail="成功入账的付费订单" />
                    <AdminStatTile label="充值金额" value={money(overview.data.paymentAmountFen)} detail="实付人民币累计金额" />
                </div>
            ) : (
                <Skeleton active paragraph={{ rows: 2 }} />
            )}
            <div className="referral-statistics-heading">
                <h2>邀请人排名{users.data && <span className="referral-statistics-count">{number(users.data.total)} 位邀请人</span>}</h2>
                <Button icon={<RefreshCw className="size-4" />} loading={overview.isFetching || users.isFetching} onClick={refresh}>
                    刷新
                </Button>
            </div>
            <div className="referral-statistics-toolbar">
                <Input.Search allowClear placeholder="搜索用户名、用户 ID 或推广码" aria-label="搜索邀请人" maxLength={200} onSearch={(keyword) => setFilters((current) => ({ ...current, keyword: keyword.trim(), page: 1 }))} />
                <div className="referral-statistics-sort">
                    <span>排名依据</span>
                    <div role="group" aria-label="排名依据">
                        {rankings.map(({ value, label }) => (
                            <button type="button" key={value} aria-pressed={filters.sortBy === value} onClick={() => setFilters((current) => ({ ...current, sortBy: value, page: 1 }))}>
                                {label}
                            </button>
                        ))}
                    </div>
                </div>
            </div>
            {users.isError ? (
                <QueryError error={users.error} onRetry={() => void users.refetch()} />
            ) : (
                <AdminDataTable<ReferralUserStatistics>
                    table={{ rowKey: "userId", loading: users.isPending, dataSource: users.data?.users ?? [], columns, pagination: false, scroll: { x: 1160, y: "max(280px, calc(100dvh - 550px))" } }}
                    skeletonColumns={9}
                    empty={
                        <AdminTableEmpty filtered={Boolean(filters.keyword)} title={filters.keyword ? "没有匹配的邀请人" : "暂无邀请数据"} description={filters.keyword ? "尝试其他用户名、用户 ID 或推广码" : "成功绑定邀请关系后，将在这里显示累计数据"} />
                    }
                    footer={
                        users.data && (
                            <PaginationBar
                                alwaysShow
                                current={filters.page}
                                pageSize={filters.pageSize}
                                total={users.data.total}
                                itemLabel="位邀请人"
                                onChange={(page, pageSize) => setFilters((current) => ({ ...current, page: pageSize === current.pageSize ? page : 1, pageSize }))}
                            />
                        )
                    }
                />
            )}
            <p className="referral-statistics-note">
                <Info aria-hidden="true" />
                统计成功入账的人民币积分充值订单，不含兑换码、人工赠送及奖励积分；仅统计当前保留的账号与记录。
            </p>
            <AdminDrawer title="被邀请人明细" open={Boolean(selected)} size="min(780px, 100vw)" rootClassName="referral-statistics-drawer" onClose={() => setSelected(undefined)}>
                {selected && <InviteeDetails key={selected} userId={selected} />}
            </AdminDrawer>
        </section>
    );
}

function InviteeDetails({ userId }: { userId: string }) {
    const [pagination, setPagination] = useState({ page: 1, pageSize: 20 });
    const query = useStatisticsQuery(["invitees", userId, pagination], (signal) => listAdminReferralInvitees(userId, pagination, signal));
    const columns: ColumnsType<ReferralInviteeStatistics> = [
        { title: "用户", width: 190, render: (_, row) => <UserIdentity user={row} /> },
        { title: "注册时间", dataIndex: "joinedAt", width: 165, render: time },
        { title: "充值笔数", dataIndex: "paymentOrderCount", width: 100, align: "right", render: number },
        { title: "充值金额", dataIndex: "paymentAmountFen", width: 135, align: "right", render: money },
    ];
    if (query.isError) return <QueryError error={query.error} onRetry={() => void query.refetch()} />;
    return (
        <div className="referral-statistics-detail">
            {query.data ? (
                <>
                    <div className="referral-statistics-person">
                        <UserIdentity user={query.data.inviter} />
                        <span>
                            推广码：<code>{query.data.inviter.code || "—"}</code>
                        </span>
                    </div>
                    <Metrics metrics={query.data.inviter} />
                    <p className="referral-statistics-note">充值转化率 {percent(query.data.inviter)} · 只统计直接邀请的好友</p>
                </>
            ) : (
                <Skeleton active paragraph={{ rows: 3 }} />
            )}
            <div className="referral-statistics-heading">
                <h2>被邀请用户{query.data && <span className="referral-statistics-count">{number(query.data.total)} 人</span>}</h2>
                <span>按注册时间倒序</span>
            </div>
            <AdminDataTable<ReferralInviteeStatistics>
                table={{ rowKey: "userId", loading: query.isPending, dataSource: query.data?.invitees ?? [], columns, pagination: false, scroll: { x: 610, y: "max(240px, calc(100dvh - 440px))" } }}
                skeletonColumns={4}
                empty={<AdminTableEmpty title="暂无被邀请用户" />}
                footer={
                    query.data && (
                        <PaginationBar
                            alwaysShow
                            pageSize={pagination.pageSize}
                            current={pagination.page}
                            total={query.data.total}
                            itemLabel="位用户"
                            onChange={(page, pageSize) => setPagination((current) => ({ page: current.pageSize === pageSize ? page : 1, pageSize }))}
                        />
                    )
                }
            />
            <p className="referral-statistics-note">
                <Info aria-hidden="true" />
                未充值用户保留在列表中；暂停返利不影响已发生的注册与充值统计。
            </p>
        </div>
    );
}
