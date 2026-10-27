import { afterAll, expect, mock, test } from "bun:test";
import { Window } from "happy-dom";
import type { AiConfig } from "@/stores/use-config-store";

const dom = new Window({ url: "http://localhost/canvas/fixture" });
for (const [key, value] of Object.entries({ window: dom, document: dom.document, navigator: dom.navigator, HTMLElement: dom.HTMLElement, Element: dom.Element, Node: dom.Node, Event: dom.Event, IS_REACT_ACT_ENVIRONMENT: true }))
    Object.defineProperty(globalThis, key, { configurable: true, writable: true, value });
const React = await import("react");
const { act } = React;
const { createRoot } = await import("react-dom/client");
mock.module("antd", () => ({
    Button: ({ children, onClick, disabled }: any) => (
        <button onClick={onClick} disabled={disabled}>
            {children}
        </button>
    ),
    Checkbox: ({ children, checked, onChange }: any) => (
        <label>
            <input type="checkbox" checked={checked} onChange={onChange} />
            {children}
        </label>
    ),
    Input: { TextArea: ({ rows, value, onChange, ...props }: any) => <textarea rows={rows} value={value} onChange={onChange} {...props} /> },
    Tag: ({ children }: any) => <span>{children}</span>,
}));
mock.module("@/components/ui/product/app-modal", () => ({ AppModal: ({ children, open }: any) => (open ? <section role="dialog">{children}</section> : null) }));
mock.module("@/components/model-picker", () => ({
    ModelPicker: ({ value, capability, onChange }: any) => (
        <button data-model={capability} onClick={() => onChange(`${capability}-changed`)}>
            {value}
        </button>
    ),
}));
mock.module("@/stores/use-config-store", () => ({ selectableModelsByCapability: (_config: unknown, capability: string) => (capability === "text" ? ["planner"] : ["image"]) }));
mock.module("@/lib/model-selection", () => ({ defaultImageParamsForModel: () => ({ size: "1024x1024", quality: "auto" }) }));
mock.module("@/lib/canvas/canvas-image-layer-plan", () => ({ imageLayerOutputSize: () => ({ size: "1024x1024" }), imageLayerPlannerError: (_config: unknown, model: string) => (model ? "" : "select planner") }));
mock.module("@/lib/canvas/canvas-image-layers", () => ({ supportsLayerDecomposition: (_config: unknown, model: string) => Boolean(model), supportsExperimentalLayerExtraction: () => false }));
const { CanvasNodeLayerDecompositionDialog } = await import("@/components/canvas/canvas-node-layer-decomposition-dialog");
const host = document.createElement("div");
document.body.append(host);
const root = createRoot(host);
afterAll(async () => {
    await act(async () => root.unmount());
    dom.happyDOM.abort();
});
const config = { model: "image", imageModel: "image", textModel: "planner" } as AiConfig;
const submissions: Array<{ plannerModel: string; generationConfig?: Partial<AiConfig> }> = [];
function confirmButton() {
    return [...host.querySelectorAll("button")].find((button) => button.textContent?.includes("确认模型并开始拆分"))!;
}

test("opening the actual dialog performs no generation; confirmation submits the chosen models", async () => {
    await act(async () => root.render(<CanvasNodeLayerDecompositionDialog dataUrl="data:image/png;base64,AA==" open config={config} sourceSize={{ width: 1024, height: 1024 }} onClose={() => {}} onConfirm={(payload) => submissions.push(payload)} />));
    expect(host.textContent).toContain("两段分别计费");
    expect(host.textContent).toContain("一次或多次拆图任务");
    expect(host.textContent).toContain("planner");
    expect(confirmButton().disabled).toBe(true);
    await act(async () => confirmButton().click());
    expect(submissions).toHaveLength(0);
    await act(async () => host.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click());
    expect(confirmButton().disabled).toBe(false);
    await act(async () => confirmButton().click());
    expect(submissions).toHaveLength(1);
    expect(submissions[0].plannerModel).toBe("planner");
    expect(submissions[0].generationConfig?.imageModel).toBe("image");
});

test("changing either model invalidates the previous acknowledgement", async () => {
    for (const capability of ["text", "image"]) {
        await act(async () => host.querySelector<HTMLButtonElement>(`[data-model="${capability}"]`)!.click());
        expect(host.querySelector<HTMLInputElement>('input[type="checkbox"]')!.checked).toBe(false);
        expect(confirmButton().disabled).toBe(true);
        await act(async () => host.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click());
        expect(confirmButton().disabled).toBe(false);
    }
    expect(submissions).toHaveLength(1);
});
