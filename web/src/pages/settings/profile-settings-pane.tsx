import { App, Button, Input } from "antd";
import { CalendarDays, Camera, Check, Copy, IdCard, Mail, Save, Trash2, UserRound } from "lucide-react";
import { type ChangeEvent, type FormEvent, type ReactNode, useEffect, useMemo, useRef, useState } from "react";

import { UserAvatar } from "@/components/layout/user-avatar";
import { normalizeUsername, usernameValidationMessage } from "@/lib/username";
import { DEFAULT_USER_AVATARS, type DefaultUserAvatarID } from "@/lib/default-user-avatars";
import { updateProfile } from "@/services/api/auth";
import { resourceFileUrl, uploadResourceFile } from "@/services/api/resources";
import { useUserStore, type LocalUser } from "@/stores/use-user-store";

const MAX_AVATAR_BYTES = 2 << 20;
const AVATAR_TYPES = new Set(["image/jpeg", "image/png", "image/webp"]);
export function ProfileSettingsPane({ onBusyChange }: { onBusyChange: (busy: boolean) => void }) {
    const { message, modal } = App.useApp();
    const inputRef = useRef<HTMLInputElement>(null);
    const user = useUserStore((state) => state.user);
    const setUser = useUserStore((state) => state.setUser);
    const initialUsername = user?.username || "";
    const [username, setUsername] = useState(initialUsername);
    const [avatarResourceId, setAvatarResourceId] = useState(user?.avatarResourceId || "");
    const [avatarUrl, setAvatarUrl] = useState(user?.avatarUrl || "");
    const [avatarPreset, setAvatarPreset] = useState<DefaultUserAvatarID | null>(null);
    const [showAvatarPresets, setShowAvatarPresets] = useState(false);
    const [saving, setSaving] = useState(false);
    const [uploading, setUploading] = useState(false);
    const [copied, setCopied] = useState(false);
    const busy = saving || uploading;

    useEffect(() => {
        onBusyChange(busy);
    }, [busy, onBusyChange]);

    useEffect(() => {
        const nextUsername = user?.username || "";
        setUsername(nextUsername);
    }, [user?.id, user?.username]);

    useEffect(() => {
        setAvatarResourceId(user?.avatarResourceId || "");
        setAvatarUrl(user?.avatarUrl || "");
        setAvatarPreset(null);
    }, [user?.id, user?.avatarResourceId, user?.avatarUrl, user?.avatarMode, user?.avatarPreset]);

    const normalizedUsername = normalizeUsername(username);
    const usernameChanged = normalizedUsername !== normalizeUsername(initialUsername);
    const usernameError = usernameChanged ? usernameValidationMessage(normalizedUsername) : "";
    const avatarUnchanged = avatarPreset === null ? avatarResourceId === (user?.avatarResourceId || "") : user?.avatarMode === "default" && (user.avatarPreset || "person") === avatarPreset;
    const unchanged = !usernameChanged && avatarUnchanged;
    const registeredAt = useMemo(() => formatRegistrationTime(user?.createdAt), [user?.createdAt]);

    if (!user) return null;

    const resetProfile = () => {
        if (busy) return;
        setUsername(initialUsername);
        setAvatarResourceId(user.avatarResourceId || "");
        setAvatarUrl(user.avatarUrl || "");
        setAvatarPreset(null);
        setShowAvatarPresets(false);
    };

    const saveProfile = async () => {
        setSaving(true);
        try {
            const result = await updateProfile({
                username: usernameChanged ? normalizedUsername : initialUsername,
                ...(avatarPreset === null ? { avatarResourceId } : { avatarPreset }),
            });
            setUser(result.user);
            message.success("个人资料已保存");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "保存个人资料失败");
        } finally {
            setSaving(false);
        }
    };

    const submit = (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        if (busy || unchanged) return;
        if (usernameError) {
            message.warning(usernameError);
            return;
        }
        if (!usernameChanged) {
            void saveProfile();
            return;
        }

        modal.confirm({
            title: "确认修改用户名？",
            content: `修改后，您将使用“${normalizedUsername}”${user.email ? `或邮箱“${user.email}”` : ""}登录，原用户名立即失效。${usernameChangeQuotaText(user)}`,
            okText: "确定修改",
            cancelText: "取消",
            centered: true,
            onOk: saveProfile,
            onCancel: () => setUsername(initialUsername),
        });
    };

    const selectAvatar = async (event: ChangeEvent<HTMLInputElement>) => {
        const file = event.target.files?.[0];
        event.target.value = "";
        if (!file) return;
        if (!AVATAR_TYPES.has(file.type)) {
            message.warning("请选择 JPG、PNG 或 WebP 图片");
            return;
        }
        if (file.size <= 0 || file.size > MAX_AVATAR_BYTES) {
            message.warning("头像大小不能超过 2 MB");
            return;
        }
        setUploading(true);
        try {
            const resource = await uploadResourceFile(file, "image", {
                fileName: file.name,
                idempotencyKey: `profile-avatar:${user.id}:${Date.now()}:${file.lastModified}:${file.size}`,
            });
            setAvatarResourceId(resource.id);
            setAvatarUrl(resourceFileUrl(resource.id));
            setAvatarPreset(null);
            message.success("头像已上传，保存资料后生效");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "头像上传失败");
        } finally {
            setUploading(false);
        }
    };

    const copyUserID = async () => {
        try {
            await navigator.clipboard.writeText(user.id);
            setCopied(true);
            message.success("用户 ID 已复制");
            window.setTimeout(() => setCopied(false), 1500);
        } catch {
            message.error("复制失败，请手动复制");
        }
    };

    const previewUser = {
        ...user,
        avatarUrl: avatarUrl || undefined,
        avatarMode: avatarPreset !== null ? ("default" as const) : avatarResourceId !== (user.avatarResourceId || "") ? ("" as const) : user.avatarMode,
        avatarPreset: avatarPreset || user.avatarPreset,
    };

    return (
        <div className="text-[var(--user-ink)]">
            <header className="settings-pane-header">
                <div className="min-w-0">
                    <h2>个人资料</h2>
                    <p>管理头像和登录用户名，查看账号注册信息。</p>
                </div>
            </header>
            <form onSubmit={submit}>
                <section className="p-5 sm:p-7">
                    <div className="flex flex-col gap-5 sm:flex-row sm:items-center">
                        <UserAvatar user={previewUser} className="size-20 rounded-full bg-[var(--user-surface-muted)] p-1 text-[var(--user-ink-muted)] ring-1 ring-[var(--user-border)] sm:size-24" />
                        <div className="min-w-0 flex-1">
                            <div className="flex flex-wrap items-center gap-2">
                                <h3 className="truncate text-base font-semibold">{normalizedUsername || user.username}</h3>
                                <span className="rounded-full bg-[var(--user-surface-muted)] px-2.5 py-1 text-xs text-[var(--user-ink-muted)]">{user.role === "admin" ? "管理员" : "创作者"}</span>
                            </div>
                            <p className="mt-1 truncate text-sm text-[var(--user-ink-muted)]">{user.email || "未绑定邮箱"}</p>
                            <p className="mt-2 text-xs text-[var(--user-ink-muted)]">支持 JPG、PNG、WebP，文件不超过 2 MB。</p>
                        </div>
                        <div className="flex shrink-0 flex-wrap gap-2">
                            <input ref={inputRef} className="sr-only" type="file" accept="image/jpeg,image/png,image/webp" onChange={(event) => void selectAvatar(event)} />
                            <Button htmlType="button" icon={<Camera className="size-4" />} loading={uploading} disabled={busy} onClick={() => inputRef.current?.click()}>
                                {avatarResourceId ? "更换头像" : "上传头像"}
                            </Button>
                            <Button htmlType="button" icon={<UserRound className="size-4" />} disabled={busy} onClick={() => setShowAvatarPresets((value) => !value)}>
                                系统头像
                            </Button>
                            {avatarResourceId || avatarUrl ? (
                                <Button
                                    htmlType="button"
                                    danger
                                    icon={<Trash2 className="size-4" />}
                                    disabled={busy}
                                    onClick={() => {
                                        setAvatarResourceId("");
                                        setAvatarUrl("");
                                        setAvatarPreset("person");
                                    }}
                                >
                                    移除
                                </Button>
                            ) : null}
                        </div>
                    </div>
                    {showAvatarPresets ? (
                        <div className="mt-5 grid grid-cols-4 gap-2 sm:grid-cols-8" role="group" aria-label="系统头像">
                            {DEFAULT_USER_AVATARS.map((preset) => {
                                const selected = avatarPreset === preset.id || (avatarPreset === null && user.avatarMode === "default" && (user.avatarPreset || "person") === preset.id);
                                return (
                                    <button
                                        key={preset.id}
                                        type="button"
                                        aria-pressed={selected}
                                        disabled={busy}
                                        className={`flex flex-col items-center gap-2 rounded-xl border p-3 text-xs focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 ${selected ? "border-[var(--control-selected-fg)] bg-[var(--control-selected-bg)] text-[var(--control-selected-fg)]" : "border-[var(--user-border)] text-[var(--user-ink-muted)] hover:bg-[var(--user-surface-hover)]"}`}
                                        onClick={() => {
                                            setAvatarPreset(preset.id);
                                            setAvatarResourceId("");
                                            setAvatarUrl("");
                                        }}
                                    >
                                        <UserAvatar user={{ ...user, avatarMode: "default", avatarPreset: preset.id }} className="size-7" />
                                        {preset.label}
                                    </button>
                                );
                            })}
                        </div>
                    ) : null}
                </section>

                <section className="border-t border-[var(--user-border)] p-5 sm:p-7">
                    <h3 className="text-base font-semibold">公开资料</h3>
                    <label htmlFor="profile-username" className="mt-4 mb-2 block text-sm font-medium">
                        登录用户名
                    </label>
                    <Input
                        id="profile-username"
                        disabled={busy}
                        size="large"
                        prefix={<UserRound className="size-4 text-[var(--user-ink-soft)]" />}
                        value={username}
                        placeholder="输入登录用户名"
                        showCount={{ formatter: ({ value }) => `${Array.from(value).length}/9` }}
                        status={usernameError ? "error" : undefined}
                        onChange={(event) => setUsername(event.target.value)}
                        autoComplete="username"
                    />
                    <p className={`mt-2 text-xs ${usernameError ? "text-red-500" : "text-[var(--user-ink-muted)]"}`}>{usernameError || "用户名为 3–9 位，可使用中文、英文字母和数字；英文字母将自动转为小写。"}</p>
                </section>

                <section className="border-t border-[var(--user-border)] p-5 sm:p-7">
                    <h3 className="text-base font-semibold">账户信息</h3>
                    <p className="mt-1 text-sm text-[var(--user-ink-muted)]">以下信息用于识别账户；联系方式可在账号安全中验证和修改。</p>
                    <dl className="mt-5 divide-y divide-[var(--user-border)] rounded-xl border border-[var(--user-border)] px-4">
                        <InfoRow icon={<Mail />} label="邮箱" value={user.email || "未绑定邮箱"} />
                        <InfoRow icon={<CalendarDays />} label="注册时间" value={registeredAt} />
                        <div className="flex min-h-14 items-center gap-3 py-3">
                            <IdCard className="size-4 shrink-0 text-[var(--user-ink-soft)]" />
                            <dt className="w-20 shrink-0 text-sm text-[var(--user-ink-muted)]">用户 ID</dt>
                            <dd className="min-w-0 flex-1 break-all text-right text-sm">{user.id}</dd>
                            <Button type="text" htmlType="button" aria-label="复制用户 ID" icon={copied ? <Check className="size-4" /> : <Copy className="size-4" />} onClick={() => void copyUserID()} />
                        </div>
                    </dl>
                    <p className="mt-3 text-xs text-[var(--user-ink-muted)]">用户 ID 是积分、项目、画布和素材的永久关联标识，修改资料不会影响现有数据。</p>
                </section>

                <div className="flex flex-col-reverse gap-2 border-t border-[var(--user-border)] px-5 py-5 sm:flex-row sm:justify-end sm:px-7">
                    <Button htmlType="button" size="large" disabled={busy || unchanged} onClick={resetProfile}>
                        重置修改
                    </Button>
                    <Button type="primary" htmlType="submit" size="large" icon={<Save className="size-4" />} loading={saving} disabled={busy || Boolean(usernameError) || unchanged}>
                        保存资料
                    </Button>
                </div>
            </form>
        </div>
    );
}

function usernameChangeQuotaText(user: LocalUser) {
    const policy = user.usernameChangePolicy;
    if (!policy || policy.limit === null) return "";
    return policy.customized ? `本次修改将计入过去 ${policy.windowDays} 天最多 ${policy.limit} 次的限额。` : "这是首次自选用户名，不计入修改限额。";
}

function InfoRow({ icon, label, value }: { icon: ReactNode; label: string; value: string }) {
    return (
        <div className="flex min-h-14 items-center gap-3 py-3">
            <span className="text-[var(--user-ink-soft)] [&_svg]:size-4">{icon}</span>
            <dt className="w-20 shrink-0 text-sm text-[var(--user-ink-muted)]">{label}</dt>
            <dd className="min-w-0 flex-1 break-all text-right text-sm">{value}</dd>
        </div>
    );
}

function formatRegistrationTime(value?: string) {
    if (!value) return "暂无记录";
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return "暂无记录";
    return new Intl.DateTimeFormat("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" }).format(date);
}
