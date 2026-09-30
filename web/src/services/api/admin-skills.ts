import { http } from "@/services/api/request";

export type AdminSkillCategory = {
    value: string;
    label: string;
    available: boolean;
    totalCount: number;
    availableCount: number;
};

export type AdminSkillItem = {
    skillId: string;
    skillName: string;
    description: string;
    tag: string;
    authorName: string;
    version: string;
    sourceType: string;
    available: boolean;
    categoryAvailable: boolean;
    effectiveAvailable: boolean;
    addedCount: number;
    likeCount: number;
    updatedAt: string;
};

export type AdminSkillCatalog = {
    globalAvailable: boolean;
    categories: AdminSkillCategory[];
    skills: AdminSkillItem[];
};

export type AdminSkillInstallUploadInput = {
    file: File;
    sourceType: "markdown" | "zip";
    name?: string;
    description?: string;
    tag: string;
};

export type AdminSkillGitHubInstallInput = {
    url: string;
    ref?: string;
    subdir?: string;
    tag: string;
    autoUpdate: boolean;
};

export function fetchAdminSkills() {
    return http.get<AdminSkillCatalog>("/admin/skills");
}

export function installAdminSkillUpload(input: AdminSkillInstallUploadInput) {
    const form = new FormData();
    form.append("file", input.file);
    form.append("sourceType", input.sourceType);
    form.append("tag", input.tag);
    if (input.name) form.append("name", input.name);
    if (input.description) form.append("description", input.description);
    return http.post<AdminSkillCatalog>("/admin/skills/install", form);
}

export function installAdminGitHubSkill(input: AdminSkillGitHubInstallInput) {
    return http.post<AdminSkillCatalog>("/admin/skills/install/github", input);
}

export function setAdminSkillAvailability(skillIds: string[], available: boolean) {
    return http.put<AdminSkillCatalog>("/admin/skills/availability", { skillIds, available });
}

export function setAdminSkillCategoryAvailability(tag: string, available: boolean) {
    return http.put<AdminSkillCatalog>(`/admin/skills/categories/${encodeURIComponent(tag)}/availability`, { available });
}
