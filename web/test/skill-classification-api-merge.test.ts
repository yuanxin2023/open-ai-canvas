import { afterEach, expect, test } from "bun:test";
import { ApiError, apiClient } from "@/services/api/request";
import { createSkillLibraryCategory, deleteSkillLibraryCategory, listSkillLibraryCategories, setSkillLibraryCategory } from "@/services/api/skills";
import { getSkillCuration, updateSkillCuration } from "@/services/api/skill-curation";

const original = apiClient.request;
afterEach(() => {
    apiClient.request = original;
});

test("classification writes preserve identities, explicit personal scope and empty category", async () => {
    const calls: { method?: string; url?: string; data?: unknown }[] = [];
    const data = { category: { id: "category", scope: "personal" }, skill: { skillId: "skill/id", libraryCategoryId: "" }, deleted: true };
    apiClient.request = (async (config) => {
        calls.push({ method: config.method, url: config.url, data: config.data });
        return { status: 200, data: { code: 0, data, msg: "ok" } };
    }) as typeof apiClient.request;
    expect((await createSkillLibraryCategory({ name: "My category", scope: "personal" })).category).toEqual(data.category);
    expect((await setSkillLibraryCategory("skill/id", "")).skill.libraryCategoryId).toBe("");
    await deleteSkillLibraryCategory("category/id");
    expect(calls).toEqual([
        { method: "post", url: "/skills/library-categories", data: { name: "My category", scope: "personal" } },
        { method: "patch", url: "/skills/skill%2Fid/library-category", data: { categoryId: "" } },
        { method: "delete", url: "/skills/library-categories/category%2Fid", data: undefined },
    ]);
});

test("curation updates send revision zero and retain business conflict errors", async () => {
    let sent: unknown;
    apiClient.request = (async (config) => {
        sent = config.data;
        return { status: 200, data: { code: 409, reason: "creation_conflict", data: null, msg: "changed" } };
    }) as typeof apiClient.request;
    try {
        await updateSkillCuration(0, { enabled: false });
        throw new Error("business conflict was swallowed");
    } catch (error) {
        expect(error).toBeInstanceOf(ApiError);
        expect((error as ApiError).code).toBe(409);
        expect((error as ApiError).reason).toBe("creation_conflict");
    }
    expect(sent).toEqual({ expectedRevision: 0, enabled: false });
});

test("classification reads carry cancellation and keep personal/admin routes distinct", async () => {
    const calls: { url?: string; signal?: AbortSignal; params?: unknown }[] = [];
    const signal = new AbortController().signal;
    apiClient.request = (async (config) => {
        calls.push({ url: config.url, signal: config.signal as AbortSignal, params: config.params });
        return { status: 200, data: { code: 0, data: {}, msg: "ok" } };
    }) as typeof apiClient.request;
    await listSkillLibraryCategories("created", signal);
    await getSkillCuration(false, signal);
    await getSkillCuration(true, signal);
    expect(calls).toEqual([
        { url: "/skills/library-categories", params: { scope: "created" }, signal },
        { url: "/skills/curation", params: undefined, signal },
        { url: "/admin/skill-curation", params: undefined, signal },
    ]);
});
