import { describe, expect, test } from "bun:test";
import { normalizeLegacyImageOptions } from "../src/lib/image-parameter-legacy";
import { defaultImageCapabilityConfig, type ImageCapabilityConfig } from "../src/lib/model-capabilities";
import { imageParameterRequest, resolveImageParameters, type ImageParameter } from "../src/lib/image-parameters";
import { initialChannelModelValues } from "../src/pages/admin/components/channel-model-editor-form";
import { priceTierToForm } from "../src/pages/admin/components/channel-model-price-tier-form";
import type { ChannelModel, ChannelModelPriceTier } from "../src/services/api/wallet";

const resolution: ImageParameter[] = [{ name: "imageResolution", label: "分辨率", type: "string", binding: "resolution", values: ["auto", "1k", "2k"], default: "1k", pricing: true }];
function profile(parameters = resolution): ImageCapabilityConfig {
    return { ...defaultImageCapabilityConfig("grok-image"), parameters };
}

describe("upstream image parameter merge", () => {
    test("legacy mappings retain each protocol's upstream meaning and input identity", () => {
        for (const [provider, quality, key, expected] of [
            ["openai-image", "1k", "quality", "low"],
            ["openai-image", "2k", "quality", "medium"],
            ["openai-image", "4k", "quality", "high"],
            ["grok-image", "high", "imageResolution", "2k"],
            ["grok-image", "4k", "imageResolution", "2k"],
            ["gemini-image", "high", "imageResolution", "4k"],
            ["subrouter-gemini-image", "medium", "imageResolution", "2k"],
            ["custom-seedream", "ultra", "quality", "ultra"],
        ]) {
            const original = { quality, rawMode: false, seed: 0 };
            const mapped = normalizeLegacyImageOptions(provider, original);
            expect(mapped[key]).toBe(expected);
            expect(mapped.rawMode).toBe(false);
            expect(mapped.seed).toBe(0);
            expect(original.quality).toBe(quality);
        }
        expect(() => normalizeLegacyImageOptions("grok-image", { quality: "2k", imageResolution: "1k" })).toThrow("冲突");
        expect(() => normalizeLegacyImageOptions("grok-image", { quality: "ultra" })).toThrow("无法等价转换");
    });
    test("strict requests preserve legacy quality before applying the new contract", () => {
        expect(imageParameterRequest(profile(), { quality: "2k" }, true, "grok-image")).toEqual({ imageResolution: "2k" });
        const quality = profile([{ name: "quality", label: "质量", type: "string", values: ["low", "medium", "high"], default: "low" }]);
        expect(imageParameterRequest(quality, { quality: "2k" }, true, "openai-image")).toEqual({ quality: "medium" });
        expect(() => imageParameterRequest(profile(), { quality: "ultra" }, true, "grok-image")).toThrow("无法等价转换");
    });
    test("explicit unknown or disabled fields cannot silently select defaults", () => {
        for (const extras of [{ extra: 0 }, { quality: "2k" }, { rawMode: false }]) {
            expect(() => imageParameterRequest(profile([...resolution, { name: "rawMode", label: "原始模式", type: "boolean", enabled: false }]), { imageParameters: JSON.stringify(extras) }, true)).toThrow("未启用");
        }
    });
    test("typed false and zero survive admission", () => {
        const image = profile([
            { name: "rawMode", label: "原始模式", type: "boolean", default: true },
            { name: "seed", label: "种子", type: "integer", min: 0, default: 7 },
        ]);
        expect(imageParameterRequest(image, { imageParameters: '{"rawMode":false,"seed":0}' }, true)).toEqual({ rawMode: false, seed: 0 });
    });
    test("legacy capability import preserves defaults and rejects incompatible values", () => {
        const image = defaultImageCapabilityConfig("grok-image");
        image.resolution = { supported: false, values: [], default: "auto" };
        image.quality = { supported: true, values: ["1k", "2k"], default: "2k" };
        const mapped = resolveImageParameters(image, resolution, "model", "grok-image");
        expect(mapped.resolution.default).toBe("2k");
        expect(mapped.quality.supported).toBe(false);
        expect(image.quality.default).toBe("2k");
        expect(() => resolveImageParameters({ ...image, quality: { supported: true, values: ["1k", "ultra"], default: "1k" } }, resolution, "model", "grok-image")).toThrow("无法等价转换");
        expect(() => resolveImageParameters({ ...image, quality: { supported: true, values: ["1k"], default: "2k" } }, resolution, "model", "grok-image")).toThrow("默认值");
    });
    test("price form projects conditions without changing amounts or frozen inputs", () => {
        const tier = { selector: { quality: "2k" }, unitPriceMicrocredits: 1_230_000, enabled: true, billingMode: "fixed_request" } as ChannelModelPriceTier;
        const openai = priceTierToForm(tier, profile(), "openai-image");
        expect(openai.quality).toBe("medium");
        expect(openai.unitPrice).toBe(1.23);
        expect(tier.selector).toEqual({ quality: "2k" });
        const grok = priceTierToForm(tier, profile(), "grok-image");
        expect(grok.imageResolution).toBe("2k");
        expect(grok.unitPrice).toBe(1.23);
        expect(priceTierToForm(tier).quality).toBe("2k");
    });
    test("OpenAI free quality declarations retain their equivalent default", () => {
        const image = defaultImageCapabilityConfig("openai-image");
        image.quality = { supported: true, values: [], default: "2k" };
        const mapped = resolveImageParameters(image, [{ name: "quality", label: "质量", type: "string", binding: "quality", pricing: true }], "model", "openai-image");
        expect(mapped.quality.default).toBe("medium");
        expect(mapped.quality.values).toEqual([]);
        expect(image.quality.default).toBe("2k");
    });
    test("invalid stored configuration remains visible and blocks automatic conversion", () => {
        const image = defaultImageCapabilityConfig("grok-image");
        image.resolution = { supported: false, values: [], default: "auto" };
        image.quality = { supported: true, values: ["ultra"], default: "ultra" };
        const item = { id: "existing", modelKey: "model", providerModelKey: "model", protocol: "grok-image", capability: "image", enabled: true, capabilityConfig: { version: 1, image } } as ChannelModel;
        const initial = initialChannelModelValues(item, [{ value: "grok-image", label: "Grok", capability: "image", create: "POST /images", contentType: "application/json", media: "image", imageParameters: resolution }]);
        expect(initial.capabilityMigrationError).toContain("无法等价转换");
        expect(initial.capabilityConfig?.image?.quality.values).toEqual(["ultra"]);
        expect(item.capabilityConfig?.image?.quality.default).toBe("ultra");
    });
});
