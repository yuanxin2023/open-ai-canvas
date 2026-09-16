import { AlipayCircleFilled, WechatFilled } from "@ant-design/icons";
import { App, Button, Input, Popover, QRCode, Skeleton } from "antd";
import { Check, CircleCheck, CreditCard, Gift, Headphones, Sparkles } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";

import { AppModal } from "@/components/ui/product/app-modal";
import { formatCredits } from "@/constant/credits";
import { useWalletBalance } from "@/hooks/use-wallet-balance";
import { createPaymentOrder, getPaymentOrder, listPaymentProviders, listTopupProducts, queryPaymentOrder, refreshPaymentCheckout, type PaymentOrder, type PaymentProvider, type TopupProduct } from "@/services/api/payments";
import { redeemCredits } from "@/services/api/wallet";

export function WorkspaceCreditPopover({ userId }: { userId: string }) {
    const { message } = App.useApp();
    const { availableMicrocredits } = useWalletBalance(userId);
    const [open, setOpen] = useState(false);
    const [productsOpen, setProductsOpen] = useState(false);
    const [products, setProducts] = useState<TopupProduct[]>([]);
    const [providers, setProviders] = useState<PaymentProvider[]>([]);
    const [productsLoading, setProductsLoading] = useState(false);
    const [productsError, setProductsError] = useState("");
    const [productsReloadKey, setProductsReloadKey] = useState(0);
    const [selectedProduct, setSelectedProduct] = useState<TopupProduct | null>(null);
    const [selectedProviderId, setSelectedProviderId] = useState("");
    const [paymentOrder, setPaymentOrder] = useState<PaymentOrder | null>(null);
    const [paymentCreating, setPaymentCreating] = useState(false);
    const [paymentQuerying, setPaymentQuerying] = useState(false);
    const [clock, setClock] = useState(Date.now());
    const [code, setCode] = useState("");
    const [redeeming, setRedeeming] = useState(false);
    const paymentIdempotencyKey = useRef("");
    const completedPaymentOrderId = useRef("");
    const balance = availableMicrocredits === null ? "--" : formatCredits(availableMicrocredits);
    const normalizedCode = code.trim().toLowerCase();
    const selectedProvider = useMemo(() => providers.find((provider) => provider.id === selectedProviderId), [providers, selectedProviderId]);

    useEffect(() => {
        if (!productsOpen) return;
        let active = true;
        setProductsLoading(true);
        setProductsError("");
        void Promise.all([listTopupProducts(), listPaymentProviders()])
            .then(([productsResult, providersResult]) => {
                if (!active) return;
                setProducts(productsResult.products);
                setProviders(providersResult.providers);
                setSelectedProviderId((current) => providersResult.providers.some((provider) => provider.id === current) ? current : providersResult.providers[0]?.id || "");
            })
            .catch((error) => {
                if (active) setProductsError(error instanceof Error ? error.message : "读取商品套餐失败");
            })
            .finally(() => {
                if (active) setProductsLoading(false);
            });
        return () => {
            active = false;
        };
    }, [productsOpen, productsReloadKey]);

    useEffect(() => {
        paymentIdempotencyKey.current = "";
    }, [selectedProduct?.id, selectedProviderId]);

    useEffect(() => {
        if (!selectedProduct || !paymentOrder || !["created", "pending", "closing"].includes(paymentOrder.status)) return;
        const interval = window.setInterval(() => {
            setClock(Date.now());
            void getPaymentOrder(paymentOrder.id)
                .then(({ order }) => {
                    setPaymentOrder(order);
                    if (order.status === "credited") paymentCompleted(order.id);
                })
                .catch(() => undefined);
        }, 2_000);
        return () => window.clearInterval(interval);
    }, [selectedProduct?.id, paymentOrder?.id, paymentOrder?.status]);

    const openPaymentSelector = (product: TopupProduct) => {
        setSelectedProduct(product);
        setSelectedProviderId((current) => providers.some((provider) => provider.id === current) ? current : providers[0]?.id || "");
        setPaymentOrder(null);
        setClock(Date.now());
    };

    const closePaymentSelector = () => {
        if (paymentCreating || paymentQuerying) return;
        setSelectedProduct(null);
        setPaymentOrder(null);
    };

    const startPayment = async () => {
        if (!selectedProduct || !selectedProvider) {
            message.error("请选择支付方式");
            return;
        }
        setPaymentCreating(true);
        try {
            if (!paymentIdempotencyKey.current) paymentIdempotencyKey.current = crypto.randomUUID();
            const result = await createPaymentOrder({ productId: selectedProduct.id, providerId: selectedProvider.id, idempotencyKey: paymentIdempotencyKey.current });
            paymentIdempotencyKey.current = "";
            setPaymentOrder(result.order);
            if (result.order.status === "credited") {
                paymentCompleted(result.order.id);
                return;
            }
            if (result.order.checkout.mode === "redirect" && result.order.checkout.url) {
                window.location.assign(result.order.checkout.url);
            }
        } catch (error) {
            message.error(error instanceof Error ? error.message : "创建支付订单失败");
        } finally {
            setPaymentCreating(false);
        }
    };

    const confirmPayment = async () => {
        if (!paymentOrder) return;
        setPaymentQuerying(true);
        try {
            const result = await queryPaymentOrder(paymentOrder.id);
            setPaymentOrder(result.order);
            if (result.order.status === "credited") paymentCompleted(result.order.id);
            else if (result.order.status === "closed") message.warning("订单已关闭，未产生积分充值");
            else message.info("支付渠道尚未确认，请稍后再试");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "查询支付结果失败");
        } finally {
            setPaymentQuerying(false);
        }
    };

    const retryPaymentCheckout = async () => {
        if (!paymentOrder) return;
        setPaymentQuerying(true);
        try {
            const result = await refreshPaymentCheckout(paymentOrder.id);
            setPaymentOrder(result.order);
            if (result.order.status === "credited") paymentCompleted(result.order.id);
            else if (result.order.checkout.mode === "redirect" && result.order.checkout.url) window.location.assign(result.order.checkout.url);
        } catch (error) {
            message.error(error instanceof Error ? error.message : "重新生成支付信息失败");
        } finally {
            setPaymentQuerying(false);
        }
    };

    const paymentCompleted = (orderId: string) => {
        if (completedPaymentOrderId.current === orderId) return;
        completedPaymentOrderId.current = orderId;
        window.dispatchEvent(new CustomEvent("wallet:updated"));
        message.success("支付成功，积分已到账");
    };

    const redeem = async () => {
        if (normalizedCode.length !== 32) {
            message.error("请输入完整的 32 位兑换码");
            return;
        }
        setRedeeming(true);
        try {
            await redeemCredits(normalizedCode);
            setCode("");
            window.dispatchEvent(new CustomEvent("wallet:updated"));
            message.success("兑换成功，积分已到账");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "兑换失败");
        } finally {
            setRedeeming(false);
        }
    };

    return (
        <>
            <Popover
            trigger="click"
            placement="bottomRight"
            rootClassName="workspace-credit-popover"
            open={open}
            onOpenChange={setOpen}
            content={(
                <div className="workspace-credit-panel">
                    <div className="workspace-credit-panel-heading">
                        <div>
                            <div className="workspace-credit-panel-title">我的积分</div>
                            <div className="workspace-credit-panel-description">兑换后余额将自动刷新</div>
                        </div>
                        <div className="workspace-credit-balance-chip" aria-label={`当前积分余额 ${balance}`}>
                            <Sparkles aria-hidden />
                            <span>余额</span>
                            <strong>{balance}</strong>
                        </div>
                    </div>

                    <div className="workspace-credit-redeem-heading">
                        <span><Gift aria-hidden />兑换码兑换</span>
                        <small>32 位兑换码</small>
                    </div>
                    <div className="workspace-credit-redeem-row">
                        <Input
                            aria-label="兑换码"
                            autoComplete="off"
                            maxLength={32}
                            placeholder="输入兑换码"
                            value={code}
                            disabled={redeeming}
                            onChange={(event) => setCode(event.target.value)}
                            onPressEnter={() => void redeem()}
                        />
                        <Button type="primary" loading={redeeming} disabled={normalizedCode.length !== 32} onClick={() => void redeem()}>
                            兑换
                        </Button>
                    </div>
                    <Button
                        className="workspace-credit-purchase-button"
                        type="primary"
                        icon={<CreditCard aria-hidden />}
                        onClick={() => {
                            setOpen(false);
                            setProductsOpen(true);
                        }}
                    >
                        购买积分与套餐
                    </Button>
                </div>
            )}
        >
            <button
                type="button"
                className="app-workspace-credit-button"
                aria-label={availableMicrocredits === null ? "查看积分余额" : `积分余额 ${balance}`}
                title="查看积分余额"
            >
                <Sparkles aria-hidden />
                <span>{balance}</span>
            </button>
            </Popover>

            <AppModal
                flush
                centered
                open={productsOpen}
                title={null}
                footer={null}
                width="min(1240px, calc(100vw - 24px))"
                rootClassName="workspace-credit-products-modal"
                onCancel={() => setProductsOpen(false)}
            >
                <section className="workspace-credit-products-shell">
                    <header className="workspace-credit-products-header">
                        <h2>购买积分与套餐</h2>
                        <p>选择管理员已上架的积分商品，支付成功后积分将自动到账。</p>
                    </header>

                    {productsLoading ? (
                        <div className="workspace-credit-products-grid" aria-label="正在加载商品套餐">
                            {Array.from({ length: 3 }, (_, index) => <Skeleton.Node key={index} active className="workspace-credit-product-skeleton" />)}
                        </div>
                    ) : productsError ? (
                        <div className="workspace-credit-products-state">
                            <strong>商品套餐加载失败</strong>
                            <span>{productsError}</span>
                            <Button onClick={() => setProductsReloadKey((current) => current + 1)}>重新加载</Button>
                        </div>
                    ) : (
                        <div className="workspace-credit-products-grid">
                            {products.map((product) => (
                                <article key={product.id} className="workspace-credit-product-card">
                                    <div className="workspace-credit-product-card-heading">
                                        <Sparkles aria-hidden />
                                        <h3>{product.name}</h3>
                                    </div>
                                    <div className="workspace-credit-product-price">
                                        <small>¥</small>
                                        <strong>{(product.amountFen / 100).toFixed(2)}</strong>
                                    </div>
                                    <p className="workspace-credit-product-description">{product.description || "管理员配置的积分充值商品"}</p>
                                    <div className="workspace-credit-product-credits">
                                        <Sparkles aria-hidden />
                                        <strong>{formatCredits(product.creditsMicrocredits)} 积分</strong>
                                    </div>
                                    <div className="workspace-credit-product-facts">
                                        <span><Check aria-hidden />按管理员配置金额结算</span>
                                        <span><Check aria-hidden />支付成功后积分自动到账</span>
                                    </div>
                                    <Button type="primary" block disabled={!providers.length} onClick={() => openPaymentSelector(product)}>{providers.length ? "选择套餐" : "暂无可用支付方式"}</Button>
                                </article>
                            ))}

                            <article className="workspace-credit-product-card workspace-credit-contact-card">
                                <div className="workspace-credit-product-card-heading">
                                    <Headphones aria-hidden />
                                    <h3>企业套餐</h3>
                                </div>
                                <div className="workspace-credit-contact-title">联系客服</div>
                                <p className="workspace-credit-product-description">定制服务</p>
                                <div className="workspace-credit-product-facts">
                                    <span><Check aria-hidden />企业用量与能力可单独报价</span>
                                    <span><Check aria-hidden />支持合同、对公与开票</span>
                                    <span><Check aria-hidden />提供企业内训和业务陪跑</span>
                                    <span><Check aria-hidden />资产存储数量与权限可按需定制</span>
                                </div>
                                <Button block onClick={() => message.info("客服联系方式将在后续配置")}>联系客服</Button>
                            </article>
                        </div>
                    )}
                </section>
            </AppModal>

            <AppModal
                flush
                centered
                open={Boolean(selectedProduct)}
                title={null}
                footer={null}
                width="min(760px, calc(100vw - 24px))"
                rootClassName="workspace-credit-checkout-modal"
                maskClosable={!paymentCreating && !paymentQuerying}
                onCancel={closePaymentSelector}
            >
                {selectedProduct ? (
                    <section className="workspace-credit-checkout-shell">
                        <header className="workspace-credit-checkout-header">
                            <div>
                                <h2>{paymentOrder ? "完成支付" : "选择支付方式"}</h2>
                                <p>{paymentOrder ? "请按收银台提示完成付款，到账状态将自动更新。" : "仅显示管理员已启用并配置完成的支付方式。"}</p>
                            </div>
                        </header>

                        {!paymentOrder ? (
                            <>
                                <div className="workspace-credit-provider-options" role="radiogroup" aria-label="支付方式">
                                    {providers.map((provider) => (
                                        <button
                                            key={provider.id}
                                            type="button"
                                            role="radio"
                                            aria-checked={selectedProviderId === provider.id}
                                            className="workspace-credit-provider-option"
                                            onClick={() => setSelectedProviderId(provider.id)}
                                        >
                                            <PaymentProviderIcon providerId={provider.id} />
                                            <span>{paymentProviderLabel(provider)}</span>
                                            <Check aria-hidden />
                                        </button>
                                    ))}
                                </div>

                                <div className="workspace-credit-checkout-summary">
                                    <div><span>所选套餐</span><strong>{selectedProduct.name}</strong></div>
                                    <div><span>到账积分</span><strong>{formatCredits(selectedProduct.creditsMicrocredits)} 积分</strong></div>
                                    <div><span>应付金额</span><strong className="workspace-credit-checkout-amount">¥ {(selectedProduct.amountFen / 100).toFixed(2)}</strong></div>
                                </div>

                                <Button type="primary" size="large" block loading={paymentCreating} disabled={!selectedProvider} onClick={() => void startPayment()}>
                                    {selectedProvider ? `使用${paymentProviderLabel(selectedProvider)}` : "暂无可用支付方式"}
                                </Button>
                            </>
                        ) : (
                            <div className="workspace-credit-payment-order">
                                <div className="workspace-credit-payment-code">
                                    {paymentOrder.status === "credited" ? (
                                        <span className="workspace-credit-payment-success"><CircleCheck aria-hidden /></span>
                                    ) : paymentOrder.checkout.mode === "qr_code" && paymentOrder.checkout.value ? (
                                        <QRCode value={paymentOrder.checkout.value} size={220} bordered={false} />
                                    ) : null}
                                    <strong>{paymentOrder.status === "credited" ? "支付成功" : paymentOrder.checkout.mode === "qr_code" ? `请使用${paymentProviderLabelById(paymentOrder.providerId)}扫码` : "请在支付页面完成付款"}</strong>
                                    {paymentOrder.status === "pending" ? <span>剩余支付时间 {formatPaymentCountdown(paymentOrder.expiresAt, clock)}</span> : null}
                                </div>
                                <div className="workspace-credit-payment-details">
                                    <PaymentProviderIcon providerId={paymentOrder.providerId} large />
                                    <h3>{paymentOrder.productName}</h3>
                                    <div className="workspace-credit-payment-total"><span>总计</span><strong>¥ {(paymentOrder.amountFen / 100).toFixed(2)}</strong></div>
                                    <dl>
                                        <div><dt>支付方式</dt><dd>{paymentProviderLabelById(paymentOrder.providerId)}</dd></div>
                                        <div><dt>到账积分</dt><dd>{formatCredits(paymentOrder.creditsMicrocredits)} 积分</dd></div>
                                        <div><dt>订单号</dt><dd>{paymentOrder.merchantOrderNo}</dd></div>
                                    </dl>
                                    {paymentOrder.status === "create_failed" ? <Button type="primary" loading={paymentQuerying} onClick={() => void retryPaymentCheckout()}>重新生成支付信息</Button> : paymentOrder.status === "pending" ? <Button type="primary" loading={paymentQuerying} onClick={() => void confirmPayment()}>我已完成支付</Button> : <Button type="primary" onClick={closePaymentSelector}>完成</Button>}
                                </div>
                            </div>
                        )}
                    </section>
                ) : null}
            </AppModal>
        </>
    );
}

function PaymentProviderIcon({ providerId, large = false }: { providerId: string; large?: boolean }) {
    const className = large ? "workspace-credit-provider-icon is-large" : "workspace-credit-provider-icon";
    if (isWechatProvider(providerId)) return <WechatFilled className={`${className} is-wechat`} aria-hidden />;
    if (isAlipayProvider(providerId)) return <AlipayCircleFilled className={`${className} is-alipay`} aria-hidden />;
    return <CreditCard className={className} aria-hidden />;
}

function paymentProviderLabel(provider: PaymentProvider) {
    if (isWechatProvider(provider.id)) return "微信支付";
    if (isAlipayProvider(provider.id)) return "支付宝支付";
    return provider.name;
}

function paymentProviderLabelById(providerId: string) {
    if (isWechatProvider(providerId)) return "微信支付";
    if (isAlipayProvider(providerId)) return "支付宝支付";
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
