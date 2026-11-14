import { compactApiParams, http } from "@/services/api/request";

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

export type ReferralMetrics = {
    inviteeCount: number;
    paidInviteeCount: number;
    paymentOrderCount: number;
    paymentAmountFen: number;
};

export type ReferralStatistics = ReferralMetrics & { inviterCount: number };
export type ReferralUserStatistics = ReferralMetrics & { rank: number; userId: string; username: string; code: string };
export type ReferralStatisticsSort = keyof ReferralMetrics;
export type ReferralStatisticsQuery = { keyword?: string; sortBy: ReferralStatisticsSort; page: number; pageSize: number };
export type ReferralUserStatisticsPage = { users: ReferralUserStatistics[]; total: number; page: number; pageSize: number };
export type ReferralInviteeStatistics = { userId: string; username: string; joinedAt: string; paymentOrderCount: number; paymentAmountFen: number };
export type ReferralInviteeStatisticsPage = { inviter: ReferralUserStatistics; invitees: ReferralInviteeStatistics[]; total: number; page: number; pageSize: number };

export const getAdminReferralStatistics = (signal?: AbortSignal) => http.get<ReferralStatistics>("/admin/referrals/statistics", { signal });
export const listAdminReferralStatisticsUsers = (query: ReferralStatisticsQuery, signal?: AbortSignal) => http.get<ReferralUserStatisticsPage>("/admin/referrals/statistics/users", { params: compactApiParams(query), signal });
export const listAdminReferralInvitees = (userId: string, query: { page: number; pageSize: number }, signal?: AbortSignal) =>
    http.get<ReferralInviteeStatisticsPage>(`/admin/referrals/users/${encodeURIComponent(userId)}/invitees`, { params: compactApiParams(query), signal });
