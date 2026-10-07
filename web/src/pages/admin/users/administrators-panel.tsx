import { App, Button, Dropdown, Input, Select } from "antd";
import { Search, Settings2, UserPlus } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { Checkbox } from "@/pages/admin/ui/controls";
import { PaginationBar } from "@/pages/admin/components/admin-ui";
import { useDebouncedValue } from "@/hooks/use-debounced-value";
import { demoteAdministrator, listAdministrators, purgeAdministrator, type AdminManagedUser, type AdminUser, type LocalUser } from "@/services/api/auth";
import type { CreditAccount } from "@/services/api/wallet";
import { useUserStore } from "@/stores/use-user-store";
import { AdminDataTable, AdminTableEmpty } from "../components/admin-ui";
import { AdminUserDetailModal } from "../components/admin-user-detail-drawer";
import { useTableUrlState } from "../lib/use-table-url-state";
import { AdministratorCreateDrawer } from "./administrator-create-drawer";
import { AdministratorEditModal } from "./administrator-edit-modal";
import { administratorColumnOptions, createAdministratorColumns, type AdministratorColumnKey } from "./administrators-columns";

const columnStorageKey = "admin-administrators-visible-columns-v1";
const allColumnKeys = administratorColumnOptions.map((item) => item.key);

export default function AdministratorsPanel({ onUserChanged, onUserDeleted }: { onUserChanged?: (user: LocalUser) => void; onUserDeleted?: (userId: string) => void }) {
    const actor = useUserStore((state) => state.user);
    const { message } = App.useApp();
    const { state, update } = useTableUrlState();
    const [filterDraft, setFilterDraft] = useState(state.filter);
    const [isFilterComposing, setIsFilterComposing] = useState(false);
    const debouncedFilterDraft = useDebouncedValue(isFilterComposing ? state.filter : filterDraft);
    const [users, setUsers] = useState<AdminUser[]>([]);
    const [total, setTotal] = useState(0);
    const [loading, setLoading] = useState(true);
    const [loadError, setLoadError] = useState("");
    const [retry, setRetry] = useState(0);
    const [detailUserId, setDetailUserId] = useState<string | null>(null);
    const [editingUser, setEditingUser] = useState<AdminUser | null>(null);
    const [createOpen, setCreateOpen] = useState(false);
    const [visibleColumns, setVisibleColumns] = useState<Set<AdministratorColumnKey>>(() => {
        if (typeof window === "undefined") return new Set(allColumnKeys);
        try {
            const saved = JSON.parse(window.localStorage.getItem(columnStorageKey) || "[]") as AdministratorColumnKey[];
            const valid = saved.filter((key) => allColumnKeys.includes(key));
            return new Set(valid.length ? [...valid, "user", "remark", "actions"] : allColumnKeys);
        } catch {
            return new Set(allColumnKeys);
        }
    });
    const requestSequence = useRef(0);
    const editReturnFocusRef = useRef<HTMLElement | null>(null);
    const pendingFilterUrlRef = useRef<string | null>(null);
    const skipFilterUrlCommitRef = useRef(false);
    const hasFilters = Boolean(filterDraft || state.status !== "all");
    const detailIndex = detailUserId ? users.findIndex((user) => user.id === detailUserId) : -1;
    const previousUserId = detailIndex > 0 ? users[detailIndex - 1]?.id : undefined;
    const nextUserId = detailIndex >= 0 && detailIndex < users.length - 1 ? users[detailIndex + 1]?.id : undefined;

    useEffect(() => window.localStorage.setItem(columnStorageKey, JSON.stringify([...visibleColumns])), [visibleColumns]);

    useEffect(() => {
        if (pendingFilterUrlRef.current === state.filter) {
            pendingFilterUrlRef.current = null;
            return;
        }
        pendingFilterUrlRef.current = null;
        skipFilterUrlCommitRef.current = true;
        setFilterDraft(state.filter);
    }, [state.filter]);

    useEffect(() => {
        if (skipFilterUrlCommitRef.current) {
            skipFilterUrlCommitRef.current = false;
            return;
        }
        if (isFilterComposing || debouncedFilterDraft === state.filter) return;
        pendingFilterUrlRef.current = debouncedFilterDraft;
        update({ filter: debouncedFilterDraft, page: 1 }, true);
    }, [debouncedFilterDraft, isFilterComposing, state.filter, update]);

    useEffect(() => {
        const sequence = ++requestSequence.current;
        setLoading(true);
        setLoadError("");
        void listAdministrators({
            keyword: state.filter || undefined,
            status: state.status === "all" ? undefined : state.status,
            page: state.page,
            pageSize: state.pageSize,
        })
            .then((result) => {
                if (sequence !== requestSequence.current) return;
                setUsers(result.users);
                setTotal(result.total);
                if (result.total > 0 && result.users.length === 0 && state.page > 1) update({ page: 1 }, true);
            })
            .catch((error) => {
                if (sequence !== requestSequence.current) return;
                const text = error instanceof Error ? error.message : "读取管理员失败";
                setLoadError(text);
                message.error(text);
            })
            .finally(() => sequence === requestSequence.current && setLoading(false));
    }, [message, retry, state.filter, state.page, state.pageSize, state.status, update]);

    const replaceUser = useCallback(
        (nextUser: LocalUser | AdminManagedUser) => {
            setUsers((items) => items.map((item) => (item.id === nextUser.id ? { ...item, ...nextUser } : item)));
            setEditingUser((current) => (current?.id === nextUser.id ? { ...current, ...nextUser } : current));
            onUserChanged?.(nextUser);
        },
        [onUserChanged],
    );

    const replaceCreditAccount = useCallback((account: CreditAccount) => {
        setUsers((items) => items.map((item) => (item.id === account.userId ? { ...item, availableMicrocredits: account.availableMicrocredits, reservedMicrocredits: account.reservedMicrocredits } : item)));
    }, []);

    const removeUser = useCallback((userId: string) => {
        setUsers((items) => items.filter((item) => item.id !== userId));
        setTotal((value) => Math.max(0, value - 1));
        setDetailUserId((current) => (current === userId ? null : current));
        setEditingUser((current) => (current?.id === userId ? null : current));
    }, []);

    const columns = useMemo(
        () =>
            createAdministratorColumns({
                actorId: actor?.id,
                visibleColumns,
                onView: (user) => setDetailUserId(user.id),
                onEdit: (user) => {
                    editReturnFocusRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
                    setEditingUser(user);
                },
                onDemote: async (user) => {
                    try {
                        const result = await demoteAdministrator(user.id);
                        removeUser(user.id);
                        onUserChanged?.(result.user);
                        message.success("管理员已降为普通用户，账号登录态已撤销");
                    } catch (error) {
                        message.error(error instanceof Error ? error.message : "降级管理员失败");
                    }
                },
                onPurge: async (user) => {
                    try {
                        await purgeAdministrator(user.id);
                        removeUser(user.id);
                        onUserDeleted?.(user.id);
                        message.success("管理员账号及关联数据已注销");
                    } catch (error) {
                        message.error(error instanceof Error ? error.message : "注销管理员失败");
                    }
                },
            }),
        [actor?.id, message, onUserChanged, onUserDeleted, removeUser, visibleColumns],
    );

    return (
        <>
            <AdminDataTable
                toolbar={
                    <Input
                        allowClear
                        autoComplete="off"
                        className="app-list-search"
                        prefix={<Search className="size-4 text-foreground/40" />}
                        value={filterDraft}
                        aria-label="搜索管理员"
                        placeholder="搜索用户名、邮箱或备注"
                        onChange={(event) => setFilterDraft(event.target.value)}
                        onCompositionStart={() => setIsFilterComposing(true)}
                        onCompositionEnd={(event) => {
                            setFilterDraft(event.currentTarget.value);
                            setIsFilterComposing(false);
                        }}
                    />
                }
                toolbarActive={hasFilters}
                onReset={() => {
                    setFilterDraft("");
                    update({ filter: "", status: "all", page: 1 });
                }}
                toolbarFilters={
                    <Select
                        aria-label="筛选管理员状态"
                        className="w-32"
                        value={state.status}
                        options={[
                            { value: "all", label: "全部状态" },
                            { value: "active", label: "已启用" },
                            { value: "disabled", label: "已停用" },
                        ]}
                        onChange={(status) => update({ status, page: 1 })}
                    />
                }
                trailing={
                    <div className="flex items-center gap-2">
                        <Button icon={<UserPlus className="size-4" />} onClick={() => setCreateOpen(true)}>
                            添加管理员
                        </Button>
                        <Dropdown
                            trigger={["click"]}
                            popupRender={() => (
                                <div className="w-48 rounded-md border border-border bg-popover p-2 shadow-lg">
                                    <div className="px-2 pb-2 text-xs font-medium text-foreground/55">显示列</div>
                                    <div className="space-y-0.5">
                                        {administratorColumnOptions.map((option) => (
                                            <label key={option.key} className="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 text-sm hover:bg-muted/60">
                                                <Checkbox
                                                    bare
                                                    checked={option.locked || visibleColumns.has(option.key)}
                                                    disabled={option.locked}
                                                    onChange={(event) =>
                                                        setVisibleColumns((current) => {
                                                            const next = new Set(current);
                                                            if (event.target.checked) next.add(option.key);
                                                            else next.delete(option.key);
                                                            return next;
                                                        })
                                                    }
                                                />
                                                {option.label}
                                            </label>
                                        ))}
                                    </div>
                                </div>
                            )}
                        >
                            <Button icon={<Settings2 className="size-4" />}>列设置</Button>
                        </Dropdown>
                    </div>
                }
                skeletonColumns={Math.max(4, columns.length)}
                table={{ className: "app-data-table admin-users-table", size: "small", rowKey: "id", loading, columns, dataSource: users, pagination: false, scroll: { x: 1120 } }}
                empty={
                    loadError ? (
                        <div className="admin-inline-load-error" role="status">
                            <span>管理员数据暂不可用：{loadError}</span>
                            <Button size="small" onClick={() => setRetry((value) => value + 1)}>
                                重试
                            </Button>
                        </div>
                    ) : (
                        <AdminTableEmpty filtered={hasFilters} />
                    )
                }
                footer={<PaginationBar alwaysShow current={state.page} pageSize={state.pageSize} total={total} onChange={(page, pageSize) => update({ page: pageSize !== state.pageSize ? 1 : page, pageSize })} />}
            />

            <AdminUserDetailModal accountKind="administrator" userId={detailUserId} previousUserId={previousUserId} nextUserId={nextUserId} onNavigate={setDetailUserId} onClose={() => setDetailUserId(null)} />
            <AdministratorCreateDrawer
                open={createOpen}
                onClose={() => setCreateOpen(false)}
                onCompleted={(user) => {
                    onUserChanged?.(user);
                    setRetry((value) => value + 1);
                }}
            />
            <AdministratorEditModal
                user={editingUser}
                actorId={actor?.id}
                onClose={() => {
                    setEditingUser(null);
                    window.setTimeout(() => {
                        if (editReturnFocusRef.current?.isConnected) editReturnFocusRef.current.focus();
                    });
                }}
                onSaved={replaceUser}
                onCreditsAdjusted={replaceCreditAccount}
            />
        </>
    );
}
