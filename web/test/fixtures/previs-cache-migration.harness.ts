import { expect, mock, test } from "bun:test";
import { readFileSync } from "node:fs";
import { Window } from "happy-dom";

const dom = new Window({ url: "http://localhost/" });
Object.defineProperty(globalThis, "window", { value: dom, configurable: true });
const { setActiveUserScope } = await import("@/lib/user-scope");
const disk = new Map<string, string>();
const writes: string[] = [];
const key = (scope: string, name = "yingce:canvas_store") => `${scope}|${name}`;
mock.module("@/lib/localforage-storage", () => ({
    localForageStorageForScope: (scope: string) => ({
        getItem: async (name: string) => disk.get(key(scope, name)) ?? null,
        setItem: async (name: string, value: string) => {
            writes.push(key(scope, name));
            disk.set(key(scope, name), value);
        },
        removeItem: async (name: string) => {
            writes.push(key(scope, name));
            disk.delete(key(scope, name));
        },
    }),
}));
mock.module("@/stores/use-theme-store", () => ({ useThemeStore: { getState: () => ({ theme: "dark" }) } }));
mock.module("@/services/diagnostics/client-diagnostics", () => ({ recordDiagnosticEvent: () => {} }));
const fixture = () => JSON.parse(readFileSync(new URL("../../../backend/internal/canvas/previslegacy/testdata/director.json", import.meta.url), "utf8"));
const ambiguous = fixture();
ambiguous.previsScenes = [];
const original = JSON.stringify({ state: { projects: [ambiguous] }, version: 0 });
setActiveUserScope("legacy-owner");
disk.set(key("legacy-owner"), original);
const { useCanvasStore, flushCanvasStorePersistence, withCanvasStorePersistenceSuppressed } = await import("@/stores/canvas/use-canvas-store");
await useCanvasStore.persist.rehydrate();

test("恢复失败阻止空状态、账号回调和普通写入覆盖原缓存", async () => {
    expect(useCanvasStore.getState().hydrated).toBe(false);
    expect(useCanvasStore.getState().hydrationError).toContain("无法可靠转换");
    expect(() => useCanvasStore.getState().createProject("must not create")).toThrow("无法可靠转换");
    expect(() => useCanvasStore.getState().replaceProjects([])).toThrow("无法可靠转换");
    await expect(flushCanvasStorePersistence()).rejects.toThrow("无法可靠转换");
    expect(disk.get(key("legacy-owner"))).toBe(original);
    expect(writes).toEqual([]);
});

test("失败只阻止原账号，切换账号可正常保存且不写旧账号缓存", async () => {
    setActiveUserScope("another-owner");
    withCanvasStorePersistenceSuppressed(() => useCanvasStore.setState({ projects: [], hydrated: false }));
    await useCanvasStore.persist.rehydrate();
    expect(useCanvasStore.getState().hydrated).toBe(true);
    expect(useCanvasStore.getState().hydrationError).toBeNull();
    useCanvasStore.getState().createProject("另一账号");
    await flushCanvasStorePersistence();
    expect(writes.length).toBeGreaterThan(0);
    expect(writes.every((name) => name.startsWith("another-owner|"))).toBe(true);
    expect(disk.get(key("legacy-owner"))).toBe(original);
});

test("原数据明确修复后重新恢复，编辑写入新版字段且保留旧身份", async () => {
    disk.set(key("legacy-owner"), JSON.stringify({ state: { projects: [fixture()] }, version: 0 }));
    setActiveUserScope("legacy-owner");
    withCanvasStorePersistenceSuppressed(() => useCanvasStore.setState({ projects: [], hydrated: false }));
    await useCanvasStore.persist.rehydrate();
    expect(useCanvasStore.getState().hydrated).toBe(true);
    expect(useCanvasStore.getState().projects[0].previsScenes[0].id).toBe("scene-1");
    useCanvasStore.getState().renameProject("legacy-canvas", "确认后改名");
    await flushCanvasStorePersistence();
    const saved = JSON.parse(disk.get(key("legacy-owner"))!);
    expect(saved.state.projects[0].title).toBe("确认后改名");
    expect(saved.state.projects[0]).not.toHaveProperty("directorScenes");
    expect(saved.state.projects[0].previsScenes[0].shots[0].aspectRatio).toBe("16:9");
});

test("无法转换的紧急恢复 journal 同样阻止写回且保留原记录", async () => {
    const scope = "journal-owner";
    setActiveUserScope(scope);
    const journalKey = `yingce:canvas_store_journal:user:${scope}:writer-1`;
    const journal = JSON.stringify({ version: 1, name: "yingce:canvas_store", scope, writerId: "writer-1", token: 1, baseRevision: 0, projects: [ambiguous], baseProjects: [fixture()] });
    dom.localStorage.setItem(journalKey, journal);
    withCanvasStorePersistenceSuppressed(() => useCanvasStore.setState({ projects: [], hydrated: false }));
    const writesBefore = writes.length;
    await useCanvasStore.persist.rehydrate();
    expect(useCanvasStore.getState().hydrated).toBe(false);
    expect(useCanvasStore.getState().hydrationError).toContain("无法可靠转换");
    await expect(flushCanvasStorePersistence()).rejects.toThrow("无法可靠转换");
    expect(dom.localStorage.getItem(journalKey)).toBe(journal);
    expect(writes).toHaveLength(writesBefore);
});
