import { afterAll, expect, test } from "bun:test";
import { Window } from "happy-dom";

const dom = new Window({ url: "http://localhost/" });
for (const [key, value] of Object.entries({
    window: dom,
    document: dom.document,
    navigator: dom.navigator,
    HTMLElement: dom.HTMLElement,
    Element: dom.Element,
    Node: dom.Node,
    Event: dom.Event,
    SVGElement: dom.SVGElement,
    ShadowRoot: dom.ShadowRoot,
    MutationObserver: dom.MutationObserver,
    ResizeObserver: dom.ResizeObserver,
    getComputedStyle: dom.getComputedStyle.bind(dom),
    requestAnimationFrame: dom.requestAnimationFrame.bind(dom),
    cancelAnimationFrame: dom.cancelAnimationFrame.bind(dom),
    IS_REACT_ACT_ENVIRONMENT: true,
    __APP_VERSION__: "1.6.9.2",
}))
    Object.defineProperty(globalThis, key, { configurable: true, writable: true, value });
const { act } = await import("react");
const { createRoot } = await import("react-dom/client");
const { App } = await import("antd");
const { CreationFeaturedWorks } = await import("@/pages/create/creation-workspace");
const { useUserStore } = await import("@/stores/use-user-store");
const { apiClient } = await import("@/services/api/request");
const originalAdapter = apiClient.defaults.adapter;
const host = document.createElement("div");
document.body.append(host);
const root = createRoot(host);
let requests = 0;
let logins = 0;
const render = () =>
    root.render(
        <App>
            <CreationFeaturedWorks
                onStartPrompt={() => undefined}
                onRequireLogin={() => {
                    logins++;
                }}
            />
        </App>,
    );
afterAll(async () => {
    await act(async () => root.unmount());
    apiClient.defaults.adapter = originalAdapter;
    dom.happyDOM.abort();
});

test("guests browse inspiration tabs without calling protected APIs and explicitly request login", async () => {
    requests = 0;
    logins = 0;
    apiClient.defaults.adapter = async () => {
        requests++;
        throw new Error("Protected request must not run for guests");
    };
    await act(async () => {
        useUserStore.setState({ user: null, hydrated: true });
        render();
    });
    expect(host.textContent).toContain("登录后查看精选灵感和个人灵感");
    const personal = [...host.querySelectorAll<HTMLButtonElement>("button")].find((button) => button.textContent === "个人灵感")!;
    await act(async () => personal.click());
    expect(host.querySelector('[aria-label="新增个人提示词"]')).toBeNull();
    expect(host.textContent).not.toContain("重新加载");
    await act(async () => [...host.querySelectorAll<HTMLButtonElement>("button")].find((button) => button.textContent?.replace(/\s/g, "") === "登录")!.click());
    expect(logins).toBe(1);
    expect(requests).toBe(0);
});

test("a protected response arriving after logout cannot replace the guest prompt", async () => {
    let finish: (() => void) | undefined;
    apiClient.defaults.adapter = (config) =>
        new Promise((resolve) => {
            requests++;
            finish = () =>
                resolve({ config, status: 200, statusText: "OK", headers: {}, data: { code: 0, data: { inspirations: [{ id: "late", title: "late-featured", mode: "image", prompt: "late", tags: [], status: "active", coverUrl: "" }] }, msg: "ok" } });
        });
    try {
        await act(async () => {
            useUserStore.setState({ user: { id: "fixture-user", username: "fixture", role: "user", status: "active", createdAt: "", updatedAt: "" } as any });
            root.render(
                <App key="signed-in">
                    <CreationFeaturedWorks
                        onStartPrompt={() => undefined}
                        onRequireLogin={() => {
                            logins++;
                        }}
                    />
                </App>,
            );
        });
        expect(finish).toBeDefined();
        await act(async () => {
            useUserStore.setState({ user: null });
            finish!();
        });
        expect(host.textContent).toContain("登录后查看精选灵感和个人灵感");
        expect(host.textContent).not.toContain("late-featured");
    } finally {
        await act(async () => useUserStore.setState({ user: null }));
    }
});
