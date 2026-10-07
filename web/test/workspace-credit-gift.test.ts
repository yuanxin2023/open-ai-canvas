import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

describe("workspace credit entry", () => {
    test("top bars share the credit popover and its balance trigger", () => {
        const topBar = readFileSync(resolve(import.meta.dir, "../src/components/layout/workspace-top-bar.tsx"), "utf8");
        const canvas = readFileSync(resolve(import.meta.dir, "../src/pages/canvas/canvas-project-top-bar.tsx"), "utf8");
        const popover = readFileSync(resolve(import.meta.dir, "../src/components/layout/workspace-credit-popover.tsx"), "utf8");
        const css = readFileSync(resolve(import.meta.dir, "../src/styles/globals.css"), "utf8");

        expect(topBar).toContain("<WorkspaceCreditPopover userId={user.id} />");
        expect(topBar).not.toContain("Coins");
        expect(canvas).toContain("<WorkspaceCreditPopover userId={user.id} />");
        expect(canvas).not.toContain("Coins");
        expect(popover).toContain('className="app-workspace-credit-button"');
        expect(popover).toContain("<Sparkles aria-hidden />");
        expect(css).toContain(".app-workspace-credit-button");
    });
});
