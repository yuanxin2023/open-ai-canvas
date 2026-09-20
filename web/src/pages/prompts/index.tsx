import { App, Button, Form, Input, Popconfirm, Select, Tag } from "antd";
import copyToClipboard from "copy-to-clipboard";
import { Copy, Image as ImageIcon, Pencil, Plus, Trash2, Upload } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { PageHeader, PaginationBar, WorkspacePage } from "@/components/layout/workspace-page";
import { WorkspaceErrorState, WorkspaceLoadingState, WorkspaceState } from "@/components/layout/workspace-state";
import { AppModal } from "@/components/ui/product/app-modal";
import { resourceFileUrl, uploadResourceFile } from "@/services/api/resources";
import { createUserPrompt, deleteUserPrompt, listUserPrompts, updateUserPrompt, type UserPrompt, type UserPromptInput, type UserPromptMode } from "@/services/api/user-prompts";
import "./prompts.css";

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

export default function UserPromptsPage() {
    const { message } = App.useApp();
    const [rows, setRows] = useState<UserPrompt[]>([]);
    const [loading, setLoading] = useState(true);
    const [loadError, setLoadError] = useState("");
    const [page, setPage] = useState(1);
    const [pageSize, setPageSize] = useState(20);
    const [total, setTotal] = useState(0);
    const [editing, setEditing] = useState<UserPrompt | null | undefined>();
    const [saving, setSaving] = useState(false);
    const [deletingID, setDeletingID] = useState("");
    const [coverFile, setCoverFile] = useState<File | null>(null);
    const [coverPreview, setCoverPreview] = useState("");
    const fileInputRef = useRef<HTMLInputElement>(null);
    const requestRef = useRef<AbortController | null>(null);
    const [form] = Form.useForm<PromptFormValues>();
    const watched = Form.useWatch([], form) as PromptFormValues | undefined;

    const load = useCallback(async (nextPage = page, nextPageSize = pageSize) => {
        requestRef.current?.abort();
        const controller = new AbortController();
        requestRef.current = controller;
        setLoading(true);
        setLoadError("");
        try {
            const result = await listUserPrompts({ page: nextPage, pageSize: nextPageSize }, controller.signal);
            setRows(result.prompts);
            setTotal(result.total);
            setPage(result.page);
            setPageSize(result.pageSize);
        } catch (error) {
            if (controller.signal.aborted) return;
            setLoadError(error instanceof Error ? error.message : "提示词加载失败");
        } finally {
            if (!controller.signal.aborted) setLoading(false);
        }
    }, [page, pageSize]);

    useEffect(() => {
        void load(1, pageSize);
        return () => requestRef.current?.abort();
    }, []); // eslint-disable-line react-hooks/exhaustive-deps

    useEffect(() => () => {
        if (coverPreview.startsWith("blob:")) URL.revokeObjectURL(coverPreview);
    }, [coverPreview]);

    const openEditor = (row?: UserPrompt) => {
        const next = row || null;
        setEditing(next);
        setCoverFile(null);
        setCoverPreview(row?.coverResourceId ? resourceFileUrl(row.coverResourceId) : row?.coverUrl || "");
        form.setFieldsValue({
            title: row?.title || "",
            mode: row?.mode || "image",
            tags: row?.tags.join(", ") || "",
            coverSource: row?.coverResourceId ? "upload" : row?.coverUrl ? "url" : "upload",
            coverUrl: row?.coverUrl || "",
            prompt: row?.prompt || "",
            description: row?.description || "",
            source: row?.source || "",
        });
    };

    const closeEditor = () => {
        if (saving) return;
        setEditing(undefined);
        setCoverFile(null);
        setCoverPreview("");
        form.resetFields();
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
        if (editing === undefined) return;
        try {
            const values = await form.validateFields();
            setSaving(true);
            let coverResourceId = values.coverSource === "upload" ? editing?.coverResourceId || "" : "";
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
            if (editing) await updateUserPrompt(editing.id, input);
            else await createUserPrompt(input);
            message.success(editing ? "提示词已更新" : "提示词已保存");
            closeEditorAfterSave(form, setEditing, setCoverFile, setCoverPreview);
            await load(editing ? page : 1, pageSize);
        } catch (error) {
            if (error && typeof error === "object" && "errorFields" in error) return;
            message.error(error instanceof Error ? error.message : "保存提示词失败");
        } finally {
            setSaving(false);
        }
    };

    const remove = async (row: UserPrompt) => {
        setDeletingID(row.id);
        try {
            await deleteUserPrompt(row.id);
            message.success("提示词已删除");
            await load(rows.length === 1 && page > 1 ? page - 1 : page, pageSize);
        } catch (error) {
            message.error(error instanceof Error ? error.message : "删除提示词失败");
        } finally {
            setDeletingID("");
        }
    };

    const copyPrompt = async (row: UserPrompt) => {
        if (await copyToClipboard(row.prompt)) message.success("提示词已复制");
        else message.error("复制失败，请手动复制");
    };

    const previewImage = useMemo(() => {
        if (watched?.coverSource === "url") return watched.coverUrl?.trim() || "";
        if (watched?.coverSource === "none") return "";
        return coverPreview;
    }, [coverPreview, watched?.coverSource, watched?.coverUrl]);

    return (
        <WorkspacePage className="user-prompts-page">
            <PageHeader
                title="我的提示词"
                description="保存自己的提示词记录，随时复制使用或继续沉淀创作方法。"
                actions={<Button type="primary" icon={<Plus className="size-4" />} onClick={() => openEditor()}>添加提示词</Button>}
            />

            <section className="user-prompts-records" aria-labelledby="user-prompts-records-title">
                <div className="user-prompts-records-header"><h2 id="user-prompts-records-title">我的记录</h2><span>共 {total} 条</span></div>
                <div className="user-prompts-table-head" aria-hidden><span>标题</span><span>分类</span><span>更新时间</span><span>操作</span></div>
                {loading ? <WorkspaceLoadingState label="正在读取提示词" detail="整理你的个人提示词记录" rows={3} className="px-5" /> : loadError ? (
                    <WorkspaceErrorState compact description={loadError} onRetry={() => void load(page, pageSize)} />
                ) : rows.length ? (
                    <div className="user-prompts-table-body">
                        {rows.map((row) => <PromptRow key={row.id} row={row} deleting={deletingID === row.id} onCopy={() => copyPrompt(row)} onEdit={() => openEditor(row)} onDelete={() => void remove(row)} />)}
                    </div>
                ) : <WorkspaceState compact title="还没有保存提示词" description="点击右上角“添加提示词”，建立自己的可复用提示词库。" action={<Button icon={<Plus className="size-4" />} onClick={() => openEditor()}>添加第一条提示词</Button>} />}
            </section>

            <PaginationBar current={page} pageSize={pageSize} total={total} itemLabel="条" onChange={(nextPage, nextPageSize) => void load(nextPage, nextPageSize)} />

            <AppModal
                flush centered open={editing !== undefined} width="min(1240px, calc(100vw - 32px))"
                title={null} footer={null} closable={false} maskClosable={!saving} keyboard={!saving}
                rootClassName="user-prompt-editor-modal" onCancel={closeEditor}
            >
                <div className="user-prompt-editor-shell">
                    <header className="user-prompt-editor-header"><h2>{editing ? "编辑提示词" : "新增提示词"}</h2><button type="button" disabled={saving} aria-label="关闭" onClick={closeEditor}>×</button></header>
                    <div className="user-prompt-editor-scroll">
                        <Form form={form} layout="vertical" requiredMark="optional" className="user-prompt-editor-layout">
                            <div className="user-prompt-editor-fields">
                                <div className="user-prompt-form-grid">
                                    <Form.Item name="title" label="提示词标题" rules={[{ required: true, message: "请输入提示词标题" }, { max: 120, message: "标题不能超过 120 个字符" }]}>
                                        <Input placeholder="例如：雨夜霓虹电影感开场" />
                                    </Form.Item>
                                    <Form.Item name="mode" label="创作类型" rules={[{ required: true }]}><Select options={modeOptions} /></Form.Item>
                                </div>
                                <Form.Item name="tags" label="标签" extra="使用逗号分隔，最多 8 个标签"><Input placeholder="例如：霓虹, 电影感, 雨夜" /></Form.Item>
                                <Form.Item name="coverSource" label="封面来源"><Select options={[{ value: "upload", label: "上传图片" }, { value: "url", label: "HTTPS 外链" }, { value: "none", label: "不设置封面" }]} /></Form.Item>
                                {watched?.coverSource === "upload" ? <Form.Item label="上传封面" extra="JPEG、PNG 或 WebP，最大 10 MB">
                                    <input ref={fileInputRef} type="file" accept="image/jpeg,image/png,image/webp" hidden onChange={(event) => { selectCover(event.target.files?.[0]); event.currentTarget.value = ""; }} />
                                    <Button icon={<Upload className="size-4" />} onClick={() => fileInputRef.current?.click()}>{coverPreview ? "更换图片" : "选择图片"}</Button>
                                </Form.Item> : null}
                                {watched?.coverSource === "url" ? <Form.Item name="coverUrl" label="封面 URL" rules={[{ type: "url", message: "请输入有效的 HTTPS 图片地址" }, { validator: (_, value) => !value || String(value).startsWith("https://") ? Promise.resolve() : Promise.reject(new Error("仅支持 HTTPS 外链")) }]}><Input placeholder="https://example.com/image.png" /></Form.Item> : null}
                                <Form.Item name="prompt" label="提示词内容" rules={[{ required: true, message: "请输入提示词内容" }, { max: 20000, message: "提示词不能超过 20000 个字符" }]}>
                                    <Input.TextArea rows={10} showCount maxLength={20000} placeholder="写入可直接用于生成的完整提示词" />
                                </Form.Item>
                                <Form.Item name="description" label="卡片说明" rules={[{ max: 500, message: "卡片说明不能超过 500 个字符" }]}><Input.TextArea rows={3} placeholder="可选，记录适用场景、参数建议或效果说明" /></Form.Item>
                                <Form.Item name="source" label="来源署名" rules={[{ max: 120, message: "来源署名不能超过 120 个字符" }]}><Input placeholder="可选，例如：原创提示词" /></Form.Item>
                            </div>
                            <aside className="user-prompt-live-preview">
                                <span>提示词卡片实时预览</span>
                                <PromptPreview image={previewImage} title={watched?.title} description={watched?.description} source={watched?.source} mode={watched?.mode} />
                                <p>预览仅用于核对内容，不会公开给其他用户。</p>
                            </aside>
                        </Form>
                    </div>
                    <footer className="user-prompt-editor-footer"><Button disabled={saving} onClick={closeEditor}>取消</Button><Button type="primary" loading={saving} onClick={() => void save()}>保存提示词</Button></footer>
                </div>
            </AppModal>
        </WorkspacePage>
    );
}

function PromptRow({ row, deleting, onCopy, onEdit, onDelete }: { row: UserPrompt; deleting: boolean; onCopy: () => void; onEdit: () => void; onDelete: () => void }) {
    return <article className="user-prompt-row">
        <div className="user-prompt-row-main">
            <div className="user-prompt-row-cover">{row.coverResourceId || row.coverUrl ? <img src={row.coverResourceId ? resourceFileUrl(row.coverResourceId) : row.coverUrl} alt="" referrerPolicy="no-referrer" /> : <ImageIcon />}</div>
            <div className="min-w-0"><h3>{row.title}</h3><p>{row.description || row.prompt}</p>{row.tags.length ? <div className="user-prompt-row-tags">{row.tags.slice(0, 3).map((tag) => <Tag key={tag}>{tag}</Tag>)}</div> : null}</div>
        </div>
        <span className="user-prompt-row-mode">{modeLabels[row.mode]}</span>
        <time dateTime={row.updatedAt}>{formatPromptTime(row.updatedAt)}</time>
        <div className="user-prompt-row-actions">
            <Button type="text" icon={<Copy className="size-4" />} onClick={onCopy}>复制</Button>
            <Button type="text" icon={<Pencil className="size-4" />} onClick={onEdit}>编辑</Button>
            <Popconfirm title="删除这条提示词？" description="删除后无法恢复。" okText="删除" cancelText="取消" okButtonProps={{ danger: true }} onConfirm={onDelete}><Button type="text" danger loading={deleting} icon={<Trash2 className="size-4" />} aria-label={`删除${row.title}`} /></Popconfirm>
        </div>
    </article>;
}

function PromptPreview({ image, title, description, source, mode }: { image?: string; title?: string; description?: string; source?: string; mode?: UserPromptMode }) {
    const [failed, setFailed] = useState(false);
    useEffect(() => setFailed(false), [image]);
    return <div className="user-prompt-preview-card">
        <div className="user-prompt-preview-media">{image && !failed ? <img src={image} alt="封面预览" referrerPolicy="no-referrer" onError={() => setFailed(true)} /> : <span><ImageIcon />等待选择封面</span>}</div>
        <div className="user-prompt-preview-copy"><h3>{title?.trim() || "提示词标题"}</h3><p>{description?.trim() || "卡片说明会显示在这里"}</p><small>{source?.trim() || "原创提示词"} · {mode ? modeLabels[mode] : "图片创作"}</small></div>
    </div>;
}

function normalizeTags(value?: string) {
    const seen = new Set<string>();
    return (value || "").split(/[,，]/).map((item) => item.trim()).filter((item) => {
        const key = item.toLowerCase();
        if (!item || seen.has(key)) return false;
        seen.add(key);
        return true;
    }).slice(0, 8);
}

function closeEditorAfterSave(form: ReturnType<typeof Form.useForm<PromptFormValues>>[0], setEditing: (value: undefined) => void, setCoverFile: (value: null) => void, setCoverPreview: (value: string) => void) {
    setEditing(undefined);
    setCoverFile(null);
    setCoverPreview("");
    form.resetFields();
}

function formatPromptTime(value: string) {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? "—" : new Intl.DateTimeFormat("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit" }).format(date);
}
