import { useCallback, useEffect, useRef, useState } from "react";
import { useIsFetching, useQueryClient } from "@tanstack/react-query";
import { useUserStore } from "@/stores/use-user-store";
import { defaultPreviewRefresh, normalizePreviewRefresh, previewRefreshRemaining, previewRefreshStorageKey, type PreviewRefreshPreference } from "./preview-refresh";

export function usePreviewRefresh() {
    const userId = useUserStore(state => state.user?.id);
    const client = useQueryClient();
    const fresh = useRef(false);
    const refreshing = useRef(false);
    const [preference, setPreference] = useState<PreviewRefreshPreference>(() => {
        if (!userId) return { ...defaultPreviewRefresh };
        try {
            return normalizePreviewRefresh(JSON.parse(localStorage.getItem(previewRefreshStorageKey(userId)) || "null"));
        } catch {
            return { ...defaultPreviewRefresh };
        }
    });
    const [visible, setVisible] = useState(() => document.visibilityState !== "hidden");
    const [countdown, setCountdown] = useState(preference.intervalSeconds);
    const queryKey = ["admin-control-preview", userId];
    const fetching = useIsFetching({ queryKey }) > 0;
    const refresh = useCallback(async (force = false) => {
        if (!userId || refreshing.current || client.isFetching({ queryKey: ["admin-control-preview", userId] })) return;
        refreshing.current = true;
        fresh.current = force;
        try {
            await client.refetchQueries({ queryKey: ["admin-control-preview", userId], type: "active" }, { cancelRefetch: false });
        } finally {
            fresh.current = false;
            refreshing.current = false;
        }
    }, [client, userId]);
    useEffect(() => {
        const update = () => {
            const next = document.visibilityState !== "hidden";
            setVisible(next);
            if (next) void refresh();
        };
        document.addEventListener("visibilitychange", update);
        return () => document.removeEventListener("visibilitychange", update);
    }, [refresh]);
    useEffect(() => {
        setCountdown(preference.intervalSeconds);
        if (!preference.enabled || !visible || fetching) return;
        const deadline = Date.now() + preference.intervalSeconds * 1000;
        const timer = window.setInterval(() => {
            const remaining = previewRefreshRemaining(deadline, Date.now());
            setCountdown(remaining);
            if (remaining === 0) {
                window.clearInterval(timer);
                void refresh();
            }
        }, 250);
        return () => window.clearInterval(timer);
    }, [preference, visible, fetching, refresh]);
    const updatePreference = (next: PreviewRefreshPreference) => {
        const normalized = normalizePreviewRefresh(next);
        setPreference(normalized);
        if (!userId) return;
        try {
            localStorage.setItem(previewRefreshStorageKey(userId), JSON.stringify(normalized));
        } catch {
            console.warn("控制预览刷新设置已更新，但无法保存偏好");
        }
    };
    return { fresh, preference, updatePreference, countdown, fetching, visible, refresh };
}
