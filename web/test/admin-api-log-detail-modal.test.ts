import { describe, expect, test } from "bun:test";

describe("admin request log detail modal", () => {
    test("opens request details in a centered modal without changing detail behavior", async () => {
        const [detail, page] = await Promise.all([
            Bun.file(new URL("../src/pages/admin/components/api-log-detail-drawer.tsx", import.meta.url)).text(),
            Bun.file(new URL("../src/pages/admin/logs/logs-page.tsx", import.meta.url)).text(),
        ]);

        expect(detail).toContain("export function ApiLogDetailModal");
        expect(detail).toContain("<AdminModal");
        expect(detail).toContain("centered");
        expect(detail).toContain('title="请求详情"');
        expect(detail).toContain('maxHeight: "min(76vh, 760px)"');
        expect(detail).not.toContain("<Drawer");
        expect(detail).toContain("queryAdminApiLogTask");
        expect(detail).toContain("复制报文");
        expect(page).toContain("<ApiLogDetailModal");
    });
});
