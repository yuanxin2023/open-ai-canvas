export function normalizeLegacyImageOptions(provider: string | undefined, input: Record<string, unknown>): Record<string, unknown> {
    provider = provider?.trim();
    const values = { ...input };
    if (provider === "openai-image") {
        const quality = typeof values.quality === "string" ? values.quality.trim().toLowerCase() : "";
        const mapped = ({ "1k": "low", "2k": "medium", "4k": "high" } as Record<string, string>)[quality];
        if (mapped) values.quality = mapped;
        return values;
    }
    const tiers = provider === "grok-image" ? ["1k", "2k"] : ["gemini-image", "subrouter-gemini-image", "agnes-image"].includes(provider || "") ? ["1k", "2k", "4k"] : provider === "km-kemei-seedream" ? ["1k", "1.5k", "2k"] : undefined;
    if (!tiers) return values;
    const aliases: Record<string, string> =
        provider === "grok-image" ? { low: "1k", standard: "1k", medium: "2k", hd: "2k", high: "2k", "4k": "2k" } : provider === "gemini-image" || provider === "subrouter-gemini-image" ? { low: "1k", medium: "2k", high: "4k" } : {};
    for (const name of ["quality", "resolution"]) {
        const raw = values[name];
        if (raw === undefined || raw === null || raw === "") continue;
        if (typeof raw !== "string") throw new Error(`旧图片参数 ${name} 类型无效，请核对渠道配置`);
        let value = raw.trim().toLowerCase();
        value = aliases[value] || value;
        if (name === "quality" && (value === "auto" || value === "any")) {
            if (values.imageResolution !== undefined && values.imageResolution !== null && values.imageResolution !== "") {
                delete values[name];
                continue;
            }
            value = "auto";
        }
        if (!tiers.includes(value) && value !== "*" && value !== "auto") throw new Error(`旧图片参数 ${name}=${value} 无法等价转换，请核对渠道配置`);
        const current = values.imageResolution;
        if (current !== undefined && current !== null && current !== "" && (typeof current !== "string" || current.trim().toLowerCase() !== value)) throw new Error("旧质量与图片分辨率条件冲突，请核对渠道配置");
        values.imageResolution = value;
        delete values[name];
    }
    return values;
}
