import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

describe("asset library category sidebar", () => {
    test("keeps type, business and folder filters in a left-hand nav", () => {
        const page = readFileSync(resolve(import.meta.dir, "../src/pages/assets/index.tsx"), "utf8");
        const css = readFileSync(resolve(import.meta.dir, "../src/styles/workspace-product.css"), "utf8");
        expect(page).toContain('className="assets-collection-layout"');
        expect(page).toContain('aria-label="素材分类"');
        expect(page).toContain('title="素材类型"');
        expect(page).toContain('title="业务分类"');
        expect(page).toContain("我的分类");
        expect(page).not.toContain("全部自定义分类");
        expect(css).toMatch(/\.assets-collection-layout\s*\{[^}]*grid-template-columns:\s*220px minmax\(0, 1fr\)/s);
    });
});

describe("wallet history pagination", () => {
    test("pins ledger pagination to the history panel footer", () => {
        const modal = readFileSync(resolve(import.meta.dir, "../src/components/layout/workspace-wallet-modal.tsx"), "utf8");
        const css = readFileSync(resolve(import.meta.dir, "../src/styles/globals.css"), "utf8");
        expect(modal).toContain("workspace-wallet-history-scroll");
        expect(modal).toContain("workspace-wallet-pagination");
        expect(modal).not.toContain("wallet.total > 20");
        expect(css).toMatch(/\.workspace-wallet-content\.is-history\s*\{[^}]*overflow:\s*hidden/s);
        expect(css).toMatch(/\.workspace-wallet-pagination\s*\{[^}]*margin-top:\s*auto/s);
    });
});

describe("workspace credit products", () => {
    test("uses the editorial heading and four-column quota cards", () => {
        const component = readFileSync(resolve(import.meta.dir, "../src/components/layout/workspace-credit-popover.tsx"), "utf8");
        const card = readFileSync(resolve(import.meta.dir, "../src/components/payments/credit-product-card.tsx"), "utf8");
        const css = readFileSync(resolve(import.meta.dir, "../src/styles/globals.css"), "utf8");
        expect(component).toContain("选择您的套餐");
        expect(component).toContain("products.map((product)");
        expect(card).toContain("Boolean(product.featured)");
        expect(card).toContain('product.actionText || "立即购买"');
        expect(component).toContain("<CreditProductCard");
        expect(component).toContain('className="workspace-credit-products-shell"');
        expect(card).toContain("workspace-credit-product-meter");
        expect(css).toMatch(/\.workspace-credit-products-shell::-webkit-scrollbar\s*\{[^}]*width:\s*6px/s);
        expect(css).toMatch(/\.workspace-credit-products-shell:hover,[^}]*--workspace-credit-scrollbar-thumb:/s);
        expect(css).toMatch(/@supports selector\(::-webkit-scrollbar\)\s*\{\s*\.workspace-credit-products-shell\s*\{[^}]*scrollbar-color:\s*auto;[^}]*scrollbar-width:\s*auto;/s);
        expect(card).toContain("product.ribbonText");
        expect(css).toMatch(/\.workspace-credit-products-grid\s*\{[^}]*grid-template-columns:\s*repeat\(4, minmax\(0, 1fr\)\)/s);
        expect(css).toMatch(/\.workspace-credit-products-header h2\s*\{[^}]*Source Han Serif SC/s);
        expect(css).toMatch(/\.workspace-credit-product-meter\s*\{[^}]*repeat\(28, minmax\(2px, 1fr\)\)/s);
    });
});
