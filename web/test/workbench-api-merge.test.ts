import { afterEach, expect, test } from "bun:test";
import { cancelGenerationTask, createGenerationTask, type GenerationTask } from "@/services/api/task-center";
import { apiClient, ApiError } from "@/services/api/request";
import { createWorkbenchTask, getWorkbenchSubmission, getWorkbenchRetryIntent, type WorkbenchTaskIntent } from "@/services/api/workbench";
import { setActiveUserScope } from "@/lib/user-scope";

const original = apiClient.request;
const originalWindow = Object.getOwnPropertyDescriptor(globalThis, "window");
afterEach(() => {
    apiClient.request = original;
    if (originalWindow) Object.defineProperty(globalThis, "window", originalWindow);
    else Reflect.deleteProperty(globalThis, "window");
});
const input: WorkbenchTaskIntent = {
    kind: "image",
    prompt: "test",
    model: "image-model",
    channelId: "system",
    parameters: { size: "1024x1024", quality: "high", imageResolution: "2k", count: 2 },
    references: [{ resourceId: "owned-image", kind: "image", role: "reference" }],
};
const task: GenerationTask = { id: "created-task", type: "canvas_image", status: "queued", prompt: "test", attempts: 0, createdAt: "2026-10-26T00:00:00Z", updatedAt: "2026-10-26T00:00:00Z" };

test("workbench uses http and preserves the server task receipt and pricing inputs", async () => {
    let sent: any;
    apiClient.request = (async (config) => {
        sent = config;
        return { status: 200, data: { code: 0, data: task, msg: "ok" } };
    }) as typeof apiClient.request;
    expect(await createWorkbenchTask(input)).toEqual(task);
    expect(sent.url).toBe("/workbench/tasks");
    expect(sent.method.toLowerCase()).toBe("post");
    expect(sent.data).toEqual(input);
});

test("HTTP 200 with a business denial propagates without retrying the creation", async () => {
    let calls = 0;
    apiClient.request = (async () => {
        calls++;
        return { status: 200, data: { code: 403, reason: "feature_disabled", data: null, msg: "closed" } };
    }) as typeof apiClient.request;
    try {
        await createWorkbenchTask(input);
        throw new Error("expected rejection");
    } catch (error) {
        expect(error).toBeInstanceOf(ApiError);
        expect((error as ApiError).reason).toBe("feature_disabled");
    }
    expect(calls).toBe(1);
});

function accountEvents() {
    const values = new Map<string, string>();
    const events: string[] = [];
    Object.defineProperty(globalThis, "window", {
        configurable: true,
        value: {
            localStorage: { getItem: (key: string) => values.get(key) ?? null, setItem: (key: string, value: string) => values.set(key, value) },
            dispatchEvent: (event: Event) => {
                events.push(event.type);
                return true;
            },
        },
    });
    setActiveUserScope("account-a");
    return events;
}

test("generic task creation publishes the receipt and wallet refresh in the same account", async () => {
    const events = accountEvents();
    apiClient.request = (async () => ({ status: 200, data: { code: 0, data: { ...task, projectId: "canvas-a" }, msg: "ok" } })) as typeof apiClient.request;
    expect((await createGenerationTask({ type: "canvas_image", prompt: "test", input: { mode: "image" } })).id).toBe(task.id);
    expect(events).toEqual(["canvas:task-created", "wallet:updated"]);
});

test("generic task response after an account switch cannot publish into the new account", async () => {
    const events = accountEvents();
    apiClient.request = (async () => {
        setActiveUserScope("account-b");
        return { status: 200, data: { code: 0, data: { ...task, projectId: "canvas-a" }, msg: "ok" } };
    }) as typeof apiClient.request;
    expect((await createGenerationTask({ type: "canvas_image", prompt: "test", input: { mode: "image" } })).id).toBe(task.id);
    expect(events).toEqual([]);
});

test("old-account credit failures still reject without notifying the new account", async () => {
    const events = accountEvents();
    apiClient.request = (async () => {
        setActiveUserScope("account-b");
        return { status: 200, data: { code: 403, data: null, msg: "insufficient", reason: "insufficient_credits" } };
    }) as typeof apiClient.request;
    await expect(createGenerationTask({ type: "canvas_image", prompt: "test", input: { mode: "image" } })).rejects.toBeInstanceOf(ApiError);
    expect(events).toEqual([]);
});

test("cancellation completed after an account switch cannot publish old task data", async () => {
    const events = accountEvents();
    apiClient.request = (async () => {
        setActiveUserScope("account-b");
        return { status: 200, data: { code: 0, data: { ...task, status: "cancelled" }, msg: "ok" } };
    }) as typeof apiClient.request;
    expect((await cancelGenerationTask(task.id)).status).toBe("cancelled");
    expect(events).toEqual([]);
});

test("workbench request key reaches POST and receipt lookup only performs GET", async () => {
    const requests: Array<{ method: string; url: string; data?: unknown }> = [];
    apiClient.request = (async (config) => {
        requests.push(config as any);
        return { status: 200, data: { code: 0, data: task, msg: "ok" } };
    }) as typeof apiClient.request;
    await createWorkbenchTask({ ...input, requestKey: "original-request-key" });
    expect(requests[0]?.data).toMatchObject({ requestKey: "original-request-key" });
    expect(await getWorkbenchSubmission("original-request-key")).toEqual(task);
    expect(requests[1]?.method.toLowerCase()).toBe("get");
    expect(requests[1]?.url).toBe("/workbench/submissions/original-request-key");
    expect(requests).toHaveLength(2);
});

test("a missing receipt propagates its reason and does not create a replacement", async () => {
    let calls = 0;
    apiClient.request = (async () => {
        calls++;
        return { status: 404, data: { code: 404, reason: "workbench_submission_missing", data: null, msg: "not confirmed" } };
    }) as typeof apiClient.request;
    try {
        await getWorkbenchSubmission("original-request-key");
        throw new Error("expected rejection");
    } catch (error) {
        expect((error as ApiError).reason).toBe("workbench_submission_missing");
    }
    expect(calls).toBe(1);
});

test("loading retry parameters is an authenticated GET with cancellation support", async () => {
    const controller = new AbortController();
    let sent: any;
    const intent = { task, retryOf: task.id, retryVersion: "a".repeat(64) };
    apiClient.request = (async (config) => {
        sent = config;
        return { status: 200, data: { code: 0, data: intent, msg: "ok" } };
    }) as typeof apiClient.request;
    expect(await getWorkbenchRetryIntent(task.id, controller.signal)).toEqual(intent);
    expect(sent.url).toBe("/workbench/tasks/created-task/retry-intent");
    expect(sent.method.toLowerCase()).toBe("get");
    expect(sent.signal).toBe(controller.signal);
});
