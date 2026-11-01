import { expect, test } from "bun:test";
import { resolve } from "node:path";

test("guest model picker and login draft lifecycle", () => {
    const result = Bun.spawnSync([process.execPath, "test", "./test/fixtures/creation-guest-models.harness.tsx"], { cwd: resolve(import.meta.dir, ".."), timeout: 20000 });
    const output = new TextDecoder().decode(result.stdout) + new TextDecoder().decode(result.stderr);
    expect(result.exitCode, output).toBe(0);
    expect(output).toContain("7 pass");
    expect(output).toContain("0 fail");
}, 25000);
