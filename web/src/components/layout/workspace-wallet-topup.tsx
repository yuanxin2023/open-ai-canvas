import { translate } from "@/i18n";
import { Button, Skeleton } from "antd";
import { CircleAlert, Coins, CreditCard } from "lucide-react";
import { useEffect, useRef, type ReactNode } from "react";

import { CreditProductCard } from "@/components/payments/credit-product-card";
import { formatCredits } from "@/constant/credits";
import { cn } from "@/lib/utils";
import { referralCredits } from "@/lib/referral-policy";
import type { PaymentProvider, TopupProduct, TopupProductKind } from "@/services/api/payments";
import type { ReferralPolicy } from "@/services/api/referrals";

import "./workspace-wallet-topup.css";

export type TopupCategory = "all" | TopupProductKind;

const KIND_META: Record<TopupProductKind, { label: string; description: string; icon: typeof Coins }> = {
    credits: { label: translate("account.layout.checkin.offerAfter"), description: translate("account.wallet.topup.kinds.credits.description"), icon: Coins },
};

function productAvailability(product: TopupProduct) {
    if (product.saleStatus === "upcoming") return `${product.saleStartAt ? new Date(product.saleStartAt).toLocaleString("zh-CN", { hour12: false, month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" }) : "待定"} 开售`;
    if (product.saleStatus === "ended") return "已结束";
    if (product.saleStatus === "sold_out") return "已售罄";
    if (product.saleStrategy === "inventory") return `剩余 ${product.stockRemaining ?? 0} 份`;
    if (product.saleStrategy === "periodic") return `每 ${product.periodDays} 天限购 ${product.periodPurchaseLimit} 次`;
    if (product.saleStrategy === "timed" && product.saleEndAt) return `${new Date(product.saleEndAt).toLocaleString("zh-CN", { hour12: false, month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" })} 截止`;
    return "";
}

/** 选中商品的权益摘要，结算栏与卡片共用。 */
export function topupProductValue(product: TopupProduct) {
    return formatCredits(product.creditsMicrocredits, 6) + " 积分";
}

function TopupGroup({ kind, products, selectedProductId, onSelectProduct, aside, children }: { kind: TopupProductKind; products: TopupProduct[]; selectedProductId: string; onSelectProduct: (id: string) => void; aside?: ReactNode; children?: ReactNode }) {
    const meta = KIND_META[kind];
    const Icon = meta.icon;
    return (
        <section className={cn("wallet-topup-group", `is-${kind}`)} data-topup-kind={kind} aria-labelledby={`wallet-topup-group-${kind}`}>
            <header className="wallet-topup-group-head">
                <span className="wallet-topup-kind-icon is-small" aria-hidden="true">
                    <Icon />
                </span>
                <div>
                    <h3 id={`wallet-topup-group-${kind}`}>
                        {meta.label}
                        {translate("local.ui.tier")}
                    </h3>
                    <p title={meta.description}>{meta.description}</p>
                </div>
                {aside}
            </header>
            {children}
            {products.length ? (
                <div className="wallet-topup-grid">
                    {products.map((product) => (
                        <CreditProductCard
                            key={product.id}
                            product={product}
                            maxCreditsMicrocredits={Math.max(...products.map((item) => item.creditsMicrocredits))}
                            actionLabel={selectedProductId === product.id ? translate("local.ui.selected") : translate("local.ui.selectTier")}
                            availabilityLabel={productAvailability(product)}
                            actionDisabled={!product.canPurchase}
                            onAction={() => onSelectProduct(product.id)}
                        />
                    ))}
                </div>
            ) : (
                <div className="wallet-topup-group-empty">
                    {translate("local.ui.noProducts")}
                    {meta.label}
                    {translate("local.ui.tier")}
                </div>
            )}
        </section>
    );
}

export function WalletTopupPanel({
    newcomerPolicy,
    loading,
    products,
    providers,
    focusKind,
    focusToken,
    selectedProduct,
    onSelectProduct,
    selectedProviderId,
    onSelectProvider,
    creating,
    onPay,
}: {
    newcomerPolicy?: ReferralPolicy;
    loading: boolean;
    products: TopupProduct[];
    providers: PaymentProvider[];
    /** 定位积分商品分组；focusToken 变化时重新定位。 */
    focusKind?: TopupProductKind;
    focusToken?: number;
    selectedProduct?: TopupProduct;
    onSelectProduct: (id: string) => void;
    selectedProviderId: string;
    onSelectProvider: (id: string) => void;
    creating: boolean;
    onPay: () => void;
}) {
    const groupsRef = useRef<HTMLDivElement>(null);

    useEffect(() => {
        if (loading || !focusKind) return;
        const container = groupsRef.current;
        const target = container?.querySelector<HTMLElement>(`[data-topup-kind="${focusKind}"]`);
        if (!container || !target) return;
        // 只滚动分组列表本身。scrollIntoView 会连带滚动 overflow: hidden 的祖先（弹窗外壳、内容区），
        // 把页头和页签推出可视区且无法滚回。
        const top = target.getBoundingClientRect().top - container.getBoundingClientRect().top + container.scrollTop;
        container.scrollTo({ top, behavior: window.matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth" });
    }, [loading, focusKind, focusToken]);

    if (loading) {
        return (
            <div className="workspace-wallet-content wallet-topup">
                <Skeleton active paragraph={{ rows: 6 }} />
            </div>
        );
    }

    const byKind = { credits: products };
    const kinds = ["credits"] as const;
    const selectedKind = selectedProduct ? KIND_META[selectedProduct.kind] : null;
    const SelectedIcon = selectedKind?.icon;

    return (
        <div className="workspace-wallet-content wallet-topup">
            <div ref={groupsRef} className="wallet-topup-groups">
                {kinds.map((kind) => (
                    <TopupGroup key={kind} kind={kind} products={byKind[kind]} selectedProductId={selectedProduct?.id || ""} onSelectProduct={onSelectProduct}></TopupGroup>
                ))}
            </div>

            {products.length ? (
                <footer className="wallet-topup-checkout">
                    <div className={cn("wallet-topup-checkout-summary", selectedProduct && `is-${selectedProduct.kind}`)}>
                        {selectedProduct && SelectedIcon ? (
                            <>
                                <span className="wallet-topup-kind-icon" aria-hidden="true">
                                    <SelectedIcon />
                                </span>
                                <span>
                                    <strong>{selectedProduct.name}</strong>
                                    <small>
                                        {topupProductValue(selectedProduct)}
                                        {selectedProduct.kind === "credits" && newcomerPolicy
                                            ? ` · 邀请新人首充预计另赠 ${formatCredits(Math.min(referralCredits(selectedProduct.creditsMicrocredits, newcomerPolicy.newcomerRateBps), newcomerPolicy.newcomerCapMicrocredits || Number.MAX_SAFE_INTEGER))} 积分`
                                            : ""}
                                    </small>
                                </span>
                                <b>¥ {(selectedProduct.amountFen / 100).toFixed(2)}</b>
                            </>
                        ) : (
                            <span className="wallet-topup-checkout-hint">{translate("account.wallet.topup.selectPlan")}</span>
                        )}
                    </div>
                    {providers.length ? (
                        <div className="workspace-wallet-providers" role="radiogroup" aria-label={translate("account.wallet.topup.paymentMethod")}>
                            {providers.map((provider) => (
                                <button key={provider.id} type="button" role="radio" aria-checked={selectedProviderId === provider.id} className={selectedProviderId === provider.id ? "is-selected" : ""} onClick={() => onSelectProvider(provider.id)}>
                                    <CreditCard />
                                    {provider.name}
                                </button>
                            ))}
                        </div>
                    ) : (
                        <span className="wallet-topup-checkout-hint">{translate("account.wallet.topup.noProvider")}</span>
                    )}
                    <Button type="primary" size="large" loading={creating} disabled={!selectedProduct || !selectedProduct.canPurchase || !providers.length || !selectedProviderId} onClick={onPay}>
                        {translate("account.wallet.topup.payNow")}
                    </Button>
                </footer>
            ) : (
                <div className="workspace-wallet-inline-state">
                    <CircleAlert />
                    <div>
                        <strong>{translate("account.wallet.topup.unavailableTitle")}</strong>
                        <span>{translate("account.wallet.topup.unavailableDescription")}</span>
                    </div>
                </div>
            )}
        </div>
    );
}
