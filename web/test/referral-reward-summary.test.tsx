import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { ReferralPaymentSummary, ReferralUserSummary } from "../src/pages/admin/referrals/reward-summary";

describe("referral review summaries", () => {
    test("shows complete identity and empty or missing user states", () => {
        const html = renderToStaticMarkup(<ReferralUserSummary user={{ id: "full-user-id", username: "alice", email: "alice@example.com" }} userId="full-user-id" />);
        for (const value of ["@alice", "full-user-id", "alice@example.com"]) expect(html).toContain(value);
        expect(renderToStaticMarkup(<ReferralUserSummary user={{ id: "id", username: "bob", email: "" }} userId="id" />)).toContain("未填写");
        const missing = renderToStaticMarkup(<ReferralUserSummary user={null} userId="missing-id" />);
        expect(missing).toContain("用户不存在");
        expect(missing).toContain("missing-id");
    });

    test("shows merchant order, amount, current status and credited time", () => {
        const order = { id: "internal-id", userId: "invitee", merchantOrderNo: "MERCHANT-12345678901234567890", amountFen: 1234, currency: "CNY", status: "credited", creditedAt: "2026-01-02T03:04:05Z" };
        const html = renderToStaticMarkup(<ReferralPaymentSummary order={order} orderId={order.id} />);
        expect(html).toContain(order.merchantOrderNo);
        expect(html).toContain("CNY 12.34");
        expect(html).toContain("已入账");
        expect(html).toContain(new Date(order.creditedAt).toLocaleString("zh-CN", { hour12: false }));
        const pending = renderToStaticMarkup(<ReferralPaymentSummary order={{ ...order, status: "pending", creditedAt: null }} orderId={order.id} />);
        expect(pending).toContain("待支付");
        expect(pending).toContain("未入账");
        const missing = renderToStaticMarkup(<ReferralPaymentSummary order={null} orderId="missing-order-id" />);
        expect(missing).toContain("关联支付订单不存在或归属异常");
        expect(missing).toContain("missing-order-id");
    });
});
