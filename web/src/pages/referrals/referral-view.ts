import type { ReferralPolicy, ReferralReward } from "@/services/api/referrals";
import { referralCredits } from "@/lib/referral-policy";

export function referralRewardTotals(rewards: ReferralReward[]) {
    return rewards.reduce(
        (totals, reward) => {
            if (reward.status === "approved") totals.approved += reward.rewardMicrocredits;
            if (reward.status === "pending") totals.pending += reward.rewardMicrocredits;
            return totals;
        },
        { approved: 0, pending: 0 },
    );
}

export function referralStageLabel(policy: ReferralPolicy, index: number): string {
    const end = policy.stages[index].until;
    const previous = index === 0 ? 0 : policy.stages[index - 1].until;
    if (policy.mode === "time") {
        if (index === 0) return end ? `首次充值后前 ${end} 天` : "首次充值后持续有效";
        return end ? `首次充值后 ${previous}—${end} 天` : `首次充值满 ${previous} 天起`;
    }
    const start = previous + 1;
    if (start === 1) return end === 1 ? "好友首笔充值" : end ? `好友前 ${end} 笔充值` : "好友每笔充值";
    return end === start ? `好友第 ${start} 笔充值` : end ? `好友第 ${start}—${end} 笔充值` : `好友第 ${start} 笔充值起`;
}

export function referralFirstRechargeExample(policy: ReferralPolicy, baseMicrocredits: number) {
    const inviter = referralCredits(baseMicrocredits, policy.stages[0].rateBps);
    const newcomer = policy.newcomerEnabled ? referralCredits(baseMicrocredits, policy.newcomerRateBps) : 0;
    return {
        inviter: Math.min(inviter, policy.perInviteeCapMicrocredits || Number.MAX_SAFE_INTEGER),
        newcomer: Math.min(newcomer, policy.newcomerCapMicrocredits || Number.MAX_SAFE_INTEGER),
    };
}
