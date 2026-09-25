import { describe, expect, test } from "bun:test";

import { applySlashCommand, findSlashCommandMatch } from "../src/lib/canvas/slash-command";

describe("creation prompt slash menu", () => {
    test("opens only for a slash at the start or after whitespace", () => {
        expect(findSlashCommandMatch("/电影", 3)).toEqual({ start: 0, end: 3, query: "电影" });
        expect(findSlashCommandMatch("请参考 /电影", 7)).toEqual({ start: 4, end: 7, query: "电影" });
        expect(findSlashCommandMatch("https://example.com", 8)).toBeNull();
        expect(findSlashCommandMatch("目录/电影", 5)).toBeNull();
    });

    test("replaces only the active slash query and leaves sending to the user", () => {
        const match = findSlashCommandMatch("开场 /电影 后续", 6);
        expect(match).not.toBeNull();
        expect(applySlashCommand("开场 /电影 后续", match!, "一段电影感开场")).toEqual({
            value: "开场 一段电影感开场 后续",
            cursor: 10,
        });
    });

    test("connects the shared composer to lazy personal and public prompt data", async () => {
        const [workspace, mentionEditor, styles] = await Promise.all([
            Bun.file(new URL("../src/pages/create/creation-workspace.tsx", import.meta.url)).text(),
            Bun.file(new URL("../src/components/canvas/canvas-resource-mention-textarea.tsx", import.meta.url)).text(),
            Bun.file(new URL("../src/styles/globals.css", import.meta.url)).text(),
        ]);

        expect(workspace).toContain('{ id: "personal", label: "我的提示词"');
        expect(workspace).toContain('{ id: "public", label: "公共提示词"');
        expect(workspace).toContain("onSlashCommandOpen={loadPromptLibrary}");
        expect(workspace).toContain("Promise.all([listAllUserPrompts(controller.signal), listInspirations(controller.signal)])");
        expect(workspace).toContain("previewUrl,");
        expect(mentionEditor).toContain("applySlashCommand(value, slashCommand, item.value)");
        expect(mentionEditor).toContain('event.key === "Enter" || event.key === "Tab"');
        expect(mentionEditor).toContain("onSelect(item)");
        expect(mentionEditor).toContain("canvas-slash-command-preview");
        expect(mentionEditor).toContain("onMouseEnter={() => setHoveredItemId(item.id)}");
        expect(mentionEditor).toContain("previewItem?.previewUrl");
        expect(styles).toContain(".canvas-slash-command-menu");
        expect(styles).toContain(".canvas-slash-command-preview > img");
    });
});
