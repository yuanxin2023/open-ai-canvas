import { afterAll, expect, mock, test } from "bun:test";
import { Window } from "happy-dom";
import type { FeatureAvailability } from "@/stores/use-user-store";

const dom = new Window({ url: "http://localhost/" });
for (const [key, value] of Object.entries({ window: dom, document: dom.document, navigator: dom.navigator, HTMLElement: dom.HTMLElement, Element: dom.Element, Node: dom.Node, Event: dom.Event, IS_REACT_ACT_ENVIRONMENT: true }))
    Object.defineProperty(globalThis, key, { configurable: true, writable: true, value });
const React = await import("react");
const { act } = React;
const { createRoot } = await import("react-dom/client");
const { MemoryRouter } = await import("react-router");
const { useUserStore, defaultFeatureAvailability } = await import("@/stores/use-user-store");
let refreshError = "";
let confirmed: { onOk: () => Promise<void> } | null = null;
let backend: FeatureAvailability = { ...defaultFeatureAvailability };
const writes: unknown[] = [];
const app = {
    message: { success() {}, error() {} },
    modal: {
        confirm(options: any) {
            confirmed = options;
        },
    },
};
mock.module("antd", () => ({
    App: { useApp: () => app },
    Button: ({ children, onClick, disabled }: any) => (
        <button onClick={onClick} disabled={disabled}>
            {children}
        </button>
    ),
    Skeleton: () => null,
    Popover: ({ children }: any) => children,
}));
mock.module("@/lib/user-session", () => ({
    refreshFeatureAvailability: async () => {
        if (refreshError) throw new Error(refreshError);
    },
}));
mock.module("@/components/layout/workspace-page", () => ({ WorkspacePage: ({ children }: any) => <main>{children}</main> }));
mock.module("@/components/layout/workspace-state", () => ({
    WorkspaceLoadingState: ({ label }: any) => <div>{label}</div>,
    WorkspaceErrorState: ({ title, description }: any) => (
        <div>
            {title}
            {description}
        </div>
    ),
    WorkspaceState: ({ title }: any) => <div>{title}</div>,
}));
mock.module("@/stores/use-appearance-store", () => ({ useAppearanceStore: () => ({ brandName: "local" }) }));
mock.module("@/hooks/use-workspace-logout", () => ({ useWorkspaceLogout: () => ({ handleLogout() {} }) }));
mock.module("@/components/brand/brand-logo", () => ({ BrandLogoFrame: () => null }));
mock.module("@/lib/workspace-route-modules", () => ({ preloadWorkspaceRoute() {} }));
mock.module("@/services/api/customer-service", () => ({ CUSTOMER_SERVICE_OPEN_EVENT: "customer-service", getPublicCustomerService: async () => null }));
mock.module("@/pages/admin/ui/controls", () => ({ Switch: ({ checked, disabled, onChange, ...props }: any) => <button {...props} role="switch" aria-checked={checked} disabled={disabled} onClick={() => onChange(!checked)} /> }));
mock.module("@/pages/admin/components/admin-ui", () => ({ AdminStatusBadge: ({ label }: any) => <span>{label}</span> }));
mock.module("@/services/api/auth", () => ({
    getAdminFeatureAvailability: async () => ({ features: backend }),
    updateAdminFeatureAvailability: async (input: Partial<FeatureAvailability>) => {
        writes.push(input);
        backend = { ...backend, ...input };
        return { features: backend };
    },
}));
const { RequireFeature } = await import("@/components/auth/require-feature");
const { buildNav, workspaceNavActiveId } = await import("@/components/layout/workspace-sidebar-nav");
const { default: FeatureAvailabilityPanel } = await import("@/pages/admin/components/feature-availability-panel");
const host = document.createElement("div");
document.body.append(host);
const root = createRoot(host);
async function render(ui: React.ReactNode) {
    await act(async () => {
        root.render(ui);
    });
}

test("closed entries preserve local home, canvas, prompts and skill visibility rules", () => {
    const { groups } = buildNav({ ...defaultFeatureAvailability, skillLibraryEnabled: false });
    const items = groups.flatMap((group) => group.items);
    expect(items.find((item) => item.id === "home")?.to).toBe("/?home=1");
    expect(items.find((item) => item.id === "canvas")?.to).toBe("/canvas");
    expect(items.find((item) => item.id === "prompts")?.to).toBe("/prompts");
    expect(items.some((item) => item.id === "skills")).toBe(false);
    expect(items.some((item) => item.id.startsWith("toolbox-"))).toBe(false);
});
test("each workbench has an independent optional entry in resources and tools", () => {
    const result = buildNav({ ...defaultFeatureAvailability, imageWorkbenchEnabled: true });
    const tools = result.groups.find((group) => group.heading === "资源与工具")!;
    expect(tools.items.find((item) => item.id === "toolbox-image")?.to).toBe("/toolbox/image");
    expect(tools.items.some((item) => item.id === "toolbox-video")).toBe(false);
    expect(workspaceNavActiveId("/toolbox/image")).toBe("toolbox-image");
    expect(workspaceNavActiveId("/toolbox/video")).toBe("toolbox-video");
    expect(workspaceNavActiveId("/settings", "wallet")).toBe("wallet");
});
for (const role of ["user", "admin"] as const)
    test(`closed route also rejects ${role} access`, async () => {
        useUserStore.setState({ user: { id: role, role } as any, features: { ...defaultFeatureAvailability } });
        refreshError = "";
        await render(
            <MemoryRouter key={role}>
                <RequireFeature feature="imageWorkbenchEnabled">
                    <div>protected-child</div>
                </RequireFeature>
            </MemoryRouter>,
        );
        expect(host.textContent).not.toContain("protected-child");
        expect(host.textContent).toContain("暂未开放");
    });
test("enabled route renders the actual child", async () => {
    useUserStore.setState({ features: { ...defaultFeatureAvailability, videoWorkbenchEnabled: true } });
    await render(
        <MemoryRouter key="enabled">
            <RequireFeature feature="videoWorkbenchEnabled">
                <div>protected-child</div>
            </RequireFeature>
        </MemoryRouter>,
    );
    expect(host.textContent).toContain("protected-child");
});
test("failed feature refresh blocks the child", async () => {
    refreshError = "feature read failed";
    await render(
        <MemoryRouter key="failure">
            <RequireFeature feature="imageWorkbenchEnabled">
                <div>protected-child</div>
            </RequireFeature>
        </MemoryRouter>,
    );
    expect(host.textContent).toContain("无法确认功能状态");
    expect(host.textContent).not.toContain("protected-child");
    refreshError = "";
});
test("admin toggle requires confirmation and preserves local disabled settings", async () => {
    backend = { ...defaultFeatureAvailability, skillLibraryEnabled: false, creditsEnabled: false, customChannelsEnabled: false };
    writes.length = 0;
    confirmed = null;
    await render(<FeatureAvailabilityPanel />);
    const image = host.querySelector<HTMLButtonElement>('button[aria-label="开启图片生成，需二次确认"]')!;
    expect(image).not.toBeNull();
    await act(async () => {
        image.click();
    });
    expect(writes).toHaveLength(0);
    expect(confirmed).not.toBeNull();
    await act(async () => {
        await confirmed!.onOk();
    });
    expect(writes).toHaveLength(1);
    expect(writes[0]).toMatchObject({ imageWorkbenchEnabled: true, videoWorkbenchEnabled: false, skillLibraryEnabled: false, creditsEnabled: false, customChannelsEnabled: false });
    expect(useUserStore.getState().features.skillLibraryEnabled).toBe(false);
    expect(useUserStore.getState().features.imageWorkbenchEnabled).toBe(true);
});
for (const [feature, title] of [
    ["inspirationHallEnabled", "灵感大厅"],
    ["inspirationGalleryEnabled", "灵感画廊"],
    ["userFeedbackEnabled", "用户反馈"],
] as const)
    test(`${title} remains optional and requires administrator confirmation`, async () => {
        backend = { ...defaultFeatureAvailability, skillLibraryEnabled: false, creditsEnabled: false };
        writes.length = 0;
        confirmed = null;
        await render(<FeatureAvailabilityPanel key={feature} />);
        const toggle = host.querySelector<HTMLButtonElement>(`button[aria-label="开启${title}，需二次确认"]`)!;
        expect(toggle).not.toBeNull();
        await act(async () => {
            toggle.click();
        });
        expect(writes).toHaveLength(0);
        expect(confirmed).not.toBeNull();
        await act(async () => {
            await confirmed!.onOk();
        });
        expect(writes).toHaveLength(1);
        expect(writes[0]).toMatchObject({ [feature]: true, [feature === "inspirationHallEnabled" ? "inspirationGalleryEnabled" : "inspirationHallEnabled"]: false, skillLibraryEnabled: false, creditsEnabled: false });
    });

afterAll(async () => {
    await act(async () => root.unmount());
    dom.close();
});
