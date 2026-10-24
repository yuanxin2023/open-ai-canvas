import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import type { PreviewModel } from "../src/services/api/control-preview";
import { ALL_ADMIN_PERMISSIONS, firstAdminPath, hasAdminPermission } from "../src/lib/admin-permissions";
import { PreviewChoice, PreviewEmpty, PreviewPanel } from "../src/pages/admin/control-preview/preview-controls";
import { previewCredits, previewModelKey, previewModelSlices, previewTime, previewUserColor } from "../src/pages/admin/control-preview/preview-utils";

function model(name: string, count: number, capability: PreviewModel["capability"] = "text"): PreviewModel {
    return { model: name, capability, generations: count, consumedMicrocredits: count * 1_000_000, succeeded: count, failed: 0, pending: 0 };
}

describe("control preview", () => {
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
        expect(previewModelSlices(rows, "consumedMicrocredits", "image")).toEqual([{ key: previewModelKey(rows[1]), label: "same", value: 20_000_000, color: "var(--admin-chart-primary)", model: rows[1] }]);
        expect(previewModelSlices([model("unused", 0)], "generations", "")).toEqual([]);
    });

    test("formatting distinguishes missing data and zero and always uses Beijing time", () => {
        expect(previewCredits(undefined)).toBe("--");
        expect(previewCredits(0)).toBe("0");
        expect(previewCredits(1_234_567)).toBe("1.234567");
        expect(previewTime("2026-10-24T02:03:04Z")).toBe("10:03:04");
        expect(previewUserColor("user-1")).toBe(previewUserColor("user-1"));
        expect(previewUserColor("user-1")).toMatch(/^var\(--admin-/);
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
