import { describe, expect, test } from "bun:test";
import { existsSync, readFileSync } from "node:fs";
import { renderToStaticMarkup } from "react-dom/server";
import { App } from "antd";
import { AgentAvatarVideo } from "../src/components/canvas/agent-avatar-video";
import { CanvasAppearanceEditor } from "../src/pages/admin/settings/components/canvas-appearance-editor";
import { agentAvatarClipForState, builtInAgentAvatarURL, DEFAULT_CANVAS_APPEARANCE, isAgentAvatarBundleManifest } from "../src/lib/canvas/agent-appearance";

const manifest = JSON.parse(readFileSync(new URL("../public/doll/avatar.json", import.meta.url), "utf8"));

describe("official built-in Agent mascot", () => {
    test("ships every clip referenced by the manifest", () => {
        expect(DEFAULT_CANVAS_APPEARANCE.avatarType).toBe("bundle");
        expect(isAgentAvatarBundleManifest(manifest)).toBe(true);
        for (const clip of Object.values(manifest.clips) as string[]) {
            const file = new URL(`../public/doll/${clip}`, import.meta.url);
            expect(existsSync(file)).toBe(true);
            expect(readFileSync(file).subarray(4, 8).toString()).toBe("ftyp");
        }
    });

    test("selects the upstream animation for all eight run states", () => {
        const expected = { idle: "idle", queued: "sleep-loop", running: "working-loop", waiting_approval: "think", completed: "wave", failed: "wipe-sweat", cancelled: "lie-down", rejected: "think" };
        for (const [state, clip] of Object.entries(expected)) expect(agentAvatarClipForState(state, manifest)).toBe(`mascot-${clip}.mp4`);
        for (const state of ["unknown", "toString", "constructor"]) expect(agentAvatarClipForState(state, manifest)).toBe("mascot-idle.mp4");
    });

    test("missing state clips fall back to idle or the first valid clip", () => {
        expect(agentAvatarClipForState("running", { ...manifest, clips: { idle: "idle.mp4" } })).toBe("idle.mp4");
        expect(agentAvatarClipForState("running", { ...manifest, clips: { wave: "wave.mp4" } })).toBe("wave.mp4");
    });

    test("rejects malformed dimensions, empty clips and external or traversal paths", () => {
        for (const value of [null, [], { ...manifest, width: 0 }, { ...manifest, height: 5000 }, { ...manifest, width: 1.5 }, { ...manifest, clips: {} }, { ...manifest, clips: [] }]) expect(isAgentAvatarBundleManifest(value)).toBe(false);
        for (const file of ["../idle.mp4", "https://example.com/a.mp4", "/a.mp4", "x.js", ".mp4"]) expect(isAgentAvatarBundleManifest({ ...manifest, clips: { idle: file } })).toBe(false);
    });

    test("resolves bundled media on the app origin", () => {
        expect(builtInAgentAvatarURL("mascot-idle.mp4")).toBe("/doll/mascot-idle.mp4");
    });

    test("muted inline playback is disabled for reduced motion", () => {
        const playing = renderToStaticMarkup(<AgentAvatarVideo src="/doll/mascot-idle.mp4" />);
        const paused = renderToStaticMarkup(<AgentAvatarVideo src="/doll/mascot-idle.mp4" paused />);
        expect(playing).toContain("autoPlay");
        expect(playing).toContain("muted");
        expect(playing).toContain("playsInline");
        expect(paused).not.toContain("autoPlay");
        expect(paused).toContain("canvas-agent-keyed-video");
    });

    test("admin displays the selected mascot inside the preview and retains Live2D controls", () => {
        const value = { ...DEFAULT_CANVAS_APPEARANCE, live2dResourceId: "existing-model", live2dEntry: "model.model3.json" };
        const markup = renderToStaticMarkup(
            <App>
                <CanvasAppearanceEditor value={value} onChange={() => {}} disabled={false} onUploading={() => {}} />
            </App>,
        );
        const preview = markup.slice(markup.indexOf('aria-label="Agent 形象预览"'));
        expect(preview).toContain('src="/doll/mascot-idle.mp4"');
        expect(preview).not.toContain("导入 Live2D 模型");
        expect(markup).toContain("预览已有 Live2D");
        expect(markup).toContain("移除模型引用");
    });
});
