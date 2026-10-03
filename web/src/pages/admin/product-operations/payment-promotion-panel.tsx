import { App, Button, DatePicker, Form, Input, InputNumber, Segmented } from "antd";
import dayjs, { type Dayjs } from "dayjs";
import { ImagePlus, RefreshCw, Save, Trash2 } from "lucide-react";
import { useEffect, useMemo, useRef, useState, type ChangeEvent } from "react";

import { PaymentPromotionBanner } from "@/components/payments/payment-promotion-banner";
import { Switch } from "@/pages/admin/ui/controls";
import { resourceFileUrl } from "@/services/api/resources";
import {
    discardAdminPaymentPromotionImage,
    getAdminPaymentPromotion,
    paymentPromotionImageUrl,
    updateAdminPaymentPromotion,
    uploadAdminPaymentPromotionImage,
    type AdminPaymentPromotion,
    type PublicPaymentPromotion,
} from "@/services/api/payments";

type PromotionFormValues = {
    cardEnabled: boolean;
    imageResourceId: string;
    activityEnabled: boolean;
    activeTitle: string;
    activeSubtitle: string;
    startsAt?: Dayjs;
    durationDays: number;
    inactiveCopyEnabled: boolean;
    inactiveTitle: string;
    inactiveSubtitle: string;
};

const DEFAULT_VALUES: PromotionFormValues = {
    cardEnabled: false,
    imageResourceId: "",
    activityEnabled: false,
    activeTitle: "限时优惠活动",
    activeSubtitle: "活动期间购买套餐可享优惠价格",
    durationDays: 3,
    inactiveCopyEnabled: false,
    inactiveTitle: "创作套餐",
    inactiveSubtitle: "选择适合你的积分套餐",
};

export function PaymentPromotionPanel({ reloadKey = 0 }: { reloadKey?: number }) {
    const { message } = App.useApp();
    const [form] = Form.useForm<PromotionFormValues>();
    const values = Form.useWatch([], form) as PromotionFormValues | undefined;
    const [loading, setLoading] = useState(true);
    const [saving, setSaving] = useState(false);
    const [uploading, setUploading] = useState(false);
    const [previewMode, setPreviewMode] = useState<"active" | "inactive">("active");
    const [saved, setSaved] = useState<AdminPaymentPromotion | null>(null);
    const [imagePreview, setImagePreview] = useState("");
    const [draftImageResourceId, setDraftImageResourceId] = useState("");
    const draftRef = useRef("");
    const fileInputRef = useRef<HTMLInputElement>(null);

    useEffect(() => {
        draftRef.current = draftImageResourceId;
    }, [draftImageResourceId]);

    const load = async () => {
        setLoading(true);
        try {
            const result = await getAdminPaymentPromotion();
            const promotion = result.promotion;
            setSaved(promotion);
            form.setFieldsValue({
                cardEnabled: promotion.cardEnabled,
                imageResourceId: promotion.imageResourceId,
                activityEnabled: promotion.activityEnabled,
                activeTitle: promotion.activeTitle,
                activeSubtitle: promotion.activeSubtitle,
                startsAt: beijingPickerValue(promotion.startsAt),
                durationDays: promotion.durationDays || 3,
                inactiveCopyEnabled: promotion.inactiveCopyEnabled,
                inactiveTitle: promotion.inactiveTitle,
                inactiveSubtitle: promotion.inactiveSubtitle,
            });
            setImagePreview(paymentPromotionImageUrl({ imageUrl: promotion.imageUrl, revision: promotion.public.revision }));
        } catch (error) {
            message.error(error instanceof Error ? error.message : "读取促销优惠配置失败");
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        void (async () => {
            const draftID = draftRef.current;
            if (draftID) {
                await discardAdminPaymentPromotionImage(draftID).catch(() => undefined);
                draftRef.current = "";
                setDraftImageResourceId("");
            }
            await load();
        })();
    }, [reloadKey]);

    useEffect(() => () => {
        if (draftRef.current) void discardAdminPaymentPromotionImage(draftRef.current).catch(() => undefined);
    }, []);

    const discardDraft = async () => {
        const id = draftRef.current;
        if (!id) return;
        await discardAdminPaymentPromotionImage(id);
        draftRef.current = "";
        setDraftImageResourceId("");
    };

    const uploadImage = async (event: ChangeEvent<HTMLInputElement>) => {
        const file = event.target.files?.[0];
        event.target.value = "";
        if (!file) return;
        if (!["image/jpeg", "image/png", "image/webp"].includes(file.type)) {
            message.warning("背景图仅支持 JPEG、PNG 或 WebP");
            return;
        }
        if (file.size > 10 * 1024 * 1024) {
            message.warning("背景图不能超过 10MB");
            return;
        }
        setUploading(true);
        try {
            await discardDraft();
            const { resource } = await uploadAdminPaymentPromotionImage(file);
            draftRef.current = resource.id;
            setDraftImageResourceId(resource.id);
            form.setFieldValue("imageResourceId", resource.id);
            setImagePreview(resourceFileUrl(resource.id));
            message.success("促销背景图已上传，保存配置后生效");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "促销背景图上传失败");
        } finally {
            setUploading(false);
        }
    };

    const clearImage = async () => {
        setUploading(true);
        try {
            await discardDraft();
            form.setFieldValue("imageResourceId", "");
            form.setFieldValue("cardEnabled", false);
            form.setFieldValue("activityEnabled", false);
            setImagePreview("");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "清理促销背景图失败");
        } finally {
            setUploading(false);
        }
    };

    const reset = async () => {
        setUploading(true);
        try {
            await discardDraft();
            await load();
        } catch (error) {
            message.error(error instanceof Error ? error.message : "重新加载促销配置失败");
        } finally {
            setUploading(false);
        }
    };

    const save = async () => {
        const next = await form.validateFields();
        setSaving(true);
        try {
            const result = await updateAdminPaymentPromotion({
                cardEnabled: next.cardEnabled,
                imageResourceId: next.imageResourceId || "",
                activityEnabled: next.activityEnabled,
                activeTitle: next.activeTitle?.trim() || "",
                activeSubtitle: next.activeSubtitle?.trim() || "",
                startsAt: next.startsAt ? `${next.startsAt.format("YYYY-MM-DDTHH:mm:ss")}+08:00` : undefined,
                durationDays: next.durationDays || 3,
                inactiveCopyEnabled: next.inactiveCopyEnabled,
                inactiveTitle: next.inactiveTitle?.trim() || "",
                inactiveSubtitle: next.inactiveSubtitle?.trim() || "",
            });
            draftRef.current = "";
            setDraftImageResourceId("");
            setSaved(result.promotion);
            form.setFieldValue("imageResourceId", result.promotion.imageResourceId);
            setImagePreview(paymentPromotionImageUrl({ imageUrl: result.promotion.imageUrl, revision: result.promotion.public.revision }));
            message.success("促销优惠配置已保存");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "保存促销优惠配置失败");
        } finally {
            setSaving(false);
        }
    };

    const previewPromotion = useMemo<PublicPaymentPromotion>(() => {
        const active = previewMode === "active";
        const durationDays = values?.durationDays || 3;
        return {
            visible: Boolean(imagePreview),
            phase: active ? "active" : "inactive",
            imageUrl: imagePreview,
            activeTitle: values?.activeTitle || "活动主标题",
            activeSubtitle: values?.activeSubtitle || "活动副标题",
            inactiveCopyEnabled: values?.inactiveCopyEnabled ?? false,
            inactiveTitle: values?.inactiveTitle || "日常主标题",
            inactiveSubtitle: values?.inactiveSubtitle || "日常副标题",
            endsAt: new Date(Date.now() + durationDays * 86_400_000).toISOString(),
            revision: draftImageResourceId || saved?.public.revision,
        };
    }, [draftImageResourceId, imagePreview, previewMode, saved?.public.revision, values]);

    return (
        <div className="admin-payment-promotion-panel" aria-busy={loading}>
            <section className="admin-payment-promotion-form">
                <Form form={form} layout="vertical" initialValues={DEFAULT_VALUES} requiredMark="optional" disabled={loading || saving}>
                    <div className="admin-payment-promotion-section-heading">
                        <div><strong>促销卡片</strong><span>关闭后卡片与限时价格同时停用。</span></div>
                        <Form.Item name="cardEnabled" valuePropName="checked" noStyle><Switch aria-label="显示促销卡片" /></Form.Item>
                    </div>
                    <Form.Item name="imageResourceId" hidden><Input /></Form.Item>
                    <div className="admin-payment-promotion-image-row">
                        <div className="admin-payment-promotion-image-copy"><strong>背景图片</strong><span>支持 JPEG、PNG、WebP，最大 10MB，推荐约 8:1。</span></div>
                        <div className="flex gap-2">
                            <Button icon={<ImagePlus className="size-4" />} loading={uploading} onClick={() => fileInputRef.current?.click()}>{imagePreview ? "替换图片" : "上传图片"}</Button>
                            {imagePreview ? <Button danger icon={<Trash2 className="size-4" />} disabled={uploading} onClick={() => void clearImage()}>移除</Button> : null}
                        </div>
                        <input ref={fileInputRef} className="hidden" type="file" accept="image/jpeg,image/png,image/webp" onChange={uploadImage} />
                    </div>

                    <div className="admin-payment-promotion-divider" />
                    <div className="admin-payment-promotion-section-heading">
                        <div><strong>限时活动</strong><span>活动期间使用商品售价；结束后恢复划线对比价。</span></div>
                        <Form.Item name="activityEnabled" valuePropName="checked" noStyle><Switch aria-label="启用限时活动" /></Form.Item>
                    </div>
                    <Form.Item noStyle shouldUpdate={(previous, current) => previous.activityEnabled !== current.activityEnabled}>
                        {({ getFieldValue }) => {
                            const enabled = getFieldValue("activityEnabled");
                            return <>
                                <Form.Item name="activeTitle" label="活动主标题" rules={[{ required: enabled, max: 120 }]}><Input placeholder="例如：品牌设计月，活动期间惊喜 5 折" /></Form.Item>
                                <Form.Item name="activeSubtitle" label="活动副标题" rules={[{ max: 240 }]}><Input.TextArea rows={2} placeholder="例如：最高立享 31 天无限创作" /></Form.Item>
                                <div className="grid grid-cols-2 gap-3">
                                    <Form.Item name="startsAt" label="开始时间（北京时间）" rules={[{ required: enabled, message: "请选择活动开始时间" }]}><DatePicker showTime format="YYYY-MM-DD HH:mm:ss" className="w-full" placeholder="选择开始时间" /></Form.Item>
                                    <Form.Item name="durationDays" label="活动时长（天）" rules={[{ required: enabled }, { type: "number", min: 1, max: 365 }]}><InputNumber min={1} max={365} precision={0} className="w-full" /></Form.Item>
                                </div>
                            </>;
                        }}
                    </Form.Item>

                    <div className="admin-payment-promotion-divider" />
                    <div className="admin-payment-promotion-section-heading">
                        <div><strong>非活动文案</strong><span>活动未开始或结束后，背景图仍显示，可选择展示日常文案。</span></div>
                        <Form.Item name="inactiveCopyEnabled" valuePropName="checked" noStyle><Switch aria-label="显示非活动文案" /></Form.Item>
                    </div>
                    <Form.Item noStyle shouldUpdate={(previous, current) => previous.inactiveCopyEnabled !== current.inactiveCopyEnabled}>
                        {({ getFieldValue }) => {
                            const enabled = getFieldValue("inactiveCopyEnabled");
                            return <>
                                <Form.Item name="inactiveTitle" label="日常主标题" rules={[{ required: enabled, max: 120 }]}><Input /></Form.Item>
                                <Form.Item name="inactiveSubtitle" label="日常副标题" rules={[{ max: 240 }]}><Input.TextArea rows={2} /></Form.Item>
                            </>;
                        }}
                    </Form.Item>
                    <div className="admin-payment-promotion-actions">
                        <Button icon={<RefreshCw className="size-4" />} disabled={saving || uploading} onClick={() => void reset()}>放弃修改</Button>
                        <Button type="primary" icon={<Save className="size-4" />} loading={saving} disabled={uploading} onClick={() => void save()}>保存配置</Button>
                    </div>
                </Form>
            </section>

            <aside className="admin-payment-promotion-preview">
                <header><div><strong>前台效果预览</strong><span>预览仅用于排版，真实状态以服务器时间为准。</span></div><Segmented size="small" value={previewMode} options={[{ label: "活动中", value: "active" }, { label: "非活动", value: "inactive" }]} onChange={(value) => setPreviewMode(value as "active" | "inactive")} /></header>
                {imagePreview ? <PaymentPromotionBanner promotion={previewPromotion} serverTime={new Date().toISOString()} /> : <div className="admin-payment-promotion-empty">上传背景图后可预览促销卡片</div>}
            </aside>
        </div>
    );
}

function beijingPickerValue(value?: string) {
    if (!value) return undefined;
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return undefined;
    const parts = new Intl.DateTimeFormat("zh-CN", {
        timeZone: "Asia/Shanghai", year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit", hourCycle: "h23",
    }).formatToParts(date).reduce<Record<string, string>>((result, part) => ({ ...result, [part.type]: part.value }), {});
    return dayjs(`${parts.year}-${parts.month}-${parts.day} ${parts.hour}:${parts.minute}:${parts.second}`);
}
