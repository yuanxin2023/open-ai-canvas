import localforage from "localforage";
import { nanoid } from "nanoid";
import { scopedStorageKey } from "@/lib/user-scope";
import type { CreationAttachment } from "./creation-assets";
import type { CreationReference } from "./creation-references";
import type { CreationMode } from "./creation-types";

export type CreationLoginDraft = {
    mode: CreationMode;
    prompt: string;
    model: string;
    ratio: string;
    seconds: string;
    quality: string;
    imageResolution: string;
    imageParameters: string;
    videoQuality: string;
    count: string;
    textStreaming: boolean;
    textThinking: boolean;
    attachments: CreationAttachment[];
    references: CreationReference[];
};

const storage = localforage.createInstance({ name: "yingce", storeName: "creation_login_drafts", driver: localforage.INDEXEDDB });
const pendingKey = "creation.pending-login-draft";
const lifetime = 60 * 60 * 1000;

// Only this tab's explicit login action can hand a guest draft to the next session.
// IndexedDB keeps media and large prompts out of synchronous browser storage.
export async function saveCreationLoginDraft(draft: CreationLoginDraft) {
    const previous = window.sessionStorage.getItem(pendingKey);
    const id = scopedStorageKey(`creation-login-draft:${nanoid()}`, "guest");
    await storage.setItem(id, { draft, expiresAt: Date.now() + lifetime });
    try {
        window.sessionStorage.setItem(pendingKey, id);
    } catch (error) {
        await storage.removeItem(id);
        throw error;
    }
    if (previous) void storage.removeItem(previous).catch((error) => console.warn("清理旧创作草稿失败", error));
}

export async function readCreationLoginDraft() {
    const id = window.sessionStorage.getItem(pendingKey);
    if (!id) return null;
    const saved = await storage.getItem<{ draft: CreationLoginDraft; expiresAt: number }>(id);
    if (!saved || saved.expiresAt <= Date.now()) {
        await discardCreationLoginDraft(id);
        return null;
    }
    return { id, draft: saved.draft };
}

export async function discardCreationLoginDraft(id: string) {
    if (window.sessionStorage.getItem(pendingKey) === id) window.sessionStorage.removeItem(pendingKey);
    await storage.removeItem(id);
}
