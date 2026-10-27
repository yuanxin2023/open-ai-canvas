import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";

import {
    readWorkspaceSidebarCollapsed,
    WORKSPACE_SIDEBAR_STORAGE_KEY,
    writeWorkspaceSidebarCollapsed,
} from "../src/components/layout/workspace-sidebar-state";

const sidebarSource = readFileSync(new URL("../src/components/layout/workspace-sidebar-nav.tsx", import.meta.url), "utf8");

function memoryStorage(initial?: string) {
    const values = new Map<string, string>();
    if (initial !== undefined) values.set(WORKSPACE_SIDEBAR_STORAGE_KEY, initial);
    return {
        values,
        getItem: (key: string) => values.get(key) ?? null,
        setItem: (key: string, value: string) => values.set(key, value),
    };
}

describe("workspace sidebar state", () => {
    test("keeps visible collapse and expand controls wired to their callbacks", () => {
        expect(sidebarSource).toContain('aria-label="收起侧栏"');
        expect(sidebarSource).toContain("onClick={onCollapse}");
        expect(sidebarSource).toContain('aria-label="展开侧栏菜单"');
        expect(sidebarSource).toContain("onClick={onExpand}");
    });

    test("restores the persisted collapsed state after refresh", () => {
        expect(readWorkspaceSidebarCollapsed(memoryStorage("1"))).toBe(true);
        expect(readWorkspaceSidebarCollapsed(memoryStorage("0"))).toBe(false);
        expect(readWorkspaceSidebarCollapsed(memoryStorage())).toBe(false);
    });

    test("persists both collapsed and expanded states", () => {
        const storage = memoryStorage();

        writeWorkspaceSidebarCollapsed(true, storage);
        expect(storage.values.get(WORKSPACE_SIDEBAR_STORAGE_KEY)).toBe("1");

        writeWorkspaceSidebarCollapsed(false, storage);
        expect(storage.values.get(WORKSPACE_SIDEBAR_STORAGE_KEY)).toBe("0");
    });

    test("keeps sidebar interaction available when storage is blocked", () => {
        const blockedStorage = {
            getItem: () => {
                throw new Error("blocked");
            },
            setItem: () => {
                throw new Error("blocked");
            },
        };

        expect(readWorkspaceSidebarCollapsed(blockedStorage)).toBe(false);
        expect(() => writeWorkspaceSidebarCollapsed(true, blockedStorage)).not.toThrow();
    });
});
