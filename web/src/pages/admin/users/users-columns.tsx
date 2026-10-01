import type { ColumnsType } from "antd/es/table";
import { Eye, SlidersHorizontal, UserRoundX } from "lucide-react";

import { formatCredits } from "@/constant/credits";
import { IdentityProviderBadge } from "@/components/layout/identity-provider-badge";
import { AdminRowActions, AdminStatusBadge } from "../components/admin-ui";
import type { AdminUser } from "@/services/api/auth";

export type UserColumnKey = "user" | "email" | "credits" | "role" | "status" | "createdAt" | "actions";

export const userColumnOptions: Array<{ key: UserColumnKey; label: string; locked?: boolean }> = [
    { key: "user", label: "用户", locked: true },
    { key: "email", label: "邮箱" },
    { key: "credits", label: "当前积分" },
    { key: "role", label: "角色" },
    { key: "status", label: "状态" },
    { key: "createdAt", label: "注册时间" },
    { key: "actions", label: "操作", locked: true },
];

export function createUserColumns({
    actorId,
    visibleColumns,
    onView,
    onEdit,
    onPurge,
}: {
    actorId?: string;
    visibleColumns: Set<UserColumnKey>;
    onView: (user: AdminUser) => void;
    onEdit: (user: AdminUser) => void;
    onPurge: (user: AdminUser) => Promise<void>;
}): ColumnsType<AdminUser> {
    const columns: Array<ColumnsType<AdminUser>[number] & { key: UserColumnKey }> = [
        {
            key: "user",
            title: "用户",
            dataIndex: "username",
            render: (_, user) => (
                <div>
                    <div className="flex items-center gap-1.5"><button type="button" className="admin-table-primary-link font-medium" onClick={() => onView(user)}>{user.username}</button><IdentityProviderBadge user={user} /></div>
                    <div className="text-xs text-foreground/45">@{user.username}</div>
                </div>
            ),
        },
        { key: "email", title: "邮箱", dataIndex: "email", align: "center", render: (email) => email || <span className="text-foreground/40">未填写</span> },
        {
            key: "credits",
            title: "当前积分",
            dataIndex: "availableMicrocredits",
            width: 130,
            align: "center",
            render: (value, user) => <span className="tabular-nums" title={`冻结积分：${formatCredits(user.reservedMicrocredits)}`}>{formatCredits(value)}</span>,
        },
        { key: "role", title: "角色", dataIndex: "role", width: 110, align: "center", render: (role) => <AdminStatusBadge label={role === "admin" ? "管理员" : "普通用户"} tone={role === "admin" ? "info" : "neutral"} /> },
        { key: "status", title: "状态", dataIndex: "status", width: 110, align: "center", render: (status) => <AdminStatusBadge label={status === "active" ? "已启用" : "已停用"} tone={status === "active" ? "success" : "neutral"} /> },
        { key: "createdAt", title: "注册时间", dataIndex: "createdAt", width: 112, align: "center", render: (value) => <span className="tabular-nums" title={formatTime(value)}>{formatCompactTime(value)}</span> },
        {
            key: "actions",
            title: "操作",
            width: 230,
            align: "center",
            render: (_, user) => (
                <AdminRowActions
                    primary={{ label: "详情", icon: <Eye className="size-3.5" />, onClick: () => onView(user) }}
                    visibleActionCount={2}
                    actions={[
                        { key: "manage", label: "管理", icon: <SlidersHorizontal className="size-3.5" />, onClick: () => onEdit(user) },
                        {
                            key: "purge",
                            label: "注销",
                            icon: <UserRoundX className="size-3.5" />,
                            danger: true,
                            disabled: user.id === actorId,
                            confirm: {
                                title: `确认注销“${user.username}”？`,
                                description: "注销后将永久删除该用户账号，以及该用户产生的画布、项目、素材、任务、积分、支付记录和其他全部资料。此操作不可恢复。",
                                okText: "确认注销并删除",
                            },
                            onClick: () => onPurge(user),
                        },
                    ]}
                />
            ),
        },
    ];
    return columns.filter((column) => visibleColumns.has(column.key));
}

function formatTime(value?: string) {
    return value ? new Date(value).toLocaleString("zh-CN", { hour12: false }) : "--";
}

function formatCompactTime(value?: string) {
    if (!value) return "--";
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return "--";
    return date.toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false });
}
