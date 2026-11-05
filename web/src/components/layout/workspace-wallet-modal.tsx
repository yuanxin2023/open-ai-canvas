import { translate } from "@/i18n";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { App, Button, Input, Skeleton } from "antd";
import { ChevronLeft, ChevronRight, CircleAlert, Coins, CreditCard, HardDrive, History, RefreshCw, TicketCheck, WalletCards } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { useLocation, useNavigate } from "react-router";

import { WalletTopupPanel, type TopupCategory } from "@/components/layout/workspace-wallet-topup";
import { usePaymentOrderFlow } from "@/components/payment/use-payment-order-flow";
import { AppModal } from "@/components/ui/product/app-modal";
import { formatCredits } from "@/constant/credits";
import { useAccountFileStorageUsage } from "@/hooks/use-account-file-storage-usage";
import { accountFileStorageUsageQueryKey, accountStorageMeter } from "@/lib/account-storage-usage";
import { generationErrorMessage } from "@/lib/generation-error";
import { listPaymentProviders, listTopupProducts, type PaymentOrder, type PaymentProvider, type TopupProduct, type TopupProductKind } from "@/services/api/payments";
import { listMyStorageGrants } from "@/services/api/storage-grants";
import { getWallet, redeemCredits, type CreditLedgerEntry, type WalletSummary } from "@/services/api/wallet";
import { getReferralDashboard } from "@/services/api/referrals";
import { cn } from "@/lib/utils";
import { consumePaymentReturnPath, openWorkspaceWallet, WORKSPACE_WALLET_OPEN_EVENT, type WorkspaceWalletOpenDetail } from "@/lib/workspace-wallet";
import { useUserStore } from "@/stores/use-user-store";

type WalletModalTab = "topup" | "redeem" | "history";

const storageGrantsQueryKey = ["account-storage-grants"] as const;

export function WorkspaceWalletHost() {
    const userId = useUserStore((state) => state.user?.id);
    const creditsEnabled = useUserStore((state) => state.features.creditsEnabled);
    const loggedIn = useUserStore((state) => Boolean(state.user));
    const { pathname, search } = useLocation();
    const navigate = useNavigate();
    const [open, setOpen] = useState(false);
    const [openDetail, setOpenDetail] = useState<WorkspaceWalletOpenDetail>({});

    // 积分关闭时仍要能打开充值页购买存储空间，只是隐藏积分余额、兑换和积分记录。
    const applyOpen = (detail: WorkspaceWalletOpenDetail = {}) => {
        setOpenDetail(detail);
        setOpen(true);
    };

    useEffect(() => {
        const handleOpen = (raw: Event) => {
            applyOpen((raw as CustomEvent<WorkspaceWalletOpenDetail>).detail || {});
        };
        window.addEventListener(WORKSPACE_WALLET_OPEN_EVENT, handleOpen);
        return () => window.removeEventListener(WORKSPACE_WALLET_OPEN_EVENT, handleOpen);
    }, []);

    useEffect(() => {
        if (pathname !== "/wallet") return;
        const params = new URLSearchParams(search);
        openWorkspaceWallet({
            paymentOrderId: params.get("paymentOrder") || undefined,
            paymentInvalid: params.get("payment") === "invalid",
        });
        navigate(consumePaymentReturnPath(), { replace: true });
    }, [navigate, pathname, search]);

    if (!loggedIn) return null;
    return (
        <WorkspaceWalletModal
            key={userId}
            open={open}
            creditsEnabled={creditsEnabled}
            initialTab={openDetail.tab}
            initialCategory={openDetail.category}
            pendingPaymentOrderId={openDetail.paymentOrderId}
            paymentInvalid={openDetail.paymentInvalid}
            onClose={() => {
                setOpen(false);
                setOpenDetail({});
            }}
        />
    );
}

export function WorkspaceWalletModal({
    open,
    onClose,
    creditsEnabled = true,
    initialTab,
    initialCategory,
    pendingPaymentOrderId,
    paymentInvalid,
}: {
    open: boolean;
    onClose: () => void;
    creditsEnabled?: boolean;
    initialTab?: WalletModalTab;
    initialCategory?: TopupCategory;
    pendingPaymentOrderId?: string;
    paymentInvalid?: boolean;
}) {
    const { message } = App.useApp();
    const actorId = useUserStore((state) => state.user?.id);
    const alive = useRef(true);
    useEffect(() => {
        alive.current = true;
        return () => {
            alive.current = false;
        };
    }, []);
    const current = () => alive.current && Boolean(actorId) && useUserStore.getState().user?.id === actorId;
    const navigate = useNavigate();
    const queryClient = useQueryClient();
    const [tab, setTab] = useState<WalletModalTab>("topup");
    const [focusKind, setFocusKind] = useState<TopupProductKind | undefined>();
    const [focusToken, setFocusToken] = useState(0);
    const [wallet, setWallet] = useState<WalletSummary | null>(null);
    const [walletLoading, setWalletLoading] = useState(false);
    const [walletError, setWalletError] = useState("");
    const [page, setPage] = useState(1);
    const [products, setProducts] = useState<TopupProduct[]>([]);
    const [providers, setProviders] = useState<PaymentProvider[]>([]);
    const [paymentsLoading, setPaymentsLoading] = useState(false);
    const [selectedProductId, setSelectedProductId] = useState("");
    const [selectedProviderId, setSelectedProviderId] = useState("");
    const [code, setCode] = useState("");
    const [redeeming, setRedeeming] = useState(false);
    const requestSequence = useRef(0);
    const storageQuery = useAccountFileStorageUsage(open);
    const grantsQuery = useQuery({ queryKey: [...storageGrantsQueryKey, actorId], queryFn: ({ signal }) => listMyStorageGrants(signal), enabled: open && Boolean(actorId) });
    const referralQuery = useQuery({ queryKey: ["wallet-referral", actorId], queryFn: ({ signal }) => getReferralDashboard(signal), enabled: open && Boolean(actorId), staleTime: 0 });
    const storageMeter = accountStorageMeter(storageQuery.data);

    const selectedProduct = useMemo(() => products.find((item) => item.id === selectedProductId), [products, selectedProductId]);
    const selectedProvider = useMemo(() => providers.find((item) => item.id === selectedProviderId), [providers, selectedProviderId]);
    const activeTab: WalletModalTab = creditsEnabled ? tab : "topup";

    const reloadWallet = async (targetPage = page) => {
        if (!creditsEnabled) return;
        const sequence = ++requestSequence.current;
        setWalletLoading(true);
        setWalletError("");
        try {
            const result = await getWallet(targetPage, 20, "all");
            if (current() && sequence === requestSequence.current) setWallet(result);
        } catch (error) {
            if (current() && sequence === requestSequence.current) setWalletError(error instanceof Error ? error.message : "读取积分账户失败");
        } finally {
            if (current() && sequence === requestSequence.current) setWalletLoading(false);
        }
    };

    useEffect(() => {
        if (!open) return;
        setTab(initialTab || "topup");
        setFocusKind(initialCategory && initialCategory !== "all" ? initialCategory : undefined);
        setPage(1);
        void reloadWallet(1);
        setPaymentsLoading(true);
        Promise.all([listTopupProducts(), listPaymentProviders()])
            .then(([productResult, providerResult]) => {
                if (!current()) return;
                const enabledProducts = productResult.products.filter((item) => item.enabled);
                const usableProviders = providerResult.providers.filter((item) => item.enabled && item.pluginEnabled && item.configured);
                setProducts(enabledProducts);
                setProviders(usableProviders);
                // 从容量入口进入时优先选中存储空间套餐。
                const preferred = enabledProducts.filter((item) => !initialCategory || initialCategory === "all" || item.kind === initialCategory);
                setSelectedProductId((current) =>
                    current && enabledProducts.some((item) => item.id === current) && !initialCategory ? current : preferred.find((item) => item.canPurchase)?.id || enabledProducts.find((item) => item.canPurchase)?.id || "",
                );
                setSelectedProviderId((current) => current || usableProviders[0]?.id || "");
            })
            .catch((error) => {
                if (current()) message.error(error instanceof Error ? error.message : "读取充值配置失败");
            })
            .finally(() => {
                if (current()) setPaymentsLoading(false);
            });
    }, [open]);

    useEffect(() => {
        if (!open || !paymentInvalid) return;
        message.error(translate("account.wallet.paymentInvalid"));
    }, [open, paymentInvalid]);

    const announceWalletUpdated = async () => {
        if (!current()) return;
        setPage(1);
        await reloadWallet(1);
        if (current()) window.dispatchEvent(new CustomEvent("wallet:updated"));
    };

    const announceGranted = async (order: PaymentOrder) => {
        if (!current()) return;
        await queryClient.invalidateQueries({ queryKey: ["wallet-referral", actorId] });
        if (order.productKind === "storage") {
            await Promise.all([queryClient.invalidateQueries({ queryKey: accountFileStorageUsageQueryKey }), queryClient.invalidateQueries({ queryKey: storageGrantsQueryKey })]);
            return;
        }
        await announceWalletUpdated();
    };

    const payment = usePaymentOrderFlow({ onGranted: announceGranted });

    useEffect(() => {
        payment.resetIdempotency();
    }, [selectedProductId, selectedProviderId]);

    useEffect(() => {
        if (!open || !pendingPaymentOrderId) return;
        void payment.showOrder(pendingPaymentOrderId);
    }, [open, pendingPaymentOrderId]);

    const redeem = async () => {
        const normalized = code.trim().toLowerCase();
        if (normalized.length !== 32) {
            message.error(translate("account.wallet.redeemCodeInvalid"));
            return;
        }
        setRedeeming(true);
        try {
            await redeemCredits(normalized);
            if (!current()) return;
            setCode("");
            await announceWalletUpdated();
            message.success(translate("account.wallet.redeemSuccess"));
        } catch (error) {
            if (current()) message.error(error instanceof Error ? error.message : "兑换失败");
        } finally {
            if (current()) setRedeeming(false);
        }
    };

    const startPayment = async () => {
        if (!selectedProduct || !selectedProvider) {
            message.error(translate("account.wallet.selectRequired"));
            return;
        }
        if (!selectedProduct.canPurchase) {
            message.error(translate("account.wallet.productUnavailable"));
            return;
        }
        await payment.start(selectedProduct.id, selectedProvider.id);
    };

    // 定位到某类套餐：滚动到分组，并在当前选中项不属于该类时改选该类第一个可买套餐。
    const focusProducts = (kind: TopupProductKind) => {
        setTab("topup");
        setFocusKind(kind);
        setFocusToken((value) => value + 1);
        if (selectedProduct?.kind === kind) return;
        const next = products.find((item) => item.kind === kind && item.canPurchase);
        if (next) setSelectedProductId(next.id);
    };

    const available = wallet?.account.availableMicrocredits ?? 0;
    const totalPages = Math.max(1, Math.ceil((wallet?.total || 0) / 20));

    return (
        <>
            <AppModal flush open={open} title={null} footer={null} centered width="min(920px, calc(100vw - 28px))" onCancel={onClose} rootClassName="workspace-wallet-modal">
                <div className="workspace-wallet-shell">
                    <header className="workspace-wallet-header">
                        <div>
                            <span className="workspace-wallet-kicker">
                                <Coins />
                                {creditsEnabled ? translate("nav.tools.wallet") : translate("account.wallet.topupCenter")}
                            </span>
                            <h2>{creditsEnabled ? translate("account.wallet.title") : translate("account.wallet.titleStorage")}</h2>
                            <p>{creditsEnabled ? translate("account.wallet.description") : translate("account.wallet.descriptionStorage")}</p>
                        </div>
                        <div className="workspace-wallet-stats">
                            {creditsEnabled ? (
                                <div className="workspace-wallet-balance">
                                    <span>{translate("shell.account.availableCredits")}</span>
                                    <strong>{wallet ? formatCredits(available, 6) : "--"}</strong>
                                    <small>
                                        {translate("local.ui.frozen")}
                                        {wallet ? formatCredits(wallet.account.reservedMicrocredits, 6) : "--"}
                                    </small>
                                </div>
                            ) : null}
                            <button type="button" className="workspace-wallet-balance is-storage" aria-label={translate("account.wallet.viewStorage")} onClick={() => focusProducts("storage")}>
                                <span>
                                    <HardDrive className="inline size-3 align-[-1px]" aria-hidden="true" />
                                    {translate("account.wallet.storage")}
                                </span>
                                <strong>{storageQuery.data ? `${storageMeter.usedLabel} / ${storageMeter.totalLabel}` : "--"}</strong>
                                <span className="wallet-topup-storage-track" aria-hidden="true">
                                    <span style={{ width: `${storageMeter.percent}%` }} />
                                </span>
                                <small>
                                    {storageMeter.state === "read_only"
                                        ? translate("account.wallet.storageReadOnly")
                                        : storageMeter.state === "grace"
                                          ? translate("account.wallet.storageGrace")
                                          : storageMeter.purchasedLabel
                                            ? `含已购 ${storageMeter.purchasedLabel}`
                                            : `剩余 ${storageMeter.remainingLabel}`}
                                </small>
                            </button>
                        </div>
                    </header>

                    {creditsEnabled ? (
                        <div className="workspace-wallet-tabs" role="tablist" aria-label={translate("nav.tools.wallet")}>
                            <button type="button" role="tab" aria-selected={activeTab === "topup"} onClick={() => setTab("topup")}>
                                <WalletCards />
                                {translate("account.wallet.tabTopup")}
                            </button>
                            <button type="button" role="tab" aria-selected={activeTab === "redeem"} onClick={() => setTab("redeem")}>
                                <TicketCheck />
                                {translate("account.wallet.tabRedeem")}
                            </button>
                            <button type="button" role="tab" aria-selected={activeTab === "history"} onClick={() => setTab("history")}>
                                <History />
                                {translate("account.wallet.tabHistory")}
                            </button>
                        </div>
                    ) : null}

                    {activeTab === "topup" ? (
                        <WalletTopupPanel
                            newcomerPolicy={referralQuery.data?.newcomerEligible ? referralQuery.data.boundPolicy : undefined}
                            loading={paymentsLoading}
                            products={products}
                            providers={providers}
                            focusKind={focusKind}
                            focusToken={focusToken}
                            selectedProduct={selectedProduct}
                            onSelectProduct={setSelectedProductId}
                            selectedProviderId={selectedProviderId}
                            onSelectProvider={setSelectedProviderId}
                            creating={payment.creating}
                            onPay={() => void startPayment()}
                            storageUsage={storageQuery.data}
                            storageGrants={grantsQuery.data?.grants || []}
                            onCleanAssets={() => {
                                onClose();
                                navigate("/assets");
                            }}
                        />
                    ) : activeTab === "redeem" ? (
                        <div className="workspace-wallet-content is-redeem">
                            <section className="workspace-wallet-section is-redeem">
                                <div className="workspace-wallet-section-heading">
                                    <div>
                                        <h3>{translate("account.wallet.redeemTitle")}</h3>
                                        <p>{translate("account.wallet.redeemDescription")}</p>
                                    </div>
                                    <TicketCheck />
                                </div>
                                <div className="workspace-wallet-redeem-row">
                                    <Input size="large" value={code} maxLength={32} placeholder={translate("account.wallet.redeemPlaceholder")} onChange={(event) => setCode(event.target.value.replace(/\s/g, ""))} onPressEnter={() => void redeem()} />
                                    <Button size="large" loading={redeeming} disabled={code.trim().length !== 32} onClick={() => void redeem()}>
                                        {translate("account.wallet.redeemSubmit")}
                                    </Button>
                                </div>
                            </section>
                        </div>
                    ) : (
                        <div className="workspace-wallet-content is-history">
                            <div className="workspace-wallet-history-toolbar">
                                <div>
                                    <h3>{translate("account.wallet.tabHistory")}</h3>
                                    <p>{translate("account.wallet.historyDescription")}</p>
                                </div>
                                <Button type="text" icon={<RefreshCw />} loading={walletLoading} onClick={() => void reloadWallet(page)}>
                                    {translate("common.refresh")}
                                </Button>
                            </div>
                            <div className="workspace-wallet-history-scroll">
                                {walletError ? (
                                    <div className="workspace-wallet-inline-state is-error">
                                        <CircleAlert />
                                        <div>
                                            <strong>{translate("account.wallet.historyLoadFailed")}</strong>
                                            <span>{walletError}</span>
                                        </div>
                                        <Button onClick={() => void reloadWallet(page)}>{translate("common.retry")}</Button>
                                    </div>
                                ) : walletLoading && !wallet ? (
                                    <Skeleton active paragraph={{ rows: 6 }} />
                                ) : wallet?.entries.length ? (
                                    <div className="workspace-wallet-ledger">
                                        {wallet.entries.map((entry) => (
                                            <WalletLedgerRow key={entry.id} entry={entry} />
                                        ))}
                                    </div>
                                ) : (
                                    <div className="workspace-wallet-empty">
                                        <History />
                                        <strong>{translate("account.wallet.historyEmpty")}</strong>
                                        <span>{translate("account.wallet.historyEmptyHint")}</span>
                                    </div>
                                )}
                            </div>
                            <div className="workspace-wallet-pagination">
                                <span>
                                    {translate("local.ui.ordinalPrefix")}
                                    {page} / {totalPages}
                                    {translate("account.layout.pagination.jumpAfter")}
                                </span>
                                <button
                                    type="button"
                                    disabled={page <= 1 || walletLoading}
                                    aria-label={translate("account.layout.pagination.prev")}
                                    onClick={() => {
                                        const next = page - 1;
                                        setPage(next);
                                        void reloadWallet(next);
                                    }}
                                >
                                    <ChevronLeft />
                                </button>
                                <button
                                    type="button"
                                    disabled={page >= totalPages || walletLoading}
                                    aria-label={translate("account.layout.pagination.next")}
                                    onClick={() => {
                                        const next = page + 1;
                                        setPage(next);
                                        void reloadWallet(next);
                                    }}
                                >
                                    <ChevronRight />
                                </button>
                            </div>
                        </div>
                    )}
                </div>
            </AppModal>

            {payment.dialog}
        </>
    );
}

function WalletLedgerRow({ entry }: { entry: CreditLedgerEntry }) {
    const positive = entry.amountMicrocredits > 0;
    const note = entry.note ? generationErrorMessage(entry.note) : "";
    const title = entry.type === "consume" ? "模型调用" : entry.type === "refund" ? "消费退款" : entry.type === "payment_topup" ? "在线充值" : entry.type === "redeem" ? "兑换码充值" : entry.type === "referral" ? "邀请返利" : note || "积分调整";
    return (
        <article className="workspace-wallet-ledger-row">
            <span className={cn("workspace-wallet-ledger-icon", positive ? "is-income" : "is-consume")}>{positive ? <Coins /> : <CreditCard />}</span>
            <div>
                <strong>{title}</strong>
                <span>{[entry.scene, entry.model, note].filter(Boolean).join(" · ") || translate("account.wallet.ledger.change")}</span>
            </div>
            <time>{new Date(entry.createdAt).toLocaleString("zh-CN", { hour12: false })}</time>
            <b className={positive ? "is-income" : "is-consume"}>
                {positive ? "+" : ""}
                {formatCredits(entry.amountMicrocredits, 6)}
            </b>
        </article>
    );
}
