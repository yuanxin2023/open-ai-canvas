import { Button } from "antd";
import { Check } from "lucide-react";
import type { CSSProperties } from "react";

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
    accentColor?: string;
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
    const accentColor = normalizeProductAccentColor(product.accentColor);
    const accentForeground = productAccentForeground(accentColor);
    const ribbonText = product.ribbonText || (featured ? "推荐套餐" : "");
    const priceCaption = product.priceCaption || `每 100 积分约 ¥${formatUnitPrice(product.amountFen, product.creditsMicrocredits)}`;
    const cardStyle = {
        "--workspace-credit-product-accent": accentColor,
        "--workspace-credit-product-accent-foreground": accentForeground,
    } as CSSProperties;

    return (
        <article className={`workspace-credit-product-card ${featured ? "is-featured" : ""}`} style={cardStyle}>
            {ribbonText ? <div className="workspace-credit-product-ribbon">{ribbonText}</div> : null}
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
                <Button className="workspace-credit-product-action" type="primary" block disabled={actionDisabled} tabIndex={preview ? -1 : undefined} aria-disabled={preview || actionDisabled} onClick={preview ? undefined : onAction}>
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

export function normalizeProductAccentColor(value?: string) {
    const normalized = value?.trim().toUpperCase();
    return normalized && /^#[0-9A-F]{6}$/.test(normalized) ? normalized : "#D8FF4F";
}

export function productAccentForeground(accentColor: string) {
    const normalized = normalizeProductAccentColor(accentColor);
    const red = Number.parseInt(normalized.slice(1, 3), 16);
    const green = Number.parseInt(normalized.slice(3, 5), 16);
    const blue = Number.parseInt(normalized.slice(5, 7), 16);
    return (red * 299 + green * 587 + blue * 114) / 1000 >= 150 ? "#111111" : "#FFFFFF";
}
