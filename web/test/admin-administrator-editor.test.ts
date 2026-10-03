import { describe, expect, test } from "bun:test";

describe("administrator editor", () => {
    test("uses four independent task tabs and minimal administrator patches", async () => {
        const [editor, panel] = await Promise.all([Bun.file(new URL("../src/pages/admin/users/administrator-edit-modal.tsx", import.meta.url)).text(), Bun.file(new URL("../src/pages/admin/users/administrators-panel.tsx", import.meta.url)).text()]);

        expect(panel).toContain("<AdministratorEditModal");
        expect(panel).not.toContain("<AdminUserEditModal");
        expect(editor).toContain('key: "profile"');
        expect(editor).toContain('key: "access"');
        expect(editor).toContain('key: "security"');
        expect(editor).toContain('key: "credits"');
        expect(editor).toContain('email: values.email?.trim() || ""');
        expect(editor).toContain("updateAdministrator(currentUser.id, { adminAccess: pendingAccess })");
        expect(editor).toContain("updateAdministrator(currentUser.id, { password: values.password })");
        expect(editor).toContain("updateAdministrator(currentUser.id, { status: pendingStatus })");
    });

    test("preserves drafts, protects self access and confirms risky operations", async () => {
        const [editor, panel] = await Promise.all([Bun.file(new URL("../src/pages/admin/users/administrator-edit-modal.tsx", import.meta.url)).text(), Bun.file(new URL("../src/pages/admin/users/administrators-panel.tsx", import.meta.url)).text()]);

        expect(editor).toContain('profileDirty ? "账号资料"');
        expect(editor).toContain('accessDirty ? "权限配置"');
        expect(editor).toContain('passwordDirty ? "安全与状态"');
        expect(editor).toContain('creditState.dirty ? "人工调账"');
        expect(editor).toContain("if (!interactionLocked) setActiveTab");
        expect(editor).toContain("editingSelf ? (");
        expect(editor).toContain("不能在这里修改自己的管理员级别或权限");
        expect(editor).toContain("核对管理员权限变更");
        expect(editor).toContain("确认停用管理员账号");
        expect(panel).toContain("editReturnFocusRef.current.focus()");
    });
});
