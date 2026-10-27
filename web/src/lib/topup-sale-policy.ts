import type { TopupProduct } from "@/services/api/payments";

type SaleInfo = Pick<TopupProduct, "saleStrategy" | "saleStatus" | "stockRemaining" | "periodDays" | "periodPurchaseLimit" | "saleStartAt">;

export function topupSaleDescription(product: SaleInfo): string {
    if (product.saleStatus === "disabled") return "已停用";
    if (product.saleStatus === "unavailable") return "暂不可购买";
    if (product.saleStatus === "upcoming") {
        const start = product.saleStartAt ? new Date(product.saleStartAt) : null;
        return start && Number.isFinite(start.getTime()) ? `发售时间：${start.toLocaleString("zh-CN", { hour12: false })}` : "待发售";
    }
    if (product.saleStatus === "ended") return "已结束";
    if (product.saleStatus === "sold_out") return "已售罄";
    if (product.saleStrategy === "inventory") return `剩余库存 ${product.stockRemaining ?? 0}`;
    if (product.saleStrategy === "periodic") return `滚动 ${product.periodDays} 天限购 ${product.periodPurchaseLimit} 次`;
    if (product.saleStrategy === "timed") return "限时发售";
    return "不限量";
}
