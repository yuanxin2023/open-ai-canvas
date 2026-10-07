import { expect, test } from "bun:test";

import { migrateFirstPartyStorage } from "../src/lib/first-party-storage-migration";

class MemoryStorage {
    private readonly values = new Map<string, string>();

    get length() {
        return this.values.size;
    }

    key(index: number) {
        return [...this.values.keys()][index] ?? null;
    }

    getItem(key: string) {
        return this.values.get(key) ?? null;
    }

    setItem(key: string, value: string) {
        this.values.set(key, value);
    }

    removeItem(key: string) {
        this.values.delete(key);
    }
}

test("first-party browser keys migrate without overwriting current values", () => {
    const earlierNamespace = String.fromCharCode(121, 105, 110, 103, 99, 101);
    const storage = new MemoryStorage();
    storage.setItem(`${earlierNamespace}.announcements.dismiss-today:user:notice`, "earlier");
    storage.setItem(`${earlierNamespace}.banner-announcements.dismissed:user`, "banner");
    storage.setItem("open-ai-canvas.banner-announcements.dismissed:user", "current");
    storage.setItem("third-party.setting", "keep");

    migrateFirstPartyStorage(storage);

    expect(storage.getItem(`${earlierNamespace}.announcements.dismiss-today:user:notice`)).toBeNull();
    expect(storage.getItem("open-ai-canvas.announcements.dismiss-today:user:notice")).toBe("earlier");
    expect(storage.getItem(`${earlierNamespace}.banner-announcements.dismissed:user`)).toBeNull();
    expect(storage.getItem("open-ai-canvas.banner-announcements.dismissed:user")).toBe("current");
    expect(storage.getItem("third-party.setting")).toBe("keep");
});
