import type { ReferralPolicy } from "@/services/api/referrals";

export function referralCredits(baseMicrocredits: number, rateBps: number): number {
    if (!Number.isSafeInteger(baseMicrocredits) || !Number.isInteger(rateBps) || baseMicrocredits < 0 || rateBps < 0 || rateBps > 10000) return 0;
    const step = BigInt(10000);
    return Number(BigInt(baseMicrocredits) * BigInt(rateBps) / step / step * step);
}

export function referralPolicySummary(policy: ReferralPolicy): string {
    let from = policy.mode === "time" ? 0 : 1;
    const parts = policy.stages.map((stage) => {
        const range = policy.mode === "time"
            ? `${from}—${stage.until || "不限"}天`
            : `第${from}—${stage.until || "不限"}单`;
        from = policy.mode === "time" ? stage.until : stage.until + 1;
        return `${range} ${stage.rateBps / 100}%`;
    });
    if (policy.maxDays) parts.push(`有效期${policy.maxDays}天`);
    if (policy.maxOrders) parts.push(`限前${policy.maxOrders}单`);
    if (policy.perInviteeCapMicrocredits) parts.push(`每位好友最多${policy.perInviteeCapMicrocredits / 1_000_000}积分`);
    return parts.join("；");
}

export const defaultReferralPolicy: ReferralPolicy = {
    versionId: "", enabled: false, mode: "orders", stages: [{ until: 3, rateBps: 2000 }, { until: 0, rateBps: 500 }],
    maxDays: 0, maxOrders: 0, perInviteeCapMicrocredits: 300_000_000,
    newcomerEnabled: false, newcomerRateBps: 500, newcomerCapMicrocredits: 0,
};
