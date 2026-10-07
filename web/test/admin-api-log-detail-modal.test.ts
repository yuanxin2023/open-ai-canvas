import { describe, expect, test } from "bun:test";

describe("admin request log detail modal", () => {
    test("refreshes the current request log query from the page header", async () => {
        const page = await Bun.file(new URL("../src/pages/admin/logs/logs-page.tsx", import.meta.url)).text();

        expect(page).toContain('<Button aria-label="刷新" title="刷新" icon={<RefreshCw className="size-4" />} loading={loading}');
        expect(page).toContain("onClick={() => setRetry((value) => value + 1)}");
        expect(page).toContain("[keyword, status, recordType, page, pageSize, retry, updateUrl]");
        expect(page).toContain("const autoRefreshIntervals = [30, 60, 120] as const");
        expect(page).toContain('label: "启用自动刷新"');
        expect(page).toContain("`自动刷新：${countdown} 秒`");
        expect(page).toContain("setRetry((value) => value + 1)");
        expect(page).toContain(":api-log-auto-refresh`");
    });

    test("keeps search input local while the IME is composing and commits the final keyword", async () => {
        const page = await Bun.file(new URL("../src/pages/admin/logs/logs-page.tsx", import.meta.url)).text();

        expect(page).toContain("const [keywordDraft, setKeywordDraft] = useState(keyword)");
        expect(page).toContain("useDebouncedValue(isKeywordComposing ? keyword : keywordDraft)");
        expect(page).toContain("setSearchParams((current) =>");
        expect(page).toContain("new URLSearchParams(searchParamsRef.current || current)");
        expect(page).toContain("pendingKeywordUrlRef.current === keyword");
        expect(page).toContain("value={keywordDraft}");
        expect(page).toContain("onChange={(event) => setKeywordDraft(event.target.value)}");
        expect(page).toContain("onCompositionStart={() => setIsKeywordComposing(true)}");
        expect(page).toContain("setIsKeywordComposing(false)");
        expect(page).toContain("listAdminApiLogs({ recordType, keyword: keyword || undefined");
        expect(page).not.toContain("onChange={(event) => updateUrl({ filter: event.target.value");
        expect(page).not.toContain("keyword: debouncedKeyword || undefined");
    });

    test("opens request details in a centered modal without changing detail behavior", async () => {
        const [detail, page] = await Promise.all([Bun.file(new URL("../src/pages/admin/components/api-log-detail-drawer.tsx", import.meta.url)).text(), Bun.file(new URL("../src/pages/admin/logs/logs-page.tsx", import.meta.url)).text()]);

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
