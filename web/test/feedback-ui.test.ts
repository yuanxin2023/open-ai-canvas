import { expect, test } from "bun:test";
import { resolve } from "node:path";

test("feedback form, inbox and admin preserve scope and confirmed behavior", () => {
    const result = Bun.spawnSync([process.execPath, "test", "./test/fixtures/feedback-ui.harness.tsx"], { cwd: resolve(import.meta.dir, ".."), timeout: 15000 });
    const output = new TextDecoder().decode(result.stdout) + new TextDecoder().decode(result.stderr);
    expect(result.exitCode, output).toBe(0);
    expect(output).toContain("0 fail");
    expect(output).toContain("8 pass");
}, 20000);
