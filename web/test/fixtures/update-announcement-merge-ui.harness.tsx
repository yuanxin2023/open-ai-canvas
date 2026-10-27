import { afterAll, expect, mock, test } from "bun:test";
import { Window } from "happy-dom";
const dom = new Window({ url: "http://localhost/" });
for (const [key, value] of Object.entries({ window: dom, document: dom.document, navigator: dom.navigator, HTMLElement: dom.HTMLElement, Element: dom.Element, Node: dom.Node, Event: dom.Event, IS_REACT_ACT_ENVIRONMENT: true, __APP_VERSION__: "1.6.9.2" }))
    Object.defineProperty(globalThis, key, { configurable: true, writable: true, value });
const React = await import("react");
const { act } = React;
const { createRoot } = await import("react-dom/client");
const { useAppearanceStore, DEFAULT_PUBLIC_APPEARANCE } = await import("@/stores/use-appearance-store");
let refreshes = 0;
mock.module("@/services/appearance-bootstrap", () => ({
    refreshPublicAppearance: async () => {
        refreshes++;
    },
}));
mock.module("@/components/layout/app-changelog-dialog", () => ({
    AppChangelogDialog: ({ onClose }: { onClose: () => void }) => (
        <div role="dialog">
            <button onClick={onClose}>关闭</button>
        </div>
    ),
}));
const { AppChangelogButton } = await import("@/components/layout/app-changelog-modal");
const host = document.createElement("div");
document.body.append(host);
const root = createRoot(host);
afterAll(async () => {
    await act(async () => root.unmount());
    dom.happyDOM.abort();
});

test("publishing or refreshing an announcement does not open a dialog", async () => {
    await act(async () => {
        useAppearanceStore.setState({ appearance: { ...DEFAULT_PUBLIC_APPEARANCE, updates: { enabled: true, currentVersion: "Studio 2", releases: [] } } });
        root.render(<AppChangelogButton showVersion showLabel />);
    });
    expect(host.textContent).toContain("更新公告");
    expect(host.textContent).toContain("Studio 2");
    expect(host.querySelector('[role="dialog"]')).toBeNull();
    expect(refreshes).toBe(0);
    await act(async () => {
        host.querySelector("button")!.click();
    });
    expect(refreshes).toBe(1);
    expect(host.querySelector('[role="dialog"]')).not.toBeNull();
    await act(async () => {
        host.querySelector('[role="dialog"] button')!.click();
    });
    expect(host.querySelector('[role="dialog"]')).toBeNull();
});

test("turning the custom switch off restores the original log and software version", async () => {
    await act(async () => {
        useAppearanceStore.setState({ appearance: { ...DEFAULT_PUBLIC_APPEARANCE } });
    });
    expect(host.textContent).toContain("更新日志");
    expect(host.textContent).toContain("v1.6.9.2");
    expect(host.textContent).not.toContain("Studio 2");
    expect(host.querySelector('[role="dialog"]')).toBeNull();
    expect(refreshes).toBe(1);
});
