import { compactApiParams, http } from "@/services/api/request";

export type UserPromptMode = "text" | "image" | "video";

export type UserPrompt = {
    id: string;
    title: string;
    description: string;
    mode: UserPromptMode;
    prompt: string;
    tags: string[];
    source?: string;
    coverResourceId?: string;
    coverUrl?: string;
    createdAt: string;
    updatedAt: string;
};

export type UserPromptInput = Pick<UserPrompt, "title" | "description" | "mode" | "prompt" | "tags"> & {
    source?: string;
    coverResourceId?: string;
    coverUrl?: string;
};

export type UserPromptPage = {
    prompts: UserPrompt[];
    total: number;
    page: number;
    pageSize: number;
};

export function listUserPrompts(input: { keyword?: string; mode?: UserPromptMode; page?: number; pageSize?: number } = {}, signal?: AbortSignal) {
    return http.get<UserPromptPage>("/user-prompts", { params: compactApiParams(input), signal });
}

export function createUserPrompt(input: UserPromptInput) {
    return http.post<{ prompt: UserPrompt }>("/user-prompts", input);
}

export function updateUserPrompt(id: string, input: UserPromptInput) {
    return http.put<{ prompt: UserPrompt }>(`/user-prompts/${encodeURIComponent(id)}`, input);
}

export function deleteUserPrompt(id: string) {
    return http.delete<{ ok: boolean }>(`/user-prompts/${encodeURIComponent(id)}`);
}
