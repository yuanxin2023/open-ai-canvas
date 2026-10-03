import { Button, Segmented } from "antd";
import { TicketCheck } from "lucide-react";
import { lazy, Suspense, useEffect, useState } from "react";

import type { RedeemFundingSource } from "@/services/api/wallet";
import { useUserStore } from "@/stores/use-user-store";
import { AdminPageFrame } from "../components/admin-shell";

const RedemptionCodesPanel = lazy(() => import("../components/redemption-codes-panel"));

export default function RedemptionCodesPage() {
    const isFullAdmin = useUserStore((state) => state.user?.adminAccess?.level === "full");
    const [fundingSource, setFundingSource] = useState<RedeemFundingSource>(isFullAdmin ? "platform" : "module_admin");
    const [createOpen, setCreateOpen] = useState(false);
    const [createBlocked, setCreateBlocked] = useState(false);
    const canCreate = !isFullAdmin || fundingSource === "platform";

    useEffect(() => {
        setCreateOpen(false);
        setFundingSource(isFullAdmin ? "platform" : "module_admin");
    }, [isFullAdmin]);

    return (
        <AdminPageFrame
            title={isFullAdmin ? "兑换码" : "我的兑换码"}
            description={isFullAdmin ? "分区管理平台与模块管理员批次" : "使用个人积分发放兑换码，跟踪核销与退回"}
            actions={canCreate ? <Button type="primary" disabled={createBlocked} title={createBlocked ? "请先核对上一次结果不确定的生成请求" : undefined} icon={<TicketCheck className="size-4" />} onClick={() => setCreateOpen(true)}>生成批次</Button> : undefined}
        >
            {isFullAdmin ? (
                <Segmented<RedeemFundingSource>
                    aria-label="兑换码资金来源"
                    value={fundingSource}
                    options={[{ label: "平台兑换码", value: "platform" }, { label: "模块管理员兑换码", value: "module_admin" }]}
                    onChange={(value) => {
                        setCreateOpen(false);
                        setFundingSource(value);
                    }}
                />
            ) : null}
            <Suspense fallback={<div className="py-16 text-center text-sm text-foreground/50">正在读取兑换码批次...</div>}>
                <RedemptionCodesPanel fundingSource={fundingSource} isFullAdmin={isFullAdmin} createOpen={createOpen} onCreateOpenChange={setCreateOpen} onCreateBlockedChange={setCreateBlocked} />
            </Suspense>
        </AdminPageFrame>
    );
}
