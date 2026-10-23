import { afterEach, expect, test } from "bun:test";

import { consumePaymentReturnPath, rememberPaymentReturnPath } from "../src/lib/workspace-wallet";

const originalWindow = globalThis.window;

afterEach(() => {
    Object.defineProperty(globalThis, "window", { configurable: true, value: originalWindow });
});

function mockWindow(path: string) {
    const saved = new Map<string, string>();
    const url = new URL(path, "https://canvas.example");
    const localStorage = { getItem: (key: string) => key === "infinite-canvas:active-user-scope" ? "user-1" : null };
    const sessionStorage = {
        getItem: (key: string) => saved.get(key) ?? null,
        setItem: (key: string, value: string) => { saved.set(key, value); },
        removeItem: (key: string) => { saved.delete(key); },
    };
    Object.defineProperty(globalThis, "window", { configurable: true, value: { location: url, localStorage, sessionStorage } });
    return saved;
}

test("payment return restores the source canvas route once", () => {
    mockWindow("/canvas/project-1?panel=images#failed");
    rememberPaymentReturnPath();
    expect(consumePaymentReturnPath()).toBe("/canvas/project-1?panel=images#failed");
    expect(consumePaymentReturnPath()).toBe("/");
});

test("payment return restores a short drama workflow route", () => {
    mockWindow("/projects/project-1/workflow/unit-1/previz");
    rememberPaymentReturnPath();
    expect(consumePaymentReturnPath()).toBe("/projects/project-1/workflow/unit-1/previz");
});

test("payment return rejects external and stale paths", () => {
    const saved = mockWindow("/canvas/project-1");
    rememberPaymentReturnPath();
    const key = [...saved.keys()][0];
    saved.set(key, JSON.stringify({ path: "//evil.example", scope: "user-1", at: Date.now() }));
    expect(consumePaymentReturnPath()).toBe("/");
    saved.set(key, JSON.stringify({ path: "/canvas/project-1", scope: "user-1", at: Date.now() - 25 * 60 * 60 * 1000 }));
    expect(consumePaymentReturnPath()).toBe("/");
    saved.set(key, JSON.stringify({ path: "/projects/project-1/workflow/unit-1", scope: "user-1", at: Date.now() }));
    expect(consumePaymentReturnPath()).toBe("/");
});
