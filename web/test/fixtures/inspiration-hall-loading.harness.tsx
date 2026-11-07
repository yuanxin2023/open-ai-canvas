import { afterAll, expect, mock, test } from "bun:test";
import { Window } from "happy-dom";

const dom = new Window({ url: "http://localhost/" });
for (const [key, value] of Object.entries({ window: dom, document: dom.document, navigator: dom.navigator, HTMLElement: dom.HTMLElement, Element: dom.Element, Node: dom.Node, Event: dom.Event, IS_REACT_ACT_ENVIRONMENT: true }))
    Object.defineProperty(globalThis, key, { configurable: true, writable: true, value });
const { act } = await import("react");
const { createRoot } = await import("react-dom/client");
const Box = ({ children }: any) => <div>{children}</div>;
mock.module("antd", () => ({
    App: { useApp: () => ({ message: { error() {}, success() {} }, modal: { confirm: () => ({ destroy() {} }) } }) },
    Button: ({ children, onClick, disabled, title, ...props }: any) => (
        <button aria-label={props["aria-label"]} title={title} disabled={disabled} onClick={onClick}>
            {children}
        </button>
    ),
    Dropdown: Box,
    Input: ({ value, onChange, ...props }: any) => <input aria-label={props["aria-label"]} value={value} onChange={onChange} />,
}));
mock.module("@/components/layout/workspace-page", () => ({ PaginationBar: () => null }));
mock.module("@/components/layout/workspace-state", () => ({
    WorkspaceLoadingState: ({ label }: any) => <div role="status">{label}</div>,
    WorkspaceErrorState: ({ title, description, onRetry }: any) => (
        <div role="alert">
            {title}
            {description}
            <button onClick={onRetry}>重试</button>
        </div>
    ),
    WorkspaceState: ({ title }: any) => <div>{title}</div>,
}));
mock.module("@/components/ui/base/segmented-control", () => ({ SegmentedControl: () => null }));
mock.module("@/components/ui/base/select", () => ({ Select: () => null }));
mock.module("@/components/ui/base/tooltip", () => ({ Tooltip: Box }));
mock.module("@/components/ui/product/app-modal", () => ({ AppModal: ({ open, children }: any) => (open ? <div role="dialog">{children}</div> : null) }));
mock.module("@/components/ui/product/media-placeholder", () => ({ MediaPlaceholder: () => null }));
mock.module("@/pages/inspirations/inspiration-editor-modal", () => ({ InspirationEditorModal: () => null }));
const { InspirationHall } = await import("@/pages/inspirations");
const { useUserStore, defaultFeatureAvailability } = await import("@/stores/use-user-store");
const { useInspirationStore } = await import("@/stores/use-inspiration-store");
const { apiClient } = await import("@/services/api/request");
const { invalidateInspirationCatalog } = await import("@/services/inspiration-catalog");
const originalFetch = globalThis.fetch;
const originalAdapter = apiClient.defaults.adapter;
let directoryRequests = 0;
const entries = [{ id: "seedance:static", sourceId: "seedance", title: "公共灵感", description: "", image: "/local.webp", mode: "video", prompt: "Public draft" }];
globalThis.fetch = (async (input) => {
    directoryRequests++;
    return new Response(JSON.stringify(String(input).endsWith("manifest.json") ? { sources: {} } : { entries: String(input).endsWith("seedance.json") ? entries : [] }));
}) as typeof fetch;
const host = document.createElement("div");
document.body.append(host);
const root = createRoot(host);
let reads = 0;
let writes = 0;
let read: () => Promise<any>;
let used = "";
function view(title = "个人灵感") {
    return { revision: 1, prompts: [{ id: "own", title, description: "", mode: "image", prompt: "Own draft", tags: [] }], state: { favorites: [], hidden: [], overrides: {}, importedCustom: {}, metadata: {} } };
}
function deferred<T>() {
    let resolve!: (value: T) => void;
    const promise = new Promise<T>((done) => {
        resolve = done;
    });
    return { resolve, promise };
}
async function flush(action: () => void = () => {}) {
    await act(async () => {
        action();
        await new Promise((resolve) => setTimeout(resolve, 0));
    });
}
async function setup(id?: string) {
    await flush(() => {
        useUserStore.setState({ user: null, features: { ...defaultFeatureAvailability } });
        root.render(null);
    });
    reads = 0;
    directoryRequests = 0;
    writes = 0;
    used = "";
    read = async () => view();
    apiClient.defaults.adapter = async (config) => {
        if (config.url === "/public/inspirations/feed") return { config, status: 200, statusText: "OK", headers: {}, data: { code: 0, data: { enabled: false }, msg: "ok" } };
        if (String(config.method).toLowerCase() !== "get") {
            writes++;
            throw new Error("Unexpected write");
        }
        expect(config.url).toBe("/inspiration-library");
        expect(config.timeout).toBe(15_000);
        reads++;
        const data = await read();
        return { config, status: 200, statusText: "OK", headers: {}, data: { code: 0, data, msg: "ok" } };
    };
    invalidateInspirationCatalog();
    await flush(() => useUserStore.setState({ user: id ? ({ id, role: "user" } as any) : null, features: { ...defaultFeatureAvailability, inspirationHallEnabled: true } }));
}
async function render() {
    await flush(() =>
        root.render(
            <InspirationHall
                onUse={(item) => {
                    used = item.prompt;
                }}
            />,
        ),
    );
    await flush();
}
function button(label: string) {
    const found = [...host.querySelectorAll<HTMLButtonElement>("button")].find((element) => element.textContent === label || element.getAttribute("aria-label") === label);
    if (!found) throw new Error(`Missing button: ${label}`);
    return found;
}
afterAll(async () => {
    await flush(() => root.unmount());
    globalThis.fetch = originalFetch;
    apiClient.defaults.adapter = originalAdapter;
    dom.happyDOM.abort();
});

test("signed-in hall starts its personal read and applies hidden entries before rendering", async () => {
    await setup("a");
    const pending = deferred<any>();
    read = () => pending.promise;
    await render();
    expect(reads).toBe(1);
    expect(host.textContent).toContain("正在准备灵感");
    expect(host.querySelector(".inspiration-card")).toBeNull();
    const next = view();
    next.state.hidden = ["seedance:static"] as any;
    await flush(() => pending.resolve(next));
    expect(host.textContent).toContain("个人灵感");
    expect(host.textContent).not.toContain("公共灵感");
    expect(button("新建灵感").disabled).toBe(false);
});

test("guest can browse, open details and fill a prompt without a protected read", async () => {
    await setup();
    await render();
    expect(reads).toBe(0);
    expect(useInspirationStore.getState().hydrated).toBe(false);
    expect(host.textContent).toContain("公共灵感");
    expect(button("新建灵感").disabled).toBe(true);
    expect(button("新建灵感").title).toContain("登录");
    expect(button("收藏：公共灵感").disabled).toBe(true);
    await flush(() => button("查看灵感：公共灵感").click());
    expect(host.querySelector('[role="dialog"]')).not.toBeNull();
    await flush(() => button("用这个创意创作").click());
    expect(used).toBe("Public draft");
});

test("personal read errors and timeouts expose retry without reloading the catalog", async () => {
    for (const reason of ["personal unavailable", "timeout of 15000ms exceeded"]) {
        await setup("a");
        read = async () => {
            throw new Error(reason);
        };
        await render();
        expect(host.textContent).toContain("个人灵感加载失败");
        expect(host.textContent).toContain(reason);
        const beforeRetry = directoryRequests;
        read = async () => view();
        await flush(() => button("重试").click());
        expect(reads).toBe(2);
        expect(directoryRequests).toBe(beforeRetry);
        expect(host.textContent).toContain("个人灵感");
        expect(host.querySelector('[role="alert"]')).toBeNull();
    }
});

test("directory errors expose their own retry and leave guests outside the personal library", async () => {
    await setup();
    const workingFetch = globalThis.fetch;
    globalThis.fetch = (async () => {
        throw new Error("灵感目录加载超时，请重试");
    }) as typeof fetch;
    try {
        await render();
        expect(host.textContent).toContain("灵感目录加载失败");
        expect(host.textContent).not.toContain("正在准备灵感");
    } finally {
        globalThis.fetch = workingFetch;
    }
    await flush(() => button("重试").click());
    expect(host.textContent).toContain("公共灵感");
    expect(reads).toBe(0);
});

test("focus refresh disables writes until the response arrives", async () => {
    await setup("a");
    await render();
    const pending = deferred<any>();
    read = () => pending.promise;
    await flush(() => window.dispatchEvent(new dom.Event("focus")));
    expect(button("新建灵感").disabled).toBe(true);
    await expect(useInspirationStore.getState().toggleFavorite("seedance:static")).rejects.toThrow("正在刷新");
    expect(writes).toBe(0);
    await flush(() => pending.resolve(view()));
    expect(button("新建灵感").disabled).toBe(false);
});

test("account changes discard old dialogs and late responses; feature closure unmounts", async () => {
    await setup("a");
    await render();
    await flush(() => button("查看灵感：个人灵感").click());
    const pending = deferred<any>();
    read = () => pending.promise;
    await flush(() => window.dispatchEvent(new dom.Event("focus")));
    read = async () => view("账号 B 灵感");
    await flush(() => useUserStore.setState({ user: { id: "b", role: "user" } as any }));
    await flush();
    expect(host.querySelector('[role="dialog"]')).toBeNull();
    await flush(() => pending.resolve(view("账号 A 迟到灵感")));
    expect(host.textContent).toContain("账号 B 灵感");
    expect(host.textContent).not.toContain("账号 A 迟到灵感");
    await flush(() => useUserStore.setState({ user: null }));
    await flush();
    expect(host.textContent).not.toContain("账号 B 灵感");
    expect(host.textContent).toContain("公共灵感");
    await flush(() => useUserStore.setState({ features: { ...defaultFeatureAvailability } }));
    expect(host.textContent).toBe("");
});
