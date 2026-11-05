import { afterEach, expect, test } from "bun:test";
import { apiClient, ApiError } from "@/services/api/request";
import { archiveAdminFeedback, deleteAdminFeedback, submitFeedback, updateAdminFeedback, uploadFeedbackAttachment } from "@/services/api/feedback";

const original = apiClient.request;
afterEach(() => {
    apiClient.request = original;
});
const payload = { type: "bug" as const, title: "test", content: "body", environment: "{}", attachments: "[]" };

test("official feedback submission uses the business envelope and session transport", async () => {
    let sent: any;
    apiClient.request = (async (config) => {
        sent = config;
        return { status: 200, data: { code: 0, data: { id: "fb" }, msg: "ok" } };
    }) as typeof apiClient.request;
    expect((await submitFeedback(payload)).id).toBe("fb");
    expect(sent.url).toBe("/feedbacks");
    expect(sent.data).toEqual(payload);
    expect(sent.data.requestId).toBeUndefined();
});

test("official admin mutations encode IDs and distinguish archive from unarchive", async () => {
    const sent: any[] = [];
    apiClient.request = (async (config) => {
        sent.push(config);
        return { status: 200, data: { code: 0, data: { ok: true }, msg: "ok" } };
    }) as typeof apiClient.request;
    await updateAdminFeedback("fb/id", { adminReply: "" });
    await archiveAdminFeedback("fb/id", true);
    await archiveAdminFeedback("fb/id", false);
    await deleteAdminFeedback("fb/id");
    expect(sent.map((item) => item.url)).toEqual(["/admin/feedbacks/fb%2Fid", "/admin/feedbacks/fb%2Fid/archive", "/admin/feedbacks/fb%2Fid/unarchive", "/admin/feedbacks/fb%2Fid"]);
    expect(sent[0].data.adminReply).toBe("");
    expect(sent[3].method.toLowerCase()).toBe("delete");
    expect(sent[3].data).toBeUndefined();
});

test("empty feedback IDs cannot issue admin mutations", async () => {
    let calls = 0;
    apiClient.request = (async () => {
        calls++;
        throw new Error("unexpected transport");
    }) as typeof apiClient.request;
    await expect(updateAdminFeedback("", { status: "resolved" })).rejects.toThrow("反馈 ID");
    await expect(archiveAdminFeedback(" ")).rejects.toThrow("反馈 ID");
    await expect(deleteAdminFeedback("")).rejects.toThrow("反馈 ID");
    expect(calls).toBe(0);
});

test("feedback upload leaves multipart Content-Type to the browser", async () => {
    let sent: any;
    apiClient.request = (async (config) => {
        sent = config;
        return { status: 200, data: { code: 0, data: { id: "owned" }, msg: "ok" } };
    }) as typeof apiClient.request;
    await uploadFeedbackAttachment(new File(["image"], "test.png", { type: "image/png" }));
    expect(sent.data).toBeInstanceOf(FormData);
    expect(sent.headers?.["Content-Type"]).toBeUndefined();
});

test("feedback write failures preserve machine-readable reasons", async () => {
    apiClient.request = (async () => ({ status: 200, data: { code: 409, reason: "feedback_changed", msg: "changed", data: null } })) as typeof apiClient.request;
    try {
        await updateAdminFeedback("fb", { status: "resolved" });
        throw new Error("unexpected success");
    } catch (error) {
        expect(error).toBeInstanceOf(ApiError);
        expect((error as ApiError).reason).toBe("feedback_changed");
    }
});
