import { afterAll, expect, spyOn, test } from "bun:test";
import { Window } from "happy-dom";
import type { CreationMessage } from "@/pages/create/creation-types";
import type { ImageAsset } from "@/stores/use-asset-store";

const dom = new Window({ url: "http://localhost/create" });
const originals = new Map<string, PropertyDescriptor | undefined>();
for (const [key, value] of Object.entries({
    window: dom,
    document: dom.document,
    navigator: dom.navigator,
    HTMLElement: dom.HTMLElement,
    Element: dom.Element,
    Node: dom.Node,
    SVGElement: dom.SVGElement,
    ShadowRoot: dom.ShadowRoot,
    getComputedStyle: dom.getComputedStyle.bind(dom),
    IS_REACT_ACT_ENVIRONMENT: true,
})) {
    originals.set(key, Object.getOwnPropertyDescriptor(globalThis, key));
    Object.defineProperty(globalThis, key, { configurable: true, writable: true, value });
}
const { act } = await import("react");
const { createRoot } = await import("react-dom/client");
const { App } = await import("antd");
const { MediaResult } = await import("@/pages/create/creation-workspace-messages");
const { creationResultDownload } = await import("@/pages/create/creation-result-download");
const { apiClient } = await import("@/services/api/request");

const previousAdapter = apiClient.defaults.adapter;
const requests: Array<{ resourceId: string; purpose: string; variant: string; downloadName?: string }> = [];
const clicks: Array<{ href: string; name: string }> = [];
let rejectDownload = false;
let waitForDownload: Promise<void> = Promise.resolve();
apiClient.defaults.adapter = async (config) => {
    const payload = JSON.parse(String(config.data)) as typeof requests;
    requests.push(...payload);
    if (payload.some((item) => item.purpose === "download")) await waitForDownload;
    return {
        config,
        status: 200,
        statusText: "OK",
        headers: {},
        data: {
            code: 0,
            data: {
                items: payload.map((item) =>
                    item.resourceId.endsWith("missing") || (rejectDownload && item.purpose === "download")
                        ? { error: { msg: "下载授权失败" } }
                        : {
                              access: {
                                  resourceId: item.resourceId,
                                  url: `https://media.example/${item.resourceId}.png?purpose=${item.purpose}`,
                                  delivery: "origin",
                                  actualVariant: "original",
                                  requestedVariant: "original",
                                  issuedAt: new Date().toISOString(),
                              },
                          },
                ),
            },
        },
    };
};
const anchorClick = spyOn(dom.HTMLAnchorElement.prototype, "click").mockImplementation(function (this: HTMLAnchorElement) {
    clicks.push({ href: this.href, name: this.download });
});

afterAll(() => {
    anchorClick.mockRestore();
    apiClient.defaults.adapter = previousAdapter;
    dom.happyDOM.abort();
    for (const [key, value] of originals) {
        if (value) Object.defineProperty(globalThis, key, value);
        else Reflect.deleteProperty(globalThis, key);
    }
});

const noop = () => {};
async function mount(id: string, mode: "image" | "video" = "image", remote = true) {
    requests.length = 0;
    clicks.length = 0;
    const item: CreationMessage = {
        id,
        role: "assistant",
        mode,
        content: "",
        createdAt: "2026-11-18T00:00:00Z",
        status: "done",
        resultUrls: remote ? ["", `https://media.example/${id}.png`] : ["https://external.example/image.png"],
        resultStorageKeys: remote ? [`resource:${id}-missing`, `resource:${id}`] : [],
    };
    const host = document.createElement("div");
    document.body.append(host);
    const root = createRoot(host);
    await act(async () =>
        root.render(
            <App>
                <MediaResult item={item} onRetryFailure={noop} onRefreshResult={noop} onRecoverMedia={noop} onCreateVariant={noop} onContinueCanvas={noop} openingCanvas={false} />
            </App>,
        ),
    );
    await settle();
    return {
        host,
        button: () => Array.from(host.querySelectorAll<HTMLButtonElement>(".creation-media-actions button")).find((button) => button.textContent?.includes("下载"))!,
        dispose: async () => {
            await act(async () => root.unmount());
            host.remove();
        },
    };
}
async function settle() {
    for (let i = 0; i < 4; i++) await act(() => new Promise((resolve) => setTimeout(resolve, 20)));
}

test("首页多图部分未加载时按稳定身份申请原文件下载，等待期间防止重复点击", async () => {
    let release!: () => void;
    waitForDownload = new Promise<void>((resolve) => {
        release = resolve;
    });
    const view = await mount("download-image");
    try {
        expect(view.host.querySelector(".creation-media-actions a")).toBeNull();
        await act(async () => {
            view.button().click();
            view.button().click();
        });
        await settle();
        expect(view.button().disabled).toBe(true);
        expect(clicks).toHaveLength(0);
        expect(requests.filter((item) => item.purpose === "download")).toEqual([{ resourceId: "download-image", purpose: "download", variant: "original", downloadName: "生成图片_2.png" }]);
        await act(async () => release());
        await settle();
        expect(view.button().disabled).toBe(false);
        expect(clicks).toEqual([{ href: "https://media.example/download-image.png?purpose=download", name: "生成图片_2.png" }]);
        expect(dom.location.pathname).toBe("/create");
    } finally {
        release();
        waitForDownload = Promise.resolve();
        await view.dispose();
    }
});

test("下载授权失败显示错误并恢复按钮，不点击展示链接", async () => {
    rejectDownload = true;
    const view = await mount("download-failure");
    try {
        await act(async () => view.button().click());
        await settle();
        expect(document.body.textContent).toContain("下载授权失败");
        expect(view.button().disabled).toBe(false);
        expect(clicks).toHaveLength(0);
        expect(dom.location.pathname).toBe("/create");
    } finally {
        rejectDownload = false;
        await view.dispose();
    }
});

test("缺少资源身份的跨域结果提示重新获取，不跳转", async () => {
    const view = await mount("unregistered", "image", false);
    try {
        await act(async () => view.button().click());
        await settle();
        expect(document.body.textContent).toContain("请先重新获取结果后重试");
        expect(clicks).toHaveLength(0);
        expect(requests).toHaveLength(0);
    } finally {
        await view.dispose();
    }
});

test("视频结果也通过下载用途申请原文件", async () => {
    const view = await mount("download-video", "video");
    try {
        await act(async () => view.button().click());
        await settle();
        expect(requests.filter((item) => item.purpose === "download")[0]?.resourceId).toBe("download-video");
        expect(clicks[0]?.href).toContain("purpose=download");
    } finally {
        await view.dispose();
    }
});

test("文件名优先使用素材标题及真实 MIME，覆盖 PNG、JPEG、WebP、SVG", () => {
    for (const [mimeType, extension] of [
        ["image/png", "png"],
        ["image/jpeg", "jpg"],
        ["image/webp", "webp"],
        ["image/svg+xml", "svg"],
    ]) {
        const asset = { kind: "image", title: "海报/原图.png", data: { storageKey: "resource:format", dataUrl: "", mimeType } } as ImageAsset;
        expect(creationResultDownload({ key: "resource:format", url: "https://media.example/wrong.png" }, ["resource:format"], [asset], false, 0, dom.location.origin).fileName).toBe(`海报_原图.${extension}`);
    }
});

test("本地、同源资源可以下载，未知格式不强制添加 PNG 扩展名", () => {
    for (const [url, extension] of [
        ["blob:http://localhost/123", ""],
        ["data:image/svg+xml;base64,PHN2Zz4=", ".svg"],
        ["/images/source.webp?signature=test", ".webp"],
        ["/api/resources/local/file", ""],
    ]) {
        expect(creationResultDownload({ key: url, url }, [], [], false, 0, dom.location.origin).fileName).toBe(`生成图片_1${extension}`);
    }
});
