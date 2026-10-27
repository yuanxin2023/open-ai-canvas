import { expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { migrateLegacyPrevisDocument, PrevisMigrationError } from "@/lib/canvas/previs/previs-legacy-document";
import { parseCanvasStorageDocument, rebaseCanvasProjects, serializeCanvasStorageDocument } from "@/lib/canvas/canvas-storage-revision";
import { groundPrevisObject, dropPrevisObjectToGround } from "@/lib/canvas/previs/previs-grounding";
import { isPrevisStandardActor } from "@/lib/canvas/previs/previs-mannequin";
import { resolvePrevisActorColor } from "@/lib/canvas/previs/previs-scene";
import { resolvePrevisShotFov, previsVerticalFov, resolvePrevisOutputSize } from "@/lib/canvas/previs/previs-optics";
import type { CanvasProject } from "@/stores/canvas/use-canvas-store";

const raw = readFileSync(new URL("../../backend/internal/canvas/previslegacy/testdata/director.json", import.meta.url), "utf8");
const fixture = (): Record<string, any> => JSON.parse(raw);

test("旧场景精确转换字段，保护身份/资源/骨骼/关键帧和第三方内容", () => {
    const source = fixture();
    const original = structuredClone(source);
    const migrated = migrateLegacyPrevisDocument(source);
    expect(source).toEqual(original);
    expect(migrated).not.toHaveProperty("directorScenes");
    const scene = migrated.previsScenes[0];
    expect(scene).toEqual({ ...original.directorScenes[0], objects: original.directorScenes[0].objects.map((object: object) => ({ ...object, legacyDirector: true })), shots: [{ ...original.directorScenes[0].shots[0], aspectRatio: "16:9" }] });
    expect(migrated.nodes[0].metadata).toEqual({
        workflowKind: "shot",
        previsSceneId: "scene-1",
        previsShotId: "shot-1",
        previsPreviewNodeId: "preview-node",
        previsDepthNodeId: "depth-node",
        previsNormalNodeId: "normal-node",
        previsClayVideoNodeId: "clay-node",
        thirdParty: { directorSceneId: "plugin-data" },
    });
    expect(migrated.custom).toEqual(original.custom);
    expect(migrateLegacyPrevisDocument(migrated)).toBe(migrated);
});

test("旧对象不会在打开/提交时被自动贴地、改色或换成新人偶，用户仍可明确贴地", () => {
    const actor = migrateLegacyPrevisDocument(fixture()).previsScenes[0].objects[0];
    expect(groundPrevisObject(actor)).toBe(actor);
    expect(isPrevisStandardActor(actor)).toBe(false);
    expect(resolvePrevisActorColor(actor.color, actor.legacyDirector)).toBe("#f1f3f5");
    const dropped = dropPrevisObjectToGround(actor);
    expect(dropped).not.toBe(actor);
    expect(actor.transform.position[1]).toBe(2);
    expect(actor.keyframes[0].transform.position[1]).toBe(3);
    expect(dropped.boneTracks).toEqual(actor.boneTracks);
});

test("按 CAN-08a 使用横屏与新版光学，不保留旧竖直视角", () => {
    const scene = migrateLegacyPrevisDocument(fixture()).previsScenes[0];
    expect(scene.shots[0].aspectRatio).toBe("16:9");
    expect(resolvePrevisShotFov(scene.cameras[0], scene.shots[0])).toBeCloseTo(previsVerticalFov(35, 16 / 9));
    expect(resolvePrevisShotFov(scene.cameras[0], scene.shots[0])).not.toBe(50);
    expect(resolvePrevisOutputSize(16 / 9)).toEqual({ width: 1280, height: 720 });
});

test("相同双字段允许收敛，新版文档保持原引用", () => {
    const source = fixture();
    source.previsScenes = structuredClone(source.directorScenes);
    expect(migrateLegacyPrevisDocument(source).previsScenes[0].id).toBe("scene-1");
    const native = { id: "new", nodes: [], previsScenes: [] };
    expect(migrateLegacyPrevisDocument(native)).toBe(native);
});

const invalidCases: Record<string, (source: Record<string, any>) => void> = {
    双字段不一致: (source) => {
        source.previsScenes = [];
    },
    节点别名冲突: (source) => {
        source.nodes[0].metadata.previsSceneId = "different";
    },
    未知场景版本: (source) => {
        source.directorScenes[0].version = 2;
    },
    相机缺失: (source) => {
        source.directorScenes[0].cameras = [];
    },
    输出引用断开: (source) => {
        source.nodes = source.nodes.slice(0, 1);
    },
    场景引用断开: (source) => {
        source.directorScenes = [];
    },
    身份重复: (source) => {
        source.directorScenes.push(structuredClone(source.directorScenes[0]));
    },
    空节点列表不是数组: (source) => {
        source.nodes = null;
    },
    引用不是字符串: (source) => {
        source.nodes[0].metadata.directorShotId = 12;
    },
    迁入标记已有其他含义: (source) => {
        source.directorScenes[0].objects[0].legacyDirector = false;
    },
};
for (const [name, mutate] of Object.entries(invalidCases)) {
    test(`无法可靠转换时停止，保留原输入：${name}`, () => {
        const source = fixture();
        mutate(source);
        const original = structuredClone(source);
        expect(() => migrateLegacyPrevisDocument(source)).toThrow(PrevisMigrationError);
        expect(source).toEqual(original);
    });
}

test("旧缓存与旧 journal 基线先转换再合并，不把字段改名当成另一次用户编辑", () => {
    const source = fixture() as CanvasProject;
    const document = parseCanvasStorageDocument(JSON.stringify({ state: { projects: [source] }, version: 0 }));
    const next = { ...source, title: "本机改名" };
    const result = rebaseCanvasProjects({ document, baseProjects: [source], localProjects: [next], baseRevision: 0 });
    expect(result.conflicts).toEqual([]);
    const saved = parseCanvasStorageDocument(serializeCanvasStorageDocument(result.document));
    expect(saved.state.projects[0].title).toBe("本机改名");
    expect(saved.state.projects[0].previsScenes[0].id).toBe("scene-1");
    expect(saved.state.projects[0]).not.toHaveProperty("directorScenes");
});
