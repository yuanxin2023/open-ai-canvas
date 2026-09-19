export const WORKSPACE_WALLET_OPEN_EVENT = "wallet:open";
export const WORKSPACE_CREDIT_PRODUCTS_OPEN_EVENT = "wallet:products-open";

export type WorkspaceWalletOpenDetail = {
    paymentOrderId?: string;
    paymentInvalid?: boolean;
};

/** 工作台任意入口打开积分中心弹窗，不再进入独立钱包页。 */
export function openWorkspaceWallet(detail: WorkspaceWalletOpenDetail = {}) {
    window.dispatchEvent(new CustomEvent<WorkspaceWalletOpenDetail>(WORKSPACE_WALLET_OPEN_EVENT, { detail }));
}

/** 跳过积分概览与兑换入口，直接打开购买积分与套餐弹窗。 */
export function openWorkspaceCreditProducts() {
    window.dispatchEvent(new CustomEvent(WORKSPACE_CREDIT_PRODUCTS_OPEN_EVENT));
}
