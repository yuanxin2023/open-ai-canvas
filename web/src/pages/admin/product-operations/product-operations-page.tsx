import { App, Button, Form, Input, InputNumber, Tabs } from "antd";
import type { ColumnsType } from "antd/es/table";
import { Plus, RefreshCw } from "lucide-react";
import { useEffect, useMemo, useState } from "react";

import { CreditProductCard, type CreditProductCardData } from "@/components/payments/credit-product-card";
import { formatCredits } from "@/constant/credits";
import { AdminPageFrame } from "@/pages/admin/components/admin-shell";
import { AdminDataTable, AdminStatusBadge, AdminTableEmpty } from "@/pages/admin/components/admin-ui";
import { Switch } from "@/pages/admin/ui/controls";
import { AdminModal } from "@/pages/admin/ui/overlays";
import {
    createAdminTopupProduct,
    listAdminTopupProducts,
    updateAdminTopupProduct,
    type TopupProduct,
} from "@/services/api/payments";

import "./product-operations-page.css";
import { PaymentPromotionPanel } from "./payment-promotion-panel";

type ProductFormValues = {
    name: string;
    description?: string;
    benefits?: string;
    ribbonText?: string;
    badgeText?: string;
    compareAmountYuan?: number;
    priceCaption?: string;
    quotaCaption?: string;
    quotaDetail?: string;
    actionText?: string;
    accentColor: string;
    featured: boolean;
    amountYuan: number;
    credits: number;
    enabled: boolean;
    sortOrder: number;
};

export default function ProductOperationsPage() {
    const { message } = App.useApp();
    const [products, setProducts] = useState<TopupProduct[]>([]);
    const [loading, setLoading] = useState(true);
    const [productDrawer, setProductDrawer] = useState<TopupProduct | null | undefined>();
    const [productSaving, setProductSaving] = useState(false);
    const [activeTab, setActiveTab] = useState("products");
    const [promotionReloadKey, setPromotionReloadKey] = useState(0);
    const [productForm] = Form.useForm<ProductFormValues>();
    const watchedProduct = Form.useWatch([], productForm) as ProductFormValues | undefined;
    const productPreview = useMemo<CreditProductCardData>(() => {
        const amountYuan = finiteNumber(watchedProduct?.amountYuan, productDrawer?.amountFen ? productDrawer.amountFen / 100 : 10);
        const credits = finiteNumber(watchedProduct?.credits, productDrawer?.creditsMicrocredits ? productDrawer.creditsMicrocredits / 1_000_000 : 10);
        return {
            id: productDrawer?.id || "admin-product-preview",
            name: watchedProduct?.name?.trim() || "套餐名称",
            description: watchedProduct?.description?.trim(),
            benefits: watchedProduct?.benefits,
            ribbonText: watchedProduct?.ribbonText,
            badgeText: watchedProduct?.badgeText,
            compareAmountFen: Math.round(finiteNumber(watchedProduct?.compareAmountYuan, 0) * 100),
            priceCaption: watchedProduct?.priceCaption,
            quotaCaption: watchedProduct?.quotaCaption,
            quotaDetail: watchedProduct?.quotaDetail,
            actionText: watchedProduct?.actionText,
            accentColor: watchedProduct?.accentColor,
            featured: watchedProduct?.featured,
            amountFen: Math.max(0, Math.round(amountYuan * 100)),
            creditsMicrocredits: Math.max(0, Math.round(credits * 1_000_000)),
        };
    }, [productDrawer, watchedProduct]);
    const productPreviewMaxCredits = useMemo(() => products.reduce((maximum, product) => product.enabled && product.id !== productPreview.id ? Math.max(maximum, product.creditsMicrocredits) : maximum, productPreview.creditsMicrocredits), [productPreview, products]);

    const loadProducts = async () => {
        setLoading(true);
        try {
            const result = await listAdminTopupProducts();
            setProducts(result.products);
        } catch (error) {
            message.error(error instanceof Error ? error.message : "读取充值商品失败");
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        void loadProducts();
    }, []);

    const openProduct = (product?: TopupProduct) => {
        productForm.resetFields();
        productForm.setFieldsValue(
            product
                ? {
                      name: product.name,
                      description: product.description,
                      benefits: product.benefits,
                      ribbonText: product.ribbonText,
                      badgeText: product.badgeText,
                      compareAmountYuan: product.compareAmountFen / 100,
                      priceCaption: product.priceCaption,
                      quotaCaption: product.quotaCaption,
                      quotaDetail: product.quotaDetail,
                      actionText: product.actionText,
                      accentColor: product.accentColor || "#D8FF4F",
                      featured: product.featured,
                      amountYuan: product.amountFen / 100,
                      credits: product.creditsMicrocredits / 1_000_000,
                      enabled: product.enabled,
                      sortOrder: product.sortOrder,
                  }
                : { enabled: true, featured: false, accentColor: "#D8FF4F", sortOrder: products.length * 10, amountYuan: 10, credits: 10, quotaCaption: "到账积分", actionText: "立即购买" },
        );
        setProductDrawer(product || null);
    };

    const saveProduct = async () => {
        if (productDrawer === undefined) return;
        const values = await productForm.validateFields();
        const input = {
            name: values.name.trim(),
            description: values.description?.trim(),
            benefits: values.benefits?.trim(),
            ribbonText: values.ribbonText?.trim() || "",
            badgeText: values.badgeText?.trim() || "",
            compareAmountFen: Math.round((values.compareAmountYuan || 0) * 100),
            priceCaption: values.priceCaption?.trim() || "",
            quotaCaption: values.quotaCaption?.trim() || "",
            quotaDetail: values.quotaDetail?.trim() || "",
            actionText: values.actionText?.trim() || "",
            accentColor: values.accentColor.toUpperCase(),
            featured: values.featured || false,
            amountFen: Math.round(values.amountYuan * 100),
            creditsMicrocredits: Math.round(values.credits * 1_000_000),
            enabled: values.enabled,
            sortOrder: values.sortOrder || 0,
        };
        setProductSaving(true);
        try {
            if (productDrawer) await updateAdminTopupProduct(productDrawer.id, input);
            else await createAdminTopupProduct(input);
            message.success(productDrawer ? "充值商品已更新" : "充值商品已创建");
            setProductDrawer(undefined);
            await loadProducts();
        } catch (error) {
            message.error(error instanceof Error ? error.message : "保存充值商品失败");
        } finally {
            setProductSaving(false);
        }
    };

    const productColumns: ColumnsType<TopupProduct> = [
        {
            title: "商品",
            key: "name",
            render: (_, product) => (
                <div>
                    <div className="font-medium">{product.name}</div>
                    <div className="mt-0.5 text-xs text-foreground/45">{product.description || "无说明"}</div>
                </div>
            ),
        },
        { title: "售价 / 活动价", dataIndex: "amountFen", width: 150, align: "right", render: (value, product) => <div className="text-right"><div className="font-medium tabular-nums">¥ {(value / 100).toFixed(2)}</div>{product.compareAmountFen > 0 ? <div className="mt-0.5 text-xs text-foreground/45 tabular-nums">常规 ¥ {(product.compareAmountFen / 100).toFixed(2)}</div> : null}</div> },
        { title: "到账积分", dataIndex: "creditsMicrocredits", width: 150, align: "right", render: (value) => <span className="tabular-nums">{formatCredits(value)}</span> },
        { title: "排序", dataIndex: "sortOrder", width: 90, align: "center" },
        { title: "状态", dataIndex: "enabled", width: 100, align: "center", render: (value) => <AdminStatusBadge label={value ? "销售中" : "已停用"} tone={value ? "success" : "neutral"} /> },
        {
            title: "操作",
            key: "actions",
            width: 90,
            align: "center",
            render: (_, product) => (
                <Button size="small" onClick={() => openProduct(product)}>
                    编辑
                </Button>
            ),
        },
    ];

    return (
        <AdminPageFrame
            title="商品管理"
            description="管理面向用户销售的充值套餐"
            actions={
                <Button icon={<RefreshCw className="size-4" />} loading={activeTab === "products" && loading} onClick={() => activeTab === "products" ? void loadProducts() : setPromotionReloadKey((value) => value + 1)}>
                    刷新
                </Button>
            }
            scroll
        >
            <Tabs
                activeKey={activeTab}
                onChange={setActiveTab}
                items={[
                    {
                        key: "products",
                        label: "商品管理",
                        children: (
                            <AdminDataTable
                                toolbar={<span />}
                                trailing={
                                    <Button type="primary" className="admin-toolbar-primary-action" icon={<Plus className="size-4" />} onClick={() => openProduct()}>
                                        新增商品
                                    </Button>
                                }
                                table={{ rowKey: "id", loading, columns: productColumns, dataSource: products, pagination: false, scroll: { x: 820 } }}
                                empty={<AdminTableEmpty title="还没有充值商品" />}
                            />
                        ),
                    },
                    {
                        key: "promotion",
                        label: "促销优惠",
                        children: <PaymentPromotionPanel reloadKey={promotionReloadKey} />,
                    },
                ]}
            />

            <AdminModal
                centered
                title={productDrawer ? "编辑充值商品" : "新增充值商品"}
                width="min(1180px, calc(100vw - 32px))"
                open={productDrawer !== undefined}
                rootClassName="admin-payment-product-modal"
                closable={!productSaving}
                keyboard={!productSaving}
                maskClosable={!productSaving}
                onCancel={() => setProductDrawer(undefined)}
                footer={
                    <div className="flex justify-end gap-2">
                        <Button disabled={productSaving} onClick={() => setProductDrawer(undefined)}>
                            取消
                        </Button>
                        <Button type="primary" loading={productSaving} onClick={() => void saveProduct()}>
                            保存商品
                        </Button>
                    </div>
                }
            >
                <Form form={productForm} layout="vertical" requiredMark="optional">
                    <div className="admin-payment-product-editor-layout">
                        <div className="admin-payment-product-editor-fields">
                            <Form.Item name="name" label="商品名称" rules={[{ required: true, max: 120 }]}>
                                <Input placeholder="例如：100 积分" />
                            </Form.Item>
                            <Form.Item name="description" label="商品说明" rules={[{ max: 500 }]}>
                                <Input.TextArea rows={3} />
                            </Form.Item>
                            <div className="grid grid-cols-2 gap-3">
                                <Form.Item name="ribbonText" label="顶部横幅" rules={[{ max: 120 }]}><Input placeholder="例如：限时加赠" /></Form.Item>
                                <Form.Item name="badgeText" label="标题角标" rules={[{ max: 80 }]}><Input placeholder="例如：热门选择" /></Form.Item>
                            </div>
                            <div className="grid grid-cols-2 gap-3">
                                <Form.Item name="featured" label="推荐套餐" valuePropName="checked" extra="开启后在卡片主体外侧增加主题色顶栏和描边；未填写顶部横幅时显示“推荐套餐”。"><Switch /></Form.Item>
                                <Form.Item name="accentColor" label="套餐主题色" rules={[{ required: true }, { pattern: /^#[0-9a-fA-F]{6}$/, message: "请选择 6 位十六进制颜色" }]} extra="用于推荐顶栏、边框、角标和购买按钮。">
                                    <Input type="color" className="admin-payment-product-color-input" />
                                </Form.Item>
                            </div>
                            <div className="grid grid-cols-2 gap-3">
                                <Form.Item name="compareAmountYuan" label="划线对比价 / 常规价（元）" extra="配置后，限时活动外将按此价格成交；活动期间按售价成交。" rules={[{ type: "number", min: 0, max: 1_000_000 }, { validator: (_, value) => !value || value > Number(productForm.getFieldValue("amountYuan")) ? Promise.resolve() : Promise.reject(new Error("对比价须高于售价")) }]}><InputNumber min={0} max={1_000_000} precision={2} className="w-full" /></Form.Item>
                                <Form.Item name="priceCaption" label="价格补充说明" rules={[{ max: 240 }]} extra="留空时按售价与到账积分显示每 100 积分的价格。"><Input placeholder="例如：一次购买，积分即时到账" /></Form.Item>
                            </div>
                            <Form.Item name="benefits" label="套餐权益（每行一项）" rules={[{ max: 1000 }]} extra="用户端商品卡片会将每一行显示为一条勾选说明；留空则不显示权益区域。">
                                <Input.TextArea rows={4} placeholder={'例如：\n支持图片与文本生成\n支付成功后积分自动到账'} />
                            </Form.Item>
                            <div className="grid grid-cols-2 gap-3">
                                <Form.Item name="amountYuan" label="售价 / 活动价（元）" extra="未配置划线对比价时始终按此价格成交。" rules={[{ required: true }, { type: "number", min: 0.01, max: 1_000_000 }]}>
                                    <InputNumber min={0.01} max={1_000_000} precision={2} className="w-full" />
                                </Form.Item>
                                <Form.Item
                                    name="credits"
                                    label="到账积分"
                                    rules={[
                                        { required: true },
                                        {
                                            validator: (_, value) => {
                                                const credits = Number(value);
                                                const microcredits = Math.round(credits * 1_000_000);
                                                return Number.isFinite(credits) && credits >= 0.01 && credits <= 1_000_000_000 && Number.isSafeInteger(microcredits) ? Promise.resolve() : Promise.reject(new Error("请输入 0.01 至 10 亿之间且可安全处理的积分"));
                                            },
                                        },
                                    ]}
                                >
                                    <InputNumber min={0.01} max={1_000_000_000} precision={2} className="w-full" />
                                </Form.Item>
                            </div>
                            <div className="grid grid-cols-2 gap-3">
                                <Form.Item name="quotaCaption" label="积分区标题" rules={[{ max: 120 }]}><Input placeholder="到账积分" /></Form.Item>
                                <Form.Item name="actionText" label="购买按钮文字" rules={[{ max: 80 }]}><Input placeholder="立即购买" /></Form.Item>
                            </div>
                            <Form.Item name="quotaDetail" label="积分区补充说明" rules={[{ max: 240 }]}><Input placeholder="例如：可用于图片、视频与文本生成" /></Form.Item>
                            <Form.Item name="sortOrder" label="排序" rules={[{ required: true }]}>
                                <InputNumber precision={0} className="w-full" />
                            </Form.Item>
                            <Form.Item name="enabled" label="上架销售" valuePropName="checked">
                                <Switch />
                            </Form.Item>
                        </div>
                        <aside className="admin-payment-product-preview" aria-label="前端支付套餐卡片预览">
                            <header>
                                <div>
                                    <strong>前端卡片实时预览</strong>
                                    <span>内容和样式与用户端购买弹窗一致</span>
                                </div>
                                <span className="admin-payment-product-preview-status">{watchedProduct?.enabled === false ? "未上架" : "上架后展示"}</span>
                            </header>
                            <div className="admin-payment-product-preview-card">
                                <CreditProductCard
                                    product={productPreview}
                                    maxCreditsMicrocredits={productPreviewMaxCredits}
                                    preview
                                />
                            </div>
                            <p>横幅、角标和权益可留空；配置对比价后，它将作为非活动期实际成交价。</p>
                        </aside>
                    </div>
                </Form>
            </AdminModal>
        </AdminPageFrame>
    );
}

function finiteNumber(value: unknown, fallback: number) {
    const number = Number(value);
    return Number.isFinite(number) ? number : fallback;
}
