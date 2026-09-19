import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

function source(path: string) {
    return readFileSync(resolve(import.meta.dir, path), "utf8");
}

function modelPickerTags(content: string) {
    return content.match(/<ModelPicker[\s\S]*?\/>/g) ?? [];
}

describe("canvas model picker direct list", () => {
    test("opens the grouped model list directly while preserving the existing two-level default", () => {
        const picker = source("../src/components/model-picker.tsx");
        const styles = source("../src/styles/globals.css");

        expect(picker).toContain("directList?: boolean");
        expect(picker).toContain("directList = false");
        expect(picker).toContain('directList ? "is-direct-list"');
        expect(picker).toContain('className="canvas-model-picker-group-label"');
        expect(picker).toContain("showDescription");
        expect(picker).not.toContain("creation-model-picker-heading");
        expect(picker).toContain('aria-label="选择模型品牌"');
        expect(styles).toContain(".creation-model-picker-surface .creation-model-picker-menu.is-direct-list");
        expect(styles).toContain("overflow-y: auto !important");
    });

    test("enables the direct list for every canvas model selection entry", () => {
        const canvasPickerFiles = [
            "../src/pages/create/creation-workspace.tsx",
            "../src/components/canvas/canvas-agent-image-approval-settings.tsx",
            "../src/components/canvas/canvas-cloud-agent-panel.tsx",
            "../src/components/canvas/canvas-cloud-agent-settings.tsx",
            "../src/components/canvas/canvas-node-mask-edit-dialog.tsx",
            "../src/components/canvas/canvas-node-prompt-panel.tsx",
            "../src/components/canvas/canvas-prompt-optimizer-drawer.tsx",
            "../src/components/canvas/canvas-script-node.tsx",
            "../src/components/canvas/canvas-video-segment-dialog.tsx",
        ];

        for (const path of canvasPickerFiles) {
            const tags = modelPickerTags(source(path));
            expect(tags.length).toBeGreaterThan(0);
            for (const tag of tags) expect(tag).toContain("directList");
        }
    });

    test("does not change model pickers outside the canvas flow", () => {
        const projectSettings = source("../src/pages/projects/detail/settings.tsx");
        const accountSettings = source("../src/pages/settings/agent-memory-pane.tsx");

        expect(modelPickerTags(projectSettings).every((tag) => !tag.includes("directList"))).toBe(true);
        expect(modelPickerTags(accountSettings).every((tag) => !tag.includes("directList"))).toBe(true);
    });
});
