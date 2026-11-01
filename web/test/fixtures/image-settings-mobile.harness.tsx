import { afterAll, afterEach, expect, test } from "bun:test";
import { Window } from "happy-dom";
import { multiTierPixelProfile } from "./image-multi-tier-profile";

const dom = new Window({ url: "http://localhost/", width: 390, height: 844 });
Object.defineProperty(globalThis, "requestAnimationFrame", { configurable: true, value: dom.requestAnimationFrame.bind(dom) });
for (const [key, value] of Object.entries({ window: dom, document: dom.document, navigator: dom.navigator, HTMLElement: dom.HTMLElement, Element: dom.Element, Node: dom.Node, SVGElement: dom.SVGElement, ShadowRoot: dom.ShadowRoot, ResizeObserver: dom.ResizeObserver, getComputedStyle: dom.getComputedStyle.bind(dom), IS_REACT_ACT_ENVIRONMENT: true })) {
    Object.defineProperty(globalThis, key, { configurable: true, writable: true, value });
}
const { act, useState } = await import("react");
const { createRoot } = await import("react-dom/client");
const { CreationSettingsOverlay } = await import("../../src/pages/create/creation-settings-overlay");
const { ImageParameterFields } = await import("../../src/components/image-parameter-fields");
const { imageParameterRequest } = await import("../../src/lib/image-parameters");
const { defaultConfig } = await import("../../src/stores/use-config-store");

let writes = 0;
const host = document.createElement("div");
document.body.append(host);
let root = createRoot(host);
afterEach(async () => {
    await act(async () => root.unmount());
    root = createRoot(host);
    writes = 0;
});
afterAll(async () => { await act(async () => root.unmount()); dom.happyDOM.abort(); });

function Harness({ profile = multiTierPixelProfile(), initialSize = "auto" }) {
    const [open, setOpen] = useState(false);
    const [config, setConfig] = useState({ ...defaultConfig, size: initialSize, quality: "high", count: "2" });
    const content = <div className="creation-parameter-menu"><ImageParameterFields profile={profile} config={config} onChange={(key, value) => {
        writes++;
        setConfig((current) => ({ ...current, [key]: value }));
    }} /></div>;
    return <><output>{JSON.stringify(imageParameterRequest(profile, config, true))}</output>
        <CreationSettingsOverlay open={open} onOpenChange={setOpen} content={content} mobileDrawer>
            <button data-testid="settings" onClick={() => setOpen(!open)}>设置</button>
        </CreationSettingsOverlay></>;
}
function button(label: string) {
    const found = [...document.querySelectorAll<HTMLButtonElement>("button")].find((element) => element.textContent?.trim() === label);
    if (!found) throw new Error(`Missing button: ${label}`);
    return found;
}
async function open() { await act(async () => button("设置").click()); }

test("mobile drawer groups sizes, submits only size and quality, and retains selection after closing", async () => {
    await act(async () => root.render(<Harness />));
    expect(document.querySelector(".ant-drawer")).toBeNull();
    await open();
    expect(document.querySelector(".ant-drawer-bottom")).not.toBeNull();
    expect(document.querySelector(".ant-popover")).toBeNull();
    expect(writes).toBe(0);
    await act(async () => button("4K").click());
    await act(async () => button("16:9").click());
    expect(JSON.parse(host.querySelector("output")!.textContent!)).toEqual({ size: "3840x2160", quality: "high", count: 2 });
    const before = writes;
    await act(async () => document.querySelector<HTMLButtonElement>(".ant-drawer-close")!.click());
    await act(async () => new Promise<void>((resolve) => dom.requestAnimationFrame(() => resolve())));
    expect(document.activeElement).toBe(host.querySelector('[data-testid="settings"]'));
    await open();
    expect(writes).toBe(before);
    expect(document.querySelector(".image-size-output")!.textContent).toContain("3840 × 2160");
});

test("tier change preserves the ratio, or uses a declared fallback even when custom size is allowed", async () => {
    const profile = multiTierPixelProfile();
    profile.size.presets = profile.size.presets!.filter((preset) => preset.tier !== "4k" || preset.ratio === "1:1");
    profile.size.values = ["auto", ...profile.size.presets.map((preset) => preset.size)];
    profile.parameters![0].values = profile.size.values;
    await act(async () => root.render(<Harness profile={profile} initialSize="1536x1024" />));
    await open();
    await act(async () => button("2K").click());
    expect(JSON.parse(host.querySelector("output")!.textContent!).size).toBe("2496x1664");
    await act(async () => button("4K").click());
    expect(JSON.parse(host.querySelector("output")!.textContent!).size).toBe("2880x2880");
});

test("changing model retains only its supported tier", async () => {
    await act(async () => root.render(<Harness initialSize="3840x2160" />));
    await open();
    const profile = multiTierPixelProfile();
    profile.size.defaultTier = "2k";
    await act(async () => root.render(<Harness profile={profile} initialSize="3840x2160" />));
    const labels = [...document.querySelectorAll(".image-size-tiers button")].map((element) => element.textContent);
    expect(labels).not.toContain("4K");
    expect(document.querySelector('[title="16:9 · 3840 × 2160"]')).toBeNull();
});

test("desktop uses the existing popover, and resize switches to the drawer without changing values", async () => {
    await act(async () => dom.happyDOM.setWindowSize({ width: 1200, height: 900 }));
    await act(async () => root.render(<Harness />));
    await open();
    expect(document.querySelector(".ant-popover")).not.toBeNull();
    expect(document.querySelector(".ant-drawer")).toBeNull();
    await act(async () => dom.happyDOM.setWindowSize({ width: 390, height: 844 }));
    expect(document.querySelector(".ant-drawer-bottom")).not.toBeNull();
    expect(writes).toBe(0);
});
