const HAN_PATTERN = /\p{Script=Han}/u;
const ALLOWED_PATTERN = /^[\p{Script=Han}a-z0-9]+$/u;
const LETTER_PATTERN = /[a-z]/;
const RESERVED_USERNAMES = new Set(["admin", "root", "system", "openai", "官方", "官方账号", "系统", "管理员", "客服", "平台"]);

export function normalizeUsername(value: string) {
    return value.normalize("NFKC").trim().toLowerCase();
}

export function usernameValidationMessage(value: string) {
    const normalized = normalizeUsername(value);
    const characters = Array.from(normalized);
    const hasHan = HAN_PATTERN.test(normalized);
    if (!ALLOWED_PATTERN.test(normalized)) return "用户名只支持中文、英文字母和数字";
    if (!hasHan && !LETTER_PATTERN.test(normalized)) return "用户名至少需要包含中文或英文字母";
    if (characters.length > 6 || (hasHan && characters.length < 2) || (!hasHan && characters.length < 3)) return "含中文的用户名需为 2-6 位，其他用户名需为 3-6 位";
    if (RESERVED_USERNAMES.has(normalized)) return "该用户名为系统保留名称";
    return "";
}
