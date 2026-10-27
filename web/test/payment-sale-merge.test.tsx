import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { topupSaleDescription } from "../src/lib/topup-sale-policy";
import { createAdminTopupProduct, createPaymentOrder, type TopupProduct, type TopupProductInput } from "../src/services/api/payments";
import { apiClient } from "../src/services/api/request";

const product: TopupProduct = {
    id: "product",
    name: "Local package",
    benefits: "Existing benefit",
    ribbonText: "Local ribbon",
    badgeText: "Popular",
    compareAmountFen: 200,
    priceCaption: "Local price",
    quotaCaption: "到账积分",
    quotaDetail: "Local quota",
    actionText: "站内购买",
    accentColor: "#123456",
    featured: true,
    amountFen: 100,
    creditsMicrocredits: 100_000_000,
    enabled: true,
    sortOrder: 0,
    saleStrategy: "unlimited",
    canPurchase: true,
    createdBy: "admin",
    updatedBy: "admin",
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
};

describe("merged product sale policy", () => {
    test("describes stock, rolling limits and timed availability", () => {
        expect(topupSaleDescription(product)).toBe("不限量");
        expect(topupSaleDescription({ ...product, saleStrategy: "periodic", periodDays: 7, periodPurchaseLimit: 2 })).toBe("滚动 7 天限购 2 次");
        expect(topupSaleDescription({ ...product, saleStrategy: "inventory", stockRemaining: 3 })).toBe("剩余库存 3");
        expect(topupSaleDescription({ ...product, saleStrategy: "inventory", saleStatus: "sold_out", stockRemaining: -1 })).toBe("已售罄");
        expect(topupSaleDescription({ ...product, saleStrategy: "timed", saleStatus: "upcoming" })).toBe("待发售");
        expect(topupSaleDescription({ ...product, saleStrategy: "timed", saleStatus: "ended" })).toBe("已结束");
        expect(topupSaleDescription({ ...product, saleStatus: "disabled" })).toBe("已停用");
        expect(topupSaleDescription({ ...product, saleStatus: "unavailable" })).toBe("暂不可购买");
    });

    test("unavailable sale keeps local card content and disables purchase", async () => {
        const { CreditProductCard } = await import("../src/components/payments/credit-product-card");
        const html = renderToStaticMarkup(<CreditProductCard product={product} maxCreditsMicrocredits={product.creditsMicrocredits} availabilityLabel="已售罄" actionLabel="暂不可购买" actionDisabled />);
        for (const text of ["Existing benefit", "Local ribbon", "Popular", "Local price", "Local quota", "已售罄", "暂不可购买"]) expect(html).toContain(text);
        expect(html).toMatch(/<button[^>]+disabled/);
    });

    test("admin sends both local presentation fields and official policy", async () => {
        const adapter = apiClient.defaults.adapter;
        const input: TopupProductInput = {
            name: product.name,
            amountFen: product.amountFen,
            creditsMicrocredits: product.creditsMicrocredits,
            enabled: true,
            sortOrder: 0,
            benefits: product.benefits,
            ribbonText: product.ribbonText,
            badgeText: product.badgeText,
            compareAmountFen: product.compareAmountFen,
            priceCaption: product.priceCaption,
            quotaCaption: product.quotaCaption,
            quotaDetail: product.quotaDetail,
            actionText: product.actionText,
            accentColor: product.accentColor,
            featured: product.featured,
            saleStrategy: "timed",
            saleStartAt: "2026-01-01T00:00:00Z",
            saleEndAt: "2026-01-02T00:00:00Z",
        };
        let request: unknown;
        apiClient.defaults.adapter = async (config) => {
            request = { url: config.url, method: config.method, body: JSON.parse(config.data) };
            return { config, headers: {}, status: 200, statusText: "OK", data: { code: 0, msg: "ok", data: { product } } };
        };
        try {
            await createAdminTopupProduct(input);
            expect(request).toMatchObject({
                url: "/admin/payments/products",
                method: "post",
                body: { benefits: "Existing benefit", ribbonText: "Local ribbon", compareAmountFen: 200, amountFen: 100, creditsMicrocredits: 100_000_000, saleStrategy: "timed", saleStartAt: input.saleStartAt, saleEndAt: input.saleEndAt },
            });
        } finally {
            apiClient.defaults.adapter = adapter;
        }
    });

    test("purchase submits IDs and idempotency without a client price or credit grant", async () => {
        const adapter = apiClient.defaults.adapter;
        let body: unknown;
        apiClient.defaults.adapter = async (config) => {
            body = JSON.parse(config.data);
            return { config, headers: {}, status: 200, statusText: "OK", data: { code: 0, msg: "ok", data: { order: { id: "order", status: "created" } } } };
        };
        try {
            await createPaymentOrder({ productId: product.id, providerId: "provider", idempotencyKey: "same-purchase" });
            expect(body).toEqual({ productId: product.id, providerId: "provider", idempotencyKey: "same-purchase" });
            expect(body).not.toHaveProperty("amountFen");
            expect(body).not.toHaveProperty("creditsMicrocredits");
        } finally {
            apiClient.defaults.adapter = adapter;
        }
    });
});
