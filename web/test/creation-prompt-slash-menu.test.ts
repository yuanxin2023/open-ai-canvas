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
});
