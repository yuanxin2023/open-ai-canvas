export function safeFeedbackPathname(value: string): string {
    const path = value.split(/[?#]/, 1)[0];
    if (path === "/") return path;
    const [first, second] = path.replace(/^\/+|\/+$/g, "").split("/");
    if (first === "canvas" || first === "projects") return second ? `/${first}/:id` : `/${first}`;
    if (["settings", "create", "prompts", "tasks", "assets", "plugins", "skills", "toolbox"].includes(first)) return `/${first}`;
    if (first === "admin") return "/admin/:section";
    return "/:page";
}

// Called only after explicit opt-in. Never reads cookies, storage, search or hash.
export function collectFeedbackEnvironment(): string {
    return JSON.stringify({
        userAgent: navigator.userAgent.slice(0, 512),
        platform: navigator.platform.slice(0, 128),
        language: navigator.language.slice(0, 64),
        screen: `${window.screen.width}x${window.screen.height}`,
        viewport: `${window.innerWidth}x${window.innerHeight}`,
        pathname: safeFeedbackPathname(window.location.pathname),
    });
}
