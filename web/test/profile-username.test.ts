import { expect, test } from "bun:test";

test("profile username changes require confirmation and cancellation restores only the username draft", async () => {
    const source = await Bun.file(new URL("../src/pages/profile/index.tsx", import.meta.url)).text();

    expect(source).toContain('title: "确认修改用户名？"');
    expect(source).toContain('okText: "确定修改"');
    expect(source).toContain('cancelText: "取消"');
    expect(source).toContain("原用户名立即失效");
    expect(source).toContain("user.email");
    expect(source).toContain("usernameChangeQuotaText(user)");
    expect(source).toContain("onCancel: () => setUsername(initialUsername)");
    expect(source).not.toContain("onCancel: () => setAvatarResourceId");
    expect(source).toContain("if (!usernameChanged) {");
    expect(source).toContain("void saveProfile();");
});

test("profile renders login username rules and remaining change quota", async () => {
    const source = await Bun.file(new URL("../src/pages/profile/index.tsx", import.meta.url)).text();

    expect(source).toContain("登录用户名");
    expect(source).toContain("含中文时 2-6 位，其他情况 3-6 位");
    expect(source).toContain("首次将系统生成的用户名改为自选名称不计入限额");
    expect(source).toContain("policy.remaining");
    expect(source).toContain("policy.nextAvailableAt");
    expect(source).toContain("管理员修改用户名不受次数限制");
});

test("account identity surfaces use username instead of deprecated display aliases", async () => {
    const paths = [
        "../src/components/layout/workspace-account-card.tsx",
        "../src/components/layout/workspace-account-menu.tsx",
        "../src/components/layout/workspace-sidebar-footer.tsx",
        "../src/pages/create/creation-workspace.tsx",
        "../src/pages/admin/users/users-columns.tsx",
        "../src/pages/admin/users/users-drawer.tsx",
    ];
    const sources = await Promise.all(paths.map((path) => Bun.file(new URL(path, import.meta.url)).text()));
    for (const [index, source] of sources.entries()) {
        expect(source).not.toContain("user.displayName", paths[index]);
        expect(source).not.toContain("user.profileName", paths[index]);
    }
});
