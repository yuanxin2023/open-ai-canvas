import { describe, expect, test } from "bun:test";
import { inspirationCoverUrl } from "../src/services/api/inspirations";

describe("curated creation inspirations", () => {
    test("keeps built-in and HTTPS cover URLs untouched", () => {
        expect(inspirationCoverUrl({ coverUrl: "/short-drama-styles/future-tech.jpg" })).toBe("/short-drama-styles/future-tech.jpg");
        expect(inspirationCoverUrl({ coverUrl: "https://cdn.example.com/cover.webp" })).toBe("https://cdn.example.com/cover.webp");
    });
    test("resolves managed cover API paths", () => {
        expect(inspirationCoverUrl({ coverUrl: "/api/inspirations/INS-1/cover" })).toContain("/inspirations/INS-1/cover");
    });
});
