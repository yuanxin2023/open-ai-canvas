import { translate } from "@/i18n";
import { lazy, Suspense, useEffect, useRef, useState, useSyncExternalStore, type ReactNode } from "react";
import { AppDrawer } from "@/components/ui/product/app-drawer";
import { useLocation, useNavigate } from "react-router";
import { useTranslation } from "react-i18next";

import { BannerAnnouncementsSlider } from "@/components/layout/banner-announcements-slider";
import { ModelSetupGuide } from "@/components/layout/model-setup-guide";
import { WorkspaceSidebarNav } from "@/components/layout/workspace-sidebar-nav";
import { readWorkspaceSidebarCollapsed, subscribeWorkspaceSidebarCollapsed, writeWorkspaceSidebarCollapsed } from "@/components/layout/workspace-sidebar-state";
import { WorkspaceTopBar } from "@/components/layout/workspace-top-bar";
import { WorkspaceTopBarExtensionProvider } from "@/components/layout/workspace-top-bar-extension";
import { WorkspaceStorageLimitNotice } from "@/components/layout/workspace-storage-limit-notice";
import { WorkspaceWalletHost } from "@/components/layout/workspace-wallet-modal";
import { GenerationSuccessNotifier } from "@/components/layout/generation-success-notifier";
import { useWorkspaceDocumentTitle } from "@/hooks/use-workspace-document-title";
import { ImageCreditShortfallHost } from "@/components/layout/image-credit-shortfall-host";
import { cn } from "@/lib/utils";
import { isSpatialWorkbenchPath } from "@/lib/workspace-routes";

const WorkspaceCommandPalette = lazy(() => import("@/components/layout/workspace-command-palette").then((module) => ({ default: module.WorkspaceCommandPalette })));
const sidebarMobileQuery = "(max-width: 1023px)";
function subscribeSidebarViewport(notify: () => void) {
    const query = window.matchMedia(sidebarMobileQuery);
    query.addEventListener("change", notify);
    return () => query.removeEventListener("change", notify);
}
const sidebarMobileSnapshot = () => window.matchMedia(sidebarMobileQuery).matches;
const sidebarServerSnapshot = () => false;

export function AppWorkspaceShell({ children }: { children: ReactNode }) {
    useWorkspaceDocumentTitle();
    const { t } = useTranslation();
    const { pathname } = useLocation();
    const navigate = useNavigate();
    const [mobileSidebarExpanded, setMobileSidebarExpanded] = useState(false);
    const [desktopSidebarCollapsed, setDesktopSidebarCollapsed] = useState(readWorkspaceSidebarCollapsed);
    const [paletteOpen, setPaletteOpen] = useState(false);
    const mobile = useSyncExternalStore(subscribeSidebarViewport, sidebarMobileSnapshot, sidebarServerSnapshot);
    const sidebarTriggerRef = useRef<HTMLButtonElement>(null);
    const closeMobileSidebar = () => {
        setMobileSidebarExpanded(false);
        requestAnimationFrame(() => sidebarTriggerRef.current?.focus({ preventScroll: true }));
    };

    const hideChrome = pathname.startsWith("/admin") || /^\/canvas\/[^/]+/.test(pathname);
    const showGlobalTopBar = !hideChrome;
    const spatialWorkbench = isSpatialWorkbenchPath(pathname);
    const creationWorkspace = pathname === "/" || pathname === "/create";

    const isMobileViewport = sidebarMobileSnapshot;

    const toggleSidebar = () => {
        if (isMobileViewport()) {
            setMobileSidebarExpanded((current) => !current);
            return;
        }
        setDesktopSidebarCollapsed((current) => {
            const next = !current;
            writeWorkspaceSidebarCollapsed(next);
            return next;
        });
    };

    const expandDesktopSidebar = () => {
        setDesktopSidebarCollapsed(false);
        writeWorkspaceSidebarCollapsed(false);
    };

    const collapseDesktopSidebar = () => {
        setDesktopSidebarCollapsed(true);
        writeWorkspaceSidebarCollapsed(true);
    };

    const handleNavClick = () => {
        if (isMobileViewport()) setMobileSidebarExpanded(false);
    };

    // ⌘K / Ctrl+K 全局呼出搜索面板。
    useEffect(() => {
        const handler = (event: KeyboardEvent) => {
            if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
                event.preventDefault();
                setPaletteOpen((open) => !open);
            }
        };
        window.addEventListener("keydown", handler);
        return () => window.removeEventListener("keydown", handler);
    }, []);

    useEffect(() => {
        const handleWorkspaceNavigation = (rawEvent: Event) => {
            const event = rawEvent as CustomEvent<{ to?: string }>;
            if (!event.detail?.to) return;
            event.preventDefault();
            navigate(event.detail.to);
        };
        window.addEventListener("workspace:navigate", handleWorkspaceNavigation);
        return () => window.removeEventListener("workspace:navigate", handleWorkspaceNavigation);
    }, [navigate]);

    useEffect(() => subscribeWorkspaceSidebarCollapsed(setDesktopSidebarCollapsed), []);
    useEffect(() => {
        setMobileSidebarExpanded(false);
    }, [mobile, pathname]);

    return (
        <>
            <WorkspaceTopBarExtensionProvider>
                <div className={cn("app-workspace-shell flex h-dvh min-h-0 w-full flex-col overflow-hidden", spatialWorkbench && "is-spatial", creationWorkspace && "is-creation-workspace")}>
                    {showGlobalTopBar ? <BannerAnnouncementsSlider /> : null}

                    <div className="app-workspace-main-row flex min-h-0 min-w-0 flex-1 overflow-hidden">
                        {!hideChrome && !mobile ? (
                            <aside className={cn("app-workspace-sidebar flex h-full shrink-0 flex-col overflow-hidden", desktopSidebarCollapsed && "is-collapsed")}>
                                <WorkspaceSidebarNav collapsed={desktopSidebarCollapsed} onNavigate={handleNavClick} onOpenSearch={() => setPaletteOpen(true)} onExpand={expandDesktopSidebar} onCollapse={collapseDesktopSidebar} />
                            </aside>
                        ) : null}

                        <div className="app-workspace-stage relative flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
                            {showGlobalTopBar ? <WorkspaceTopBar sidebarTriggerRef={sidebarTriggerRef} sidebarOpen={mobile ? mobileSidebarExpanded : !desktopSidebarCollapsed} onToggleSidebar={toggleSidebar} /> : null}
                            <div className="relative min-h-0 min-w-0 flex-1 overflow-hidden">{children}</div>
                        </div>
                    </div>

                    {paletteOpen ? (
                        <Suspense fallback={null}>
                            <WorkspaceCommandPalette open onClose={() => setPaletteOpen(false)} />
                        </Suspense>
                    ) : null}
                </div>
                {!hideChrome && mobile ? (
                    <AppDrawer
                        flush
                        title={null}
                        closable={false}
                        placement="left"
                        size="min(280px, calc(100vw - 32px))"
                        open={mobileSidebarExpanded}
                        onClose={closeMobileSidebar}
                        focusable={{ focusTriggerAfterClose: false }}
                        rootClassName="workspace-navigation-drawer app-user-overlays"
                        aria-label={translate("local.ui.workspaceNav")}
                    >
                        <div className="app-user-workspace app-product-workspace workspace-navigation-drawer-content">
                            <WorkspaceSidebarNav
                                collapsed={false}
                                onNavigate={closeMobileSidebar}
                                onOpenSearch={() => {
                                    setMobileSidebarExpanded(false);
                                    setPaletteOpen(true);
                                }}
                                onExpand={() => setMobileSidebarExpanded(true)}
                                onCollapse={closeMobileSidebar}
                            />
                        </div>
                    </AppDrawer>
                ) : null}
            </WorkspaceTopBarExtensionProvider>
            <WorkspaceWalletHost />
            <ImageCreditShortfallHost />
            <WorkspaceStorageLimitNotice />
            <GenerationSuccessNotifier />
            <ModelSetupGuide hidden={pathname === "/login" || pathname === "/register" || pathname.startsWith("/admin")} />
        </>
    );
}
