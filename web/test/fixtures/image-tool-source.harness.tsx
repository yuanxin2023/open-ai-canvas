import { afterAll, afterEach, beforeEach, expect, test } from "bun:test";
import { Window } from "happy-dom";

const dom = new Window({ url: "https://canvas.example/" });
const decoded: string[] = [];
class TestImage {
    naturalWidth = 1200;
    naturalHeight = 800;
    onload?: () => void;
    onerror?: () => void;
    set src(value: string) {
        if (!value) return;
        decoded.push(value);
        queueMicrotask(() => (value.includes("broken") ? this.onerror?.() : this.onload?.()));
    }
}
class TestFileReader {
    result: string | ArrayBuffer = "";
    onload?: () => void;
    onerror?: () => void;
    readAsArrayBuffer(blob: Blob) {
        void blob.arrayBuffer().then(
            (bytes) => {
                this.result = bytes;
                this.onload?.();
            },
            () => this.onerror?.(),
        );
    }
    readAsDataURL(blob: Blob) {
        void blob.arrayBuffer().then(
            (bytes) => {
                this.result = `data:${blob.type};base64,${Buffer.from(bytes).toString("base64")}`;
                this.onload?.();
            },
            () => this.onerror?.(),
        );
    }
}
const originals = new Map<string, PropertyDescriptor | undefined>();
for (const [key, value] of Object.entries({
    window: dom,
    document: dom.document,
    navigator: dom.navigator,
    location: dom.location,
    HTMLElement: dom.HTMLElement,
    Element: dom.Element,
    Node: dom.Node,
    SVGElement: dom.SVGElement,
    ShadowRoot: dom.ShadowRoot,
    getComputedStyle: dom.getComputedStyle.bind(dom),
    localStorage: dom.localStorage,
    Image: TestImage,
    FileReader: TestFileReader,
    IS_REACT_ACT_ENVIRONMENT: true,
})) {
    originals.set(key, Object.getOwnPropertyDescriptor(globalThis, key));
    Object.defineProperty(globalThis, key, { configurable: true, writable: true, value });
}
const { act } = await import("react");
const { createRoot } = await import("react-dom/client");
const { useImageToolSource } = await import("../../src/hooks/use-image-tool-source");
const { ImageCropDialog } = await import("../../src/components/image-crop-dialog");
const { CanvasNodeAnnotationDialog } = await import("../../src/components/canvas/canvas-node-annotation-dialog");
const { imageToDataUrl } = await import("../../src/services/image-storage");
const { getResourceBlob, clearResourceAccessCache } = await import("../../src/services/api/resources");
const { primeResourceBlobCache, clearResourceBlobCache } = await import("../../src/services/resource-blob-cache");
const { apiClient } = await import("../../src/services/api/request");
const originalFetch = globalThis.fetch;
const originalAdapter = apiClient.defaults.adapter;
let delivery = "origin";
const accessPayloads: Array<{ purpose: string }> = [];
apiClient.defaults.adapter = async (config) => {
    const payload = JSON.parse(String(config.data)) as Array<{ resourceId: string; purpose: string }>;
    accessPayloads.push(...payload);
    return {
        config,
        status: 200,
        statusText: "OK",
        headers: {},
        data: { code: 0, data: { items: payload.map(({ resourceId }) => ({ access: { resourceId, delivery, url: delivery === "origin" ? "https://r2.example/image.png?signature=unchanged" : "/api/resources/test/file" } })) } },
    };
};
let root: ReturnType<typeof createRoot> | undefined;
const settle = async () => {
    for (let i = 0; i < 4; i++) await act(() => new Promise((resolve) => setTimeout(resolve, 15)));
};
const host = dom.document.createElement("div");
dom.document.body.append(host);

beforeEach(() => dom.localStorage.setItem("yingce:active-user-scope", "image-tool-test"));
afterEach(async () => {
    await act(() => root?.unmount());
    root = undefined;
    globalThis.fetch = originalFetch;
    decoded.length = 0;
    accessPayloads.length = 0;
    clearResourceAccessCache();
    clearResourceBlobCache();
    delivery = "origin";
    dom.localStorage.clear();
});
afterAll(() => {
    apiClient.defaults.adapter = originalAdapter;
    dom.happyDOM.abort();
    for (const [key, value] of originals) {
        if (value) Object.defineProperty(globalThis, key, value);
        else Reflect.deleteProperty(globalThis, key);
    }
});

function Probe({ source, storageKey }: { source: string; storageKey?: string }) {
    const result = useImageToolSource(source, storageKey);
    return (
        <div>
            <output>{result.error || (result.size ? `${result.size.width}x${result.size.height}` : "loading")}</output>
            {result.previewUrl ? <img src={result.previewUrl} /> : null}
        </div>
    );
}
async function render(element: React.ReactNode) {
    root ??= createRoot(host);
    await act(() => root!.render(element));
    await settle();
}

test("resource reads bypass opaque display cache without changing signed URLs or leaking cookies", async () => {
    const requests: Array<{ url: string; options?: RequestInit }> = [];
    globalThis.fetch = (async (url, options) => {
        requests.push({ url: String(url), options });
        if (options?.cache !== "no-store") throw new TypeError("cached display response is unreadable");
        return new Response(new Blob(["pixels"], { type: "image/png" }));
    }) as typeof fetch;
    expect((await getResourceBlob("resource:cloud"))?.type).toBe("image/png");
    expect(requests[0].url).toBe("https://r2.example/image.png?signature=unchanged");
    expect(requests[0].options).toMatchObject({ cache: "no-store", mode: "cors", credentials: "omit" });
    expect(accessPayloads[0].purpose).toBe("browser-process");
    delivery = "platform-local";
    await getResourceBlob("resource:local");
    expect(requests[1].options?.credentials).toBe("include");
});

test("crop preview and dimensions use the same prepared image instead of the display URL", async () => {
    globalThis.fetch = (async (_url, options) => {
        expect(options).toMatchObject({ cache: "no-store", mode: "cors" });
        return new Response(new Blob(["pixels"], { type: "image/png" }));
    }) as typeof fetch;
    await render(<ImageCropDialog dataUrl="https://r2.example/display.png" storageKey="resource:crop" open onClose={() => {}} onConfirm={() => {}} />);
    const preview = dom.document.querySelector<HTMLImageElement>('img[alt="待裁剪图片"]');
    expect(preview?.src).toStartWith("data:image/png;");
    expect(decoded).toContain(preview!.src);
    expect(dom.document.body.textContent).toContain("输出 960 × 640 px");
    expect(accessPayloads[0].purpose).toBe("browser-process");
});

test("primed resource Blob cache avoids network reads", async () => {
    dom.localStorage.setItem("yingce:active-user-scope", "image-tool-test");
    const blob = new Blob(["cached pixels"], { type: "image/png" });
    await primeResourceBlobCache("resource:cached", blob);
    globalThis.fetch = (async () => {
        throw new Error("must reuse Blob cache");
    }) as typeof fetch;
    await render(<Probe source="https://r2.example/cached.png" storageKey="resource:cached" />);
    expect(host.textContent).toBe("1200x800");
    expect(accessPayloads).toHaveLength(0);
});

test("concurrent tools share one resource download and reuse its Blob afterward", async () => {
    let downloads = 0;
    globalThis.fetch = (async () => {
        downloads++;
        return new Response(new Blob(["shared pixels"], { type: "image/png" }));
    }) as typeof fetch;
    const input = { url: "https://r2.example/display.png", storageKey: "resource:shared" };
    const [first, second] = await Promise.all([imageToDataUrl(input), imageToDataUrl(input)]);
    expect(first).toBe(second);
    expect(await imageToDataUrl(input)).toBe(first);
    expect(downloads).toBe(1);
    expect(accessPayloads).toHaveLength(1);
});

test("switching source and closing prevents late reads from updating the preview", async () => {
    let complete!: (response: Response) => void;
    let signal: AbortSignal | null | undefined;
    globalThis.fetch = (async (_url, options) => {
        signal = options?.signal;
        return new Promise<Response>((resolve) => {
            complete = resolve;
        });
    }) as typeof fetch;
    await render(<Probe source="https://r2.example/slow.png" />);
    await render(<Probe source="data:image/png;base64,new" />);
    expect(signal?.aborted).toBe(true);
    await act(async () => complete(new Response(new Blob(["old"], { type: "image/png" }))));
    await settle();
    expect(host.querySelector("img")?.getAttribute("src")).toBe("data:image/png;base64,new");
    await render(<Probe source="https://r2.example/closing.png" />);
    await act(() => root!.unmount());
    root = undefined;
    expect(signal?.aborted).toBe(true);
    complete(new Response(new Blob(["closed"], { type: "image/png" })));
    await settle();
    expect(host.childNodes.length).toBe(0);
});

test("real HTTP failures propagate, and failed annotation can be closed and reopened", async () => {
    globalThis.fetch = (async () => new Response("forbidden", { status: 403 })) as typeof fetch;
    await expect(getResourceBlob("resource:denied")).rejects.toThrow("资源读取失败（403）");
    await expect(imageToDataUrl({ url: "https://r2.example/denied.png" })).rejects.toThrow("原图读取失败（403）");
    const props = { image: { url: "https://r2.example/denied.png" }, onClose: () => {}, onConfirm: () => {} };
    await render(<CanvasNodeAnnotationDialog {...props} open />);
    expect(dom.document.querySelector('[role="alert"]')?.textContent).toContain("403");
    expect([...dom.document.querySelectorAll("button")].find((button) => button.textContent?.includes("保存为新节点"))?.disabled).toBe(true);
    await render(<CanvasNodeAnnotationDialog {...props} open={false} />);
    await render(<CanvasNodeAnnotationDialog {...props} image={{ url: "data:image/png;base64,valid" }} open />);
    expect(dom.document.querySelector('img[alt="待标注图片"]')?.getAttribute("src")).toBe("data:image/png;base64,valid");
    expect(dom.document.querySelector('[role="alert"]')).toBeNull();
});

test("local data URLs skip fetching and broken image decode stays a failure", async () => {
    globalThis.fetch = (async () => {
        throw new Error("unexpected network");
    }) as typeof fetch;
    await render(<Probe source="data:image/png;base64,local" />);
    expect(host.textContent).toBe("1200x800");
    await render(<Probe source="data:image/png;base64,broken" />);
    expect(host.textContent).toBe("图片加载失败，无法处理该图片");
    expect(host.querySelector("img")).toBeNull();
});

test("local Blob input and aborted reads retain their original semantics", async () => {
    const url = URL.createObjectURL(new Blob(["local pixels"], { type: "image/png" }));
    try {
        expect(await imageToDataUrl({ url })).toStartWith("data:image/png;");
    } finally {
        URL.revokeObjectURL(url);
    }
    const request = new AbortController();
    request.abort();
    await expect(imageToDataUrl({ url: "https://r2.example/unused.png" }, { signal: request.signal })).rejects.toMatchObject({ name: "AbortError" });
    const failure = new TypeError("Failed to fetch");
    globalThis.fetch = (async () => {
        throw failure;
    }) as typeof fetch;
    await expect(imageToDataUrl({ url: "https://r2.example/cors-denied.png" })).rejects.toBe(failure);
});
