import { Button } from "antd";
import { Check } from "lucide-react";

import { formatCredits } from "@/constant/credits";

export type CreditProductCardData = {
    id: string;
    name: string;
    description?: string;
    benefits?: string;
    amountFen: number;
    creditsMicrocredits: number;
};

type CreditProductCardProps = {
    product: CreditProductCardData;
    isBestValue?: boolean;
    maxCreditsMicrocredits: number;
    actionLabel?: string;
    actionDisabled?: boolean;
    onAction?: () => void;
    preview?: boolean;
};

export function CreditProductCard({ product, isBestValue = false, maxCreditsMicrocredits, actionLabel = "立即购买", actionDisabled = false, onAction, preview = false }: CreditProductCardProps) {
    const benefits = splitProductBenefits(product.benefits);
    const activeSegments = creditProductActiveSegments(product.creditsMicrocredits, maxCreditsMicrocredits);

    return (
        <article className={`workspace-credit-product-card ${isBestValue ? "is-best-value" : ""}`}>
            <div className="workspace-credit-product-card-heading">
                <h3>{product.name || "套餐名称"}</h3>
                {isBestValue ? <span>积分更划算</span> : null}
            </div>
            <div className="workspace-credit-product-price">
                <small>¥</small>
                <strong>{formatProductPrice(product.amountFen)}</strong>
            </div>
            <p className="workspace-credit-product-description">{product.description || "管理员配置的积分充值商品"}</p>
            <Button className="workspace-credit-product-action" type={isBestValue ? "primary" : "default"} block disabled={actionDisabled} tabIndex={preview ? -1 : undefined} aria-disabled={preview || actionDisabled} onClick={preview ? undefined : onAction}>
                {actionLabel}
            </Button>
            <div className="workspace-credit-product-quota">
                <strong>包含 {formatCredits(product.creditsMicrocredits)} 积分</strong>
                <span>用于图片、视频与文本等创作任务</span>
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

export function findBestValueProductId(products: CreditProductCardData[]) {
    if (products.length < 2) return "";
    return products.reduce((best, product) => {
        const productValue = product.amountFen > 0 ? product.creditsMicrocredits / product.amountFen : 0;
        const bestValue = best.amountFen > 0 ? best.creditsMicrocredits / best.amountFen : 0;
        return productValue > bestValue || (productValue === bestValue && product.creditsMicrocredits > best.creditsMicrocredits) ? product : best;
    }).id;
}
