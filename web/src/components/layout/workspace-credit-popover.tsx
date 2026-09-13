import { App, Button, Input, Popover, Skeleton } from "antd";
import { Check, CreditCard, Gift, Headphones, Sparkles } from "lucide-react";
import { useEffect, useState } from "react";

import { AppModal } from "@/components/ui/product/app-modal";
import { formatCredits } from "@/constant/credits";
import { useWalletBalance } from "@/hooks/use-wallet-balance";
import { listTopupProducts, type TopupProduct } from "@/services/api/payments";
import { redeemCredits } from "@/services/api/wallet";

export function WorkspaceCreditPopover({ userId }: { userId: string }) {
    const { message } = App.useApp();
    const { availableMicrocredits } = useWalletBalance(userId);
    const [open, setOpen] = useState(false);
    const [productsOpen, setProductsOpen] = useState(false);
    const [products, setProducts] = useState<TopupProduct[]>([]);
    const [productsLoading, setProductsLoading] = useState(false);
    const [productsError, setProductsError] = useState("");
    const [productsReloadKey, setProductsReloadKey] = useState(0);
    const [code, setCode] = useState("");
    const [redeeming, setRedeeming] = useState(false);
    const balance = availableMicrocredits === null ? "--" : formatCredits(availableMicrocredits);
    const normalizedCode = code.trim().toLowerCase();

    useEffect(() => {
        if (!productsOpen) return;
        let active = true;
        setProductsLoading(true);
        setProductsError("");
        void listTopupProducts()
            .then((result) => {
                if (active) setProducts(result.products);
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
                        <p>选择管理员已上架的积分商品；购买与客服能力将在下一步接入。</p>
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
                                    <Button type="primary" block onClick={() => message.info("套餐购买功能将在下一步接入")}>选择套餐</Button>
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
        </>
    );
}
