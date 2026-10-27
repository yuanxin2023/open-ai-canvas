import { expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { prepareAgentSkillPreset, confirmAgentSkillPreset } from "@/services/agent-skill-presets";
import { AgentPresetReviewContents } from "@/components/canvas/canvas-agent-preset-confirmation";
import type { Skill, SkillPreset } from "@/services/api/skills";

const preset = { presetId: "test", name: "测试配方", skillIds: ["first", "second"] } as SkillPreset;
function fixture() {
    const skills = new Map(preset.skillIds.map((id) => [id, { skillId: id, skillName: `技能 ${id}`, status: 1, isAdded: false, versionId: `v-${id}`, contentHash: "a".repeat(64) } as Skill]));
    const added: string[] = [];
    let current = true;
    const api = {
        getSkill: async (id: string) => ({ skill: { ...skills.get(id)! } }),
        addSkill: async (id: string) => {
            added.push(id);
            skills.get(id)!.isAdded = true;
            return { skill: { ...skills.get(id)! } };
        },
        listAddedSkills: async () => ({ skills: [...skills.values()].filter((skill) => skill.isAdded) }),
    };
    return {
        skills,
        added,
        api,
        isCurrent: () => current,
        switchAccount: () => {
            current = false;
        },
    };
}

test("preparation and review list names and missing installs without writing", async () => {
    const f = fixture();
    f.skills.get("first")!.isAdded = true;
    const preview = await prepareAgentSkillPreset(preset, f.isCurrent, f.api);
    const html = renderToStaticMarkup(<AgentPresetReviewContents preview={preview} />);
    expect(html).toContain("技能 first");
    expect(html).toContain("技能 second");
    expect(html).toContain("已在技能库");
    expect(html).toContain("待安装");
    expect(html).toContain("生成仍需单独提交");
    expect(f.added).toEqual([]);
});

test("a disabled item discovered at confirmation prevents every install", async () => {
    const f = fixture();
    const preview = await prepareAgentSkillPreset(preset, f.isCurrent, f.api);
    f.api.getSkill = async (id) => {
        if (id === "second") throw new Error("平台技能已停用");
        return { skill: { ...f.skills.get(id)! } };
    };
    await expect(confirmAgentSkillPreset(preview, f.isCurrent, f.api)).rejects.toThrow("停用");
    expect(f.added).toEqual([]);
});

test("already installed skills are revalidated and updated content needs new review", async () => {
    const f = fixture();
    f.skills.get("first")!.isAdded = true;
    const preview = await prepareAgentSkillPreset(preset, f.isCurrent, f.api);
    f.skills.get("first")!.versionId = "updated-version";
    await expect(confirmAgentSkillPreset(preview, f.isCurrent, f.api)).rejects.toThrow("内容已更新");
    expect(f.added).toEqual([]);
});

test("confirmation installs only missing items and requires installed version receipts", async () => {
    const f = fixture();
    f.skills.get("first")!.isAdded = true;
    const preview = await prepareAgentSkillPreset(preset, f.isCurrent, f.api);
    const result = await confirmAgentSkillPreset(preview, f.isCurrent, f.api);
    expect(f.added).toEqual(["second"]);
    expect(result.installedCount).toBe(1);
    expect(result.skills.map((skill) => skill.skillId)).toEqual(preset.skillIds);
});

test("partial install failure keeps completed installs and never returns an activated recipe", async () => {
    const f = fixture();
    const preview = await prepareAgentSkillPreset(preset, f.isCurrent, f.api);
    const install = f.api.addSkill;
    f.api.addSkill = async (id) => {
        if (id === "second") throw new Error("installation failed");
        return install(id);
    };
    await expect(confirmAgentSkillPreset(preview, f.isCurrent, f.api)).rejects.toThrow("installation failed");
    expect(f.added).toEqual(["first"]);
    expect(f.skills.get("first")!.isAdded).toBe(true);
});

test("account or conversation changes stop remaining writes", async () => {
    const f = fixture();
    const preview = await prepareAgentSkillPreset(preset, f.isCurrent, f.api);
    const install = f.api.addSkill;
    f.api.addSkill = async (id) => {
        const result = await install(id);
        f.switchAccount();
        return result;
    };
    await expect(confirmAgentSkillPreset(preview, f.isCurrent, f.api)).rejects.toThrow("账号或会话");
    expect(f.added).toEqual(["first"]);
});

test("a changed install response or missing final receipt cannot claim success", async () => {
    const f = fixture();
    const preview = await prepareAgentSkillPreset(preset, f.isCurrent, f.api);
    f.api.listAddedSkills = async () => ({ skills: [] });
    await expect(confirmAgentSkillPreset(preview, f.isCurrent, f.api)).rejects.toThrow("尚未确认全部");
    const other = fixture();
    const before = await prepareAgentSkillPreset(preset, other.isCurrent, other.api);
    other.api.addSkill = async (id) => ({ skill: { ...other.skills.get(id)!, isAdded: true, versionId: "changed" } });
    await expect(confirmAgentSkillPreset(before, other.isCurrent, other.api)).rejects.toThrow("版本或状态已变化");
});

test("invalid preset identities and incomplete backend versions fail before installation", async () => {
    const f = fixture();
    await expect(prepareAgentSkillPreset({ ...preset, skillIds: ["first", "first"] }, f.isCurrent, f.api)).rejects.toThrow("清单无效");
    f.skills.get("first")!.contentHash = "";
    await expect(prepareAgentSkillPreset(preset, f.isCurrent, f.api)).rejects.toThrow("版本不完整");
    expect(f.added).toEqual([]);
});
