import { http } from "@/services/api/request";

export type ReferralPolicy = {
    enabled: boolean;
    globalRateBps: number;
    creditsPerYuanMicro: number;
    perInviteeCapMicrocredits: number;
};

export type ReferralReward = {
    id: string;
    paymentOrderId: string;
    inviterId: string;
    inviteeId: string;
    amountFen: number;
    creditsPerYuanMicro: number;
    rateBps: number;
    rewardMicrocredits: number;
    status: "pending" | "approved" | "rejected";
    reviewedBy?: string;
    reviewNote?: string;
    reviewedAt?: string;
    createdAt: string;
};

export type ReferralDashboard = {
    enabled: boolean;
    code: string;
    inviterId?: string;
    effectiveRateBps: number;
    inviteeCount: number;
    invitees: Array<{ userId: string; email: string; joinedAt: string }>;
    rewards: ReferralReward[];
};

export type ReferralRewardPage = { rewards: ReferralReward[]; total: number; page: number; pageSize: number };

export const getReferralDashboard = () => http.get<ReferralDashboard>("/referrals/me");
export const getAdminReferralPolicy = () => http.get<ReferralPolicy>("/admin/referrals/policy");
export const updateAdminReferralPolicy = (policy: ReferralPolicy) => http.put<ReferralPolicy>("/admin/referrals/policy", policy);
export const setAdminReferralRate = (userId: string, rateBps: number | null) => http.put<{ userId: string; code: string; rateBps?: number }>(`/admin/referrals/users/${encodeURIComponent(userId)}/rate`, { rateBps });
export const getAdminReferralUser = (userId: string) => http.get<{ userId: string; code: string; rateBps?: number }>(`/admin/referrals/users/${encodeURIComponent(userId)}`);
export const listAdminReferralRewards = (status: ReferralReward["status"] | "all", page: number, pageSize: number) => http.get<ReferralRewardPage>("/admin/referrals/rewards", { params: { status, page, pageSize } });
export const reviewAdminReferralReward = (id: string, approve: boolean, note: string) => http.post<ReferralReward>(`/admin/referrals/rewards/${encodeURIComponent(id)}/${approve ? "approve" : "reject"}`, { note });
