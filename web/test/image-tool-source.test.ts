import { expect, test } from "bun:test";
import { resolve } from "node:path";

// DOM 与存储驱动探测必须早于业务模块初始化，避免与其他文件的浏览器环境互相污染。
test("image tool reads, previews and lifecycle in an isolated browser environment", () => {
    const result = Bun.spawnSync([process.execPath, "test", "./test/fixtures/image-tool-source.harness.tsx"], { cwd: resolve(import.meta.dir, ".."), timeout: 20000 });
    const output = new TextDecoder().decode(result.stdout) + new TextDecoder().decode(result.stderr);
    expect(result.exitCode, output).toBe(0);
    expect(output).toContain("8 pass");
}, 25000);
