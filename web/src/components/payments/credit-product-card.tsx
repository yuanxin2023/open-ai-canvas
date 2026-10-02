import { Button } from "antd";
import { Check } from "lucide-react";

import { formatCredits } from "@/constant/credits";

export type CreditProductCardData = {
    id: string;
    name: string;
    description?: string;
    benefits?: string;
    ribbonText?: string;
    badgeText?: string;
    compareAmountFen?: number;
    priceCaption?: string;
    quotaCaption?: string;
    quotaDetail?: string;
    actionText?: string;
    featured?: boolean;
    amountFen: number;
    creditsMicrocredits: number;
};

type CreditProductCardProps = {
    product: CreditProductCardData;
    maxCreditsMicrocredits: number;
    actionLabel?: string;
    actionDisabled?: boolean;
    onAction?: () => void;
    preview?: boolean;
};

export function CreditProductCard({ product, maxCreditsMicrocredits, actionLabel, actionDisabled = false, onAction, preview = false }: CreditProductCardProps) {
    const benefits = splitProductBenefits(product.benefits);
    const activeSegments = creditProductActiveSegments(product.creditsMicrocredits, maxCreditsMicrocredits);
    const featured = Boolean(product.featured);
    const priceCaption = product.priceCaption || `每 100 积分约 ¥${formatUnitPrice(product.amountFen, product.creditsMicrocredits)}`;

    return (
        <article className={`workspace-credit-product-card ${featured ? "is-featured" : ""}`}>
            {product.ribbonText ? <div className="workspace-credit-product-ribbon">{product.ribbonText}</div> : null}
            <div className="workspace-credit-product-main">
                <div className="workspace-credit-product-card-heading">
                    <h3>{product.name || "套餐名称"}</h3>
                    {product.badgeText ? <span>{product.badgeText}</span> : null}
                </div>
                <div className="workspace-credit-product-price">
                    <small>¥</small>
                    <strong>{formatProductPrice(product.amountFen)}</strong>
                    {product.compareAmountFen ? <del>¥{formatProductPrice(product.compareAmountFen)}</del> : null}
                </div>
                {product.description ? <p className="workspace-credit-product-description">{product.description}</p> : null}
                <p className="workspace-credit-product-caption">{priceCaption}</p>
                <Button className="workspace-credit-product-action" type={featured ? "primary" : "default"} block disabled={actionDisabled} tabIndex={preview ? -1 : undefined} aria-disabled={preview || actionDisabled} onClick={preview ? undefined : onAction}>
                    {actionLabel || product.actionText || "立即购买"}
                </Button>
            </div>
            <div className="workspace-credit-product-quota">
                <strong>{product.quotaCaption || "到账积分"} {formatCredits(product.creditsMicrocredits)}</strong>
                {product.quotaDetail ? <span>{product.quotaDetail}</span> : null}
            </div>
            <div className="workspace-credit-product-meter" aria-hidden="true">
                {Array.from({ length: 28 }, (_, index) => <i key={index} className={index < activeSegments ? "is-active" : ""} />)}
            </div>
            {benefits.length ? (
                <div className="workspace-credit-product-facts">
                    {benefits.map((benefit, index) => <span key={`${product.id}-benefit-${index}`}><Check aria-hidden />{benefit}</span>)}
                </div>
            ) : null}
        </article>
    );
}

export function creditProductActiveSegments(creditsMicrocredits: number, maxCreditsMicrocredits: number, segmentCount = 28) {
    if (creditsMicrocredits <= 0 || maxCreditsMicrocredits <= 0 || segmentCount <= 0) return 0;
    return Math.min(segmentCount, Math.max(1, Math.round((creditsMicrocredits / maxCreditsMicrocredits) * segmentCount)));
}

export function splitProductBenefits(benefits?: string) {
    return (benefits || "").split(/\r?\n/).map((item) => item.trim()).filter(Boolean);
}

export function formatProductPrice(amountFen: number) {
    const amount = amountFen / 100;
    return Number.isInteger(amount) ? amount.toFixed(0) : amount.toFixed(2);
}

export function formatUnitPrice(amountFen: number, creditsMicrocredits: number) {
    if (creditsMicrocredits <= 0) return "--";
    return (amountFen * 1_000_000 / creditsMicrocredits).toFixed(2);
}
