import { expect, test } from "bun:test";
import { resolve } from "node:path";

test("creation page history mobile and desktop behavior", () => {
    const result = Bun.spawnSync([process.execPath, "test", "./test/fixtures/creation-history-mobile.harness.tsx"], { cwd: resolve(import.meta.dir, ".."), timeout: 20000 });
    const output = new TextDecoder().decode(result.stdout) + new TextDecoder().decode(result.stderr);
    expect(result.exitCode, output).toBe(0);
    expect(output).toContain("3 pass");
    expect(output).toContain("0 fail");
}, 25000);
