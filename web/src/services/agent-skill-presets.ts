import { addSkill, getSkill, listAddedSkills, type Skill, type SkillPreset } from "@/services/api/skills";

export type AgentSkillPresetPreview = { preset: SkillPreset; skills: Skill[] };
type PresetAPI = {
    getSkill: (id: string) => Promise<{ skill: Skill }>;
    addSkill: (id: string) => Promise<{ skill: Skill }>;
    listAddedSkills: () => Promise<{ skills: Skill[] }>;
};
const presetAPI: PresetAPI = { getSkill, addSkill, listAddedSkills };

function assertCurrent(isCurrent: () => boolean) {
    if (!isCurrent()) throw new DOMException("账号或会话已变化", "AbortError");
}

function validateReceipt(skill: Skill, id: string) {
    if (!skill || skill.skillId !== id || skill.status !== 1 || !skill.skillName?.trim() || !skill.versionId || !/^[a-f\d]{64}$/i.test(skill.contentHash)) {
        throw new Error("技能身份、状态或版本不完整，请刷新后核对");
    }
}

export async function prepareAgentSkillPreset(preset: SkillPreset, isCurrent: () => boolean, api: PresetAPI = presetAPI): Promise<AgentSkillPresetPreview> {
    assertCurrent(isCurrent);
    if (
        !preset.presetId?.trim() ||
        !preset.name?.trim() ||
        !Array.isArray(preset.skillIds) ||
        !preset.skillIds.length ||
        preset.skillIds.length > 8 ||
        new Set(preset.skillIds).size !== preset.skillIds.length ||
        preset.skillIds.some((id) => typeof id !== "string" || !id.trim() || id !== id.trim())
    ) {
        throw new Error("预设技能清单无效");
    }
    const skills = await Promise.all(
        preset.skillIds.map(async (id) => {
            const { skill } = await api.getSkill(id);
            assertCurrent(isCurrent);
            validateReceipt(skill, id);
            return skill;
        }),
    );
    return { preset: { ...preset, skillIds: [...preset.skillIds] }, skills };
}

export async function confirmAgentSkillPreset(preview: AgentSkillPresetPreview, isCurrent: () => boolean, api: PresetAPI = presetAPI) {
    // Re-read every item before the first write, including already installed
    // skills: an administrator may disable or update one while review is open.
    const current = await prepareAgentSkillPreset(preview.preset, isCurrent, api);
    for (const skill of current.skills) {
        const reviewed = preview.skills.find((item) => item.skillId === skill.skillId);
        if (!reviewed || reviewed.versionId !== skill.versionId || reviewed.contentHash !== skill.contentHash) {
            throw new Error("技能内容已更新，请重新打开预设检查后确认");
        }
    }
    let installedCount = 0;
    for (const skill of current.skills) {
        if (skill.isAdded) continue;
        assertCurrent(isCurrent);
        const { skill: installed } = await api.addSkill(skill.skillId);
        assertCurrent(isCurrent);
        validateReceipt(installed, skill.skillId);
        if (!installed.isAdded || installed.versionId !== skill.versionId || installed.contentHash !== skill.contentHash) {
            throw new Error("安装后技能版本或状态已变化，未选用整组预设");
        }
        installedCount++;
    }
    const result = await api.listAddedSkills();
    assertCurrent(isCurrent);
    if (current.skills.some((item) => !result.skills.some((skill) => skill.skillId === item.skillId && skill.isAdded && skill.versionId === item.versionId))) {
        throw new Error("技能库尚未确认全部预设技能及版本，未选用整组预设");
    }
    return { skills: result.skills, installedCount };
}
