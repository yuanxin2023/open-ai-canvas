import { Alert, App, Button, Skeleton, Table, Tabs } from "antd";
import type { ColumnsType } from "antd/es/table";
import { Copy, Gift, ReceiptText, UsersRound } from "lucide-react";
import { useEffect, useState } from "react";

import { TableSurface } from "@/components/layout/workspace-page";
import { StatusBadge } from "@/components/ui/base/badges";
import { EmptyState } from "@/components/ui/product/empty-state";
import { formatCredits } from "@/constant/credits";
import { getReferralDashboard, type ReferralDashboard, type ReferralReward } from "@/services/api/referrals";

type ReferralInvitee = ReferralDashboard["invitees"][number];

const rewardStatus: Record<ReferralReward["status"], { label: string; tone: "warning" | "success" | "error" }> = {
    pending: { label: "待管理员审核", tone: "warning" },
    approved: { label: "已发放", tone: "success" },
    rejected: { label: "已拒绝", tone: "error" },
};

const inviteeColumns: ColumnsType<ReferralInvitee> = [
    { title: "被邀请用户", dataIndex: "email", ellipsis: true },
    { title: "注册时间", dataIndex: "joinedAt", width: 190, render: (value: string) => new Date(value).toLocaleString("zh-CN") },
];

const rewardColumns: ColumnsType<ReferralReward> = [
    { title: "充值订单", dataIndex: "paymentOrderId", ellipsis: true },
    { title: "返利积分", dataIndex: "rewardMicrocredits", width: 130, align: "right", render: (value: number) => <span className="tabular-nums">{formatCredits(value)}</span> },
    { title: "状态", dataIndex: "status", width: 150, render: (value: ReferralReward["status"]) => <StatusBadge {...rewardStatus[value]} /> },
    { title: "时间", dataIndex: "createdAt", width: 190, render: (value: string) => new Date(value).toLocaleString("zh-CN") },
];

export function ReferralsPane() {
    const { message } = App.useApp();
    const [dashboard, setDashboard] = useState<ReferralDashboard | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState("");
    const [requestVersion, setRequestVersion] = useState(0);
    const [activeTab, setActiveTab] = useState("invitees");

    useEffect(() => {
        let cancelled = false;
        setLoading(true);
        setError("");
        void getReferralDashboard()
            .then((value) => {
                if (!cancelled) setDashboard(value);
            })
            .catch((reason) => {
                if (cancelled) return;
                setDashboard(null);
                setError(reason instanceof Error ? reason.message : "读取邀请返利失败");
            })
            .finally(() => {
                if (!cancelled) setLoading(false);
            });
        return () => {
            cancelled = true;
        };
    }, [requestVersion]);

    const copyText = async (value: string, successMessage: string) => {
        try {
            await navigator.clipboard.writeText(value);
            message.success(successMessage);
        } catch {
            message.error("复制失败，请检查浏览器剪贴板权限");
        }
    };

    const copyLink = () => {
        if (!dashboard?.enabled) return;
        void copyText(`${window.location.origin}/register?ref=${encodeURIComponent(dashboard.code)}`, "邀请链接已复制");
    };

    const invitees = dashboard?.invitees ?? [];
    const rewards = dashboard?.rewards ?? [];

    return (
        <div className="mx-auto min-w-0 max-w-5xl">
            <div className="settings-pane-header">
                <div className="min-w-0">
                    <h2>邀请返利</h2>
                    <p>分享推广码，好友注册并完成在线充值后获得返利积分。</p>
                </div>
            </div>

            {loading ? (
                <div className="rounded-xl border border-border/70 bg-surface p-5 sm:p-8">
                    <Skeleton active paragraph={{ rows: 3 }} />
                </div>
            ) : null}

            {error ? (
                <Alert
                    type="error"
                    showIcon
                    message="邀请返利加载失败"
                    description={error}
                    action={
                        <Button size="small" onClick={() => setRequestVersion((value) => value + 1)}>
                            重试
                        </Button>
                    }
                />
            ) : null}

            {dashboard && !loading ? (
                <>
                    {!dashboard.enabled ? <Alert className="mb-4" type="info" showIcon message="管理员目前关闭了邀请返利，仍可查看已有邀请和返利记录。" /> : null}

                    <section aria-label="邀请信息" className="overflow-hidden rounded-xl border border-border/70 bg-surface shadow-sm">
                        {dashboard.enabled ? (
                            <>
                                <div className="grid gap-6 p-5 sm:p-6 lg:grid-cols-[minmax(0,1fr)_minmax(280px,0.8fr)] lg:items-center lg:gap-8 lg:p-8">
                                    <div className="min-w-0">
                                        <p className="mb-2 text-caption text-muted-foreground">我的推广码</p>
                                        <div className="flex flex-wrap items-center gap-2">
                                            <span className="font-mono text-3xl font-semibold tracking-wide text-foreground sm:text-4xl">{dashboard.code}</span>
                                            <Button type="text" aria-label="复制推广码" title="复制推广码" icon={<Copy className="size-4" />} onClick={() => void copyText(dashboard.code, "推广码已复制")} />
                                        </div>
                                        <Button className="mt-5" type="primary" icon={<Copy className="size-4" />} onClick={copyLink}>
                                            复制邀请链接
                                        </Button>
                                    </div>
                                    <div className="grid grid-cols-2 gap-4 border-t border-border/70 pt-5 lg:border-l lg:border-t-0 lg:pl-8 lg:pt-0">
                                        <div>
                                            <p className="text-caption text-muted-foreground">返利比例</p>
                                            <p className="mt-2 text-2xl font-semibold tabular-nums text-foreground sm:text-3xl">{(dashboard.effectiveRateBps / 100).toFixed(2)}%</p>
                                        </div>
                                        <div className="border-l border-border/70 pl-4">
                                            <p className="text-caption text-muted-foreground">已邀请好友</p>
                                            <p className="mt-2 text-2xl font-semibold tabular-nums text-foreground sm:text-3xl">
                                                {dashboard.inviteeCount} <span className="text-base font-medium">人</span>
                                            </p>
                                        </div>
                                    </div>
                                </div>
                                <p className="border-t border-border/70 bg-secondary/25 px-5 py-3 text-caption leading-relaxed text-muted-foreground sm:px-6 lg:px-8">仅在线充值参与返利 · 管理员审核通过后发放积分</p>
                            </>
                        ) : (
                            <div className="flex flex-wrap items-center gap-5 p-5 sm:p-6">
                                <div className="flex size-11 items-center justify-center rounded-lg bg-secondary/60 text-muted-foreground">
                                    <Gift aria-hidden className="size-5" />
                                </div>
                                <div>
                                    <p className="text-caption text-muted-foreground">已邀请好友</p>
                                    <p className="mt-1 text-2xl font-semibold tabular-nums text-foreground">{dashboard.inviteeCount} 人</p>
                                </div>
                            </div>
                        )}
                    </section>

                    <section aria-label="邀请历史" className="mt-5 min-w-0 rounded-xl border border-border/70 bg-surface p-4 shadow-sm sm:p-6">
                        <p className="mb-2 text-caption text-muted-foreground sm:hidden">仅展示最近 100 {activeTab === "invitees" ? "人" : "笔"}</p>
                        <Tabs
                            activeKey={activeTab}
                            onChange={setActiveTab}
                            tabBarExtraContent={<span className="hidden text-caption text-muted-foreground sm:inline">仅展示最近 100 {activeTab === "invitees" ? "人" : "笔"}</span>}
                            items={[
                                {
                                    key: "invitees",
                                    label: "被邀请用户",
                                    children:
                                        invitees.length > 0 ? (
                                            <TableSurface className="mt-1 border border-border/70 bg-transparent">
                                                <Table<ReferralInvitee>
                                                    className="app-data-table"
                                                    rowKey="userId"
                                                    size="small"
                                                    dataSource={invitees}
                                                    columns={inviteeColumns}
                                                    pagination={{ pageSize: 10, hideOnSinglePage: true, showSizeChanger: false }}
                                                    tableLayout="fixed"
                                                    scroll={{ x: 500 }}
                                                />
                                            </TableSurface>
                                        ) : (
                                            <EmptyState
                                                icon={UsersRound}
                                                title="还没有好友通过你的链接注册"
                                                description={dashboard.enabled ? "分享邀请链接，邀请好友一起创作。" : "已有邀请记录会显示在这里。"}
                                                action={dashboard.enabled ? <Button onClick={copyLink}>复制邀请链接</Button> : undefined}
                                            />
                                        ),
                                },
                                {
                                    key: "rewards",
                                    label: "返利记录",
                                    children:
                                        rewards.length > 0 ? (
                                            <TableSurface className="mt-1 border border-border/70 bg-transparent">
                                                <Table<ReferralReward>
                                                    className="app-data-table"
                                                    rowKey="id"
                                                    size="small"
                                                    dataSource={rewards}
                                                    columns={rewardColumns}
                                                    pagination={{ pageSize: 10, hideOnSinglePage: true, showSizeChanger: false }}
                                                    tableLayout="fixed"
                                                    scroll={{ x: 760 }}
                                                />
                                            </TableSurface>
                                        ) : (
                                            <EmptyState icon={ReceiptText} title="暂无返利记录" description="好友完成在线充值后，待审核返利会显示在这里。" />
                                        ),
                                },
                            ]}
                        />
                    </section>
                </>
            ) : null}
        </div>
    );
}
