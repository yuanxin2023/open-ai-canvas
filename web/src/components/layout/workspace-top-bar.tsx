import { translate } from "@/i18n";
import { useTranslation } from "react-i18next";
import { PanelLeftClose, PanelLeftOpen } from "lucide-react";
import { useLocation } from "react-router";
import type { Ref } from "react";

import { TopbarFeedbackButton } from "@/components/feedback/topbar-feedback-button";
import { SystemAnnouncementCenter } from "@/components/layout/system-announcement-center";
import { WorkspaceAccountMenu } from "@/components/layout/workspace-account-menu";
import { WorkspaceCreditPopover } from "@/components/layout/workspace-credit-popover";
import { WorkspaceTopBarCheckin } from "@/components/layout/workspace-top-bar-checkin";
import { WorkspaceTopBarExtensionSlot } from "@/components/layout/workspace-top-bar-extension";
import { AnimatedThemeToggler } from "@/components/ui/animated-theme-toggler";
import { LanguageMenu } from "@/components/layout/language-menu";
import { useThemeStore } from "@/stores/use-theme-store";
import { useUserStore } from "@/stores/use-user-store";

const PAGE_TITLES: Record<string, string> = {
    home: "nav.sidebar.home",
    create: "nav.sidebar.home",
    projects: "nav.sidebar.projects",
    ecommerce: "nav.sidebar.ecommerce",
    canvas: "nav.sidebar.canvas",
    tasks: "nav.sidebar.history",
    assets: "nav.sidebar.assets",
    prompts: "local.nav.prompts",
    skills: "nav.sidebar.skills",
    plugins: "nav.sidebar.plugins",
    settings: "nav.tools.settings",
};

export function WorkspaceTopBar({ sidebarOpen, onToggleSidebar, sidebarTriggerRef }: { sidebarOpen: boolean; onToggleSidebar: () => void; sidebarTriggerRef?: Ref<HTMLButtonElement> }) {
    const { t } = useTranslation();
    const theme = useThemeStore((state) => state.theme);
    const setTheme = useThemeStore((state) => state.setTheme);
    const user = useUserStore((state) => state.user);
    const creditsEnabled = useUserStore((state) => state.features.creditsEnabled);
    const { pathname } = useLocation();
    const [slug, subSlug] = pathname.split("/").filter(Boolean);
    const pageTitle = translate(slug === "toolbox" ? (subSlug === "video" ? "nav.sidebar.toolboxVideo" : "nav.sidebar.toolboxImage") : PAGE_TITLES[slug || "home"] || "shell.workbench");

    return (
        <header className="app-workspace-topbar">
            <button
                ref={sidebarTriggerRef}
                type="button"
                className="app-workspace-mobile-menu app-workspace-topbar-icon-button"
                aria-label={sidebarOpen ? translate("shell.collapseSidebar") : translate("shell.expandSidebar")}
                aria-expanded={sidebarOpen}
                onClick={onToggleSidebar}
            >
                {sidebarOpen ? <PanelLeftClose className="size-4" /> : <PanelLeftOpen className="size-4" />}
            </button>
            <div className="app-workspace-topbar-breadcrumb" aria-label={translate("shell.breadcrumb")}>
                <span className="truncate font-medium text-foreground">{pageTitle}</span>
            </div>
            <WorkspaceTopBarExtensionSlot />
            <div className="app-workspace-topbar-actions">
                <LanguageMenu />
                {user ? <TopbarFeedbackButton /> : null}
                {user && creditsEnabled ? <WorkspaceCreditPopover userId={user.id} /> : null}
                {user ? <SystemAnnouncementCenter userId={user.id} className="app-workspace-topbar-icon-button" autoOpen /> : null}
                <AnimatedThemeToggler className="app-workspace-topbar-icon-button" theme={theme} onThemeChange={setTheme} aria-label={t("shell.toggleTheme")} />
                <WorkspaceAccountMenu />
                {pathname === "/" ? <WorkspaceTopBarCheckin /> : null}
            </div>
        </header>
    );
}
