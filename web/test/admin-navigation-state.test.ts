import { describe, expect, test } from "bun:test";

import { ADMIN_NAVIGATION_GROUPS_STORAGE_KEY, DEFAULT_ADMIN_NAVIGATION_GROUP_STATE, readAdminNavigationGroupState, writeAdminNavigationGroupState } from "../src/pages/admin/lib/admin-navigation-state";

function memoryStorage(initial?: string) {
    const values = new Map<string, string>();
    if (initial !== undefined) values.set(ADMIN_NAVIGATION_GROUPS_STORAGE_KEY, initial);
    return {
        values,
        getItem: (key: string) => values.get(key) ?? null,
        setItem: (key: string, value: string) => values.set(key, value),
    };
}

describe("admin navigation group state", () => {
    test("defaults every collapsible group to open", () => {
        expect(readAdminNavigationGroupState(memoryStorage())).toEqual(DEFAULT_ADMIN_NAVIGATION_GROUP_STATE);
    });

    test("restores known preferences and defaults newly added groups", () => {
        const storage = memoryStorage(JSON.stringify({ platform: false, operations: true }));
        expect(readAdminNavigationGroupState(storage)).toEqual({ platform: false, operations: true, announcements: true, settings: true, storage: true });
    });

    test("persists group preferences", () => {
        const storage = memoryStorage();
        const state = { platform: false, operations: true, announcements: false, settings: false, storage: true };

        writeAdminNavigationGroupState(state, storage);

        expect(JSON.parse(storage.values.get(ADMIN_NAVIGATION_GROUPS_STORAGE_KEY) || "{}")).toEqual(state);
    });

    test("falls back safely when storage is unavailable or malformed", () => {
        const blockedStorage = {
            getItem: () => {
                throw new Error("blocked");
            },
            setItem: () => {
                throw new Error("blocked");
            },
        };

        expect(readAdminNavigationGroupState(memoryStorage("not-json"))).toEqual(DEFAULT_ADMIN_NAVIGATION_GROUP_STATE);
        expect(readAdminNavigationGroupState(blockedStorage)).toEqual(DEFAULT_ADMIN_NAVIGATION_GROUP_STATE);
        expect(() => writeAdminNavigationGroupState(DEFAULT_ADMIN_NAVIGATION_GROUP_STATE, blockedStorage)).not.toThrow();
    });
});
