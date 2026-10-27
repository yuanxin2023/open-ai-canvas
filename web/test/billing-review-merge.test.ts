import { describe, expect, test } from "bun:test";
import { billingReviewDescription, billingReviewRefundLabel, billingReviewReservation, billingReviewStatusLabel } from "../src/lib/billing-review";

describe("billing review after refund", () => {
    test("already returned funds are not shown as another refundable reservation", () => {
        const order = { amountMicrocredits: 50_000, reservedAmountMicrocredits: 50_000 };
        expect(billingReviewReservation(order)).toBe(50_000);
        expect(billingReviewReservation({ ...order, refundRecoveryPending: true })).toBe(0);
        expect(billingReviewReservation({ amountMicrocredits: 0, reservedAmountMicrocredits: 0 })).toBe(0);
    });
    test("recovery review and original pending states remain distinct", () => {
        expect(billingReviewStatusLabel({ status: "uncertain", refundRecoveryPending: true })).toBe("退款后待核对");
        expect(billingReviewStatusLabel({ status: "uncertain" })).toBe("待核对");
        expect(billingReviewStatusLabel({ status: "settled" })).toBe("已结算");
        expect(billingReviewStatusLabel({ status: "refunded" })).toBe("已退款");
    });
    test("manual settlement explains the ceiling and platform audit", () => {
        const description = billingReviewDescription("settle", [{}]);
        for (const text of ["重新核实时长", "原授权上限", "平台承担并审计", "缺少时长", "待核对", "Token"]) expect(description).toContain(text);
    });
    test("refund recovery draws from available balance without negative credit", () => {
        const description = billingReviewDescription("settle", [{ refundRecoveryPending: true }]);
        expect(description).toContain("从当前可用余额扣回");
        expect(description).toContain("不再使用冻结积分");
        expect(description).toContain("余额不足不扣成负数");
    });
    test("mixed refunds cannot promise duplicate credit for returned orders", () => {
        expect(billingReviewDescription("refund", [{}, { refundRecoveryPending: true }])).toContain("不会重复增加余额");
        expect(billingReviewDescription("refund", [{}])).toBe("退回操作只会释放尚未结算订单的预授权积分。");
    });
    test("returned orders offer cancellation of recovery instead of another refund", () => {
        expect(billingReviewRefundLabel({ refundRecoveryPending: true })).toBe("取消恢复扣费");
        expect(billingReviewRefundLabel()).toBe("退回预授权");
    });
});
