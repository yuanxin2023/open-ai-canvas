const STORAGE_KEY_NAMESPACE = "open-ai-canvas";
const STORAGE_KEY_MARKERS = [".admin.announcements.pending-review", ".announcements.dismiss-today", ".announcements.dismiss-session", ".banner-announcements.dismissed"] as const;

export function migrateFirstPartyStorage(storage: Pick<Storage, "length" | "key" | "getItem" | "setItem" | "removeItem">) {
    const migrations: Array<{ source: string; target: string; value: string }> = [];
    for (let index = 0; index < storage.length; index += 1) {
        const source = storage.key(index);
        if (!source || source.startsWith(STORAGE_KEY_NAMESPACE)) continue;
        const marker = STORAGE_KEY_MARKERS.find((candidate) => source.includes(candidate));
        if (!marker) continue;
        const markerIndex = source.indexOf(marker);
        const value = storage.getItem(source);
        if (markerIndex < 1 || value === null) continue;
        migrations.push({ source, target: `${STORAGE_KEY_NAMESPACE}${source.slice(markerIndex)}`, value });
    }
    for (const migration of migrations) {
        if (storage.getItem(migration.target) === null) {
            storage.setItem(migration.target, migration.value);
        }
        storage.removeItem(migration.source);
    }
}

export function migrateFirstPartyStorageKeys() {
    if (typeof window === "undefined") return;
    for (const storage of [window.localStorage, window.sessionStorage]) {
        try {
            migrateFirstPartyStorage(storage);
        } catch {
            // 浏览器可能禁用存储；迁移失败不应阻断应用启动。
        }
    }
}
