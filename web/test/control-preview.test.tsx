import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import type { PreviewModel } from "../src/services/api/control-preview";
import { normalizePreviewRefresh, previewRefreshRemaining, previewRefreshStorageKey } from "../src/pages/admin/control-preview/preview-refresh";
import { ALL_ADMIN_PERMISSIONS, firstAdminPath, hasAdminPermission } from "../src/lib/admin-permissions";
import { PreviewChoice, PreviewEmpty, PreviewPanel } from "../src/pages/admin/control-preview/preview-controls";
import { previewColors, previewCredits, previewModelKey, previewModelSlices, previewTime, previewTrendTime, previewRangeDates, previewRangeLabel } from "../src/pages/admin/control-preview/preview-utils";
import { PreviewTrend } from "../src/pages/admin/control-preview/preview-trend";

function model(name: string, count: number, capability: PreviewModel["capability"] = "text"): PreviewModel {
    return { model: name, capability, generations: count, consumedMicrocredits: count * 1_000_000, succeeded: count, failed: 0, pending: 0 };
}

describe("control preview", () => {
    test("refresh preferences validate saved values and isolate accounts from request logs", () => {
        expect(normalizePreviewRefresh(null)).toEqual({ enabled: true, intervalSeconds: 30 });
        expect(normalizePreviewRefresh({ enabled: false, intervalSeconds: 120 })).toEqual({ enabled: false, intervalSeconds: 120 });
        expect(normalizePreviewRefresh({ enabled: "true", intervalSeconds: 1 })).toEqual({ enabled: true, intervalSeconds: 30 });
        expect(previewRefreshStorageKey("one")).not.toBe(previewRefreshStorageKey("two"));
        expect(previewRefreshStorageKey("one")).not.toContain("api-log-auto-refresh");
        expect(previewRefreshRemaining(30_000, 1_001)).toBe(29);
        expect(previewRefreshRemaining(30_000, 31_000)).toBe(0);
    });
    test("date picker shortcuts use Beijing dates across midnight and preserve custom bounds", () => {
        const now = Date.parse("2026-11-01T16:01:00Z");
        const dates = (range: Parameters<typeof previewRangeDates>[0]) => previewRangeDates(range, now).map((date) => date.format("YYYY-MM-DD"));
        expect(dates({ period: "day" })).toEqual(["2026-11-02", "2026-11-02"]);
        expect(dates({ period: "week" })).toEqual(["2026-11-02", "2026-11-02"]);
        expect(dates({ period: "month" })).toEqual(["2026-11-01", "2026-11-02"]);
        const custom = { period: "custom" as const, from: "2026-10-04", to: "2026-11-02" };
        expect(dates(custom)).toEqual([custom.from, custom.to]);
        expect(previewRangeLabel(custom)).toBe("2026-10-04 至 2026-11-02");
    });
    test("top five plus other preserve the full denominator and capability scope", () => {
        const rows = [model("same", 30), model("same", 20, "image"), ...Array.from({ length: 6 }, (_, index) => model(`m${index}`, index + 1))];
        const copy = [...rows];
        const slices = previewModelSlices(rows, "generations", "");
        expect(slices).toHaveLength(6);
        expect(slices.at(-1)?.label).toBe("其他");
        expect(slices.at(-1)?.model).toBeUndefined();
        expect(slices.reduce((sum, row) => sum + row.value, 0)).toBe(71);
        expect(slices[0].key).not.toBe(slices[1].key);
        expect(rows).toEqual(copy);
        expect(previewModelSlices(rows, "consumedMicrocredits", "image")).toEqual([{ key: previewModelKey(rows[1]), label: "same", value: 20_000_000, color: slices[1].color, model: rows[1] }]);
        expect(slices.at(-1)?.color).toBe(previewColors[5]);
        expect(previewModelSlices([model("unused", 0)], "generations", "")).toEqual([]);
    });

    test("model colors survive metric sorting, capability filtering and response ordering", () => {
        const rows = [model("same", 30), model("same", 20, "image"), model("video", 10, "video")];
        rows[2].consumedMicrocredits = 90_000_000;
        const calls = previewModelSlices(rows, "generations", "");
        const consumption = previewModelSlices([...rows].reverse(), "consumedMicrocredits", "");
        expect(calls[0].key).not.toBe(consumption[0].key);
        expect(new Set(calls.map((row) => row.color)).size).toBe(3);
        for (const row of calls) {
            expect(consumption.find((other) => other.key === row.key)?.color).toBe(row.color);
            expect(previewModelSlices(rows, "consumedMicrocredits", row.model!.capability).find((other) => other.key === row.key)?.color).toBe(row.color);
        }
    });

    test("formatting distinguishes missing data and zero and always uses Beijing time", () => {
        expect(previewCredits(undefined)).toBe("--");
        expect(previewCredits(0)).toBe("0");
        expect(previewCredits(1_234_567)).toBe("1.234567");
        expect(previewCredits(1)).toBe("0.000001");
        expect(previewTime("2026-10-24T02:03:04Z")).toBe("10:03:04");
        expect(previewTrendTime("2026-10-24T16:00:00Z")).toBe("2026/10/25 00:00");
    });

    test("trend distinguishes missing buckets from real zero buckets and keeps its accessible unit", () => {
        expect(renderToStaticMarkup(<PreviewTrend trend={[]} />)).toContain("当前统计范围暂无数据");
        const trend = [{ time: "2026-10-24T02:00:00Z", label: "10:00", inflowMicrocredits: 0, consumedMicrocredits: 0 }];
        const empty = renderToStaticMarkup(<PreviewTrend trend={trend} />);
        expect(empty).toContain("当前范围暂无到账与消费");
        expect(empty).toContain("单位为积分");
        const populated = renderToStaticMarkup(<PreviewTrend trend={[{ ...trend[0], consumedMicrocredits: 1 }]} />);
        expect(populated).not.toContain("当前范围暂无到账与消费");
    });

    test("new permission is explicitly grantable without granting unrelated financial modules", () => {
        const scoped = { level: "scoped" as const, permissions: ["admin.analytics.control_preview" as const] };
        expect(ALL_ADMIN_PERMISSIONS).toContain("admin.analytics.control_preview");
        expect(firstAdminPath(scoped)).toBe("/admin/control-preview");
        expect(hasAdminPermission(scoped, "admin.finance.credits")).toBe(false);
        expect(hasAdminPermission({ level: "scoped", permissions: ["admin.analytics.overview"] }, "admin.analytics.control_preview")).toBe(false);
    });

    test("real selection controls expose persistent selected state to keyboard users", () => {
        const html = renderToStaticMarkup(
            <PreviewChoice
                label="刷新间隔"
                options={[
                    { value: 0, label: "暂停" },
                    { value: 30_000, label: "30s" },
                ]}
                value={30_000}
                onChange={() => {}}
            />,
        );
        expect(html).toContain('role="group" aria-label="刷新间隔"');
        expect(html).toContain('aria-pressed="false">暂停');
        expect(html).toContain('aria-pressed="true">30s');
        const panel = renderToStaticMarkup(
            <PreviewPanel title="模型使用分布">
                <PreviewEmpty />
            </PreviewPanel>,
        );
        expect(panel).toContain("模型使用分布");
        expect(panel).toContain("当前统计范围暂无数据");
    });
});
