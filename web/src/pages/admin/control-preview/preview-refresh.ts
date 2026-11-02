export const previewRefreshIntervals = [5, 30, 60, 120] as const;
export type PreviewRefreshPreference = { enabled: boolean; intervalSeconds: number };
export const defaultPreviewRefresh: PreviewRefreshPreference = { enabled: true, intervalSeconds: 30 };

export function normalizePreviewRefresh(value: unknown): PreviewRefreshPreference {
    const saved = value as Partial<PreviewRefreshPreference> | null;
    return {
        enabled: typeof saved?.enabled === "boolean" ? saved.enabled : defaultPreviewRefresh.enabled,
        intervalSeconds: previewRefreshIntervals.some(seconds => seconds === saved?.intervalSeconds) ? saved!.intervalSeconds! : defaultPreviewRefresh.intervalSeconds,
    };
}

export function previewRefreshStorageKey(userId: string) {
    return `admin-console:${encodeURIComponent(userId)}:control-preview-auto-refresh`;
}

export function previewRefreshRemaining(deadline: number, now: number) {
    return Math.max(0, Math.ceil((deadline - now) / 1000));
}
