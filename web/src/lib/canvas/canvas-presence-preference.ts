import { useCallback, useSyncExternalStore } from "react";
import { getActiveUserScope, isUserScopedPersistenceSuppressed, scopedStorageKey } from "@/lib/user-scope";

export const CANVAS_PRESENCE_PREFERENCE_KEY = "open_ai_canvas:canvas_presence_hints";
const changedEvent = "canvas-presence-preference-changed";

export function readCanvasPresenceHints(scope: string) {
    if (typeof window === "undefined" || scope !== getActiveUserScope()) return false;
    try {
        return window.localStorage.getItem(scopedStorageKey(CANVAS_PRESENCE_PREFERENCE_KEY, scope)) === "true";
    } catch {
        return false;
    }
}

export function writeCanvasPresenceHints(scope: string, enabled: boolean) {
    if (typeof window === "undefined" || scope !== getActiveUserScope() || isUserScopedPersistenceSuppressed()) return;
    window.localStorage.setItem(scopedStorageKey(CANVAS_PRESENCE_PREFERENCE_KEY, scope), String(enabled));
    window.dispatchEvent(new Event(changedEvent));
}

function subscribe(listener: () => void) {
    if (typeof window === "undefined") return () => {};
    window.addEventListener("storage", listener);
    window.addEventListener(changedEvent, listener);
    return () => {
        window.removeEventListener("storage", listener);
        window.removeEventListener(changedEvent, listener);
    };
}

export function useCanvasPresenceHints(scope: string) {
    const snapshot = useCallback(() => readCanvasPresenceHints(scope), [scope]);
    const enabled = useSyncExternalStore(subscribe, snapshot, () => false);
    const setEnabled = useCallback((next: boolean) => writeCanvasPresenceHints(scope, next), [scope]);
    return [enabled, setEnabled] as const;
}
