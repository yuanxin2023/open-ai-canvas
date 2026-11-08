import { AlipayCircleFilled, WechatFilled } from "@ant-design/icons";
import { App, Button, ConfigProvider, Input, QRCode, Skeleton } from "antd";
import { CircleCheck, Clock3, CreditCard, ExternalLink, History, Sparkles, TicketCheck } from "lucide-react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useLocation, useNavigate } from "react-router";

import { usePaymentOrderFlow } from "@/components/payment/use-payment-order-flow";
import { CreditProductCard } from "@/components/payments/credit-product-card";
import { PaymentPromotionBanner, promotionClockOffset } from "@/components/payments/payment-promotion-banner";
import { AppModal } from "@/components/ui/product/app-modal";
import { formatCredits } from "@/constant/credits";
import { useWalletBalance } from "@/hooks/use-wallet-balance";
import { useCreditRedemption } from "@/hooks/use-credit-redemption";
import { getWorkspaceAntThemeConfig } from "@/lib/app-theme";
import { referralCredits } from "@/lib/referral-policy";
import { isPaymentQrImageURL } from "@/lib/payment-checkout-code";
import { topupSaleDescription } from "@/lib/topup-sale-policy";
import { consumePaymentReturnPath, WORKSPACE_WALLET_OPEN_EVENT, type WorkspaceWalletOpenDetail } from "@/lib/workspace-wallet";
import { listPaymentProviders, listTopupProducts, type PaymentProvider, type TopupProduct } from "@/services/api/payments";
import { getReferralDashboard } from "@/services/api/referrals";
import { useUserStore } from "@/stores/use-user-store";

const workspaceCreditTheme = getWorkspaceAntThemeConfig();

export function WorkspacePackageCenterHost() {
    const userId = useUserStore((state) => state.user?.id);
    const creditsEnabled = useUserStore((state) => state.features.creditsEnabled);
    return userId && creditsEnabled ? <WorkspacePackageCenter key={userId} userId={userId} /> : null;
}

function WorkspacePackageCenter({ userId }: { userId: string }) {
    const { t } = useTranslation();
    const { message } = App.useApp();
    const navigate = useNavigate();
    const { pathname, search } = useLocation();
    const queryClient = useQueryClient();
    const { availableMicrocredits } = useWalletBalance(userId);
    const balance = availableMicrocredits === null ? "--" : formatCredits(availableMicrocredits);
    const [productsOpen, setProductsOpen] = useState(false);
    const [redeemOpen, setRedeemOpen] = useState(false);
    const [selectedProduct, setSelectedProduct] = useState<TopupProduct | null>(null);
    const [selectedProviderId, setSelectedProviderId] = useState("");
    const { code, setCode, normalizedCode, redeeming, redeem } = useCreditRedemption(userId);
    const paymentCatalogQuery = useQuery({
        queryKey: ["payment-catalog", userId],
        queryFn: async () => {
            const [productsResult, providersResult] = await Promise.all([listTopupProducts(), listPaymentProviders()]);
            return {
                products: productsResult.products.filter((product) => product.enabled && product.kind === "credits"),
                promotion: productsResult.promotion,
                serverTime: productsResult.serverTime,
                providers: providersResult.providers.filter((provider) => provider.enabled && provider.pluginEnabled && provider.configured),
            };
        },
        staleTime: 5 * 60_000,
        refetchOnWindowFocus: false,
    });
    const products = useMemo(() => paymentCatalogQuery.data?.products ?? [], [paymentCatalogQuery.data?.products]);
    const providers = useMemo(() => paymentCatalogQuery.data?.providers ?? [], [paymentCatalogQuery.data?.providers]);
    const promotion = paymentCatalogQuery.data?.promotion;
    const catalogServerTime = paymentCatalogQuery.data?.serverTime || new Date().toISOString();
    const maxProductCredits = useMemo(() => products.reduce((maximum, product) => Math.max(maximum, product.creditsMicrocredits), 0), [products]);
    const productsLoading = paymentCatalogQuery.isPending;
    const productsError = !paymentCatalogQuery.data && paymentCatalogQuery.error ? (paymentCatalogQuery.error instanceof Error ? paymentCatalogQuery.error.message : "读取商品套餐失败") : "";
    const selectedProvider = providers.find((provider) => provider.id === selectedProviderId);
    const referralQuery = useQuery({
        queryKey: ["wallet-referral", userId],
        queryFn: ({ signal }) => getReferralDashboard(signal),
        enabled: productsOpen || Boolean(selectedProduct),
        staleTime: 0,
    });
    const newcomerPolicy = referralQuery.data?.newcomerEligible ? referralQuery.data.boundPolicy : undefined;
    const newcomerBonus = selectedProduct && newcomerPolicy?.newcomerEnabled ? Math.min(referralCredits(selectedProduct.creditsMicrocredits, newcomerPolicy.newcomerRateBps), newcomerPolicy.newcomerCapMicrocredits || Number.MAX_SAFE_INTEGER) : 0;
    const payment = usePaymentOrderFlow({
        onGranted: () => {
            window.dispatchEvent(new CustomEvent("wallet:updated"));
            void queryClient.invalidateQueries({ queryKey: ["wallet-referral", userId] });
            void queryClient.invalidateQueries({ queryKey: ["payment-catalog", userId] });
        },
    });
    const { order: paymentOrder, creating: paymentCreating, querying: paymentQuerying, clock } = payment;
    const paymentRef = useRef(payment);
    paymentRef.current = payment;
    const paymentReturnTarget = useRef<string | null>(null);

    useEffect(() => {
        const openCenter = (raw: Event) => {
            const detail = (raw as CustomEvent<WorkspaceWalletOpenDetail>).detail || {};
            paymentRef.current.close();
            setSelectedProduct(null);
            setRedeemOpen(detail.view === "redeem");
            setProductsOpen(detail.view !== "redeem");
            if (detail.paymentInvalid) message.error("支付结果无效，未产生充值");
            if (detail.paymentOrderId) void paymentRef.current.showOrder(detail.paymentOrderId);
        };
        window.addEventListener(WORKSPACE_WALLET_OPEN_EVENT, openCenter);
        return () => window.removeEventListener(WORKSPACE_WALLET_OPEN_EVENT, openCenter);
    }, [message]);

    useEffect(() => {
        if (pathname !== "/wallet") {
            paymentReturnTarget.current = null;
            return;
        }
        // React 开发模式会重放 effect，来源路径只能消费一次，订单读取可重新发起。
        const firstReturn = paymentReturnTarget.current === null;
        const target = paymentReturnTarget.current ?? consumePaymentReturnPath();
        paymentReturnTarget.current = target;
        const params = new URLSearchParams(search);
        setProductsOpen(true);
        if (firstReturn && params.get("payment") === "invalid") message.error("支付结果无效，未产生充值");
        const orderId = params.get("paymentOrder");
        if (orderId) void paymentRef.current.showOrder(orderId);
        navigate(target, { replace: true });
    }, [navigate, pathname, search, message]);

    useEffect(() => {
        if (productsOpen) void paymentCatalogQuery.refetch({ cancelRefetch: false });
    }, [productsOpen, paymentCatalogQuery.refetch]);

    useEffect(() => {
        if (!productsOpen || !promotion) return;
        const boundary = promotion.phase === "scheduled" ? promotion.startsAt : promotion.phase === "active" ? promotion.endsAt : undefined;
        if (!boundary) return;
        const remaining = Date.parse(boundary) - (Date.now() + promotionClockOffset(catalogServerTime));
        const timer = window.setTimeout(() => void paymentCatalogQuery.refetch({ cancelRefetch: false }), Math.max(250, Math.min(remaining + 150, 2_147_000_000)));
        return () => window.clearTimeout(timer);
    }, [catalogServerTime, paymentCatalogQuery.refetch, productsOpen, promotion]);

    useEffect(() => {
        if (!productsOpen) return;
        const refreshWhenVisible = () => {
            if (document.visibilityState === "visible") void paymentCatalogQuery.refetch({ cancelRefetch: false });
        };
        document.addEventListener("visibilitychange", refreshWhenVisible);
        return () => document.removeEventListener("visibilitychange", refreshWhenVisible);
    }, [paymentCatalogQuery.refetch, productsOpen]);

    useEffect(() => {
        if (selectedProduct) setSelectedProduct(products.find((product) => product.id === selectedProduct.id) ?? null);
    }, [products, selectedProduct?.id]);

    useEffect(() => {
        setSelectedProviderId((current) => (providers.some((provider) => provider.id === current) ? current : providers[0]?.id || ""));
    }, [providers]);

    useEffect(() => {
        paymentRef.current.resetIdempotency();
    }, [selectedProduct?.id, selectedProviderId]);

    const openPaymentSelector = (product: TopupProduct) => {
        if (!product.canPurchase) {
            message.warning("该套餐当前不可购买");
            return;
        }
        payment.close();
        setSelectedProduct(product);
    };
    const closePaymentSelector = () => {
        if (paymentCreating || paymentQuerying) return;
        payment.close();
        setSelectedProduct(null);
    };
    const startPayment = async () => {
        if (!selectedProduct?.canPurchase) {
            message.warning("该套餐当前不可购买");
            return;
        }
        if (!selectedProvider) {
            message.error("请选择支付方式");
            return;
        }
        await payment.start(selectedProduct.id, selectedProvider.id);
    };
    const confirmPayment = payment.refresh;
    const cancelPayment = payment.cancel;
    const retryPaymentCheckout = payment.retry;

    return (
        <ConfigProvider theme={workspaceCreditTheme}>
            <AppModal flush centered open={productsOpen} title={null} footer={null} width="min(1280px, calc(100vw - 32px))" rootClassName="workspace-credit-products-modal" onCancel={() => setProductsOpen(false)}>
                <section className="workspace-credit-products-shell">
                    {promotion ? <PaymentPromotionBanner promotion={promotion} serverTime={catalogServerTime} onBoundary={() => void paymentCatalogQuery.refetch({ cancelRefetch: false })} /> : null}
                    <header className="workspace-credit-products-header">
                        <h2>选择您的套餐</h2>
                        <p>选择适合你的积分套餐，购买成功后积分将自动到账</p>
                        <span className="workspace-credit-products-balance">
                            <Sparkles aria-hidden />
                            当前余额 {balance} 积分
                        </span>
                    </header>

                    {productsLoading ? (
                        <div className="workspace-credit-products-grid" aria-label="正在加载商品套餐">
                            {Array.from({ length: 3 }, (_, index) => (
                                <Skeleton.Node key={index} active className="workspace-credit-product-skeleton" />
                            ))}
                        </div>
                    ) : productsError ? (
                        <div className="workspace-credit-products-state">
                            <strong>商品套餐加载失败</strong>
                            <span>{productsError}</span>
                            <Button onClick={() => void paymentCatalogQuery.refetch()}>重新加载</Button>
                        </div>
                    ) : products.length ? (
                        <div className="workspace-credit-products-grid">
                            {products.map((product) => (
                                <CreditProductCard
                                    key={product.id}
                                    product={product}
                                    maxCreditsMicrocredits={maxProductCredits}
                                    availabilityLabel={topupSaleDescription(product)}
                                    actionLabel={!product.canPurchase ? "暂不可购买" : providers.length ? undefined : "暂无可用支付方式"}
                                    actionDisabled={!product.canPurchase || !providers.length}
                                    onAction={() => openPaymentSelector(product)}
                                />
                            ))}
                        </div>
                    ) : (
                        <div className="workspace-credit-products-state">
                            <strong>暂无可购买套餐</strong>
                            <span>管理员尚未上架积分商品，请稍后再试。</span>
                        </div>
                    )}
                    <nav className="mt-6 flex flex-wrap justify-center gap-3" aria-label={t("nav.tools.wallet")}>
                        <Button
                            type="text"
                            icon={<TicketCheck />}
                            onClick={() => {
                                setProductsOpen(false);
                                setRedeemOpen(true);
                            }}
                        >
                            {t("account.wallet.tabRedeem")}
                        </Button>
                        <Button
                            type="text"
                            icon={<History />}
                            onClick={() => {
                                setProductsOpen(false);
                                navigate("/settings?section=wallet");
                            }}
                        >
                            {t("account.wallet.tabHistory")}
                        </Button>
                    </nav>
                </section>
            </AppModal>

            <AppModal
                flush
                centered
                open={Boolean(selectedProduct || paymentOrder)}
                title={null}
                footer={null}
                width={paymentOrder ? "min(780px, calc(100vw - 24px))" : "min(600px, calc(100vw - 24px))"}
                rootClassName="workspace-credit-checkout-modal"
                mask={{ closable: !paymentCreating && !paymentQuerying }}
                onCancel={closePaymentSelector}
            >
                {selectedProduct || paymentOrder ? (
                    <section className={`workspace-credit-checkout-shell${paymentOrder ? " is-payment-order" : ""}`}>
                        <header className="workspace-credit-checkout-header">
                            <div>
                                <h2>{paymentOrder ? (paymentOrder.status === "credited" ? "支付成功" : paymentOrder.checkout.mode === "qr_code" ? "扫码支付" : "完成支付") : "确认订单"}</h2>
                                <p>
                                    {paymentOrder
                                        ? paymentOrder.status !== "credited" && paymentOrder.checkout.mode === "qr_code"
                                            ? `请使用${paymentProviderLabel(providers.find((provider) => provider.id === paymentOrder.providerId))}扫描二维码完成付款`
                                            : "请按收银台提示完成付款，到账状态将自动更新。"
                                        : "确认套餐与支付方式，付款成功后积分自动到账"}
                                </p>
                            </div>
                        </header>

                        {!paymentOrder && selectedProduct ? (
                            <>
                                <div className="workspace-credit-checkout-total">
                                    <span>应付金额</span>
                                    <strong>
                                        <small>¥</small>
                                        {(selectedProduct.amountFen / 100).toFixed(2)}
                                    </strong>
                                </div>
                                <div className="workspace-credit-checkout-summary">
                                    <div>
                                        <span>所选套餐</span>
                                        <strong>{selectedProduct.name}</strong>
                                    </div>
                                    <div>
                                        <span>到账积分</span>
                                        <strong>{formatCredits(selectedProduct.creditsMicrocredits)} 积分</strong>
                                    </div>
                                    {newcomerBonus > 0 ? <p className="workspace-credit-checkout-note">邀请新人首充预计另赠 {formatCredits(newcomerBonus)} 积分</p> : null}
                                </div>
                                <h3 className="workspace-credit-checkout-method-title">支付方式</h3>
                                <div className="workspace-credit-provider-options" role="radiogroup" aria-label="支付方式">
                                    {providers.map((provider) => (
                                        <button
                                            key={provider.id}
                                            type="button"
                                            role="radio"
                                            aria-checked={selectedProviderId === provider.id}
                                            tabIndex={selectedProviderId === provider.id ? 0 : -1}
                                            className="workspace-credit-provider-option"
                                            onClick={() => setSelectedProviderId(provider.id)}
                                            onKeyDown={(event) => {
                                                const index = providers.indexOf(provider);
                                                const direction = event.key === "ArrowRight" || event.key === "ArrowDown" ? 1 : event.key === "ArrowLeft" || event.key === "ArrowUp" ? -1 : 0;
                                                if (!direction && event.key !== "Home" && event.key !== "End") return;
                                                event.preventDefault();
                                                const next = event.key === "Home" ? 0 : event.key === "End" ? providers.length - 1 : (index + direction + providers.length) % providers.length;
                                                setSelectedProviderId(providers[next].id);
                                                event.currentTarget.parentElement?.querySelectorAll<HTMLButtonElement>('[role="radio"]')[next]?.focus();
                                            }}
                                        >
                                            <PaymentProviderIcon providerId={provider.paymentMethod} />
                                            <span>{paymentProviderLabel(provider)}</span>
                                            <span className="workspace-credit-provider-radio" aria-hidden />
                                        </button>
                                    ))}
                                </div>

                                <p className="workspace-credit-checkout-hint">
                                    {selectedProvider?.checkoutMode === "qr_code" ? "确认后请扫码完成付款" : "确认后将跳转至支付页面完成付款"}
                                    {selectedProvider?.checkoutMode === "redirect" ? <ExternalLink aria-hidden /> : null}
                                </p>
                                <Button className="workspace-credit-checkout-submit" type="primary" size="large" block loading={paymentCreating} disabled={!selectedProvider} onClick={() => void startPayment()}>
                                    {selectedProvider ? `确认支付 ¥${(selectedProduct.amountFen / 100).toFixed(2)}` : "暂无可用支付方式"}
                                </Button>
                                <p className="workspace-credit-checkout-note">支付结果确认后，积分将自动到账</p>
                            </>
                        ) : paymentOrder ? (
                            <>
                                <div className="workspace-credit-payment-order">
                                    <div className="workspace-credit-payment-code">
                                        <div className="workspace-credit-payment-brand">
                                            <PaymentProviderIcon providerId={providers.find((provider) => provider.id === paymentOrder.providerId)?.paymentMethod || "online"} />
                                            <strong>{paymentProviderLabel(providers.find((provider) => provider.id === paymentOrder.providerId))}</strong>
                                        </div>
                                        {paymentOrder.status === "credited" ? (
                                            <span className="workspace-credit-payment-success">
                                                <CircleCheck aria-hidden />
                                            </span>
                                        ) : paymentOrder.status === "pending" && paymentOrder.checkout.mode === "qr_code" && paymentOrder.checkout.value ? (
                                            <div className="workspace-credit-payment-qr-frame">
                                                {isPaymentQrImageURL(paymentOrder.checkout.value) ? (
                                                    <img src={paymentOrder.checkout.value} alt={t("account.payment.qrAlt")} width={220} height={220} referrerPolicy="no-referrer" />
                                                ) : (
                                                    <QRCode value={paymentOrder.checkout.value} size={220} color="#111111" bgColor="#FFFFFF" bordered={false} />
                                                )}
                                            </div>
                                        ) : null}
                                        {paymentOrder.status === "credited" ? <strong>支付成功</strong> : paymentOrder.status === "pending" && paymentOrder.checkout.mode !== "qr_code" ? <strong>请在支付页面完成付款</strong> : null}
                                        {paymentOrder.status === "pending" ? (
                                            <span className="workspace-credit-payment-countdown">
                                                <Clock3 aria-hidden />
                                                支付剩余时间 <time>{formatPaymentCountdown(paymentOrder.expiresAt, clock)}</time>
                                            </span>
                                        ) : null}
                                        <p className="workspace-credit-payment-note" role="status">
                                            {paymentOrder.status === "closed"
                                                ? paymentOrder.closurePending
                                                    ? "订单已在站内关闭，渠道结果待确认；若已支付，将按原订单到账"
                                                    : "订单已关闭，未产生积分充值"
                                                : paymentOrder.status === "create_failed"
                                                  ? "支付信息创建失败，请重新生成"
                                                  : paymentOrder.status === "closing"
                                                    ? "渠道正在确认关单结果"
                                                    : "支付完成后，到账状态将自动更新"}
                                        </p>
                                    </div>
                                    <div className="workspace-credit-payment-details">
                                        <div className="workspace-credit-payment-total">
                                            <span>应付金额</span>
                                            <strong>
                                                <small>¥</small>
                                                {(paymentOrder.amountFen / 100).toFixed(2)}
                                            </strong>
                                        </div>
                                        <dl>
                                            <div>
                                                <dt>所选套餐</dt>
                                                <dd>{paymentOrder.productName}</dd>
                                            </div>
                                            <div>
                                                <dt>到账积分</dt>
                                                <dd>
                                                    {formatCredits(paymentOrder.creditsMicrocredits)} 积分{paymentOrder.newcomerBonusMicrocredits ? ` · 新人赠送 ${formatCredits(paymentOrder.newcomerBonusMicrocredits)} 积分` : ""}
                                                </dd>
                                            </div>
                                            <div>
                                                <dt>支付方式</dt>
                                                <dd>
                                                    <PaymentProviderIcon providerId={providers.find((provider) => provider.id === paymentOrder.providerId)?.paymentMethod || "online"} />
                                                    {paymentProviderLabel(providers.find((provider) => provider.id === paymentOrder.providerId))}
                                                </dd>
                                            </div>
                                        </dl>
                                        <div className="workspace-credit-payment-order-number">
                                            <span>订单号</span>
                                            <span>{paymentOrder.merchantOrderNo}</span>
                                        </div>
                                    </div>
                                </div>
                                <footer className="workspace-credit-payment-footer">
                                    {paymentOrder.status === "create_failed" ? (
                                        <Button type="primary" loading={paymentQuerying} onClick={() => void retryPaymentCheckout()}>
                                            重新生成支付信息
                                        </Button>
                                    ) : ["created", "pending", "closing"].includes(paymentOrder.status) ? (
                                        <>
                                            <Button disabled={paymentQuerying} onClick={() => void cancelPayment()}>
                                                关闭订单
                                            </Button>
                                            <Button type="primary" loading={paymentQuerying} onClick={() => void confirmPayment()}>
                                                我已完成支付
                                            </Button>
                                        </>
                                    ) : (
                                        <Button type="primary" onClick={closePaymentSelector}>
                                            完成
                                        </Button>
                                    )}
                                </footer>
                            </>
                        ) : null}
                    </section>
                ) : null}
            </AppModal>
            <AppModal open={redeemOpen} title="兑换码兑换" centered footer={null} onCancel={() => setRedeemOpen(false)}>
                <div className="workspace-credit-panel">
                    <p>输入 32 位兑换码，兑换成功后积分自动到账。</p>
                    <div className="workspace-credit-redeem-row">
                        <Input aria-label="兑换码" autoComplete="off" maxLength={32} value={code} disabled={redeeming} placeholder="输入兑换码" onChange={(event) => setCode(event.target.value)} onPressEnter={() => void redeem()} />
                        <Button type="primary" loading={redeeming} disabled={normalizedCode.length !== 32} onClick={() => void redeem()}>
                            兑换
                        </Button>
                    </div>
                    <Button
                        type="text"
                        onClick={() => {
                            setRedeemOpen(false);
                            setProductsOpen(true);
                        }}
                    >
                        返回套餐中心
                    </Button>
                </div>
            </AppModal>
        </ConfigProvider>
    );
}

function PaymentProviderIcon({ providerId, large = false }: { providerId: string; large?: boolean }) {
    const className = large ? "workspace-credit-provider-icon is-large" : "workspace-credit-provider-icon";
    if (isWechatProvider(providerId))
        return (
            <span className={`${className} is-wechat`} aria-hidden>
                <WechatFilled />
            </span>
        );
    if (isAlipayProvider(providerId))
        return (
            <span className={`${className} is-alipay`} aria-hidden>
                <AlipayCircleFilled />
            </span>
        );
    return <CreditCard className={className} aria-hidden />;
}

function paymentProviderLabel(provider?: PaymentProvider) {
    if (provider?.paymentMethod === "wechat") return "微信支付";
    if (provider?.paymentMethod === "alipay") return "支付宝支付";
    return "在线支付";
}

function isWechatProvider(providerId: string) {
    return providerId.toLowerCase().includes("wechat");
}

function isAlipayProvider(providerId: string) {
    return providerId.toLowerCase().includes("alipay");
}

function formatPaymentCountdown(expiresAt: string, now: number) {
    const seconds = Math.max(0, Math.floor((new Date(expiresAt).getTime() - now) / 1000));
    const minutes = Math.floor(seconds / 60);
    return `${String(minutes).padStart(2, "0")}:${String(seconds % 60).padStart(2, "0")}`;
}
