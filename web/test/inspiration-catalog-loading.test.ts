import { afterAll, expect, test } from "bun:test";
import { invalidateInspirationCatalog, loadGalleryPool, loadInspirationCatalog } from "@/services/inspiration-catalog";
import { apiClient } from "@/services/api/request";
import { useUserStore, defaultFeatureAvailability } from "@/stores/use-user-store";

const original = globalThis.fetch;
const originalAdapter = apiClient.defaults.adapter;
let feedData = { enabled: false, entries: [] as any[], categories: {} };
let feedFailure = "";
apiClient.defaults.adapter = async (config) => {
    expect(config.url).toBe("/public/inspirations/feed");
    expect(config.timeout).toBe(5_000);
    if (feedFailure) throw new Error(feedFailure);
    return { config, status: 200, statusText: "OK", headers: {}, data: { code: 0, data: feedData, msg: "ok" } };
};
const calls: string[] = [];
globalThis.fetch = (async (input) => {
    const url = String(input);
    calls.push(url);
    const data = url.endsWith("manifest.json") ? { sources: {} } : { entries: [{ id: `seedance:${url}`, sourceId: "seedance", title: "Static", description: "", image: "/inspiration-thumbs/local.webp", mode: "video", prompt: "Static draft" }] };
    return new Response(JSON.stringify(data), { status: 200, headers: { "Content-Type": "application/json" } });
}) as typeof fetch;
afterAll(() => {
    globalThis.fetch = original;
    apiClient.defaults.adapter = originalAdapter;
});

test("both closed loaders reject before requesting any directory", async () => {
    useUserStore.setState({ features: { ...defaultFeatureAvailability } });
    await expect(loadInspirationCatalog()).rejects.toThrow("暂未开放");
    await expect(loadGalleryPool()).rejects.toThrow("暂未开放");
    expect(calls).toEqual([]);
});
test("hall only loads the adopted same-site snapshots after it opens", async () => {
    useUserStore.setState({ features: { ...defaultFeatureAvailability, inspirationHallEnabled: true } });
    const catalog = await loadInspirationCatalog();
    expect(catalog.entries.length).toBeGreaterThan(22);
    expect(calls).toEqual(["/inspirations/manifest.json", "/inspirations/seedance.json", "/inspirations/haohaoxue.json", "/inspirations/youmind.json"]);
    await expect(loadGalleryPool()).rejects.toThrow("暂未开放");
});
test("turning the feature off also blocks an already cached catalog", async () => {
    useUserStore.setState({ features: { ...defaultFeatureAvailability } });
    await expect(loadInspirationCatalog()).rejects.toThrow("暂未开放");
    expect(calls).toHaveLength(4);
});
test("gallery independently loads its local pool without model calls", async () => {
    useUserStore.setState({ features: { ...defaultFeatureAvailability, inspirationGalleryEnabled: true } });
    expect((await loadGalleryPool()).length).toBe(23);
    expect(calls.at(-1)).toBe("/inspirations/gallery-pool.json");
    expect(calls).toHaveLength(5);
});

test("feed adds new entries and failures preserve the static catalog", async () => {
    useUserStore.setState({ features: { ...defaultFeatureAvailability, inspirationHallEnabled: true } });
    feedData = { enabled: true, entries: [{ id: "seedance:new", sourceId: "seedance", title: "New feed", image: "https://example.com/new.webp", mode: "video", prompt: "New draft", addedAt: "2026-11-07T00:00:00Z" }], categories: {} };
    invalidateInspirationCatalog();
    expect((await loadInspirationCatalog()).entries.some((entry) => entry.title === "New feed")).toBe(true);
    feedFailure = "feed unavailable";
    invalidateInspirationCatalog();
    const catalog = await loadInspirationCatalog();
    expect(catalog.entries.length).toBeGreaterThan(22);
    expect(catalog.entries.some((entry) => entry.title === "New feed")).toBe(false);
});

test("an unresponsive static pack aborts and a later load retries it", async () => {
    const workingFetch = globalThis.fetch;
    const originalSetTimeout = globalThis.setTimeout;
    let aborted = false;
    // Accelerate only the directory deadline, keeping the production timeout value under assertion.
    globalThis.setTimeout = ((callback: any, delay: number, ...args: any[]) => originalSetTimeout(callback, delay === 15_000 ? 5 : delay, ...args)) as typeof setTimeout;
    globalThis.fetch = ((input: any, init: any) => {
        if (String(input).endsWith("seedance.json"))
            return new Promise((_resolve, reject) =>
                init.signal.addEventListener("abort", () => {
                    aborted = true;
                    reject(new DOMException("aborted", "AbortError"));
                }),
            );
        return workingFetch(input, init);
    }) as typeof fetch;
    try {
        invalidateInspirationCatalog();
        await expect(loadInspirationCatalog()).rejects.toThrow("灵感目录加载超时");
        expect(aborted).toBe(true);
    } finally {
        globalThis.fetch = workingFetch;
        globalThis.setTimeout = originalSetTimeout;
    }
    expect((await loadInspirationCatalog()).entries.length).toBeGreaterThan(22);
});

test("a feed timeout releases the catalog without changing other request defaults", async () => {
    const defaultsTimeout = apiClient.defaults.timeout;
    apiClient.defaults.adapter = (config) =>
        new Promise((_resolve, reject) => {
            expect(config.timeout).toBe(5_000);
            setTimeout(() => reject(new Error("timeout of 5000ms exceeded")), 5);
        });
    invalidateInspirationCatalog();
    expect((await loadInspirationCatalog()).entries.length).toBeGreaterThan(22);
    expect(apiClient.defaults.timeout).toBe(defaultsTimeout);
});
