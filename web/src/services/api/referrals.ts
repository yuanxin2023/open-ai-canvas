import { http } from "@/services/api/request";

export type ReferralPolicy = {
    enabled: boolean;
    versionId: string;
    mode: "time" | "orders";
    stages: Array<{ until: number; rateBps: number }>;
    maxDays: number;
    maxOrders: number;
    perInviteeCapMicrocredits: number;
    newcomerEnabled: boolean;
    newcomerRateBps: number;
    newcomerCapMicrocredits: number;
};

export type ReferralReward = {
    id: string;
    paymentOrderId: string;
    inviterId: string;
    inviteeId: string;
    amountFen: number;
    policyVersionId?: string;
    baseMicrocredits: number;
    orderOrdinal: number;
    stageIndex: number;
    uncappedMicrocredits: number;
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
    policy: ReferralPolicy;
    boundPolicy?: ReferralPolicy;
    newcomerEligible: boolean;
    inviteeCount: number;
    invitees: Array<{ userId: string; email: string; joinedAt: string; policy: ReferralPolicy }>;
    rewards: ReferralReward[];
};

export type ReferralRewardPage = { rewards: ReferralReward[]; total: number; page: number; pageSize: number };

export const getReferralDashboard = (signal?: AbortSignal) => http.get<ReferralDashboard>("/referrals/me", { signal });
export const getAdminReferralPolicy = () => http.get<ReferralPolicy>("/admin/referrals/policy");
export const updateAdminReferralPolicy = (policy: ReferralPolicy) => http.put<ReferralPolicy>("/admin/referrals/policy", policy);
export type ReferralUserPolicy = { userId: string; code: string; paused: boolean; policy: ReferralPolicy | null };
export const setAdminReferralUserPolicy = (userId: string, policy: ReferralPolicy | null, paused: boolean) => http.put<ReferralUserPolicy>(`/admin/referrals/users/${encodeURIComponent(userId)}/policy`, { policy, paused });
export const getAdminReferralUser = (userId: string) => http.get<ReferralUserPolicy>(`/admin/referrals/users/${encodeURIComponent(userId)}`);
export const listAdminReferralRewards = (status: ReferralReward["status"] | "all", page: number, pageSize: number) => http.get<ReferralRewardPage>("/admin/referrals/rewards", { params: { status, page, pageSize } });
export const reviewAdminReferralReward = (id: string, approve: boolean, note: string) => http.post<ReferralReward>(`/admin/referrals/rewards/${encodeURIComponent(id)}/${approve ? "approve" : "reject"}`, { note });
