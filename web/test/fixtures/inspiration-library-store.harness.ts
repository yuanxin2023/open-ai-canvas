import { expect, test } from "bun:test";
import { apiClient } from "@/services/api/request";
import { useUserStore, defaultFeatureAvailability } from "@/stores/use-user-store";
import { useInspirationStore } from "@/stores/use-inspiration-store";
import type { InspirationLibraryView } from "@/services/api/inspiration-library";

const library = () => useInspirationStore.getState();
const draft = { title: "Saved", description: "", image: "/inspiration-thumbs/owned.webp", mode: "image" as const, prompt: "A scene", tags: [] };
const initial = (): InspirationLibraryView => ({
    revision: 7,
    prompts: [{ id: "old-id", title: "Existing personal", description: "Keep", mode: "image", prompt: "Original body", tags: [], coverResourceId: "owned-cover", createdAt: "2026-10-26T00:00:00Z", updatedAt: "2026-10-26T00:00:00Z" }],
    state: { favorites: [], hidden: [], overrides: {}, importedCustom: {}, metadata: {} },
});
let reads: any[] = [],
    writes: any[] = [];
let read: () => Promise<InspirationLibraryView>;
let write: (value: any) => Promise<InspirationLibraryView>;
function user(id = "a") {
    return { id, role: "user" } as any;
}
function deferred<T>() {
    let resolve!: (value: T) => void;
    const promise = new Promise<T>((done) => {
        resolve = done;
    });
    return { resolve, promise };
}
function setup(enabled = true) {
    useUserStore.setState({ user: null });
    useUserStore.setState({ user: user(), features: { ...defaultFeatureAvailability, inspirationHallEnabled: enabled } });
    reads = [];
    writes = [];
    read = async () => initial();
    write = async () => initial();
    apiClient.request = (async (config) => {
        let data: InspirationLibraryView;
        if (String(config.method).toLowerCase() === "get") {
            reads.push(config);
            data = await read();
        } else {
            writes.push(config.data);
            data = await write(config.data);
        }
        return { status: 200, data: { code: 0, data, msg: "ok" } };
    }) as typeof apiClient.request;
}

test("closed library performs no fetch or write", async () => {
    setup(false);
    await expect(library().load()).rejects.toThrow("暂未开放");
    await expect(library().createEntry(draft)).rejects.toThrow("暂未开放");
    expect(reads).toHaveLength(0);
    expect(writes).toHaveLength(0);
});
test("anonymous operations have no default user", async () => {
    setup();
    useUserStore.setState({ user: null });
    await expect(library().load()).rejects.toThrow("请先登录");
    expect(reads).toHaveLength(0);
});
test("existing personal identities, bodies and owned covers are reused", async () => {
    setup();
    await library().load();
    expect(library().custom[0]).toMatchObject({ id: "custom:old-id", prompt: "Original body", coverResourceId: "owned-cover" });
    expect(library().custom[0].image).toContain("/resources/owned-cover/file");
    expect(reads[0].signal).toBeInstanceOf(AbortSignal);
});
test("favorite waits for the real receipt and carries actor and revision", async () => {
    setup();
    await library().load();
    const pending = deferred<InspirationLibraryView>();
    write = () => pending.promise;
    const saving = library().toggleFavorite("seedance:scene");
    expect(library().favorites).toEqual([]);
    expect(writes[0]).toMatchObject({ expectedUserId: "a", expectedRevision: 7, operation: "favorite", id: "seedance:scene", favorited: true });
    const next = initial();
    next.revision++;
    next.state.favorites = ["seedance:scene"];
    pending.resolve(next);
    await saving;
    expect(library().favorites).toEqual(["seedance:scene"]);
});
test("failed writes keep the previous snapshot and release the button lock", async () => {
    setup();
    await library().load();
    write = async () => {
        throw new Error("server write failed");
    };
    await expect(library().toggleFavorite("seedance:scene")).rejects.toThrow("server write failed");
    expect(library().favorites).toEqual([]);
    expect(library().busy).toBe(false);
    expect(library().custom[0].id).toBe("custom:old-id");
});
test("stale revisions block the next write until a refresh", async () => {
    setup();
    await library().load();
    write = async () => {
        throw Object.assign(new Error("refresh required"), { reason: "inspiration_library_changed" });
    };
    await expect(library().updateEntry("custom:old-id", draft)).rejects.toThrow("refresh required");
    expect(library().hydrated).toBe(false);
    await expect(library().toggleFavorite("seedance:scene")).rejects.toThrow("刷新");
    expect(writes).toHaveLength(1);
});
test("rapid actions cannot create concurrent writes", async () => {
    setup();
    await library().load();
    const pending = deferred<InspirationLibraryView>();
    write = () => pending.promise;
    const first = library().toggleFavorite("seedance:scene");
    await expect(library().toggleFavorite("seedance:other")).rejects.toThrow("正在保存");
    expect(writes).toHaveLength(1);
    pending.resolve({ ...initial(), revision: 8 });
    await first;
});
test("old account reads cannot enter the new account", async () => {
    setup();
    const pending = deferred<InspirationLibraryView>();
    read = () => pending.promise;
    const loading = library().load();
    useUserStore.setState({ user: user("b") });
    pending.resolve(initial());
    await loading;
    expect(library().custom).toEqual([]);
    expect(library().hydrated).toBe(false);
});
test("old account write receipts are rejected without publishing", async () => {
    setup();
    await library().load();
    const pending = deferred<InspirationLibraryView>();
    write = () => pending.promise;
    const saving = library().toggleFavorite("seedance:scene");
    useUserStore.setState({ user: user("b") });
    pending.resolve({ ...initial(), revision: 8 });
    await expect(saving).rejects.toThrow("账号");
    expect(library().custom).toEqual([]);
    expect(library().favorites).toEqual([]);
});
test("late reads cannot roll a newer snapshot back", async () => {
    setup();
    await library().load();
    const pending = deferred<InspirationLibraryView>();
    read = () => pending.promise;
    const loading = library().load();
    write = async () => ({ ...initial(), revision: 8, state: { ...initial().state, favorites: ["seedance:scene"] } });
    await library().toggleFavorite("seedance:scene");
    pending.resolve(initial());
    await loading;
    expect(library().revision).toBe(8);
    expect(library().favorites).toEqual(["seedance:scene"]);
});
test("hide keeps the confirmed server copy and favorites", async () => {
    setup();
    const snapshot = initial();
    snapshot.state.overrides["seedance:scene"] = "old-id";
    snapshot.state.favorites = ["seedance:scene"];
    read = async () => snapshot;
    await library().load();
    write = async () => ({ ...snapshot, revision: 8, state: { ...snapshot.state, hidden: ["seedance:scene"] } });
    await library().removeEntry("seedance:scene");
    expect(writes[0].operation).toBe("hide");
    expect(library().overrides["seedance:scene"].prompt).toBe("Original body");
    expect(library().favorites).toEqual(["seedance:scene"]);
});
test("an editor keeps its opening revision after a background refresh", async () => {
    setup();
    await library().load();
    const openedAt = library().revision;
    read = async () => ({ ...initial(), revision: 8 });
    await library().load();
    write = async (command) => {
        if (command.expectedRevision !== 8) throw Object.assign(new Error("refresh required"), { reason: "inspiration_library_changed" });
        return { ...initial(), revision: 9 };
    };
    await expect(library().updateEntry("custom:old-id", draft, openedAt)).rejects.toThrow("refresh required");
    expect(writes[0].expectedRevision).toBe(7);
    expect(library().custom[0].prompt).toBe("Original body");
});
test("import failure publishes no partial entries and export requires a loaded account", async () => {
    setup();
    expect(() => library().exportPayload()).toThrow("刷新");
    await library().load();
    write = async () => {
        throw new Error("500 prompt limit");
    };
    await expect(library().importPayload(JSON.stringify({ version: 1, custom: [{ ...draft, id: "custom:imported" }], overrides: {}, hidden: [], favorites: [] }))).rejects.toThrow("500 prompt limit");
    expect(library().custom).toHaveLength(1);
    expect(library().custom[0].id).toBe("custom:old-id");
    expect(JSON.parse(library().exportPayload()).custom[0].id).toBe("custom:old-id");
});
