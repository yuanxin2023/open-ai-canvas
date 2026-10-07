import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { shouldShowWorkspaceCheckin, workspaceCheckinTitle } from "../src/lib/workspace-checkin";

describe("workspace checkin offer", () => {
    test("shows only when credits and a checkin bonus are enabled and today is unclaimed", () => {
        expect(shouldShowWorkspaceCheckin({ creditsEnabled: true, checkinBonusMicrocredits: 100_000_000, checkedInToday: false })).toBe(true);
        expect(shouldShowWorkspaceCheckin({ creditsEnabled: false, checkinBonusMicrocredits: 100_000_000, checkedInToday: false })).toBe(false);
        expect(shouldShowWorkspaceCheckin({ creditsEnabled: true, checkinBonusMicrocredits: 0, checkedInToday: false })).toBe(false);
        expect(shouldShowWorkspaceCheckin({ creditsEnabled: true, checkinBonusMicrocredits: 100_000_000, checkedInToday: true })).toBe(false);
    });

    test("uses the configured brand name instead of Buddy", () => {
        expect(workspaceCheckinTitle("AI 创作工作台")).toBe("AI 创作工作台加油站");
        expect(workspaceCheckinTitle(" 本地工作室 ")).toBe("本地工作室加油站");
        expect(workspaceCheckinTitle("")).toBe("AI 创作工作台加油站");
    });
});

describe("workspace top bar checkin card", () => {
    test("is anchored below the top-right actions and no longer mounted in the sidebar", () => {
        const topBar = readFileSync(resolve(import.meta.dir, "../src/components/layout/workspace-top-bar.tsx"), "utf8");
        const sidebar = readFileSync(resolve(import.meta.dir, "../src/components/layout/workspace-sidebar-nav.tsx"), "utf8");
        const card = readFileSync(resolve(import.meta.dir, "../src/components/layout/workspace-top-bar-checkin.tsx"), "utf8");
        const css = readFileSync(resolve(import.meta.dir, "../src/styles/globals.css"), "utf8");
        const start = css.indexOf(".app-user-workspace .app-workspace-topbar-checkin {");
        const end = css.indexOf(".app-user-workspace .app-workspace-sidebar-storage {", start);
        const block = css.slice(start, end);

        expect(topBar).toContain('{pathname === "/" ? <WorkspaceTopBarCheckin /> : null}');
        expect(sidebar).not.toContain("WorkspaceTopBarCheckin");
        expect(card).toContain("立即领取");
        expect(card).toContain("今日可领");
        expect(card).toContain("checkinCredits()");
        expect(block).toContain("top: calc(100% + 24px);");
        expect(block).toContain("right: 0;");
        expect(block).toContain("background: #fff;");
        expect(block).toContain("background: #171717;");
        expect(block).toContain("color: #12c8a0;");
        expect(block).not.toMatch(/#(?:dbeafe|e8f1ff|eef4ff|e6f0ff|d6e8ff)/i);
    });
});
