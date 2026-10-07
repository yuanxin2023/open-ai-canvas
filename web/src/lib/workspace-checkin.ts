export function shouldShowWorkspaceCheckin(input: { creditsEnabled?: boolean; checkinBonusMicrocredits?: number | null; checkedInToday?: boolean }) {
    return Boolean(input.creditsEnabled && (input.checkinBonusMicrocredits || 0) > 0 && !input.checkedInToday);
}

export function workspaceCheckinTitle(brandName: string) {
    const name = brandName.trim() || "AI 创作工作台";
    return `${name}加油站`;
}
