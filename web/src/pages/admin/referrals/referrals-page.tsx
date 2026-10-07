import { App, Button, Form, Input, InputNumber, Select, Space, Switch, Table, Tabs, Typography } from "antd";
import { useCallback, useEffect, useRef, useState } from "react";

import { formatCredits } from "@/constant/credits";
import { useAdminContext } from "@/pages/admin/admin-context";
import { AdminPageFrame } from "@/pages/admin/components/admin-shell";
import { searchAdminUserReferences, type AdminUserReference } from "@/services/api/auth";
import { getAdminReferralPolicy, getAdminReferralUser, listAdminReferralRewards, reviewAdminReferralReward, setAdminReferralRate, updateAdminReferralPolicy, type ReferralPolicy, type ReferralReward } from "@/services/api/referrals";

type PolicyForm = { enabled: boolean; globalRatePercent: number; creditsPerYuan: number; perInviteeCap: number };
const rewardStatusLabels: Record<ReferralReward["status"], string> = { pending: "待审核", approved: "已批准", rejected: "已拒绝" };

export default function ReferralsAdminPage() {
    const { message } = App.useApp();
    const { references } = useAdminContext();
    const [form] = Form.useForm<PolicyForm>();
    const [loading, setLoading] = useState(true);
    const [saving, setSaving] = useState(false);
    const [selectedUserId, setSelectedUserId] = useState<string>();
    const [userOptions, setUserOptions] = useState<AdminUserReference[]>([]);
    const userSearchSequence = useRef(0);
    const [customRate, setCustomRate] = useState<number | null>(null);
    const [savingRate, setSavingRate] = useState(false);
    const [status, setStatus] = useState<ReferralReward["status"] | "all">("pending");
    const [page, setPage] = useState(1);
    const [total, setTotal] = useState(0);
    const [rewards, setRewards] = useState<ReferralReward[]>([]);
    const [loadingRewards, setLoadingRewards] = useState(false);
    const [reviewNotes, setReviewNotes] = useState<Record<string, string>>({});
    const [reviewingId, setReviewingId] = useState("");

    const loadRewards = useCallback(
        async (nextStatus = status, nextPage = page) => {
            setLoadingRewards(true);
            try {
                const result = await listAdminReferralRewards(nextStatus, nextPage, 20);
                setRewards(result.rewards);
                setTotal(result.total);
                setPage(result.page);
            } catch (error) {
                message.error(error instanceof Error ? error.message : "读取返利记录失败");
            } finally {
                setLoadingRewards(false);
            }
        },
        [message, page, status],
    );

    useEffect(() => {
        void getAdminReferralPolicy()
            .then((policy) =>
                form.setFieldsValue({
                    enabled: policy.enabled,
                    globalRatePercent: policy.globalRateBps / 100,
                    creditsPerYuan: policy.creditsPerYuanMicro / 1_000_000,
                    perInviteeCap: policy.perInviteeCapMicrocredits / 1_000_000,
                }),
            )
            .catch((error) => message.error(error instanceof Error ? error.message : "读取返利规则失败"))
            .finally(() => setLoading(false));
    }, [form, message]);
    useEffect(() => {
        void loadRewards();
    }, [loadRewards]);
    useEffect(() => {
        setUserOptions(references.users);
    }, [references.users]);

    const searchUsers = async (keyword: string) => {
        const sequence = ++userSearchSequence.current;
        try {
            const result = await searchAdminUserReferences({ keyword, limit: 100 });
            if (sequence === userSearchSequence.current) setUserOptions(result.users);
        } catch (error) {
            message.error(error instanceof Error ? error.message : "搜索用户失败");
        }
    };

    const savePolicy = async (values: PolicyForm) => {
        setSaving(true);
        try {
            const policy: ReferralPolicy = {
                enabled: values.enabled,
                globalRateBps: Math.round(values.globalRatePercent * 100),
                creditsPerYuanMicro: Math.round(values.creditsPerYuan * 1_000_000),
                perInviteeCapMicrocredits: Math.round(values.perInviteeCap * 1_000_000),
            };
            await updateAdminReferralPolicy(policy);
            message.success("邀请返利规则已保存");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "保存失败");
        } finally {
            setSaving(false);
        }
    };

    const selectUser = async (userId: string) => {
        setSelectedUserId(userId);
        try {
            const profile = await getAdminReferralUser(userId);
            setCustomRate(profile.rateBps === undefined ? null : profile.rateBps / 100);
        } catch (error) {
            message.error(error instanceof Error ? error.message : "读取用户返利比例失败");
        }
    };

    const saveRate = async () => {
        if (!selectedUserId) return;
        setSavingRate(true);
        try {
            await setAdminReferralRate(selectedUserId, customRate === null ? null : Math.round(customRate * 100));
            message.success("专属比例已保存");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "保存失败");
        } finally {
            setSavingRate(false);
        }
    };

    const review = async (reward: ReferralReward, approve: boolean) => {
        const note = reviewNotes[reward.id]?.trim() || "";
        if (!approve && !note) {
            message.warning("拒绝时请填写原因");
            return;
        }
        setReviewingId(reward.id);
        try {
            await reviewAdminReferralReward(reward.id, approve, note);
            message.success(approve ? "返利已发放到邀请人钱包" : "返利已拒绝");
            await loadRewards();
        } catch (error) {
            message.error(error instanceof Error ? error.message : "审核失败");
        } finally {
            setReviewingId("");
        }
    };

    return (
        <AdminPageFrame title="邀请返利" description="管理推广规则、专属比例和逐笔审核" scroll>
            <Tabs
                items={[
                    {
                        key: "policy",
                        label: "规则与开关",
                        children: (
                            <Form<PolicyForm> form={form} layout="vertical" onFinish={(values) => void savePolicy(values)} initialValues={{ enabled: false, globalRatePercent: 0, creditsPerYuan: 1, perInviteeCap: 0 }} style={{ maxWidth: 560 }}>
                                <Form.Item name="enabled" label="启用邀请返利" valuePropName="checked">
                                    <Switch aria-label="启用邀请返利" />
                                </Form.Item>
                                <Form.Item name="globalRatePercent" label="全局返利比例（%）" rules={[{ required: true }]}>
                                    <InputNumber min={0} max={100} precision={2} style={{ width: "100%" }} />
                                </Form.Item>
                                <Form.Item name="creditsPerYuan" label="每实付 1 元对应的基础积分" rules={[{ required: true }]}>
                                    <InputNumber min={0.01} max={1000} precision={2} style={{ width: "100%" }} />
                                </Form.Item>
                                <Form.Item name="perInviteeCap" label="每位被邀请人的累计返利上限（积分，0 为不限）" rules={[{ required: true }]}>
                                    <InputNumber min={0} precision={2} style={{ width: "100%" }} />
                                </Form.Item>
                                <Typography.Paragraph type="secondary">仅已验证且成功入账的在线充值参与返利。新返利需人工审核；关闭开关后不再接受推广码或计提新返利。</Typography.Paragraph>
                                <Button type="primary" htmlType="submit" loading={saving || loading}>
                                    保存规则
                                </Button>
                            </Form>
                        ),
                    },
                    {
                        key: "rates",
                        label: "专属比例",
                        children: (
                            <Space direction="vertical" style={{ width: "100%", maxWidth: 560 }}>
                                <Select
                                    showSearch
                                    filterOption={false}
                                    placeholder="搜索并选择邀请人"
                                    value={selectedUserId}
                                    onSearch={(keyword) => void searchUsers(keyword)}
                                    onChange={(value: string) => void selectUser(value)}
                                    options={userOptions.map((user) => ({ value: user.id, label: `${user.username} (${user.id})` }))}
                                />
                                <InputNumber value={customRate} onChange={setCustomRate} min={0} max={100} precision={2} placeholder="留空使用全局比例" addonAfter="%" style={{ width: "100%" }} />
                                <Button disabled={!selectedUserId} loading={savingRate} onClick={() => void saveRate()}>
                                    保存专属比例
                                </Button>
                                <Typography.Text type="secondary">设置为 0% 可停止该邀请人的新返利；清空则恢复全局比例。已生成的返利不重算。</Typography.Text>
                            </Space>
                        ),
                    },
                    {
                        key: "rewards",
                        label: "返利审核",
                        children: (
                            <>
                                <Select
                                    value={status}
                                    onChange={(value) => {
                                        setStatus(value);
                                        setPage(1);
                                    }}
                                    options={[
                                        { value: "pending", label: "待审核" },
                                        { value: "approved", label: "已批准" },
                                        { value: "rejected", label: "已拒绝" },
                                        { value: "all", label: "全部" },
                                    ]}
                                    style={{ width: 160, marginBottom: 16 }}
                                />
                                <Table
                                    rowKey="id"
                                    loading={loadingRewards}
                                    dataSource={rewards}
                                    pagination={{ current: page, pageSize: 20, total, onChange: (next) => setPage(next) }}
                                    scroll={{ x: 1050 }}
                                    columns={[
                                        { title: "订单", dataIndex: "paymentOrderId", width: 200, ellipsis: true },
                                        { title: "邀请人", dataIndex: "inviterId", width: 150, ellipsis: true },
                                        { title: "被邀请人", dataIndex: "inviteeId", width: 150, ellipsis: true },
                                        { title: "实付", dataIndex: "amountFen", width: 90, render: (value: number) => `¥${(value / 100).toFixed(2)}` },
                                        { title: "比例", dataIndex: "rateBps", width: 90, render: (value: number) => `${(value / 100).toFixed(2)}%` },
                                        { title: "返利积分", dataIndex: "rewardMicrocredits", width: 100, render: (value: number) => formatCredits(value) },
                                        { title: "状态", dataIndex: "status", width: 100, render: (value: ReferralReward["status"]) => rewardStatusLabels[value] },
                                        {
                                            title: "审核",
                                            width: 280,
                                            render: (_, reward: ReferralReward) =>
                                                reward.status === "pending" ? (
                                                    <Space direction="vertical">
                                                        <Input value={reviewNotes[reward.id] || ""} onChange={(event) => setReviewNotes((current) => ({ ...current, [reward.id]: event.target.value }))} placeholder="审核备注 / 拒绝原因" maxLength={500} />
                                                        <Space>
                                                            <Button type="primary" loading={reviewingId === reward.id} onClick={() => void review(reward, true)}>
                                                                批准并发放
                                                            </Button>
                                                            <Button danger disabled={Boolean(reviewingId)} onClick={() => void review(reward, false)}>
                                                                拒绝
                                                            </Button>
                                                        </Space>
                                                    </Space>
                                                ) : (
                                                    reward.reviewNote || "—"
                                                ),
                                        },
                                    ]}
                                />
                            </>
                        ),
                    },
                ]}
            />
        </AdminPageFrame>
    );
}
