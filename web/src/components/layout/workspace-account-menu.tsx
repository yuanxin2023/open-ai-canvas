import { translate } from "@/i18n";
import { useTranslation } from "react-i18next";
import { Popover } from "antd";
import { Switch } from "@/components/ui/base/switch";
import { CircleUserRound, Moon, Sun } from "lucide-react";
import { useState } from "react";
import { useWorkspaceAuth } from "@/components/auth/workspace-auth-context";

import { WorkspaceAccountCard } from "./workspace-account-card";
import { UserAvatar } from "./user-avatar";
import { UserGroupAvatarRing } from "./user-group-badge";
import { openWorkspaceCreditProducts } from "@/lib/workspace-wallet";
import { useThemeStore } from "@/stores/use-theme-store";
import { useUserStore } from "@/stores/use-user-store";

/** 顶部与侧栏复用同一账户卡片；顶部额外保留主题偏好。 */
export function WorkspaceAccountMenu() {
    const { openAuth } = useWorkspaceAuth();
    const theme = useThemeStore((state) => state.theme);
    const setTheme = useThemeStore((state) => state.setTheme);
    const user = useUserStore((state) => state.user);
    const hydrated = useUserStore((state) => state.hydrated);
    const [menuOpen, setMenuOpen] = useState(false);
    const { t } = useTranslation();
    if (!hydrated) {
        return <span className="size-9 animate-pulse rounded-[var(--r-md)] bg-foreground/[.06]" aria-hidden />;
    }

    return user ? (
        <>
            <Popover
                trigger="click"
                placement="bottomRight"
                arrow={false}
                rootClassName="workspace-account-popover"
                open={menuOpen}
                onOpenChange={setMenuOpen}
                content={
                    <div className="workspace-topbar-account-menu">
                        <WorkspaceAccountCard
                            onNavigate={() => setMenuOpen(false)}
                            onBuyCredits={() => {
                                setMenuOpen(false);
                                openWorkspaceCreditProducts();
                            }}
                        />

                        <div className="workspace-topbar-account-section">
                            <div className="workspace-topbar-account-theme">
                                {theme === "dark" ? <Moon className="size-3.5 text-foreground/45" /> : <Sun className="size-3.5 text-foreground/45" />}
                                <span className="ml-2 flex-1 text-xs text-foreground/65">{translate("shell.darkMode")}</span>
                                <Switch size="sm" checked={theme === "dark"} onChange={(checked) => setTheme(checked ? "dark" : "light")} aria-label={translate("shell.darkMode")} />
                            </div>
                        </div>
                    </div>
                }
            >
                <button type="button" className="app-workspace-topbar-icon-button app-workspace-account-trigger" aria-label={t("shell.accountMenu")} title={user.username}>
                    <UserGroupAvatarRing>
                        <UserAvatar user={user} className="size-6" />
                    </UserGroupAvatarRing>
                </button>
            </Popover>
        </>
    ) : (
        <button type="button" onClick={() => void openAuth()} className="inline-flex items-center gap-2 px-2 text-xs text-foreground/55" aria-label={translate("shell.login")} title={translate("shell.login")}>
            <CircleUserRound className="size-5" />
            <span className="hidden sm:inline">{translate("shell.guestShort")}</span>
        </button>
    );
}
