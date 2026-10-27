import { create } from "zustand";
import { createClientUUID } from "@/lib/client-id";
import { type CreationInspiration } from "@/lib/inspirations/catalog";
import { CUSTOM_ID_PREFIX, isCustomInspiration, type InspirationOverride } from "@/lib/inspirations/resolve";
import { applyInspirationLibrary, getInspirationLibrary, type InspirationLibraryView, type InspirationLibraryCommand, type InspirationDraft, type InspirationPayload } from "@/services/api/inspiration-library";
import { resourceFileUrl } from "@/services/api/resources";
import { useUserStore } from "@/stores/use-user-store";

export type { InspirationDraft, InspirationPayload } from "@/services/api/inspiration-library";
export const INSPIRATION_PAYLOAD_VERSION = 1;
export type InspirationImportResult = { created: number; updated: number; skipped: number };
type InspirationStore = {
    scope: string;
    revision: number;
    hydrated: boolean;
    loading: boolean;
    busy: boolean;
    error: string;
    custom: CreationInspiration[];
    overrides: Record<string, InspirationOverride>;
    hidden: string[];
    favorites: string[];
    load: () => Promise<void>;
    createEntry: (draft: InspirationDraft, revision?: number) => Promise<string>;
    updateEntry: (id: string, draft: InspirationOverride, revision?: number) => Promise<void>;
    removeEntry: (id: string, revision?: number) => Promise<void>;
    restoreEntry: (id: string) => Promise<void>;
    toggleFavorite: (id: string) => Promise<void>;
    exportPayload: () => string;
    importPayload: (text: string, revision?: number) => Promise<InspirationImportResult>;
};

function required(value: unknown, field: string) {
    if (typeof value !== "string" || !value.trim()) throw new Error(`缺少必填字段 ${field}`);
    return value.trim();
}
export function normalizeDraft(value: unknown): InspirationDraft {
    if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error("灵感内容格式不正确");
    const raw = value as Record<string, unknown>;
    if (raw.mode !== "text" && raw.mode !== "image" && raw.mode !== "video") throw new Error("创作类型无效");
    return {
        title: required(raw.title, "标题"),
        description: typeof raw.description === "string" ? raw.description.trim() : "",
        image: typeof raw.image === "string" ? raw.image.trim() : "",
        mode: raw.mode,
        prompt: required(raw.prompt, "提示词正文"),
        ...(typeof raw.coverResourceId === "string" && raw.coverResourceId ? { coverResourceId: raw.coverResourceId } : {}),
        ...(typeof raw.promptZh === "string" ? { promptZh: raw.promptZh.trim() } : {}),
        ...(raw.promptZhNeedsReview === true ? { promptZhNeedsReview: true } : {}),
        ...(typeof raw.category === "string" ? { category: raw.category.trim() } : {}),
        ...(typeof raw.ratio === "string" ? { ratio: raw.ratio.trim() } : {}),
        tags: Array.isArray(raw.tags) ? raw.tags.filter((tag): tag is string => typeof tag === "string" && !!tag.trim()).map((tag) => tag.trim()) : [],
    };
}
function projection(view: InspirationLibraryView) {
    if (
        !Number.isSafeInteger(view.revision) ||
        view.revision < 0 ||
        !Array.isArray(view.prompts) ||
        !view.state ||
        !Array.isArray(view.state.favorites) ||
        !Array.isArray(view.state.hidden) ||
        !view.state.overrides ||
        !view.state.importedCustom ||
        !view.state.metadata
    )
        throw new Error("个人灵感响应格式无效");
    const drafts = new Map<string, InspirationDraft>(
        view.prompts.map((prompt) => [
            prompt.id,
            {
                title: prompt.title,
                description: prompt.description,
                mode: prompt.mode,
                prompt: prompt.prompt,
                tags: prompt.tags,
                image: prompt.coverResourceId ? resourceFileUrl(prompt.coverResourceId) : prompt.coverUrl || "",
                ...(prompt.coverResourceId ? { coverResourceId: prompt.coverResourceId } : {}),
                ...view.state.metadata[prompt.id],
            } as InspirationDraft,
        ]),
    );
    const linked = new Set(Object.values(view.state.overrides));
    const promptSources = new Map<string, string | undefined>(view.prompts.map((prompt) => [prompt.id, prompt.source]));
    const canonical = (id: string) => (view.state.importedCustom[id] ? CUSTOM_ID_PREFIX + view.state.importedCustom[id] : id);
    return {
        revision: view.revision,
        custom: [...drafts]
            .filter(([id]) => !linked.has(id))
            .map(([id, draft]) => ({ ...draft, ...view.state.metadata[id], credit: view.state.metadata[id]?.credit || promptSources.get(id), id: CUSTOM_ID_PREFIX + id, sourceId: view.state.metadata[id]?.sourceId || ("original" as const) })),
        overrides: Object.fromEntries(Object.entries(view.state.overrides).flatMap(([key, id]) => (drafts.has(id) ? [[key, drafts.get(id)!]] : []))),
        hidden: view.state.hidden.map(canonical),
        favorites: view.state.favorites.map(canonical),
    };
}
const empty = () => ({ scope: "", revision: 0, hydrated: false, loading: false, busy: false, error: "", custom: [] as CreationInspiration[], overrides: {} as Record<string, InspirationOverride>, hidden: [] as string[], favorites: [] as string[] });
let generation = 0;
let readSequence = 0;
let controller: AbortController | null = null;
const actor = () => {
    const state = useUserStore.getState();
    if (!state.user?.id) throw new Error("请先登录后管理个人灵感");
    if (!state.features.inspirationHallEnabled) throw new Error("灵感大厅暂未开放");
    return state.user.id;
};
const current = (scope: string, version: number) => generation === version && useUserStore.getState().user?.id === scope && useUserStore.getState().features.inspirationHallEnabled;

export const useInspirationStore = create<InspirationStore>((set, get) => {
    const commit = async (input: Omit<InspirationLibraryCommand, "expectedUserId" | "expectedRevision">, expectedRevision?: number) => {
        const scope = actor(),
            version = generation,
            before = get();
        if (!before.hydrated || before.scope !== scope) throw new Error("请先刷新个人灵感后再保存");
        if (before.busy) throw new Error("正在保存，请稍候");
        set({ busy: true });
        try {
            const view = await applyInspirationLibrary({ ...input, expectedUserId: scope, expectedRevision: expectedRevision ?? before.revision });
            if (!current(scope, version)) throw new Error("账号或功能状态已变化，请重新打开灵感库");
            const next = projection(view);
            if (next.revision < get().revision) throw new Error("个人灵感已更新，请刷新核对保存结果");
            set({ ...next, scope, hydrated: true, error: "" });
            return view;
        } catch (error) {
            if (current(scope, version) && error && typeof error === "object" && "reason" in error && error.reason === "inspiration_library_changed") set({ hydrated: false, error: error instanceof Error ? error.message : "个人灵感已变化，请刷新" });
            throw error;
        } finally {
            if (current(scope, version)) set({ busy: false });
        }
    };
    return {
        ...empty(),
        load: async () => {
            let scope: string;
            try {
                scope = actor();
            } catch (error) {
                set({ ...empty(), error: error instanceof Error ? error.message : "无法读取个人灵感" });
                throw error;
            }
            const version = generation,
                sequence = ++readSequence;
            controller?.abort();
            controller = new AbortController();
            if (get().scope !== scope) set({ ...empty(), scope });
            set({ loading: true, error: "" });
            try {
                const view = await getInspirationLibrary(controller.signal);
                if (!current(scope, version) || sequence !== readSequence) return;
                const next = projection(view);
                if (next.revision >= get().revision) set({ ...next, scope, hydrated: true });
            } catch (error) {
                if (current(scope, version) && sequence === readSequence) set({ hydrated: false, error: error instanceof Error ? error.message : "个人灵感加载失败" });
                throw error;
            } finally {
                if (current(scope, version) && sequence === readSequence) set({ loading: false });
            }
        },
        createEntry: async (value, revision) => {
            const id = CUSTOM_ID_PREFIX + createClientUUID();
            const view = await commit({ operation: "save", id, draft: normalizeDraft(value) }, revision);
            const promptID = view.state.importedCustom[id];
            if (!promptID) throw new Error("已保存，请刷新核对个人副本");
            return CUSTOM_ID_PREFIX + promptID;
        },
        updateEntry: async (id, value, revision) => {
            await commit({ operation: "save", id, draft: normalizeDraft(value) }, revision);
        },
        removeEntry: async (id, revision) => {
            await commit({ operation: isCustomInspiration(id) ? "delete" : "hide", id }, revision);
        },
        restoreEntry: async (id) => {
            await commit({ operation: "restore", id });
        },
        toggleFavorite: async (id) => {
            await commit({ operation: "favorite", id, favorited: !get().favorites.includes(id) });
        },
        exportPayload: () => {
            const state = get();
            if (!state.hydrated || state.scope !== actor()) throw new Error("请先刷新个人灵感再导出");
            return JSON.stringify({ version: INSPIRATION_PAYLOAD_VERSION, exportedAt: new Date().toISOString(), custom: state.custom, overrides: state.overrides, hidden: state.hidden, favorites: state.favorites } satisfies InspirationPayload, null, 2);
        },
        importPayload: async (text, revision) => {
            let raw: any;
            try {
                raw = JSON.parse(text);
            } catch {
                throw new Error("文件不是合法的 JSON");
            }
            if (!raw || raw.version !== INSPIRATION_PAYLOAD_VERSION || !Array.isArray(raw.custom)) throw new Error("文件不是支持的灵感导出格式");
            let skipped = 0;
            const custom: CreationInspiration[] = [];
            for (const value of raw.custom) {
                try {
                    const draft = normalizeDraft(value);
                    const originalID = typeof value.id === "string" && value.id.trim() ? value.id.trim() : createClientUUID();
                    custom.push({
                        ...draft,
                        id: originalID.startsWith(CUSTOM_ID_PREFIX) ? originalID : CUSTOM_ID_PREFIX + originalID,
                        sourceId: (typeof value.sourceId === "string" ? value.sourceId : "original") as CreationInspiration["sourceId"],
                        ...(typeof value.credit === "string" ? { credit: value.credit } : {}),
                        ...(typeof value.sourceUrl === "string" ? { sourceUrl: value.sourceUrl } : {}),
                    });
                } catch {
                    skipped++;
                }
            }
            const overrides = Object.fromEntries(Object.entries(raw.overrides || {}).map(([id, value]) => [id, normalizeDraft(value)]));
            const strings = (values: unknown) => (Array.isArray(values) ? values.filter((value): value is string => typeof value === "string" && !!value.trim()) : []);
            const payload: InspirationPayload = { version: 1, exportedAt: typeof raw.exportedAt === "string" ? raw.exportedAt : "", custom, overrides, hidden: strings(raw.hidden), favorites: strings(raw.favorites) };
            const result = await commit({ operation: "import", payload }, revision);
            return { created: result.created || 0, updated: result.updated || 0, skipped };
        },
    };
});
useUserStore.subscribe((state, previous) => {
    if (state.user?.id !== previous.user?.id || state.features.inspirationHallEnabled !== previous.features.inspirationHallEnabled) {
        generation++;
        readSequence++;
        controller?.abort();
        controller = null;
        useInspirationStore.setState(empty());
    }
});
