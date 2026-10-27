import { expect, test } from "bun:test";
import { createHash } from "node:crypto";
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../../backend/", import.meta.url));
const baseline = JSON.parse(readFileSync(join(root, "internal/skills/testdata/builtin-merge-baseline.json"), "utf8")) as { localIDs: string[]; newIDs: string[]; preservedBodies: Record<string, string> };
const entries = readdirSync(join(root, "builtin"), { withFileTypes: true })
    .filter((entry) => entry.isDirectory())
    .map((entry) => {
        const source = readFileSync(join(root, "builtin", entry.name, "SKILL.md"), "utf8").replace(/\r\n/g, "\n");
        const end = source.indexOf("\n---", 4);
        const frontmatter = source.slice(0, end);
        const match = /^\s+skillId:\s*"([^"]+)"\s*$/m.exec(frontmatter);
        if (!match) throw new Error(`Missing stable skill ID: ${entry.name}`);
        const sourceType = /^\s+source:\s*(\d+)\s*$/m.exec(frontmatter);
        return { id: match[1], source, sourceType: sourceType ? Number(sourceType[1]) : 3, body: source.slice(end + 4).trim(), disabled: /^\s+defaultDisabled:\s*true\s*$/m.test(frontmatter) };
    });

test("canonical sources preserve all 108 local identities and add exactly 37 optional skills", () => {
    expect(new Set(entries.map((item) => item.id)).size).toBe(145);
    expect(entries.map((item) => item.id).sort()).toEqual([...baseline.localIDs, ...baseline.newIDs].sort());
    expect(
        entries
            .filter((item) => item.disabled)
            .map((item) => item.id)
            .sort(),
    ).toEqual([...baseline.newIDs].sort());
    expect(entries.some((item) => /^yingce-image-(editing|annotation|layer-split)$/.test(item.id))).toBe(false);
    expect(entries.every((item) => item.sourceType === 3)).toBe(true);
});

test("all 72 legacy card bodies and three image workflows survive byte-equivalent migration", () => {
    expect(Object.keys(baseline.preservedBodies)).toHaveLength(75);
    for (const [id, hash] of Object.entries(baseline.preservedBodies)) {
        const body = entries.find((item) => item.id === id)?.body;
        expect(body).toBeDefined();
        expect(createHash("sha256").update(body!).digest("hex")).toBe(hash);
    }
});

test("the eight preset recipes reference canonical catalog identities without changing their availability", () => {
    const data = JSON.parse(readFileSync(join(root, "internal/skills/seed/presets.json"), "utf8")) as { presets: { presetId: string; skillIds: string[] }[] };
    expect(data.presets).toHaveLength(8);
    expect(new Set(data.presets.map((preset) => preset.presetId)).size).toBe(8);
    const ids = new Set(entries.map((item) => item.id));
    for (const preset of data.presets) {
        expect(preset.skillIds.length).toBeGreaterThan(0);
        expect(preset.skillIds.length).toBeLessThanOrEqual(8);
        expect(new Set(preset.skillIds).size).toBe(preset.skillIds.length);
        for (const id of preset.skillIds) expect(ids.has(id)).toBe(true);
    }
    expect(entries.filter((item) => item.disabled)).toHaveLength(37);
});
