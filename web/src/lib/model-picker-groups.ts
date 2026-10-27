import { groupModelsByDisplayName, isDirectSystemModel, type DisplayModelGroup } from "@/lib/model-selection";
import { modelIcon, modelOptionName, PUBLIC_MODEL_CATALOG_ID, resolveModelChannel, type AiConfig } from "@/stores/use-config-store";

export type ModelPickerGroup = {
    key: string;
    label: string;
    icon: string;
    scope: string;
    kind: "product" | "channel";
    models: DisplayModelGroup[];
};

export function modelPickerGroupHasSubmenu(group: ModelPickerGroup) {
    return group.kind === "channel" || group.models.length > 1;
}

export { isDirectSystemModel } from "@/lib/model-selection";

export function modelChannelLabel(config: AiConfig, value: string) {
    const channel = resolveModelChannel(config, value);
    const cost = channel.modelCosts?.find((item) => item.model === modelOptionName(value));
    return cost?.channelLabel?.trim() || channel.name || "未命名渠道";
}

// 一级按已设置的模型展示名跨渠道聚合，空值归入未分类；二级保留独立调用与售价。
export function groupModelsForPicker(config: AiConfig, options: string[]): ModelPickerGroup[] {
    const groups = new Map<string, ModelPickerGroup>();
    for (const channel of config.channels) {
        const models = options.filter((value) => resolveModelChannel(config, value).id === channel.id);
        const directModels = models.filter((value) => isDirectSystemModel(config, value));
        for (const value of directModels) {
            const cost = channel.modelCosts?.find((item) => item.model === modelOptionName(value));
            const label = cost?.displayName?.trim() || "未分类";
            const key = JSON.stringify(["product", label]);
            let group = groups.get(key);
            if (!group) {
                group = { key, label, icon: modelIcon(config, value), scope: "平台服务", kind: "product", models: [] };
                groups.set(key, group);
            }
            group.models.push({ key: value, label: modelChannelLabel(config, value), models: [value] });
        }
        const otherModels = models.filter((value) => !isDirectSystemModel(config, value));
        if (otherModels.length) {
            const key = JSON.stringify(["channel", channel.id]);
            groups.set(key, {
                key,
                label: channel.name || "未命名渠道",
                icon: modelIcon(config, otherModels[0]),
                scope: channel.id === PUBLIC_MODEL_CATALOG_ID ? "" : "我的模型",
                kind: "channel",
                models: groupModelsByDisplayName(config, otherModels),
            });
        }
    }
    for (const group of groups.values()) {
        if (group.kind !== "product") continue;
        const identities = group.models.map((model) => `${resolveModelChannel(config, model.models[0]).id}\0${model.label}`);
        for (const model of group.models) {
            const identity = `${resolveModelChannel(config, model.models[0]).id}\0${model.label}`;
            if (identities.filter((value) => value === identity).length > 1) model.label += ` · ${modelOptionName(model.models[0])}`;
        }
    }
    return Array.from(groups.values());
}
