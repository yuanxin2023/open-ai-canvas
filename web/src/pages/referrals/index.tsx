import { Alert, App, Button, Input, Skeleton, Table, Tabs } from "antd";
import type { ColumnsType } from "antd/es/table";
import { ArrowRight, CheckCircle2, Clock3, Copy, Gift, Info, Link2, ReceiptText, RefreshCw, UsersRound } from "lucide-react";
import { useEffect, useMemo, useState, type ReactNode } from "react";

import { TableSurface, WorkspacePage } from "@/components/layout/workspace-page";
import { StatusBadge } from "@/components/ui/base/badges";
import { EmptyState } from "@/components/ui/product/empty-state";
import { formatCredits } from "@/constant/credits";
import { getReferralDashboard, type ReferralDashboard, type ReferralPolicy, type ReferralReward } from "@/services/api/referrals";
import { referralPolicySummary } from "@/lib/referral-policy";
import { useUserStore } from "@/stores/use-user-store";
import { referralFirstRechargeExample, referralRewardTotals, referralStageLabel } from "./referral-view";
import "./referrals.css";

type ReferralInvitee = ReferralDashboard["invitees"][number];

const rewardStatus: Record<ReferralReward["status"], { label: string; tone: "warning" | "success" | "error" }> = {
    pending: { label: "待审核", tone: "warning" },
    approved: { label: "已到账", tone: "success" },
    rejected: { label: "未通过", tone: "error" },
};

const inviteeColumns: ColumnsType<ReferralInvitee> = [
    { title: "被邀请好友", dataIndex: "email", width: 210, ellipsis: true },
    { title: "注册时间", dataIndex: "joinedAt", width: 190, render: (value: string) => new Date(value).toLocaleString("zh-CN") },
    { title: "绑定奖励规则", key: "policy", width: 360, render: (_, item) => referralPolicySummary(item.policy) },
];

function ReferralStat({ label, icon, value, unit, note, pending = false }: {
    label: string; icon: ReactNode; value: string | number; unit: string; note: string; pending?: boolean;
}) {
    return <div className={`user-referral-card user-referral-stat ${pending ? "is-pending" : ""}`}>
        <div className="user-referral-stat-label"><span>{label}</span>{icon}</div>
        <p className="user-referral-stat-value" title={String(value)}>{value}<small>{unit}</small></p>
        <p className="user-referral-stat-note">{note}</p>
    </div>;
}

function ReferralRules({ policy, enabled }: { policy: ReferralPolicy; enabled: boolean }) {
    const last = policy.stages.at(-1);
    const example = referralFirstRechargeExample(policy, 1_200_000_000);
    return <aside className="user-referral-card user-referral-rules" aria-label="新邀请奖励规则">
        <header><h3>新邀请奖励规则</h3><span className="user-referral-chip">当前规则</span></header>
        <div className="user-referral-stage-list">
            {policy.stages.map((stage, index) => <div className="user-referral-stage" key={index}>
                <div><p>{referralStageLabel(policy, index)}</p><small>{stage.rateBps === 0 ? "本阶段不产生邀请人奖励" : "按套餐基础积分计算"}</small></div>
                <strong>{stage.rateBps / 100}<small>%</small></strong>
            </div>)}
        </div>
        {last?.until ? <p className="user-referral-stop">{policy.mode === "time" ? `首次充值满 ${last.until} 天后停止返利` : `第 ${last.until + 1} 笔起不再返利`}</p> : null}
        {policy.maxDays || policy.maxOrders || policy.perInviteeCapMicrocredits ? <dl className="user-referral-limits">
            {policy.maxDays ? <div><dt>有效期</dt><dd>首次充值起 {policy.maxDays} 天</dd></div> : null}
            {policy.maxOrders ? <div><dt>参与单数</dt><dd>每位好友前 {policy.maxOrders} 笔</dd></div> : null}
            {policy.perInviteeCapMicrocredits ? <div><dt>累计奖励上限</dt><dd>每位好友 {formatCredits(policy.perInviteeCapMicrocredits)} 积分</dd></div> : null}
        </dl> : null}
        {enabled ? <div className="user-referral-example"><div><span>例如，好友首次充值</span><p>99 元 / 1,200 积分</p></div><div><span>你可获得</span><strong>{formatCredits(example.inviter)}<small>积分</small></strong></div></div> : null}
        {enabled && policy.newcomerEnabled ? <p className="user-referral-newcomer"><Gift size={14} />好友首充该套餐还可额外获得 {formatCredits(example.newcomer)} 积分。</p> : null}
        <p className="user-referral-rules-note"><Info size={13} />以上规则适用于新邀请。已绑定好友继续使用其注册时的规则，可在好友记录中查看。</p>
        <p className="user-referral-rules-foot">{policy.mode === "time" ? "天数按首次成功充值起的连续 24 小时计算。" : "仅在线积分充值参与。"}奖励经管理员审核后发放。</p>
    </aside>;
}

export default function ReferralsPage() {
    const { message } = App.useApp();
    const userId = useUserStore((state) => state.user?.id);
    const [loaded, setLoaded] = useState<{ owner: string; data: ReferralDashboard } | null>(null);
    const dashboard = loaded && loaded.owner === userId ? loaded.data : null;
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState("");
    const [requestVersion, setRequestVersion] = useState(0);
    const [activeTab, setActiveTab] = useState("invitees");

    useEffect(() => {
        const controller = new AbortController();
        if (!userId) { setLoaded(null); setLoading(false); return () => controller.abort(); }
        setLoading(true); setError("");
        void getReferralDashboard(controller.signal)
            .then((value) => { if (!controller.signal.aborted) setLoaded({ owner: userId, data: value }); })
            .catch((reason) => {
                if (controller.signal.aborted) return;
                setLoaded(null);
                setError(reason instanceof Error ? reason.message : "读取邀请返利失败");
            })
            .finally(() => { if (!controller.signal.aborted) setLoading(false); });
        return () => controller.abort();
    }, [requestVersion, userId]);

    const invitees = useMemo(() => dashboard?.invitees ?? [], [dashboard]);
    const rewards = useMemo(() => dashboard?.rewards ?? [], [dashboard]);
    const totals = useMemo(() => referralRewardTotals(rewards), [rewards]);
    const names = useMemo(() => new Map(invitees.map((invitee) => [invitee.userId, invitee.email])), [invitees]);
    const invitationLink = dashboard?.enabled ? `${window.location.origin}/register?ref=${encodeURIComponent(dashboard.code)}` : "";

    const copyText = async (value: string, successMessage: string) => {
        if (!dashboard?.enabled || loading) return;
        try { await navigator.clipboard.writeText(value); message.success(successMessage); }
        catch { message.error("复制失败，请检查浏览器剪贴板权限"); }
    };
    const copyLink = () => { if (invitationLink) void copyText(invitationLink, "邀请链接已复制"); };

    const rewardColumns: ColumnsType<ReferralReward> = [
        { title: "被邀请好友", key: "invitee", width: 215, render: (_, item) => <div className="user-referral-record-person"><span title={item.inviteeId}>{names.get(item.inviteeId) || `用户 ${item.inviteeId.slice(0, 8)}…`}</span><small title={item.paymentOrderId}>订单 …{item.paymentOrderId.slice(-8)}</small></div> },
        { title: "奖励依据", key: "basis", width: 230, render: (_, item) => <div className="user-referral-record-basis">{item.policyVersionId ? <>好友第 {item.orderOrdinal} 笔充值<small>{formatCredits(item.baseMicrocredits)} 基础积分 × {item.rateBps / 100}%</small></> : <span title={item.paymentOrderId}>历史返利记录</span>}</div> },
        { title: "返利积分", dataIndex: "rewardMicrocredits", width: 140, render: (value: number) => <span className="user-referral-record-amount">{formatCredits(value)}<small>积分</small></span> },
        { title: "奖励状态", key: "status", width: 180, render: (_, item) => <div><StatusBadge {...rewardStatus[item.status]} />{item.status === "rejected" && item.reviewNote ? <small className="user-referral-rejection" title={item.reviewNote}>{item.reviewNote}</small> : null}</div> },
        { title: "产生时间", dataIndex: "createdAt", width: 170, render: (value: string) => new Date(value).toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false }) },
    ];

    return <WorkspacePage className="settings-page app-workspace-page app-user-workspace user-referrals-page">
        <div className="user-referrals-shell">
            <header className="user-referrals-header"><div><h2>邀请返利</h2><p>把好用的创作工具分享给朋友，获得更多创作积分。</p></div><span className="user-referrals-header-badge"><Gift size={13} />积分奖励 · 审核后到账</span></header>
            {loading && !dashboard ? <div className="user-referral-card user-referrals-loading"><Skeleton active paragraph={{ rows: 5 }} /></div> : null}
            {error ? <Alert type="error" showIcon message="邀请返利加载失败" description={error} action={<Button size="small" onClick={() => setRequestVersion((value) => value + 1)}>重试</Button>} /> : null}
            {dashboard ? <>
                <div className="user-referrals-upper"><div className="user-referrals-share-column">
                    {dashboard.enabled ? <section className="user-referral-card user-referrals-share" aria-label="分享邀请">
                        <div className="user-referrals-gift" aria-hidden><Gift /></div>
                        <div className="user-referrals-share-intro"><span><UsersRound size={14} />邀请好友，一起创作</span><h3>分享邀请，赚取创作积分</h3><p>好友通过你的链接注册并完成充值，你即可获得积分奖励。</p></div>
                        <div className="user-referrals-link-row"><Input aria-label="我的邀请链接" readOnly value={invitationLink} prefix={<Link2 size={15} />} onFocus={(event) => event.target.select()} /><Button type="primary" icon={<Copy size={15} />} disabled={loading} onClick={copyLink}>复制邀请链接</Button></div>
                        <div className="user-referrals-code"><span>我的推广码</span><strong>{dashboard.code}</strong><Button type="text" size="small" aria-label="复制推广码" title="复制推广码" icon={<Copy size={14} />} disabled={loading} onClick={() => void copyText(dashboard.code, "推广码已复制")} /><span className="user-referrals-code-hint">好友注册时也可手动填写</span></div>
                        <ol className="user-referrals-steps">{["分享邀请链接", "好友注册并充值", "奖励审核后到账"].map((step, index) => <li key={step}><i>{index + 1}</i><span>{step}</span>{index < 2 ? <ArrowRight size={14} aria-hidden /> : null}</li>)}</ol>
                    </section> : <section className="user-referral-card user-referrals-unavailable"><Gift size={24} /><h3>邀请推广暂不可用</h3><p>目前无法分享新邀请，已有邀请和返利记录仍可查看。</p></section>}
                    <section className="user-referrals-stats" aria-label="我的推广统计">
                        <ReferralStat label="已邀请好友" icon={<UsersRound />} value={dashboard.inviteeCount} unit="人" note="通过你的链接注册的好友" />
                        <ReferralStat label="已到账积分" icon={<CheckCircle2 />} value={formatCredits(totals.approved)} unit="积分" note="最近 100 条返利中已发放的奖励" />
                        <ReferralStat label="待审核积分" icon={<Clock3 />} value={formatCredits(totals.pending)} unit="积分" note="最近 100 条返利中等待审核的奖励" pending />
                    </section>
                </div><ReferralRules policy={dashboard.policy} enabled={dashboard.enabled} /></div>
                <section className="user-referral-card user-referrals-records" aria-label="我的推广记录">
                    <Tabs activeKey={activeTab} onChange={setActiveTab}
                        tabBarExtraContent={<div className="user-referrals-record-tools"><span className="user-referrals-record-scope">展示最近 100 {activeTab === "invitees" ? "位好友" : "条返利"}</span><Button type="text" size="small" icon={<RefreshCw size={13} />} loading={loading} onClick={() => setRequestVersion((value) => value + 1)}>刷新</Button></div>}
                        items={[
                            { key: "invitees", label: <span className="user-referrals-tab"><UsersRound size={15} />邀请的好友</span>, children: invitees.length ? <TableSurface className="user-referrals-table"><Table<ReferralInvitee> className="app-data-table" rowKey="userId" size="small" dataSource={invitees} columns={inviteeColumns} pagination={{ pageSize: 10, hideOnSinglePage: true, showSizeChanger: false }} tableLayout="fixed" scroll={{ x: 760 }} /></TableSurface> : <div className="user-referrals-empty"><EmptyState size="compact" icon={UsersRound} title={dashboard.enabled ? "从邀请第一位好友开始" : "暂无邀请记录"} description={dashboard.enabled ? "复制上方邀请链接，分享给需要 AI 创作的朋友。" : "已有邀请记录会显示在这里。"} /><p>好友通过链接注册后，会在这里显示。</p></div> },
                            { key: "rewards", label: <span className="user-referrals-tab"><ReceiptText size={15} />返利记录</span>, children: rewards.length ? <><TableSurface className="user-referrals-table"><Table<ReferralReward> className="app-data-table" rowKey="id" size="small" dataSource={rewards} columns={rewardColumns} pagination={{ pageSize: 10, hideOnSinglePage: true, showSizeChanger: false }} tableLayout="fixed" scroll={{ x: 935 }} /></TableSurface><p className="user-referrals-record-summary">最近 {rewards.length} 条返利记录 · 已到账 {formatCredits(totals.approved)} 积分，待审核 {formatCredits(totals.pending)} 积分</p></> : <div className="user-referrals-empty"><EmptyState size="compact" icon={ReceiptText} title="暂无返利记录" description="好友完成在线充值后，奖励会先进入待审核状态。" /><p>审核通过后，积分将发放到你的账户。</p></div> },
                        ]} />
                    <p className="user-referrals-mobile-scope">展示最近 100 {activeTab === "invitees" ? "位好友" : "条返利"}</p>
                </section>
                <p className="user-referrals-page-note"><Info size={13} />积分按好友购买套餐的基础积分计算，活动赠送积分不参与返利。实际奖励受对应规则的有效期、参与单数和累计上限限制。</p>
            </> : null}
        </div>
    </WorkspacePage>;
}
