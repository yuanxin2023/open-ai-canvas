import { App, Input } from "antd";
import type { ColumnsType } from "antd/es/table";
import { Eye, SlidersHorizontal, UserRoundX } from "lucide-react";
import { useState } from "react";

import { formatCredits } from "@/constant/credits";
import { IdentityProviderBadge } from "@/components/layout/identity-provider-badge";
import { AdminRowActions, AdminStatusBadge } from "../components/admin-ui";
import type { AdminUser } from "@/services/api/auth";
import { UserGroupBadge } from "@/components/layout/user-group-badge";
import { userGroupCode, type PublicUserGroup } from "@/services/api/user-groups";

export type UserColumnKey = "user" | "remark" | "email" | "credits" | "group" | "recharged" | "consumed" | "lastLoginAt" | "status" | "createdAt" | "actions";

export const userColumnOptions: Array<{ key: UserColumnKey; label: string; locked?: boolean }> = [
    { key: "user", label: "用户", locked: true },
    { key: "remark", label: "备注", locked: true },
    { key: "email", label: "邮箱" },
    { key: "credits", label: "当前积分" },
    { key: "group", label: "分组" },
    { key: "recharged", label: "累计充值" },
    { key: "consumed", label: "累计消耗" },
    { key: "lastLoginAt", label: "最近登录" },
    { key: "status", label: "状态" },
    { key: "createdAt", label: "注册时间" },
    { key: "actions", label: "操作", locked: true },
];

export function createUserColumns({
    groups = [],
    actorId,
    visibleColumns,
    onView,
    onEdit,
    onSaveRemark,
    onPurge,
}: {
    groups?: PublicUserGroup[];
    actorId?: string;
    visibleColumns: Set<UserColumnKey>;
    onView: (user: AdminUser) => void;
    onEdit: (user: AdminUser) => void;
    onSaveRemark: (userId: string, remark: string) => Promise<void>;
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
        {
            key: "remark",
            title: "备注",
            dataIndex: "remark",
            width: 180,
            ellipsis: true,
            render: (_, user) => <UserRemarkCell key={user.id} user={user} onSave={onSaveRemark} />,
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
        { key: "group", title: "分组", width: 130, align: "center", render: (_, user) => {
            const code = userGroupCode(user.userGroup);
            const group = groups.find((item) => item.code === code);
            return <span className="inline-flex items-center gap-1"><span>{group?.name || (code === "default" ? "默认分组" : code)}</span>{group ? <UserGroupBadge group={group} compact /> : null}</span>;
        } },
        { key: "recharged", title: "累计充值", dataIndex: "rechargedMicrocredits", width: 130, align: "center", render: (value?: number) => <span className="tabular-nums">{value === undefined ? "—" : formatCredits(value)}</span> },
        { key: "consumed", title: "累计消耗", dataIndex: "consumedMicrocredits", width: 130, align: "center", render: (value?: number) => <span className="tabular-nums">{value === undefined ? "—" : formatCredits(value)}</span> },
        { key: "lastLoginAt", title: "最近登录", dataIndex: "lastLoginAt", width: 150, align: "center", render: (value?: string) => value ? <span title={formatTime(value)}>{formatTime(value)}</span> : <span className="text-foreground/40">从未登录</span> },
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
    return columns.filter((column) => column.key === "remark" || visibleColumns.has(column.key));
}

function UserRemarkCell({ user, onSave }: { user: AdminUser; onSave: (userId: string, remark: string) => Promise<void> }) {
    const { message } = App.useApp();
    const [editing, setEditing] = useState(false);
    const [draft, setDraft] = useState(user.remark || "");
    const [saving, setSaving] = useState(false);

    const save = async () => {
        if (saving) return;
        const remark = draft.trim();
        if (remark === (user.remark || "")) {
            setEditing(false);
            return;
        }
        setSaving(true);
        try {
            await onSave(user.id, remark);
            setEditing(false);
        } catch (error) {
            message.error(error instanceof Error ? error.message : "保存备注失败");
        } finally {
            setSaving(false);
        }
    };

    if (editing) return (
        <Input
            autoFocus
            variant="borderless"
            size="small"
            maxLength={500}
            value={draft}
            readOnly={saving}
            aria-busy={saving}
            aria-label={`编辑 ${user.username} 的备注`}
            placeholder="未备注"
            onClick={(event) => event.stopPropagation()}
            onChange={(event) => setDraft(event.target.value)}
            onBlur={() => void save()}
            onKeyDown={(event) => {
                if (event.nativeEvent.isComposing) return;
                if (event.key === "Enter") {
                    event.preventDefault();
                    event.currentTarget.blur();
                }
            }}
        />
    );

    return (
        <button
            type="button"
            className="block w-full min-w-0 truncate rounded text-left hover:text-primary focus-visible:outline-2 focus-visible:outline-primary"
            aria-label={`编辑 ${user.username} 的备注`}
            title={user.remark || "点击添加备注"}
            onClick={(event) => {
                event.stopPropagation();
                setDraft(user.remark || "");
                setEditing(true);
            }}
        >
            <span className={user.remark ? undefined : "text-foreground/40"}>{user.remark || "未备注"}</span>
        </button>
    );
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
