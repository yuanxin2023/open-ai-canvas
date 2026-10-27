import { createHash } from "node:crypto";
import { readFile, readdir, rename, writeFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = dirname(fileURLToPath(import.meta.url));
const names = (await readdir(root)).filter((name) => /\.(canvas|yingce|lovwow)-plugin$/.test(name)).sort();
const rows = [];
for (const name of names) {
  const digest = createHash("sha256").update(await readFile(join(root, name))).digest("hex");
  rows.push(`${digest}  ${name}`);
}
await writeFile(join(root, ".SHA256SUMS.tmp"), `${rows.join("\n")}\n`);
await rename(join(root, ".SHA256SUMS.tmp"), join(root, "SHA256SUMS"));
console.log(`wrote checksums for ${names.length} plugin packages`);
