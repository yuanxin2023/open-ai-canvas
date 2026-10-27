import { afterEach, beforeEach, expect, test } from "bun:test";
import { CANVAS_PRESENCE_PREFERENCE_KEY, readCanvasPresenceHints, writeCanvasPresenceHints } from "../src/lib/canvas/canvas-presence-preference";
import { scopedStorageKey, setActiveUserScope, withUserScopedPersistenceSuppressed } from "../src/lib/user-scope";

const original = Object.getOwnPropertyDescriptor(globalThis, "window");
let storage: Map<string, string>;
beforeEach(() => {
    storage = new Map();
    const target = Object.assign(new EventTarget(), {
        localStorage: {
            getItem: (key: string) => storage.get(key) ?? null,
            setItem: (key: string, value: string) => {
                storage.set(key, value);
            },
        },
    });
    Object.defineProperty(globalThis, "window", { configurable: true, value: target });
    setActiveUserScope("alice");
});
afterEach(() => {
    if (original) Object.defineProperty(globalThis, "window", original);
    else Reflect.deleteProperty(globalThis, "window");
});

test("presence hints are off for new users and unrecognized stored values", () => {
    expect(readCanvasPresenceHints("alice")).toBe(false);
    for (const value of ["1", '"true"', "{}", "null"]) {
        storage.set(scopedStorageKey(CANVAS_PRESENCE_PREFERENCE_KEY, "alice"), value);
        expect(readCanvasPresenceHints("alice")).toBe(false);
    }
});

test("explicit opt-in persists only in the active user's scope", () => {
    writeCanvasPresenceHints("alice", true);
    expect(readCanvasPresenceHints("alice")).toBe(true);
    setActiveUserScope("bob");
    expect(readCanvasPresenceHints("alice")).toBe(false);
    expect(readCanvasPresenceHints("bob")).toBe(false);
    writeCanvasPresenceHints("alice", false);
    expect(storage.get(scopedStorageKey(CANVAS_PRESENCE_PREFERENCE_KEY, "alice"))).toBe("true");
    writeCanvasPresenceHints("bob", true);
    writeCanvasPresenceHints("bob", false);
    expect(readCanvasPresenceHints("bob")).toBe(false);
    setActiveUserScope("alice");
    expect(readCanvasPresenceHints("alice")).toBe(true);
});

test("account rehydration cannot write a preference into another scope", () => {
    withUserScopedPersistenceSuppressed(() => writeCanvasPresenceHints("alice", true));
    expect(readCanvasPresenceHints("alice")).toBe(false);
    expect(storage.has(scopedStorageKey(CANVAS_PRESENCE_PREFERENCE_KEY, "alice"))).toBe(false);
});

test("failed preference writes propagate and do not claim opt-in succeeded", () => {
    window.localStorage.setItem = () => {
        throw new Error("storage full");
    };
    expect(() => writeCanvasPresenceHints("alice", true)).toThrow("storage full");
    expect(readCanvasPresenceHints("alice")).toBe(false);
});
