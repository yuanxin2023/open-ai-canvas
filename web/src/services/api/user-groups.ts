import { http } from "@/services/api/request";

/** 用户端分组徽章外观；none 不展示。 */
export type UserGroupBadgeStyle = "none" | "vip" | "svip";

export type UserGroup = {
    id: string;
    code: string;
    name: string;
    description: string;
    badgeLabel: string;
    badgeStyle: UserGroupBadgeStyle;
    sortOrder: number;
    allChannels: boolean;
    isDefault: boolean;
    channelIds: string[];
    userCount: number;
    createdAt: string;
    updatedAt: string;
};

export type UserGroupInput = Pick<UserGroup, "code" | "name" | "description" | "badgeLabel" | "badgeStyle" | "sortOrder" | "allChannels" | "channelIds">;

/** 会话里的分组展示信息，不含渠道绑定。 */
export type PublicUserGroup = {
    code: string;
    name: string;
    badgeLabel: string;
    badgeStyle: UserGroupBadgeStyle;
};

export const DEFAULT_USER_GROUP_CODE = "default";

export function listAdminUserGroups() {
    return http.get<{ groups: UserGroup[] }>("/admin/user-groups");
}

export function createAdminUserGroup(input: UserGroupInput) {
    return http.post<{ group: UserGroup }>("/admin/user-groups", input);
}

export function updateAdminUserGroup(id: string, input: UserGroupInput) {
    return http.patch<{ group: UserGroup }>(`/admin/user-groups/${encodeURIComponent(id)}`, input);
}

export function deleteAdminUserGroup(id: string) {
    return http.delete<{ ok: boolean }>(`/admin/user-groups/${encodeURIComponent(id)}`);
}

/** 用户 user_group 为空时属于默认分组。 */
export function userGroupCode(value?: string) {
    return value?.trim() || DEFAULT_USER_GROUP_CODE;
}
