export type WorkbenchRestoreModel = { value: string; logicalModelId?: string };

export function resolveWorkbenchRestoreModel(source: { model?: string; logicalModelId?: string; modelSelector?: string }, options: WorkbenchRestoreModel[]) {
    // Plain names do not identify a channel. A single same-name alternative can
    // still belong to a different provider after the original was disabled.
    const matches = source.logicalModelId ? options.filter((option) => option.logicalModelId === source.logicalModelId) : source.modelSelector ? options.filter((option) => option.value === source.modelSelector) : [];
    return matches.length === 1 ? matches[0]!.value : null;
}
