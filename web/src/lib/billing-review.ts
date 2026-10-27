import type { BillingOrder } from "@/services/api/wallet";

export function billingReviewReservation(order: Pick<BillingOrder, "refundRecoveryPending" | "reservedAmountMicrocredits" | "amountMicrocredits">) {
    return order.refundRecoveryPending ? 0 : order.reservedAmountMicrocredits || order.amountMicrocredits;
}

export function billingReviewStatusLabel(order: Pick<BillingOrder, "status" | "refundRecoveryPending">) {
    if (order.refundRecoveryPending) return "退款后待核对";
    return { uncertain: "待核对", running: "运行中", reserved: "已预授权", settled: "已结算", refunded: "已退款" }[order.status];
}

export function billingReviewRefundLabel(order?: Pick<BillingOrder, "refundRecoveryPending">) {
    return order?.refundRecoveryPending ? "取消恢复扣费" : "退回预授权";
}

export function billingReviewDescription(action: "settle" | "refund", orders: ReadonlyArray<Pick<BillingOrder, "refundRecoveryPending">>) {
    const recovering = orders.some((order) => order.refundRecoveryPending);
    if (action === "refund") return recovering ? "仍冻结的预授权会退回；已经退回的订单只会取消恢复扣费，不会重复增加余额。" : "退回操作只会释放尚未结算订单的预授权积分。";
    const base = "媒体订单重新核实时长，按原授权上限结算，超出部分由平台承担并审计；缺少时长或余额不足时保留待核对。Token 订单按既有用量规则结算。";
    return recovering ? `${base} 已退款订单从当前可用余额扣回，不再使用冻结积分，余额不足不扣成负数。` : base;
}
