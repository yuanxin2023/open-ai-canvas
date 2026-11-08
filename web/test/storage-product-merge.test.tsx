import { expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { WalletTopupPanel } from "../src/components/layout/workspace-wallet-topup";
import type { TopupProduct } from "../src/services/api/payments";

const credits: TopupProduct = {
    id: "credits",
    name: "定制积分套餐",
    kind: "credits",
    amountFen: 500,
    creditsMicrocredits: 100_000_000,
    benefits: "原有套餐权益",
    ribbonText: "本地横幅",
    badgeText: "精选",
    compareAmountFen: 800,
    priceCaption: "本地活动价格",
    quotaCaption: "到账积分",
    quotaDetail: "支持创作",
    actionText: "站内购买",
    accentColor: "#123456",
    featured: true,
    enabled: true,
    sortOrder: 0,
    saleStrategy: "unlimited",
    canPurchase: true,
    createdBy: "admin",
    updatedBy: "admin",
    createdAt: "",
    updatedAt: "",
};

function render(products: TopupProduct[]) {
    return renderToStaticMarkup(<WalletTopupPanel loading={false} products={products} providers={[]} selectedProviderId="" onSelectProduct={() => {}} onSelectProvider={() => {}} creating={false} onPay={() => {}} />);
}

test("credit purchase preserves custom presentation without independent storage sales", () => {
    const html = render([credits]);
    for (const label of ["原有套餐权益", "本地横幅", "本地活动价格", "到账积分"]) expect(html).toContain(label);
    expect(html).toContain('data-topup-kind="credits"');
    expect(html).not.toContain('data-topup-kind="storage"');
    expect(html).not.toContain("空间扩容");
});

test("an empty catalog does not introduce storage purchase offers", () => {
    const html = render([]);
    expect(html).not.toContain('data-topup-kind="storage"');
    expect(html).not.toContain("购买空间");
});
