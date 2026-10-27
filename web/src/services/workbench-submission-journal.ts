import { createClientUUID } from "@/lib/client-id";
import type { WorkbenchKind } from "@/services/api/workbench";

export type WorkbenchSubmissionEndpoint = "workbench" | "custom";
export type PendingWorkbenchSubmission = { version: 1; key: string; hash: string; endpoint: WorkbenchSubmissionEndpoint; createdAt: string };
export type WorkbenchSubmissionStore = {
    mutate: (scope: string, kind: WorkbenchKind, update: (old: PendingWorkbenchSubmission | null) => PendingWorkbenchSubmission | null) => Promise<PendingWorkbenchSubmission | null>;
};

function validateRecord(value: unknown): PendingWorkbenchSubmission | null {
    if (value == null) return null;
    const record = value as PendingWorkbenchSubmission;
    if (record.version !== 1 || !/^[A-Za-z0-9_-]{16,96}$/.test(record.key) || !/^[a-f0-9]{64}$/.test(record.hash) || !["workbench", "custom"].includes(record.endpoint) || typeof record.createdAt !== "string")
        throw new Error("待确认请求记录无法识别，请先核对原任务");
    return record;
}

// A read-modify-write transaction prevents two tabs from replacing each other's
// pending identity. Only identifiers and digests are stored, never request bodies.
const indexedDBStore: WorkbenchSubmissionStore = {
    async mutate(scope, kind, update) {
        if (!scope || typeof indexedDB === "undefined") throw new Error("无法保存请求标识，请检查浏览器本地存储后再提交");
        const db = await new Promise<IDBDatabase>((resolve, reject) => {
            let blocked = false;
            const open = indexedDB.open("yingce-workbench-submissions", 1);
            open.onupgradeneeded = () => {
                open.result.createObjectStore("pending");
                open.result.createObjectStore("archive");
            };
            open.onsuccess = () => {
                if (blocked) {
                    open.result.close();
                    return;
                }
                open.result.onversionchange = () => open.result.close();
                resolve(open.result);
            };
            open.onerror = () => reject(open.error || new Error("打开请求记录失败"));
            open.onblocked = () => {
                blocked = true;
                reject(new Error("请求记录被其他窗口占用，请关闭旧窗口后重试"));
            };
        });
        try {
            return await new Promise<PendingWorkbenchSubmission | null>((resolve, reject) => {
                const tx = db.transaction(["pending", "archive"], "readwrite");
                const pending = tx.objectStore("pending");
                const id = `${scope}:${kind}`;
                let result: PendingWorkbenchSubmission | null = null;
                let failure: unknown;
                const read = pending.get(id);
                read.onsuccess = () => {
                    try {
                        const old = validateRecord(read.result);
                        result = update(old);
                        if (old && (!result || old.key !== result.key)) tx.objectStore("archive").put(old, `${id}:${old.key}`);
                        if (result) pending.put(result, id);
                        else pending.delete(id);
                    } catch (error) {
                        failure = error;
                        tx.abort();
                    }
                };
                tx.oncomplete = () => resolve(result);
                tx.onabort = () => reject(failure || tx.error || new Error("保存请求记录失败"));
                tx.onerror = () => {
                    failure ||= tx.error;
                };
            });
        } finally {
            db.close();
        }
    },
};

export async function workbenchPayloadHash(payload: unknown) {
    const wire = JSON.parse(JSON.stringify(payload));
    const canonical = JSON.stringify(wire, (_key, value) =>
        value && typeof value === "object" && !Array.isArray(value)
            ? Object.fromEntries(
                  Object.keys(value)
                      .sort()
                      .map((key) => [key, value[key]]),
              )
            : value,
    );
    const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(canonical));
    return Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, "0")).join("");
}

export function createWorkbenchSubmissionJournal(store: WorkbenchSubmissionStore = indexedDBStore, createKey = createClientUUID) {
    return {
        load: (scope: string, kind: WorkbenchKind) => store.mutate(scope, kind, (old) => old),
        async reserve(scope: string, kind: WorkbenchKind, endpoint: WorkbenchSubmissionEndpoint, payload: unknown) {
            const hash = await workbenchPayloadHash(payload);
            return (await store.mutate(scope, kind, (old) => {
                if (old) {
                    if (old.hash !== hash || old.endpoint !== endpoint) throw new Error("上次提交尚未确认且参数已改变，请先核对原任务，或明确创建新任务");
                    return old;
                }
                return { version: 1, key: createKey(), hash, endpoint, createdAt: new Date().toISOString() };
            }))!;
        },
        clear: (scope: string, kind: WorkbenchKind, key: string) => store.mutate(scope, kind, (old) => (old?.key === key ? null : old)),
    };
}

const journal = createWorkbenchSubmissionJournal();
export const loadWorkbenchSubmission = journal.load;
export const reserveWorkbenchSubmission = journal.reserve;
export const clearWorkbenchSubmission = journal.clear;
