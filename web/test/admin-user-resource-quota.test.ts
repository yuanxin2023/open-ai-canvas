import { describe, expect, test } from "bun:test";
import { buildResourceQuotaPatch } from "../src/pages/admin/users/user-resource-quota-patch";
import { resourcePolicyFields } from "../src/pages/admin/lib/resource-policy-fields";

describe("per-user resource quota patches", () => {
    test("keeps inherited fields omitted and sends null only for reset", () => {
        expect(buildResourceQuotaPatch({}, {})).toEqual({});
        expect(buildResourceQuotaPatch({ resourceUploadMB: 10, storedFileGB: 5 }, { resourceUploadMB: 20, storedFileGB: 5 })).toEqual({ resourceUploadMB: 20 });
        expect(buildResourceQuotaPatch({ resourceUploadMB: 10, storedFileGB: 5 }, { storedFileGB: 5 })).toEqual({ resourceUploadMB: null });
    });
    test("an explicit value equal to the default remains an override", () => {
        expect(buildResourceQuotaPatch({}, { storedFileGB: 20 })).toEqual({ storedFileGB: 20 });
        expect(buildResourceQuotaPatch({ recycleBinRetentionDays: 10 }, { recycleBinRetentionDays: 0 })).toEqual({ recycleBinRetentionDays: 0 });
    });
    test("covers all eleven resource fields without task scheduling or request limits", () => {
        expect(new Set(resourcePolicyFields.map((field) => field.name)).size).toBe(11);
        expect(resourcePolicyFields.every((field) => field.group === "resource")).toBe(true);
        expect(resourcePolicyFields.find((field) => field.name === "recycleBinRetentionDays")?.min).toBe(0);
    });
});
