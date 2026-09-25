export type SlashCommandMatch = {
    start: number;
    end: number;
    query: string;
};

/**
 * Slash commands only start at the beginning of the composer or after
 * whitespace. This keeps URLs and ordinary path fragments from opening the
 * command menu.
 */
export function findSlashCommandMatch(value: string, cursor: number): SlashCommandMatch | null {
    const safeCursor = Math.max(0, Math.min(cursor, value.length));
    const prefix = value.slice(0, safeCursor);
    const match = /(?:^|\s)\/([^\s/]*)$/.exec(prefix);
    if (!match) return null;
    const slashOffset = match[0].lastIndexOf("/");
    return {
        start: match.index + slashOffset,
        end: safeCursor,
        query: match[1],
    };
}

export function applySlashCommand(value: string, match: SlashCommandMatch, replacement: string) {
    const next = `${value.slice(0, match.start)}${replacement}${value.slice(match.end)}`;
    return { value: next, cursor: match.start + replacement.length };
}
