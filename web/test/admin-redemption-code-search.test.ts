import { expect, test } from "bun:test";

test("admin redemption search keeps plaintext codes out of URLs and exposes audit details", async () => {
    const [panelSource, apiSource] = await Promise.all([Bun.file(new URL("../src/pages/admin/components/redemption-codes-panel.tsx", import.meta.url)).text(), Bun.file(new URL("../src/services/api/wallet.ts", import.meta.url)).text()]);

    expect(apiSource).toContain('http.post<AdminRedeemCodeSearchResult>("/admin/redeem-codes/search", { query, fundingSource, creatorId })');
    expect(apiSource).toContain('fundingSource: RedeemFundingSource');
    expect(panelSource).toContain("const result = queryKeyword ? await searchAdminRedeemBatches(params) : await listAdminRedeemBatches(params);");
    expect(panelSource).toContain("normalized.length >= 1 && normalized.length <= 32");
    expect(panelSource).toContain('placeholder="搜索兑换码片段、批次备注、积分或数量"');
    expect(panelSource).toContain('label="核销用户"');
    expect(panelSource).toContain('label="核销时间"');
    expect(panelSource).toContain('label="核销 IP"');
    expect(panelSource).toContain('item?.status === "unused"');
    expect(panelSource).toContain("disableAdminRedeemCode(batch.id, item.id, batch.fundingSource)");
});

test("redemption management separates platform and module-admin ownership", async () => {
    const [pageSource, panelSource, apiSource] = await Promise.all([
        Bun.file(new URL("../src/pages/admin/redemption-codes/redemption-codes-page.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/admin/components/redemption-codes-panel.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/services/api/wallet.ts", import.meta.url)).text(),
    ]);

    expect(pageSource).toContain('value: "platform"');
    expect(pageSource).toContain('value: "module_admin"');
    expect(pageSource).toContain('fundingSource === "platform"');
    expect(panelSource).toContain('creatorId: fundingSource === "module_admin" && isFullAdmin');
    expect(panelSource).toContain('title: "创建管理员"');
    expect(panelSource).toContain("当前可用积分");
    expect(panelSource).toContain("本批冻结积分");
    expect(apiSource).toContain('fundingSource: "platform" | "module_admin"');
    expect(apiSource).toContain("refundedMicrocredits");
});

test("redemption batch creation uses a centered modal instead of a drawer", async () => {
    const panelSource = await Bun.file(new URL("../src/pages/admin/components/redemption-codes-panel.tsx", import.meta.url)).text();
    const createSection = panelSource.slice(panelSource.indexOf("function CreateRedeemBatchModal"), panelSource.indexOf("function BatchStatusDistribution"));

    expect(createSection).toContain("<AdminModal");
    expect(createSection).toContain("centered");
    expect(createSection).toContain('rootClassName="admin-redemption-create-modal"');
    expect(createSection).not.toContain("<Drawer");
});
