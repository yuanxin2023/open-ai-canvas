import { apiBaseURL, compactApiParams, http, serializeApiParams } from "./request";

export type FeedbackType = "bug" | "feature" | "improvement" | "other";
export type FeedbackStatus = "pending" | "processing" | "resolved" | "closed";

export interface FeedbackAttachment {
    id: string;
    resourceId?: string;
    name: string;
    type: "image" | "video";
    url: string;
    size: number;
    mimeType?: string;
}

export interface FeedbackItem {
    id: string;
    version: number;
    userId: string;
    userName: string;
    userEmail?: string;
    contact?: string;
    type: FeedbackType;
    title: string;
    content: string;
    attachments?: string; // JSON encoded FeedbackAttachment[]
    environment?: string; // JSON encoded client info
    status: FeedbackStatus;
    adminReply?: string;
    repliedAt?: string;
    archived?: boolean;
    archivedAt?: string;
    createdAt: string;
    updatedAt: string;
}

export interface SubmitFeedbackPayload {
    requestId: string;
    type: FeedbackType;
    title: string;
    content: string;
    contact?: string;
    attachments?: string;
    environment?: string;
}

export interface FeedbackListResult {
    list: FeedbackItem[];
    total: number;
    page: number;
    limit: number;
}

export interface AdminFeedbackFilter {
    type?: string;
    status?: string;
    keyword?: string;
    archived?: boolean | "all";
    page?: number;
    limit?: number;
}

function mutationHeaders(actorId: string, version?: number) {
    if (!actorId) throw new Error("请先登录");
    if (version !== undefined && (!Number.isSafeInteger(version) || version < 1)) throw new Error("反馈版本无效，请刷新列表");
    return { "X-Canvas-Expected-User-ID": actorId, ...(version !== undefined ? { "X-Canvas-Feedback-Version": String(version) } : {}) };
}

export async function submitFeedback(payload: SubmitFeedbackPayload, actorId: string): Promise<FeedbackItem> {
    return http.post<FeedbackItem>("/feedbacks", payload, { headers: mutationHeaders(actorId) });
}

export async function getMyFeedbacks(signal?: AbortSignal): Promise<FeedbackItem[]> {
    return http.get<FeedbackItem[]>("/feedbacks/my", { signal });
}

export async function uploadFeedbackAttachment(file: File, actorId: string): Promise<FeedbackAttachment> {
    const formData = new FormData();
    formData.append("file", file);
    return http.post<FeedbackAttachment>("/feedbacks/attachments", formData, { headers: mutationHeaders(actorId) });
}

export async function getAdminFeedbacks(filter: AdminFeedbackFilter = {}, signal?: AbortSignal): Promise<FeedbackListResult> {
    const params = compactApiParams({
        type: filter.type && filter.type !== "all" ? filter.type : undefined,
        status: filter.status && filter.status !== "all" ? filter.status : undefined,
        archived: filter.archived !== undefined && filter.archived !== "all" ? String(filter.archived) : undefined,
        keyword: filter.keyword,
        page: filter.page || undefined,
        pageSize: filter.limit || undefined,
    });
    const queryStr = serializeApiParams(params).toString();
    const url = queryStr ? `/admin/feedbacks?${queryStr}` : "/admin/feedbacks";
    return http.get<FeedbackListResult>(url, { signal });
}

export async function updateAdminFeedback(id: string, payload: { status?: FeedbackStatus; adminReply?: string; archived?: boolean }, actorId: string, version: number): Promise<FeedbackItem> {
    return http.patch<FeedbackItem>(`/admin/feedbacks/${encodeURIComponent(id)}`, payload, { headers: mutationHeaders(actorId, version) });
}

export async function archiveAdminFeedback(id: string, archive: boolean, actorId: string, version: number): Promise<FeedbackItem> {
    const action = archive ? "archive" : "unarchive";
    return http.post<FeedbackItem>(`/admin/feedbacks/${encodeURIComponent(id)}/${action}`, undefined, { headers: mutationHeaders(actorId, version) });
}

export async function deleteAdminFeedback(id: string, actorId: string, version: number): Promise<{ ok: boolean }> {
    return http.delete<{ ok: boolean }>(`/admin/feedbacks/${encodeURIComponent(id)}`, { headers: mutationHeaders(actorId, version) });
}

export function feedbackAttachmentURL(feedbackId: string, resourceId: string): string {
    if (!feedbackId || !resourceId) throw new Error("反馈附件标识无效");
    return `${apiBaseURL.replace(/\/$/, "")}/feedbacks/${encodeURIComponent(feedbackId)}/attachments/${encodeURIComponent(resourceId)}`;
}
