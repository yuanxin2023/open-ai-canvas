import { expect, test } from "bun:test";
import { copyFile, mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { dirname, join, resolve } from "node:path";
import { tmpdir } from "node:os";

test("catalog scaffolding cannot roll back maintained image contracts or local documentation", async () => {
    const root = resolve(import.meta.dir, "../../plugin-packages");
    const fixture = await mkdtemp(join(tmpdir(), "canvas-plugin-merge-"));
    try {
        await copyFile(join(root, "generate-catalog.mjs"), join(fixture, "generate-catalog.mjs"));
        for (const id of ["openai-images", "xai-grok-images"]) {
            await mkdir(join(fixture, id, "docs"), { recursive: true });
            for (const file of ["manifest.json", "README.md", "docs/interface.md"]) await copyFile(join(root, id, file), join(fixture, id, file));
        }
        const marker = "local reviewed documentation must remain";
        await writeFile(join(fixture, "openai-images", "README.md"), marker);
        const before = await readFile(join(fixture, "openai-images", "manifest.json"), "utf8");
        const child = Bun.spawnSync(["node", join(fixture, "generate-catalog.mjs"), "openai-images", "xai-grok-images"]);
        expect(child.exitCode).toBe(0);
        expect(await readFile(join(fixture, "openai-images", "manifest.json"), "utf8")).toBe(before);
        expect(await readFile(join(fixture, "openai-images", "README.md"), "utf8")).toBe(marker);
        expect(new TextDecoder().decode(child.stdout)).toContain("preserved 2 maintained manifests");
    } finally {
        if (dirname(resolve(fixture)) !== resolve(tmpdir()) || !fixture.includes("canvas-plugin-merge-")) throw new Error("unexpected fixture path");
        await rm(fixture, { recursive: true, force: true });
    }
});
