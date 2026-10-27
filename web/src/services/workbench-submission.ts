import type { GenerationTask } from "@/services/api/task-center";

type WorkbenchSubmission = {
    create: () => Promise<GenerationTask | null>;
    read: (id: string) => Promise<GenerationTask>;
    isCurrent: () => boolean;
    onAccepted: (task: GenerationTask) => void;
};

// A successful create receipt remains authoritative when the subsequent read fails.
export async function submitWorkbenchGeneration({ create, read, isCurrent, onAccepted }: WorkbenchSubmission) {
    const accepted = await create();
    if (!accepted || !isCurrent()) return null;
    if (!accepted.id?.trim()) throw new Error("任务已提交，但没有返回任务编号");
    onAccepted(accepted);
    try {
        const task = await read(accepted.id);
        if (!isCurrent()) return null;
        if (task.id !== accepted.id) throw new Error("任务详情编号与提交回执不一致");
        return { task: accepted.workbenchSubmission ? { ...task, workbenchSubmission: accepted.workbenchSubmission } : task, detailPending: false };
    } catch {
        if (!isCurrent()) return null;
        return { task: accepted, detailPending: true };
    }
}
