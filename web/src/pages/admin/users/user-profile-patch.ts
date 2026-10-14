import type { AdminManagedUser } from "@/services/api/auth";

export type UserProfileValues = Pick<AdminManagedUser, "email" | "remark">;

export function buildUserProfilePatch(current: UserProfileValues, draft: UserProfileValues): Partial<UserProfileValues> {
    const patch: Partial<UserProfileValues> = {};
    const email = draft.email?.trim() || "";
    const remark = draft.remark?.trim() || "";
    if (email !== (current.email || "")) patch.email = email;
    if (remark !== (current.remark || "")) patch.remark = remark;
    return patch;
}
