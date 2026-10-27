import { getActiveUserScope } from "@/lib/user-scope";

export const WORKSPACE_WALLET_OPEN_EVENT = "wallet:open";
export const WORKSPACE_CREDIT_PRODUCTS_OPEN_EVENT = "wallet:products-open";
const PAYMENT_RETURN_PATH_KEY = "wallet:payment-return-path";
const PAYMENT_RETURN_MAX_AGE_MS = 24 * 60 * 60 * 1000;
const PAYMENT_RETURN_ROUTES = [
    /^\/(?:|create|tasks|assets|prompts|skills|settings|profile)\/?$/,
    /^\/plugins(?:\/eagle)?\/?$/,
    /^\/canvas(?:\/[^/]+)?\/?$/,
    /^\/projects(?:\/[^/]+(?:\/[^/]+|\/chapters\/[^/]+|\/workflow\/[^/]+\/[^/]+)?)?\/?$/,
];

function validPaymentReturnPath(value: unknown): value is string {
    if (typeof value !== "string" || !value.startsWith("/") || value.startsWith("//") || /[\\\u0000-\u001f]/.test(value)) return false;
    try {
        const url = new URL(value, window.location.origin);
        return url.origin === window.location.origin && PAYMENT_RETURN_ROUTES.some((route) => route.test(url.pathname));
    } catch {
        return false;
    }
}

export function rememberPaymentReturnPath() {
    const path = `${window.location.pathname}${window.location.search}${window.location.hash}`;
    if (!validPaymentReturnPath(path)) return;
    window.sessionStorage.setItem(PAYMENT_RETURN_PATH_KEY, JSON.stringify({ path, scope: getActiveUserScope(), at: Date.now() }));
}

export function consumePaymentReturnPath() {
    const raw = window.sessionStorage.getItem(PAYMENT_RETURN_PATH_KEY);
    window.sessionStorage.removeItem(PAYMENT_RETURN_PATH_KEY);
    if (!raw) return "/";
    try {
        const saved = JSON.parse(raw) as { path?: unknown; scope?: unknown; at?: unknown };
        if (saved.scope !== getActiveUserScope() || typeof saved.at !== "number" || Date.now() - saved.at > PAYMENT_RETURN_MAX_AGE_MS || !validPaymentReturnPath(saved.path)) return "/";
        return saved.path;
    } catch {
        return "/";
    }
}

export type WorkspaceWalletOpenDetail = {
    paymentOrderId?: string;
    paymentInvalid?: boolean;
    /** 打开后所在的标签，默认“充值”。 */
    tab?: "topup" | "redeem" | "history";
    /** 充值页的商品分类，默认“全部”。 */
    category?: "all" | "credits" | "storage";
};

/** 工作台任意入口打开积分中心弹窗，不再进入独立钱包页。 */
export function openWorkspaceWallet(detail: WorkspaceWalletOpenDetail = {}) {
    window.dispatchEvent(new CustomEvent<WorkspaceWalletOpenDetail>(WORKSPACE_WALLET_OPEN_EVENT, { detail }));
}

/** 跳过积分概览与兑换入口，直接打开购买积分与套餐弹窗。 */
export function openWorkspaceCreditProducts() {
    window.dispatchEvent(new CustomEvent(WORKSPACE_CREDIT_PRODUCTS_OPEN_EVENT));
}
