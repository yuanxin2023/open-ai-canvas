import { http } from "@/services/api/request";
import type { UserPrompt } from "@/services/api/user-prompts";
import type { CreationInspiration } from "@/lib/inspirations/catalog";

export type InspirationDraft = {
    title: string;
    description: string;
    image: string;
    coverResourceId?: string;
    mode: CreationInspiration["mode"];
    prompt: string;
    promptZh?: string;
    promptZhNeedsReview?: boolean;
    category?: string;
    tags?: string[];
    ratio?: string;
};
export type InspirationLibraryView = {
    revision: number;
    prompts: UserPrompt[];
    state: {
        favorites: string[];
        hidden: string[];
        overrides: Record<string, string>;
        importedCustom: Record<string, string>;
        metadata: Record<string, Pick<InspirationDraft, "promptZh" | "promptZhNeedsReview" | "category" | "ratio"> & Pick<CreationInspiration, "sourceId" | "credit" | "sourceUrl">>;
    };
    created?: number;
    updated?: number;
};
export type InspirationPayload = {
    version: number;
    exportedAt: string;
    custom: CreationInspiration[];
    overrides: Record<string, Partial<InspirationDraft>>;
    hidden: string[];
    favorites: string[];
};
export type InspirationLibraryCommand = {
    expectedUserId: string;
    expectedRevision: number;
    operation: "save" | "favorite" | "hide" | "restore" | "delete" | "import";
    id?: string;
    draft?: InspirationDraft;
    favorited?: boolean;
    payload?: InspirationPayload;
};
export function getInspirationLibrary(signal?: AbortSignal) {
    return http.get<InspirationLibraryView>("/inspiration-library", { signal });
}
export function applyInspirationLibrary(command: InspirationLibraryCommand) {
    return http.post<InspirationLibraryView>("/inspiration-library/commands", command);
}
