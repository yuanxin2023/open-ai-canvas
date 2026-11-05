import { translate } from "@/i18n";
import { lazy, Suspense, type ReactNode } from "react";
import { createBrowserRouter, Navigate, Outlet, useLocation } from "react-router";
import { lazyAdminPage } from "@/pages/admin/admin-page-loader";

import { RequireAuth } from "@/components/auth/require-auth";
import { RequireFeature } from "@/components/auth/require-feature";
import { WorkspaceAuthProvider } from "@/components/auth/workspace-auth-provider";
import { AdminLanding, RequireAdminPermission, RequireFullAdmin } from "@/components/auth/require-admin-permission";
import type { AdminPermission } from "@/lib/admin-permissions";
import { CustomerServiceWidget } from "@/components/customer-service/customer-service-widget";
import { FullScreenLoader, WorkspaceRouteLoader } from "@/components/ui/aceternity/full-screen-loader";
import { workspaceRoutePages } from "@/lib/workspace-route-modules";
import { CanvasRefreshShell } from "@/pages/canvas/canvas-refresh-shell";
import { AuthScene } from "@/pages/auth/auth-scene";
import RouteErrorPage from "@/pages/route-error";
import { isGuestWorkspacePath } from "@/lib/guest-workspace";

const AdminPage = lazy(() => import("@/pages/admin"));
const EcommerceManagementPage = lazyAdminPage(() => import("@/pages/admin/ecommerce-management-page"));
const EcommerceAgentConfigPage = lazyAdminPage(() => import("@/pages/admin/ecommerce-agent-config-page"));
const UserGroupsPage = lazyAdminPage(() => import("@/pages/admin/user-groups/user-groups-page"));
const AnalyticsPage = lazyAdminPage(() => import("@/pages/admin/admin-route-pages").then((module) => ({ default: module.AnalyticsPage })));
const ControlPreviewPage = lazyAdminPage(() => import("@/pages/admin/control-preview/control-preview-page"));
const AnnouncementsPage = lazyAdminPage(() => import("@/pages/admin/admin-route-pages").then((module) => ({ default: module.AnnouncementsPage })));
const BannerAnnouncementsPage = lazyAdminPage(() => import("@/pages/admin/admin-route-pages").then((module) => ({ default: module.BannerAnnouncementsPage })));
const StorageResourcesPage = lazyAdminPage(() => import("@/pages/admin/admin-route-pages").then((module) => ({ default: module.StorageResourcesPage })));
const CreditOperationsPage = lazyAdminPage(() => import("@/pages/admin/admin-route-pages").then((module) => ({ default: module.CreditOperationsPage })));
const AccessSettingsPage = lazyAdminPage(() => import("@/pages/admin/admin-route-pages").then((module) => ({ default: module.AccessSettingsPage })));
const EmailSettingsPage = lazyAdminPage(() => import("@/pages/admin/admin-route-pages").then((module) => ({ default: module.EmailSettingsPage })));
const FeatureAvailabilityPage = lazyAdminPage(() => import("@/pages/admin/admin-route-pages").then((module) => ({ default: module.FeatureAvailabilityPage })));
const AgentLessonsPage = lazyAdminPage(() => import("@/pages/admin/admin-route-pages").then((module) => ({ default: module.AgentLessonsPage })));
const InspirationsPage = lazyAdminPage(() => import("@/pages/admin/admin-route-pages").then((module) => ({ default: module.InspirationsPage })));
const ChannelsPage = lazyAdminPage(() => import("@/pages/admin/channels/channels-page"));
const LogicalModelsPage = lazyAdminPage(() => import("@/pages/admin/logical-models/logical-models-page"));
const AdminPluginsPage = lazyAdminPage(() => import("@/pages/admin/plugins/plugins-page"));
const AdminSkillsPage = lazyAdminPage(() => import("@/pages/admin/skills/skills-page"));
const SkillCurationPage = lazyAdminPage(() => import("@/pages/admin/skill-curation-page"));
const AdminPaymentsPage = lazy(() => import("@/pages/admin/payments/payments-page"));
const ProductOperationsPage = lazyAdminPage(() => import("@/pages/admin/product-operations/product-operations-page"));
const CustomerServicePage = lazyAdminPage(() => import("@/pages/admin/customer-service/customer-service-page"));
const LogsPage = lazyAdminPage(() => import("@/pages/admin/logs/logs-page"));
const LoginLogsPage = lazyAdminPage(() => import("@/pages/admin/login-logs/login-logs-page"));
const RedemptionCodesPage = lazyAdminPage(() => import("@/pages/admin/redemption-codes/redemption-codes-page"));
const ReferralsAdminPage = lazyAdminPage(() => import("@/pages/admin/referrals/referrals-page"));
const RuntimePolicySettingsPage = lazyAdminPage(() => import("@/pages/admin/settings/runtime-policy-settings-page"));
const AppearanceSettingsPage = lazyAdminPage(() => import("@/pages/admin/settings/appearance-settings-page"));
const DrawingEngineSettingsPage = lazyAdminPage(() => import("@/pages/admin/settings/drawing-engine-settings-page"));
const StorageSettingsPage = lazyAdminPage(() => import("@/pages/admin/settings/storage-settings-page"));
const ArkPrivateAssetsSettingsPage = lazyAdminPage(() => import("@/pages/admin/settings/ark-private-assets-settings-page"));
const ResponseInterceptionSettingsPage = lazyAdminPage(() => import("@/pages/admin/settings/response-interception-settings-page"));
const ThirdPartySettingsPage = lazyAdminPage(() => import("@/pages/admin/settings/libtv-settings-page"));
const WorkbenchToolsPage = lazyAdminPage(() => import("@/pages/admin/workbench-tools/workbench-tools-page"));
const SystemUpdatePage = lazyAdminPage(() => import("@/pages/admin/settings/system-update-page"));
const SystemPerformancePage = lazyAdminPage(() => import("@/pages/admin/settings/system-performance-page"));
const StoryboardPromptsPage = lazyAdminPage(() => import("@/pages/admin/storyboard-prompts/storyboard-prompts-page"));
const AdminFeedbacksPage = lazyAdminPage(() => import("@/pages/admin/feedbacks/feedbacks-page"));
const UsersPage = lazyAdminPage(() => import("@/pages/admin/users/users-page"));
const AdministratorsPage = lazyAdminPage(() => import("@/pages/admin/users/administrators-page"));
const AssetsPage = workspaceRoutePages.assets;
const PromptsPage = workspaceRoutePages.prompts;
const LoginPage = lazy(() => import("@/pages/auth/login"));
const RegisterPage = lazy(() => import("@/pages/auth/register"));
const ForgotPasswordPage = lazy(() => import("@/pages/auth/forgot-password"));
const CanvasPage = workspaceRoutePages.canvas;
const CanvasProjectPage = workspaceRoutePages.canvasProject;
const SharedCanvasPage = lazy(() => import("@/pages/canvas/shared"));
const CreatePage = workspaceRoutePages.create;
const EcommercePage = workspaceRoutePages.ecommerce;
const ImageWorkbenchPage = workspaceRoutePages.imageWorkbench;
const VideoWorkbenchPage = workspaceRoutePages.videoWorkbench;
const NotFound = lazy(() => import("@/pages/not-found"));
const SkillsPage = workspaceRoutePages.skills;
const SkillWorkbenchPage = workspaceRoutePages.skillWorkbench;
const PluginsPage = workspaceRoutePages.plugins;
const EagleLibraryPage = workspaceRoutePages.eagleLibrary;
const TasksPage = workspaceRoutePages.tasks;
const ProjectsPage = workspaceRoutePages.projects;
const ProjectDetailPage = workspaceRoutePages.projectDetail;
const ProjectCreatePage = workspaceRoutePages.projectCreate;
const SettingsPage = workspaceRoutePages.settings;
const ReferralsPage = lazy(() => import("@/pages/referrals"));
const ProfilePage = lazy(() => import("@/pages/profile"));
const TestVoiceRecording = lazy(() => import("@/pages/test-voice-recording"));
const UserLayout = lazy(() => import("@/layouts/user-layout"));

function deferred(element: ReactNode) {
    return <Suspense fallback={<WorkspaceRouteLoader />}>{element}</Suspense>;
}

function fullScreenDeferred(element: ReactNode) {
    return <Suspense fallback={<FullScreenLoader label={translate("local.ui.openingWorkspace")} detail={translate("local.ui.preparePage")} />}>{element}</Suspense>;
}

function adminRoute(permission: AdminPermission, element: ReactNode) {
    return <RequireAdminPermission permission={permission}>{element}</RequireAdminPermission>;
}

function AuthenticatedWorkspaceLayout() {
    const { pathname } = useLocation();
    const isCanvasProjectRoute = pathname.startsWith("/canvas/");
    const fallback = isCanvasProjectRoute ? <CanvasRefreshShell /> : <FullScreenLoader label={translate("local.ui.openingWorkspace")} detail={translate("local.ui.preparePage")} />;
    return (
        <WorkspaceAuthProvider>
            <RequireAuth allowAnonymous={isGuestWorkspacePath(pathname)}>
                <Suspense fallback={fallback}>
                    <UserLayout>
                        <Outlet />
                    </UserLayout>
                </Suspense>
                <CustomerServiceWidget />
            </RequireAuth>
        </WorkspaceAuthProvider>
    );
}

/**
 * DEV 专用实验室路由。
 *
 * lazy(() => import(...)) 写在函数体内，而不是模块顶层常量：
 * 生产构建时 import.meta.env.DEV 被替换为 false，本函数随之不可达，
 * 摇树会连同其中的动态 import 一起删除，实验室代码不进入生产依赖图。
 * 若把 lazy 提到模块顶层，动态 import 会被静态分析成真实 chunk 并打进 dist。
 */
function devRoutes() {
    const FolderPreviewLab = lazy(() => import("@/pages/dev/folder-preview-lab"));
    const PrevisReproLab = lazy(() => import("@/pages/dev/previs-repro-lab"));
    return [
        { path: "/dev/folders", element: fullScreenDeferred(<FolderPreviewLab />), errorElement: <RouteErrorPage /> },
        { path: "/dev/previs-repro", element: fullScreenDeferred(<PrevisReproLab />), errorElement: <RouteErrorPage /> },
    ];
}

export const router = createBrowserRouter([
    {
        element: <AuthScene />,
        errorElement: <RouteErrorPage />,
        children: [
            { path: "/login", element: fullScreenDeferred(<LoginPage />) },
            { path: "/register", element: fullScreenDeferred(<RegisterPage />) },
            { path: "/forgot-password", element: fullScreenDeferred(<ForgotPasswordPage />) },
        ],
    },
    { path: "/share/canvas/:token", element: fullScreenDeferred(<SharedCanvasPage />), errorElement: <RouteErrorPage /> },
    { path: "/welcome", element: <Navigate to="/" replace /> },
    ...(import.meta.env.DEV ? devRoutes() : []),
    {
        element: <AuthenticatedWorkspaceLayout />,
        errorElement: <RouteErrorPage />,
        children: [
            { path: "/", element: deferred(<CreatePage />) },
            { path: "/create", element: deferred(<CreatePage />) },
            { path: "/ecommerce", element: <RequireAuth>{deferred(<EcommercePage />)}</RequireAuth> },
            {
                path: "/toolbox/image",
                element: (
                    <RequireAuth>
                        <RequireFeature feature="imageWorkbenchEnabled">{deferred(<ImageWorkbenchPage />)}</RequireFeature>
                    </RequireAuth>
                ),
            },
            {
                path: "/toolbox/video",
                element: (
                    <RequireAuth>
                        <RequireFeature feature="videoWorkbenchEnabled">{deferred(<VideoWorkbenchPage />)}</RequireFeature>
                    </RequireAuth>
                ),
            },
            {
                path: "/tasks",
                element: (
                    <RequireAuth>
                        <RequireFeature feature="taskCenterEnabled">{deferred(<TasksPage />)}</RequireFeature>
                    </RequireAuth>
                ),
            },
            { path: "/assets", element: <RequireAuth>{deferred(<AssetsPage />)}</RequireAuth> },
            { path: "/prompts", element: <RequireAuth>{deferred(<PromptsPage />)}</RequireAuth> },
            {
                path: "/skills",
                element: (
                    <RequireAuth>
                        <RequireFeature feature="skillLibraryEnabled">{deferred(<SkillsPage />)}</RequireFeature>
                    </RequireAuth>
                ),
            },
            {
                path: "/skills/:skillId/edit",
                element: (
                    <RequireAuth>
                        <RequireFeature feature="skillLibraryEnabled">{deferred(<SkillWorkbenchPage />)}</RequireFeature>
                    </RequireAuth>
                ),
            },
            {
                path: "/plugins",
                element: (
                    <RequireAuth>
                        <RequireFeature feature="pluginCenterEnabled">{deferred(<PluginsPage />)}</RequireFeature>
                    </RequireAuth>
                ),
            },
            {
                path: "/plugins/eagle",
                element: (
                    <RequireAuth>
                        <RequireFeature feature="pluginCenterEnabled">{deferred(<EagleLibraryPage />)}</RequireFeature>
                    </RequireAuth>
                ),
            },
            {
                path: "/wallet",
                element: <RequireAuth>{null}</RequireAuth>,
            },
            { path: "/settings", element: <RequireAuth>{deferred(<SettingsPage />)}</RequireAuth> },
            {
                path: "/referrals",
                element: (
                    <RequireAuth>
                        <RequireFeature feature="creditsEnabled">{deferred(<ReferralsPage />)}</RequireFeature>
                    </RequireAuth>
                ),
            },
            { path: "/profile", element: <RequireAuth>{deferred(<ProfilePage />)}</RequireAuth> },
            { path: "/test-voice-recording", element: <RequireAuth>{deferred(<TestVoiceRecording />)}</RequireAuth> },
            {
                path: "/projects",
                element: (
                    <RequireAuth>
                        <RequireFeature feature="shortDramaEnabled">{deferred(<ProjectsPage />)}</RequireFeature>
                    </RequireAuth>
                ),
            },
            {
                path: "/projects/new",
                element: (
                    <RequireAuth>
                        <RequireFeature feature="shortDramaEnabled">{deferred(<ProjectCreatePage />)}</RequireFeature>
                    </RequireAuth>
                ),
            },
            {
                path: "/projects/:projectId",
                element: (
                    <RequireAuth>
                        <RequireFeature feature="shortDramaEnabled">{deferred(<ProjectDetailPage />)}</RequireFeature>
                    </RequireAuth>
                ),
            },
            {
                path: "/projects/:projectId/e/:episodeId",
                element: (
                    <RequireAuth>
                        <RequireFeature feature="shortDramaEnabled">{deferred(<ProjectDetailPage />)}</RequireFeature>
                    </RequireAuth>
                ),
            },
            { path: "/projects/:projectId/*", element: <Navigate to=".." relative="path" replace /> },
            { path: "/canvas", element: <RequireAuth>{deferred(<CanvasPage />)}</RequireAuth> },
            {
                path: "/canvas/:id",
                element: (
                    <RequireAuth>
                        <CanvasProjectPage />
                    </RequireAuth>
                ),
            },
            {
                path: "/admin",
                element: <RequireAuth>{deferred(<AdminPage />)}</RequireAuth>,
                children: [
                    { index: true, element: <AdminLanding>{adminRoute("admin.analytics.overview", <AnalyticsPage />)}</AdminLanding> },
                    { path: "control-preview", element: adminRoute("admin.analytics.control_preview", <ControlPreviewPage />) },
                    { path: "users", element: adminRoute("admin.users.accounts", <UsersPage />) },
                    { path: "user-groups", element: adminRoute("admin.users.groups", <UserGroupsPage />) },
                    { path: "ecommerce", element: <Navigate to="/admin/ecommerce/agent" replace /> },
                    { path: "ecommerce/agent", element: adminRoute("admin.ecommerce.config", <EcommerceAgentConfigPage />) },
                    { path: "ecommerce/library", element: adminRoute("admin.ecommerce.knowledge", <EcommerceManagementPage />) },
                    { path: "ecommerce/library/skills", element: adminRoute("admin.ecommerce.knowledge", <EcommerceManagementPage />) },
                    { path: "ecommerce/library/categories", element: adminRoute("admin.ecommerce.knowledge", <EcommerceManagementPage />) },
                    { path: "login-logs", element: adminRoute("admin.users.accounts", <LoginLogsPage />) },
                    { path: "feedbacks", element: adminRoute("admin.users.feedbacks", <AdminFeedbacksPage />) },
                    {
                        path: "administrators",
                        element: (
                            <RequireFullAdmin>
                                <AdministratorsPage />
                            </RequireFullAdmin>
                        ),
                    },
                    { path: "channels", element: adminRoute("admin.platform.channels", <ChannelsPage />) },
                    {
                        path: "models",
                        element: adminRoute(
                            "admin.platform.logical_models",
                            <RequireFeature feature="frontendModelsEnabled">
                                <LogicalModelsPage />
                            </RequireFeature>,
                        ),
                    },
                    { path: "plugins", element: adminRoute("admin.platform.plugins", <AdminPluginsPage />) },
                    { path: "skills", element: adminRoute("admin.platform.skills", <AdminSkillsPage />) },
                    { path: "payments", element: adminRoute("admin.finance.payment_providers", <AdminPaymentsPage view="providers" />) },
                    { path: "payment-orders", element: adminRoute("admin.finance.payment_orders", <AdminPaymentsPage view="orders" />) },
                    { path: "payment-reconciliation", element: adminRoute("admin.finance.reconciliation", <AdminPaymentsPage view="reconciliation" />) },
                    { path: "product-operations", element: adminRoute("admin.commerce.products", <ProductOperationsPage />) },
                    { path: "prompt-templates", element: adminRoute("admin.platform.prompt_templates", <StoryboardPromptsPage />) },
                    { path: "workbench-tools", element: adminRoute("admin.platform.workbench_tools", <WorkbenchToolsPage />) },
                    { path: "skill-curation", element: adminRoute("admin.platform.skills", <SkillCurationPage />) },
                    { path: "storyboard-prompts", element: <Navigate to="/admin/prompt-templates" replace /> },
                    { path: "announcements", element: adminRoute("admin.content.announcements", <AnnouncementsPage />) },
                    { path: "customer-service", element: adminRoute("admin.users.customer_service", <CustomerServicePage />) },
                    { path: "inspirations", element: adminRoute("admin.content.inspirations", <InspirationsPage />) },
                    { path: "banner-announcements", element: adminRoute("admin.content.banner_announcements", <BannerAnnouncementsPage />) },
                    { path: "agent-lessons", element: adminRoute("admin.users.agent_lessons", <AgentLessonsPage />) },
                    { path: "resources", element: adminRoute("admin.storage.resources", <StorageResourcesPage />) },
                    { path: "credit-operations", element: adminRoute("admin.finance.credits", <CreditOperationsPage />) },
                    { path: "referrals", element: adminRoute("admin.finance.referrals", <ReferralsAdminPage />) },
                    { path: "redemption-codes", element: adminRoute("admin.commerce.redeem_codes", <RedemptionCodesPage />) },
                    { path: "logs", element: adminRoute("admin.analytics.api_logs", <LogsPage />) },
                    { path: "settings", element: <Navigate to="runtime-policy" replace /> },
                    { path: "settings/appearance", element: adminRoute("admin.settings.appearance", <AppearanceSettingsPage />) },
                    { path: "settings/drawing-engine", element: adminRoute("admin.settings.drawing_engine", <DrawingEngineSettingsPage />) },
                    { path: "settings/concurrency", element: <Navigate to="/admin/settings/runtime-policy" replace /> },
                    { path: "settings/runtime-policy", element: adminRoute("admin.storage.runtime_policy", <RuntimePolicySettingsPage />) },
                    { path: "settings/features", element: adminRoute("admin.settings.features", <FeatureAvailabilityPage />) },
                    { path: "settings/access", element: adminRoute("admin.settings.access", <AccessSettingsPage />) },
                    { path: "settings/email", element: adminRoute("admin.settings.email", <EmailSettingsPage />) },
                    { path: "settings/storage", element: adminRoute("admin.storage.service", <StorageSettingsPage />) },
                    { path: "settings/ark-private-assets", element: adminRoute("admin.settings.ark_private_assets", <ArkPrivateAssetsSettingsPage />) },
                    { path: "settings/response-interception", element: adminRoute("admin.settings.response_interception", <ResponseInterceptionSettingsPage />) },
                    { path: "settings/third-party", element: adminRoute("admin.settings.third_party", <ThirdPartySettingsPage />) },
                    { path: "settings/system-update", element: adminRoute("admin.settings.system_update", <SystemUpdatePage />) },
                    { path: "settings/system-performance", element: adminRoute("admin.settings.system_performance", <SystemPerformancePage />) },
                    { path: "settings/libtv", element: <Navigate to="/admin/settings/third-party" replace /> },
                ],
            },
        ],
    },
    { path: "*", element: fullScreenDeferred(<NotFound />) },
]);
