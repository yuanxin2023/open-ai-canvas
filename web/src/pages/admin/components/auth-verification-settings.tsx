import { App, Button, Skeleton } from "antd";
import { RefreshCw, RotateCcw, Save, ShieldCheck } from "lucide-react";
import { useEffect, useState } from "react";

import { getVerificationPolicy, saveVerificationPolicy, type VerificationPolicy } from "@/services/api/verification";
import { Switch } from "@/pages/admin/ui/controls";
import { SettingsSectionCard } from "./admin-ui";

const fields: { key: keyof VerificationPolicy; label: string }[] = [
    { key: "emailRegistration", label: "邮箱验证码注册" },
    { key: "smsRegistration", label: "短信验证码注册" },
    { key: "smsAndEmailRegistration", label: "短信加邮箱双重验证注册" },
    { key: "emailLogin", label: "邮箱验证码登录" },
    { key: "smsLogin", label: "短信验证码登录" },
];

export function AuthVerificationSettings({ refreshKey, onDirtyChange, onBusyChange }: { refreshKey: number; onDirtyChange: (dirty: boolean) => void; onBusyChange: (busy: boolean) => void }) {
    const { message } = App.useApp();
    const [stored, setStored] = useState<VerificationPolicy | null>(null);
    const [draft, setDraft] = useState<VerificationPolicy | null>(null);
    const [error, setError] = useState("");
    const [retry, setRetry] = useState(0);
    const [saving, setSaving] = useState(false);
    const dirty = Boolean(stored && draft && fields.some(({ key }) => stored[key] !== draft[key]));

    useEffect(() => {
        onDirtyChange(dirty);
    }, [dirty, onDirtyChange]);
    useEffect(() => {
        onBusyChange(saving);
    }, [saving, onBusyChange]);
    useEffect(() => {
        let active = true;
        setError("");
        setStored(null);
        setDraft(null);
        void getVerificationPolicy()
            .then((value) => {
                if (active) {
                    setStored(value);
                    setDraft(value);
                }
            })
            .catch((cause) => {
                if (active) setError(cause instanceof Error ? cause.message : "读取验证方式失败");
            });
        return () => {
            active = false;
        };
    }, [refreshKey, retry]);

    const change = (key: keyof VerificationPolicy, enabled: boolean) => {
        if (!draft || saving) return;
        const next = { ...draft, [key]: enabled };
        if (enabled && key === "smsAndEmailRegistration") next.smsRegistration = next.emailRegistration = false;
        if (enabled && (key === "smsRegistration" || key === "emailRegistration")) next.smsAndEmailRegistration = false;
        setDraft(next);
    };

    const save = async () => {
        if (!draft || saving) return;
        setSaving(true);
        setError("");
        try {
            const result = await saveVerificationPolicy(draft);
            setStored(result);
            setDraft(result);
            message.success("验证方式已保存");
        } catch (cause) {
            setError(cause instanceof Error ? cause.message : "保存验证方式失败");
        } finally {
            setSaving(false);
        }
    };

    return (
        <SettingsSectionCard
            icon={<ShieldCheck className="size-4" />}
            title="4. 登录与注册验证方式"
            description="短信及双重验证默认关闭。开启前配置对应场景的短信渠道或邮件服务；双重验证不能与单通道注册同时开启。账号密码登录继续可用。"
            footer={
                <div className="flex flex-wrap justify-end gap-2">
                    <Button icon={<RotateCcw className="size-4" />} disabled={!dirty || saving} onClick={() => setDraft(stored)}>
                        撤销
                    </Button>
                    <Button type="primary" icon={<Save className="size-4" />} loading={saving} disabled={!draft || !dirty} onClick={() => void save()}>
                        保存验证方式
                    </Button>
                </div>
            }
        >
            {error ? (
                <div role="alert" className="mb-4 text-sm">
                    {error}
                    {!draft ? (
                        <Button type="link" icon={<RefreshCw className="size-4" />} onClick={() => setRetry((value) => value + 1)}>
                            重试
                        </Button>
                    ) : null}
                </div>
            ) : null}
            {draft ? (
                <div className="divide-y divide-[var(--user-border)]">
                    {fields.map(({ key, label }) => (
                        <div key={key} className="flex items-center justify-between gap-4 py-4">
                            <span className="text-sm">{label}</span>
                            <Switch aria-label={label} checked={draft[key]} disabled={saving} onChange={(checked) => change(key, checked)} />
                        </div>
                    ))}
                </div>
            ) : !error ? (
                <Skeleton active paragraph={{ rows: 4 }} />
            ) : null}
        </SettingsSectionCard>
    );
}
