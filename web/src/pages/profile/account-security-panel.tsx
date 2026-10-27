import { useQuery } from "@tanstack/react-query";
import { App, Button, Input, Skeleton } from "antd";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { useNavigate } from "react-router";

import { VerificationFields } from "@/components/auth/verification-fields";
import { Callout } from "@/components/ui/product/callout";
import { applyUserSession } from "@/lib/user-session";
import { invalidateAuthSessionCache } from "@/services/api/auth";
import { changeProfileEmail, changeProfilePassword, profileLinuxDOReauthURL, requestProfileEmailCode } from "@/services/api/profile";
import { ApiError } from "@/services/api/request";
import { bindVerification, emptyVerification } from "@/services/api/verification";
import { loadProfileSession } from "@/services/profile-session";
import { useUserStore } from "@/stores/use-user-store";
import { profilePasswordError, profileReauthReady, profileSecondsRemaining } from "../settings/profile-validation";

export function AccountSecurityPanel({ onBusyChange }: { onBusyChange: (busy: boolean) => void }) {
    const { message } = App.useApp();
    const navigate = useNavigate();
    const userId = useUserStore((state) => state.user?.id);
    const profileQuery = useQuery({ queryKey: ["user-profile", userId], queryFn: ({ signal }) => loadProfileSession(userId, signal), enabled: !!userId, retry: false, refetchOnWindowFocus: false, staleTime: 0 });
    const profile = profileQuery.data?.user.id === userId ? profileQuery.data : undefined;
    const [currentPassword, setCurrentPassword] = useState("");
    const [newPassword, setNewPassword] = useState("");
    const [confirmPassword, setConfirmPassword] = useState("");
    const [email, setEmail] = useState("");
    const [emailCode, setEmailCode] = useState("");
    const [emailTicket, setEmailTicket] = useState<{ value: string; expiresAt: number } | null>(null);
    const [phone, setPhone] = useState({ ...emptyVerification });
    const [retryAt, setRetryAt] = useState(0);
    const [now, setNow] = useState(Date.now());
    const [pending, setPending] = useState("");
    const pendingRef = useRef(false);
    const [error, setError] = useState("");
    const busy = Boolean(pending);
    const reauthReady = profileReauthReady(profile?.reauthenticatedUntil, now);
    const identityReady = Boolean(profile && (profile.hasPassword ? currentPassword : reauthReady));
    const sameUser = () => useUserStore.getState().user?.id === userId;
    const retrySeconds = profileSecondsRemaining(retryAt, now);
    const ticketReady = Boolean(emailTicket && profileSecondsRemaining(emailTicket.expiresAt, now));

    useEffect(() => {
        onBusyChange(busy);
    }, [busy, onBusyChange]);
    useEffect(() => {
        const timer = window.setInterval(() => setNow(Date.now()), 1000);
        return () => window.clearInterval(timer);
    }, []);

    const begin = (action: string) => {
        if (pendingRef.current || !profile || !sameUser()) return false;
        pendingRef.current = true;
        setPending(action);
        setError("");
        return true;
    };
    const end = () => {
        pendingRef.current = false;
        setPending("");
    };
    const fail = (cause: unknown) => setError(cause instanceof Error ? cause.message : "安全操作失败，请稍后重试");

    const savePassword = async (event: FormEvent) => {
        event.preventDefault();
        const validation = profilePasswordError(newPassword);
        if (validation || newPassword !== confirmPassword) {
            setError(validation || "两次输入的密码不一致");
            return;
        }
        if (!identityReady || !begin("password")) return;
        try {
            await changeProfilePassword({ currentPassword, newPassword });
            if (!sameUser()) return;
            setCurrentPassword("");
            setNewPassword("");
            setConfirmPassword("");
            setEmailTicket(null);
            setPhone({ ...emptyVerification });
            invalidateAuthSessionCache();
            message.success("密码已更新，保留当前登录，其他设备需要重新登录");
            await profileQuery.refetch();
        } catch (cause) {
            fail(cause);
        } finally {
            end();
        }
    };

    const sendEmailCode = async () => {
        if (!profile?.emailEnabled || !identityReady || retrySeconds || !begin("email-code")) return;
        setEmailTicket(null);
        setEmailCode("");
        setRetryAt(Date.now() + 60_000);
        try {
            const result = await requestProfileEmailCode({ email: email.trim(), currentPassword });
            if (!sameUser()) return;
            const received = Date.now();
            setEmailTicket({ value: result.ticket, expiresAt: received + result.expiresIn * 1000 });
            setRetryAt(received + result.retryAfter * 1000);
            setNow(received);
            message.success("验证码已发送到新邮箱");
        } catch (cause) {
            if (cause instanceof ApiError && cause.retryAfterMs) setRetryAt(Date.now() + cause.retryAfterMs);
            fail(cause);
        } finally {
            end();
        }
    };

    const saveEmail = async (event: FormEvent) => {
        event.preventDefault();
        if (!identityReady || !ticketReady || !emailTicket || !begin("email")) return;
        try {
            const result = await changeProfileEmail({ ticket: emailTicket.value, emailCode, currentPassword });
            if (!sameUser()) return;
            invalidateAuthSessionCache();
            let cleanupFailed = false;
            try {
                await applyUserSession({ user: null });
            } catch {
                cleanupFailed = true;
                useUserStore.getState().clearSession();
                useUserStore.getState().setHydrated(true);
            }
            const notice = `邮箱已更新，所有设备需要重新登录${result.notificationStatus === "failed" ? "；旧邮箱通知发送失败" : ""}${cleanupFailed ? "；本地缓存清理失败" : ""}`;
            if (result.notificationStatus === "failed" || cleanupFailed) message.warning(notice);
            else message.success(notice);
            navigate("/login", { replace: true });
        } catch (cause) {
            fail(cause);
        } finally {
            end();
        }
    };

    const savePhone = async (event: FormEvent) => {
        event.preventDefault();
        if (!phone.ticket || !/^\d{6}$/.test(phone.smsCode) || !begin("phone")) return;
        try {
            const result = await bindVerification(phone);
            if (!sameUser()) return;
            if (result.user.id !== userId) throw new Error("绑定结果与当前账号不一致，请刷新后重试");
            useUserStore.getState().setUser(result.user);
            setPhone({ ...emptyVerification });
            message.success("手机已验证并绑定");
            await profileQuery.refetch();
        } catch (cause) {
            fail(cause);
        } finally {
            end();
        }
    };

    if (profileQuery.isPending) return <Skeleton active paragraph={{ rows: 6 }} />;
    if (profileQuery.isError || !profile)
        return (
            <Callout tone="error" title="安全资料加载失败" action={<Button onClick={() => void profileQuery.refetch()}>重试</Button>}>
                {profileQuery.error instanceof Error ? profileQuery.error.message : "请稍后重试"}
            </Callout>
        );

    return (
        <div className="space-y-6">
            <h3 className="text-base font-semibold">账号安全</h3>
            {profile.hasPassword ? (
                <div>
                    <label htmlFor="security-current-password" className="mb-2 block text-sm">
                        当前密码（安全操作验证）
                    </label>
                    <Input.Password
                        id="security-current-password"
                        value={currentPassword}
                        autoComplete="current-password"
                        disabled={busy}
                        onChange={(event) => {
                            setCurrentPassword(event.target.value);
                            setEmailTicket(null);
                            setEmailCode("");
                            setPhone({ ...emptyVerification });
                        }}
                    />
                </div>
            ) : (
                <Callout
                    tone={reauthReady ? "success" : "info"}
                    title="验证当前身份"
                    action={
                        profile.linuxdoEnabled ? (
                            <Button disabled={busy} onClick={() => window.location.assign(profileLinuxDOReauthURL())}>
                                {reauthReady ? "重新验证" : "使用 Linux.do 验证"}
                            </Button>
                        ) : undefined
                    }
                >
                    {reauthReady
                        ? `身份验证有效期剩余 ${profileSecondsRemaining(Date.parse(profile.reauthenticatedUntil!), now)} 秒`
                        : profile.linuxdoEnabled
                          ? "请通过已绑定的 Linux.do 账号验证，随后可设置密码或更换邮箱。"
                          : "当前账号未设置密码，且第三方身份验证不可用，请联系管理员。"}
                </Callout>
            )}
            <form className="space-y-4 border-t border-[var(--user-border)] pt-5" onSubmit={savePassword}>
                <h4 className="font-medium">{profile.hasPassword ? "修改登录密码" : "设置登录密码"}</h4>
                <p className="text-sm text-[var(--user-ink-muted)]">更新后保留当前登录，其他设备需要重新登录。</p>
                <label className="block space-y-2">
                    <span className="text-sm">新密码</span>
                    <Input.Password value={newPassword} autoComplete="new-password" disabled={busy} onChange={(event) => setNewPassword(event.target.value)} required />
                </label>
                <label className="block space-y-2">
                    <span className="text-sm">确认新密码</span>
                    <Input.Password value={confirmPassword} autoComplete="new-password" disabled={busy} onChange={(event) => setConfirmPassword(event.target.value)} required />
                </label>
                <Button type="primary" htmlType="submit" loading={pending === "password"} disabled={busy || !identityReady || !newPassword || !confirmPassword}>
                    保存密码
                </Button>
            </form>
            <form className="space-y-4 border-t border-[var(--user-border)] pt-5" onSubmit={saveEmail}>
                <h4 className="font-medium">安全邮箱 · {profile.user.email || "未绑定"}</h4>
                <p className="text-sm text-[var(--user-ink-muted)]">更换成功后，所有设备需要重新登录。</p>
                {profile.emailEnabled ? (
                    <>
                        <label className="block space-y-2">
                            <span className="text-sm">新邮箱</span>
                            <Input
                                type="email"
                                value={email}
                                disabled={busy}
                                onChange={(event) => {
                                    setEmail(event.target.value);
                                    setEmailTicket(null);
                                    setEmailCode("");
                                }}
                                autoComplete="email"
                                required
                            />
                        </label>
                        <div className="flex items-end gap-2">
                            <label className="block min-w-0 flex-1 space-y-2">
                                <span className="text-sm">邮箱验证码</span>
                                <Input value={emailCode} disabled={busy} onChange={(event) => setEmailCode(event.target.value.replace(/\D/g, "").slice(0, 6))} autoComplete="one-time-code" inputMode="numeric" required />
                            </label>
                            <Button loading={pending === "email-code"} disabled={busy || !identityReady || !email.trim() || retrySeconds > 0} onClick={() => void sendEmailCode()}>
                                {retrySeconds ? `${retrySeconds} 秒后重发` : "获取验证码"}
                            </Button>
                        </div>
                        <Button type="primary" htmlType="submit" loading={pending === "email"} disabled={busy || !identityReady || !ticketReady || emailCode.length !== 6}>
                            更换邮箱
                        </Button>
                    </>
                ) : (
                    <p className="text-sm text-[var(--user-ink-muted)]">邮件服务暂不可用，请联系管理员。</p>
                )}
            </form>
            <form className="space-y-4 border-t border-[var(--user-border)] pt-5" onSubmit={savePhone}>
                <h4 className="font-medium">验证手机 · {profile.user.phone || "未绑定"}</h4>
                {profile.smsBindingAvailable && profile.hasPassword ? (
                    <>
                        <VerificationFields purpose="bind" method="sms" value={phone} onChange={setPhone} password={currentPassword} disabled={busy} />
                        <Button type="primary" htmlType="submit" loading={pending === "phone"} disabled={busy || !phone.ticket || phone.smsCode.length !== 6}>
                            验证并绑定手机
                        </Button>
                    </>
                ) : (
                    <p className="text-sm text-[var(--user-ink-muted)]">{!profile.hasPassword ? "请先设置登录密码，再验证手机。" : "手机绑定暂未启用。"}</p>
                )}
            </form>
            {error ? (
                <Callout tone="error" role="alert">
                    {error}
                </Callout>
            ) : null}
        </div>
    );
}
