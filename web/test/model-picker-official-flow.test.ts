import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

function source(path: string) {
    return readFileSync(resolve(import.meta.dir, path), "utf8");
}

describe("official model picker flow", () => {
    test("uses the shared two-level model and channel layout", () => {
        const picker = source("../src/components/model-picker.tsx");

        expect(picker).not.toContain("directList");
        expect(picker).not.toContain("is-direct-list");
        expect(picker).toContain('activeGroupKey === null ? "is-brand-list" : "is-model-list"');
        expect(picker).toContain('className="canvas-model-picker-brands"');
        expect(picker).toContain('className="canvas-model-picker-two-pane"');
        expect(picker).toContain('className="canvas-model-picker-brand-rail"');
    });

    test("keeps homepage and canvas entries on the shared default flow", () => {
        const pickerFiles = [
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

        for (const path of pickerFiles) expect(source(path)).not.toContain("directList");
    });
});
