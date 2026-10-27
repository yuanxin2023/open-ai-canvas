export function inspirationCoverKind(image: string): "local" | "external" | "invalid" {
    if (!image || image.includes("\\")) return "invalid";
    if (image.startsWith("/") && !image.startsWith("//") && !image.includes("..")) return "local";
    try {
        const url = new URL(image);
        return url.protocol === "https:" && !url.username && !url.password ? "external" : "invalid";
    } catch {
        return "invalid";
    }
}
