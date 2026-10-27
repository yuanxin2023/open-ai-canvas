export const loginMethodLabels: Record<string, string> = { password: "密码", email: "邮箱验证码", sms: "短信验证码", linuxdo: "Linux.do", verification: "验证码（方式未识别）", unknown: "未记录", email_register: "邮箱注册" };
export const loginEventLabels: Record<string, string> = { login: "登录", logout: "主动退出" };
export const loginSourceLabels: Record<string, string> = { login: "用户登录", registration: "注册后自动登录", logout: "主动退出", legacy: "历史登录记录" };

// UA 只用于描述客户端自报信息，不能作为身份或安全判断依据。
export function loginLogDevice(userAgent: string, recorded?: { browser?: string; browserVersion?: string; os?: string; osVersion?: string }) {
    const recordedBrowser = [recorded?.browser, recorded?.browserVersion].filter(Boolean).join(" ");
    const recordedOS = [recorded?.os, recorded?.osVersion].filter(Boolean).join(" ");
    if (!userAgent) return { browser: recordedBrowser || "未记录", os: recordedOS || "未记录" };
    const browsers: Array<[RegExp, string]> = [
        [/(?:Edg|EdgA|EdgiOS)\/(\d+)/, "Edge"],
        [/OPR\/(\d+)/, "Opera"],
        [/(?:Firefox|FxiOS)\/(\d+)/, "Firefox"],
        [/(?:Chrome|CriOS)\/(\d+)/, "Chrome"],
        [/Version\/(\d+).*Safari\//, "Safari"],
    ];
    let browser = "未识别浏览器";
    for (const [pattern, name] of browsers) {
        const match = userAgent.match(pattern);
        if (match) {
            browser = `${name} ${match[1]}`;
            break;
        }
    }
    const android = userAgent.match(/Android ([\d.]+)/);
    const ios = userAgent.match(/(?:iPhone|iPad|iPod).*OS ([\d_]+)/);
    const mac = userAgent.match(/Mac OS X ([\d_]+)/);
    const os = android ? `Android ${android[1]}` : ios ? `iOS ${ios[1].replaceAll("_", ".")}` : /Windows NT/.test(userAgent) ? "Windows" : mac ? `macOS ${mac[1].replaceAll("_", ".")}` : /Linux/.test(userAgent) ? "Linux" : "未识别系统";
    return { browser: recordedBrowser || browser, os: recordedOS || os };
}
