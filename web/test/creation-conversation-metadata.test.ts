import { expect, test } from "bun:test";

import { compareCreationConversations, creationConversationDisplayTitle, creationConversationTitle } from "../src/pages/create/creation-conversations";
import type { CreationConversation } from "../src/pages/create/creation-types";

const updatedAt = "2026-09-25T08:41:00.000Z";

function conversation(overrides: Partial<CreationConversation> = {}): CreationConversation {
    return {
        id: "conversation",
        title: "新创作",
        updatedAt,
        messages: [{ id: "message", role: "user", mode: "text", content: "你好", createdAt: updatedAt }],
        ...overrides,
    };
}

test("creation conversation titles make generic prompts distinguishable without replacing manual titles", () => {
    expect(creationConversationTitle("你好！", "text", updatedAt).startsWith("文本创作 · ")).toBe(true);
    expect(creationConversationTitle("  雨夜天台，镜头缓缓推近主角  ", "video", updatedAt)).toBe("雨夜天台，镜头缓缓推近主角");
    expect(creationConversationTitle("一".repeat(40), "image", updatedAt)).toBe(`${"一".repeat(28)}…`);
    expect(creationConversationDisplayTitle(conversation({ title: "你好" })).startsWith("文本创作 · ")).toBe(true);
    expect(creationConversationDisplayTitle(conversation({ title: "你好", titleEdited: true }))).toBe("你好");
});

test("pinned creation conversations sort before newer unpinned conversations", () => {
    const pinned = conversation({ id: "pinned", pinned: true, updatedAt: "2026-09-20T00:00:00.000Z" });
    const newest = conversation({ id: "newest", updatedAt: "2026-09-25T00:00:00.000Z" });
    const older = conversation({ id: "older", updatedAt: "2026-09-24T00:00:00.000Z" });
    expect([older, newest, pinned].sort(compareCreationConversations).map((item) => item.id)).toEqual(["pinned", "newest", "older"]);
});
