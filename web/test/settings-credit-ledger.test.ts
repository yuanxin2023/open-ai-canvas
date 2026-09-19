import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

function source(path: string) {
    return readFileSync(resolve(import.meta.dir, path), "utf8");
}

describe("settings credit ledger", () => {
    test("places the feature-gated ledger directly after payment orders", () => {
        const settings = source("../src/pages/settings/index.tsx");
        const orders = settings.indexOf('{ key: "orders", label: "我的订单"');
        const wallet = settings.indexOf('{ key: "wallet", label: "积分流水"');
        const channels = settings.indexOf('{ key: "channels", label: "个人渠道"');

        expect(orders).toBeGreaterThan(-1);
        expect(wallet).toBeGreaterThan(orders);
        expect(channels).toBeGreaterThan(wallet);
        expect(settings).toContain('["wallet", "orders"].includes(section.key) || creditsEnabled');
        expect(settings).toContain("wallet: <SettingsPane><CreditLedgerPane /></SettingsPane>");
    });

    test("opens the first visible settings section by default", () => {
        const settings = source("../src/pages/settings/index.tsx");

        expect(settings).toContain('const firstVisibleSection = visibleConfigSections[0]?.key || "models"');
        expect(settings).toContain("isVisibleConfigSection(requestedSection) ? requestedSection : firstVisibleSection");
        expect(settings).toContain("setActiveTab(firstVisibleSection)");
        expect(settings).not.toContain('requestedSectionEnabled ? requestedSection : customChannelsEnabled ? "channels" : "models"');
    });

    test("keeps horizontal category scrolling on mobile only", () => {
        const settings = source("../src/pages/settings/index.tsx");

        expect(settings).toContain("overflow-x-auto");
        expect(settings).toContain("md:overflow-x-hidden");
    });

    test("renders settings categories without secondary descriptions", () => {
        const settings = source("../src/pages/settings/index.tsx");

        expect(settings).not.toContain("item.description");
        expect(settings).not.toContain("description: \"查看充值记录与取消订单\"");
        expect(settings).not.toContain("description: \"按领域选择默认模型\"");
    });

    test("uses the wallet API with filters, paging, readable types and balances", () => {
        const pane = source("../src/pages/settings/credit-ledger-pane.tsx");

        for (const filter of ["all", "income", "consume", "refund"]) {
            expect(pane).toContain(`value: "${filter}"`);
        }
        for (const label of ["兑换充值", "在线充值", "管理员充值", "模型消费", "消费退款", "管理员调账", "注册奖励", "签到奖励"]) {
            expect(pane).toContain(`label: "${label}"`);
        }
        expect(pane).toContain("getWallet(targetPage, targetPageSize, targetFilter)");
        expect(pane).toContain('title: "积分变化"');
        expect(pane).toContain('title: "变更后余额"');
        expect(pane).toContain('value > 0 ? "+" : ""');
        expect(pane).toContain("modelDisplayName(config, entry.model)");
        expect(pane).toContain("entry.note");
        expect(pane).toContain("requestSequence.current");
        expect(pane).toContain("pageSizeOptions={[20, 50, 100]}");
        expect(pane).toContain("scroll={{ x: 1000 }}");
    });
});
