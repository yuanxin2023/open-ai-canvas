import type { RuntimeResourcePolicy, UserResourceQuotaOverrides, UserResourceQuotaPatch } from "@/services/api/auth";

export type ResourceQuotaDraft = Partial<Record<keyof RuntimeResourcePolicy, number | null>>;

export function buildResourceQuotaPatch(baseline: UserResourceQuotaOverrides, draft: ResourceQuotaDraft): UserResourceQuotaPatch {
    const patch: UserResourceQuotaPatch = {};
    for (const key of new Set([...Object.keys(baseline), ...Object.keys(draft)] as Array<keyof RuntimeResourcePolicy>)) {
        if (baseline[key] !== draft[key]) patch[key] = draft[key] ?? null;
    }
    return patch;
}
