import { describe, expect, test } from "bun:test";

import { buildUserProfilePatch } from "../src/pages/admin/users/user-profile-patch";

describe("ordinary user profile patch", () => {
    const current = { email: "user@example.com", remark: "客户" };

    test("omits unchanged values and whitespace-only changes", () => {
        expect(buildUserProfilePatch(current, current)).toEqual({});
        expect(buildUserProfilePatch(current, { email: " user@example.com ", remark: " 客户 " })).toEqual({});
    });

    test("editing email does not submit an unchanged remark", () => {
        expect(buildUserProfilePatch(current, { ...current, email: " new@example.com " })).toEqual({ email: "new@example.com" });
    });

    test("editing or clearing remark does not submit email", () => {
        expect(buildUserProfilePatch(current, { ...current, remark: " 大客户 " })).toEqual({ remark: "大客户" });
        expect(buildUserProfilePatch(current, { ...current, remark: " " })).toEqual({ remark: "" });
    });

    test("accepts absent email and excludes password and status drafts", () => {
        expect(buildUserProfilePatch({ remark: "" }, { email: "", remark: "" })).toEqual({});
        const draft = { email: "new@example.com", remark: "新备注", password: "unsaved-password", status: "disabled" };
        expect(buildUserProfilePatch(current, draft)).toEqual({ email: "new@example.com", remark: "新备注" });
    });
});
