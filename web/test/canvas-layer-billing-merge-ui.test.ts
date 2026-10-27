import { expect, test } from "bun:test";
import { resolve } from "node:path";

test("continuous layer generation requires model and billing acknowledgement", () => {
    const result = Bun.spawnSync([process.execPath, "test", "./test/fixtures/canvas-layer-billing-merge-ui.harness.tsx"], { cwd: resolve(import.meta.dir, ".."), timeout: 15000 });
    const output = new TextDecoder().decode(result.stdout) + new TextDecoder().decode(result.stderr);
    expect(result.exitCode, output).toBe(0);
    expect(output).toContain("2 pass");
    expect(output).toContain("0 fail");
}, 20000);
