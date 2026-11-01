import { expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { ImageParameterFields } from "../src/components/image-parameter-fields";
import { ImageSizePicker } from "../src/components/image-size-picker";
import { imageSizePresets } from "../src/lib/image-size-presets";
import { imageParameterRequest } from "../src/lib/image-parameters";
import { defaultConfig } from "../src/stores/use-config-store";
import { multiTierPixelProfile } from "./fixtures/image-multi-tier-profile";

test("multi-tier auto size keeps all declared presets and uses the grouped picker", () => {
    const profile = multiTierPixelProfile();
    expect(imageSizePresets(profile)).toHaveLength(30);
    const html = renderToStaticMarkup(<ImageParameterFields profile={profile} config={{ ...defaultConfig, size: "auto" }} onChange={() => {}} />);
    expect(html).toContain("宽高比");
    expect(html).toContain("4K");
    expect(html).toContain("尺寸由模型决定");
    expect(html).not.toContain("画幅 / 尺寸");
});

test("an explicit single tier remains restricted", () => {
    const profile = multiTierPixelProfile();
    profile.size.defaultTier = "2k";
    expect(imageSizePresets(profile)).toHaveLength(10);
    expect(imageSizePresets(profile).every((preset) => preset.tier === "2k")).toBe(true);
});

test("restored pixel size selects its tier and leaves quality independent", () => {
    const profile = multiTierPixelProfile();
    const html = renderToStaticMarkup(<ImageSizePicker profile={profile} size="3840x2160" quality="high" onChange={() => {}} />);
    expect(html).toMatch(/aria-pressed="true"[^>]*>4K/);
    expect(html).toContain("3840 × 2160 px");
    expect(imageParameterRequest(profile, { size: "3840x2160", quality: "high", imageResolution: "4k", count: "2" }, true)).toEqual({ size: "3840x2160", quality: "high", count: 2 });
});

test("missing presets keep the raw sizes available", () => {
    const profile = multiTierPixelProfile();
    profile.size.presets = [];
    const html = renderToStaticMarkup(<ImageParameterFields profile={profile} config={{ ...defaultConfig, size: "auto" }} onChange={() => {}} />);
    expect(html).toContain("画幅 / 尺寸");
    expect(html).toContain("3840x2160");
});

test("schema ratio models without a resolution parameter keep their ratio presets", () => {
    const profile = multiTierPixelProfile();
    profile.size.parameter = "aspect_ratio";
    profile.size.defaultTier = "2k";
    profile.size.values = ["auto", "16:9"];
    profile.parameters![0] = { ...profile.parameters![0], encoding: "ratio", values: profile.size.values };
    const html = renderToStaticMarkup(<ImageSizePicker profile={profile} size="16:9" onChange={() => {}} />);
    expect(html).toContain('title="16:9 · 2752 × 1536"');
    expect(html).not.toContain("未配置预设尺寸");
});

test("a declared pixel value without a preset remains selectable in the raw size list", () => {
    const profile = multiTierPixelProfile();
    profile.size.values.push("2000x1000");
    const html = renderToStaticMarkup(<ImageParameterFields profile={profile} config={{ ...defaultConfig, size: "auto" }} onChange={() => {}} />);
    expect(html).toContain("2000x1000");
    expect(html).toContain("image-size-pixels");
});
