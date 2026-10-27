export type CanvasAppearance = {
    agentName: string;
    launcherLabel: string;
    panelTitle: string;
    welcomeTitle: string;
    welcomeDescription: string;
    inputPlaceholder: string;
    avatarType: "orb" | "live2d" | "bundle";
    live2dResourceId: string;
    live2dEntry: string;
    avatarHeight: number;
};

export const DEFAULT_CANVAS_APPEARANCE: CanvasAppearance = {
    agentName: "创作助手",
    launcherLabel: "Agent",
    panelTitle: "画布助手",
    welcomeTitle: "在这里，和{agentName}让灵感，慢慢成形",
    welcomeDescription: "从一个想法开始，和{agentName}一起创作。",
    inputPlaceholder: "输入操作指导；用 @ 引用画布节点，用 / 或 、 引用 Skills",
    avatarType: "bundle",
    live2dResourceId: "",
    live2dEntry: "",
    avatarHeight: 220,
};

export type AgentAvatarState = "idle" | "queued" | "running" | "waiting_approval" | "completed" | "failed" | "cancelled" | "rejected";

export type AgentAvatarBundleManifest = {
    version: 1;
    width: number;
    height: number;
    clips: Record<string, string>;
};

const AGENT_AVATAR_CLIPS: Record<AgentAvatarState, string> = {
    idle: "idle",
    queued: "sleep_loop",
    running: "working_loop",
    waiting_approval: "think",
    completed: "wave",
    failed: "wipe_sweat",
    cancelled: "lie_down",
    rejected: "think",
};

export function isAgentAvatarBundleManifest(value: unknown): value is AgentAvatarBundleManifest {
    if (typeof value !== "object" || value === null) return false;
    const manifest = value as Record<string, unknown>;
    if (manifest.version !== 1 || !Number.isInteger(manifest.width) || !Number.isInteger(manifest.height)) return false;
    const width = Number(manifest.width);
    const height = Number(manifest.height);
    if (width < 1 || width > 4096 || height < 1 || height > 4096 || typeof manifest.clips !== "object" || manifest.clips === null || Array.isArray(manifest.clips)) return false;
    const clips = manifest.clips as Record<string, unknown>;
    return Object.keys(clips).length > 0 && Object.keys(clips).length <= 64 && Object.entries(clips).every(([id, file]) => /^[A-Za-z0-9_-]{1,64}$/.test(id) && typeof file === "string" && /^[A-Za-z0-9_-][A-Za-z0-9._-]{0,123}\.mp4$/i.test(file));
}

export function agentAvatarClipForState(status: string, manifest: AgentAvatarBundleManifest) {
    const state = Object.hasOwn(AGENT_AVATAR_CLIPS, status) ? (status as AgentAvatarState) : "idle";
    return manifest.clips[AGENT_AVATAR_CLIPS[state]] || manifest.clips.idle || Object.values(manifest.clips)[0] || "";
}

export function builtInAgentAvatarURL(file: string) {
    return `${import.meta.env?.BASE_URL || "/"}doll/${encodeURIComponent(file)}`;
}

export function agentCopy(template: string, name: string) {
    return template.replaceAll("{agentName}", () => name);
}
