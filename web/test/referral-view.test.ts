import { expect, test } from "bun:test";
import { defaultReferralPolicy } from "../src/lib/referral-policy";
import type { ReferralReward } from "../src/services/api/referrals";
import { referralFirstRechargeExample, referralRewardTotals, referralStageLabel } from "../src/pages/referrals/referral-view";

test("到账与待审核只统计返回记录的实际奖励，不计入拒绝金额或截断前奖励", () => {
    const records = [
        { status: "approved", rewardMicrocredits: 240_000_000, uncappedMicrocredits: 300_000_000 },
        { status: "approved", rewardMicrocredits: 20_000_000 },
        { status: "pending", rewardMicrocredits: 60_000_000 },
        { status: "rejected", rewardMicrocredits: 500_000_000 },
    ] as ReferralReward[];
    expect(referralRewardTotals(records)).toEqual({ approved: 260_000_000, pending: 60_000_000 });
    expect(referralRewardTotals([])).toEqual({ approved: 0, pending: 0 });
});

test("阶梯文案区分首笔、连续单数及连续天数", () => {
    const p = {
        ...defaultReferralPolicy,
        stages: [
            { until: 1, rateBps: 2000 },
            { until: 10, rateBps: 500 },
            { until: 0, rateBps: 0 },
        ],
    };
    expect(referralStageLabel(p, 0)).toBe("好友首笔充值");
    expect(referralStageLabel(p, 1)).toBe("好友第 2—10 笔充值");
    expect(referralStageLabel(p, 2)).toBe("好友第 11 笔充值起");
    const time = {
        ...p,
        mode: "time" as const,
        stages: [
            { until: 30, rateBps: 2000 },
            { until: 90, rateBps: 500 },
            { until: 0, rateBps: 0 },
        ],
    };
    expect(referralStageLabel(time, 0)).toBe("首次充值后前 30 天");
    expect(referralStageLabel(time, 1)).toBe("首次充值后 30—90 天");
    expect(referralStageLabel(time, 2)).toBe("首次充值满 90 天起");
});

test("首充试算分别应用邀请人上限和新人赠送上限", () => {
    const p = { ...defaultReferralPolicy, newcomerEnabled: true, perInviteeCapMicrocredits: 100_000_000, newcomerCapMicrocredits: 30_000_000 };
    expect(referralFirstRechargeExample(p, 1_200_000_000)).toEqual({ inviter: 100_000_000, newcomer: 30_000_000 });
    expect(referralFirstRechargeExample({ ...p, newcomerEnabled: false, perInviteeCapMicrocredits: 0 }, 1_200_000_000)).toEqual({ inviter: 240_000_000, newcomer: 0 });
});
