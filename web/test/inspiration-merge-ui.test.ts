import { expect, test } from "bun:test";
import { resolve } from "node:path";
test("inspiration cover consent, editor receipts and optional modules", () => {
    const result = Bun.spawnSync([process.execPath, "test", "./test/fixtures/inspiration-merge-ui.harness.tsx"], { cwd: resolve(import.meta.dir, ".."), timeout: 15000 });
    const output = new TextDecoder().decode(result.stdout) + new TextDecoder().decode(result.stderr);
    expect(result.exitCode, output).toBe(0);
    expect(output).toContain("10 pass");
    expect(output).toContain("0 fail");
}, 20000);
