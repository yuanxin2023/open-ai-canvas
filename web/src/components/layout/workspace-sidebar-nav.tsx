import { Popover } from "antd";
import { translate } from "@/i18n";
import { BookOpen, ChevronRight, CircleHelp, Headphones, Image as ImageIcon, PanelLeftClose, PanelLeftOpen, ShoppingBag, Video } from "lucide-react";
import { LayoutGroup, motion, useReducedMotion } from "motion/react";
import { useEffect, useMemo, useRef, useState, type ComponentType, type CSSProperties, type MouseEvent } from "react";
import { useWorkspaceAuth } from "@/components/auth/workspace-auth-context";
import { isGuestWorkspacePath } from "@/lib/guest-workspace";
import { Link, useLocation, useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";

import { BrandLogoFrame } from "@/components/brand/brand-logo";
import { Kbd } from "@/components/ui/base/kbd";
import { navigationTools, type NavigationToolSlug } from "@/constant/navigation-tools";
import { useWorkspaceLogout } from "@/hooks/use-workspace-logout";
import { aceternityMotion } from "@/lib/aceternity-motion";
import { cn } from "@/lib/utils";
import { preloadWorkspaceRoute } from "@/lib/workspace-route-modules";
import { useUserStore, type FeatureAvailability } from "@/stores/use-user-store";
import { useAppearanceStore } from "@/stores/use-appearance-store";
import { CUSTOMER_SERVICE_OPEN_EVENT, getPublicCustomerService, type PublicCustomerService } from "@/services/api/customer-service";

export type WorkspaceNavItem = {
    id: string;
    title: string;
    icon?: ComponentType<{ className?: string; strokeWidth?: number }>;
    to?: string;
    shortcut?: string;
    badge?: string | number;
    action?: "search" | "logout";
    children?: WorkspaceNavItem[];
};

type WorkspaceNavGroup = {
    heading?: string;
    items: WorkspaceNavItem[];
};

// 收起侧栏时只显示短名称。
const COLLAPSED_TITLE_KEYS: Record<string, string> = {
    home: "nav.collapsed.home",
    projects: "nav.collapsed.projects",
    canvas: "nav.collapsed.canvas",
    assets: "nav.collapsed.assets",
    skills: "nav.collapsed.skills",
    plugins: "nav.collapsed.plugins",
    tasks: "nav.collapsed.tasks",
};

function toolItem(slug: NavigationToolSlug, to: string, t: (key: string) => string): WorkspaceNavItem {
    const tool = navigationTools.find((item) => item.slug === slug);
    return { id: slug, title: tool ? t(tool.labelKey) : slug, icon: tool?.icon, to };
}

export function buildNav(features: FeatureAvailability, t: (key: string) => string = translate): { groups: WorkspaceNavGroup[]; footer: WorkspaceNavItem[] } {
    const groups: WorkspaceNavGroup[] = [
        {
            items: [
                { ...toolItem("create", "/?home=1", t), id: "home", title: t("nav.sidebar.home") },
                ...(features.shortDramaEnabled ? [{ ...toolItem("projects", "/projects", t), title: t("nav.sidebar.projects") }] : []),
                ...(features.ecommerceEnabled ? [{ id: "ecommerce", title: t("nav.sidebar.ecommerce"), icon: ShoppingBag, to: "/ecommerce" }] : []),
                { ...toolItem("canvas", "/canvas", t), title: t("nav.sidebar.canvas") },
            ],
        },
        {
            heading: t("nav.sidebar.headingTools"),
            items: [
                ...(features.imageWorkbenchEnabled ? [{ id: "toolbox-image", title: t("nav.sidebar.toolboxImage"), icon: ImageIcon, to: "/toolbox/image" }] : []),
                { ...toolItem("prompts", "/prompts", t), title: t("local.nav.prompts") },
                ...(features.videoWorkbenchEnabled ? [{ id: "toolbox-video", title: t("nav.sidebar.toolboxVideo"), icon: Video, to: "/toolbox/video" }] : []),
                ...(features.skillLibraryEnabled ? [{ ...toolItem("skills", "/skills", t), title: t("nav.sidebar.skills") }] : []),
                ...(features.pluginCenterEnabled ? [{ ...toolItem("plugins", "/plugins", t), title: t("nav.sidebar.plugins") }] : []),
            ],
        },
        {
            heading: t("nav.sidebar.headingAssets"),
            items: [{ ...toolItem("assets", "/assets", t), title: t("nav.sidebar.assets") }, ...(features.taskCenterEnabled ? [{ id: "tasks", title: t("nav.sidebar.history"), icon: ImageIcon, to: "/tasks" }] : [])],
        },
    ];

    return { groups, footer: [{ id: "help", title: t("local.ui.help"), icon: CircleHelp }] };
}

function WorkspaceHelpMenu({ collapsed, onNavigate }: { collapsed: boolean; onNavigate: () => void }) {
    const [menuOpen, setMenuOpen] = useState(false);
    const [setting, setSetting] = useState<PublicCustomerService | null>(null);
    const [mobile, setMobile] = useState(() => window.matchMedia("(max-width: 767px)").matches);
    const { t } = useTranslation();
    useEffect(() => {
        let controller: AbortController | undefined;
        const load = () => {
            controller?.abort();
            const request = new AbortController();
            controller = request;
            void getPublicCustomerService(request.signal).then(
                (value) => {
                    if (!request.signal.aborted) setSetting(value);
                },
                () => {
                    if (!request.signal.aborted) setSetting(null);
                },
            );
        };
        load();
        window.addEventListener("customer-service-config-updated", load);
        return () => {
            controller?.abort();
            window.removeEventListener("customer-service-config-updated", load);
        };
    }, []);
    useEffect(() => {
        const media = window.matchMedia("(max-width: 767px)");
        const update = () => setMobile(media.matches);
        media.addEventListener("change", update);
        return () => media.removeEventListener("change", update);
    }, []);
    const tutorialUrl = setting?.tutorialUrl?.trim();
    const contactVisible = Boolean(setting?.enabled && (mobile ? setting.mobileEnabled : setting.desktopEnabled));
    const close = () => {
        setMenuOpen(false);
        onNavigate();
    };
    return (
        <Popover
            open={menuOpen}
            onOpenChange={setMenuOpen}
            trigger="click"
            placement={collapsed ? "rightBottom" : "topLeft"}
            rootClassName="workspace-help-popover"
            content={
                <div className="flex w-44 flex-col gap-1 text-[var(--fs-ui)]">
                    {tutorialUrl ? (
                        <a className="flex items-center gap-2 rounded-[var(--r-sm)] p-2 hover:bg-surface-hover focus-visible:outline" href={tutorialUrl} target="_blank" rel="noopener noreferrer" onClick={close}>
                            <BookOpen className="size-4" aria-hidden />
                            {t("local.ui.tutorial")}
                        </a>
                    ) : null}
                    {contactVisible ? (
                        <button
                            type="button"
                            className="flex items-center gap-2 rounded-[var(--r-sm)] p-2 text-left hover:bg-surface-hover focus-visible:outline"
                            onClick={() => {
                                close();
                                window.dispatchEvent(new CustomEvent(CUSTOMER_SERVICE_OPEN_EVENT));
                            }}
                        >
                            <Headphones className="size-4" aria-hidden />
                            {t("local.ui.contactSupport")}
                        </button>
                    ) : null}
                    {!tutorialUrl && !contactVisible ? <span className="p-2 text-foreground/55">{t("local.ui.unavailable")}</span> : null}
                </div>
            }
        >
            <button
                type="button"
                data-nav-id="help"
                aria-label={t("local.ui.help")}
                title={collapsed ? t("local.ui.help") : undefined}
                aria-expanded={menuOpen}
                className={cn("app-workspace-nav-link flex w-full items-center rounded-[var(--r-sm)] px-2 py-2 text-left", collapsed && "is-collapsed", menuOpen && "is-active")}
            >
                <span className="app-workspace-nav-main flex items-center gap-2">
                    <CircleHelp className="size-4" aria-hidden />
                    <span className="app-workspace-nav-title">{t("local.ui.help")}</span>
                </span>
            </button>
        </Popover>
    );
}

function WorkspaceSwitcher({ collapsed, onNavigate, onExpand, onCollapse }: { collapsed: boolean; onNavigate: () => void; onExpand: () => void; onCollapse: () => void }) {
    const appearance = useAppearanceStore((state) => state.appearance);
    const { t } = useTranslation();
    if (collapsed)
        return (
            <div className="app-workspace-sidebar-rail-header shrink-0">
                <button type="button" className="app-workspace-sidebar-rail-button" aria-label={t("shell.expandSidebarMenu")} title={t("shell.expandSidebarMenu")} onClick={onExpand}>
                    <PanelLeftOpen className="size-4" strokeWidth={1.7} />
                </button>
            </div>
        );
    // 未上传自定义 Logo 时用内置取景框字形（与欢迎页、登录框、加载页同一个 /logo.svg），放在墨色实底方块里；
    // 自定义 Logo 的配色不可预知，改用中性浅底方块承托。
    const mark = (
        <BrandLogoFrame
            className={cn("app-workspace-brand-mark grid shrink-0 place-items-center", !appearance.logoConfigured && "is-builtin")}
            logoClassName="size-5 object-contain"
            alt=""
            fallback={<span className="app-workspace-brand-glyph" aria-hidden />}
        />
    );
    return (
        <div className={cn("app-workspace-sidebar-brand-row relative shrink-0", collapsed && "is-collapsed")}>
            <Link to="/?home=1" onClick={onNavigate} className="app-workspace-sidebar-brand-button group" aria-label={t("shell.brandHome", { brand: appearance.brandName })}>
                {mark}
                {!collapsed ? <span className="app-workspace-brand-wordmark truncate">{appearance.brandName}</span> : null}
            </Link>
            <button type="button" className="app-workspace-sidebar-collapse-button" aria-label={t("shell.collapseSidebar")} title={t("shell.collapseSidebar")} onClick={onCollapse}>
                <PanelLeftClose className="size-4" strokeWidth={1.7} />
            </button>
        </div>
    );
}

function NavItem({
    item,
    activeId,
    onSelect,
    onOpenSearch,
    onLogout,
    level = 0,
    collapsed = false,
}: {
    item: WorkspaceNavItem;
    activeId: string;
    onSelect: (id: string) => void;
    onOpenSearch: () => void;
    onLogout: () => void;
    level?: number;
    collapsed?: boolean;
}) {
    const { t } = useTranslation();
    const isActive = activeId === item.id || (item.id === "settings" && activeId.startsWith("settings:"));
    const user = useUserStore((state) => state.user);
    const { openAuth } = useWorkspaceAuth();
    const hasChildren = Boolean(item.children?.length);
    const [isOpen, setIsOpen] = useState(false);
    const reducedMotion = useReducedMotion();

    // 激活分支自动展开（如设置分区子项），保证当前位置可见。
    useEffect(() => {
        if (isActive && hasChildren) setIsOpen(true);
    }, [isActive, hasChildren]);

    const Icon = item.icon;
    const rowStyle = collapsed ? undefined : ({ paddingLeft: `${level * 12 + 10}px` } as CSSProperties);

    const collapsedTitle = COLLAPSED_TITLE_KEYS[item.id] ? t(COLLAPSED_TITLE_KEYS[item.id]) : item.title.slice(0, 2);
    const rowContent = (
        <>
            <span className="app-workspace-nav-main flex min-w-0 items-center gap-2">
                {Icon ? (
                    <span className="app-workspace-nav-icon" aria-hidden>
                        <Icon className="size-4 shrink-0" strokeWidth={1.8} />
                    </span>
                ) : (
                    <span className="size-1.5 shrink-0 rounded-full bg-current opacity-40" aria-hidden />
                )}
                <span className="app-workspace-nav-title truncate">{collapsed ? collapsedTitle : item.title}</span>
            </span>
            <span className="app-workspace-nav-meta flex shrink-0 items-center gap-2">
                {item.shortcut ? <Kbd className="hidden group-hover:inline-flex">{item.shortcut}</Kbd> : null}
                {item.badge !== undefined ? <span className="flex min-w-5 items-center justify-center rounded-full bg-primary/10 px-2 py-0.5 text-[var(--fs-tiny)] font-medium tabular-nums text-primary">{item.badge}</span> : null}
                {hasChildren ? <ChevronRight className={cn("size-3.5 shrink-0 text-foreground/40 transition-transform duration-200", isOpen && "rotate-90")} strokeWidth={2} /> : null}
            </span>
        </>
    );

    const rowClassName = cn(
        "app-workspace-nav-link group relative isolate flex min-h-11 w-full items-center justify-between gap-2 rounded-[var(--r-md)] px-3 py-2 text-[var(--fs-body)] transition-[color,transform] duration-200 select-none",
        collapsed && "is-collapsed",
        isActive ? "is-active font-medium" : "text-foreground/62 hover:bg-surface-hover hover:text-foreground",
    );
    const activePill = isActive ? <motion.span layoutId="workspace-nav-active-pill" className="app-workspace-nav-active-pill" aria-hidden transition={reducedMotion ? { duration: 0 } : aceternityMotion.spring.dock} /> : null;

    const handleClick = (event: MouseEvent) => {
        if (!user && item.to && !isGuestWorkspacePath(item.to.split("?")[0])) {
            event.preventDefault();
            void openAuth({ next: item.to });
            return;
        }
        if (item.action === "search") {
            onOpenSearch();
            return;
        }
        if (item.action === "logout") {
            onLogout();
            return;
        }
        if (hasChildren) {
            setIsOpen((open) => !open);
            return;
        }
        onSelect(item.id);
    };

    // 局部 const：闭包内 TS 保留窄化，供 preload 回调使用。
    const linkTo = item.to;

    return (
        <div className="flex w-full flex-col">
            {linkTo ? (
                <Link
                    to={linkTo}
                    className={rowClassName}
                    data-nav-id={item.id}
                    style={rowStyle}
                    aria-label={collapsed ? item.title : undefined}
                    title={collapsed ? item.title : undefined}
                    onClick={handleClick}
                    onFocus={() => preloadWorkspaceRoute(linkTo)}
                    onPointerDown={() => preloadWorkspaceRoute(linkTo)}
                    onPointerEnter={() => preloadWorkspaceRoute(linkTo)}
                >
                    {activePill}
                    {rowContent}
                </Link>
            ) : (
                <button
                    type="button"
                    className={rowClassName}
                    data-nav-id={item.id}
                    style={rowStyle}
                    aria-label={collapsed ? item.title : undefined}
                    title={collapsed ? item.title : undefined}
                    onClick={handleClick}
                    aria-expanded={hasChildren ? isOpen : undefined}
                >
                    {activePill}
                    {rowContent}
                </button>
            )}

            {hasChildren && !collapsed ? (
                <div className={cn("grid transition-[grid-template-rows,opacity] duration-200 ease-in-out", isOpen ? "grid-rows-[1fr] opacity-100" : "grid-rows-[0fr] opacity-0")}>
                    <div className="relative flex min-h-0 flex-col gap-0.5 overflow-hidden pt-0.5">
                        {item.children!.map((child) => (
                            <NavItem key={child.id} item={child} activeId={activeId} onSelect={onSelect} onOpenSearch={onOpenSearch} onLogout={onLogout} level={level + 1} collapsed={false} />
                        ))}
                    </div>
                </div>
            ) : null}
        </div>
    );
}

function NavGroup({ group, activeId, onNavigate, onOpenSearch, onLogout, collapsed }: { group: WorkspaceNavGroup; activeId: string; onNavigate: () => void; onOpenSearch: () => void; onLogout: () => void; collapsed: boolean }) {
    const [isOpen, setIsOpen] = useState(true);
    const hasActive = group.items.some((item) => item.id === activeId || (item.id === "settings" && activeId.startsWith("settings:")));

    // 激活项所在分组自动展开，保证当前位置可见。
    useEffect(() => {
        if (hasActive) setIsOpen(true);
    }, [hasActive]);

    const content = (
        <div className="flex flex-col gap-2">
            {group.items.map((item) => (
                <NavItem key={item.id} item={item} activeId={activeId} onSelect={onNavigate} onOpenSearch={onOpenSearch} onLogout={onLogout} collapsed={collapsed} />
            ))}
        </div>
    );

    // 无标题分组（核心导航入口）常驻展示，不做折叠。
    if (!group.heading || collapsed) {
        return (
            <div className="app-workspace-nav-group flex shrink-0 flex-col" data-nav-group-heading={group.heading || ""}>
                {content}
            </div>
        );
    }

    return (
        <div className="app-workspace-nav-group flex shrink-0 flex-col" data-nav-group-heading={group.heading}>
            <button type="button" onClick={() => setIsOpen((open) => !open)} aria-expanded={isOpen} className="app-workspace-nav-group-toggle select-none">
                <span className="app-workspace-nav-group-label">{group.heading}</span>
                <ChevronRight className={cn("size-3.5 shrink-0 text-foreground/35 transition-transform duration-200", isOpen && "rotate-90")} strokeWidth={2} />
            </button>
            <div className={cn("grid transition-[grid-template-rows,opacity] duration-200 ease-in-out", isOpen ? "grid-rows-[1fr] opacity-100" : "grid-rows-[0fr] opacity-0")}>
                <div className="min-h-0 overflow-hidden pt-0.5">{content}</div>
            </div>
        </div>
    );
}

// 菜单项 id 与实际路由不一定同名：/toolbox/image 和 /toolbox/video 共用 toolbox 前缀，
// 只取首段会让两者都匹配不到任何菜单项，侧栏因此没有高亮。
export function workspaceNavActiveId(pathname: string, section?: string | null) {
    const segments = pathname.split("/").filter(Boolean);
    if (segments[0] === "toolbox" && segments[1]) return `toolbox-${segments[1]}`;
    const slug = segments[0] || "home";
    if (slug === "create") return "home";
    return slug === "settings" && section ? `settings:${section}` : slug;
}

export function WorkspaceSidebarNav({ collapsed, onNavigate, onOpenSearch, onExpand, onCollapse }: { collapsed: boolean; onNavigate: () => void; onOpenSearch: () => void; onExpand: () => void; onCollapse: () => void }) {
    const { pathname } = useLocation();
    const [searchParams] = useSearchParams();
    const features = useUserStore((state) => state.features);
    const { handleLogout } = useWorkspaceLogout();
    const { t, i18n } = useTranslation();
    const { groups, footer } = useMemo(() => buildNav(features, t), [features, t, i18n.language]);

    const activeId = workspaceNavActiveId(pathname, searchParams.get("section"));

    const scrollRef = useRef<HTMLDivElement>(null);
    const [scrollState, setScrollState] = useState({ hasTopFade: false, hasBottomFade: false });
    const handleScroll = () => {
        const element = scrollRef.current;
        if (!element) return;
        const { scrollTop, scrollHeight, clientHeight } = element;
        setScrollState({
            hasTopFade: scrollTop > 0,
            hasBottomFade: scrollTop + clientHeight < scrollHeight - 1,
        });
    };
    useEffect(() => {
        handleScroll();
    }, [groups]);

    return (
        <div className={cn("app-workspace-sidebar-nav flex h-full shrink-0 flex-col", collapsed && "is-collapsed")}>
            <WorkspaceSwitcher collapsed={collapsed} onNavigate={onNavigate} onExpand={onExpand} onCollapse={onCollapse} />

            <LayoutGroup id="workspace-sidebar-nav">
                <div
                    ref={scrollRef}
                    onScroll={handleScroll}
                    className={cn(
                        "app-workspace-sidebar-scroll-area flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto px-3 pb-3 pt-7",
                        collapsed && "is-collapsed",
                        scrollState.hasTopFade && "has-top-fade",
                        scrollState.hasBottomFade && "has-bottom-fade",
                    )}
                >
                    {groups.map((group, index) => (
                        <NavGroup key={index} group={group} activeId={activeId} onNavigate={onNavigate} onOpenSearch={onOpenSearch} onLogout={() => void handleLogout()} collapsed={collapsed} />
                    ))}
                </div>
            </LayoutGroup>

            <div className="app-workspace-sidebar-footer shrink-0 px-3 py-3">
                {footer.length ? (
                    <div className="flex flex-col gap-0.5">
                        {footer.map((item) =>
                            item.id === "help" ? (
                                <WorkspaceHelpMenu key={item.id} collapsed={collapsed} onNavigate={onNavigate} />
                            ) : (
                                <NavItem key={item.id} item={item} activeId={activeId} onSelect={onNavigate} onOpenSearch={onOpenSearch} onLogout={() => void handleLogout()} collapsed={collapsed} />
                            ),
                        )}
                    </div>
                ) : null}
            </div>
        </div>
    );
}
