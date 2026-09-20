import { apiBaseURL, compactApiParams, http } from "@/services/api/request";
import type { RemoteResource } from "@/services/api/resources";

export type InspirationMode = "text" | "image" | "video";
export type InspirationStatus = "active" | "disabled";

export type Inspiration = {
    id: string;
    title: string;
    description: string;
    mode: InspirationMode;
    prompt: string;
    tags: string[];
    source?: string;
    coverResourceId?: string;
    coverUrl: string;
    status: InspirationStatus;
    sortOrder: number;
    createdBy: string;
    updatedBy: string;
    createdAt: string;
    updatedAt: string;
};

export type InspirationInput = Pick<Inspiration, "title" | "description" | "mode" | "prompt" | "tags"> & {
    source?: string;
    coverResourceId?: string;
    coverUrl?: string;
};

function absoluteAPIURL(path: string) {
    if (!path.startsWith("/api/")) return path;
    const base = String(apiBaseURL).replace(/\/+$/, "");
    return base === "/api" ? path : `${base}${path.slice("/api".length)}`;
}

export function inspirationCoverUrl(item: Pick<Inspiration, "coverUrl">) {
    return absoluteAPIURL(item.coverUrl || "");
}

export function inspirationDraftCoverUrl(resourceId: string) {
    return absoluteAPIURL(`/api/admin/inspiration-covers/${encodeURIComponent(resourceId)}`);
}

export function listInspirations() {
    return http.get<{ inspirations: Inspiration[] }>("/inspirations");
}

export function listAdminInspirations(params: { keyword?: string; mode?: InspirationMode; status?: InspirationStatus; page?: number; pageSize?: number } = {}) {
    return http.get<{ inspirations: Inspiration[]; total: number; page: number; pageSize: number }>("/admin/inspirations", { params: compactApiParams(params) });
}

export function createAdminInspiration(input: InspirationInput) {
    return http.post<{ inspiration: Inspiration }>("/admin/inspirations", input);
}

export function updateAdminInspiration(id: string, input: InspirationInput) {
    return http.put<{ inspiration: Inspiration }>(`/admin/inspirations/${encodeURIComponent(id)}`, input);
}

export function setAdminInspirationStatus(id: string, status: InspirationStatus) {
    return http.patch<{ inspiration: Inspiration }>(`/admin/inspirations/${encodeURIComponent(id)}/status`, { status });
}

export function deleteAdminInspiration(id: string) {
    return http.delete<{ ok: boolean }>(`/admin/inspirations/${encodeURIComponent(id)}`);
}

export function batchDeleteAdminInspirations(ids: string[]) {
    return http.post<{ ok: boolean }>("/admin/inspirations/batch-delete", { ids });
}

export function getAdminInspirationOrder() {
    return http.get<{ items: Array<{ id: string; name: string; enabled: boolean }> }>("/admin/inspirations/order");
}

export function saveAdminInspirationOrder(ids: string[], expectedIds: string[]) {
    return http.put<{ saved: boolean }>("/admin/inspirations/order", { ids, expectedIds });
}

export function uploadAdminInspirationCover(file: File) {
    const body = new FormData();
    body.append("file", file, file.name);
    return http.post<{ resource: RemoteResource }>("/admin/inspiration-covers", body);
}

export function discardAdminInspirationCover(resourceId: string) {
    return http.delete<{ ok: boolean }>(`/admin/inspiration-covers/${encodeURIComponent(resourceId)}`);
}
