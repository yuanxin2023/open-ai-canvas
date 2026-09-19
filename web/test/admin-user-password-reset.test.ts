import { describe, expect, test } from "bun:test";

import { generateAdminPassword } from "../src/pages/admin/users/admin-password";

describe("admin user password reset", () => {
    test("generates a secure 16-character password with every required character class", () => {
        for (let index = 0; index < 50; index += 1) {
            const password = generateAdminPassword();
            expect(password).toHaveLength(16);
            expect(password).toMatch(/[A-Z]/);
            expect(password).toMatch(/[a-z]/);
            expect(password).toMatch(/[0-9]/);
            expect(password).toMatch(/[!@#$%&*+\-_]/);
        }
    });

    test("exposes custom input, copy, generation and optional update behavior in the edit drawer", async () => {
        const drawer = await Bun.file(new URL("../src/pages/admin/users/users-drawer.tsx", import.meta.url)).text();
        const api = await Bun.file(new URL("../src/services/api/auth.ts", import.meta.url)).text();

        expect(drawer).toContain('label="修改密码"');
        expect(drawer).toContain('aria-label="复制密码"');
        expect(drawer).toContain('aria-label="随机生成 16 位密码"');
        expect(drawer).toContain("generateAdminPassword(16)");
        expect(drawer).toContain('...(password ? { password } : {})');
        expect(drawer).toContain("修改后会清除该用户当前的全部登录状态");
        expect(api).toContain('& { password?: string }');
    });
});
