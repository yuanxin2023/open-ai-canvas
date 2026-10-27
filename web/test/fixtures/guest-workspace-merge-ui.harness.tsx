import { afterAll, expect, mock, test } from "bun:test";
import { Window } from "happy-dom";
import type { ReactNode } from "react";
import type { LocalUser } from "@/stores/use-user-store";

const dom = new Window({ url: "http://localhost/create" });
for (const [key, value] of Object.entries({ window: dom, document: dom.document, navigator: dom.navigator, HTMLElement: dom.HTMLElement, Element: dom.Element, Node: dom.Node, Event: dom.Event, IS_REACT_ACT_ENVIRONMENT: true }))
    Object.defineProperty(globalThis, key, { configurable: true, writable: true, value });
const React = await import("react");
const { act } = React;
const { createRoot } = await import("react-dom/client");
const { MemoryRouter, Route, Routes, useLocation } = await import("react-router");
const { useUserStore } = await import("@/stores/use-user-store");
let canvasHydrated = false;
let creates = 0;
let loginRequests = 0;
const errors: string[] = [];
let createCanvas: () => Promise<{ id: string }> = async () => ({ id: "fixture-canvas" });
mock.module("antd", () => ({
    App: { useApp: () => ({ message: { error: (text: string) => errors.push(text) } }) },
    Button: ({ children, onClick, disabled }: any) => (
        <button onClick={onClick} disabled={disabled}>
            {children}
        </button>
    ),
}));
mock.module("@/components/ui/fluid-orb", () => ({ FluidOrb: () => null }));
mock.module("@/components/ui/aceternity/full-screen-loader", () => ({ FullScreenLoader: () => <div>loading-session</div> }));
mock.module("@/stores/canvas/use-canvas-store", () => ({ useCanvasStore: (select: (state: { hydrated: boolean }) => unknown) => select({ hydrated: canvasHydrated }) }));
mock.module("@/services/user-data-sync", () => ({
    hasRemoteUserDataSyncSession: () => true,
    createCanvasProjectWithRemoteSync: async () => {
        creates++;
        return createCanvas();
    },
    saveRemoteUserDataNow: async () => undefined,
}));
const { RequireAuth } = await import("@/components/auth/require-auth");
const { CreationAgentEntry } = await import("@/pages/create/creation-agent-entry");
const { isGuestWorkspacePath } = await import("@/lib/guest-workspace");
const host = document.createElement("div");
document.body.append(host);
const root = createRoot(host);
afterAll(async () => {
    await act(async () => root.unmount());
    dom.happyDOM.abort();
});
const user = (id: string) => ({ id, username: id, role: "user", status: "active" }) as LocalUser;
function Location() {
    const location = useLocation();
    return (
        <output>
            {location.pathname}
            {location.search}
        </output>
    );
}
function Guarded() {
    const location = useLocation();
    return (
        <RequireAuth allowAnonymous={isGuestWorkspacePath(location.pathname)}>
            <div>workspace</div>
        </RequireAuth>
    );
}
async function renderRoute(path: string, content: ReactNode = <Guarded />) {
    await act(async () =>
        root.render(
            <MemoryRouter key={path} initialEntries={[path]}>
                <Routes>
                    <Route path="/login" element={<Location />} />
                    <Route
                        path="*"
                        element={
                            <>
                                {content}
                                <Location />
                            </>
                        }
                    />
                </Routes>
            </MemoryRouter>,
        ),
    );
}

test("guests can browse both home routes while private routes redirect with their return address", async () => {
    await act(async () => useUserStore.setState({ hydrated: true, user: null }));
    for (const path of ["/", "/create"]) {
        await renderRoute(path);
        expect(host.textContent).toContain("workspace");
        expect(host.querySelector("output")!.textContent).toBe(path);
    }
    await renderRoute("/canvas/private?agent=1");
    expect(host.textContent).not.toContain("workspace");
    expect(host.querySelector("output")!.textContent).toBe("/login?next=%2Fcanvas%2Fprivate%3Fagent%3D1");
});

test("unresolved sessions cannot briefly expose workspace content", async () => {
    await act(async () => useUserStore.setState({ hydrated: false, user: null }));
    await renderRoute("/create");
    expect(host.textContent).toContain("loading-session");
    expect(host.textContent).not.toContain("workspace");
    await act(async () => useUserStore.setState({ hydrated: true }));
    expect(host.textContent).toContain("workspace");
});

test("guest Agent activation requests login before creating a canvas or task", async () => {
    canvasHydrated = false;
    await act(async () => useUserStore.setState({ hydrated: true, user: null }));
    await renderRoute("/create", <CreationAgentEntry onRequireLogin={() => loginRequests++} />);
    const button = host.querySelector("button")!;
    expect(button.disabled).toBe(false);
    await act(async () => button.click());
    expect(loginRequests).toBe(1);
    expect(creates).toBe(0);
    expect(errors).toEqual([]);
});

test("a canvas creation finishing after an account switch cannot navigate the new account", async () => {
    canvasHydrated = true;
    let finish!: (result: { id: string }) => void;
    createCanvas = () =>
        new Promise((resolve) => {
            finish = resolve;
        });
    await act(async () => useUserStore.setState({ user: user("owner") }));
    await renderRoute("/create", <CreationAgentEntry />);
    await act(async () => host.querySelector("button")!.click());
    expect(creates).toBe(1);
    await act(async () => {
        useUserStore.setState({ user: user("next-owner") });
        finish({ id: "owner-canvas" });
    });
    expect(host.querySelector("output")!.textContent).toBe("/create");
    expect(errors).toEqual([]);
});
