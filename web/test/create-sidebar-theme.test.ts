import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

describe("create page sidebar theme", () => {
    test("keeps the shared workspace sidebar surface", () => {
        const styles = readFileSync(resolve(import.meta.dir, "../src/styles/globals.css"), "utf8");

        expect(styles).not.toContain(".app-workspace-shell.is-creation-workspace .app-workspace-sidebar");
        expect(styles).toContain(".app-workspace-shell.is-creation-workspace .app-workspace-stage");
        expect(styles).toContain("background: color-mix(in srgb, var(--workspace-sidebar) 96%, var(--foreground) 4%);");
        expect(styles.match(/--workspace-sidebar: var\(--workspace-navigation\);/g)?.length).toBeGreaterThanOrEqual(4);
    });

    test("separates the user sidebar from the page with theme-aware neutral surfaces", () => {
        const styles = readFileSync(resolve(import.meta.dir, "../src/styles/globals.css"), "utf8");

        expect(styles).toContain("--user-page-bg: #f8f8fa;\n    --user-sidebar-bg: #f3f3f5;");
        expect(styles).toContain("--user-page-bg: #151517;\n    --user-sidebar-bg: #1c1c1f;");
        expect(styles).toContain("--user-context-panel-bg: #fcfcfd;");
        expect(styles).toContain("--user-context-panel-bg: #202024;");
        expect(styles).toContain("border-right: 1px solid var(--user-panel-divider) !important;");
        expect(styles).toContain("--workspace-sidebar: var(--user-sidebar-bg);");
        expect(styles).toContain(".app-user-workspace .app-workspace-sidebar-nav { background: var(--user-sidebar-bg) !important; }");
    });

    test("keeps the workspace brand link free of a filled click state", () => {
        const styles = readFileSync(resolve(import.meta.dir, "../src/styles/globals.css"), "utf8");

        expect(styles).toContain(".app-workspace-sidebar-brand-button:active,");
        expect(styles).toContain("-webkit-tap-highlight-color: transparent;");
        expect(styles).toContain("background: transparent;");
        expect(styles).toContain(".app-workspace-sidebar-brand-button:focus-visible {");
        expect(styles).toContain("outline: 2px solid var(--user-accent);");
    });

    test("keeps a visible surface on the selected navigation item", () => {
        const styles = readFileSync(resolve(import.meta.dir, "../src/styles/globals.css"), "utf8");
        const productStyles = readFileSync(resolve(import.meta.dir, "../src/styles/workspace-product.css"), "utf8");
        const navigation = readFileSync(resolve(import.meta.dir, "../src/components/layout/workspace-sidebar-nav.tsx"), "utf8");

        expect(styles).toContain(".app-user-workspace .app-workspace-nav-link.is-active { color: var(--user-ink) !important;");
        expect(productStyles).toContain(".app-user-workspace .app-workspace-nav-active-pill {");
        expect(productStyles).toContain("background: var(--workspace-nav-active-bg);");
        expect(productStyles).toContain(".app-user-workspace .app-workspace-nav-link.is-active:hover .app-workspace-nav-active-pill");
        expect(productStyles).toContain("background: var(--workspace-nav-active-hover-bg);");
        expect(navigation).toContain('selected ? <span className="app-workspace-nav-active-pill" aria-hidden /> : null');
        expect(navigation).not.toContain('background: "var(--workspace-nav-active-bg)"');
    });

    test("centers a single-line workspace brand beside the logo", () => {
        const styles = readFileSync(resolve(import.meta.dir, "../src/styles/globals.css"), "utf8");
        const navigation = readFileSync(resolve(import.meta.dir, "../src/components/layout/workspace-sidebar-nav.tsx"), "utf8");

        expect(navigation).not.toContain("创作工作台");
        expect(navigation).toContain('className="flex min-w-0 items-center gap-2"');
        expect(styles).toContain(".app-user-workspace .app-workspace-brand-wordmark {\n    font-size: 16px;");
        expect(styles).toContain("font-weight: 600;");
        expect(styles).not.toContain(".app-workspace-brand-subtitle");
    });
});
