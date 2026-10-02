import { describe, expect, test } from "bun:test";

describe("admin user credit adjustment entry", () => {
    test("reuses the existing adjustment API from the user management modal", async () => {
        const [drawer, panel, walletApi] = await Promise.all([
            Bun.file(new URL("../src/pages/admin/users/users-drawer.tsx", import.meta.url)).text(),
            Bun.file(new URL("../src/pages/admin/users/users-panel.tsx", import.meta.url)).text(),
            Bun.file(new URL("../src/services/api/wallet.ts", import.meta.url)).text(),
        ]);

        expect(drawer).toContain('label="积分变化"');
        expect(drawer).toContain('label="调整原因"');
        expect(drawer).toContain("adjustAdminUserCredits(user.id");
        expect(drawer).toContain("确认扣减用户积分");
        expect(drawer).toContain("确认增加用户积分");
        expect(drawer).toContain("扣减后可用积分不能低于 0");
        expect(panel).toContain("availableMicrocredits: account.availableMicrocredits");
        expect(walletApi).toContain("/credits/adjust");
    });

    test("keeps the user search draft local until IME composition and debounce finish", async () => {
        const [panel, urlState] = await Promise.all([
            Bun.file(new URL("../src/pages/admin/users/users-panel.tsx", import.meta.url)).text(),
            Bun.file(new URL("../src/pages/admin/lib/use-table-url-state.ts", import.meta.url)).text(),
        ]);

        expect(panel).toContain("const [filterDraft, setFilterDraft] = useState(state.filter)");
        expect(panel).toContain("useDebouncedValue(isFilterComposing ? state.filter : filterDraft)");
        expect(panel).toContain("value={filterDraft}");
        expect(panel).toContain("onChange={(event) => setFilterDraft(event.target.value)}");
        expect(panel).toContain("onCompositionStart={() => setIsFilterComposing(true)}");
        expect(panel).toContain("keyword: state.filter || undefined");
        expect(panel).not.toContain("onChange={(event) => update({ filter: event.target.value");
        expect(urlState).toContain("setSearchParams((current) =>");
        expect(urlState).toContain("new URLSearchParams(searchParamsRef.current || current)");
    });
});
