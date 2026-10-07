import { App, Button, Input } from "antd";
import { CalendarDays, Camera, Check, Copy, IdCard, LockKeyhole, Mail, Save, Trash2, UserRound } from "lucide-react";
import { type ChangeEvent, type FormEvent, type ReactNode, useEffect, useMemo, useRef, useState } from "react";
import { useLocation, useNavigate } from "react-router";

import { UserAvatar } from "@/components/layout/user-avatar";
import { AppModal } from "@/components/ui/product/app-modal";
import { normalizeUsername, usernameValidationMessage } from "@/lib/username";
import { changePassword, updateProfile } from "@/services/api/auth";
import { resourceFileUrl, uploadResourceFile } from "@/services/api/resources";
import { useUserStore, type LocalUser } from "@/stores/use-user-store";

const MAX_AVATAR_BYTES = 2 << 20;
const AVATAR_TYPES = new Set(["image/jpeg", "image/png", "image/webp"]);
export default function ProfilePage() {
    const { message, modal } = App.useApp();
    const navigate = useNavigate();
    const location = useLocation();
    const inputRef = useRef<HTMLInputElement>(null);
    const user = useUserStore((state) => state.user);
    const setUser = useUserStore((state) => state.setUser);
    const initialUsername = user?.username || "";
    const [username, setUsername] = useState(initialUsername);
    const [avatarResourceId, setAvatarResourceId] = useState(user?.avatarResourceId || "");
    const [avatarUrl, setAvatarUrl] = useState(user?.avatarUrl || "");
    const [saving, setSaving] = useState(false);
    const [uploading, setUploading] = useState(false);
    const [changingPassword, setChangingPassword] = useState(false);
    const [currentPassword, setCurrentPassword] = useState("");
    const [newPassword, setNewPassword] = useState("");
    const [confirmPassword, setConfirmPassword] = useState("");
    const [copied, setCopied] = useState(false);
    const [activeSection, setActiveSection] = useState<"profile" | "password">("profile");

    useEffect(() => {
        const nextUsername = user?.username || "";
        setUsername(nextUsername);
        setAvatarResourceId(user?.avatarResourceId || "");
        setAvatarUrl(user?.avatarUrl || "");
    }, [user]);

    const normalizedUsername = normalizeUsername(username);
    const usernameChanged = normalizedUsername !== normalizeUsername(initialUsername);
    const usernameError = usernameChanged ? usernameValidationMessage(normalizedUsername) : "";
    const unchanged = !usernameChanged && avatarResourceId === (user?.avatarResourceId || "");
    const registeredAt = useMemo(() => formatRegistrationTime(user?.createdAt), [user?.createdAt]);
    const returnTo = profileReturnPath(location.state);
    const busy = saving || uploading || changingPassword;

    if (!user) return null;

    const close = () => {
        if (busy) return;
        navigate(returnTo, { replace: true });
    };

    const saveProfile = async () => {
        setSaving(true);
        try {
            const result = await updateProfile({ username: usernameChanged ? normalizedUsername : initialUsername, avatarResourceId });
            setUser(result.user);
            message.success("个人资料已保存");
            navigate(returnTo, { replace: true });
        } catch (error) {
            message.error(error instanceof Error ? error.message : "保存个人资料失败");
        } finally {
            setSaving(false);
        }
    };

    const submit = (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();
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

    const submitPassword = async (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        if (!currentPassword) {
            message.warning("请输入当前密码");
            return;
        }
        if ([...newPassword].length < 8) {
            message.warning("新密码至少需要 8 个字符");
            return;
        }
        if (newPassword !== confirmPassword) {
            message.warning("两次输入的新密码不一致");
            return;
        }
        if (newPassword === currentPassword) {
            message.warning("新密码不能与当前密码相同");
            return;
        }

        setChangingPassword(true);
        try {
            await changePassword({ currentPassword, newPassword });
            setCurrentPassword("");
            setNewPassword("");
            setConfirmPassword("");
            message.success("密码已修改，其他设备需要重新登录");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "修改密码失败");
        } finally {
            setChangingPassword(false);
        }
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

    const previewUser = { ...user, avatarUrl: avatarUrl || undefined };

    return (
        <AppModal flush centered open title={null} footer={null} width="min(960px, calc(100vw - 24px))" maskClosable={!busy} keyboard={!busy} closable={!busy} onCancel={close}>
            <div className="app-user-workspace flex h-[720px] max-h-[calc(100vh-24px)] min-h-0 flex-col overflow-hidden bg-[var(--user-surface)] text-[var(--user-ink)] sm:flex-row">
                <aside className="shrink-0 border-b border-[var(--user-border)] bg-[var(--user-sidebar-bg)] px-4 py-4 sm:w-52 sm:border-r sm:border-b-0 sm:px-5 sm:py-6">
                    <h1 className="px-2 text-base font-semibold tracking-[-0.01em] sm:text-lg">账户管理</h1>
                    <nav className="mt-3 grid grid-cols-2 gap-1 sm:mt-5 sm:grid-cols-1" aria-label="账户管理">
                        <button
                            type="button"
                            aria-pressed={activeSection === "profile"}
                            className={`flex min-w-0 items-center gap-2 rounded-xl px-3 py-2.5 text-left text-sm font-medium transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 ${activeSection === "profile" ? "bg-[var(--control-selected-bg)] text-[var(--control-selected-fg)]" : "text-[var(--user-ink-muted)] hover:bg-[var(--user-surface-hover)] hover:text-[var(--user-ink)]"}`}
                            disabled={busy}
                            onClick={() => setActiveSection("profile")}
                        >
                            <UserRound className="size-4 shrink-0" />
                            <span className="truncate">个人主页</span>
                        </button>
                        <button
                            type="button"
                            aria-pressed={activeSection === "password"}
                            className={`flex min-w-0 items-center gap-2 rounded-xl px-3 py-2.5 text-left text-sm font-medium transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 ${activeSection === "password" ? "bg-[var(--control-selected-bg)] text-[var(--control-selected-fg)]" : "text-[var(--user-ink-muted)] hover:bg-[var(--user-surface-hover)] hover:text-[var(--user-ink)]"}`}
                            disabled={busy}
                            onClick={() => setActiveSection("password")}
                        >
                            <LockKeyhole className="size-4 shrink-0" />
                            <span className="truncate">密码重置</span>
                        </button>
                    </nav>
                </aside>

                <main className="flex min-h-0 min-w-0 flex-1 flex-col bg-[var(--user-surface)]">
                    <header className="shrink-0 border-b border-[var(--user-border)] px-5 py-5 pr-14 sm:px-7 sm:py-6 sm:pr-14">
                        <h2 className="text-xl font-semibold tracking-[-0.02em]">{activeSection === "profile" ? "个人主页" : "密码重置"}</h2>
                        <p className="mt-1 text-sm text-[var(--user-ink-muted)]">{activeSection === "profile" ? "管理头像和登录用户名，查看账户注册信息。" : "验证当前密码后，为账户设置新的登录密码。"}</p>
                    </header>

                    {activeSection === "profile" ? (
                        <>
                            <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain">
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
                                                <Button htmlType="button" icon={<Camera className="size-4" />} loading={uploading} onClick={() => inputRef.current?.click()}>
                                                    {avatarResourceId ? "更换头像" : "上传头像"}
                                                </Button>
                                                {avatarResourceId || avatarUrl ? (
                                                    <Button
                                                        htmlType="button"
                                                        danger
                                                        icon={<Trash2 className="size-4" />}
                                                        disabled={uploading}
                                                        onClick={() => {
                                                            setAvatarResourceId("");
                                                            setAvatarUrl("");
                                                        }}
                                                    >
                                                        移除
                                                    </Button>
                                                ) : null}
                                            </div>
                                        </div>
                                    </section>

                                    <section className="border-t border-[var(--user-border)] p-5 sm:p-7">
                                        <h3 className="text-base font-semibold">公开资料</h3>
                                        <label htmlFor="profile-username" className="mt-4 mb-2 block text-sm font-medium">
                                            登录用户名
                                        </label>
                                        <Input
                                            id="profile-username"
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
                                        <p className="mt-1 text-sm text-[var(--user-ink-muted)]">以下信息用于识别账户，不能在此修改。</p>
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
                                        <Button htmlType="button" size="large" onClick={close}>
                                            取消
                                        </Button>
                                        <Button type="primary" htmlType="submit" size="large" icon={<Save className="size-4" />} loading={saving} disabled={uploading || Boolean(usernameError) || unchanged}>
                                            保存资料
                                        </Button>
                                    </div>
                                </form>
                            </div>
                        </>
                    ) : (
                        <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain p-5 sm:p-7">
                            <form className="overflow-hidden rounded-xl border border-[var(--user-border)] bg-[var(--user-surface)]" onSubmit={submitPassword}>
                                <div className="border-b border-[var(--user-border)] px-4 py-4 sm:px-5">
                                    <div className="flex items-center gap-2">
                                        <LockKeyhole className="size-4 text-[var(--user-ink-soft)]" />
                                        <h3 className="text-base font-semibold">修改登录密码</h3>
                                    </div>
                                    <p className="mt-1 text-sm text-[var(--user-ink-muted)]">修改成功后保留当前登录，其他设备需要重新登录。</p>
                                </div>
                                <div className="space-y-5 bg-[var(--user-surface-muted)] p-4 sm:p-5">
                                    <div>
                                        <label htmlFor="current-password" className="mb-2 block text-sm font-medium">
                                            当前密码
                                        </label>
                                        <Input.Password id="current-password" size="large" value={currentPassword} onChange={(event) => setCurrentPassword(event.target.value)} autoComplete="current-password" disabled={changingPassword} />
                                    </div>
                                    <div>
                                        <label htmlFor="new-password" className="mb-2 block text-sm font-medium">
                                            新密码
                                        </label>
                                        <Input.Password id="new-password" size="large" value={newPassword} onChange={(event) => setNewPassword(event.target.value)} autoComplete="new-password" disabled={changingPassword} />
                                        <p className="mt-2 text-xs text-[var(--user-ink-muted)]">密码至少需要 8 个字符。</p>
                                    </div>
                                    <div>
                                        <label htmlFor="confirm-password" className="mb-2 block text-sm font-medium">
                                            确认新密码
                                        </label>
                                        <Input.Password id="confirm-password" size="large" value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} autoComplete="new-password" disabled={changingPassword} />
                                    </div>
                                    <div className="flex justify-end pt-1">
                                        <Button type="primary" htmlType="submit" size="large" icon={<LockKeyhole className="size-4" />} loading={changingPassword} disabled={!currentPassword || !newPassword || !confirmPassword}>
                                            修改密码
                                        </Button>
                                    </div>
                                </div>
                            </form>
                        </div>
                    )}
                </main>
            </div>
        </AppModal>
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

function profileReturnPath(state: unknown) {
    if (!state || typeof state !== "object" || !("profileReturnTo" in state)) return "/";
    const value = (state as { profileReturnTo?: unknown }).profileReturnTo;
    return typeof value === "string" && value.startsWith("/") && !value.startsWith("//") && !value.startsWith("/profile") ? value : "/";
}
