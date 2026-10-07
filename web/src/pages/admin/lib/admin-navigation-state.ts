export const ADMIN_NAVIGATION_GROUPS_STORAGE_KEY = "infinite-canvas:admin-navigation-groups";

export const DEFAULT_ADMIN_NAVIGATION_GROUP_STATE = {
    analytics: true,
    platform: true,
    users: true,
    commerce: true,
    finance: true,
    content: true,
    settings: true,
    storage: true,
};

export type AdminNavigationGroupId = keyof typeof DEFAULT_ADMIN_NAVIGATION_GROUP_STATE;
export type AdminNavigationGroupState = Record<AdminNavigationGroupId, boolean>;

type AdminNavigationStorage = Pick<Storage, "getItem" | "setItem">;

function adminNavigationStorage(storage?: AdminNavigationStorage) {
    if (storage) return storage;
    if (typeof window === "undefined") return undefined;
    try {
        return window.localStorage;
    } catch {
        return undefined;
    }
}

export function readAdminNavigationGroupState(storage?: AdminNavigationStorage): AdminNavigationGroupState {
    try {
        const stored = adminNavigationStorage(storage)?.getItem(ADMIN_NAVIGATION_GROUPS_STORAGE_KEY);
        if (!stored) return { ...DEFAULT_ADMIN_NAVIGATION_GROUP_STATE };

        const parsed = JSON.parse(stored) as Record<string, unknown>;
        return Object.fromEntries(Object.entries(DEFAULT_ADMIN_NAVIGATION_GROUP_STATE).map(([groupId, defaultOpen]) => [groupId, typeof parsed[groupId] === "boolean" ? parsed[groupId] : defaultOpen])) as AdminNavigationGroupState;
    } catch {
        return { ...DEFAULT_ADMIN_NAVIGATION_GROUP_STATE };
    }
}

export function writeAdminNavigationGroupState(state: AdminNavigationGroupState, storage?: AdminNavigationStorage) {
    try {
        adminNavigationStorage(storage)?.setItem(ADMIN_NAVIGATION_GROUPS_STORAGE_KEY, JSON.stringify(state));
    } catch {
        // localStorage 不可用时保留当前内存状态，不能阻断导航交互。
    }
}
