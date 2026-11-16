import { Typography } from "antd";
import { paymentOrderStatus } from "@/lib/payment-order-status";
import type { AdminReferralReward } from "@/services/api/referrals";
import { AdminStatusBadge } from "../components/admin-ui";

export function ReferralUserSummary({ user, userId }: { user: AdminReferralReward["inviter"]; userId: string }) {
    return (
        <div className="flex min-w-0 flex-col gap-1 break-all">
            <strong>{user ? `@${user.username}` : "用户不存在"}</strong>
            <div className="text-caption text-muted-foreground">
                ID：
                <Typography.Text copyable={{ text: userId }} className="font-mono">
                    {userId}
                </Typography.Text>
            </div>
            <div className="text-caption text-muted-foreground">邮箱：{user?.email ? <Typography.Text copyable={{ text: user.email }}>{user.email}</Typography.Text> : user ? "未填写" : "—"}</div>
        </div>
    );
}

export function ReferralPaymentSummary({ order, orderId }: { order: AdminReferralReward["paymentOrder"]; orderId: string }) {
    if (!order) {
        return (
            <div className="flex flex-col gap-1 break-all">
                <span className="text-muted-foreground">关联支付订单不存在或归属异常</span>
                <Typography.Text copyable={{ text: orderId }} className="font-mono">
                    {orderId}
                </Typography.Text>
            </div>
        );
    }
    const status = paymentOrderStatus[order.status];
    return (
        <div className="flex min-w-0 flex-col items-start gap-1 break-all">
            <Typography.Text copyable={{ text: order.merchantOrderNo }} className="font-mono">
                {order.merchantOrderNo}
            </Typography.Text>
            <div className="flex flex-wrap items-center gap-2">
                <span className="tabular-nums">
                    {order.currency} {(order.amountFen / 100).toFixed(2)}
                </span>
                <AdminStatusBadge tone={status?.tone ?? "neutral"} label={status?.label ?? order.status} />
            </div>
            <span className="text-caption text-muted-foreground">{order.creditedAt ? `入账：${new Date(order.creditedAt).toLocaleString("zh-CN", { hour12: false })}` : "未入账"}</span>
        </div>
    );
}
