import { afterAll, expect, mock, test } from "bun:test";
import { Window } from "happy-dom";
const dom = new Window({ url: "http://localhost/" });
for (const [key, value] of Object.entries({ window: dom, document: dom.document, navigator: dom.navigator, HTMLElement: dom.HTMLElement, Element: dom.Element, Node: dom.Node, Event: dom.Event, IS_REACT_ACT_ENVIRONMENT: true }))
    Object.defineProperty(globalThis, key, { configurable: true, writable: true, value });
const React = await import("react");
const { act } = React;
const { createRoot } = await import("react-dom/client");
const { useUserStore } = await import("@/stores/use-user-store");
let reads = 0,
    starts: any[] = [],
    checks: string[] = [],
    confirmation: any,
    destroyed = 0;
const Box = ({ children }: any) => <div>{children}</div>;
const app = {
    message: { success() {}, error() {}, warning() {} },
    modal: {
        confirm(options: any) {
            confirmation = options;
            return {
                destroy() {
                    destroyed++;
                },
            };
        },
    },
};
mock.module("antd", () => ({
    App: { useApp: () => app },
    Skeleton: () => null,
    Input: { TextArea: () => null },
    Button: ({ children, disabled, onClick }: any) => (
        <button disabled={disabled} onClick={onClick}>
            {children}
        </button>
    ),
}));
mock.module("@/pages/admin/components/admin-shell", () => ({
    AdminPageFrame: ({ children, actions }: any) => (
        <main>
            {actions}
            {children}
        </main>
    ),
}));
mock.module("@/pages/admin/components/admin-ui", () => ({
    AdminStatusBadge: Box,
    SettingsSectionCard: ({ children, footer }: any) => (
        <section>
            {children}
            {footer}
        </section>
    ),
}));
const ready = {
    supported: true,
    connected: true,
    repository: "yuanxin2023/open-ai-canvas",
    currentVersion: "v1.2.2",
    latestRelease: { version: "v1.2.3", name: "release", body: "notes", url: "https://github.com/yuanxin2023/open-ai-canvas/releases/tag/v1.2.3", prerelease: false },
    updateAvailable: true,
    checks: [],
    operation: { phase: "ready", logs: [] },
};
mock.module("@/services/api/system-update", () => ({
    getSystemUpdateStatus: async () => {
        reads++;
        return ready;
    },
    checkSystemUpdate: async (actor: string) => {
        checks.push(actor);
        return ready;
    },
    startSystemUpdate: async (...args: any[]) => {
        starts.push(args);
        return { ...ready, updateAvailable: false, operation: { phase: "succeeded", logs: [] } };
    },
    rollbackSystemUpdate: async () => ready,
}));
const { default: SystemUpdatePage } = await import("@/pages/admin/settings/system-update-page");
const host = document.createElement("div");
document.body.append(host);
let root = createRoot(host);
async function fresh(admin = true) {
    await act(async () => {
        root.unmount();
        root = createRoot(host);
    });
    reads = 0;
    starts = [];
    checks = [];
    confirmation = null;
    destroyed = 0;
    useUserStore.setState({ user: { id: "admin-a", role: admin ? "admin" : "user", adminAccess: admin ? { level: "scoped", permissions: ["admin.settings.system_update"] } : undefined } as any });
    await act(async () => {
        root.render(<SystemUpdatePage />);
    });
}
async function startDialog() {
    const button = [...host.querySelectorAll("button")].find((item) => item.textContent?.trim() === "开始更新");
    if (!button) throw new Error("missing update command");
    await act(async () => {
        button.click();
    });
}
afterAll(async () => {
    await act(async () => root.unmount());
    dom.happyDOM.abort();
});
test("ordinary users cannot load deployment information", async () => {
    await fresh(false);
    expect(reads).toBe(0);
    expect(host.textContent).toContain("没有系统更新权限");
});
test("an available release does not start until explicit confirmation", async () => {
    await fresh();
    expect(reads).toBe(1);
    expect(starts).toHaveLength(0);
    expect(checks).toHaveLength(0);
    await act(async () => {
        [...host.querySelectorAll("button")].find((item) => item.textContent?.trim() === "检查更新")!.click();
    });
    expect(checks).toEqual(["admin-a"]);
    expect(starts).toHaveLength(0);
    await startDialog();
    expect(starts).toHaveLength(0);
    expect(confirmation).not.toBeNull();
    await act(async () => {
        await confirmation.onOk();
    });
    expect(starts).toEqual([["v1.2.3", "admin-a"]]);
});
test("switching accounts closes the dialog and rejects its stale callback", async () => {
    await fresh();
    await startDialog();
    const old = confirmation;
    await act(async () => {
        useUserStore.setState({ user: { id: "other", role: "user" } as any });
    });
    expect(destroyed).toBe(1);
    await expect(old.onOk()).rejects.toThrow("账号已变化");
    expect(starts).toHaveLength(0);
});
