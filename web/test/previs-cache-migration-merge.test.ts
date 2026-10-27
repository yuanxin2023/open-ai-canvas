import { expect, test } from "bun:test";
import { fileURLToPath } from "node:url";

test("实际画布缓存恢复保护与账号隔离", () => {
    const result = Bun.spawnSync([process.execPath, "test", "./test/fixtures/previs-cache-migration.harness.ts"], { cwd: fileURLToPath(new URL("..", import.meta.url)), timeout: 20000 });
    const output = new TextDecoder().decode(result.stdout) + new TextDecoder().decode(result.stderr);
    expect(result.exitCode, output).toBe(0);
    expect(output).toContain("4 pass");
    expect(output).toContain("0 fail");
}, 25000);
