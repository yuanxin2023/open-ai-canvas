import { App, Button, Form, Input, Modal, Select, Spin, Tag, Upload } from "antd";
import type { ColumnsType } from "antd/es/table";
import { ArrowDown, ArrowUp, GripVertical, ListOrdered, PencilLine, Plus, RefreshCw, Search, Trash2, UploadCloud } from "lucide-react";
import { useEffect, useRef, useState, type Key } from "react";

import { useDebouncedValue } from "@/hooks/use-debounced-value";
import { readImageFileSize } from "@/lib/image-utils";
import { AdminModal } from "@/pages/admin/ui/overlays";
import {
    batchDeleteAdminInspirations,
    createAdminInspiration,
    deleteAdminInspiration,
    discardAdminInspirationCover,
    getAdminInspirationOrder,
    inspirationCoverUrl,
    inspirationDraftCoverUrl,
    listAdminInspirations,
    saveAdminInspirationOrder,
    setAdminInspirationStatus,
    updateAdminInspiration,
    uploadAdminInspirationCover,
    type Inspiration,
    type InspirationInput,
    type InspirationMode,
    type InspirationStatus,
} from "@/services/api/inspirations";
import { AdminBatchBar, AdminDataTable, AdminRowActions, AdminStatusBadge, AdminTableEmpty, PaginationBar } from "./admin-ui";
import { moveOrderItem } from "./channel-order-dialog";

type EditorValues = {
    title: string;
    mode: InspirationMode;
    tags: string;
    description: string;
    prompt: string;
    source?: string;
    coverKind: "upload" | "url";
    coverUrl?: string;
};

type CoverDimensions = { source: string; width: number; height: number };

const modeLabels: Record<InspirationMode, string> = { text: "文本创作", image: "图片创作", video: "视频创作" };

function parseTags(value = "") {
    const seen = new Set<string>();
    return value
        .split(/[，,]/)
        .map((tag) => tag.trim())
        .filter((tag) => {
            const key = tag.toLowerCase();
            if (!tag || seen.has(key)) return false;
            seen.add(key);
            return true;
        });
}

function validateTags(value?: string) {
    const tags = parseTags(value);
    if (tags.length > 8) return Promise.reject(new Error("最多填写 8 个标签"));
    if (tags.some((tag) => [...tag].length > 24)) return Promise.reject(new Error("单个标签不能超过 24 个字符"));
    return Promise.resolve();
}

function isHTTPSURL(value: string) {
    try {
        const url = new URL(value);
        return url.protocol === "https:" && Boolean(url.hostname);
    } catch {
        return false;
    }
}

export default function AdminInspirationsPanel() {
    const { message, modal } = App.useApp();
    const [form] = Form.useForm<EditorValues>();
    const [items, setItems] = useState<Inspiration[]>([]);
    const [total, setTotal] = useState(0);
    const [page, setPage] = useState(1);
    const [pageSize, setPageSize] = useState(20);
    const [keyword, setKeyword] = useState("");
    const search = useDebouncedValue(keyword);
    const [mode, setMode] = useState<"all" | InspirationMode>("all");
    const [status, setStatus] = useState<"all" | InspirationStatus>("all");
    const [selected, setSelected] = useState<Key[]>([]);
    const [loading, setLoading] = useState(true);
    const [refresh, setRefresh] = useState(0);
    const [editorOpen, setEditorOpen] = useState(false);
    const [editing, setEditing] = useState<Inspiration | null>(null);
    const [saving, setSaving] = useState(false);
    const [draftCoverId, setDraftCoverId] = useState("");
    const [coverDimensions, setCoverDimensions] = useState<CoverDimensions | null>(null);
    const [uploading, setUploading] = useState(false);
    const [orderOpen, setOrderOpen] = useState(false);
    const [orderItems, setOrderItems] = useState<Array<{ id: string; name: string; enabled: boolean }>>([]);
    const [orderOriginal, setOrderOriginal] = useState<string[]>([]);
    const [orderLoading, setOrderLoading] = useState(false);
    const [orderSaving, setOrderSaving] = useState(false);
    const dragged = useRef<string | null>(null);
    const watched = Form.useWatch([], form) as EditorValues | undefined;

    const reload = async () => {
        setLoading(true);
        try {
            const result = await listAdminInspirations({ keyword: search.trim() || undefined, mode: mode === "all" ? undefined : mode, status: status === "all" ? undefined : status, page, pageSize });
            setItems(result.inspirations || []);
            setTotal(result.total || 0);
            setSelected([]);
            const last = Math.max(1, Math.ceil((result.total || 0) / pageSize));
            if (page > last) setPage(last);
        } catch (error) {
            message.error(error instanceof Error ? error.message : "读取精选灵感失败");
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        void reload();
    }, [search, mode, status, page, pageSize, refresh]);

    const openCreate = () => {
        setEditing(null);
        setDraftCoverId("");
        setCoverDimensions(null);
        form.setFieldsValue({ title: "", mode: "image", tags: "", description: "", prompt: "", source: "", coverKind: "upload", coverUrl: "" });
        setEditorOpen(true);
    };

    const openEdit = (item: Inspiration) => {
        setEditing(item);
        setDraftCoverId("");
        setCoverDimensions(item.coverWidth > 0 && item.coverHeight > 0 ? { source: item.coverResourceId || item.coverUrl, width: item.coverWidth, height: item.coverHeight } : null);
        form.setFieldsValue({
            title: item.title,
            mode: item.mode,
            tags: item.tags.join("，"),
            description: item.description,
            prompt: item.prompt,
            source: item.source || "",
            coverKind: item.coverResourceId ? "upload" : "url",
            coverUrl: item.coverResourceId ? "" : item.coverUrl,
        });
        setEditorOpen(true);
    };

    const discardDraft = async () => {
        const id = draftCoverId;
        setDraftCoverId("");
        if (id) await discardAdminInspirationCover(id).catch(() => undefined);
    };

    const closeEditor = () => {
        if (saving || uploading) return;
        void discardDraft();
        setEditorOpen(false);
    };

    const save = async () => {
        let values: EditorValues;
        try {
            values = await form.validateFields();
        } catch {
            return;
        }
        const coverResourceId = values.coverKind === "upload" ? draftCoverId || editing?.coverResourceId || "" : "";
        const coverUrl = values.coverKind === "url" ? values.coverUrl?.trim() || "" : "";
        if (!coverResourceId && !coverUrl) {
            message.error("请上传封面或填写 HTTPS 封面地址");
            return;
        }
        const coverIdentity = coverResourceId || coverUrl;
        const dimensions = coverDimensions?.source === coverIdentity ? coverDimensions : null;
        if (!dimensions) {
            message.error("正在读取封面尺寸，请确认图片可以正常显示后再保存");
            return;
        }
        const payload: InspirationInput = {
            title: values.title.trim(),
            description: values.description.trim(),
            mode: values.mode,
            prompt: values.prompt.trim(),
            tags: parseTags(values.tags),
            source: values.source?.trim() || "",
            coverResourceId,
            coverUrl,
            coverWidth: dimensions.width,
            coverHeight: dimensions.height,
        };
        setSaving(true);
        try {
            if (editing) await updateAdminInspiration(editing.id, payload);
            else await createAdminInspiration(payload);
            if (values.coverKind === "url" && draftCoverId) {
                await discardAdminInspirationCover(draftCoverId).catch(() => undefined);
            }
            setDraftCoverId("");
            setEditorOpen(false);
            message.success(editing ? "精选灵感已更新" : "精选灵感已保存为停用状态");
            setRefresh((value) => value + 1);
        } catch (error) {
            message.error(error instanceof Error ? error.message : "保存失败");
        } finally {
            setSaving(false);
        }
    };

    const uploadCover = async (file: File) => {
        setUploading(true);
        try {
            const dimensions = await readImageFileSize(file);
            if (draftCoverId) await discardAdminInspirationCover(draftCoverId).catch(() => undefined);
            const result = await uploadAdminInspirationCover(file, dimensions);
            setDraftCoverId(result.resource.id);
            setCoverDimensions({ source: result.resource.id, width: result.resource.width || dimensions.width, height: result.resource.height || dimensions.height });
            form.setFieldValue("coverKind", "upload");
            message.success("封面已上传，保存后正式使用");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "上传失败");
        } finally {
            setUploading(false);
        }
        return false;
    };

    const toggleStatus = async (item: Inspiration) => {
        const next = item.status === "active" ? "disabled" : "active";
        try {
            await setAdminInspirationStatus(item.id, next);
            message.success(next === "active" ? "已启用并同步到首页" : "已停用并从首页隐藏");
            setRefresh((value) => value + 1);
        } catch (error) {
            message.error(error instanceof Error ? error.message : "状态更新失败");
        }
    };

    const confirmDelete = (targets: Inspiration[]) =>
        modal.confirm({
            title: targets.length > 1 ? `永久删除 ${targets.length} 条精选灵感？` : `永久删除“${targets[0]?.title}”？`,
            content: "建议优先使用停用。永久删除不可恢复，托管封面也会进入安全清理流程。",
            okText: "永久删除",
            okButtonProps: { danger: true },
            cancelText: "取消",
            onOk: async () => {
                if (targets.length === 1) await deleteAdminInspiration(targets[0]!.id);
                else await batchDeleteAdminInspirations(targets.map((item) => item.id));
                message.success("已永久删除");
                setRefresh((value) => value + 1);
            },
        });

    const openOrder = async () => {
        setOrderOpen(true);
        setOrderLoading(true);
        try {
            const result = await getAdminInspirationOrder();
            setOrderItems(result.items);
            setOrderOriginal(result.items.map((item) => item.id));
        } catch (error) {
            message.error(error instanceof Error ? error.message : "读取排序失败");
            setOrderOpen(false);
        } finally {
            setOrderLoading(false);
        }
    };

    const saveOrder = async () => {
        setOrderSaving(true);
        try {
            await saveAdminInspirationOrder(
                orderItems.map((item) => item.id),
                orderOriginal,
            );
            setOrderOpen(false);
            message.success("排序已保存");
            setRefresh((value) => value + 1);
        } catch (error) {
            message.error(error instanceof Error ? error.message : "保存排序失败");
        } finally {
            setOrderSaving(false);
        }
    };

    const columns: ColumnsType<Inspiration> = [
        {
            title: "精选灵感",
            key: "inspiration",
            render: (_, item) => (
                <div className="admin-inspiration-cell">
                    <img src={inspirationCoverUrl(item)} alt="" referrerPolicy="no-referrer" />
                    <div>
                        <strong>{item.title}</strong>
                        <p>{item.description}</p>
                        <div>
                            {item.tags.map((tag) => (
                                <Tag key={tag}>{tag}</Tag>
                            ))}
                        </div>
                    </div>
                </div>
            ),
        },
        { title: "类型", dataIndex: "mode", width: 110, render: (value: InspirationMode) => modeLabels[value] },
        { title: "状态", dataIndex: "status", width: 100, render: (value: InspirationStatus) => <AdminStatusBadge label={value === "active" ? "已启用" : "已停用"} tone={value === "active" ? "success" : "neutral"} /> },
        { title: "更新时间", dataIndex: "updatedAt", width: 160, render: (value: string) => new Date(value).toLocaleString("zh-CN", { hour12: false }) },
        {
            title: "操作",
            key: "actions",
            width: 190,
            render: (_, item) => (
                <AdminRowActions
                    primary={{ label: "编辑", icon: <PencilLine className="size-3.5" />, onClick: () => openEdit(item) }}
                    visibleActionCount={1}
                    actions={[
                        { key: "status", label: item.status === "active" ? "停用" : "启用", onClick: () => toggleStatus(item) },
                        {
                            key: "delete",
                            label: "永久删除",
                            icon: <Trash2 className="size-3.5" />,
                            danger: true,
                            onClick: () => {
                                confirmDelete([item]);
                            },
                        },
                    ]}
                />
            ),
        },
    ];

    const previewCover = draftCoverId ? inspirationDraftCoverUrl(draftCoverId) : watched?.coverKind === "url" ? watched.coverUrl || "" : editing ? inspirationCoverUrl(editing) : "";
    const previewCoverIdentity = draftCoverId || (watched?.coverKind === "url" ? watched.coverUrl?.trim() || "" : editing?.coverResourceId || "");
    const previewDimensions = coverDimensions?.source === previewCoverIdentity ? coverDimensions : null;
    const selectedItems = items.filter((item) => selected.includes(item.id));
    return (
        <>
            <AdminDataTable
                toolbar={
                    <Input
                        allowClear
                        prefix={<Search className="size-4" />}
                        value={keyword}
                        onChange={(event) => {
                            setKeyword(event.target.value);
                            setPage(1);
                        }}
                        placeholder="搜索标题、说明、标签或提示词内容"
                    />
                }
                toolbarFilters={
                    <>
                        <Select
                            value={mode}
                            onChange={(value) => {
                                setMode(value);
                                setPage(1);
                            }}
                            options={[{ value: "all", label: "全部类型" }, ...Object.entries(modeLabels).map(([value, label]) => ({ value, label }))]}
                        />
                        <Select
                            value={status}
                            onChange={(value) => {
                                setStatus(value);
                                setPage(1);
                            }}
                            options={[
                                { value: "all", label: "全部状态" },
                                { value: "active", label: "已启用" },
                                { value: "disabled", label: "已停用" },
                            ]}
                        />
                    </>
                }
                trailing={
                    <>
                        <Button icon={<RefreshCw className="size-4" />} onClick={() => setRefresh((value) => value + 1)}>
                            刷新
                        </Button>
                        <Button icon={<ListOrdered className="size-4" />} onClick={() => void openOrder()}>
                            设置排序
                        </Button>
                        <Button type="primary" icon={<Plus className="size-4" />} onClick={openCreate}>
                            新增提示词
                        </Button>
                    </>
                }
                batchActions={
                    selected.length ? (
                        <AdminBatchBar count={selected.length} onClear={() => setSelected([])}>
                            <Button danger icon={<Trash2 className="size-4" />} onClick={() => confirmDelete(selectedItems)}>
                                批量删除
                            </Button>
                        </AdminBatchBar>
                    ) : null
                }
                table={{ rowKey: "id", loading, columns, dataSource: items, pagination: false, scroll: { x: 900 }, rowSelection: { selectedRowKeys: selected, onChange: setSelected } }}
                empty={<AdminTableEmpty filtered={Boolean(search || mode !== "all" || status !== "all")} title="暂无精选灵感" />}
                footer={
                    <PaginationBar
                        current={page}
                        pageSize={pageSize}
                        total={total}
                        onChange={(next, size) => {
                            setPage(next);
                            setPageSize(size);
                        }}
                        alwaysShow
                    />
                }
            />

            <AdminModal
                centered
                open={editorOpen}
                onCancel={closeEditor}
                width="min(1040px, calc(100vw - 32px))"
                title={editing ? "编辑精选灵感" : "新增精选灵感"}
                rootClassName="admin-inspiration-editor-modal"
                maskClosable={!saving && !uploading}
                closable={!saving && !uploading}
                footer={
                    <div className="flex justify-end gap-2">
                        <Button disabled={saving || uploading} onClick={closeEditor}>
                            取消
                        </Button>
                        <Button type="primary" loading={saving} onClick={() => void save()}>
                            保存提示词
                        </Button>
                    </div>
                }
            >
                <div className="admin-inspiration-editor">
                    <Form form={form} layout="vertical" requiredMark={false}>
                        <div className="admin-inspiration-form-grid">
                            <Form.Item label="提示词标题" name="title" rules={[{ required: true, message: "请输入标题" }, { max: 120 }]}>
                                <Input placeholder="例如：雨夜霓虹电影感开场" />
                            </Form.Item>
                            <Form.Item label="创作类型" name="mode" rules={[{ required: true }]}>
                                <Select options={Object.entries(modeLabels).map(([value, label]) => ({ value, label }))} />
                            </Form.Item>
                        </div>
                        <Form.Item label="标签" name="tags" rules={[{ validator: (_, value: string | undefined) => validateTags(value) }]}>
                            <Input placeholder="用逗号分隔，最多 8 个" />
                        </Form.Item>
                        <Form.Item label="封面来源" name="coverKind">
                            <Select
                                options={[
                                    { value: "upload", label: "上传图片" },
                                    { value: "url", label: "HTTPS 外链" },
                                ]}
                            />
                        </Form.Item>
                        {watched?.coverKind === "url" ? (
                            <Form.Item
                                label="封面 URL"
                                name="coverUrl"
                                rules={[
                                    {
                                        validator: (_, value: string | undefined) => {
                                            const normalized = value?.trim() || "";
                                            if (!normalized) return Promise.reject(new Error("请输入 HTTPS 地址"));
                                            if (isHTTPSURL(normalized)) return Promise.resolve();
                                            if (editing?.coverUrl === normalized && normalized.startsWith("/")) return Promise.resolve();
                                            return Promise.reject(new Error("仅支持 HTTPS；迁移的站内封面可原样保留"));
                                        },
                                    },
                                ]}
                            >
                                <Input placeholder="https://example.com/image.webp" />
                            </Form.Item>
                        ) : (
                            <Form.Item label="上传封面">
                                <Upload accept="image/jpeg,image/png,image/webp" showUploadList={false} beforeUpload={(file) => uploadCover(file)}>
                                    <Button loading={uploading} icon={<UploadCloud className="size-4" />}>
                                        选择图片
                                    </Button>
                                </Upload>
                                <span className="ml-3 text-xs text-foreground/50">JPEG、PNG 或 WebP，最大 10 MB</span>
                            </Form.Item>
                        )}
                        <Form.Item label="提示词内容" name="prompt" rules={[{ required: true, message: "请输入提示词内容" }, { max: 20000 }]}>
                            <Input.TextArea rows={8} placeholder="写入可直接用于生成的完整提示词" />
                        </Form.Item>
                        <Form.Item label="卡片说明" name="description" rules={[{ required: true, message: "请输入卡片说明" }, { max: 240 }]}>
                            <Input.TextArea rows={3} placeholder="用于首页卡片的简短预览说明" />
                        </Form.Item>
                        <Form.Item label="来源署名" name="source" rules={[{ max: 120 }]}>
                            <Input placeholder="可选；留空时首页显示原创提示词" />
                        </Form.Item>
                    </Form>
                    <aside className="admin-inspiration-preview">
                        <span>首页卡片实时预览</span>
                        <div className="admin-inspiration-preview-card">
                            {previewCover ? (
                                <img
                                    src={previewCover}
                                    alt=""
                                    width={previewDimensions?.width}
                                    height={previewDimensions?.height}
                                    style={previewDimensions ? { aspectRatio: `${previewDimensions.width} / ${previewDimensions.height}` } : undefined}
                                    referrerPolicy="no-referrer"
                                    onLoad={(event) => {
                                        if (!previewCoverIdentity) return;
                                        const image = event.currentTarget;
                                        if (image.naturalWidth > 0 && image.naturalHeight > 0) setCoverDimensions({ source: previewCoverIdentity, width: image.naturalWidth, height: image.naturalHeight });
                                    }}
                                />
                            ) : (
                                <div className="admin-inspiration-preview-empty">等待选择封面</div>
                            )}
                            <div>
                                <strong>{watched?.title || "提示词标题"}</strong>
                                <p>{watched?.description || "卡片说明会显示在这里"}</p>
                                <small>
                                    {watched?.source ? "开源改编 · CC0" : "原创提示词"} · {modeLabels[watched?.mode || "image"]}
                                </small>
                            </div>
                        </div>
                        <p>图片按原始比例展示。预览不等于发布，新增内容保存后默认为停用状态。</p>
                    </aside>
                </div>
            </AdminModal>

            <Modal
                title="调整精选灵感顺序"
                open={orderOpen}
                onCancel={() => setOrderOpen(false)}
                width={620}
                confirmLoading={orderSaving}
                onOk={() => void saveOrder()}
                okButtonProps={{ disabled: orderLoading || orderItems.every((item, index) => item.id === orderOriginal[index]) }}
                okText="保存排序"
                cancelText="取消"
            >
                <p className="text-sm text-foreground/60">拖动条目或使用箭头调整全量顺序；第一条已启用内容显示在首页首位。</p>
                <Spin spinning={orderLoading}>
                    <div className="admin-inspiration-order-list">
                        {orderItems.map((item, index) => (
                            <div
                                key={item.id}
                                draggable={!orderSaving}
                                onDragStart={() => {
                                    dragged.current = item.id;
                                }}
                                onDragOver={(event) => event.preventDefault()}
                                onDrop={() => {
                                    if (dragged.current) setOrderItems((current) => moveOrderItem(current, dragged.current!, index));
                                    dragged.current = null;
                                }}
                            >
                                <GripVertical className="size-4" />
                                <span>
                                    {item.name}
                                    {!item.enabled ? <small>已停用</small> : index === orderItems.findIndex((row) => row.enabled) ? <small>首页首位</small> : null}
                                </span>
                                <Button icon={<ArrowUp className="size-4" />} disabled={index === 0} onClick={() => setOrderItems((current) => moveOrderItem(current, item.id, index - 1))} />
                                <Button icon={<ArrowDown className="size-4" />} disabled={index === orderItems.length - 1} onClick={() => setOrderItems((current) => moveOrderItem(current, item.id, index + 1))} />
                            </div>
                        ))}
                    </div>
                </Spin>
            </Modal>
        </>
    );
}
