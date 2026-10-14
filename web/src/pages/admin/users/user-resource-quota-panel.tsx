import { App, Button, InputNumber, Select, Spin } from "antd";
import { useEffect, useRef, useState } from "react";
import { getAdminUserResourceQuota, patchAdminUserResourceQuota, type RuntimeResourcePolicy, type UserResourceQuotaView } from "@/services/api/auth";
import { resourcePolicyFields } from "../lib/resource-policy-fields";
import { buildResourceQuotaPatch, type ResourceQuotaDraft } from "./user-resource-quota-patch";

export type ResourceQuotaEditorState = { dirty: boolean; busy: boolean };

export function UserResourceQuotaPanel({ userId, disabled, onStateChange }: { userId: string; disabled: boolean; onStateChange: (state: ResourceQuotaEditorState) => void }) {
    const { message } = App.useApp();
    const [view, setView] = useState<UserResourceQuotaView | null>(null);
    const [draft, setDraft] = useState<ResourceQuotaDraft>({});
    const [loading, setLoading] = useState(true);
    const [saving, setSaving] = useState(false);
    const [error, setError] = useState("");
    const [retry, setRetry] = useState(0);
    const savingRef = useRef(false);
    const patch = buildResourceQuotaPatch(view?.overrides || {}, draft);
    const dirty = Object.keys(patch).length > 0;

    useEffect(() => {
        const controller = new AbortController();
        setLoading(true);
        setError("");
        setView(null);
        setDraft({});
        void getAdminUserResourceQuota(userId, controller.signal)
            .then((next) => { if (!controller.signal.aborted) { setView(next); setDraft({ ...next.overrides }); } })
            .catch((error) => { if (!controller.signal.aborted) setError(error instanceof Error ? error.message : "读取个人配额失败"); })
            .finally(() => { if (!controller.signal.aborted) setLoading(false); });
        return () => controller.abort();
    }, [retry, userId]);

    useEffect(() => { onStateChange({ dirty, busy: saving }); }, [dirty, onStateChange, saving]);

    const save = async () => {
        if (!view || disabled || savingRef.current || !dirty) return;
        for (const field of resourcePolicyFields) {
            if (!Object.hasOwn(draft, field.name)) continue;
            const value = draft[field.name];
            if (value == null || !Number.isSafeInteger(value) || value < (field.min ?? 1) || value > field.max) {
                message.error(`${field.label}必须是 ${field.min ?? 1}-${field.max} ${field.unit}的整数`);
                return;
            }
        }
        savingRef.current = true;
        setSaving(true);
        try {
            const next = await patchAdminUserResourceQuota(userId, patch);
            setView(next);
            setDraft({ ...next.overrides });
            message.success("用户资源配额已保存");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "保存个人配额失败");
        } finally { savingRef.current = false; setSaving(false); }
    };

    if (loading) return <div className="flex min-h-64 items-center justify-center"><Spin /></div>;
    if (!view) return <div role="status" className="space-y-3"><p>{error}</p><Button onClick={() => setRetry((value) => value + 1)}>重新加载</Button></div>;

    const usage = (key: keyof RuntimeResourcePolicy) => {
        const storage = view.storageUsage;
        const values: Partial<Record<keyof RuntimeResourcePolicy, string>> = {
            storedFileGB: `${(view.storedFileBytes / 1024 ** 3).toLocaleString("zh-CN", { maximumFractionDigits: 3 })} GB`,
            dailyUploadMB: `${(view.dailyUploadBytes / 1024 ** 2).toLocaleString("zh-CN", { maximumFractionDigits: 2 })} MB`,
            structuredDataMB: `${((storage.assetBytes + storage.canvasBytes) / 1024 ** 2).toLocaleString("zh-CN", { maximumFractionDigits: 2 })} MB`,
            taskDataGB: `${(storage.taskBytes / 1024 ** 3).toLocaleString("zh-CN", { maximumFractionDigits: 3 })} GB`,
            assetCount: `${storage.assetCount.toLocaleString("zh-CN")} 条`, canvasCount: `${storage.canvasCount.toLocaleString("zh-CN")} 个`,
            taskCount: `${storage.taskCount.toLocaleString("zh-CN")} 条`, apiCallLogCount: `${storage.apiCallCount.toLocaleString("zh-CN")} 条`,
        };
        return values[key];
    };

    return (
        <div className="space-y-4">
            <p className="text-xs text-foreground/55">未单独设置的项目跟随全局默认。下调配额不会删除已有内容，超额后限制新增；回收站按生效保留天数自动清理。</p>
            {resourcePolicyFields.map((field) => {
                const custom = Object.hasOwn(draft, field.name);
                return <div key={field.name} className="rounded-lg border border-border/60 p-3">
                    <div className="mb-2 text-sm font-medium">{field.label}</div>
                    <p className="mb-2 text-xs text-foreground/50">{field.extra}</p>
                    <div className="flex flex-wrap gap-2">
                        <Select
                            className="w-32"
                            aria-label={`${field.label}配置方式`}
                            value={custom ? "custom" : "inherit"}
                            disabled={disabled || saving}
                            options={[{ value: "inherit", label: "继承全局" }, { value: "custom", label: "单独设置" }]}
                            onChange={(mode) => setDraft((current) => {
                                const next = { ...current };
                                if (mode === "inherit") delete next[field.name];
                                else next[field.name] = view.effectiveQuota[field.name];
                                return next;
                            })}
                        />
                        <InputNumber
                            className="min-w-0 flex-1"
                            aria-label={field.label}
                            value={custom ? draft[field.name] : view.defaultQuota[field.name]}
                            min={field.min ?? 1} max={field.max} precision={0} suffix={field.unit}
                            disabled={!custom || disabled || saving}
                            onChange={(value) => setDraft((current) => ({ ...current, [field.name]: value }))}
                        />
                    </div>
                    <p className="mt-2 text-xs text-foreground/50">全局默认 {view.defaultQuota[field.name]} {field.unit} · 已生效 {view.effectiveQuota[field.name]} {field.unit}{usage(field.name) ? ` · 已用 ${usage(field.name)}` : ""}</p>
                </div>;
            })}
            <div className="flex justify-end"><Button type="primary" disabled={disabled || !dirty} loading={saving} onClick={() => void save()}>保存资源配额</Button></div>
        </div>
    );
}
