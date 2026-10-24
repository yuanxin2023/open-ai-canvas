import { compactApiParams, http } from "./request";

export type PreviewPeriod = "day" | "week" | "month";
export type PreviewCapability = "text" | "image" | "video" | "audio";

export type PreviewMeta = {
    period: PreviewPeriod;
    from: string;
    to: string;
    asOf: string;
    timezone: string;
};

export type PreviewModel = {
    model: string;
    capability: PreviewCapability;
    generations: number;
    succeeded: number;
    failed: number;
    pending: number;
    consumedMicrocredits: number;
};

export type PreviewUser = {
    rank: number;
    userId: string;
    username: string;
    consumedMicrocredits: number;
    generations: number;
    share: number;
};

export type PreviewOverview = PreviewMeta & {
    kpi: {
        availableMicrocredits: number;
        reservedMicrocredits: number;
        consumedMicrocredits: number;
        lifetimeConsumedMicrocredits: number;
        inflowMicrocredits: number;
        paidAmountFen: number;
        payingUsers: number;
        paidOrders: number;
        consumingUsers: number;
        generations: number;
        succeeded: number;
        failed: number;
        pending: number;
        unlinkedGenerations: number;
    };
    ranking: PreviewUser[];
    models: PreviewModel[];
    inflowSources: Array<{ source: "redeem" | "payment" | "admin" | "reward"; amountMicrocredits: number }>;
    trend: Array<{ time: string; label: string; inflowMicrocredits: number; consumedMicrocredits: number }>;
};

export type PreviewUsers = PreviewMeta & {
    users: PreviewUser[];
    total: number;
    page: number;
    pageSize: number;
};

export type PreviewUserDetail = PreviewMeta & {
    user: PreviewUser;
    models: PreviewModel[];
    entries: Array<{ id: string; model: string; capability: PreviewCapability; billingOrderId: string; consumedMicrocredits: number; createdAt: string }>;
    total: number;
    page: number;
    pageSize: number;
};

const base = "/admin/analytics/control-preview";

export function getControlPreview(period: PreviewPeriod, signal?: AbortSignal) {
    return http.get<PreviewOverview>(base, { params: { period }, signal });
}

export function getControlPreviewUsers(params: { period: PreviewPeriod; page: number; pageSize: number; keyword?: string; model?: string; capability?: string }, signal?: AbortSignal) {
    return http.get<PreviewUsers>(`${base}${params.model !== undefined ? "/model-users" : "/users"}`, { params: compactApiParams(params), signal });
}

export function getControlPreviewUser(userId: string, params: { period: PreviewPeriod; page: number; pageSize: number }, signal?: AbortSignal) {
    return http.get<PreviewUserDetail>(`${base}/users/${encodeURIComponent(userId)}`, { params, signal });
}
