import { afterAll, expect, mock, test } from "bun:test";
import { Window } from "happy-dom";
import type { GenerationTask } from "@/services/api/task-center";
import type { WorkbenchRetryIntent } from "@/services/api/workbench";
import { createWorkbenchSubmissionJournal, type PendingWorkbenchSubmission } from "@/services/workbench-submission-journal";

const dom = new Window({ url: "http://localhost/" });
for (const [key, value] of Object.entries({
    window: dom,
    document: dom.document,
    navigator: dom.navigator,
    HTMLElement: dom.HTMLElement,
    Element: dom.Element,
    Node: dom.Node,
    Event: dom.Event,
    CustomEvent: dom.CustomEvent,
    IS_REACT_ACT_ENVIRONMENT: true,
}))
    Object.defineProperty(globalThis, key, { configurable: true, writable: true, value });
const React = await import("react");
const { act } = React;
const { createRoot } = await import("react-dom/client");
const { create: createStore } = await import("zustand");
const { MemoryRouter } = await import("react-router");
const { useUserStore, defaultFeatureAvailability } = await import("@/stores/use-user-store");
let editor: any;
let channelScope = "system";
let creates = 0;
let reads = 0;
let prepare: (options?: any) => Promise<any> = async () => ({});
let create: () => Promise<GenerationTask>;
let read: () => Promise<GenerationTask>;
let lookup: (key: string) => Promise<GenerationTask>;
let confirmation: any;
let storageFailure = false;
let sequence = 0;
const pendingRecords = new Map<string, PendingWorkbenchSubmission>();
const requestKeys: string[] = [];
const requestActors: string[] = [];
const wireRequests: any[] = [];
let historyTasks: GenerationTask[] = [];
let modelOptions = ["test-model"];
let logicalIdentities: Record<string, string> = {};
let modelPicker: any;
let retryIntentReads = 0;
let retryIntent: (id: string) => Promise<WorkbenchRetryIntent> = async () => {
    throw new Error("unexpected retry intent");
};
let resourcePreview: (id?: string) => Promise<string | undefined> = async () => "https://media.example/owned-image";
const journal = createWorkbenchSubmissionJournal(
    {
        async mutate(scope, kind, update) {
            if (storageFailure) throw new Error("request storage unavailable");
            const id = `${scope}:${kind}`,
                next = update(pendingRecords.get(id) || null);
            if (next) pendingRecords.set(id, next);
            else pendingRecords.delete(id);
            return next;
        },
    },
    () => `request-key-${String(++sequence).padStart(16, "0")}`,
);
mock.module("@/services/workbench-submission-journal", () => ({ loadWorkbenchSubmission: journal.load, reserveWorkbenchSubmission: journal.reserve, clearWorkbenchSubmission: journal.clear }));
const notices: string[] = [];
const task: GenerationTask = { id: "accepted-task", type: "canvas_image", status: "queued", attempts: 0, prompt: "original", createdAt: "2026-10-26T00:00:00Z", updatedAt: "2026-10-26T00:00:00Z" };
const app = {
    message: { success: (text: string) => notices.push(text), warning: (text: string) => notices.push(text), error: (text: string) => notices.push(text) },
    modal: {
        confirm(options: any) {
            confirmation = options;
        },
    },
};
const quote = { billingEnabled: true, amountMicrocredits: 10000, billingMode: "fixed_request", priceTierVersion: 1, priceVersion: 1, quantity: 1 };
function withReceipt(task: GenerationTask, key: string): GenerationTask {
    return { ...task, workbenchSubmission: { requestKey: key, replayed: false, quote } };
}
const Box = ({ children }: any) => <div>{children}</div>;
mock.module("antd", () => ({
    App: { useApp: () => app },
    Button: ({ children, onClick, disabled, className, "aria-label": label }: any) => (
        <button aria-label={label} className={className} disabled={disabled} onClick={onClick}>
            {children}
        </button>
    ),
    Input: () => null,
    InputNumber: () => null,
    Modal: ({ open, children }: any) => (open ? <div>{children}</div> : null),
    Progress: () => null,
    Segmented: () => null,
    Select: () => null,
    Spin: () => null,
    Switch: () => null,
    Tooltip: Box,
}));
mock.module("@/components/image-size-picker", () => ({ ImageSizePicker: () => null }));
mock.module("@/components/media-preview", () => ({ MediaPreview: () => null }));
mock.module("@/components/model-picker", () => ({
    ModelPicker: (props: any) => {
        modelPicker = props;
        return (
            <select aria-label="fixture model choice" value={props.value} onChange={(event) => props.onChange(event.target.value)}>
                <option value="">选择模型</option>
                {modelOptions.map((value) => (
                    <option key={value} value={value}>
                        {value}
                    </option>
                ))}
            </select>
        );
    },
}));
mock.module("@/components/ui/aceternity/spotlight-surface", () => ({ SpotlightSurface: Box }));
mock.module("@/components/layout/workspace-page", () => ({ WorkspacePage: Box }));
mock.module("@/components/canvas/canvas-resource-mention-textarea", () => ({
    CanvasResourceMentionTextarea: (props: any) => {
        editor = props;
        return <textarea value={props.value} onChange={(event) => props.onChange(event.target.value)} />;
    },
}));
mock.module("@/lib/canvas/canvas-resource-references", () => ({ normalizeCanvasNodeMentionTokens: (value: string) => value }));
const image = { size: { default: "1:1" }, quality: { default: "auto", supported: false }, resolution: { default: "auto", supported: false }, references: { maxImages: 2, promptMaxChars: 32000 }, maxOutputs: 1 };
const video = {
    duration: { default: 6, selection: "enum", values: [6, 8], min: 1, max: 8, step: 1 },
    defaultRatio: "16:9",
    defaultResolution: "720p",
    ratios: ["16:9", "9:16"],
    resolutions: ["720p", "1080p"],
    references: { maxImages: 2, maxVideos: 1, maxAudios: 1, promptMaxChars: 8000 },
    generateAudio: { supported: true },
    watermark: { supported: true },
};
let normalizeRestoredImage = (input: any) => input;
mock.module("@/lib/model-capabilities", () => ({
    modelCapabilityConfigFor: () => ({ image, video }),
    normalizeImageValue: (_: any, input: any) => normalizeRestoredImage(input),
    normalizeVideoValue: (_: any, input: any) => ({ ...input, resolution: String(input.resolution || "720").replace(/p$/i, "") + "p" }),
}));
mock.module("@/pages/create/creation-generation-config", () => ({
    creationVideoConfig: (current: any, model: string, settings: any) => ({ ...current, model, videoModel: model, size: settings.ratio, videoSeconds: settings.seconds, vquality: settings.videoQuality }),
}));
mock.module("@/lib/model-pricing", () => ({ modelQuoteRequest: () => null }));
mock.module("@/services/api/logical-models", () => ({
    quoteModel: async () => {
        throw new Error("unexpected quote");
    },
}));
mock.module("@/constant/credits", () => ({ formatCredits: String, requestCreditCost: () => null }));
mock.module("@/services/api/generation-task", () => ({ prepareBackendGenerationTask: (options: any) => prepare(options) }));
mock.module("@/services/api/task-center", () => ({
    createGenerationTask: async (input: any) => {
        creates++;
        wireRequests.push(input);
        requestKeys.push(input.workbenchRequestKey);
        requestActors.push(input.workbenchExpectedUserId);
        return withReceipt(await create(), input.workbenchRequestKey);
    },
    queryGenerationTask: async () => {
        reads++;
        return read();
    },
    cancelGenerationTask: async () => task,
    retryGenerationTask: async () => {
        throw new Error("unexpected retry");
    },
}));
mock.module("@/services/api/workbench", () => ({
    capabilitySupportedModes: () => [],
    createWorkbenchTask: async (input: any) => {
        creates++;
        wireRequests.push(input);
        requestKeys.push(input.requestKey);
        requestActors.push(input.expectedUserId);
        return withReceipt(await create(), input.requestKey);
    },
    getWorkbenchSubmission: (key: string) => lookup(key),
    getWorkbenchRetryIntent: async (id: string) => {
        retryIntentReads++;
        return retryIntent(id);
    },
    listWorkbenchTasks: async () => [...historyTasks],
    listWorkbenchTools: async () => ({ tools: [] }),
    normalizeWorkbenchGuide: () => null,
    resolveWorkbenchResourceURL: (id?: string) => resourcePreview(id),
}));
mock.module("@/services/api/resources", () => ({
    resourceFileUrl: String,
    resourceStorageKey: String,
    resolveResourceUrl: async () => "",
    uploadResourceFile: async () => {
        throw new Error("unexpected upload");
    },
}));
const preferences = { hydrated: false, rememberImageSettings() {}, rememberVideoSettings() {}, preferences: {} };
const preferencesHook = Object.assign((select: any) => select(preferences), { getState: () => preferences });
mock.module("@/stores/use-creation-preferences-store", () => ({ useCreationPreferencesStore: preferencesHook }));
const config = { model: "test-model", imageModel: "test-model" };
const configState = createStore<{ config: typeof config }>(() => ({ config }));
const updateConfig = (key: string, value: string) => configState.setState((current) => ({ config: { ...current.config, [key]: value } }));
mock.module("@/stores/use-config-store", () => ({
    logicalModelIDForConfig: (value: any) => logicalIdentities[value.model] || "",
    modelDisplayName: () => "测试模型",
    modelOptionName: (value: string) => value.split("::").at(-1)!,
    resolveModelChannel: (_config: any, value: string) => ({ id: value.includes("::") ? value.split("::")[0] : "test-channel", scope: channelScope }),
    selectableModelsByCapability: () => modelOptions,
    useConfigStore: (select: any) => select({ updateConfig, isAiConfigReady: () => true }),
    useEffectiveConfig: () => configState((state) => state.config),
}));
const { WorkbenchPage } = await import("@/pages/workbench/image-workbench-page");
const host = document.createElement("div");
document.body.append(host);
const root = createRoot(host);
async function mount(id: string, kind: "image" | "video" = "image") {
    await act(async () => {
        useUserStore.setState({ user: { id, role: "user" } as any, features: { ...defaultFeatureAvailability, creditsEnabled: false } });
        root.render(
            <MemoryRouter>
                <WorkbenchPage kind={kind} />
            </MemoryRouter>,
        );
    });
}
async function input(text: string) {
    await act(async () => editor.onChange(text));
}
function deferred<T>() {
    let resolve!: (value: T) => void;
    const promise = new Promise<T>((done) => {
        resolve = done;
    });
    return { promise, resolve };
}
function submit() {
    host.querySelector<HTMLButtonElement>(".workbench-submit")!.click();
}

test("accepted task remains visible after detail failure and rapid submit only creates once", async () => {
    channelScope = "system";
    creates = reads = 0;
    notices.length = 0;
    const pending = deferred<GenerationTask>();
    create = () => pending.promise;
    read = async () => {
        throw new Error("detail offline");
    };
    await mount("account-a");
    await input("draft-a");
    await act(async () => {
        submit();
        submit();
    });
    expect(creates).toBe(1);
    await act(async () => pending.resolve(task));
    expect(reads).toBe(1);
    expect(host.textContent).toContain("排队中");
    expect(notices.some((message) => message.includes("任务已提交，详情暂时不可用"))).toBe(true);
    expect(notices.some((message) => message.includes("任务提交失败"))).toBe(false);
});

test("account switch clears drafts and stops old-account detail reads and notices", async () => {
    creates = reads = 0;
    notices.length = 0;
    const pending = deferred<GenerationTask>();
    create = () => pending.promise;
    read = async () => task;
    await mount("account-b");
    await input("private-draft-b");
    await act(async () => submit());
    await mount("account-c");
    expect(editor.value).toBe("");
    await act(async () => pending.resolve(task));
    expect(creates).toBe(1);
    expect(reads).toBe(0);
    expect(notices).toEqual([]);
    expect(host.textContent).not.toContain("original");
});

test("custom-channel preparation finishing after an account switch cannot create a task", async () => {
    channelScope = "custom";
    creates = reads = 0;
    notices.length = 0;
    const pending = deferred<any>();
    prepare = () => pending.promise;
    await mount("account-d");
    await input("private-draft-d");
    await act(async () => submit());
    await mount("account-e");
    await act(async () => pending.resolve({ type: "canvas_image", input: {} }));
    expect(creates).toBe(0);
    expect(reads).toBe(0);
    expect(editor.value).toBe("");
    expect(notices).toEqual([]);
});

test("unknown creation response keeps the key and manual retry reuses it", async () => {
    channelScope = "system";
    creates = reads = 0;
    requestKeys.length = requestActors.length = 0;
    create = async () => {
        if (creates === 1) throw new Error("creation response lost");
        return task;
    };
    read = async () => task;
    await mount("retry-account");
    await input("same draft");
    await act(async () => submit());
    expect(host.textContent).toContain("有一笔提交待确认");
    expect(pendingRecords.has("retry-account:image")).toBe(true);
    await act(async () => submit());
    expect(creates).toBe(2);
    expect(requestKeys[0]).toBe(requestKeys[1]);
    expect(requestActors).toEqual(["retry-account", "retry-account"]);
    expect(pendingRecords.has("retry-account:image")).toBe(false);
});

test("changed pending request requires explicit new-task confirmation", async () => {
    creates = 0;
    requestKeys.length = 0;
    confirmation = null;
    create = async () => {
        if (creates === 1) throw new Error("creation response lost");
        return task;
    };
    await mount("changed-account");
    await input("original draft");
    await act(async () => submit());
    await input("changed draft");
    await act(async () => submit());
    expect(creates).toBe(1);
    const button = [...host.querySelectorAll<HTMLButtonElement>("button")].find((item) => item.textContent === "明确创建新任务")!;
    await act(async () => button.click());
    expect(confirmation).not.toBeNull();
    expect(pendingRecords.has("changed-account:image")).toBe(true);
    await act(async () => confirmation.onOk());
    await act(async () => submit());
    expect(creates).toBe(2);
    expect(requestKeys[0]).not.toBe(requestKeys[1]);
});

test("remount restores only pending identity and lookup recovers the original task without POST", async () => {
    creates = 0;
    requestKeys.length = 0;
    create = async () => {
        throw new Error("creation response lost");
    };
    await mount("reload-account");
    await input("draft before reload");
    await act(async () => submit());
    const key = requestKeys[0]!;
    lookup = async (requested) => {
        expect(requested).toBe(key);
        return withReceipt(task, key);
    };
    await act(async () => root.render(null));
    await mount("reload-account");
    expect(creates).toBe(1);
    expect(host.textContent).toContain(key);
    const button = [...host.querySelectorAll<HTMLButtonElement>("button")].find((item) => item.textContent === "核对原任务")!;
    await act(async () => button.click());
    expect(creates).toBe(1);
    expect(pendingRecords.has("reload-account:image")).toBe(false);
    expect(host.textContent).toContain("原提交报价");
});

test("local request storage failure prevents generation", async () => {
    storageFailure = true;
    creates = 0;
    await mount("storage-error-account");
    await input("draft");
    expect(host.textContent).toContain("request storage unavailable");
    expect(host.querySelector<HTMLButtonElement>(".workbench-submit")!.disabled).toBe(true);
    await act(async () => editor.onSubmit());
    expect(creates).toBe(0);
    storageFailure = false;
});

test("custom-channel retries bind the original key and expected account", async () => {
    channelScope = "custom";
    creates = 0;
    requestKeys.length = requestActors.length = notices.length = 0;
    prepare = async () => ({ type: "canvas_image", prompt: "custom draft", input: { mode: "image", metadata: { source: "workbench", workbenchKind: "image" } } });
    create = async () => {
        if (creates === 1) throw new Error("custom response lost");
        return task;
    };
    read = async () => task;
    await mount("custom-retry-account");
    await input("custom draft");
    await act(async () => submit());
    for (let i = 0; i < 50 && !notices.includes("custom response lost"); i++)
        await act(async () => {
            await new Promise((done) => setTimeout(done, 1));
        });
    expect(notices).toContain("custom response lost");
    await act(async () => submit());
    for (let i = 0; i < 50 && creates < 2; i++)
        await act(async () => {
            await new Promise((done) => setTimeout(done, 1));
        });
    expect(creates, notices.join(" | ")).toBe(2);
    expect(requestKeys[0]).toBe(requestKeys[1]);
    expect(requestActors).toEqual(["custom-retry-account", "custom-retry-account"]);
});
const sourceVersion = "a".repeat(64);
function sourceTask(overrides: Partial<GenerationTask> = {}): GenerationTask {
    return {
        ...task,
        id: "history-source",
        status: "failed",
        prompt: "original-history-prompt",
        model: "public-model",
        logicalModelId: "logical-source",
        inputJson: JSON.stringify({ metadata: { source: "workbench", workbenchKind: "image", workbench: { kind: "image", parameters: { size: "16:9", quality: "high", imageResolution: "2k", count: 2 }, references: [] } } }),
        ...overrides,
    };
}
async function historyFixture(account: string, source = sourceTask(), options = ["test-model"], identities: Record<string, string> = { "test-model": "logical-source" }, kind: "image" | "video" = "image") {
    channelScope = "system";
    creates = reads = retryIntentReads = 0;
    notices.length = wireRequests.length = 0;
    historyTasks = [source];
    modelOptions = options;
    logicalIdentities = identities;
    normalizeRestoredImage = (input: any) => input;
    await act(async () => configState.setState({ config: { model: options[0]!, imageModel: options[0]! } }));
    resourcePreview = async () => "https://media.example/owned-image";
    retryIntent = async () => ({ task: source, retryOf: source.id, retryVersion: sourceVersion, modelSelector: !source.logicalModelId && source.model?.includes("::") ? source.model : undefined });
    create = async () => task;
    read = async () => task;
    await mount(account, kind);
    return source;
}
async function loadHistory() {
    const button = host.querySelector<HTMLButtonElement>('button[aria-label="载入原参数"]');
    expect(button).not.toBeNull();
    await act(async () => button!.click());
}
async function waitForUI(check: () => boolean) {
    for (let i = 0; i < 100 && !check(); i++)
        await act(async () => {
            await new Promise((done) => setTimeout(done, 1));
        });
    expect(check(), notices.join(" | ")).toBe(true);
}

test("history load is read-only and manual generation carries the original retry reference", async () => {
    const source = await historyFixture("history-known");
    await loadHistory();
    expect(retryIntentReads).toBe(1);
    expect(creates).toBe(0);
    expect(editor.value).toBe(source.prompt);
    expect(modelPicker.value).toBe("test-model");
    expect(host.textContent).toContain("核对模型和当前报价后手动生成");
    await act(async () => submit());
    await waitForUI(() => creates === 1);
    expect(wireRequests[0]).toMatchObject({ retryOf: source.id, retryVersion: sourceVersion, parameters: { size: "16:9", quality: "high", imageResolution: "2k", count: 2 } });
    expect(wireRequests[0].requestKey).toBeTruthy();
    expect(source.status).toBe("failed");
    expect(task.id).not.toBe(source.id);
});
test("ambiguous history model requires a visible explicit channel choice before generation", async () => {
    await historyFixture("history-ambiguous", sourceTask({ model: "same", logicalModelId: undefined }), ["a::same", "b::same"], {});
    await loadHistory();
    expect(modelPicker.value).toBe("");
    expect(creates).toBe(0);
    await act(async () => editor.onSubmit());
    expect(creates).toBe(0);
    await act(async () => modelPicker.onChange("b::same"));
    await act(async () => submit());
    await waitForUI(() => creates === 1);
    expect(wireRequests[0]).toMatchObject({ channelId: "b", model: "same", retryOf: "history-source", retryVersion: sourceVersion });
});
test("fee and media recovery denials preserve the current draft and never submit", async () => {
    for (const reason of ["费用核对中", "请使用重试保存"]) {
        await historyFixture(`history-denied-${reason}`);
        await input("my-current-draft");
        retryIntent = async () => {
            throw new Error(reason);
        };
        await loadHistory();
        expect(creates).toBe(0);
        expect(editor.value).toBe("my-current-draft");
        expect(notices).toContain(reason);
    }
});
test("unavailable reference media cannot partially overwrite the composer", async () => {
    const source = sourceTask();
    const data = JSON.parse(source.inputJson!);
    data.metadata.workbench.references = [{ resourceId: "unavailable-resource", kind: "image", role: "reference" }];
    source.inputJson = JSON.stringify(data);
    await historyFixture("history-missing-reference", source);
    await input("keep-current-draft");
    resourcePreview = async () => undefined;
    await loadHistory();
    expect(creates).toBe(0);
    expect(editor.value).toBe("keep-current-draft");
    expect(notices.some((item) => item.includes("素材暂不可用"))).toBe(true);
});
test("an edited composer ignores a late retry-intent response", async () => {
    const source = await historyFixture("history-late-edit");
    const pending = deferred<WorkbenchRetryIntent>();
    retryIntent = () => pending.promise;
    await loadHistory();
    await input("later-user-edit");
    await act(async () => pending.resolve({ task: source, retryOf: source.id, retryVersion: sourceVersion }));
    expect(editor.value).toBe("later-user-edit");
    expect(creates).toBe(0);
});
test("account switching discards a late history load", async () => {
    const source = await historyFixture("history-old-account");
    const pending = deferred<WorkbenchRetryIntent>();
    retryIntent = () => pending.promise;
    await loadHistory();
    historyTasks = [];
    await mount("history-new-account");
    await act(async () => pending.resolve({ task: source, retryOf: source.id, retryVersion: sourceVersion }));
    expect(editor.value).toBe("");
    expect(creates).toBe(0);
    expect(notices).toEqual([]);
});
test("completed task detail failure stops parameter loading without a retry fallback", async () => {
    await historyFixture("history-detail-error", sourceTask({ status: "succeeded" }));
    await input("existing-draft");
    read = async () => {
        throw new Error("detail unavailable");
    };
    await loadHistory();
    expect(creates).toBe(0);
    expect(retryIntentReads).toBe(0);
    expect(editor.value).toBe("existing-draft");
    expect(notices).toContain("detail unavailable");
});
test("custom-channel history uses the manual idempotent create path", async () => {
    await historyFixture("history-custom", sourceTask({ model: "custom::same", logicalModelId: undefined }), ["custom::same"], {});
    channelScope = "custom";
    prepare = async (options) => ({ type: "canvas_image", prompt: options.prompt, input: { mode: "image", metadata: options.metadata } });
    await loadHistory();
    expect(creates).toBe(0);
    await act(async () => submit());
    await waitForUI(() => creates === 1);
    expect(wireRequests[0]).toMatchObject({ workbenchRetryOf: "history-source", workbenchRetryVersion: sourceVersion, workbenchExpectedUserId: "history-custom" });
    expect(wireRequests[0].workbenchRequestKey).toBeTruthy();
});
test("video history restores duration, framing and explicit false flags without sending", async () => {
    const source = sourceTask({
        type: "canvas_video",
        inputJson: JSON.stringify({ metadata: { source: "workbench", workbenchKind: "video", workbench: { kind: "video", parameters: { seconds: 8, ratio: "9:16", resolution: "1080p", generateAudio: false, watermark: false }, references: [] } } }),
    });
    await historyFixture("history-video", source, ["test-model"], { "test-model": "logical-source" }, "video");
    await loadHistory();
    expect(creates).toBe(0);
    await act(async () => submit());
    await waitForUI(() => creates === 1);
    expect(wireRequests[0]).toMatchObject({ kind: "video", retryOf: source.id, parameters: { seconds: 8, ratio: "9:16", resolution: "1080p", generateAudio: false, watermark: false } });
});
test("changed model specifications require explicit model confirmation", async () => {
    await historyFixture("history-changed-spec");
    normalizeRestoredImage = (input: any) => ({ ...input, count: "1" });
    await loadHistory();
    expect(modelPicker.value).toBe("");
    expect(creates).toBe(0);
    expect(modelPicker.requirements).toEqual({ capability: "image" });
    expect(host.textContent).toContain("历史模型或规格无法完整复用");
});
test("incomplete history parameters do not borrow unrelated current defaults", async () => {
    const source = sourceTask({ inputJson: JSON.stringify({ workbench: { kind: "image", parameters: {}, references: [] } }) });
    await historyFixture("history-incomplete", source);
    await input("current-draft");
    await loadHistory();
    expect(editor.value).toBe("current-draft");
    expect(creates).toBe(0);
    expect(notices.some((item) => item.includes("规格不完整"))).toBe(true);
});
afterAll(async () => {
    await act(async () => root.unmount());
    dom.close();
});
