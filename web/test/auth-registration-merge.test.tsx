import { describe, expect, test } from "bun:test";
import { App } from "antd";
import { renderToStaticMarkup } from "react-dom/server";

import { VerificationFields } from "../src/components/auth/verification-fields";
import { getAuthSession, invalidateAuthSessionCache, register, type LocalUser } from "../src/services/api/auth";
import { apiClient } from "../src/services/api/request";
import { emptyVerification, verificationMethods } from "../src/services/api/verification";

const user: LocalUser = {
    id: "registered-user",
    username: "member",
    displayName: "member",
    role: "user",
    status: "active",
    usernameChangePolicy: { customized: false, limit: 3, used: 0, remaining: 3, windowDays: 30 },
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
};

describe("merged registration contract", () => {
    test("combined verification renders separate codes and sends both channels together", () => {
        const markup = renderToStaticMarkup(
            <App>
                <VerificationFields purpose="register" method="sms_email" value={{ ...emptyVerification, smsCode: "123456", emailCode: "654321" }} onChange={() => undefined} />
            </App>,
        );
        expect(markup).toContain("短信验证码");
        expect(markup).toContain("邮箱验证码");
        expect(markup).toContain('value="123456"');
        expect(markup).toContain('value="654321"');
        expect(markup.match(/获取双重验证码/g)?.length).toBe(1);
    });

    test("registration methods respect disabled SMS and exclusive combined policy", () => {
        const policy = { smsLogin: false, emailLogin: false, smsRegistration: false, emailRegistration: true, smsAndEmailRegistration: false };
        expect(verificationMethods(policy, "register")).toEqual(["email"]);
        expect(verificationMethods(policy, "login")).toEqual([]);
        expect(verificationMethods({ ...policy, emailRegistration: false }, "register")).toEqual([]);
        expect(verificationMethods({ ...policy, emailRegistration: false, smsAndEmailRegistration: true }, "register")).toEqual(["sms_email"]);
    });

    test("registration preserves consent, distinct codes and referral, then refreshes guest session", async () => {
        const originalAdapter = apiClient.defaults.adapter;
        const payload = { ticket: "verification.secret", phone: "+8613812345678", email: "member@example.com", smsCode: "123456", emailCode: "654321", password: "test-password", referralCode: "ABC234", acceptedTerms: true };
        let registered = false;
        let sessionReads = 0;
        let received: unknown;
        const selectedAvatar = { ...user, avatarMode: "custom" as const, avatarUrl: "/api/auth/profile/avatar/registered-user?v=version1" };
        invalidateAuthSessionCache();
        apiClient.defaults.adapter = async (config) => {
            let data: unknown;
            if (config.url === "/auth/session") {
                sessionReads++;
                data = { user: registered ? selectedAvatar : null };
            } else if (config.url === "/auth/register" && config.method === "post") {
                received = JSON.parse(config.data);
                registered = true;
                data = { user: selectedAvatar };
            } else throw new Error(`Unexpected request: ${config.method} ${config.url}`);
            return { config, headers: {}, status: 200, statusText: "OK", data: { code: 0, msg: "ok", data } };
        };
        try {
            expect((await getAuthSession()).user).toBeNull();
            const result = await register(payload);
            expect(received).toEqual(payload);
            expect(result.user.avatarUrl).toBe(selectedAvatar.avatarUrl);
            expect(result.user.usernameChangePolicy.customized).toBe(false);
            expect((await getAuthSession()).user?.id).toBe(user.id);
            expect(sessionReads).toBe(2);
        } finally {
            apiClient.defaults.adapter = originalAdapter;
            invalidateAuthSessionCache();
        }
    });

    test("failed registration propagates business failure and retains the guest session", async () => {
        const originalAdapter = apiClient.defaults.adapter;
        let reads = 0;
        invalidateAuthSessionCache();
        apiClient.defaults.adapter = async (config) => {
            const session = config.url === "/auth/session";
            if (session) reads++;
            return { config, headers: {}, status: 200, statusText: "OK", data: session ? { code: 0, data: { user: null }, msg: "ok" } : { code: 403, data: null, msg: "registration disabled", reason: "registration_disabled" } };
        };
        try {
            await getAuthSession();
            await expect(register({ email: "member@example.com", password: "test-password", acceptedTerms: true })).rejects.toMatchObject({ code: 403, reason: "registration_disabled" });
            expect((await getAuthSession()).user).toBeNull();
            expect(reads).toBe(1);
        } finally {
            apiClient.defaults.adapter = originalAdapter;
            invalidateAuthSessionCache();
        }
    });
});
