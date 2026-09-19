import { describe, expect, test } from "bun:test";

import { applyAppearanceMetadata, normalizePublicAppearance } from "../src/stores/use-appearance-store";

describe("site appearance", () => {
    test("custom site metadata, footer, and filing survive normalization", () => {
        const appearance = normalizePublicAppearance({
            brandName: "HIMA Studio",
            brandSlug: "hima-studio",
            seoTitle: "HIMA Studio - AI 影视工作台",
            seoDescription: "面向 AI 影视与短剧生产的一体化创作工作台。",
            seoKeywords: "AI 影视,短剧,画布",
            footerCopyright: "© 2026 HIMA Studio. All rights reserved.",
            icpFilingEnabled: true,
            icpFilingNumber: "蜀ICP备2026000000号-1",
        });

        expect(appearance).toMatchObject({
            schemaVersion: 8,
            brandName: "HIMA Studio",
            brandSlug: "hima-studio",
            seoTitle: "HIMA Studio - AI 影视工作台",
            seoKeywords: "AI 影视,短剧,画布",
            footerCopyright: "© 2026 HIMA Studio. All rights reserved.",
            icpFilingEnabled: true,
            icpFilingNumber: "蜀ICP备2026000000号-1",
        });
        expect(appearance).not.toHaveProperty("skinId");
        expect(appearance).not.toHaveProperty("activeSkin");
    });

    test("metadata uses fixed classic colors for light and dark mode", () => {
        const meta = new Map<string, string>();
        const icon = { href: "" };
        const target = {
            title: "",
            documentElement: { classList: { contains: () => false } },
            head: { appendChild: () => undefined },
            defaultView: undefined,
            createElement: () => {
                let key = "";
                return {
                    rel: "",
                    href: "",
                    setAttribute: (_attribute: string, value: string) => { key = value; },
                    set content(value: string) { meta.set(key, value); },
                    remove: () => meta.delete(key),
                };
            },
            querySelector: (selector: string) => {
                if (selector === 'link[rel~="icon"]') return icon;
                const match = selector.match(/meta\[(?:name|property)="([^"]+)"\]/);
                if (!match) return null;
                return { set content(value: string) { meta.set(match[1], value); }, remove: () => meta.delete(match[1]) };
            },
        } as unknown as Document;

        applyAppearanceMetadata(normalizePublicAppearance(), target);
        expect(meta.get("theme-color")).toBe("#ffffff");
        target.documentElement.classList.contains = () => true;
        applyAppearanceMetadata(normalizePublicAppearance(), target);
        expect(meta.get("theme-color")).toBe("#0a0a0a");
    });

    test("site UI keeps metadata and official ICP wiring without skin controls", async () => {
        const [storeSource, footerSource, pageSource, globalStyles, adminStyles, adminTokens] = await Promise.all([
            Bun.file(new URL("../src/stores/use-appearance-store.ts", import.meta.url)).text(),
            Bun.file(new URL("../src/components/layout/site-compliance-footer.tsx", import.meta.url)).text(),
            Bun.file(new URL("../src/pages/admin/settings/appearance-settings-page.tsx", import.meta.url)).text(),
            Bun.file(new URL("../src/styles/globals.css", import.meta.url)).text(),
            Bun.file(new URL("../src/styles/admin-ui.css", import.meta.url)).text(),
            Bun.file(new URL("../src/pages/admin/theme/admin-tokens.css", import.meta.url)).text(),
        ]);

        expect(storeSource).toContain('setMeta(targetDocument, "name", "description"');
        expect(storeSource).toContain('setMeta(targetDocument, "property", "og:title"');
        expect(footerSource).toContain("https://beian.miit.gov.cn/");
        expect(footerSource).toContain('rel="noopener noreferrer"');
        expect(pageSource).not.toContain("皮肤主题");
        expect(globalStyles).toContain("--control-switch-checked-bg: #16a34a");
        expect(globalStyles).toContain("--plugin-switch-checked-bg: var(--control-switch-checked-bg)");
        expect(adminTokens).toContain("--admin-status-warning:");
        expect(adminStyles).toContain("border-radius: var(--menu-radius);");
    });
});
