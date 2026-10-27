import { afterEach, expect, test } from "bun:test";
import { apiClient, ApiError } from "@/services/api/request";
import { archiveAdminFeedback, deleteAdminFeedback, feedbackAttachmentURL, submitFeedback, updateAdminFeedback, uploadFeedbackAttachment } from "@/services/api/feedback";
import { safeFeedbackPathname } from "@/lib/feedback-environment";

const original = apiClient.request;
afterEach(() => {
    apiClient.request = original;
});
const payload = { requestId: "feedback-request-001", type: "bug" as const, title: "test", content: "body", environment: "{}", attachments: "[]" };

test("feedback carries expected actor and stable request ID through http", async () => {
    let sent: any;
    apiClient.request = (async (config) => {
        sent = config;
        return { status: 200, data: { code: 0, data: { id: "fb", version: 1 }, msg: "ok" } };
    }) as typeof apiClient.request;
    expect((await submitFeedback(payload, "actor-a")).id).toBe("fb");
    expect(sent.url).toBe("/feedbacks");
    expect(sent.data.requestId).toBe(payload.requestId);
    expect(sent.headers["X-Canvas-Expected-User-ID"]).toBe("actor-a");
});

test("feedback admin mutations encode IDs and send captured versions including DELETE", async () => {
    const sent: any[] = [];
    apiClient.request = (async (config) => {
        sent.push(config);
        return { status: 200, data: { code: 0, data: { ok: true }, msg: "ok" } };
    }) as typeof apiClient.request;
    await updateAdminFeedback("fb/id", { adminReply: "" }, "admin-a", 4);
    await archiveAdminFeedback("fb/id", true, "admin-a", 5);
    await deleteAdminFeedback("fb/id", "admin-a", 6);
    expect(sent.map((item) => item.headers["X-Canvas-Feedback-Version"])).toEqual(["4", "5", "6"]);
    expect(sent.map((item) => item.url)).toEqual(["/admin/feedbacks/fb%2Fid", "/admin/feedbacks/fb%2Fid/archive", "/admin/feedbacks/fb%2Fid"]);
    expect(sent[0].data.adminReply).toBe("");
    expect(sent[2].method.toLowerCase()).toBe("delete");
    expect(sent[2].data).toBeUndefined();
});

test("invalid actor or version cannot issue a mutation", async () => {
    let calls = 0;
    apiClient.request = (async () => {
        calls++;
        throw new Error("unexpected transport");
    }) as typeof apiClient.request;
    await expect(submitFeedback(payload, "")).rejects.toThrow("请先登录");
    for (const version of [0, -1, Number.NaN, 1.5]) await expect(deleteAdminFeedback("fb", "admin", version)).rejects.toThrow("版本无效");
    expect(calls).toBe(0);
});

test("upload leaves multipart Content-Type to the browser", async () => {
    let sent: any;
    apiClient.request = (async (config) => {
        sent = config;
        return { status: 200, data: { code: 0, data: { id: "owned" }, msg: "ok" } };
    }) as typeof apiClient.request;
    await uploadFeedbackAttachment(new File(["image"], "test.png", { type: "image/png" }), "actor");
    expect(sent.data).toBeInstanceOf(FormData);
    expect(sent.headers["Content-Type"]).toBeUndefined();
    expect(sent.headers["X-Canvas-Expected-User-ID"]).toBe("actor");
});

test("business conflicts propagate with machine-readable reason", async () => {
    apiClient.request = (async () => ({ status: 200, data: { code: 409, reason: "feedback_changed", msg: "changed", data: null } })) as typeof apiClient.request;
    try {
        await updateAdminFeedback("fb", { status: "resolved" }, "admin", 1);
        throw new Error("unexpected success");
    } catch (error) {
        expect(error).toBeInstanceOf(ApiError);
        expect((error as ApiError).reason).toBe("feedback_changed");
    }
});

test("media URLs use feedback scope; environment paths drop queries and dynamic IDs", () => {
    expect(feedbackAttachmentURL("fb/id", "res/id")).toEndWith("/feedbacks/fb%2Fid/attachments/res%2Fid");
    expect(() => feedbackAttachmentURL("", "resource")).toThrow();
    for (const [path, safe] of [
        ["/canvas/private?key=secret#hidden", "/canvas/:id"],
        ["/admin/users/person", "/admin/:section"],
        ["/settings?token=private", "/settings"],
        ["/private/person", "/:page"],
    ])
        expect(safeFeedbackPathname(path)).toBe(safe);
});
