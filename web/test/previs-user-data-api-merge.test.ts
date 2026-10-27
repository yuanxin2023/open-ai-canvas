import { afterEach, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { apiClient, ApiError } from "@/services/api/request";
import { getRemoteCanvasProject, getRemoteUserDataSnapshot, applyRemoteCanvasOperations, type CanvasOperationsRequest } from "@/services/api/user-data";
import { migrateLegacyPrevisDocument, PrevisMigrationError } from "@/lib/canvas/previs/previs-legacy-document";
import type { CanvasProject } from "@/stores/canvas/use-canvas-store";

const fixture = () => JSON.parse(readFileSync(new URL("../../backend/internal/canvas/previslegacy/testdata/director.json", import.meta.url), "utf8")) as CanvasProject;
const original = apiClient.request;
afterEach(() => {
    apiClient.request = original;
});

test("画布 GET、完整快照和 304 缓存返回统一采用新版字段", async () => {
    const project = fixture();
    apiClient.request = (async () => ({ status: 200, data: { code: 0, data: { project, projects: [project], assets: [] }, msg: "ok" } })) as typeof apiClient.request;
    expect((await getRemoteCanvasProject(project.id)).project).toEqual(migrateLegacyPrevisDocument(project));
    expect((await getRemoteUserDataSnapshot()).projects[0].previsScenes[0].id).toBe("scene-1");
    apiClient.request = (async () => ({ status: 304, data: undefined })) as typeof apiClient.request;
    const cached = await getRemoteCanvasProject(project.id, project);
    expect(cached.notModified).toBe(true);
    expect(cached.project.previsScenes[0].shots[0].aspectRatio).toBe("16:9");
    expect(project).toHaveProperty("directorScenes");
});

test("快照里任何旧字段冲突都向上报告，不返回默认空画布", async () => {
    const project = fixture();
    project.previsScenes = [];
    apiClient.request = (async () => ({ status: 200, data: { code: 0, data: { projects: [project], assets: [] }, msg: "ok" } })) as typeof apiClient.request;
    await expect(getRemoteUserDataSnapshot()).rejects.toBeInstanceOf(PrevisMigrationError);
});

test("后端冲突原因保留，已入队的原操作请求与幂等身份不被 API 层改写", async () => {
    const request = JSON.parse('{"mutationId":"original-retry","operations":[{"type":"set_field","field":"directorScenes","value":[],"beforeValue":[]}]}') as CanvasOperationsRequest;
    let sent: unknown;
    apiClient.request = (async (config) => {
        sent = config.data;
        return { status: 409, data: { code: 409, data: null, reason: "canvas_previs_migration_conflict", msg: "requires review" } };
    }) as typeof apiClient.request;
    try {
        await applyRemoteCanvasOperations("legacy-canvas", request);
        throw new Error("conflict swallowed");
    } catch (error) {
        expect(error).toBeInstanceOf(ApiError);
        expect((error as ApiError).reason).toBe("canvas_previs_migration_conflict");
    }
    expect(sent).toBe(request);
    expect(JSON.stringify(sent)).toBe('{"mutationId":"original-retry","operations":[{"type":"set_field","field":"directorScenes","value":[],"beforeValue":[]}]}');
});
