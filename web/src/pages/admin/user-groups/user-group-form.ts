import type { UserGroup, UserGroupBadgeStyle, UserGroupInput } from "@/services/api/user-groups";

export type UserGroupFormValues = {
    code: string;
    name: string;
    description?: string;
    badgeLabel?: string;
    badgeStyle: UserGroupBadgeStyle;
    sortOrder: number;
    allChannels: boolean;
    channelIds: string[];
};

export function emptyUserGroupForm(): UserGroupFormValues {
    return { code: "", name: "", description: "", badgeLabel: "", badgeStyle: "none", sortOrder: 0, allChannels: false, channelIds: [] };
}

export function userGroupFormValues(group: UserGroup): UserGroupFormValues {
    return {
        code: group.code,
        name: group.name,
        description: group.description,
        badgeLabel: group.badgeLabel,
        badgeStyle: group.badgeStyle,
        sortOrder: group.sortOrder,
        allChannels: group.allChannels,
        channelIds: group.channelIds,
    };
}

export function userGroupInput(values: UserGroupFormValues): UserGroupInput {
    return {
        code: values.code.trim().toLowerCase(),
        name: values.name.trim(),
        description: values.description?.trim() || "",
        badgeLabel: values.badgeLabel?.trim() || "",
        badgeStyle: values.badgeStyle,
        sortOrder: values.sortOrder || 0,
        allChannels: values.allChannels,
        channelIds: values.allChannels ? [] : values.channelIds,
    };
}
