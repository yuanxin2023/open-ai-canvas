import { registerPlugin } from "../plugin-registry";
import { PLUGIN_API_VERSION, type RegisteredPlugin } from "../plugin-types";
import { usePluginStore } from "@/stores/use-plugin-store";
import { useUserStore } from "@/stores/use-user-store";

export const USER_FEEDBACK_PLUGIN_ID = "user-feedback";

export const userFeedbackPlugin: RegisteredPlugin = {
    manifest: {
        id: USER_FEEDBACK_PLUGIN_ID,
        name: "意见与问题反馈",
        version: "1.0.0",
        publishedAt: "2026-10-20",
        updatedAt: "2026-10-20",
        apiVersion: PLUGIN_API_VERSION,
        description: "在设置的我的反馈中提交问题与建议，支持图片、视频附件和站内答复；需要管理员开启反馈功能。",
        author: "影策社区",
        surfaces: ["hybrid"],
        permissions: ["canvas.read"],
        trusted: true,
        contributes: { usageObservers: ["user-feedback"] },
    },
};

registerPlugin(userFeedbackPlugin);

export function useUserFeedbackEnabled(): boolean {
    const featureEnabled = useUserStore((state) => state.features.userFeedbackEnabled);
    const pluginEnabled = usePluginStore((state) => {
        const pluginState = state.pluginStates[USER_FEEDBACK_PLUGIN_ID];
        if (pluginState && typeof pluginState.effectiveEnabled === "boolean") {
            return pluginState.effectiveEnabled;
        }
        const installation = state.installations.find((item) => item.manifest.id === USER_FEEDBACK_PLUGIN_ID);
        if (installation && typeof installation.enabled === "boolean") {
            return installation.enabled;
        }
        return true;
    });
    return featureEnabled && pluginEnabled;
}
