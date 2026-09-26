import { PanelLeftClose, PanelLeftOpen } from "lucide-react";
import { useLocation } from "react-router";

import { SystemAnnouncementCenter } from "@/components/layout/system-announcement-center";
import { WorkspaceAccountMenu } from "@/components/layout/workspace-account-menu";
import { WorkspaceCreditPopover } from "@/components/layout/workspace-credit-popover";
import { WorkspaceTopBarCheckin } from "@/components/layout/workspace-top-bar-checkin";
import { WorkspaceTopBarExtensionSlot } from "@/components/layout/workspace-top-bar-extension";
import { AnimatedThemeToggler } from "@/components/ui/animated-theme-toggler";
import { useThemeStore } from "@/stores/use-theme-store";
import { useUserStore } from "@/stores/use-user-store";

const PAGE_TITLES: Record<string, string> = {
    home: "创作",
    create: "创作",
    projects: "短剧 Agent",
    canvas: "自由画布",
    tasks: "创作历史",
    assets: "资产",
    prompts: "提示词",
    skills: "技能",
    plugins: "插件",
    settings: "设置",
};

export function WorkspaceTopBar({ sidebarOpen, onToggleSidebar }: { sidebarOpen: boolean; onToggleSidebar: () => void }) {
    const theme = useThemeStore((state) => state.theme);
    const setTheme = useThemeStore((state) => state.setTheme);
    const user = useUserStore((state) => state.user);
    const creditsEnabled = useUserStore((state) => state.features.creditsEnabled);
    const { pathname } = useLocation();
    const slug = pathname.split("/").filter(Boolean)[0];
    const pageTitle = PAGE_TITLES[slug || "home"] || "工作台";

    return (
        <header className="app-workspace-topbar">
            <button type="button" className="app-workspace-mobile-menu app-workspace-topbar-icon-button" aria-label={sidebarOpen ? "收起侧栏" : "展开侧栏"} onClick={onToggleSidebar}>
                {sidebarOpen ? <PanelLeftClose className="size-4" /> : <PanelLeftOpen className="size-4" />}
            </button>
            <div className="app-workspace-topbar-breadcrumb" aria-label="当前位置">
                <span className="truncate font-medium text-foreground">{pageTitle}</span>
            </div>
            <WorkspaceTopBarExtensionSlot />
            <div className="app-workspace-topbar-actions">
                {user && creditsEnabled ? <WorkspaceCreditPopover userId={user.id} /> : null}
                {user ? <SystemAnnouncementCenter userId={user.id} className="app-workspace-topbar-icon-button" autoOpen /> : null}
                <AnimatedThemeToggler className="app-workspace-topbar-icon-button" theme={theme} onThemeChange={setTheme} aria-label="切换主题" />
                <WorkspaceAccountMenu />
                {pathname === "/" ? <WorkspaceTopBarCheckin /> : null}
            </div>
        </header>
    );
}
