import { expect, test } from "bun:test";
import { createWorkbenchSubmissionJournal, workbenchPayloadHash, type PendingWorkbenchSubmission, type WorkbenchSubmissionStore } from "@/services/workbench-submission-journal";

function fixture() {
    const data = new Map<string, PendingWorkbenchSubmission>();
    const archive: PendingWorkbenchSubmission[] = [];
    let serial = 0;
    const store: WorkbenchSubmissionStore = {
        async mutate(scope, kind, update) {
            const id = `${scope}:${kind}`,
                old = data.get(id) || null;
            const next = update(old);
            if (old && (!next || next.key !== old.key)) archive.push(old);
            if (next) data.set(id, next);
            else data.delete(id);
            return next;
        },
    };
    return { data, archive, store, journal: createWorkbenchSubmissionJournal(store, () => `request-key-${String(++serial).padStart(16, "0")}`) };
}

test("durable retry reuses identity and canonical request hash", async () => {
    const { journal, data } = fixture();
    const first = await journal.reserve("user", "image", "workbench", { parameters: { count: 1, audio: false }, prompt: "draft" });
    const second = await journal.reserve("user", "image", "workbench", { prompt: "draft", parameters: { audio: false, count: 1 } });
    expect(second.key).toBe(first.key);
    expect(data.size).toBe(1);
    expect((await journal.load("user", "image"))?.key).toBe(first.key);
});
test("changed fields and changed endpoint cannot consume the pending identity", async () => {
    const { journal } = fixture();
    const first = await journal.reserve("user", "image", "workbench", { count: 1 });
    await expect(journal.reserve("user", "image", "workbench", { count: 2 })).rejects.toThrow("参数已改变");
    await expect(journal.reserve("user", "image", "custom", { count: 1 })).rejects.toThrow("参数已改变");
    expect((await journal.load("user", "image"))?.key).toBe(first.key);
});
test("account and media kind keep independent pending identities", async () => {
    const { journal } = fixture();
    const keys = await Promise.all(
        [
            ["a", "image"],
            ["b", "image"],
            ["a", "video"],
        ].map(async ([scope, kind]) => (await journal.reserve(scope!, kind as "image" | "video", "workbench", {})).key),
    );
    expect(new Set(keys).size).toBe(3);
});
test("a late acknowledgement cannot clear the next explicit submission", async () => {
    const { journal, archive } = fixture();
    const first = await journal.reserve("user", "image", "workbench", {});
    await journal.clear("user", "image", first.key);
    const next = await journal.reserve("user", "image", "workbench", {});
    await journal.clear("user", "image", first.key);
    expect((await journal.load("user", "image"))?.key).toBe(next.key);
    expect(archive.map((item) => item.key)).toEqual([first.key]);
});
test("journal stores no credentials, request body or temporary media URLs", async () => {
    const { journal, data } = fixture();
    await journal.reserve("user", "image", "custom", { prompt: "private-draft", config: { apiKey: "private-api-key", url: "https://example.com/?signature=secret" } });
    const stored = JSON.stringify([...data.values()]);
    expect(stored).not.toContain("private-draft");
    expect(stored).not.toContain("private-api-key");
    expect(stored).not.toContain("signature");
    expect([...data.values()][0]!.hash).toMatch(/^[a-f0-9]{64}$/);
});
test("storage failure rejects reservation before a caller can send", async () => {
    const journal = createWorkbenchSubmissionJournal({
        async mutate() {
            throw new Error("storage offline");
        },
    });
    await expect(journal.reserve("user", "image", "workbench", {})).rejects.toThrow("storage offline");
});
test("hash follows JSON wire semantics and preserves explicit false and zero", async () => {
    expect(await workbenchPayloadHash({ optional: undefined, count: 0, audio: false })).toBe(await workbenchPayloadHash({ audio: false, count: 0 }));
    expect(await workbenchPayloadHash({ count: 0 })).not.toBe(await workbenchPayloadHash({}));
});
