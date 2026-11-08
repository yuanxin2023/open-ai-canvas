import { expect, test } from "bun:test";
import { resolve } from "node:path";

test("套餐中心支付、兑换及账号隔离交互", () => {
    const result = Bun.spawnSync([process.execPath, "test", "./test/fixtures/package-center-flow.harness.tsx"], { cwd: resolve(import.meta.dir, ".."), timeout: 20000 });
    const output = new TextDecoder().decode(result.stdout) + new TextDecoder().decode(result.stderr);
    expect(result.exitCode, output).toBe(0);
    expect(output).toContain("0 fail");
});
