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
    storageBytes: 0,
    storageDurationDays: 0,
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
const storage: TopupProduct = { ...credits, id: "storage", name: "空间扩容", kind: "storage", creditsMicrocredits: 0, storageBytes: 4 * 1024 ** 3, storageDurationDays: 30 };

function render(products: TopupProduct[]) {
    return renderToStaticMarkup(
        <WalletTopupPanel
            loading={false}
            products={products}
            providers={[]}
            selectedProviderId=""
            onSelectProduct={() => {}}
            onSelectProvider={() => {}}
            creating={false}
            onPay={() => {}}
            storageGrants={[]}
            onCleanAssets={() => {}}
            storageUsage={{ usedBytes: 2 * 1024 ** 3, baseBytes: 1024 ** 3, purchasedBytes: 4 * 1024 ** 3, totalBytes: 5 * 1024 ** 3, overLimit: false, readOnly: false }}
        />,
    );
}

test("storage expansion keeps the existing credit product presentation", () => {
    const html = render([credits, storage]);
    for (const label of ["原有套餐权益", "本地横幅", "本地活动价格", "空间扩容", "基础 1 GB", "+ 已购 4 GB"]) expect(html).toContain(label);
    expect(html).toContain('data-topup-kind="credits"');
    expect(html).toContain('data-topup-kind="storage"');
});

test("storage-only sales show capacity without restoring credit products", () => {
    const html = render([storage]);
    expect(html).toContain("空间扩容");
    expect(html).not.toContain('data-topup-kind="credits"');
    expect(html).not.toContain("原有套餐权益");
});
