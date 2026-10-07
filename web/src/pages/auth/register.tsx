import { type FormEvent, useEffect, useRef, useState, type ReactNode } from "react";
import { App, Button, Divider, Input } from "antd";
import { ArrowLeft, ArrowRight, Gift, Info, KeyRound, LockKeyhole, Mail, ShieldCheck, TriangleAlert } from "lucide-react";
import { useNavigate, useSearchParams } from "react-router";

import { getAuthSession, getAuthSettings, linuxDOLoginURL, register, sendRegistrationEmailCode } from "@/services/api/auth";
import { ApiError } from "@/services/api/request";
import { LinuxDOIcon } from "./auth-scene";

type AuthSettings = Awaited<ReturnType<typeof getAuthSettings>>;
type RegistrationStep = "details" | "verification";

export default function RegisterPage() {
    const navigate = useNavigate();
    const [params] = useSearchParams();
    const { message } = App.useApp();
    const [settings, setSettings] = useState<AuthSettings | null>(null);
    const [step, setStep] = useState<RegistrationStep>("details");
    const [email, setEmail] = useState("");
    const [emailCode, setEmailCode] = useState("");
    const [password, setPassword] = useState("");
    const [confirmPassword, setConfirmPassword] = useState("");
    const [invitationCode, setInvitationCode] = useState(() => params.get("ref")?.trim().toUpperCase() || "");
    const [promoCode, setPromoCode] = useState("");
    const [codeEmail, setCodeEmail] = useState("");
    const [submitting, setSubmitting] = useState(false);
    const [sendingCode, setSendingCode] = useState(false);
    const [countdown, setCountdown] = useState(0);
    const [registerCountdown, setRegisterCountdown] = useState(0);
    const sending = useRef(false);
    const registering = useRef(false);
    const next = safeNext(params.get("next"));

    useEffect(() => {
        let cancelled = false;
        void getAuthSettings()
            .then((value) => !cancelled && setSettings(value))
            .catch((error) => !cancelled && message.error(error instanceof Error ? error.message : "读取注册设置失败"));
        return () => {
            cancelled = true;
        };
    }, [message]);

    useEffect(() => {
        if (countdown <= 0) return;
        const timer = window.setInterval(() => setCountdown((value) => Math.max(0, value - 1)), 1000);
        return () => window.clearInterval(timer);
    }, [countdown]);

    useEffect(() => {
        if (registerCountdown <= 0) return;
        const timer = window.setInterval(() => setRegisterCountdown((value) => Math.max(0, value - 1)), 1000);
        return () => window.clearInterval(timer);
    }, [registerCountdown]);

    const normalizedEmail = email.trim().toLowerCase();
    const firstUser = settings?.firstUser === true;
    const backendOutdated = Boolean(settings && settings.emailFirstRegistration !== true);
    const registrationClosed = settings?.registrationEnabled === false;
    const mailUnavailable = Boolean(settings && !firstUser && settings.emailCodeRequired && !settings.emailEnabled);
    const unavailableMessage = backendOutdated ? "注册服务仍是旧版本，请先升级或重启后端服务。" : registrationClosed ? "当前已关闭普通注册，请联系管理员创建账号。" : mailUnavailable ? "管理员尚未配置注册邮件，普通邮箱注册暂不可用。" : "";
    const formDisabled = !settings || Boolean(unavailableMessage);
    const detailsCooldown = !firstUser && countdown > 0 && codeEmail !== normalizedEmail;

    const validateDetails = () => {
        if (password !== confirmPassword) {
            message.error("两次输入的密码不一致");
            return false;
        }
        return true;
    };

    const sendCode = async (advance: boolean) => {
        if (sending.current || (!advance && countdown > 0)) return;
        if (!normalizedEmail) {
            message.warning("请先输入邮箱");
            return;
        }
        if (advance && codeEmail === normalizedEmail && countdown > 0) {
            setStep("verification");
            return;
        }
        if (countdown > 0) return;
        sending.current = true;
        setSendingCode(true);
        try {
            await sendRegistrationEmailCode(normalizedEmail);
            if (codeEmail !== normalizedEmail) setEmailCode("");
            setCodeEmail(normalizedEmail);
            setCountdown(60);
            if (advance) setStep("verification");
            message.success("验证码已发送，请检查邮箱");
        } catch (error) {
            if (error instanceof ApiError && error.status === 429) {
                setCountdown(Math.max(1, Math.ceil((error.retryAfterMs ?? 60000) / 1000)));
            }
            message.error(error instanceof Error ? error.message : "发送验证码失败");
        } finally {
            sending.current = false;
            setSendingCode(false);
        }
    };

    const completeRegistration = async (code?: string) => {
        if (registering.current || registerCountdown > 0) return;
        registering.current = true;
        setSubmitting(true);
        try {
            await register({ email: normalizedEmail, password, emailCode: code, referralCode: settings?.referralEnabled && !firstUser ? invitationCode.trim() || undefined : undefined });
            const { applyUserSession } = await import("@/lib/user-session");
            await applyUserSession(await getAuthSession());
            if (!firstUser) window.sessionStorage.setItem("infinite-canvas:model-setup-guide", "1");
            message.success(firstUser ? "管理员账号已创建" : "注册成功");
            navigate(next, { replace: true });
        } catch (error) {
            if (error instanceof ApiError && error.status === 429) setRegisterCountdown(Math.max(1, Math.ceil((error.retryAfterMs ?? 60000) / 1000)));
            message.error(error instanceof Error ? error.message : "注册失败");
        } finally {
            registering.current = false;
            setSubmitting(false);
        }
    };

    const submitDetails = async (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        if (formDisabled || !validateDetails()) return;
        if (firstUser) {
            await completeRegistration();
            return;
        }
        await sendCode(true);
    };

    const submitVerification = async (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        if (!/^\d{6}$/.test(emailCode)) {
            message.error("请输入 6 位邮箱验证码");
            return;
        }
        await completeRegistration(emailCode);
    };

    return (
        <>
            {unavailableMessage ? (
                <Notice icon={<TriangleAlert className="size-3.5" />} tone="amber">
                    {unavailableMessage}
                </Notice>
            ) : step === "verification" ? (
                <form onSubmit={submitVerification} className="space-y-5">
                    <div className="flex items-start justify-between gap-4">
                        <div>
                            <p className="text-sm font-medium text-white/88">验证你的邮箱</p>
                            <p className="mt-1 text-xs leading-5 text-white/45">验证码已发送至 {codeEmail || normalizedEmail}，10 分钟内有效。</p>
                        </div>
                        <Button type="text" size="small" icon={<ArrowLeft className="size-3.5" />} onClick={() => setStep("details")} disabled={submitting}>
                            返回修改
                        </Button>
                    </div>

                    <AuthField label="邮箱验证码">
                        <Input
                            size="large"
                            prefix={<ShieldCheck className="size-4 text-white/35" />}
                            value={emailCode}
                            onChange={(event) => setEmailCode(event.target.value.replace(/\D/g, "").slice(0, 6))}
                            placeholder="6 位验证码"
                            inputMode="numeric"
                            autoComplete="one-time-code"
                            autoFocus
                            required
                            disabled={submitting}
                        />
                    </AuthField>

                    <Button type="primary" htmlType="submit" size="large" block loading={submitting} disabled={registerCountdown > 0} icon={<ArrowRight className="size-4" />} iconPlacement="end">
                        {registerCountdown > 0 ? `${registerCountdown} 秒后可重试` : "验证并创建账号"}
                    </Button>
                    <div className="text-center">
                        <Button type="link" htmlType="button" size="small" loading={sendingCode} disabled={submitting || countdown > 0} onClick={() => void sendCode(false)}>
                            {countdown > 0 ? `${countdown} 秒后可重新发送` : "重新发送验证码"}
                        </Button>
                    </div>
                </form>
            ) : (
                <form onSubmit={submitDetails} className="space-y-4">
                    {firstUser ? (
                        <Notice icon={<Info className="size-3.5" />} tone="blue">
                            首个账号自动成为管理员，无需邮箱验证码。
                        </Notice>
                    ) : null}

                    <AuthField label="邮箱">
                        <Input
                            size="large"
                            prefix={<Mail className="size-4 text-white/35" />}
                            value={email}
                            onChange={(event) => setEmail(event.target.value)}
                            placeholder="用于登录与安全验证"
                            type="email"
                            autoComplete="email"
                            autoFocus
                            required
                            disabled={formDisabled}
                        />
                    </AuthField>

                    <AuthField label="密码">
                        <Input.Password
                            size="large"
                            prefix={<LockKeyhole className="size-4 text-white/35" />}
                            value={password}
                            onChange={(event) => setPassword(event.target.value)}
                            placeholder="至少 8 位"
                            autoComplete="new-password"
                            minLength={8}
                            required
                            disabled={formDisabled}
                        />
                    </AuthField>

                    <AuthField label="确认密码">
                        <Input.Password
                            size="large"
                            prefix={<LockKeyhole className="size-4 text-white/35" />}
                            value={confirmPassword}
                            onChange={(event) => setConfirmPassword(event.target.value)}
                            placeholder="再次输入密码"
                            autoComplete="new-password"
                            minLength={8}
                            required
                            disabled={formDisabled}
                        />
                    </AuthField>

                    {!firstUser ? (
                        <>
                            {settings?.referralEnabled ? (
                                <AuthField label="6 位推广码（可选）">
                                    <Input
                                        size="large"
                                        prefix={<KeyRound className="size-4 text-white/35" />}
                                        value={invitationCode}
                                        onChange={(event) => setInvitationCode(event.target.value.toUpperCase())}
                                        placeholder="请输入好友的 6 位推广码"
                                        maxLength={6}
                                        autoComplete="off"
                                        disabled={formDisabled}
                                    />
                                </AuthField>
                            ) : null}
                            <AuthField label="优惠码（可选）">
                                <Input size="large" prefix={<Gift className="size-4 text-white/35" />} value={promoCode} onChange={(event) => setPromoCode(event.target.value)} placeholder="请输入优惠码" autoComplete="off" disabled={formDisabled} />
                            </AuthField>
                            <Notice icon={<Info className="size-3.5" />} tone="blue">
                                {settings?.referralEnabled ? "推广码仅在注册时绑定邀请关系；优惠码仍为后续功能预留。" : "优惠码为后续功能预留，当前不会提交或发放权益。"}
                            </Notice>
                        </>
                    ) : null}

                    <Button
                        type="primary"
                        htmlType="submit"
                        size="large"
                        block
                        loading={firstUser ? submitting : sendingCode}
                        disabled={formDisabled || registerCountdown > 0 || detailsCooldown}
                        icon={<ArrowRight className="size-4" />}
                        iconPlacement="end"
                    >
                        {registerCountdown > 0 ? `${registerCountdown} 秒后可重试` : detailsCooldown ? `${countdown} 秒后可重试` : firstUser ? "创建管理员账号" : "继续"}
                    </Button>
                </form>
            )}

            {settings?.linuxdoEnabled ? (
                <>
                    <Divider plain className="!border-white/10 !text-white/30">
                        或
                    </Divider>
                    <Button size="large" block icon={<LinuxDOIcon />} href={linuxDOLoginURL(next)}>
                        使用 Linux.do 登录
                    </Button>
                </>
            ) : null}
        </>
    );
}

function AuthField({ label, children }: { label: string; children: ReactNode }) {
    return (
        <label className="block space-y-2">
            <span className="text-xs font-medium text-white/62">{label}</span>
            {children}
        </label>
    );
}

function Notice({ icon, tone, children }: { icon: ReactNode; tone: "blue" | "amber"; children: ReactNode }) {
    return (
        <div className={`flex items-start gap-2 rounded-lg border px-3 py-2.5 text-xs leading-5 ${tone === "blue" ? "border-blue-300/15 bg-blue-300/[0.06] text-blue-100/78" : "border-amber-300/15 bg-amber-300/[0.06] text-amber-100/78"}`}>
            <span className="mt-0.5 shrink-0">{icon}</span>
            {children}
        </div>
    );
}

function safeNext(value: string | null) {
    if (!value || !value.startsWith("/") || value.startsWith("//")) return "/";
    return value;
}
