import { App, Button, Form, Input, Select } from "antd";
import { useEffect, useRef, useState } from "react";

import { AppModal } from "@/components/ui/product/app-modal";
import type { CreationInspiration } from "@/lib/inspirations/catalog";
import { normalizeDraft, type InspirationDraft } from "@/stores/use-inspiration-store";

export type InspirationCategoryChoice = { label: string; value: string };

/** 编辑器收集可修改字段；记录身份与来源由服务端个人库管理。 */
type FormValues = {
    title: string;
    description?: string;
    image: string;
    mode: CreationInspiration["mode"];
    category?: string;
    tags?: string;
    ratio?: string;
    prompt: string;
    promptZh?: string;
};

function toFormValues(item: CreationInspiration | null): FormValues {
    return {
        title: item?.title ?? "",
        description: item?.description ?? "",
        image: item?.image ?? "",
        mode: item?.mode ?? "image",
        category: item?.category ?? "",
        tags: item?.tags?.join("，") ?? "",
        ratio: item?.ratio ?? "",
        prompt: item?.prompt ?? "",
        promptZh: item?.promptZh ?? "",
    };
}

export function InspirationEditorModal({
    open,
    item,
    categoryOptions,
    onClose,
    onSubmit,
}: {
    open: boolean;
    item: CreationInspiration | null;
    categoryOptions: InspirationCategoryChoice[];
    onClose: () => void;
    onSubmit: (draft: InspirationDraft) => Promise<void>;
}) {
    const [form] = Form.useForm<FormValues>();
    const { message } = App.useApp();
    const [saving, setSaving] = useState(false);
    const inFlight = useRef(false);
    const mounted = useRef(true);
    useEffect(() => {
        mounted.current = true;
        return () => {
            mounted.current = false;
        };
    }, []);

    const handleOk = async () => {
        if (inFlight.current) return;
        inFlight.current = true;
        setSaving(true);
        try {
            const values = await form.validateFields().catch(() => null);
            if (!values || !mounted.current) return;
            const tags = (values.tags ?? "")
                .split(/[，,]/)
                .map((tag) => tag.trim())
                .filter(Boolean);
            // 走 store 里同一套收敛规则，表单不做第二套校验。
            const draft = normalizeDraft({
                title: values.title,
                description: values.description ?? "",
                image: values.image,
                ...(item?.coverResourceId && values.image === item.image ? { coverResourceId: item.coverResourceId } : {}),
                mode: values.mode,
                prompt: values.prompt,
                promptZh: values.promptZh,
                category: values.category,
                ratio: values.ratio,
                ...(tags.length ? { tags } : {}),
            });
            await onSubmit(draft);
            if (!mounted.current) return;
            form.resetFields();
            onClose();
        } catch (error) {
            if (mounted.current) message.error(error instanceof Error ? error.message : "个人灵感保存失败");
        } finally {
            inFlight.current = false;
            if (mounted.current) setSaving(false);
        }
    };

    return (
        <AppModal
            open={open}
            title={item ? "编辑灵感" : "新建灵感"}
            width={720}
            onCancel={() => {
                if (inFlight.current) return;
                form.resetFields();
                onClose();
            }}
            closable={!saving}
            keyboard={!saving}
            maskClosable={!saving}
            afterOpenChange={(visible) => {
                if (visible) form.setFieldsValue(toFormValues(item));
            }}
            footer={
                <div className="flex justify-end gap-2">
                    <Button
                        disabled={saving}
                        onClick={() => {
                            form.resetFields();
                            onClose();
                        }}
                    >
                        取消
                    </Button>
                    <Button type="primary" loading={saving} onClick={() => void handleOk()}>
                        {item?.promptZhNeedsReview ? "确认中文版本并保存" : item ? "保存" : "创建"}
                    </Button>
                </div>
            }
        >
            <Form form={form} layout="vertical" initialValues={toFormValues(item)} preserve={false} disabled={saving}>
                {item?.promptZhNeedsReview ? <p role="status">原文已在个人库更新，原中文版本仍保留。请核对两份正文后再确认保存。</p> : null}
                {/* 字段较多，不限高的话底部按钮会被顶出视口，提交就点不到了。 */}
                <div className="max-h-[64vh] overflow-y-auto pr-1">
                    <Form.Item name="title" label="标题" rules={[{ required: true, message: "请填写标题" }]}>
                        <Input placeholder="例如：护肤品质感广告主视觉" maxLength={80} />
                    </Form.Item>
                    <Form.Item name="description" label="一句话描述">
                        <Input placeholder="卡片上铺两行，说明这个提示词适合做什么" maxLength={120} />
                    </Form.Item>
                    <Form.Item name="image" label="封面地址（可选）">
                        <Input placeholder="HTTPS 图片地址或内置灵感图片路径" />
                    </Form.Item>
                    <div className="grid gap-x-4 sm:grid-cols-3">
                        <Form.Item name="mode" label="创作类型" rules={[{ required: true }]}>
                            <Select
                                options={[
                                    { label: "图片", value: "image" },
                                    { label: "视频", value: "video" },
                                    { label: "文本", value: "text" },
                                ]}
                            />
                        </Form.Item>
                        <Form.Item name="category" label="分类">
                            <Select allowClear showSearch placeholder="选一个或留空" options={categoryOptions} />
                        </Form.Item>
                        <Form.Item name="ratio" label="画面比例">
                            <Input placeholder="例如 16:9" maxLength={12} />
                        </Form.Item>
                    </div>
                    <Form.Item name="tags" label="标签" extra="用逗号分隔，最多六个">
                        <Input placeholder="电商产品，海报排版" maxLength={120} />
                    </Form.Item>
                    <Form.Item name="prompt" label="提示词正文" rules={[{ required: true, message: "请填写提示词正文" }]}>
                        <Input.TextArea rows={6} placeholder="粘贴完整提示词；外部英文提示词也可以直接放这里" />
                    </Form.Item>
                    <Form.Item name="promptZh" label="中文正文（可选）" extra="填了以后，卡片和「用这个创意创作」会优先用中文这版">
                        <Input.TextArea rows={4} placeholder="中文改写版，留空则只用上面的正文" />
                    </Form.Item>
                </div>
            </Form>
        </AppModal>
    );
}
