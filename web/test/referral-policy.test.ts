import { expect, test } from "bun:test";
import { defaultReferralPolicy, referralCredits, referralPolicySummary } from "../src/lib/referral-policy";
import { paymentOrderGrantLabel } from "../src/lib/payment-order-grant";

test("套餐返利预览使用微积分整数并截断到百分之一积分", () => {
    expect(referralCredits(1_200_000_000, 2000)).toBe(240_000_000);
    expect(referralCredits(1_200_000_000, 500)).toBe(60_000_000);
    expect(referralCredits(999_999_999_999_999, 9999)).toBe(999_899_999_990_000);
    expect(referralCredits(10_000_001, 9999)).toBe(9_990_000);
});

test("规则摘要说明每阶段及独立限制", () => {
    const text = referralPolicySummary({ ...defaultReferralPolicy, maxDays: 90, maxOrders: 10 });
    expect(text).toContain("第1—3单 20%");
    expect(text).toContain("第4—不限单 5%");
    expect(text).toContain("有效期90天");
    expect(text).toContain("限前10单");
    expect(text).toContain("最多300积分");
});

test("已入账订单分别展示套餐积分和新人赠送", () => {
    const label = paymentOrderGrantLabel({ productKind: "credits", creditsMicrocredits: 1_200_000_000, newcomerBonusMicrocredits: 60_000_000 });
    expect(label).toContain("新人赠送 60 积分");
});
