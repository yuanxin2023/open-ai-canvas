import { createContext, useContext } from "react";
import type { AuthView } from "./auth-form-navigation";

export type WorkspaceAuthRequest = { view?: AuthView; next?: string };
export const WorkspaceAuthContext = createContext<{
    openAuth: (request?: WorkspaceAuthRequest) => Promise<void>;
    prepareDraft: { current: (() => Promise<void>) | null };
} | null>(null);

export function useWorkspaceAuth() {
    const context = useContext(WorkspaceAuthContext);
    if (!context) throw new Error("Workspace authentication provider is missing");
    return context;
}
