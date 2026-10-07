import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

function source(path: string) {
    return readFileSync(resolve(import.meta.dir, path), "utf8");
}

describe("workspace changelog visibility", () => {
    test("hides changelog and application version from user workspace account menus", () => {
        const topbarAccount = source("../src/components/layout/workspace-account-menu.tsx");
        const sidebarAccount = source("../src/components/layout/workspace-sidebar-footer.tsx");

        for (const accountMenu of [topbarAccount, sidebarAccount]) {
            expect(accountMenu).not.toContain("AppChangelogButton");
            expect(accountMenu).not.toContain("APP_VERSION");
            expect(accountMenu).not.toContain("更新日志");
            expect(accountMenu).not.toContain("showVersion");
        }
    });

    test("keeps version and changelog access inside the admin console", () => {
        const adminShell = source("../src/pages/admin/components/admin-shell.tsx");
        const changelog = source("../src/components/layout/app-changelog-modal.tsx");

        expect(adminShell).toContain('import { AppChangelogButton } from "@/components/layout/app-changelog-modal"');
        expect(adminShell).toContain("showVersion={!collapsed}");
        expect(adminShell).toContain("admin-mobile-navigation-action");
        expect(changelog).toContain("export const APP_VERSION = __APP_VERSION__");
    });
});
