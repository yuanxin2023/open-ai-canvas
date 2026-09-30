import { describe, expect, test } from "bun:test";

describe("admin credit reconciliation request details", () => {
    test("opens the exact billable request from a reconciliation row", async () => {
        const [panel, modal, api] = await Promise.all([
            Bun.file(new URL("../src/pages/admin/components/credit-operations-panel.tsx", import.meta.url)).text(),
            Bun.file(new URL("../src/pages/admin/components/api-log-detail-drawer.tsx", import.meta.url)).text(),
            Bun.file(new URL("../src/services/api/auth.ts", import.meta.url)).text(),
        ]);

        expect(panel).toContain('title: "创建时间 / 详情"');
        expect(panel).toContain("setDetailBillingOrderId(order.id)");
        expect(panel).toContain('className: "admin-table-clickable-row"');
        expect(panel).toContain('<ApiLogDetailModal billingOrderId={detailBillingOrderId}');
        expect(modal).toContain("getAdminApiLogByBillingOrder(billingOrderId!)");
        expect(api).toContain("/admin/billing-orders/${encodeURIComponent(billingOrderId)}/api-log");
    });
});
