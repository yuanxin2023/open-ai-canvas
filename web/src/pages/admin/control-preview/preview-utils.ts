import dayjs, { type Dayjs } from "dayjs";
import type { PreviewModel, PreviewRange } from "@/services/api/control-preview";

export const previewPeriods = [
    { value: "day", label: "今日" },
    { value: "week", label: "本周" },
    { value: "month", label: "本月" },
] as const;

export function previewRangeDates(range: PreviewRange, now = Date.now()): [Dayjs, Dayjs] {
    if (range.period === "custom") return [dayjs(range.from), dayjs(range.to)];
    const today = dayjs(new Date(now + 8 * 60 * 60 * 1000).toISOString().slice(0, 10));
    const from = range.period === "week" ? today.subtract((today.day() + 6) % 7, "day") : range.period === "month" ? today.startOf("month") : today;
    return [from, today];
}

export function previewRangeLabel(range: PreviewRange) {
    return range.period === "custom" ? `${range.from} 至 ${range.to}` : previewPeriods.find((item) => item.value === range.period)!.label;
}
export const previewCapabilities = [
    { value: "", label: "全部" },
    { value: "text", label: "文本" },
    { value: "image", label: "图片" },
    { value: "video", label: "视频" },
    { value: "audio", label: "音频" },
] as const;
export const previewColors = ["var(--admin-preview-chart-1)", "var(--admin-preview-chart-2)", "var(--admin-preview-chart-3)", "var(--admin-preview-chart-4)", "var(--admin-preview-chart-5)", "var(--admin-preview-chart-other)"];

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

export function previewTrendTime(value: string) {
    return new Intl.DateTimeFormat("zh-CN", { timeZone: "Asia/Shanghai", year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false }).format(new Date(value));
}

export function previewModelKey(row: Pick<PreviewModel, "model" | "capability">) {
    return JSON.stringify([row.model, row.capability]);
}

export function previewModelSlices(models: PreviewModel[], metric: "generations" | "consumedMicrocredits", capability: string) {
    // Assign before filtering or metric sorting so chart controls do not recolor models.
    const keys = [...new Set(models.map(previewModelKey))].sort();
    const colors = new Map(keys.map((key, index) => [key, previewColors[index % (previewColors.length - 1)]]));
    const sorted = models.filter((row) => (!capability || row.capability === capability) && row[metric] > 0).sort((a, b) => b[metric] - a[metric] || previewModelKey(a).localeCompare(previewModelKey(b)));
    const rows: Array<{ key: string; label: string; value: number; color: string; model?: PreviewModel }> = sorted
        .slice(0, 5)
        .map((row) => ({ key: previewModelKey(row), label: row.model, value: row[metric], color: colors.get(previewModelKey(row))!, model: row }));
    if (sorted.length > 5) rows.push({ key: "other", label: "其他", value: sorted.slice(5).reduce((sum, row) => sum + row[metric], 0), color: previewColors[5] });
    return rows;
}
