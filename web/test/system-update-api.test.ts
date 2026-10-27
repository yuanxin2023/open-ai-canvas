import { afterEach, expect, test } from "bun:test";
import { apiClient, ApiError } from "@/services/api/request";
import { checkSystemUpdate, getSystemUpdateStatus, rollbackSystemUpdate, startSystemUpdate } from "@/services/api/system-update";

const original = apiClient.request;
afterEach(() => {
    apiClient.request = original;
});
test("status reads do not initiate checks or upgrades", async () => {
    const calls: any[] = [];
    apiClient.request = (async (config) => {
        calls.push(config);
        return { status: 200, data: { code: 0, data: { repository: "yuanxin2023/open-ai-canvas" }, msg: "ok" } };
    }) as typeof apiClient.request;
    expect((await getSystemUpdateStatus()).repository).toBe("yuanxin2023/open-ai-canvas");
    expect(calls).toHaveLength(1);
    expect(calls[0].method.toLowerCase()).toBe("get");
});
test("check, explicit upgrade and rollback bind to the captured admin", async () => {
    const calls: any[] = [];
    apiClient.request = (async (config) => {
        calls.push(config);
        return { status: 200, data: { code: 0, data: {}, msg: "ok" } };
    }) as typeof apiClient.request;
    await checkSystemUpdate("admin-a");
    await startSystemUpdate("v1.6.9.2", "admin-a");
    await rollbackSystemUpdate(" reason ", "admin-a");
    expect(calls.map((item) => item.headers["X-Canvas-Expected-User-ID"])).toEqual(["admin-a", "admin-a", "admin-a"]);
    expect(calls[1].data).toEqual({ targetVersion: "v1.6.9.2" });
    expect(calls[2].data).toEqual({ reason: "reason" });
});
test("empty identity, non-release version and empty rollback reason never call the server", () => {
    let calls = 0;
    apiClient.request = (async () => {
        calls++;
        throw new Error("unexpected");
    }) as typeof apiClient.request;
    expect(() => checkSystemUpdate("")).toThrow();
    for (const value of ["latest", "main", "v1.2.3-rc.1", "v1.2.3+build", "v1.2"]) expect(() => startSystemUpdate(value, "admin")).toThrow();
    expect(() => rollbackSystemUpdate(" ", "admin")).toThrow();
    expect(calls).toBe(0);
});
test("update denial propagates through the http envelope", async () => {
    apiClient.request = (async () => ({ status: 200, data: { code: 403, reason: "admin_permission", msg: "denied", data: null } })) as typeof apiClient.request;
    try {
        await startSystemUpdate("v1.2.3", "admin");
        throw new Error("unexpected success");
    } catch (error) {
        expect(error).toBeInstanceOf(ApiError);
        expect((error as ApiError).reason).toBe("admin_permission");
    }
});
