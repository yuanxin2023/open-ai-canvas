import { Button, Input, Skeleton } from "antd";
import type { ColumnsType } from "antd/es/table";
import { ArrowLeft } from "lucide-react";
import { useState } from "react";

import { getControlPreviewUser, getControlPreviewUsers, type PreviewModel, type PreviewOverview, type PreviewPeriod, type PreviewUser, type PreviewUserDetail } from "@/services/api/control-preview";
import { AdminDataTable, AdminStatTile, PaginationBar } from "../components/admin-ui";
import { AdminDrawer } from "../ui/overlays";
import { PreviewEmpty, PreviewError } from "./preview-controls";
import { previewCapabilities, previewCredits, previewModelKey, previewNumber, previewPercent, previewTime } from "./preview-utils";
import { usePreviewQuery } from "./use-preview-query";

export type PreviewSelection = { kind: "ranking" } | { kind: "model"; row: PreviewModel } | { kind: "models" } | { kind: "user"; id: string };

export function previewModelColumns(onModel?: (row: PreviewModel) => void): ColumnsType<PreviewModel> {
    return [
        {
            title: "模型",
            dataIndex: "model",
            ellipsis: true,
            render: (value, row) =>
                onModel ? (
                    <button type="button" className="admin-preview-link" onClick={() => onModel(row)}>
                        {value}
                    </button>
                ) : (
                    value
                ),
        },
        { title: "能力", dataIndex: "capability", width: 70, render: (value) => previewCapabilities.find((item) => item.value === value)?.label || value },
        { title: "生成次数", dataIndex: "generations", width: 95, align: "right", render: previewNumber },
        { title: "成功 / 失败 / 进行中", width: 165, align: "right", render: (_, row) => `${previewNumber(row.succeeded)} / ${previewNumber(row.failed)} / ${previewNumber(row.pending)}` },
        { title: "消费积分", dataIndex: "consumedMicrocredits", width: 130, align: "right", render: previewCredits },
    ];
}

export function PreviewDetails({ stack, onStackChange, period, interval, overview }: { stack: PreviewSelection[]; onStackChange: (stack: PreviewSelection[]) => void; period: PreviewPeriod; interval: number; overview?: PreviewOverview }) {
    const selection = stack.at(-1);
    const title = selection?.kind === "user" ? "用户消费明细" : selection?.kind === "models" ? "完整模型统计" : selection?.kind === "model" ? `${selection.row.model} · 用户统计` : "完整消费排行";
    const openUser = (id: string) => onStackChange([...stack, { kind: "user", id }]);
    return (
        <AdminDrawer
            open={Boolean(selection)}
            title={title}
            width="min(960px, calc(100vw - 24px))"
            rootClassName="admin-preview-drawer"
            onClose={() => onStackChange([])}
            extra={
                stack.length > 1 ? (
                    <Button icon={<ArrowLeft className="size-4" />} onClick={() => onStackChange(stack.slice(0, -1))}>
                        返回
                    </Button>
                ) : undefined
            }
        >
            {selection && (
                <div className="admin-preview-detail" key={JSON.stringify(selection) + period}>
                    <p className="admin-preview-note">{period === "day" ? "今日" : period === "week" ? "本周" : "本月"} · 北京时间 · 仅普通用户</p>
                    {selection.kind === "user" ? (
                        <UserDetail id={selection.id} period={period} interval={interval} />
                    ) : selection.kind === "models" ? (
                        <ModelsTable models={overview?.models || []} onModel={(row) => onStackChange([...stack, { kind: "model", row }])} />
                    ) : (
                        <UserTable period={period} interval={interval} model={selection.kind === "model" ? selection.row : undefined} onUser={openUser} />
                    )}
                </div>
            )}
        </AdminDrawer>
    );
}

function UserTable({ period, interval, model, onUser }: { period: PreviewPeriod; interval: number; model?: PreviewModel; onUser: (id: string) => void }) {
    const [page, setPage] = useState(1);
    const [keyword, setKeyword] = useState("");
    const query = usePreviewQuery(["users", period, model?.model, model?.capability, page, keyword], (signal) => getControlPreviewUsers({ period, page, pageSize: 20, keyword, model: model?.model, capability: model?.capability }, signal), interval);
    const columns: ColumnsType<PreviewUser> = [
        { title: "排名", dataIndex: "rank", width: 65 },
        {
            title: "用户",
            dataIndex: "username",
            render: (value, row) => (
                <button className="admin-preview-link" type="button" onClick={() => onUser(row.userId)}>
                    {value}
                </button>
            ),
        },
        { title: "消费积分", dataIndex: "consumedMicrocredits", align: "right", render: previewCredits },
        { title: model ? "该模型消费占比" : "全体消费占比", dataIndex: "share", align: "right", render: previewPercent },
        { title: "生成次数", dataIndex: "generations", align: "right", render: previewNumber },
    ];
    return (
        <>
            <PreviewError error={query.error} stale={Boolean(query.data)} retry={() => void query.refetch({ cancelRefetch: false })} />
            <AdminDataTable
                toolbar={
                    <Input.Search
                        aria-label="搜索排行用户"
                        placeholder="搜索用户名或用户 ID"
                        allowClear
                        onSearch={(value) => {
                            setKeyword(value.trim());
                            setPage(1);
                        }}
                    />
                }
                table={{ rowKey: "userId", columns, dataSource: query.data?.users || [], loading: query.isPending, pagination: false, scroll: { x: 640 } }}
                empty={<PreviewEmpty />}
                footer={<PaginationBar alwaysShow current={page} pageSize={20} pageSizeOptions={[20]} total={query.data?.total || 0} onChange={setPage} />}
            />
        </>
    );
}

function ModelsTable({ models, onModel }: { models: PreviewModel[]; onModel?: (row: PreviewModel) => void }) {
    const [page, setPage] = useState(1);
    return (
        <AdminDataTable
            table={{ rowKey: previewModelKey, columns: previewModelColumns(onModel), dataSource: models.slice((page - 1) * 20, page * 20), pagination: false, scroll: { x: 680 } }}
            empty={<PreviewEmpty />}
            footer={<PaginationBar alwaysShow current={page} pageSize={20} pageSizeOptions={[20]} total={models.length} onChange={setPage} />}
        />
    );
}

function UserDetail({ id, period, interval }: { id: string; period: PreviewPeriod; interval: number }) {
    const [page, setPage] = useState(1);
    const query = usePreviewQuery(["user", id, period, page], (signal) => getControlPreviewUser(id, { period, page, pageSize: 20 }, signal), interval);
    const detail = query.data;
    const columns: ColumnsType<PreviewUserDetail["entries"][number]> = [
        { title: "结算时间（北京）", dataIndex: "createdAt", width: 155, render: (value) => previewTime(value, true) },
        { title: "模型", dataIndex: "model", ellipsis: true },
        { title: "计费订单", dataIndex: "billingOrderId", ellipsis: true },
        { title: "消费积分", dataIndex: "consumedMicrocredits", width: 130, align: "right", render: previewCredits },
    ];
    return (
        <>
            <PreviewError error={query.error} stale={Boolean(detail)} retry={() => void query.refetch({ cancelRefetch: false })} />
            {query.isPending && <Skeleton active paragraph={{ rows: 5 }} />}
            {detail && (
                <>
                    <h3>{detail.user.username}</h3>
                    <div className="admin-preview-detail-stats">
                        <AdminStatTile label="消费积分" value={previewCredits(detail.user.consumedMicrocredits)} />
                        <AdminStatTile label="生成次数" value={previewNumber(detail.user.generations)} />
                    </div>
                    <h3>模型使用与消费</h3>
                    <ModelsTable models={detail.models} />
                    <h3>消费记录</h3>
                    <AdminDataTable
                        table={{ rowKey: "id", columns, dataSource: detail.entries, pagination: false, scroll: { x: 620 } }}
                        empty={<PreviewEmpty />}
                        footer={<PaginationBar alwaysShow current={page} pageSize={20} pageSizeOptions={[20]} total={detail.total} onChange={setPage} />}
                    />
                </>
            )}
        </>
    );
}
