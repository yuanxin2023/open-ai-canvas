import { App, Button, Popconfirm, Tag } from "antd";
import copyToClipboard from "copy-to-clipboard";
import { Copy, Image as ImageIcon, Pencil, Plus, Trash2 } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";

import { PageHeader, PaginationBar, WorkspacePage } from "@/components/layout/workspace-page";
import { WorkspaceErrorState, WorkspaceLoadingState, WorkspaceState } from "@/components/layout/workspace-state";
import { UserPromptEditorModal } from "@/components/user-prompt-editor-modal";
import { resourceFileUrl } from "@/services/api/resources";
import { deleteUserPrompt, listUserPrompts, type UserPrompt, type UserPromptMode } from "@/services/api/user-prompts";
import "./prompts.css";

const modeLabels: Record<UserPromptMode, string> = { text: "文本创作", image: "图片创作", video: "视频创作" };

export default function UserPromptsPage() {
    const { message } = App.useApp();
    const [rows, setRows] = useState<UserPrompt[]>([]);
    const [loading, setLoading] = useState(true);
    const [loadError, setLoadError] = useState("");
    const [page, setPage] = useState(1);
    const [pageSize, setPageSize] = useState(20);
    const [total, setTotal] = useState(0);
    const [editing, setEditing] = useState<UserPrompt | null | undefined>();
    const [deletingID, setDeletingID] = useState("");
    const requestRef = useRef<AbortController | null>(null);

    const load = useCallback(
        async (nextPage = page, nextPageSize = pageSize) => {
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
        },
        [page, pageSize],
    );

    useEffect(() => {
        void load(1, pageSize);
        return () => requestRef.current?.abort();
    }, []); // eslint-disable-line react-hooks/exhaustive-deps

    const openEditor = (row?: UserPrompt) => {
        setEditing(row || null);
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

    return (
        <WorkspacePage className="user-prompts-page">
            <PageHeader
                title="我的提示词"
                description="保存自己的提示词记录，随时复制使用或继续沉淀创作方法。"
                actions={
                    <Button type="primary" icon={<Plus className="size-4" />} onClick={() => openEditor()}>
                        添加提示词
                    </Button>
                }
            />

            <section className="user-prompts-records" aria-labelledby="user-prompts-records-title">
                <div className="user-prompts-records-header">
                    <h2 id="user-prompts-records-title">我的记录</h2>
                    <span>共 {total} 条</span>
                </div>
                <div className="user-prompts-table-head" aria-hidden>
                    <span>标题</span>
                    <span>分类</span>
                    <span>更新时间</span>
                    <span>操作</span>
                </div>
                {loading ? (
                    <WorkspaceLoadingState label="正在读取提示词" detail="整理你的个人提示词记录" rows={3} className="px-5" />
                ) : loadError ? (
                    <WorkspaceErrorState compact description={loadError} onRetry={() => void load(page, pageSize)} />
                ) : rows.length ? (
                    <div className="user-prompts-table-body">
                        {rows.map((row) => (
                            <PromptRow key={row.id} row={row} deleting={deletingID === row.id} onCopy={() => copyPrompt(row)} onEdit={() => openEditor(row)} onDelete={() => void remove(row)} />
                        ))}
                    </div>
                ) : (
                    <WorkspaceState
                        compact
                        title="还没有保存提示词"
                        description="点击右上角“添加提示词”，建立自己的可复用提示词库。"
                        action={
                            <Button icon={<Plus className="size-4" />} onClick={() => openEditor()}>
                                添加第一条提示词
                            </Button>
                        }
                    />
                )}
            </section>

            <PaginationBar current={page} pageSize={pageSize} total={total} itemLabel="条" onChange={(nextPage, nextPageSize) => void load(nextPage, nextPageSize)} />

            <UserPromptEditorModal open={editing !== undefined} prompt={editing} onClose={() => setEditing(undefined)} onSaved={() => void load(editing ? page : 1, pageSize)} />
        </WorkspacePage>
    );
}

function PromptRow({ row, deleting, onCopy, onEdit, onDelete }: { row: UserPrompt; deleting: boolean; onCopy: () => void; onEdit: () => void; onDelete: () => void }) {
    return (
        <article className="user-prompt-row">
            <div className="user-prompt-row-main">
                <div className="user-prompt-row-cover">{row.coverResourceId || row.coverUrl ? <img src={row.coverResourceId ? resourceFileUrl(row.coverResourceId) : row.coverUrl} alt="" referrerPolicy="no-referrer" /> : <ImageIcon />}</div>
                <div className="min-w-0">
                    <h3>{row.title}</h3>
                    <p>{row.description || row.prompt}</p>
                    {row.tags.length ? (
                        <div className="user-prompt-row-tags">
                            {row.tags.slice(0, 3).map((tag) => (
                                <Tag key={tag}>{tag}</Tag>
                            ))}
                        </div>
                    ) : null}
                </div>
            </div>
            <span className="user-prompt-row-mode">{modeLabels[row.mode]}</span>
            <time dateTime={row.updatedAt}>{formatPromptTime(row.updatedAt)}</time>
            <div className="user-prompt-row-actions">
                <Button type="text" icon={<Copy className="size-4" />} onClick={onCopy}>
                    复制
                </Button>
                <Button type="text" icon={<Pencil className="size-4" />} onClick={onEdit}>
                    编辑
                </Button>
                <Popconfirm title="删除这条提示词？" description="删除后无法恢复。" okText="删除" cancelText="取消" okButtonProps={{ danger: true }} onConfirm={onDelete}>
                    <Button type="text" danger loading={deleting} icon={<Trash2 className="size-4" />} aria-label={`删除${row.title}`} />
                </Popconfirm>
            </div>
        </article>
    );
}

function formatPromptTime(value: string) {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? "—" : new Intl.DateTimeFormat("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit" }).format(date);
}
