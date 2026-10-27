import { expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { catalogIdOf, curatedInspirations } from "@/lib/inspirations/catalog";
import { inspirationCoverKind } from "@/lib/inspirations/cover";
import { fillablePrompt } from "@/lib/inspirations/resolve";

test("adopted frontend catalog IDs match the backend state contract", () => {
    const base = resolve(import.meta.dir, "..");
    const entries = ["seedance", "haohaoxue", "youmind"].flatMap((name) => JSON.parse(readFileSync(resolve(base, `public/inspirations/${name}.json`), "utf8")).entries);
    const ids = [...new Set([...entries, ...curatedInspirations].map(catalogIdOf))].sort();
    const accepted = JSON.parse(readFileSync(resolve(base, "../backend/internal/prompts/inspiration_catalog_ids.json"), "utf8"));
    expect(ids).toHaveLength(1341);
    expect(ids).toEqual(accepted);
    expect(entries.filter((entry) => inspirationCoverKind(entry.image) === "external")).toHaveLength(964);
    const gallery = JSON.parse(readFileSync(resolve(base, "public/inspirations/gallery-pool.json"), "utf8")).entries;
    expect(gallery).toHaveLength(329);
    expect(gallery.every((entry: any) => ids.includes(catalogIdOf(entry)) && inspirationCoverKind(entry.image) === "local")).toBe(true);
});

test("a preserved Chinese draft cannot silently replace a newer personal original", () => {
    const item = { title: "Personal", description: "", image: "", mode: "text" as const, prompt: "Updated original", promptZh: "保留的旧中文版本", promptZhNeedsReview: true };
    expect(fillablePrompt(item)).toBe("Updated original");
    expect(item.promptZh).toBe("保留的旧中文版本");
    expect(fillablePrompt({ ...item, promptZhNeedsReview: false })).toBe("保留的旧中文版本");
});
