import type { AdminManagedUser } from "@/services/api/auth";

export type UserProfileValues = Pick<AdminManagedUser, "email" | "remark" | "userGroup">;

export function buildUserProfilePatch(current: UserProfileValues, draft: UserProfileValues): Partial<UserProfileValues> {
    const patch: Partial<UserProfileValues> = {};
    const email = draft.email?.trim() || "";
    const remark = draft.remark?.trim() || "";
    if (email !== (current.email || "")) patch.email = email;
    if (remark !== (current.remark || "")) patch.remark = remark;
    if (draft.userGroup !== undefined) {
        const group = draft.userGroup.trim() || "default";
        if (group !== (current.userGroup?.trim() || "default")) patch.userGroup = group;
    }
    return patch;
}
