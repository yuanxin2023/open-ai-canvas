import { describe, expect, test } from "bun:test";
import { zhCN as libraryZh } from "../src/i18n/messages/areas/library/zh-CN";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { moduleGroupSource } from "./helpers/module-group-source";

describe("asset library category sidebar", () => {
    test("keeps type, business and folder filters in a left-hand nav", () => {
        const page = moduleGroupSource("pages/assets/index.tsx");
        const css = readFileSync(resolve(import.meta.dir, "../src/styles/workspace-product.css"), "utf8");
        expect(page).toContain('className="assets-collection-layout"');
        expect(page).toContain('aria-label={t("library.assets.page.filters")}');
        expect(libraryZh.assets.page.filters).toBe("素材分类");
        expect(page).toContain('title={t("library.assets.page.kindFilter")}');
        expect(page).toContain('title={t("library.assets.batch.category")}');
        expect(page).toContain('t("library.assets.page.myFolders")');
        expect(libraryZh.assets.page).toMatchObject({ kindFilter: "素材类型", myFolders: "我的分类" });
        expect(libraryZh.assets.batch.category).toBe("业务分类");
        expect(JSON.stringify(libraryZh)).not.toContain("全部自定义分类");
        expect(css).toMatch(/\.assets-collection-layout\s*\{[^}]*grid-template-columns:\s*220px minmax\(0, 1fr\)/s);
    });
});

describe("workspace credit products", () => {
    test("uses the editorial heading and four-column quota cards", () => {
        const component = readFileSync(resolve(import.meta.dir, "../src/components/layout/workspace-package-center.tsx"), "utf8");
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
