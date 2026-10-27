import { create } from "zustand";

import { DEFAULT_DRAWING_ENGINE, type CanvasDrawingEngineSetting } from "@/lib/canvas/canvas-drawing-engine";
import type { AdminAccess } from "@/lib/admin-permissions";

export type LocalUser = {
    id: string;
    username: string;
    email?: string;
    phone?: string;
    emailVerifiedAt?: string;
    phoneVerifiedAt?: string;
    displayName: string;
    profileName?: string;
    usernameChangePolicy: UsernameChangePolicy;
    avatarResourceId?: string;
    avatarUrl?: string;
    avatarMode?: "custom" | "default" | "";
    avatarPreset?: string;
    identityProvider?: string;
    identityId?: string;
    identityUsername?: string;
    role: "admin" | "user";
    adminAccess?: AdminAccess;
    status: "active" | "disabled";
    lastLoginAt?: string;
    createdAt?: string;
    updatedAt?: string;
};

export type UsernameChangePolicy = {
    customized: boolean;
    limit: number | null;
    used: number;
    remaining: number | null;
    windowDays: number;
    nextAvailableAt?: string;
};

export type RuntimeLimits = {
    activeTaskLimit: number;
    resourceUploadMB: number;
    recycleBinRetentionDays?: number;
};

export type FeatureAvailability = {
    welcomeEnabled: boolean;
    shortDramaEnabled: boolean;
    taskCenterEnabled: boolean;
    creditsEnabled: boolean;
    customChannelsEnabled: boolean;
    frontendModelsEnabled: boolean;
    skillLibraryEnabled: boolean;
    pluginCenterEnabled: boolean;
    systemPluginsVisibleToUsers: boolean;
    inspirationSourcesVisible: boolean;
    inspirationHallEnabled: boolean;
    inspirationGalleryEnabled: boolean;
    userFeedbackEnabled: boolean;
    imageWorkbenchEnabled: boolean;
    videoWorkbenchEnabled: boolean;
    configured?: boolean;
    updatedBy?: string;
    updatedAt?: string;
};

export const defaultFeatureAvailability: FeatureAvailability = {
    welcomeEnabled: true,
    shortDramaEnabled: true,
    taskCenterEnabled: true,
    creditsEnabled: true,
    customChannelsEnabled: true,
    frontendModelsEnabled: false,
    skillLibraryEnabled: true,
    pluginCenterEnabled: true,
    systemPluginsVisibleToUsers: true,
    inspirationSourcesVisible: true,
    inspirationHallEnabled: false,
    inspirationGalleryEnabled: false,
    userFeedbackEnabled: false,
    imageWorkbenchEnabled: false,
    videoWorkbenchEnabled: false,
};

type UserStore = {
    hydrated: boolean;
    user: LocalUser | null;
    runtimeLimits: RuntimeLimits;
    drawingEngine: CanvasDrawingEngineSetting;
    features: FeatureAvailability;
    setUser: (user: LocalUser | null) => void;
    setRuntimeLimits: (limits?: RuntimeLimits) => void;
    setDrawingEngine: (setting?: CanvasDrawingEngineSetting) => void;
    setFeatures: (features?: FeatureAvailability) => void;
    setHydrated: (hydrated: boolean) => void;
    clearSession: () => void;
};

export const useUserStore = create<UserStore>()((set) => ({
    hydrated: false,
    user: null,
    runtimeLimits: { activeTaskLimit: 5, resourceUploadMB: 50, recycleBinRetentionDays: 30 },
    drawingEngine: { defaultEngine: DEFAULT_DRAWING_ENGINE },
    features: defaultFeatureAvailability,
    setUser: (user) => set({ user }),
    setRuntimeLimits: (runtimeLimits) => set({ runtimeLimits: runtimeLimits || { activeTaskLimit: 5, resourceUploadMB: 50, recycleBinRetentionDays: 30 } }),
    setDrawingEngine: (drawingEngine) => set({ drawingEngine: drawingEngine || { defaultEngine: DEFAULT_DRAWING_ENGINE } }),
    setFeatures: (features) => set({ features: features ? { ...defaultFeatureAvailability, ...features } : defaultFeatureAvailability }),
    setHydrated: (hydrated) => set({ hydrated }),
    clearSession: () => set({ user: null, runtimeLimits: { activeTaskLimit: 5, resourceUploadMB: 50, recycleBinRetentionDays: 30 }, drawingEngine: { defaultEngine: DEFAULT_DRAWING_ENGINE }, features: defaultFeatureAvailability }),
}));
