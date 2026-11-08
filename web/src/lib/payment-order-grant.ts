import { formatCredits } from "@/constant/credits";
import type { PaymentOrder } from "@/services/api/payments";

/** 订单发放的积分与新人赠送描述。 */
export function paymentOrderGrantLabel(order: Pick<PaymentOrder, "creditsMicrocredits" | "newcomerBonusMicrocredits">) {
    return `${formatCredits(order.creditsMicrocredits, 6)} 积分${order.newcomerBonusMicrocredits ? ` + 新人赠送 ${formatCredits(order.newcomerBonusMicrocredits)} 积分` : ""}`;
}
