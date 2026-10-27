import { afterEach, expect, test } from "bun:test";
import { apiClient, ApiError } from "@/services/api/request";
import { createAdminWorkbenchPreset, listWorkbenchTools, saveAdminWorkbenchTool, setAdminWorkbenchPresetEnabled, setAdminWorkbenchToolEnabled, updateAdminWorkbenchPreset } from "@/services/api/workbench";

const original = apiClient.request;
afterEach(() => {
    apiClient.request = original;
});

test("administrative directory keeps unpublished tools and presets for review", async () => {
    let sent: any;
    const tool = { id: "tool", enabled: false, presets: [{ id: "preset", enabled: false, promptTemplate: "Review first" }] };
    apiClient.request = (async (config) => {
        sent = config;
        return { status: 200, data: { code: 0, data: { tools: [tool] }, msg: "ok" } };
    }) as typeof apiClient.request;
    const controller = new AbortController();
    expect(await listWorkbenchTools("image", controller.signal, true)).toEqual({ tools: [tool] });
    expect(sent).toMatchObject({ url: "/admin/workbench/tools", signal: controller.signal, params: { kind: "image" } });
});

test("saving an existing tool preserves IDs, prompt contents, parameters and publication states", async () => {
    let sent: any;
    apiClient.request = (async (config) => {
        sent = config;
        return { status: 200, data: { code: 0, data: { tool: { id: "tool" } }, msg: "ok" } };
    }) as typeof apiClient.request;
    await saveAdminWorkbenchTool({
        id: "tool",
        kind: "image",
        categoryId: "storyboard",
        name: "Review",
        subtitle: "",
        description: "Text",
        sortOrder: 0,
        enabled: false,
        presets: [
            { id: "old-preset", name: "Edited", promptTemplate: "Keep administrator edits", enabled: false, defaultParameters: { count: 2 } },
            { id: "published-preset", name: "Published", promptTemplate: "Published content", enabled: true },
            { id: "", name: "New", promptTemplate: "Explicit review required" },
        ],
    });
    expect(sent.method.toLowerCase()).toBe("patch");
    expect(sent.url).toBe("/admin/workbench/tools/tool");
    expect(sent.data).toMatchObject({
        enabled: false,
        presets: [
            { id: "old-preset", enabled: false, promptTemplate: "Keep administrator edits", defaultParameters: { count: 2 } },
            { id: "published-preset", enabled: true },
            { id: undefined, enabled: false },
        ],
    });
});

test("preset CRUD and publication share the parent tool path", async () => {
    const requests: any[] = [];
    apiClient.request = (async (config) => {
        requests.push(config);
        return { status: 200, data: { code: 0, data: { preset: { id: "preset" }, enabled: true }, msg: "ok" } };
    }) as typeof apiClient.request;
    const input = { toolId: "tool/one", name: "Reviewed", promptTemplate: "A scene", sortOrder: 0, enabled: false };
    await createAdminWorkbenchPreset(input);
    await updateAdminWorkbenchPreset("preset/one", input);
    expect(await setAdminWorkbenchPresetEnabled(input.toolId, "preset/one", true)).toBe(true);
    expect(requests.map((request) => request.url)).toEqual(["/admin/workbench/tools/tool%2Fone/presets", "/admin/workbench/tools/tool%2Fone/presets/preset%2Fone", "/admin/workbench/tools/tool%2Fone/presets/preset%2Fone/publish"]);
    expect(requests[0].data.enabled).toBe(false);
});

test("unpublish uses the same tool and preset identities", async () => {
    const requests: any[] = [];
    apiClient.request = (async (config) => {
        requests.push(config);
        return { status: 200, data: { code: 0, data: { enabled: false }, msg: "ok" } };
    }) as typeof apiClient.request;
    expect(await setAdminWorkbenchToolEnabled("tool", false)).toBe(false);
    expect(await setAdminWorkbenchPresetEnabled("tool", "preset", false)).toBe(false);
    expect(requests.map((request) => request.url)).toEqual(["/admin/workbench/tools/tool/unpublish", "/admin/workbench/tools/tool/presets/preset/unpublish"]);
});

test("business permission denial rejects administrative writes without retries", async () => {
    let calls = 0;
    apiClient.request = (async () => {
        calls++;
        return { status: 403, data: { code: 403, reason: "admin_permission_required", data: null, msg: "Denied" } };
    }) as typeof apiClient.request;
    await expect(setAdminWorkbenchToolEnabled("tool", true)).rejects.toBeInstanceOf(ApiError);
    expect(calls).toBe(1);
});
