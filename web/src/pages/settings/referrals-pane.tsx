import { Alert, App, Button, Table, Typography } from "antd";
import { useEffect, useState } from "react";

import { formatCredits } from "@/constant/credits";
import { getReferralDashboard, type ReferralDashboard, type ReferralReward } from "@/services/api/referrals";

const statusLabels: Record<ReferralReward["status"], string> = { pending: "待管理员审核", approved: "已发放", rejected: "已拒绝" };

export function ReferralsPane() {
    const { message } = App.useApp();
    const [dashboard, setDashboard] = useState<ReferralDashboard | null>(null);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        let cancelled = false;
        void getReferralDashboard().then((value) => { if (!cancelled) setDashboard(value); })
            .catch((error) => { if (!cancelled) message.error(error instanceof Error ? error.message : "读取邀请返利失败"); })
            .finally(() => { if (!cancelled) setLoading(false); });
        return () => { cancelled = true; };
    }, [message]);

    const copyLink = async () => {
        if (!dashboard) return;
        try {
            await navigator.clipboard.writeText(`${window.location.origin}/register?ref=${encodeURIComponent(dashboard.code)}`);
            message.success("邀请链接已复制");
        } catch {
            message.error("复制失败，请检查浏览器剪贴板权限");
        }
    };

    return <div className="settings-section">
        <div className="settings-pane-header"><div><h2>邀请返利</h2><p>分享推广码，好友注册并完成在线充值后获得待审核积分。</p></div></div>
        {loading ? <Typography.Text>正在读取邀请信息…</Typography.Text> : null}
        {dashboard && !dashboard.enabled ? <Alert type="info" showIcon message="管理员目前关闭了邀请返利；已有返利记录仍可查看。" /> : null}
        {dashboard?.enabled ? <div className="settings-section">
            <Typography.Paragraph>当前返利比例：{(dashboard.effectiveRateBps / 100).toFixed(2)}%。只有在线充值参与返利，管理员审核通过后直接发放积分。</Typography.Paragraph>
            <Typography.Paragraph>我的推广码：<Typography.Text copyable>{dashboard.code}</Typography.Text></Typography.Paragraph>
            <Button onClick={() => void copyLink()}>复制邀请链接</Button>
            <Typography.Paragraph>已邀请 {dashboard.inviteeCount} 人，下面显示最近 100 人</Typography.Paragraph>
            <Table rowKey="userId" size="small" pagination={false} dataSource={dashboard.invitees} columns={[
                { title: "被邀请用户", dataIndex: "email" },
                { title: "注册时间", dataIndex: "joinedAt", render: (value: string) => new Date(value).toLocaleString("zh-CN") },
            ]} />
        </div> : null}
        {dashboard ? <div className="settings-section">
            <Typography.Title level={5}>最近 100 笔返利记录</Typography.Title>
            <Table rowKey="id" size="small" pagination={{ pageSize: 10 }} dataSource={dashboard.rewards} columns={[
                { title: "充值订单", dataIndex: "paymentOrderId", ellipsis: true },
                { title: "返利积分", dataIndex: "rewardMicrocredits", render: (value: number) => formatCredits(value) },
                { title: "状态", dataIndex: "status", render: (value: ReferralReward["status"]) => statusLabels[value] },
                { title: "时间", dataIndex: "createdAt", render: (value: string) => new Date(value).toLocaleString("zh-CN") },
            ]} />
        </div> : null}
    </div>;
}
