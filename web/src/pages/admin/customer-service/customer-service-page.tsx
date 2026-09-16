import { App, Button, Input, InputNumber, Segmented, Slider, Switch } from "antd";
import { ImagePlus, LoaderCircle, MessageCircle, RotateCcw, Save, Upload, X } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";

import { AdminPageFrame } from "@/pages/admin/components/admin-shell";
import { getAdminCustomerService, updateAdminCustomerService, uploadCustomerServiceButtonImage, type AdminCustomerService, type CustomerServiceDisplayType, type CustomerServicePosition } from "@/services/api/customer-service";
import { cn } from "@/lib/utils";

type EditableCustomerService = Pick<AdminCustomerService, "enabled" | "position" | "displayType" | "color" | "label" | "imageResourceId" | "draggable" | "desktopEnabled" | "mobileEnabled" | "buttonSize" | "offsetX" | "offsetY" | "tutorialUrl">;

const positionOptions = [
    { label: "右下", value: "bottom-right" },
    { label: "左下", value: "bottom-left" },
    { label: "右上", value: "top-right" },
    { label: "左上", value: "top-left" },
];

const displayOptions = [
    { label: "圆形图标", value: "circle" },
    { label: "文字胶囊", value: "pill" },
    { label: "图标＋文字", value: "icon-text" },
    { label: "自定义 PNG", value: "custom-image" },
];

function editableSetting(setting: AdminCustomerService): EditableCustomerService {
    return {
        enabled: setting.enabled,
        position: setting.position,
        displayType: setting.displayType,
        color: setting.color,
        label: setting.label,
        imageResourceId: setting.imageResourceId,
        draggable: setting.draggable,
        desktopEnabled: setting.desktopEnabled,
        mobileEnabled: setting.mobileEnabled,
        buttonSize: Number.isFinite(setting.buttonSize) && setting.buttonSize >= 20 && setting.buttonSize <= 96 ? setting.buttonSize : 56,
        offsetX: setting.offsetX,
        offsetY: setting.offsetY,
        tutorialUrl: setting.tutorialUrl || "",
    };
}
export default function CustomerServicePage() {
    const { message } = App.useApp();
    const [setting, setSetting] = useState<AdminCustomerService | null>(null);
    const [draft, setDraft] = useState<EditableCustomerService | null>(null);
    const [loading, setLoading] = useState(true);
    const [saving, setSaving] = useState(false);
    const [uploading, setUploading] = useState(false);
    const [loadError, setLoadError] = useState("");
    const [localPreview, setLocalPreview] = useState("");
    const uploadInputRef = useRef<HTMLInputElement>(null);

    const load = useCallback(async () => {
        setLoading(true);
        setLoadError("");
        try {
            const result = await getAdminCustomerService();
            setSetting(result);
            setDraft(editableSetting(result));
        } catch (error) {
            setLoadError(error instanceof Error ? error.message : "读取客服配置失败");
        } finally {
            setLoading(false);
        }
    }, []);

    useEffect(() => {
        void load();
    }, [load]);

    useEffect(
        () => () => {
            if (localPreview) URL.revokeObjectURL(localPreview);
        },
        [localPreview],
    );

    const dirty = useMemo(() => Boolean(setting && draft && JSON.stringify(editableSetting(setting)) !== JSON.stringify(draft)), [draft, setting]);
    const imagePreview = draft?.imageResourceId ? localPreview || setting?.public.imageUrl || "" : "";

    const patchDraft = <K extends keyof EditableCustomerService>(key: K, value: EditableCustomerService[K]) => {
        setDraft((current) => (current ? { ...current, [key]: value } : current));
    };

    const save = async () => {
        if (!draft) return;
        if (draft.displayType === "custom-image" && !draft.imageResourceId) {
            message.warning("请先上传 PNG 按钮图片");
            return;
        }
        const tutorialUrl = draft.tutorialUrl.trim();
        if (tutorialUrl) {
            try {
                const parsed = new URL(tutorialUrl);
                if (!/^https?:$/.test(parsed.protocol) || !parsed.hostname) throw new Error("invalid tutorial URL");
            } catch {
                message.warning("使用教程链接必须是有效的 HTTP 或 HTTPS 地址");
                return;
            }
        }
        setSaving(true);
        try {
            const updated = await updateAdminCustomerService({ ...draft, tutorialUrl });
            if (updated.schemaVersion < 3 || updated.buttonSize !== draft.buttonSize || updated.tutorialUrl !== tutorialUrl) {
                throw new Error("当前运行的后端版本尚未支持使用教程配置，请更新并重启后端服务后再保存");
            }
            setSetting(updated);
            setDraft(editableSetting(updated));
            if (localPreview) {
                URL.revokeObjectURL(localPreview);
                setLocalPreview("");
            }
            window.dispatchEvent(new CustomEvent("customer-service-config-updated"));
            message.success("客服配置已保存");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "保存客服配置失败");
        } finally {
            setSaving(false);
        }
    };

    const uploadImage = async (file: File) => {
        if (file.type !== "image/png") {
            message.warning("仅支持 PNG 图片");
            return;
        }
        if (file.size > 2 * 1024 * 1024) {
            message.warning("PNG 图片不能超过 2MB");
            return;
        }
        setUploading(true);
        try {
            const resource = await uploadCustomerServiceButtonImage(file);
            if (localPreview) URL.revokeObjectURL(localPreview);
            setLocalPreview(URL.createObjectURL(file));
            setDraft((current) => (current ? { ...current, imageResourceId: resource.id, displayType: "custom-image" } : current));
            message.success("图片已上传，保存配置后生效");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "上传客服按钮图片失败");
        } finally {
            setUploading(false);
            if (uploadInputRef.current) uploadInputRef.current.value = "";
        }
    };

    return (
        <AdminPageFrame
            title="客服配置"
            description="控制用户端客服入口的显示、位置、外观与拖动行为"
            scroll
            actions={
                <Button type="primary" icon={saving ? <LoaderCircle className="size-4 animate-spin" /> : <Save className="size-4" />} disabled={!dirty || loading || saving || uploading} onClick={() => void save()}>
                    保存配置
                </Button>
            }
        >
            <div className="admin-settings-stack mx-auto w-full max-w-6xl space-y-4 pb-8">
                {loadError ? (
                    <section className="admin-settings-section border border-destructive/25 p-6">
                        <p className="text-sm text-destructive">{loadError}</p>
                        <Button className="mt-4" icon={<RotateCcw className="size-4" />} onClick={() => void load()}>
                            重新读取
                        </Button>
                    </section>
                ) : loading || !draft ? (
                    <section className="admin-settings-section grid min-h-48 place-items-center">
                        <LoaderCircle className="size-6 animate-spin text-foreground/45" />
                    </section>
                ) : (
                    <div className="grid items-start gap-4 xl:grid-cols-[minmax(0,1fr)_360px]">
                        <div className="space-y-4">
                            <SettingsSection title="使用教程" description="配置用户端左侧“帮助”菜单中的使用教程入口。">
                                <label className="block space-y-2 text-xs font-semibold text-foreground/65">
                                    教程链接
                                    <Input
                                        value={draft.tutorialUrl}
                                        maxLength={2048}
                                        placeholder="https://docs.example.com/guide"
                                        onChange={(event) => patchDraft("tutorialUrl", event.target.value)}
                                    />
                                </label>
                                <p className="text-xs leading-5 text-foreground/45">支持 HTTP 或 HTTPS 地址；留空时用户端不显示“使用教程”选项。</p>
                            </SettingsSection>

                            <SettingsSection title="显示范围" description="总开关关闭后，用户端不加载 Chatwoot，也不会显示客服入口。">
                                <SettingRow title="显示客服入口" description="控制整个用户端客服功能是否启用。">
                                    <Switch checked={draft.enabled} onChange={(value) => patchDraft("enabled", value)} />
                                </SettingRow>
                                <SettingRow title="桌面端显示" description="在桌面浏览器和大屏设备上显示。">
                                    <Switch checked={draft.desktopEnabled} onChange={(value) => patchDraft("desktopEnabled", value)} />
                                </SettingRow>
                                <SettingRow title="移动端显示" description="在手机和窄屏设备上显示。">
                                    <Switch checked={draft.mobileEnabled} onChange={(value) => patchDraft("mobileEnabled", value)} />
                                </SettingRow>
                            </SettingsSection>

                            <SettingsSection title="位置与移动" description="设置首次出现的位置；允许移动后，用户最后的位置会保存在自己的浏览器中。">
                                <div className="space-y-2">
                                    <span className="text-xs font-semibold text-foreground/65">默认位置</span>
                                    <Segmented block options={positionOptions} value={draft.position} onChange={(value) => patchDraft("position", value as CustomerServicePosition)} />
                                </div>
                                <div className="grid gap-4 sm:grid-cols-2">
                                    <label className="space-y-2 text-xs font-semibold text-foreground/65">
                                        水平边距
                                        <InputNumber className="w-full" min={8} max={200} addonAfter="px" value={draft.offsetX} onChange={(value) => patchDraft("offsetX", value ?? 24)} />
                                    </label>
                                    <label className="space-y-2 text-xs font-semibold text-foreground/65">
                                        垂直边距
                                        <InputNumber className="w-full" min={8} max={200} addonAfter="px" value={draft.offsetY} onChange={(value) => patchDraft("offsetY", value ?? 24)} />
                                    </label>
                                </div>
                                <SettingRow title="允许用户拖动" description="支持鼠标和触控拖动，并记住该浏览器中的最后位置。">
                                    <Switch checked={draft.draggable} onChange={(value) => patchDraft("draggable", value)} />
                                </SettingRow>
                            </SettingsSection>

                            <SettingsSection title="按钮外观" description="自定义的是网站客服入口；聊天窗口内容仍由 Chatwoot 管理。">
                                <div className="space-y-2">
                                    <span className="text-xs font-semibold text-foreground/65">显示形态</span>
                                    <Segmented block options={displayOptions} value={draft.displayType} onChange={(value) => patchDraft("displayType", value as CustomerServiceDisplayType)} />
                                </div>
                                <div className="grid gap-4 sm:grid-cols-2">
                                    <label className="space-y-2 text-xs font-semibold text-foreground/65">
                                        按钮文字
                                        <Input maxLength={20} value={draft.label} onChange={(event) => patchDraft("label", event.target.value)} />
                                    </label>
                                    <label className="space-y-2 text-xs font-semibold text-foreground/65">
                                        主题颜色
                                        <div className="flex gap-2">
                                            <input
                                                aria-label="选择客服按钮颜色"
                                                type="color"
                                                className="h-8 w-11 cursor-pointer rounded-md border border-border bg-transparent p-1"
                                                value={draft.color}
                                                onChange={(event) => patchDraft("color", event.target.value.toUpperCase())}
                                            />
                                            <Input value={draft.color} maxLength={7} onChange={(event) => patchDraft("color", event.target.value.toUpperCase())} />
                                        </div>
                                    </label>
                                </div>
                                <div className="space-y-2">
                                    <div className="flex items-center justify-between gap-4">
                                        <span className="text-xs font-semibold text-foreground/65">按钮大小</span>
                                        <InputNumber className="w-28" min={20} max={96} addonAfter="px" value={draft.buttonSize} onChange={(value) => patchDraft("buttonSize", value ?? 56)} />
                                    </div>
                                    <Slider min={20} max={96} value={draft.buttonSize} onChange={(value) => patchDraft("buttonSize", value)} />
                                    <p className="text-xs text-foreground/45">可设置 20–96 px，文字按钮按高度缩放，自定义图片按正方形尺寸缩放。</p>
                                </div>
                                <div className="rounded-xl border border-border/70 bg-surface-subtle p-4">
                                    <div className="flex flex-wrap items-center justify-between gap-3">
                                        <div className="flex items-center gap-3">
                                            <span className="grid size-10 place-items-center rounded-lg bg-background text-foreground/55">
                                                <ImagePlus className="size-5" />
                                            </span>
                                            <div>
                                                <strong className="block text-sm">自定义 PNG 按钮</strong>
                                                <small className="text-xs text-foreground/45">透明背景效果最佳，最大 2MB</small>
                                            </div>
                                        </div>
                                        <div className="flex gap-2">
                                            {draft.imageResourceId ? (
                                                <Button
                                                    icon={<X className="size-4" />}
                                                    onClick={() => {
                                                        patchDraft("imageResourceId", "");
                                                        patchDraft("displayType", "circle");
                                                        if (localPreview) {
                                                            URL.revokeObjectURL(localPreview);
                                                            setLocalPreview("");
                                                        }
                                                    }}
                                                >
                                                    移除
                                                </Button>
                                            ) : null}
                                            <Button icon={uploading ? <LoaderCircle className="size-4 animate-spin" /> : <Upload className="size-4" />} loading={uploading} onClick={() => uploadInputRef.current?.click()}>
                                                上传 PNG
                                            </Button>
                                            <input
                                                ref={uploadInputRef}
                                                hidden
                                                type="file"
                                                accept="image/png,.png"
                                                onChange={(event) => {
                                                    const file = event.target.files?.[0];
                                                    if (file) void uploadImage(file);
                                                }}
                                            />
                                        </div>
                                    </div>
                                </div>
                            </SettingsSection>
                        </div>

                        <section className="admin-settings-section sticky top-4 overflow-hidden border border-border/60">
                            <div className="admin-settings-section-summary px-5 py-4">
                                <h2 className="text-sm font-semibold">用户端预览</h2>
                                <p className="mt-1 text-xs text-foreground/45">预览按钮形态、颜色与默认位置</p>
                            </div>
                            <div className="admin-settings-section-content p-4">
                                <div className="relative h-[420px] overflow-hidden rounded-xl border border-border/70 bg-[radial-gradient(circle_at_top,#ffffff10,transparent_55%)]">
                                    <div className="absolute inset-x-0 top-0 border-b border-border/50 px-4 py-3 text-xs text-foreground/40">用户工作区</div>
                                    {draft.enabled ? <PreviewButton draft={draft} imagePreview={imagePreview} /> : <div className="grid h-full place-items-center text-xs text-foreground/35">客服入口已关闭</div>}
                                </div>
                            </div>
                        </section>
                    </div>
                )}
            </div>
        </AdminPageFrame>
    );
}

function SettingsSection({ title, description, children }: { title: string; description: string; children: ReactNode }) {
    return (
        <section className="admin-settings-section border border-border/60">
            <div className="admin-settings-section-summary px-5 py-4">
                <h2 className="text-sm font-semibold">{title}</h2>
                <p className="mt-1 text-xs text-foreground/45">{description}</p>
            </div>
            <div className="admin-settings-section-content space-y-5 p-5">{children}</div>
        </section>
    );
}

function SettingRow({ title, description, children }: { title: string; description: string; children: ReactNode }) {
    return (
        <div className="flex items-center justify-between gap-6">
            <div>
                <strong className="block text-sm font-medium">{title}</strong>
                <p className="mt-1 text-xs leading-5 text-foreground/45">{description}</p>
            </div>
            <div className="shrink-0">{children}</div>
        </div>
    );
}

function PreviewButton({ draft, imagePreview }: { draft: EditableCustomerService; imagePreview: string }) {
    const horizontal = draft.position.endsWith("right") ? { right: Math.min(draft.offsetX, 120) } : { left: Math.min(draft.offsetX, 120) };
    const vertical = draft.position.startsWith("bottom") ? { bottom: Math.min(draft.offsetY, 120) } : { top: Math.max(56, Math.min(draft.offsetY, 120)) };
    const customImage = draft.displayType === "custom-image" && imagePreview;
    const square = draft.displayType === "circle" || draft.displayType === "custom-image";
    const iconSize = Math.max(12, Math.min(28, Math.round(draft.buttonSize * 0.38)));
    return (
        <button
            type="button"
            className={cn(
                "absolute z-10 inline-flex select-none items-center justify-center gap-2 text-white shadow-xl",
                draft.displayType === "circle" && "rounded-full",
                draft.displayType === "pill" && "rounded-full",
                draft.displayType === "icon-text" && "rounded-xl",
                draft.displayType === "custom-image" && "overflow-hidden rounded-2xl",
            )}
            style={{ ...horizontal, ...vertical, width: square ? draft.buttonSize : undefined, height: draft.buttonSize, paddingInline: square ? undefined : Math.round(draft.buttonSize * 0.36), backgroundColor: customImage ? "transparent" : draft.color }}
        >
            {customImage ? (
                <img src={imagePreview} alt="" className="size-full object-contain" />
            ) : (
                <>
                    <MessageCircle size={iconSize} />
                    {draft.displayType !== "circle" ? <span className="whitespace-nowrap text-sm font-semibold">{draft.label}</span> : null}
                </>
            )}
        </button>
    );
}
