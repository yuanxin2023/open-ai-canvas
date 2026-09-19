const passwordCharacterGroups = [
    "ABCDEFGHJKLMNPQRSTUVWXYZ",
    "abcdefghijkmnopqrstuvwxyz",
    "23456789",
    "!@#$%&*+-_",
] as const;

const passwordCharacters = passwordCharacterGroups.join("");

function secureRandomIndex(max: number) {
    if (!Number.isSafeInteger(max) || max <= 0) throw new Error("随机密码字符集无效");
    const limit = Math.floor(0x1_0000_0000 / max) * max;
    const values = new Uint32Array(1);
    do {
        globalThis.crypto.getRandomValues(values);
    } while (values[0] >= limit);
    return values[0] % max;
}

function randomCharacter(characters: string) {
    return characters[secureRandomIndex(characters.length)];
}

export function generateAdminPassword(length = 16) {
    if (!Number.isSafeInteger(length) || length < passwordCharacterGroups.length) {
        throw new Error(`随机密码长度不能少于 ${passwordCharacterGroups.length} 位`);
    }
    const result = passwordCharacterGroups.map(randomCharacter);
    while (result.length < length) result.push(randomCharacter(passwordCharacters));
    for (let index = result.length - 1; index > 0; index -= 1) {
        const swapIndex = secureRandomIndex(index + 1);
        [result[index], result[swapIndex]] = [result[swapIndex], result[index]];
    }
    return result.join("");
}
