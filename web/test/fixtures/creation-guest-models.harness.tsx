import { afterAll, expect, mock, test } from "bun:test";
import { Window } from "happy-dom";

const dom = new Window({ url: "http://localhost/create" });
Object.defineProperty(globalThis, "HTMLAnchorElement", { configurable: true, value: dom.HTMLAnchorElement });
Object.defineProperty(globalThis, "ResizeObserver", { configurable: true, value: dom.ResizeObserver });
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
    getComputedStyle: dom.getComputedStyle.bind(dom),
    IS_REACT_ACT_ENVIRONMENT: true,
})) {
    Object.defineProperty(globalThis, key, { configurable: true, writable: true, value });
}
const records = new Map<string, any>();
let failWrite = false;
mock.module("localforage", () => ({
    default: {
        INDEXEDDB: "indexeddb",
        config: () => {},
        getItem: async () => null,
        setItem: async () => {},
        removeItem: async () => {},
        createInstance: () => ({
            getItem: async (key: string) => records.get(key) || null,
            setItem: async (key: string, value: any) => {
                if (failWrite) throw new Error("storage unavailable");
                records.set(key, value);
            },
            removeItem: async (key: string) => {
                records.delete(key);
            },
        }),
    },
}));
const antd = await import("antd");
mock.module("antd", () => ({
    ...antd,
    Popover: ({ children, content }: any) => (
        <>
            {children}
            {content}
        </>
    ),
}));
const { act } = await import("react");
mock.module("@/components/auth/auth-login-form", () => ({ AuthLoginForm: Login }));
mock.module("@/components/auth/auth-register-form", () => ({ AuthRegisterForm: AuthSubView }));
mock.module("@/components/auth/auth-recovery-form", () => ({ AuthRecoveryForm: AuthSubView }));
mock.module("@/components/ui/product/app-modal", () => ({
    AppModal: ({ open, children, onCancel }: any) =>
        open ? (
            <div role="dialog">
                <button data-testid="close-auth" onClick={onCancel}>
                    关闭
                </button>
                {children}
            </div>
        ) : null,
}));
const { WorkspaceAuthProvider: ProductionAuthProvider } = await import("@/components/auth/workspace-auth-provider");
const { default: AuthDialog } = await import("@/components/auth/auth-dialog");
const { QueryClient, QueryClientProvider } = await import("@tanstack/react-query");
const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
const { useAuthSettingsStore } = await import("@/stores/use-auth-settings-store");
useAuthSettingsStore.setState({ settings: { loginMode: "modal" } as any, status: "ready", loadedAt: Date.now() });
const { WorkspaceSidebarNav } = await import("@/components/layout/workspace-sidebar-nav");
const { createRoot } = await import("react-dom/client");
const { saveCreationLoginDraft, readCreationLoginDraft, discardCreationLoginDraft } = await import("@/pages/create/creation-login-draft");
const { useUserStore } = await import("@/stores/use-user-store");
const { useModelCatalogStore } = await import("@/stores/use-model-catalog-store");
const { createModelChannel, defaultConfig, normalizeConfigSnapshot, useConfigStore } = await import("@/stores/use-config-store");
const { apiClient } = await import("@/services/api/request");
const { ModelPicker } = await import("@/components/model-picker");
let composer: any;
mock.module("@/pages/create/creation-workspace", () => ({
    CreationComposer: (props: any) => {
        composer = props;
        return (
            <button data-testid="start-creation" onClick={props.onSubmit}>
                开始创作
            </button>
        );
    },
    CreationConversationSidebar: () => null,
    CreationEmptySuggest: () => null,
    CreationFeaturedWorks: () => null,
    CreationMessageView: () => null,
    CreationModeTabs: () => null,
    CreationWorkspaceToolbar: () => null,
    creationAssetCategoryLabels: {},
}));
const { default: CreatePage } = await import("@/pages/create/index");
const { MemoryRouter, Routes, Route, useLocation, useNavigate } = await import("react-router");
function WorkspaceAuthProvider({ children }: any) {
    const navigate = useNavigate();
    return (
        <QueryClientProvider client={queryClient}>
            <ProductionAuthProvider>
                {children}
                <AuthDialog navigate={navigate} />
            </ProductionAuthProvider>
        </QueryClientProvider>
    );
}
const { useCreationPreferencesStore } = await import("@/stores/use-creation-preferences-store");
const host = document.createElement("div");
document.body.append(host);
const root = createRoot(host);
const originalRequest = apiClient.request;
let quotes = 0;
apiClient.request = (async (config: any) => {
    if (config.url === "/public/customer-service") return { data: { code: 0, data: { customerService: { enabled: true, configured: true, tutorialUrl: "https://example.com/tutorial" } }, msg: "ok" } };
    if (config.url === "/auth/settings") return { data: { code: 0, data: { loginMode: "modal" }, msg: "ok" } };
    if (!String(config.url).includes("quote")) return { data: { code: 0, data: { projects: [], total: 0 }, msg: "ok" } };
    quotes++;
    return { data: { code: 0, data: { quote: {} }, msg: "ok" } };
}) as typeof apiClient.request;
afterAll(async () => {
    await act(async () => root.unmount());
    apiClient.request = originalRequest;
    dom.happyDOM.abort();
});

const draft = {
    mode: "image" as const,
    prompt: "雨中的城市",
    model: "system::image-model",
    ratio: "1:1",
    seconds: "6",
    quality: "high",
    imageResolution: "1k",
    imageParameters: "{}",
    videoQuality: "720",
    count: "2",
    textStreaming: true,
    textThinking: false,
    attachments: [],
    references: [],
};
test("login handoff retains prompt, model and parameters and consumes once", async () => {
    await saveCreationLoginDraft(draft);
    const saved = await readCreationLoginDraft();
    expect(saved?.draft).toEqual(draft);
    expect(dom.sessionStorage.length).toBe(1);
    expect(dom.sessionStorage.getItem(dom.sessionStorage.key(0)!)).not.toContain(draft.prompt);
    await discardCreationLoginDraft(saved!.id);
    expect(await readCreationLoginDraft()).toBeNull();
    expect(records.size).toBe(0);
});
test("handoff is tab scoped and expired drafts are removed", async () => {
    await saveCreationLoginDraft(draft);
    const saved = (await readCreationLoginDraft())!;
    const key = dom.sessionStorage.key(0)!;
    dom.sessionStorage.removeItem(key);
    expect(await readCreationLoginDraft()).toBeNull();
    dom.sessionStorage.setItem(key, saved.id);
    records.get(saved.id).expiresAt = Date.now() - 1;
    expect(await readCreationLoginDraft()).toBeNull();
    expect(records.size).toBe(0);
});
test("saving a replacement clears the old draft and write failure preserves it", async () => {
    await saveCreationLoginDraft(draft);
    const prior = (await readCreationLoginDraft())!;
    failWrite = true;
    await expect(saveCreationLoginDraft({ ...draft, prompt: "第二份" })).rejects.toThrow("storage unavailable");
    failWrite = false;
    expect((await readCreationLoginDraft())?.id).toBe(prior.id);
    await saveCreationLoginDraft({ ...draft, prompt: "第二份" });
    expect(records.has(prior.id)).toBe(false);
    const saved = (await readCreationLoginDraft())!;
    expect(saved.draft.prompt).toBe("第二份");
    await discardCreationLoginDraft(saved.id);
});
test("guests see system image models without authenticated quote requests", async () => {
    const channel = createModelChannel({
        id: "system",
        name: "系统模型",
        scope: "system",
        models: ["image-model"],
        imageModels: ["image-model"],
        modelCosts: [{ model: "image-model", displayName: "图片测试模型", capability: "image", pricePolicy: "channel", billingMode: "fixed_request", unitPriceMicrocredits: 1000000 }],
    });
    const config = normalizeConfigSnapshot({ config: { ...defaultConfig, channels: [channel], imageModel: "system::image-model", imageModels: undefined } }).config;
    let selected = "";
    await act(async () => {
        useUserStore.setState({ user: null, features: { ...useUserStore.getState().features, creditsEnabled: true } });
        useModelCatalogStore.setState({ status: "ready" });
        root.render(
            <ModelPicker
                config={config}
                value={config.imageModel}
                capability="image"
                onChange={(value) => {
                    selected = value;
                }}
            />,
        );
    });
    expect(host.textContent).toContain("图片测试模型");
    const group = host.querySelector<HTMLButtonElement>(".canvas-model-picker-brand")!;
    await act(async () => group.click());
    expect(selected).toBe("system::image-model");
    expect(host.textContent).not.toContain("当前没有可用模型");
    expect(quotes).toBe(0);
});

test("sidebar leaves public navigation available and resumes a protected destination only after login", async () => {
    let expanded = 0;
    await act(async () => {
        useUserStore.setState({ hydrated: true, user: null });
        root.render(
            <antd.App>
                <MemoryRouter key="sidebar" initialEntries={["/"]}>
                    <WorkspaceAuthProvider>
                        <Location />
                        <WorkspaceSidebarNav
                            collapsed={false}
                            onNavigate={() => {}}
                            onOpenSearch={() => {}}
                            onExpand={() => {}}
                            onCollapse={() => {
                                expanded++;
                            }}
                        />
                    </WorkspaceAuthProvider>
                </MemoryRouter>
            </antd.App>,
        );
    });
    await act(async () => {
        host.querySelector<HTMLButtonElement>('[aria-label="收起侧栏"]')!.click();
    });
    expect(expanded).toBe(1);
    expect(host.querySelector('[role="dialog"]')).toBeNull();
    await act(async () => {
        host.querySelector<HTMLAnchorElement>('[data-nav-id="home"]')!.click();
    });
    expect(host.querySelector("output")!.textContent).toBe("/?home=1");
    expect(host.querySelector('[role="dialog"]')).toBeNull();
    await act(async () => {
        host.querySelector<HTMLAnchorElement>('[data-nav-id="assets"]')!.click();
    });
    expect(host.querySelector("output")!.textContent).toBe("/?home=1");
    expect(host.querySelectorAll('[role="dialog"]').length).toBe(1);
    await act(async () => {
        [...host.querySelectorAll<HTMLElement>('[role="tab"]')].find((tab) => tab.textContent === "注册")!.click();
    });
    expect(host.textContent).toContain("返回登录表单");
    expect(host.querySelector("output")!.textContent).toBe("/?home=1");
    await act(async () => {
        host.querySelector<HTMLButtonElement>('[data-testid="return-login"]')!.click();
    });
    await act(async () => {
        host.querySelector<HTMLAnchorElement>('[data-testid="recover"]')!.click();
    });
    expect(host.textContent).toContain("找回密码");
    expect(host.querySelector("output")!.textContent).toBe("/?home=1");
    await act(async () => {
        host.querySelector<HTMLButtonElement>('[data-testid="return-login"]')!.click();
    });
    await act(async () => {
        host.querySelector<HTMLButtonElement>('[data-testid="close-auth"]')!.click();
    });
    expect(host.querySelector("output")!.textContent).toBe("/?home=1");
    await act(async () => {
        host.querySelector<HTMLAnchorElement>('[data-nav-id="assets"]')!.click();
        host.querySelector<HTMLAnchorElement>('[data-nav-id="assets"]')!.click();
    });
    expect(host.querySelectorAll('[role="dialog"]').length).toBe(1);
    await act(async () => {
        host.querySelector<HTMLButtonElement>('[data-testid="confirm-login"]')!.click();
    });
    expect(host.querySelector("output")!.textContent).toBe("/assets");
    expect(host.querySelector('[role="dialog"]')).toBeNull();
});
test("empty model picker distinguishes loading, failure and empty directory", async () => {
    const config = normalizeConfigSnapshot({ config: { ...defaultConfig, channels: [] } }).config;
    for (const [status, label] of [
        ["loading", "正在加载模型"],
        ["error", "模型加载失败"],
        ["ready", "当前没有可用模型"],
    ] as const) {
        await act(async () => {
            useModelCatalogStore.setState({ status });
            root.render(<ModelPicker config={config} capability="image" onChange={() => {}} />);
        });
        expect(host.textContent).toContain(label);
        if (status === "error") expect(host.textContent).toContain("重新加载");
    }
});

function Login({ onAuthenticated, onViewChange }: any) {
    return (
        <>
            <button
                data-testid="confirm-login"
                onClick={() => {
                    useUserStore.setState({ hydrated: true, user: { id: "logged-in-user", username: "creator", role: "user", status: "active" } as any });
                    onAuthenticated();
                }}
            >
                确认登录
            </button>
            <button role="tab" onClick={() => onViewChange("register")}>
                注册
            </button>
            <a data-testid="recover" onClick={() => onViewChange("recovery")}>
                忘记密码
            </a>
        </>
    );
}

function AuthSubView({ onViewChange }: any) {
    return (
        <button data-testid="return-login" onClick={() => onViewChange("login")}>
            返回登录表单
        </button>
    );
}

function Location() {
    const location = useLocation();
    return <output>{location.pathname + location.search}</output>;
}

test("actual creation page opens a modal in place and restores the guest draft without generating", async () => {
    const channel = createModelChannel({ id: "system", scope: "system", models: ["image-model"], modelCosts: [{ model: "image-model", capability: "image", billingMode: "fixed_request", unitPriceMicrocredits: 1000000 }] });
    await act(async () => {
        useConfigStore.getState().replaceConfig(normalizeConfigSnapshot({ config: { ...defaultConfig, channels: [channel], imageModel: "system::image-model" } }).config);
        useUserStore.setState({ hydrated: true, user: null });
        useModelCatalogStore.setState({ status: "ready" });
        useCreationPreferencesStore.setState({ hydrated: true, preferences: {} });
        root.render(
            <antd.App>
                <MemoryRouter initialEntries={["/create?source=home"]}>
                    <WorkspaceAuthProvider>
                        <Location />
                        <Routes>
                            <Route path="/create" element={<CreatePage />} />
                        </Routes>
                    </WorkspaceAuthProvider>
                </MemoryRouter>
            </antd.App>,
        );
    });
    expect(host.querySelector('[role="dialog"]')).toBeNull();
    await act(async () => {
        composer.onModeChange("image");
        composer.setPrompt("登录前的提示词");
    });
    await act(async () => {
        composer.setRatio("1:1");
    });
    expect(composer.model).toBe("system::image-model");
    failWrite = true;
    await act(async () => {
        host.querySelector<HTMLButtonElement>('[data-testid="start-creation"]')!.click();
    });
    expect(host.querySelector('[role="dialog"]')).toBeNull();
    expect(composer.prompt).toBe("登录前的提示词");
    expect(document.body.textContent).toContain("storage unavailable");
    failWrite = false;
    await act(async () => {
        host.querySelector<HTMLButtonElement>('[data-testid="start-creation"]')!.click();
    });
    expect(host.textContent).toContain("确认登录");
    expect(host.querySelector("output")!.textContent).toBe("/create?source=home");
    expect((await readCreationLoginDraft())?.draft.prompt).toBe("登录前的提示词");
    await act(async () => {
        host.querySelector<HTMLButtonElement>('[data-testid="close-auth"]')!.click();
    });
    expect(composer.prompt).toBe("登录前的提示词");
    expect(host.querySelector('[role="dialog"]')).toBeNull();
    await act(async () => {
        host.querySelector<HTMLButtonElement>('[data-testid="start-creation"]')!.click();
    });
    await act(async () => {
        host.querySelector<HTMLButtonElement>('[data-testid="confirm-login"]')!.click();
    });
    expect(host.querySelector('[role="dialog"]')).toBeNull();
    expect(composer.prompt).toBe("登录前的提示词");
    expect(composer.mode).toBe("image");
    expect(composer.model).toBe("system::image-model");
    expect(composer.ratio).toBe("1:1");
    expect(await readCreationLoginDraft()).toBeNull();
    expect(quotes).toBe(0);
});
