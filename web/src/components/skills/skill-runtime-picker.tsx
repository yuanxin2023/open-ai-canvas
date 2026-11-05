import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

import { listAddedSkills, type Skill } from "@/services/api/skills";
import { SKILL_RUNTIME_PROFILES, type SkillRuntimeProfile } from "@/services/skill-runtime";
import { useUserStore } from "@/stores/use-user-store";
import { Select } from "@/components/ui/base/select";

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

export function SkillRuntimePicker({
    skills,
    loading,
    value,
    onChange,
    placeholder,
    profile = "canvas",
}: {
    skills: Skill[];
    loading?: boolean;
    value: string[];
    onChange: (skillIds: string[]) => void;
    placeholder?: string;
    profile?: SkillRuntimeProfile;
}) {
    const { t } = useTranslation();
    const options = useMemo(() => skills.map((skill) => ({ value: skill.skillId, label: skill.skillName, title: skill.description })), [skills]);
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
            placeholder={placeholder ?? t("library.skills.runtime.placeholder")}
            optionFilterProp="label"
            options={options}
            onChange={onChange}
            notFoundContent={loading ? t("library.skills.runtime.loading") : t("library.skills.runtime.empty")}
            aria-label={t("library.skills.runtime.ariaLabel")}
        />
    );
}
