import { useCallback, useMemo, useRef, type ReactNode } from "react";
import { App } from "antd";
import { useAuthEntry } from "@/hooks/use-auth-entry";
import { WorkspaceAuthContext, type WorkspaceAuthRequest } from "./workspace-auth-context";

// Persist the workspace draft before entering the shared page/dialog authentication flow.
export function WorkspaceAuthProvider({ children }: { children: ReactNode }) {
    const prepareDraft = useRef<(() => Promise<void>) | null>(null);
    const opening = useRef(false);
    const { openAuth: openEntry } = useAuthEntry();
    const { message } = App.useApp();
    const openAuth = useCallback(
        async (options: WorkspaceAuthRequest = {}) => {
            if (opening.current) return;
            opening.current = true;
            try {
                await prepareDraft.current?.();
                openEntry({ ...options, view: options.view === "forgot-password" ? "recovery" : options.view });
            } catch (error) {
                message.error(error instanceof Error ? error.message : "保存创作草稿失败，请重试");
            } finally {
                opening.current = false;
            }
        },
        [message, openEntry],
    );
    const context = useMemo(() => ({ openAuth, prepareDraft }), [openAuth]);
    return <WorkspaceAuthContext.Provider value={context}>{children}</WorkspaceAuthContext.Provider>;
}
