import { App, Button, Form, Input, Select } from "antd";
import { Sparkles, Upload } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";

import { resourceFileUrl, uploadResourceFile } from "@/services/api/resources";
import { createUserPrompt, updateUserPrompt, type UserPrompt, type UserPromptInput, type UserPromptMode } from "@/services/api/user-prompts";
import { AppModal } from "@/components/ui/product/app-modal";
import "./user-prompt-editor-modal.css";

type PromptFormValues = {
    title: string;
    mode: UserPromptMode;
    tags?: string;
    coverSource: "upload" | "url" | "none";
    coverUrl?: string;
    prompt: string;
    description?: string;
    source?: string;
};

const modeOptions = [
    { value: "text", label: "文本创作" },
    { value: "image", label: "图片创作" },
    { value: "video", label: "视频创作" },
] satisfies Array<{ value: UserPromptMode; label: string }>;

const modeLabels = Object.fromEntries(modeOptions.map((item) => [item.value, item.label])) as Record<UserPromptMode, string>;
const cardModeLabels: Record<UserPromptMode, string> = { text: "文本", image: "图片", video: "视频" };

export function UserPromptEditorModal({ open, prompt, onClose, onSaved }: { open: boolean; prompt?: UserPrompt | null; onClose: () => void; onSaved?: (prompt: UserPrompt) => void }) {
    const { message } = App.useApp();
    const [saving, setSaving] = useState(false);
    const [coverFile, setCoverFile] = useState<File | null>(null);
    const [coverPreview, setCoverPreview] = useState("");
    const fileInputRef = useRef<HTMLInputElement>(null);
    const [form] = Form.useForm<PromptFormValues>();
    const watched = Form.useWatch([], form) as PromptFormValues | undefined;

    useEffect(() => {
        if (!open) return;
        setCoverFile(null);
        setCoverPreview(prompt?.coverResourceId ? resourceFileUrl(prompt.coverResourceId) : prompt?.coverUrl || "");
        form.setFieldsValue({
            title: prompt?.title || "",
            mode: prompt?.mode || "image",
            tags: prompt?.tags.join(", ") || "",
            coverSource: prompt?.coverResourceId ? "upload" : prompt?.coverUrl ? "url" : "upload",
            coverUrl: prompt?.coverUrl || "",
            prompt: prompt?.prompt || "",
            description: prompt?.description || "",
            source: prompt?.source || "",
        });
    }, [form, open, prompt]);

    useEffect(
        () => () => {
            if (coverPreview.startsWith("blob:")) URL.revokeObjectURL(coverPreview);
        },
        [coverPreview],
    );

    const close = () => {
        if (saving) return;
        setCoverFile(null);
        setCoverPreview("");
        form.resetFields();
        onClose();
    };

    const selectCover = (file?: File) => {
        if (!file) return;
        if (!(["image/jpeg", "image/png", "image/webp"] as string[]).includes(file.type)) {
            message.error("封面仅支持 JPEG、PNG 或 WebP");
            return;
        }
        if (file.size > 10 * 1024 * 1024) {
            message.error("封面不能超过 10 MB");
            return;
        }
        if (coverPreview.startsWith("blob:")) URL.revokeObjectURL(coverPreview);
        setCoverFile(file);
        setCoverPreview(URL.createObjectURL(file));
    };

    const save = async () => {
        try {
            const values = await form.validateFields();
            setSaving(true);
            let coverResourceId = values.coverSource === "upload" ? prompt?.coverResourceId || "" : "";
            if (values.coverSource === "upload" && coverFile) {
                const resource = await uploadResourceFile(coverFile, "image", { fileName: coverFile.name });
                coverResourceId = resource.id;
            }
            const input: UserPromptInput = {
                title: values.title.trim(),
                description: values.description?.trim() || "",
                mode: values.mode,
                prompt: values.prompt.trim(),
                tags: normalizeTags(values.tags),
                source: values.source?.trim() || "",
                coverResourceId,
                coverUrl: values.coverSource === "url" ? values.coverUrl?.trim() || "" : "",
            };
            const result = prompt ? await updateUserPrompt(prompt.id, input) : await createUserPrompt(input);
            message.success(prompt ? "提示词已更新" : "提示词已保存");
            setCoverFile(null);
            setCoverPreview("");
            form.resetFields();
            onClose();
            onSaved?.(result.prompt);
        } catch (error) {
            if (error && typeof error === "object" && "errorFields" in error) return;
            message.error(error instanceof Error ? error.message : "保存提示词失败");
        } finally {
            setSaving(false);
        }
    };

    const previewImage = useMemo(() => {
        if (watched?.coverSource === "url") return watched.coverUrl?.trim() || "";
        if (watched?.coverSource === "none") return "";
        return coverPreview;
    }, [coverPreview, watched?.coverSource, watched?.coverUrl]);

    return (
        <AppModal flush centered open={open} width="min(1240px, calc(100vw - 32px))" title={null} footer={null} closable={false} maskClosable={!saving} keyboard={!saving} rootClassName="user-prompt-editor-modal" onCancel={close}>
            <div className="user-prompt-editor-shell">
                <header className="user-prompt-editor-header">
                    <h2>{prompt ? "编辑提示词" : "新增提示词"}</h2>
                    <button type="button" disabled={saving} aria-label="关闭" onClick={close}>
                        ×
                    </button>
                </header>
                <div className="user-prompt-editor-scroll">
                    <Form form={form} layout="vertical" requiredMark="optional" className="user-prompt-editor-layout">
                        <div className="user-prompt-editor-fields">
                            <div className="user-prompt-form-grid">
                                <Form.Item
                                    name="title"
                                    label="提示词标题"
                                    rules={[
                                        { required: true, message: "请输入提示词标题" },
                                        { max: 120, message: "标题不能超过 120 个字符" },
                                    ]}
                                >
                                    <Input placeholder="例如：雨夜霓虹电影感开场" />
                                </Form.Item>
                                <Form.Item name="mode" label="创作类型" rules={[{ required: true }]}>
                                    <Select options={modeOptions} />
                                </Form.Item>
                            </div>
                            <Form.Item name="tags" label="标签" extra="使用逗号分隔，最多 8 个标签">
                                <Input placeholder="例如：霓虹, 电影感, 雨夜" />
                            </Form.Item>
                            <Form.Item name="coverSource" label="封面来源">
                                <Select
                                    options={[
                                        { value: "upload", label: "上传图片" },
                                        { value: "url", label: "HTTPS 外链" },
                                        { value: "none", label: "不设置封面" },
                                    ]}
                                />
                            </Form.Item>
                            {watched?.coverSource === "upload" ? (
                                <Form.Item label="上传封面" extra="JPEG、PNG 或 WebP，最大 10 MB">
                                    <input
                                        ref={fileInputRef}
                                        type="file"
                                        accept="image/jpeg,image/png,image/webp"
                                        hidden
                                        onChange={(event) => {
                                            selectCover(event.target.files?.[0]);
                                            event.currentTarget.value = "";
                                        }}
                                    />
                                    <Button icon={<Upload className="size-4" />} onClick={() => fileInputRef.current?.click()}>
                                        {coverPreview ? "更换图片" : "选择图片"}
                                    </Button>
                                </Form.Item>
                            ) : null}
                            {watched?.coverSource === "url" ? (
                                <Form.Item
                                    name="coverUrl"
                                    label="封面 URL"
                                    rules={[{ type: "url", message: "请输入有效的 HTTPS 图片地址" }, { validator: (_, value) => (!value || String(value).startsWith("https://") ? Promise.resolve() : Promise.reject(new Error("仅支持 HTTPS 外链"))) }]}
                                >
                                    <Input placeholder="https://example.com/image.png" />
                                </Form.Item>
                            ) : null}
                            <Form.Item
                                name="prompt"
                                label="提示词内容"
                                rules={[
                                    { required: true, message: "请输入提示词内容" },
                                    { max: 20000, message: "提示词不能超过 20000 个字符" },
                                ]}
                            >
                                <Input.TextArea rows={10} showCount maxLength={20000} placeholder="写入可直接用于生成的完整提示词" />
                            </Form.Item>
                            <Form.Item name="description" label="卡片说明" rules={[{ max: 500, message: "卡片说明不能超过 500 个字符" }]}>
                                <Input.TextArea rows={3} placeholder="可选，记录适用场景、参数建议或效果说明" />
                            </Form.Item>
                            <Form.Item name="source" label="来源署名" rules={[{ max: 120, message: "来源署名不能超过 120 个字符" }]}>
                                <Input placeholder="可选，例如：原创提示词" />
                            </Form.Item>
                        </div>
                        <aside className="user-prompt-live-preview">
                            <span>提示词卡片实时预览</span>
                            <PromptPreview image={previewImage} title={watched?.title} description={watched?.description} source={watched?.source} mode={watched?.mode} />
                            <p>按首页五列卡片的实际尺寸和图片原始比例预览，不会公开给其他用户。</p>
                        </aside>
                    </Form>
                </div>
                <footer className="user-prompt-editor-footer">
                    <Button disabled={saving} onClick={close}>
                        取消
                    </Button>
                    <Button type="primary" loading={saving} onClick={() => void save()}>
                        保存提示词
                    </Button>
                </footer>
            </div>
        </AppModal>
    );
}

function PromptPreview({ image, title, description, source, mode }: { image?: string; title?: string; description?: string; source?: string; mode?: UserPromptMode }) {
    const [failed, setFailed] = useState(false);
    const [aspectRatio, setAspectRatio] = useState("");
    const fallbackImage = "/welcome/wing-it/barn.webp";
    const previewImage = image && !failed ? image : fallbackImage;
    useEffect(() => {
        setFailed(false);
        setAspectRatio("");
    }, [image]);
    return (
        <div className="product-collection-card creation-featured-card user-prompt-preview-card">
            <span className="creation-featured-media" style={aspectRatio ? { aspectRatio } : undefined}>
                <img
                    src={previewImage}
                    alt="封面预览"
                    referrerPolicy="no-referrer"
                    onLoad={(event) => {
                        const current = event.currentTarget;
                        if (current.naturalWidth > 0 && current.naturalHeight > 0) setAspectRatio(`${current.naturalWidth} / ${current.naturalHeight}`);
                    }}
                    onError={() => setFailed(true)}
                />
            </span>
            <span className="creation-featured-copy">
                <strong>{title?.trim() || "提示词标题"}</strong>
                {description?.trim() ? <span>{description.trim()}</span> : null}
                <em>
                    <Sparkles />
                    {source?.trim() || "个人灵感"} · {mode ? cardModeLabels[mode] : "图片"}
                </em>
            </span>
        </div>
    );
}

function normalizeTags(value?: string) {
    const seen = new Set<string>();
    return (value || "")
        .split(/[,，]/)
        .map((item) => item.trim())
        .filter((item) => {
            const key = item.toLowerCase();
            if (!item || seen.has(key)) return false;
            seen.add(key);
            return true;
        })
        .slice(0, 8);
}
