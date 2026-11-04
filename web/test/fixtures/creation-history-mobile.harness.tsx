import { afterAll, expect, mock, test } from "bun:test";
import { Window } from "happy-dom";

const dom = new Window({ url: "http://localhost/create", width: 390, height: 844 });
for (const name of ["HTMLAnchorElement", "HTMLButtonElement", "HTMLInputElement", "HTMLTextAreaElement", "HTMLSelectElement", "MutationObserver", "KeyboardEvent", "FocusEvent"]) {
    Object.defineProperty(globalThis, name, { configurable: true, value: (dom as any)[name] });
}
for (const [key, value] of Object.entries({ window: dom, document: dom.document, navigator: dom.navigator, HTMLElement: dom.HTMLElement, Element: dom.Element, Node: dom.Node, SVGElement: dom.SVGElement, ShadowRoot: dom.ShadowRoot, ResizeObserver: dom.ResizeObserver, getComputedStyle: dom.getComputedStyle.bind(dom), requestAnimationFrame: dom.requestAnimationFrame.bind(dom), IS_REACT_ACT_ENVIRONMENT: true })) {
    Object.defineProperty(globalThis, key, { configurable: true, writable: true, value });
}
const records = new Map<string, any>();
const storage = {
    getItem: async (key: string) => records.get(key) ?? null,
    setItem: async (key: string, value: any) => { records.set(key, value); },
    removeItem: async (key: string) => { records.delete(key); },
};
mock.module("localforage", () => ({ default: { ...storage, config: () => {}, INDEXEDDB: "indexeddb", createInstance: () => storage } }));
const { act } = await import("react");
const { createRoot } = await import("react-dom/client");
const { MemoryRouter } = await import("react-router");
const { App } = await import("antd");
const workspace = await import("../../src/pages/create/creation-workspace");
mock.module("@/pages/create/creation-workspace", () => ({ ...workspace,
    CreationComposer: () => <textarea data-testid="composer" defaultValue="未发送的草稿" />,
    CreationMessageView: ({ item }: any) => <div data-testid="message">{item.content}</div>,
    CreationWorkspaceToolbar: () => null,
}));
const { default: CreatePage } = await import("../../src/pages/create/index");
const { WorkspaceAuthProvider } = await import("../../src/components/auth/workspace-auth-provider");
const { useUserStore } = await import("../../src/stores/use-user-store");
const { useModelCatalogStore } = await import("../../src/stores/use-model-catalog-store");
const { useCreationPreferencesStore } = await import("../../src/stores/use-creation-preferences-store");
const { apiClient } = await import("../../src/services/api/request");
apiClient.request = (async () => ({ data: { code: 0, data: { models: [], personal: [], public: [] }, msg: "ok" } })) as any;
const host = document.createElement("div");
document.body.append(host);
const root = createRoot(host);
const preference = "creation.sidebar.conversations-open";
const history = ["测试生图一", "测试生图二"].map((title, i) => ({
    id: `history-${i}`, title, titleEdited: true, updatedAt: new Date().toISOString(), messages: [
        { id: `prompt-${i}`, role: "user", mode: "image", content: title, createdAt: new Date().toISOString() },
        { id: `result-${i}`, role: "assistant", mode: "image", content: "任务处理中", status: "pending", createdAt: new Date().toISOString() },
    ],
}));
afterAll(async () => { await act(async () => root.unmount()); dom.happyDOM.abort(); });
async function frame() { await act(async () => new Promise<void>((resolve) => dom.requestAnimationFrame(() => resolve()))); }
async function openHistory() { await act(async () => host.querySelector<HTMLButtonElement>('[aria-label="展开创作对话"]')!.click()); }

test("mobile history opens on demand, switches and closes without unmounting the thread", async () => {
    dom.localStorage.setItem(preference, "0");
    records.set("creation-conversations-v1:user:guest", JSON.stringify(history));
    await act(async () => {
        useUserStore.setState({ hydrated: true, user: null });
        useCreationPreferencesStore.setState({ hydrated: true, preferences: {} });
        useModelCatalogStore.setState({ status: "ready" });
        root.render(<App><MemoryRouter initialEntries={["/create"]}><WorkspaceAuthProvider><CreatePage /></WorkspaceAuthProvider></MemoryRouter></App>);
    });
    expect(host.textContent).toContain("任务处理中");
    expect(document.querySelector(".creation-history-panel")).toBeNull();
    const composer = host.querySelector('[data-testid="composer"]');
    await openHistory();
    expect(document.querySelector(".ant-drawer-left")).not.toBeNull();
    expect(document.querySelectorAll(".creation-conversation-sidebar-item-main")).toHaveLength(2);
    await act(async () => [...document.querySelectorAll<HTMLButtonElement>(".creation-conversation-sidebar-item-main")].find((button) => button.textContent!.includes("测试生图二"))!.click());
    await frame();
    expect(host.querySelector('[aria-label="展开创作对话"]')!.getAttribute("aria-expanded")).toBe("false");
    expect(document.activeElement).toBe(host.querySelector('[aria-label="展开创作对话"]'));
    expect(host.querySelector('[data-testid="composer"]')).toBe(composer);
    expect(host.textContent).toContain("测试生图二");
    expect(host.textContent).toContain("任务处理中");
    expect(dom.localStorage.getItem(preference)).toBe("0");
});

test("collapse, mask close and new conversation close the mobile drawer", async () => {
    await openHistory();
    await act(async () => document.querySelector<HTMLButtonElement>('[aria-label="收起创作对话"]')!.click());
    await frame();
    expect(host.querySelector('[aria-label="展开创作对话"]')!.getAttribute("aria-expanded")).toBe("false");
    await openHistory();
    await act(async () => document.querySelector<HTMLElement>(".creation-history-panel .ant-drawer-mask")!.click());
    await frame();
    expect(host.querySelector('[aria-label="展开创作对话"]')!.getAttribute("aria-expanded")).toBe("false");
    await openHistory();
    await act(async () => document.querySelector<HTMLButtonElement>(".creation-conversation-sidebar-new")!.click());
    await frame();
    expect(host.querySelector('[aria-label="展开创作对话"]')!.getAttribute("aria-expanded")).toBe("false");
    expect(host.textContent).toContain("开始新的创作对话");
});

test("desktop preference stays independent and resizing never opens the mobile drawer", async () => {
    await act(async () => dom.happyDOM.setWindowSize({ width: 1440, height: 1000 }));
    expect(document.querySelector(".creation-history-panel")).toBeNull();
    expect(host.querySelector(".creation-conversation-sidebar")).toBeNull();
    await openHistory();
    expect(host.querySelector(".creation-thread-workbench > .creation-conversation-sidebar")).not.toBeNull();
    expect(dom.localStorage.getItem(preference)).toBe("1");
    await act(async () => dom.happyDOM.setWindowSize({ width: 390, height: 844 }));
    expect(host.querySelector(".creation-conversation-sidebar")).toBeNull();
    expect(host.querySelector('[aria-label="展开创作对话"]')!.getAttribute("aria-expanded")).toBe("false");
    await openHistory();
    await act(async () => dom.happyDOM.setWindowSize({ width: 1440, height: 1000 }));
    expect(host.querySelector(".creation-thread-workbench > .creation-conversation-sidebar")).not.toBeNull();
    await act(async () => dom.happyDOM.setWindowSize({ width: 390, height: 844 }));
    expect(host.querySelector('[aria-label="展开创作对话"]')!.getAttribute("aria-expanded")).toBe("false");
});
