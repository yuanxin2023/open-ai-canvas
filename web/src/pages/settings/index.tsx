import { translate } from "@/i18n";
import { App, Button, InputNumber } from "antd";
import { SettingsRow } from "@/components/ui/product/settings-row";
import { ArrowLeft, Boxes, Brain, Bug, CircleDollarSign, Cloud, Inbox, LockKeyhole, MessageSquareText, RadioTower, ReceiptText, SlidersHorizontal, UserRound, Workflow } from "lucide-react";
import { useEffect, useMemo, useState, type ReactNode } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";
import { LanguageOptionGrid } from "@/components/layout/language-menu";

import { UserOSSSettingsForm } from "@/components/layout/user-oss-settings-form";
import { refreshSystemChannels } from "@/lib/user-session";
import { defaultConfig, useConfigStore, useEffectiveConfig } from "@/stores/use-config-store";
import { useUserStore } from "@/stores/use-user-store";
import { ChannelSettingsPane, channelValidationError, focusInvalidChannelField, isChannelReady } from "./channel-settings-pane";
import { ModelDefaultGrid } from "./model-default-grid";
import { PromptPreferencesPane } from "./prompt-preferences-pane";
import DiagnosticsPanel from "./diagnostics-panel";
import AgentMemoryPane from "./agent-memory-pane";
import { RunningHubSettingsPane } from "./runninghub-settings-pane";
import { RUNNINGHUB_PLUGIN_ID } from "@/lib/plugins/builtin/workflows";
import { usePluginStore } from "@/stores/use-plugin-store";
import { PaymentOrdersPane } from "./payment-orders-pane";
import { openWorkspaceWallet } from "@/lib/workspace-wallet";
import { CreditLedgerPane } from "./credit-ledger-pane";
import { FeedbackInboxPane } from "./feedback-inbox-pane";
import { ProfileSettingsPane } from "./profile-settings-pane";
import { AccountSecurityPanel } from "./account-security-panel";

type ConfigSectionKey = "profile" | "security" | "orders" | "wallet" | "channels" | "models" | "runninghub" | "preferences" | "prompts" | "agent-memory" | "storage" | "diagnostics" | "feedbacks";

const configSections: Array<{ key: ConfigSectionKey; label: string; icon: ReactNode }> = [
    { key: "profile", label: translate("local.ui.profile"), icon: <UserRound className="size-4" /> },
    { key: "security", label: translate("local.ui.security"), icon: <LockKeyhole className="size-4" /> },
    { key: "orders", label: translate("local.ui.orders"), icon: <ReceiptText className="size-4" /> },
    { key: "wallet", label: translate("local.ui.creditLedger"), icon: <CircleDollarSign className="size-4" /> },
    { key: "channels", label: translate("settings.sections.channels.label"), icon: <RadioTower className="size-4" /> },
    { key: "runninghub", label: translate("settings.sections.runninghub.label"), icon: <Workflow className="size-4" /> },
    { key: "models", label: translate("settings.sections.models.label"), icon: <Boxes className="size-4" /> },
    { key: "preferences", label: translate("local.ui.generationPrefs"), icon: <SlidersHorizontal className="size-4" /> },
    { key: "prompts", label: translate("settings.sections.prompts.label"), icon: <MessageSquareText className="size-4" /> },
    { key: "agent-memory", label: translate("settings.sections.agent-memory.label"), icon: <Brain className="size-4" /> },
    { key: "storage", label: translate("settings.sections.storage.label"), icon: <Cloud className="size-4" /> },
    { key: "diagnostics", label: translate("settings.sections.diagnostics.label"), icon: <Bug className="size-4" /> },
    { key: "feedbacks", label: translate("settings.sections.feedbacks.label"), icon: <Inbox className="size-4" /> },
];

export function isConfigSection(value: string | null): value is ConfigSectionKey {
    return configSections.some((section) => section.key === value);
}

export default function SettingsPage() {
    const { message } = App.useApp();
    const { t } = useTranslation();
    const navigate = useNavigate();
    const [searchParams, setSearchParams] = useSearchParams();
    const requestedSection = searchParams.get("section");
    const customChannelsEnabled = useUserStore((state) => state.features.customChannelsEnabled);
    const creditsEnabled = useUserStore((state) => state.features.creditsEnabled);
    const feedbackEnabled = useUserStore((state) => state.features.userFeedbackEnabled);
    const runtimeStatuses = usePluginStore((state) => state.runtimeStatuses);
    const runningHubPluginEnabled = runtimeStatuses[RUNNINGHUB_PLUGIN_ID] === "enabled";
    const visibleConfigSections = useMemo(
        () =>
            (customChannelsEnabled ? configSections : configSections.filter((section) => section.key !== "channels"))
                .filter((section) => section.key !== "runninghub" || runningHubPluginEnabled)
                .filter((section) => section.key !== "feedbacks" || feedbackEnabled)
                .filter((section) => !["wallet", "orders"].includes(section.key) || creditsEnabled),
        [creditsEnabled, customChannelsEnabled, runningHubPluginEnabled, feedbackEnabled],
    );
    const isVisibleConfigSection = (value: string | null): value is ConfigSectionKey => isConfigSection(value) && visibleConfigSections.some((section) => section.key === value);
    const firstVisibleSection = visibleConfigSections[0]?.key || "models";
    const [activeTab, setActiveTab] = useState<ConfigSectionKey>(isVisibleConfigSection(requestedSection) ? requestedSection : firstVisibleSection);
    const [accountBusy, setAccountBusy] = useState(false);
    const config = useConfigStore((state) => state.config);
    const effectiveConfig = useEffectiveConfig();
    const updateConfig = useConfigStore((state) => state.updateConfig);
    const shouldPromptContinue = searchParams.get("continue") === "1";
    const userId = useUserStore((state) => state.user?.id);
    const userChannels = config.channels.filter((channel) => channel.scope !== "system");

    useEffect(() => {
        if (isVisibleConfigSection(requestedSection)) {
            setActiveTab(requestedSection);
            return;
        }
        setActiveTab(firstVisibleSection);
    }, [firstVisibleSection, requestedSection, visibleConfigSections]);

    useEffect(() => {
        if (!userId) return;
        let cancelled = false;
        void refreshSystemChannels().catch((error) => {
            if (!cancelled) message.warning(error instanceof Error ? t("account.settings.systemRefreshFailedWith", { error: error.message }) : t("account.settings.systemRefreshFailed"));
        });
        return () => {
            cancelled = true;
        };
    }, [message, t, userId]);

    const selectSection = (section: ConfigSectionKey) => {
        if (accountBusy) return;
        if (!visibleConfigSections.some((item) => item.key === section)) return;
        setActiveTab(section);
        const next = new URLSearchParams(searchParams);
        next.set("section", section);
        setSearchParams(next, { replace: true });
    };

    const finishConfig = () => {
        const invalidChannel = customChannelsEnabled ? userChannels.find((channel) => channelValidationError(channel)) : undefined;
        if (invalidChannel) {
            selectSection("channels");
            message.warning(t("account.settings.channelError", { name: invalidChannel.name || t("account.settings.channels.unnamed"), error: channelValidationError(invalidChannel) }));
            focusInvalidChannelField(invalidChannel);
            return;
        }
        const workflowReady = Boolean(runningHubPluginEnabled && config.runningHub.enabled && config.runningHub.workflowId.trim() && config.runningHub.baseUrl.trim() && config.runningHub.apiKey.trim());
        if (!effectiveConfig.channels.some(isChannelReady) && !workflowReady) {
            selectSection(customChannelsEnabled ? "channels" : "models");
            message.error(customChannelsEnabled ? (shouldPromptContinue ? t("account.settings.needChannel") : t("account.settings.noChannel")) : t("account.settings.noSystemModel"));
            return;
        }
        message.success(t("account.settings.saved"));
        navigate(-1);
    };

    const panes: Record<ConfigSectionKey, ReactNode> = {
        profile: (
            <SettingsPane>
                <ProfileSettingsPane key={userId} onBusyChange={setAccountBusy} />
            </SettingsPane>
        ),
        security: (
            <SettingsPane>
                <div className="settings-pane-header">
                    <div className="min-w-0">
                        <h2>{translate("local.ui.security")}</h2>
                        <p>{translate("local.ui.verifyIdentity")}</p>
                    </div>
                </div>
                <div className="settings-section">
                    <AccountSecurityPanel key={userId} onBusyChange={setAccountBusy} />
                </div>
            </SettingsPane>
        ),
        orders: (
            <SettingsPane>
                <PaymentOrdersPane onOpenWallet={() => openWorkspaceWallet()} />
            </SettingsPane>
        ),
        wallet: (
            <SettingsPane>
                <CreditLedgerPane />
            </SettingsPane>
        ),
        channels: (
            <SettingsPane>
                <ChannelSettingsPane onOpenModels={() => selectSection("models")} onOpenRunningHub={runningHubPluginEnabled ? () => selectSection("runninghub") : undefined} />
            </SettingsPane>
        ),
        models: (
            <SettingsPane>
                <div className="settings-pane-header">
                    <div className="min-w-0">
                        <h2>{t("settings.sections.models.label")}</h2>
                        <p>{t("account.settings.modelsDescription")}</p>
                    </div>
                </div>
                <div className="settings-section">
                    <ModelDefaultGrid config={effectiveConfig} onChange={(key, model) => updateConfig(key, model)} />
                </div>
            </SettingsPane>
        ),
        runninghub: (
            <SettingsPane>
                <RunningHubSettingsPane />
            </SettingsPane>
        ),
        preferences: (
            <SettingsPane>
                <div className="settings-pane-header">
                    <div className="min-w-0">
                        <h2>{t("settings.preferences.title")}</h2>
                        <p>{t("settings.preferences.description")}</p>
                    </div>
                </div>
                <div className="settings-section">
                    <section className="settings-preference-block">
                        <div className="settings-preference-heading">
                            <h3>{t("settings.preferences.interface")}</h3>
                            <p>{t("settings.preferences.interfaceDescription")}</p>
                        </div>
                        <SettingsRow label={t("language.label")} description={t("language.description")} align="top" control={null} />
                        <LanguageOptionGrid />
                    </section>
                    <section className="settings-preference-block">
                        <div className="settings-preference-heading">
                            <h3>{t("settings.preferences.canvas")}</h3>
                            <p>{t("settings.preferences.canvasDescription")}</p>
                        </div>
                        <SettingsRow
                            label={t("settings.preferences.imageCount")}
                            control={
                                <InputNumber
                                    min={1}
                                    max={15}
                                    precision={0}
                                    className="w-full"
                                    value={Number(config.canvasImageCount)}
                                    onChange={(value) => updateConfig("canvasImageCount", normalizeImageCount(String(value ?? defaultConfig.canvasImageCount)))}
                                />
                            }
                            controlClassName="w-[200px]"
                        />
                    </section>
                </div>
            </SettingsPane>
        ),
        prompts: (
            <SettingsPane fill>
                <PromptPreferencesPane />
            </SettingsPane>
        ),
        "agent-memory": (
            <SettingsPane>
                <div className="settings-pane-header">
                    <div className="min-w-0">
                        <h2>{t("settings.sections.agent-memory.label")}</h2>
                        <p>{t("account.settings.agentMemoryDescription")}</p>
                    </div>
                </div>
                <div className="settings-section">
                    <AgentMemoryPane />
                </div>
            </SettingsPane>
        ),
        diagnostics: (
            <SettingsPane>
                <DiagnosticsPanel taskId={searchParams.get("taskId") || undefined} projectId={searchParams.get("projectId") || undefined} />
            </SettingsPane>
        ),
        storage: (
            <SettingsPane>
                <div className="settings-section">
                    <UserOSSSettingsForm />
                </div>
            </SettingsPane>
        ),
        feedbacks: (
            <SettingsPane>
                <FeedbackInboxPane />
            </SettingsPane>
        ),
    };

    return (
        <main className="settings-page app-workspace-page app-user-workspace flex h-full min-h-0 flex-col text-foreground">
            {shouldPromptContinue ? (
                <div className="settings-topbar shrink-0">
                    <div className="ml-auto flex flex-wrap items-center justify-end gap-2">
                        <Button icon={<ArrowLeft className="size-4" />} disabled={accountBusy} onClick={() => navigate(-1)}>
                            {translate("settings.back")}
                        </Button>
                        <Button type="primary" disabled={accountBusy} onClick={finishConfig}>
                            {translate("settings.saveAndBack")}
                        </Button>
                    </div>
                </div>
            ) : null}
            <div className="settings-library-frame flex min-h-0 flex-1 flex-col md:flex-row">
                <aside className="settings-nav-panel w-full shrink-0 md:w-[200px]">
                    <nav className="thin-scrollbar flex gap-1 overflow-x-auto p-2 md:block md:space-y-1 md:overflow-x-hidden md:p-2.5" aria-label={translate("settings.nav")}>
                        {visibleConfigSections.map((item) => {
                            const selected = item.key === activeTab;
                            return (
                                <button
                                    key={item.key}
                                    type="button"
                                    disabled={accountBusy}
                                    className={`settings-nav-item flex h-9 shrink-0 items-center gap-2 rounded-md px-3 text-left text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring md:h-auto md:w-full md:items-start md:gap-3 md:py-2 ${selected ? "is-active" : "text-foreground/58 hover:bg-muted/55 hover:text-foreground"}`}
                                    onClick={() => selectSection(item.key)}
                                    aria-current={selected ? "page" : undefined}
                                >
                                    <span className="shrink-0 md:mt-0.5">{item.icon}</span>
                                    <span className="min-w-0">
                                        <span className="block whitespace-nowrap text-sm font-medium">{item.label}</span>
                                    </span>
                                </button>
                            );
                        })}
                    </nav>
                </aside>
                <section className="settings-content flex min-h-0 min-w-0 flex-1 flex-col">
                    <div className="app-workspace-scroll min-h-0 flex-1 overflow-y-auto overscroll-contain px-4 py-4 md:px-6 md:py-5">
                        <div className={`settings-pane-root ${activeTab === "prompts" ? "h-full w-full" : "mx-auto w-full max-w-none"}`}>{visibleConfigSections.some((item) => item.key === activeTab) ? panes[activeTab] : panes[firstVisibleSection]}</div>
                    </div>
                </section>
            </div>
        </main>
    );
}

function SettingsPane({ children, fill = false }: { children: ReactNode; fill?: boolean }) {
    return <div className={fill ? "settings-pane h-full" : "settings-pane"}>{children}</div>;
}

function normalizeImageCount(value: string) {
    return String(Math.max(1, Math.min(15, Math.floor(Math.abs(Number(value)) || Number(defaultConfig.canvasImageCount)))));
}
