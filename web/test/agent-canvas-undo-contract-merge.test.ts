import { expect, test } from "bun:test";
import { agentCanvasUndoTarget } from "@/lib/canvas/agent-canvas-undo";
import { applyAgentEvent } from "@/lib/canvas/cloud-agent-events";
import type { AgentEvent } from "@/services/api/agent";
import type { CloudAgentChatMessage } from "@/components/canvas/canvas-cloud-agent-chat-ui";

const hash = "a".repeat(64);
const receipt = { runId: "historical-run", canvasId: "canvas", stepId: "step", expectedSnapshotHash: hash };

test("invalid persisted receipts cannot fall back to a different legacy operation", () => {
    const detail = { eventType: "tool_completed", runId: "new-run", canvasId: "canvas", callId: "new-step", toolName: "canvas_apply_ops", result: { snapshotHash: hash } };
    expect(agentCanvasUndoTarget({ ...detail, undo: { ...receipt, expectedSnapshotHash: "invalid" } })).toBeNull();
    expect(agentCanvasUndoTarget({ ...detail, undo: receipt })).toEqual(receipt);
});

test("replayed mutation and completion events cannot revive an undone historical step", () => {
    let messages: CloudAgentChatMessage[] = [];
    let seq = 0;
    const apply = (type: string, payload: Record<string, unknown>) => {
        const event: AgentEvent = { eventId: `event-${++seq}`, runId: receipt.runId, seq, type, payload, createdAt: new Date(0).toISOString() };
        applyAgentEvent(
            event,
            (next) => {
                messages = typeof next === "function" ? next(messages) : next;
            },
            () => {},
            () => {},
        );
    };
    const mutation = { callId: receipt.stepId, canvasId: receipt.canvasId, operation: "canvas_edit_storyboard", undo: receipt };
    const completion = { callId: receipt.stepId, toolName: "canvas_edit_storyboard", result: { committed: true } };
    apply("canvas_updated", mutation);
    apply("tool_completed", completion);
    expect(agentCanvasUndoTarget(messages[0].detail)).toEqual(receipt);
    apply("canvas_undone", { stepId: receipt.stepId });
    apply("tool_completed", completion);
    apply("canvas_updated", mutation);
    expect(messages).toHaveLength(1);
    expect(agentCanvasUndoTarget(messages[0].detail)).toBeNull();
});
