/**
 * Export the checked-in frontend catalog into the backend's embedded,
 * append-only inspiration seed. This command is offline and deterministic.
 *
 * Usage: bun scripts/export-inspirations-backend-catalog.mjs
 */
import { readFileSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";

const WEB_DIR = resolve(import.meta.dir, "..");
const REPO_DIR = resolve(WEB_DIR, "..");
const PUBLIC_DIR = join(WEB_DIR, "public", "inspirations");
const LEGACY_SEED = join(REPO_DIR, "backend", "internal", "database", "seed", "inspirations.json");
const OUTPUT = join(REPO_DIR, "backend", "internal", "database", "seed", "inspiration-catalog.json");

const sources = {
    seedance: { repository: "https://github.com/LearnPrompt/awesome-seedance" },
    haohaoxue: { repository: "https://www.haohaoxue.com/prompts/" },
    youmind: { repository: "https://github.com/YouMind-OpenLab/awesome-nano-banana-pro-prompts" },
};

function readJSON(file) {
    return JSON.parse(readFileSync(file, "utf8"));
}

function normalizeTags(tags) {
    if (!Array.isArray(tags)) return [];
    const seen = new Set();
    return tags
        .map((tag) => String(tag).trim())
        .filter((tag) => tag && [...tag].length <= 24 && !seen.has(tag.toLowerCase()) && seen.add(tag.toLowerCase()))
        .slice(0, 8);
}

const legacy = readJSON(LEGACY_SEED);
const entries = legacy.map((item) => ({
    catalogKey: `builtin:${item.id}`,
    legacyId: item.id,
    title: item.title,
    description: item.description,
    mode: item.mode,
    prompt: item.prompt,
    tags: normalizeTags(item.tags),
    source: item.source || "",
    sourceId: item.source ? "chatgpt-prompts" : "original",
    sourceUrl: item.source ? "https://github.com/f/awesome-chatgpt-prompts" : "",
    coverUrl: item.coverUrl,
}));

for (const [sourceId, source] of Object.entries(sources)) {
    const pack = readJSON(join(PUBLIC_DIR, `${sourceId}.json`));
    for (const item of pack.entries || []) {
        entries.push({
            catalogKey: item.id,
            revision: pack.revision || "",
            title: item.title,
            description: item.description || item.category || ({ text: "文本创作灵感", image: "图片创作灵感", video: "视频创作灵感" }[item.mode] ?? "创作灵感"),
            mode: item.mode,
            prompt: item.promptZh || item.prompt,
            tags: normalizeTags(item.tags),
            source: item.credit || "",
            sourceId,
            sourceUrl: item.sourceUrl || source.repository,
            category: item.category || "",
            ratio: item.ratio || "",
            coverUrl: item.image,
        });
    }
}

const keys = new Set(entries.map((entry) => entry.catalogKey));
if (entries.length !== 1341 || keys.size !== entries.length) {
    throw new Error(`目录数量或 ID 异常：entries=${entries.length}, unique=${keys.size}`);
}
for (const entry of entries) {
    if (!entry.catalogKey || !entry.title || !entry.description || !entry.mode || !entry.prompt || !entry.coverUrl) {
        throw new Error(`目录条目字段不完整：${entry.catalogKey || entry.title || "unknown"}`);
    }
    if ([...entry.catalogKey].length > 160 || [...entry.title].length > 120 || [...entry.description].length > 240 || [...entry.prompt].length > 20000) {
        throw new Error(`目录条目超过后台字段限制：${entry.catalogKey}`);
    }
}

writeFileSync(OUTPUT, `${JSON.stringify({ entries })}\n`, "utf8");
console.log(`已导出 ${entries.length} 条官方灵感：${OUTPUT}`);
