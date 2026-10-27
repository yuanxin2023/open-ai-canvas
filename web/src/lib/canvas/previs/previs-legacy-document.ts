import { canonicalize } from "json-canonicalize";

const metadataFields = {
    directorSceneId: "previsSceneId",
    directorShotId: "previsShotId",
    directorPreviewNodeId: "previsPreviewNodeId",
    directorDepthNodeId: "previsDepthNodeId",
    directorNormalNodeId: "previsNormalNodeId",
    directorClayVideoNodeId: "previsClayVideoNodeId",
} as const;

type RecordValue = Record<string, unknown>;
const record = (value: unknown): value is RecordValue => Boolean(value && typeof value === "object" && !Array.isArray(value));
const owns = (value: object, key: string) => Object.prototype.hasOwnProperty.call(value, key);

export class PrevisMigrationError extends Error {
    readonly reason = "canvas_previs_migration_conflict";
    constructor(readonly path: string) {
        super(`旧导演台数据无法可靠转换（${path}），已停止写入，请保留原文件后核对`);
        this.name = "PrevisMigrationError";
    }
}

function objects(value: unknown, path: string): RecordValue[] {
    if (!Array.isArray(value)) throw new PrevisMigrationError(path);
    const ids = new Set<string>();
    return value.map((item) => {
        if (!record(item) || typeof item.id !== "string" || !item.id || ids.has(item.id)) throw new PrevisMigrationError(path);
        ids.add(item.id);
        return item;
    });
}

function migrateScenes(value: unknown): RecordValue[] {
    return objects(value, "directorScenes").map((scene) => {
        if (scene.version !== 1) throw new PrevisMigrationError("directorScenes.version");
        const actors = objects(scene.objects, "directorScenes.objects");
        const cameras = objects(scene.cameras, "directorScenes.cameras");
        if (actors.some((actor) => owns(actor, "legacyDirector") && actor.legacyDirector !== true)) throw new PrevisMigrationError("directorScenes.objects.legacyDirector");
        objects(scene.lights, "directorScenes.lights");
        const shots = objects(scene.shots, "directorScenes.shots");
        const cameraIds = new Set(cameras.map((camera) => camera.id));
        if (shots.some((shot) => !cameraIds.has(shot.cameraId))) throw new PrevisMigrationError("directorScenes.shots.cameraId");
        if (scene.activeShotId && !shots.some((shot) => shot.id === scene.activeShotId)) throw new PrevisMigrationError("directorScenes.activeShotId");
        return {
            ...scene,
            objects: actors.map((actor) => ({ ...actor, legacyDirector: true })),
            shots: shots.map((shot) => ({ ...shot, aspectRatio: "16:9" })),
        };
    });
}

function migrateMetadata(value: RecordValue): RecordValue {
    let migrated = value;
    for (const [oldKey, key] of Object.entries(metadataFields)) {
        if (!owns(value, oldKey)) continue;
        if (typeof value[oldKey] !== "string") throw new PrevisMigrationError(`nodes.metadata.${oldKey}`);
        if (owns(value, key) && canonicalize(value[oldKey]) !== canonicalize(value[key])) throw new PrevisMigrationError(`nodes.metadata.${key}`);
        if (migrated === value) migrated = { ...value };
        migrated[key] = value[oldKey];
        delete migrated[oldKey];
    }
    return migrated;
}

function validateReferences(project: RecordValue) {
    const nodes = objects(project.nodes === undefined ? [] : project.nodes, "nodes");
    const scenes = objects(project.previsScenes === undefined ? [] : project.previsScenes, "previsScenes");
    const nodeIds = new Set(nodes.map((node) => node.id));
    const sceneById = new Map(scenes.map((scene) => [scene.id, scene]));
    const checkOutput = (value: unknown, path: string) => {
        if (value !== undefined && value !== "" && (typeof value !== "string" || !nodeIds.has(value))) throw new PrevisMigrationError(path);
    };
    for (const scene of scenes) {
        for (const shot of objects(scene.shots, "previsScenes.shots")) {
            for (const field of ["previewNodeId", "depthNodeId", "normalNodeId"]) checkOutput(shot[field], `previsScenes.shots.${field}`);
        }
    }
    for (const node of nodes) {
        const metadata = node.metadata;
        if (!record(metadata)) continue;
        for (const field of Object.values(metadataFields)) {
            if (owns(metadata, field) && typeof metadata[field] !== "string") throw new PrevisMigrationError(`nodes.metadata.${field}`);
        }
        if (metadata.previsSceneId) {
            const scene = sceneById.get(metadata.previsSceneId);
            if (!scene) throw new PrevisMigrationError("nodes.metadata.previsSceneId");
            if (metadata.previsShotId && !objects(scene.shots, "previsScenes.shots").some((shot) => shot.id === metadata.previsShotId)) throw new PrevisMigrationError("nodes.metadata.previsShotId");
        } else if (metadata.previsShotId) throw new PrevisMigrationError("nodes.metadata.previsShotId");
        for (const field of ["previsPreviewNodeId", "previsDepthNodeId", "previsNormalNodeId", "previsClayVideoNodeId"]) checkOutput(metadata[field], `nodes.metadata.${field}`);
    }
}

// Only persisted scene fields and node metadata are translated. Prompts, IDs,
// URLs and unknown document content are never rewritten by string replacement.
export function migrateLegacyPrevisDocument<T extends object>(project: T): T {
    if (!record(project)) throw new PrevisMigrationError("document");
    const source = project as RecordValue;
    let result = source;
    const nodes = Array.isArray(source.nodes)
        ? source.nodes.map((node: unknown) => {
              if (!record(node) || !record(node.metadata)) return node;
              const metadata = migrateMetadata(node.metadata);
              return metadata === node.metadata ? node : { ...node, metadata };
          })
        : source.nodes;
    if (Array.isArray(nodes) && nodes.some((node, index) => node !== (source.nodes as unknown[])[index])) result = { ...source, nodes };
    if (owns(source, "directorScenes")) {
        const migrated = migrateScenes(source.directorScenes);
        if (owns(source, "previsScenes") && canonicalize(source.previsScenes) !== canonicalize(source.directorScenes) && canonicalize(source.previsScenes) !== canonicalize(migrated)) throw new PrevisMigrationError("previsScenes");
        result = { ...result, previsScenes: migrated };
        delete result.directorScenes;
    }
    if (result !== source) validateReferences(result);
    return result as T;
}
