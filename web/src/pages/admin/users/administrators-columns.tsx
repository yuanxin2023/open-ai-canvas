import type { ColumnsType } from "antd/es/table";
import { Eye, ShieldMinus, SlidersHorizontal, UserRoundX } from "lucide-react";

import { ADMIN_PERMISSION_GROUPS } from "@/lib/admin-permissions";
import type { AdminUser } from "@/services/api/auth";
import { IdentityProviderBadge } from "@/components/layout/identity-provider-badge";
import { AdminRowActions, AdminStatusBadge } from "../components/admin-ui";

const permissionLabels = new Map<string, string>(ADMIN_PERMISSION_GROUPS.flatMap((group) => group.items.map((item) => [item.permission, item.label])));

export type AdministratorColumnKey = "user" | "remark" | "email" | "level" | "permissions" | "status" | "createdAt" | "actions";

export const administratorColumnOptions: Array<{ key: AdministratorColumnKey; label: string; locked?: boolean }> = [
    { key: "user", label: "管理员", locked: true },
    { key: "remark", label: "备注", locked: true },
    { key: "email", label: "邮箱" },
    { key: "level", label: "管理员级别" },
    { key: "permissions", label: "模块权限" },
    { key: "status", label: "状态" },
    { key: "createdAt", label: "创建时间" },
    { key: "actions", label: "操作", locked: true },
];

export function createAdministratorColumns({
    actorId,
    visibleColumns,
    onView,
    onEdit,
    onDemote,
    onPurge,
}: {
    actorId?: string;
    visibleColumns: Set<AdministratorColumnKey>;
    onView: (user: AdminUser) => void;
    onEdit: (user: AdminUser) => void;
    onDemote: (user: AdminUser) => Promise<void>;
    onPurge: (user: AdminUser) => Promise<void>;
}): ColumnsType<AdminUser> {
    const columns: Array<ColumnsType<AdminUser>[number] & { key: AdministratorColumnKey }> = [
        {
            key: "user",
            title: "管理员",
            dataIndex: "username",
            render: (_, user) => (
                <div>
                    <div className="flex items-center gap-1.5">
                        <button type="button" className="admin-table-primary-link font-medium" onClick={() => onView(user)}>
                            {user.username}
                        </button>
                        <IdentityProviderBadge user={user} />
                    </div>
                    <div className="text-xs text-foreground/45">@{user.username}</div>
                </div>
            ),
        },
        { key: "remark", title: "备注", dataIndex: "remark", width: 180, ellipsis: true, render: (remark) => remark || <span className="text-foreground/40">未备注</span> },
        {
            key: "email",
            title: "邮箱",
            dataIndex: "email",
            width: 210,
            align: "center",
            render: (email) =>
                email ? (
                    <span className="block max-w-full truncate whitespace-nowrap" title={email}>
                        {email}
                    </span>
                ) : (
                    <span className="text-foreground/40">未填写</span>
                ),
        },
        {
            key: "level",
            title: "管理员级别",
            dataIndex: "adminAccess",
            width: 130,
            align: "center",
            render: (access) => <AdminStatusBadge label={access?.level === "full" ? "全权限管理员" : "模块管理员"} tone="info" />,
        },
        {
            key: "permissions",
            title: "模块权限",
            dataIndex: "adminAccess",
            width: 220,
            ellipsis: true,
            render: (access) => {
                if (access?.level === "full") return <span className="text-foreground/65">全部当前及未来权限</span>;
                const labels = (access?.permissions || []).map((permission: string) => permissionLabels.get(permission) || permission);
                return <span title={labels.join("、")}>{labels.length ? `${labels.slice(0, 3).join("、")}${labels.length > 3 ? ` 等 ${labels.length} 项` : ""}` : "未配置"}</span>;
            },
        },
        { key: "status", title: "状态", dataIndex: "status", width: 100, align: "center", render: (status) => <AdminStatusBadge label={status === "active" ? "已启用" : "已停用"} tone={status === "active" ? "success" : "neutral"} /> },
        {
            key: "createdAt",
            title: "创建时间",
            dataIndex: "createdAt",
            width: 112,
            align: "center",
            render: (value) => (
                <span className="tabular-nums" title={formatTime(value)}>
                    {formatCompactTime(value)}
                </span>
            ),
        },
        {
            key: "actions",
            title: "操作",
            width: 260,
            align: "center",
            render: (_, user) => (
                <AdminRowActions
                    primary={{ label: "详情", icon: <Eye className="size-3.5" />, iconOnly: true, onClick: () => onView(user) }}
                    visibleActionCount={2}
                    actions={[
                        { key: "manage", label: "管理", icon: <SlidersHorizontal className="size-3.5" />, iconOnly: true, onClick: () => onEdit(user) },
                        {
                            key: "demote",
                            label: "降级",
                            icon: <ShieldMinus className="size-3.5" />,
                            disabled: user.id === actorId,
                            confirm: {
                                title: `确认将“${user.username}”降为普通用户？`,
                                description: "该账号将立即失去后台权限，当前登录态会被撤销，并移入用户管理列表。",
                                okText: "确认降级",
                            },
                            onClick: () => onDemote(user),
                        },
                        {
                            key: "purge",
                            label: "注销",
                            icon: <UserRoundX className="size-3.5" />,
                            danger: true,
                            disabled: user.id === actorId,
                            confirm: {
                                title: `确认注销“${user.username}”？`,
                                description: "注销后将永久删除该管理员账号及其全部关联资料，此操作不可恢复。",
                                okText: "确认注销并删除",
                            },
                            onClick: () => onPurge(user),
                        },
                    ]}
                />
            ),
        },
    ];
    return columns.filter((column) => column.key === "remark" || visibleColumns.has(column.key));
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
