import { defaultImageCapabilityConfig } from "../../src/lib/model-capabilities";
import { IMAGE_RATIOS, IMAGE_RESOLUTIONS, imagePresetForRatio } from "../../src/lib/image-size-presets";

// Public catalog shape of the affected model; no provider credentials or pricing.
export function multiTierPixelProfile() {
    const profile = defaultImageCapabilityConfig("openai-image");
    const presets = IMAGE_RESOLUTIONS.flatMap((tier) => IMAGE_RATIOS.map((ratio) => imagePresetForRatio(tier, ratio)));
    const values = ["auto", ...presets.map((preset) => preset.size)];
    profile.size = { parameter: "size", default: "auto", allowCustom: true, values, presets };
    profile.resolution = { supported: false, values: [], default: "auto" };
    profile.parameters = [
        { name: "size", label: "画幅 / 尺寸", type: "string", encoding: "pixels", enabled: true, default: "auto", values },
        { name: "imageResolution", label: "分辨率", type: "string", enabled: false },
        { name: "quality", label: "图片质量", type: "string", default: "auto", values: ["auto", "low", "medium", "high"] },
        { name: "count", label: "生成张数", type: "integer", default: 1, min: 1, max: 15 },
    ];
    return profile;
}
