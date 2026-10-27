import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";

import { ALL_ADMIN_PERMISSIONS, firstAdminPath, hasAdminPermission, type AdminAccess, type AdminPermission } from "../src/lib/admin-permissions";

const newPermissions = ["admin.platform.workbench_tools", "admin.users.feedbacks", "admin.settings.sms"] as const;

describe("merged admin permissions", () => {
    test("existing grants do not implicitly authorize any new module", () => {
        const oldPermissions = ALL_ADMIN_PERMISSIONS.filter((permission) => !newPermissions.includes(permission as (typeof newPermissions)[number]));
        const scoped: AdminAccess = { level: "scoped", permissions: oldPermissions };
        for (const permission of newPermissions) {
            expect(hasAdminPermission(scoped, permission)).toBe(false);
            expect(hasAdminPermission({ level: "scoped", permissions: [] }, permission)).toBe(false);
            expect(hasAdminPermission(undefined, permission)).toBe(false);
        }
    });

    test("an explicit grant authorizes only its own new module", () => {
        for (const permission of newPermissions) {
            const scoped: AdminAccess = { level: "scoped", permissions: [permission] };
            for (const candidate of newPermissions) expect(hasAdminPermission(scoped, candidate)).toBe(candidate === permission);
            expect(hasAdminPermission(scoped, "admin.users.accounts")).toBe(false);
            expect(hasAdminPermission(scoped, "admin.settings.access")).toBe(false);
        }
    });

    test("full administrators retain access without expanding persisted grants", () => {
        const full: AdminAccess = { level: "full", permissions: [] };
        for (const permission of newPermissions) expect(hasAdminPermission(full, permission)).toBe(true);
        expect(full.permissions).toEqual([]);
    });

    test("landing routes follow the granted module and skip unmounted modules", () => {
        expect(firstAdminPath({ level: "scoped", permissions: ["admin.users.feedbacks"] })).toBe("/admin/feedbacks");
        expect(firstAdminPath({ level: "scoped", permissions: ["admin.platform.workbench_tools"] })).toBe("/admin/workbench-tools");
        expect(firstAdminPath({ level: "scoped", permissions: ["admin.settings.sms"] })).toBe("/");
        expect(firstAdminPath({ level: "scoped", permissions: ["admin.settings.sms", "admin.users.accounts"] })).toBe("/admin/users");
        expect(firstAdminPath({ level: "full", permissions: [] })).toBe("/admin");
    });

    test("the frontend and backend expose the same unique permission keys", () => {
        const source = readFileSync(new URL("../../backend/internal/model/admin_permissions.go", import.meta.url), "utf8");
        const backend = Array.from(source.matchAll(/AdminPermission\w+\s+AdminPermission\s*=\s*"(admin\.[^"]+)"/g), (match) => match[1]);
        expect(new Set(ALL_ADMIN_PERMISSIONS).size).toBe(ALL_ADMIN_PERMISSIONS.length);
        expect(new Set(backend).size).toBe(backend.length);
        expect([...backend].sort()).toEqual([...ALL_ADMIN_PERMISSIONS].sort());
        for (const permission of newPermissions) expect(ALL_ADMIN_PERMISSIONS).toContain(permission satisfies AdminPermission);
    });
});
