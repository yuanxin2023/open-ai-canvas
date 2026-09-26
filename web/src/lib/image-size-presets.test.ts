import assert from "node:assert/strict";
import test from "node:test";

import { setImageAutoSizeEnabled } from "./image-size-presets";
import type { ImageCapabilityConfig } from "./model-capabilities";

const size: ImageCapabilityConfig["size"] = {
    parameter: "size",
    values: ["auto", "1024x1024", "1824x1024"],
    default: "auto",
    allowCustom: false,
};

test("setImageAutoSizeEnabled removes auto and selects the first concrete default when disabled", () => {
    assert.deepEqual(setImageAutoSizeEnabled(size, false), {
        ...size,
        values: ["1024x1024", "1824x1024"],
        default: "1024x1024",
    });
});

test("setImageAutoSizeEnabled keeps an existing concrete default when disabled", () => {
    assert.equal(setImageAutoSizeEnabled({ ...size, default: "1824x1024" }, false).default, "1824x1024");
});

test("setImageAutoSizeEnabled adds auto without changing an existing concrete default when enabled", () => {
    assert.deepEqual(setImageAutoSizeEnabled({ ...size, values: ["1024x1024"], default: "1024x1024" }, true), {
        ...size,
        values: ["auto", "1024x1024"],
        default: "1024x1024",
    });
});
