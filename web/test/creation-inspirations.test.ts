import { describe, expect, test } from "bun:test";
import { inspirationCoverUrl, type Inspiration } from "../src/services/api/inspirations";

describe("curated creation inspirations", () => {
    test("keeps built-in and HTTPS cover URLs untouched", () => {
        expect(inspirationCoverUrl({ coverUrl: "/short-drama-styles/future-tech.jpg" })).toBe("/short-drama-styles/future-tech.jpg");
        expect(inspirationCoverUrl({ coverUrl: "https://cdn.example.com/cover.webp" })).toBe("https://cdn.example.com/cover.webp");
    });
    test("resolves managed cover API paths", () => {
        expect(inspirationCoverUrl({ coverUrl: "/api/inspirations/INS-1/cover" })).toContain("/inspirations/INS-1/cover");
    });
    test("exposes persisted cover dimensions for stable mixed-ratio layouts", () => {
        const inspiration = { coverWidth: 900, coverHeight: 1200 } as Inspiration;
        expect(inspiration.coverWidth / inspiration.coverHeight).toBe(0.75);
    });
    test("personal inspirations expose the shared create-prompt dialog", async () => {
        const [workspace, editor, editorStyles, promptsPage] = await Promise.all([
            Bun.file(new URL("../src/pages/create/creation-workspace.tsx", import.meta.url)).text(),
            Bun.file(new URL("../src/components/user-prompt-editor-modal.tsx", import.meta.url)).text(),
            Bun.file(new URL("../src/components/user-prompt-editor-modal.css", import.meta.url)).text(),
            Bun.file(new URL("../src/pages/prompts/index.tsx", import.meta.url)).text(),
        ]);
        expect(workspace).toContain('source === "personal" ? <button type="button" className="creation-inspiration-add"');
        expect(workspace).toContain("<UserPromptEditorModal");
        expect(workspace).toContain("setInspirations((current) => [created, ...current.filter");
        expect(editor).toContain("await createUserPrompt(input)");
        expect(editor).toContain('className="product-collection-card creation-featured-card user-prompt-preview-card"');
        expect(editor).toContain('source?.trim() || "个人灵感"');
        expect(editorStyles).toContain("width: min(162px, 100%)");
        expect(editorStyles).not.toContain("aspect-ratio: 4 / 3");
        expect(promptsPage).toContain("<UserPromptEditorModal");
    });
});
