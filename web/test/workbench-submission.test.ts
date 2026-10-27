import { expect, test } from "bun:test";
import type { GenerationTask } from "@/services/api/task-center";
import { submitWorkbenchGeneration } from "@/services/workbench-submission";

const accepted: GenerationTask = { id: "task-accepted", type: "canvas_image", prompt: "test", status: "queued", attempts: 0, createdAt: "2026-10-26T00:00:00Z", updatedAt: "2026-10-26T00:00:00Z" };

test("records the paid submission receipt before reading task details", async () => {
    const calls: string[] = [];
    const detailed = { ...accepted, status: "running" as const };
    const result = await submitWorkbenchGeneration({
        create: async () => {
            calls.push("create");
            return accepted;
        },
        onAccepted: (task) => {
            expect(task).toBe(accepted);
            calls.push("accepted");
        },
        read: async (id) => {
            expect(id).toBe(accepted.id);
            calls.push("read");
            return detailed;
        },
        isCurrent: () => true,
    });
    expect(calls).toEqual(["create", "accepted", "read"]);
    expect(result).toEqual({ task: detailed, detailPending: false });
});

test("detail failure retains the receipt and never repeats the create", async () => {
    let creates = 0;
    const receipts: GenerationTask[] = [];
    const result = await submitWorkbenchGeneration({
        create: async () => {
            creates++;
            return accepted;
        },
        read: async () => {
            throw new Error("offline");
        },
        onAccepted: (task) => receipts.push(task),
        isCurrent: () => true,
    });
    expect(creates).toBe(1);
    expect(receipts).toEqual([accepted]);
    expect(result).toEqual({ task: accepted, detailPending: true });
});

test("write errors propagate without reporting an accepted task", async () => {
    let followups = 0;
    await expect(
        submitWorkbenchGeneration({
            create: async () => {
                throw new Error("creation timeout");
            },
            read: async () => {
                followups++;
                return accepted;
            },
            onAccepted: () => followups++,
            isCurrent: () => true,
        }),
    ).rejects.toThrow("creation timeout");
    expect(followups).toBe(0);
});

test("account change during create stops receipt publication and detail requests", async () => {
    let current = true;
    let followups = 0;
    const result = await submitWorkbenchGeneration({
        create: async () => {
            current = false;
            return accepted;
        },
        read: async () => {
            followups++;
            return accepted;
        },
        onAccepted: () => followups++,
        isCurrent: () => current,
    });
    expect(result).toBeNull();
    expect(followups).toBe(0);
});

test("account change during detail loading prevents the old result from returning", async () => {
    let current = true;
    const result = await submitWorkbenchGeneration({
        create: async () => accepted,
        onAccepted: () => {},
        isCurrent: () => current,
        read: async () => {
            current = false;
            return accepted;
        },
    });
    expect(result).toBeNull();
});

test("unrelated task details cannot replace the accepted receipt", async () => {
    const result = await submitWorkbenchGeneration({
        create: async () => accepted,
        onAccepted: () => {},
        isCurrent: () => true,
        read: async () => ({ ...accepted, id: "other-task" }),
    });
    expect(result).toEqual({ task: accepted, detailPending: true });
});

test("detail reads cannot replace the original submission quote with current pricing", async () => {
    const receipt = { requestKey: "original-request-key", replayed: true, quote: { billingEnabled: true, amountMicrocredits: 10000, billingMode: "fixed_request", priceTierVersion: 1, priceVersion: 1, quantity: 1 } };
    const result = await submitWorkbenchGeneration({ create: async () => ({ ...accepted, workbenchSubmission: receipt }), onAccepted: () => {}, isCurrent: () => true, read: async () => accepted });
    expect(result?.task.workbenchSubmission).toEqual(receipt);
});
