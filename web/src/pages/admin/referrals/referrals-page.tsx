import { Alert, App, Button, Input, Select, Space, Switch, Table, Tabs } from "antd";
import { useCallback, useEffect, useRef, useState } from "react";
import { formatCredits } from "@/constant/credits";
import { defaultReferralPolicy } from "@/lib/referral-policy";
import { AdminPageFrame } from "@/pages/admin/components/admin-shell";
import { searchAdminUserReferences, type AdminUserReference } from "@/services/api/auth";
import { getAdminReferralPolicy, getAdminReferralUser, listAdminReferralRewards, reviewAdminReferralReward, setAdminReferralUserPolicy, updateAdminReferralPolicy, type ReferralPolicy, type ReferralReward, type ReferralUserPolicy } from "@/services/api/referrals";
import { PolicyEditor } from "./policy-editor";

const statuses = { pending: "待审核", approved: "已批准", rejected: "已拒绝", all: "全部" };

export default function ReferralsAdminPage() {
    const { message } = App.useApp();
    const [policy, setPolicy] = useState<ReferralPolicy>();
    const [saving, setSaving] = useState(false);
    const [selectedUserId, setSelectedUserId] = useState<string>();
    const [userOptions, setUserOptions] = useState<AdminUserReference[]>([]);
    const sequence = useRef(0);
    const [user, setUser] = useState<ReferralUserPolicy>();
    const [custom, setCustom] = useState(false);
    const [paused, setPaused] = useState(false);
    const [loadingUser, setLoadingUser] = useState(false);
    const [status, setStatus] = useState<ReferralReward["status"] | "all">("pending");
    const [page, setPage] = useState(1);
    const [total, setTotal] = useState(0);
    const [rewards, setRewards] = useState<ReferralReward[]>([]);
    const [loadingRewards, setLoadingRewards] = useState(false);
    const [notes, setNotes] = useState<Record<string, string>>({});
    const [reviewingId, setReviewingId] = useState("");
    const fail = useCallback((error: unknown) => { message.error(error instanceof Error ? error.message : "操作失败"); }, [message]);
    useEffect(() => { void getAdminReferralPolicy().then(setPolicy).catch(fail); }, [fail]);
    const loadRewards = useCallback(async () => {
        setLoadingRewards(true);
        try { const result = await listAdminReferralRewards(status, page, 20); setRewards(result.rewards); setTotal(result.total); }
        catch (error) { fail(error); } finally { setLoadingRewards(false); }
    }, [status, page, fail]);
    useEffect(() => { void loadRewards(); }, [loadRewards]);
    useEffect(() => {
        if (!selectedUserId) return;
        let cancelled = false;
        setLoadingUser(true); setUser(undefined);
        void getAdminReferralUser(selectedUserId).then((next) => {
            if (cancelled) return;
            setUser(next); setCustom(Boolean(next.policy)); setPaused(next.paused);
        }).catch((error) => { if (!cancelled) fail(error); }).finally(() => { if (!cancelled) setLoadingUser(false); });
        return () => { cancelled = true; };
    }, [selectedUserId, fail]);
    const searchUsers = async (keyword: string) => {
        const current = ++sequence.current;
        try { const result = await searchAdminUserReferences({ keyword, limit: 100 }); if (current === sequence.current) setUserOptions(result.users); }
        catch (error) { fail(error); }
    };
    const save = async (next: ReferralPolicy, personal: boolean) => {
        setSaving(true);
        try {
            if (personal && selectedUserId) setUser(await setAdminReferralUserPolicy(selectedUserId, custom ? next : null, paused));
            else setPolicy(await updateAdminReferralPolicy(next));
            message.success("规则已保存，已有邀请关系继续使用绑定版本");
        } catch (error) { fail(error); } finally { setSaving(false); }
    };
    const review = async (reward: ReferralReward, approve: boolean) => {
        const note = notes[reward.id]?.trim() || "";
        if (!approve && !note) { message.warning("拒绝时请填写原因"); return; }
        setReviewingId(reward.id);
        try { await reviewAdminReferralReward(reward.id, approve, note); message.success(approve ? "返利已发放" : "返利已拒绝"); await loadRewards(); }
        catch (error) { fail(error); } finally { setReviewingId(""); }
    };
    return <AdminPageFrame title="邀请返利" description="管理阶梯规则、专属方案与逐笔审核" scroll>
        <Tabs items={[
            { key: "policy", label: "全局推广规则", children: policy ? <>
                <PolicyEditor global key={policy.versionId} initial={policy} saving={saving} onSave={(value) => void save(value, false)} />
            </> : <Alert type="info" message="规则未加载，刷新页面重试" /> },
            { key: "users", label: "个人专属规则", children: <div className="min-w-0">
                <Select className="mb-4 w-full" showSearch filterOption={false} placeholder="搜索并选择邀请人" value={selectedUserId} onSearch={(keyword) => void searchUsers(keyword)} onChange={setSelectedUserId} loading={loadingUser} disabled={saving} options={userOptions.map((item) => ({ value: item.id, label: item.username + " (" + item.id + ")" }))} />
                {user ? <>
                    <div className="mb-4 flex flex-wrap gap-6"><label className="flex items-center gap-2"><Switch checked={custom} onChange={setCustom} /> 使用专属完整规则</label><label className="flex items-center gap-2"><Switch checked={paused} onChange={setPaused} /> 暂停该邀请人的新绑定及奖励</label></div>
                    {custom ? <PolicyEditor key={user.userId + ":" + (user.policy?.versionId ?? "new")} initial={user.policy ?? { ...(policy ?? defaultReferralPolicy), versionId: "" }} saving={saving} onSave={(value) => void save(value, true)} /> : <Button type="primary" loading={saving} onClick={() => void save(policy ?? defaultReferralPolicy, true)}>保存，之后的新邀请使用全局规则</Button>}
                    <p className="mt-3 text-caption text-muted-foreground">专属规则完整替代全局方案。暂停对现有关系立即生效；修改方案和恢复全局仅影响之后的新绑定。</p>
                </> : null}
            </div> },
            { key: "rewards", label: "返利审核", children: <>
                <Select className="mb-4 w-40" value={status} onChange={(value) => { setStatus(value); setPage(1); }} options={Object.entries(statuses).map(([value, label]) => ({ value, label }))} />
                <Table rowKey="id" loading={loadingRewards} dataSource={rewards} pagination={{ current: page, pageSize: 20, total, onChange: setPage }} scroll={{ x: 1450 }} columns={[
                    { title: "订单", dataIndex: "paymentOrderId", width: 160, ellipsis: true },
                    { title: "邀请人", dataIndex: "inviterId", width: 140, ellipsis: true },
                    { title: "被邀请人", dataIndex: "inviteeId", width: 140, ellipsis: true },
                    { title: "规则版本", dataIndex: "policyVersionId", width: 140, ellipsis: true, render: (value?: string) => value || "历史记录" },
                    { title: "基础积分 / 单序 / 阶段", width: 180, render: (_, row: ReferralReward) => row.policyVersionId ? formatCredits(row.baseMicrocredits) + " / 第" + row.orderOrdinal + "单 / " + (row.stageIndex + 1) : "—" },
                    { title: "比例", dataIndex: "rateBps", width: 80, render: (value: number) => value / 100 + "%" },
                    { title: "原始 / 实际积分", width: 140, render: (_, row: ReferralReward) => (row.policyVersionId ? formatCredits(row.uncappedMicrocredits) : "—") + " / " + formatCredits(row.rewardMicrocredits) },
                    { title: "状态", dataIndex: "status", width: 100, render: (value: ReferralReward["status"]) => statuses[value] },
                    { title: "审核", width: 270, render: (_, row: ReferralReward) => row.status === "pending" ? <Space orientation="vertical">
                        <Input value={notes[row.id] || ""} onChange={(event) => setNotes((old) => ({ ...old, [row.id]: event.target.value }))} placeholder="审核备注 / 拒绝原因" maxLength={500} />
                        <Space><Button type="primary" disabled={Boolean(reviewingId)} loading={reviewingId === row.id} onClick={() => void review(row, true)}>批准并发放</Button><Button danger disabled={Boolean(reviewingId)} onClick={() => void review(row, false)}>拒绝</Button></Space>
                    </Space> : row.reviewNote || "—" },
                ]} />
            </> },
        ]} />
    </AdminPageFrame>;
}
