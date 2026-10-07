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

test("profile renders login username rules and explains the change quota", async () => {
    const source = await Bun.file(new URL("../src/pages/profile/index.tsx", import.meta.url)).text();

    expect(source).toContain("登录用户名");
    expect(source).toContain("用户名为 3–9 位，可使用中文、英文字母和数字");
    expect(source).toContain("这是首次自选用户名，不计入修改限额。");
    expect(source).toContain("policy.customized");
    expect(source).toContain("policy.windowDays");
    expect(source).toContain("policy.limit");
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
