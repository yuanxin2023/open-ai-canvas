import { useEffect, useMemo, useState } from "react";
import { Select } from "antd";

import { listAddedSkills, type Skill } from "@/services/api/skills";
import { SKILL_RUNTIME_PROFILES, type SkillRuntimeProfile } from "@/services/skill-runtime";
import { useUserStore } from "@/stores/use-user-store";

export function useSkillRuntimeCatalog() {
    const enabled = useUserStore((state) => state.features.skillLibraryEnabled);
    const [skills, setSkills] = useState<Skill[]>([]);
    const [loading, setLoading] = useState(enabled);

    useEffect(() => {
        if (!enabled) {
            setSkills([]);
            setLoading(false);
            return;
        }
        let cancelled = false;
        setLoading(true);
        listAddedSkills()
            .then((result) => {
                if (!cancelled) setSkills(result.skills.filter((skill) => skill.isAdded));
            })
            .catch(() => {
                if (!cancelled) setSkills([]);
            })
            .finally(() => {
                if (!cancelled) setLoading(false);
            });
        return () => {
            cancelled = true;
        };
    }, [enabled]);

    return { skills, loading, enabled };
}

export function SkillRuntimePicker({ skills, loading, value, onChange, placeholder = "选择本次生成使用的技能", profile = "canvas" }: { skills: Skill[]; loading?: boolean; value: string[]; onChange: (skillIds: string[]) => void; placeholder?: string; profile?: SkillRuntimeProfile }) {
    const options = useMemo(
        () => skills.map((skill) => ({ value: skill.skillId, label: skill.skillName, title: skill.description })),
        [skills],
    );
    const maxSkills = SKILL_RUNTIME_PROFILES[profile].maxSkills;

    return (
        <Select
            className="w-full"
            style={{ width: "100%" }}
            mode="multiple"
            allowClear
            showSearch
            maxCount={maxSkills || undefined}
            maxTagCount="responsive"
            loading={loading}
            value={value}
            placeholder={placeholder}
            optionFilterProp="label"
            options={options}
            onChange={onChange}
            notFoundContent={loading ? "正在读取技能库" : "暂无已加入的技能"}
            aria-label="本次生成使用的技能"
        />
    );
}
