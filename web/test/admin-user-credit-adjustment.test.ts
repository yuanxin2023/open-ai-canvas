import { describe, expect, test } from "bun:test";

describe("admin user credit adjustment entry", () => {
    test("reuses the existing adjustment API from the user management modal", async () => {
        const [drawer, fields, administratorEditor, panel, walletApi] = await Promise.all([
            Bun.file(new URL("../src/pages/admin/users/users-drawer.tsx", import.meta.url)).text(),
            Bun.file(new URL("../src/pages/admin/users/admin-user-editor-fields.tsx", import.meta.url)).text(),
            Bun.file(new URL("../src/pages/admin/users/administrator-edit-modal.tsx", import.meta.url)).text(),
            Bun.file(new URL("../src/pages/admin/users/users-panel.tsx", import.meta.url)).text(),
            Bun.file(new URL("../src/services/api/wallet.ts", import.meta.url)).text(),
        ]);

        expect(fields).toContain('label="积分变化"');
        expect(fields).toContain('label="调整原因"');
        expect(fields).toContain("adjustAdminUserCredits(user.id");
        expect(fields).toContain("确认扣减用户积分");
        expect(fields).toContain("确认增加用户积分");
        expect(fields).toContain("扣减后可用积分不能低于 0");
        expect(drawer).toContain("<AdminCreditAdjustmentPanel");
        expect(administratorEditor).toContain("<AdminCreditAdjustmentPanel");
        expect(administratorEditor).toContain("调账独立提交，不会连带保存账号资料、权限或密码草稿");
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
