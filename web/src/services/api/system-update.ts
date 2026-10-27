import { http } from "./request";

export type UpdatePhase =
    | "idle"
    | "checking"
    | "ready"
    | "no_update"
    | "preflight"
    | "backing_up"
    | "pulling"
    | "draining"
    | "migrating"
    | "switching"
    | "verifying"
    | "succeeded"
    | "rolling_back"
    | "rolled_back"
    | "failed"
    | "manual_intervention";

export type SystemUpdateRelease = {
    version: string;
    name: string;
    body: string;
    url: string;
    publishedAt: string;
    prerelease: boolean;
};

export type SystemUpdateCheck = {
    key: string;
    label: string;
    status: "passed" | "pending" | "failed" | string;
    detail?: string;
    blocking: boolean;
};

export type SystemUpdateBackup = {
    id: string;
    path: string;
    checksum: string;
    size: number;
    createdAt: string;
    version: string;
};

export type SystemUpdateLog = {
    at: string;
    phase: UpdatePhase;
    message: string;
};

export type SystemUpdateOperation = {
    id?: string;
    phase: UpdatePhase;
    fromVersion?: string;
    targetVersion?: string;
    startedAt?: string;
    finishedAt?: string;
    error?: string;
    rollbackError?: string;
    automaticRollback: boolean;
    logs: SystemUpdateLog[];
};

export type SystemUpdateStatus = {
    supported: boolean;
    connected: boolean;
    repository: string;
    deployment: string;
    currentVersion: string;
    latestRelease?: SystemUpdateRelease;
    updateAvailable: boolean;
    checks: SystemUpdateCheck[];
    lastBackup?: SystemUpdateBackup;
    rollbackVersion?: string;
    operation: SystemUpdateOperation;
};

export function getSystemUpdateStatus(signal?: AbortSignal) {
    return http.get<SystemUpdateStatus>("/admin/system-update", { signal });
}

function mutationHeaders(actorId: string) {
    if (!actorId) throw new Error("请先登录");
    return { "X-Canvas-Expected-User-ID": actorId };
}

export function checkSystemUpdate(actorId: string) {
    return http.post<SystemUpdateStatus>("/admin/system-update/check", undefined, { headers: mutationHeaders(actorId) });
}

export function startSystemUpdate(targetVersion: string, actorId: string) {
    if (!/^v[0-9]+(?:\.[0-9]+){2,}$/.test(targetVersion) || targetVersion.length > 128) throw new Error("请选择自有仓库的正式发布版本");
    return http.post<SystemUpdateStatus>("/admin/system-update/start", { targetVersion }, { headers: mutationHeaders(actorId) });
}

export function rollbackSystemUpdate(reason: string, actorId: string) {
    if (!reason.trim()) throw new Error("请填写回退原因");
    return http.post<SystemUpdateStatus>("/admin/system-update/rollback", { reason: reason.trim() }, { headers: mutationHeaders(actorId) });
}
