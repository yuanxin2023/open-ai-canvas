import { expect, test } from "bun:test";
import { resolve } from "node:path";

test("the actual changelog entry opens announcements only after a user click", () => {
    const result = Bun.spawnSync([process.execPath, "test", "./test/fixtures/update-announcement-merge-ui.harness.tsx"], { cwd: resolve(import.meta.dir, ".."), timeout: 15000 });
    const output = new TextDecoder().decode(result.stdout) + new TextDecoder().decode(result.stderr);
    expect(result.exitCode, output).toBe(0);
    expect(output).toContain("2 pass");
    expect(output).toContain("0 fail");
}, 20000);
