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
