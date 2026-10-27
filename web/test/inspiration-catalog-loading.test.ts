import { afterAll, expect, test } from "bun:test";
import { loadGalleryPool, loadInspirationCatalog } from "@/services/inspiration-catalog";
import { useUserStore, defaultFeatureAvailability } from "@/stores/use-user-store";

const original = globalThis.fetch;
const calls: string[] = [];
globalThis.fetch = (async (input) => {
    const url = String(input);
    calls.push(url);
    const data = url.endsWith("manifest.json") ? { sources: {} } : { entries: [{ id: `seedance:${url}`, sourceId: "seedance", title: "Static", description: "", image: "/inspiration-thumbs/local.webp", mode: "video", prompt: "Static draft" }] };
    return new Response(JSON.stringify(data), { status: 200, headers: { "Content-Type": "application/json" } });
}) as typeof fetch;
afterAll(() => {
    globalThis.fetch = original;
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
