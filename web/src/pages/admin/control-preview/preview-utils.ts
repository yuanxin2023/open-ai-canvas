import type { PreviewModel } from "@/services/api/control-preview";

export const previewPeriods = [
    { value: "day", label: "今日" },
    { value: "week", label: "本周" },
    { value: "month", label: "本月" },
] as const;
export const previewCapabilities = [
    { value: "", label: "全部" },
    { value: "text", label: "文本" },
    { value: "image", label: "图片" },
    { value: "video", label: "视频" },
    { value: "audio", label: "音频" },
] as const;
export const previewColors = ["var(--admin-chart-primary)", "var(--admin-chart-secondary)", "var(--admin-status-info)", "var(--admin-chart-warning)", "var(--admin-status-success)", "var(--admin-text-muted)"];

export function previewUserColor(userId: string) {
    let hash = 0;
    for (let index = 0; index < userId.length; index++) hash = (Math.imul(hash, 31) + userId.charCodeAt(index)) | 0;
    return previewColors[(hash >>> 0) % previewColors.length];
}

export function previewCredits(value: number | undefined) {
    return value === undefined ? "--" : new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 6 }).format(value / 1_000_000);
}

export function previewNumber(value: number | undefined) {
    return value === undefined ? "--" : new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 2 }).format(value);
}

export function previewPercent(value: number) {
    return `${(value * 100).toFixed(1)}%`;
}

export function previewTime(value: string, date = false) {
    return new Intl.DateTimeFormat("zh-CN", { timeZone: "Asia/Shanghai", ...(date ? ({ month: "2-digit", day: "2-digit" } as const) : {}), hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false }).format(new Date(value));
}

export function previewModelKey(row: Pick<PreviewModel, "model" | "capability">) {
    return JSON.stringify([row.model, row.capability]);
}

export function previewModelSlices(models: PreviewModel[], metric: "generations" | "consumedMicrocredits", capability: string) {
    const sorted = models.filter((row) => (!capability || row.capability === capability) && row[metric] > 0).sort((a, b) => b[metric] - a[metric] || previewModelKey(a).localeCompare(previewModelKey(b)));
    const rows: Array<{ key: string; label: string; value: number; color: string; model?: PreviewModel }> = sorted
        .slice(0, 5)
        .map((row, index) => ({ key: previewModelKey(row), label: row.model, value: row[metric], color: previewColors[index], model: row }));
    if (sorted.length > 5) rows.push({ key: "other", label: "其他", value: sorted.slice(5).reduce((sum, row) => sum + row[metric], 0), color: previewColors[5] });
    return rows;
}
