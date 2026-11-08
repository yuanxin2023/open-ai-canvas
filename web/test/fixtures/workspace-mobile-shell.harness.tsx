import { afterAll, expect, mock, test } from "bun:test";
import { Window } from "happy-dom";
const dom = new Window({ url: "http://localhost/create", width: 390, height: 844 });
for (const name of ["Event", "CustomEvent"]) Object.defineProperty(globalThis, name, { configurable: true, value: (dom as any)[name] });
for (const name of ["HTMLElement", "HTMLButtonElement", "HTMLInputElement", "HTMLTextAreaElement", "HTMLSelectElement", "Element", "Node", "SVGElement", "ShadowRoot", "MutationObserver", "ResizeObserver", "KeyboardEvent"])
    Object.defineProperty(globalThis, name, { configurable: true, value: (dom as any)[name] });
for (const [name, value] of Object.entries({ window: dom, document: dom.document, navigator: dom.navigator, getComputedStyle: dom.getComputedStyle.bind(dom), requestAnimationFrame: dom.requestAnimationFrame.bind(dom), IS_REACT_ACT_ENVIRONMENT: true }))
    Object.defineProperty(globalThis, name, { configurable: true, value });
mock.module("localforage", () => ({ default: { config: () => {}, getItem: async () => null, setItem: async () => {}, createInstance: () => ({ getItem: async () => null, setItem: async () => {} }) } }));
mock.module("@/hooks/use-workspace-document-title", () => ({ useWorkspaceDocumentTitle: () => {} }));
mock.module("@/hooks/use-wallet-balance", () => ({ useWalletBalance: () => ({ availableMicrocredits: 123456000000 }) }));
mock.module("@/components/layout/generation-success-notifier", () => ({ GenerationSuccessNotifier: () => null }));
mock.module("@/components/layout/workspace-sidebar-checkin", () => ({ WorkspaceSidebarCheckin: () => null }));
mock.module("@/components/layout/workspace-sidebar-storage-meter", () => ({ WorkspaceSidebarStorageMeter: () => null }));
mock.module("@/components/layout/workspace-account-card", () => ({ WorkspaceAccountCard: () => null }));
for (const [path, name] of [
    ["banner-announcements-slider", "BannerAnnouncementsSlider"],
    ["model-setup-guide", "ModelSetupGuide"],
    ["workspace-package-center", "WorkspacePackageCenterHost"],
    ["image-credit-shortfall-host", "ImageCreditShortfallHost"],
    ["workspace-storage-limit-notice", "WorkspaceStorageLimitNotice"],
    ["workspace-top-bar-checkin", "WorkspaceTopBarCheckin"],
] as const)
    mock.module(`@/components/layout/${path}`, () => ({ [name]: () => null }));
mock.module("@/components/feedback/topbar-feedback-button", () => ({
    TopbarFeedbackButton: () => (
        <button aria-label="反馈" className="app-workspace-topbar-feedback-pill">
            反馈
        </button>
    ),
}));
mock.module("@/components/layout/system-announcement-center", () => ({
    SystemAnnouncementCenter: () => (
        <button aria-label="通知" className="app-workspace-topbar-icon-button">
            通知
        </button>
    ),
}));
mock.module("@/components/layout/workspace-credit-popover", () => ({ WorkspaceCreditPopover: () => <button className="app-workspace-credit-button">123456</button> }));
mock.module("@/components/layout/workspace-account-menu", () => ({
    WorkspaceAccountMenu: () => (
        <button aria-label="账户" className="app-workspace-topbar-icon-button">
            账户
        </button>
    ),
}));
const { act } = await import("react");
const { createRoot } = await import("react-dom/client");
const { MemoryRouter, useNavigate } = await import("react-router");
const { App } = await import("antd");
const { AppWorkspaceShell: Shell } = await import("../../src/components/layout/app-top-nav");
const { CreationWorkspaceToolbar } = await import("../../src/pages/create/creation-workspace");
const { WorkspaceAuthContext } = await import("../../src/components/auth/workspace-auth-context");
const { useUserStore } = await import("../../src/stores/use-user-store");
const { publishWorkspaceSidebarCollapsed, readWorkspaceSidebarCollapsed } = await import("../../src/components/layout/workspace-sidebar-state");
const { apiClient } = await import("../../src/services/api/request");
apiClient.request = (async () => ({ data: { code: 0, data: { customerService: { enabled: false } }, msg: "ok" } })) as any;
const host = document.createElement("div");
document.body.append(host);
const root = createRoot(host);
let navigate: ReturnType<typeof useNavigate>;
function Screen() {
    navigate = useNavigate();
    return (
        <Shell>
            <CreationWorkspaceToolbar shots={[]} onJumpToShot={() => {}} onNewConversation={() => {}} onContinueCanvas={() => {}} openingCanvas={false} />
            <textarea defaultValue="保留当前任务与输入" />
        </Shell>
    );
}
const screen = (key: string) => (
    <App>
        <MemoryRouter key={key} initialEntries={["/create"]}>
            <WorkspaceAuthContext.Provider value={{ openAuth: async () => {}, prepareDraft: { current: null } }}>
                <Screen />
            </WorkspaceAuthContext.Provider>
        </MemoryRouter>
    </App>
);
afterAll(async () => {
    await act(async () => root.unmount());
    dom.happyDOM.abort();
});
async function frame() {
    await act(async () => new Promise<void>((resolve) => dom.requestAnimationFrame(() => resolve())));
}
test("mobile navigation opens fully even when generation collapsed the desktop rail", async () => {
    await act(async () => {
        useUserStore.setState({ hydrated: true, user: { id: "fixture-user", username: "fixture", role: "user", status: "active" } as any });
        publishWorkspaceSidebarCollapsed(true);
        root.render(screen("mobile"));
    });
    const input = host.querySelector("textarea");
    expect(host.querySelector(".app-workspace-sidebar")).toBeNull();
    await act(async () => host.querySelector<HTMLButtonElement>('[aria-label="展开侧栏"]')!.click());
    expect(document.querySelector(".workspace-navigation-drawer.ant-drawer-open")).not.toBeNull();
    expect(document.querySelector(".workspace-navigation-drawer .app-workspace-sidebar-nav.is-collapsed")).toBeNull();
    expect(document.querySelector('.workspace-navigation-drawer [data-nav-id="assets"]')).not.toBeNull();
    await act(async () => document.querySelector<HTMLButtonElement>('.workspace-navigation-drawer [aria-label="收起侧栏"]')!.click());
    await frame();
    expect(host.querySelector("textarea")).toBe(input);
    expect(document.activeElement).toBe(host.querySelector('[aria-label="展开侧栏"]'));
    expect(readWorkspaceSidebarCollapsed()).toBe(true);
});
test("navigation selection closes the drawer and keeps the desktop preference", async () => {
    await act(async () => host.querySelector<HTMLButtonElement>('[aria-label="展开侧栏"]')!.click());
    await act(async () => document.querySelector<HTMLAnchorElement>('.workspace-navigation-drawer [data-nav-id="home"]')!.click());
    await frame();
    expect(host.querySelector('[aria-label="展开侧栏"]')!.getAttribute("aria-expanded")).toBe("false");
    expect(readWorkspaceSidebarCollapsed()).toBe(true);
    expect(host.querySelector(".app-workspace-topbar-extension")!.textContent).toContain("镜头时间线");
    expect(host.querySelector(".app-workspace-topbar-extension")!.textContent).toContain("画布中继续");
});
test("desktop reload preserves the rail preference and canvas detail hides navigation chrome", async () => {
    await act(async () => dom.happyDOM.setWindowSize({ width: 1440, height: 1000 }));
    await act(async () => root.render(screen("desktop")));
    expect(document.querySelector(".workspace-navigation-drawer")).toBeNull();
    expect(host.querySelector(".app-workspace-sidebar.is-collapsed")).not.toBeNull();
    await act(async () => dom.happyDOM.setWindowSize({ width: 390, height: 844 }));
    await act(async () => root.render(screen("mobile-again")));
    expect(host.querySelector('[aria-label="展开侧栏"]')!.getAttribute("aria-expanded")).toBe("false");
    await act(async () => navigate("/canvas/fixture"));
    expect(host.querySelector(".app-workspace-topbar")).toBeNull();
    expect(document.querySelector(".workspace-navigation-drawer")).toBeNull();
});
