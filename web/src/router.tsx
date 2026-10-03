import { lazy, Suspense, type ReactNode } from "react";
import { createBrowserRouter, Navigate, Outlet, useLocation } from "react-router";

import { RequireAuth } from "@/components/auth/require-auth";
import { AdminLanding, RequireAdminPermission, RequireFullAdmin } from "@/components/auth/require-admin-permission";
import type { AdminPermission } from "@/lib/admin-permissions";
import { CustomerServiceWidget } from "@/components/customer-service/customer-service-widget";
import { FullScreenLoader, WorkspaceRouteLoader } from "@/components/ui/aceternity/full-screen-loader";
import { loadAssetsPage, loadCanvasPage, loadCanvasProjectPage, loadCreatePage, loadProjectDetailPage, loadProjectsPage } from "@/lib/workspace-route-modules";
import { CanvasRefreshShell } from "@/pages/canvas/canvas-refresh-shell";
import { AuthScene } from "@/pages/auth/auth-scene";
import RouteErrorPage from "@/pages/route-error";

const AdminPage = lazy(() => import("@/pages/admin"));
const AnalyticsPage = lazy(() => import("@/pages/admin/admin-route-pages").then((module) => ({ default: module.AnalyticsPage })));
const AnnouncementsPage = lazy(() => import("@/pages/admin/admin-route-pages").then((module) => ({ default: module.AnnouncementsPage })));
const BannerAnnouncementsPage = lazy(() => import("@/pages/admin/admin-route-pages").then((module) => ({ default: module.BannerAnnouncementsPage })));
const StorageResourcesPage = lazy(() => import("@/pages/admin/admin-route-pages").then((module) => ({ default: module.StorageResourcesPage })));
const CreditOperationsPage = lazy(() => import("@/pages/admin/admin-route-pages").then((module) => ({ default: module.CreditOperationsPage })));
const AccessSettingsPage = lazy(() => import("@/pages/admin/admin-route-pages").then((module) => ({ default: module.AccessSettingsPage })));
const EmailSettingsPage = lazy(() => import("@/pages/admin/admin-route-pages").then((module) => ({ default: module.EmailSettingsPage })));
const FeatureAvailabilityPage = lazy(() => import("@/pages/admin/admin-route-pages").then((module) => ({ default: module.FeatureAvailabilityPage })));
const AgentLessonsPage = lazy(() => import("@/pages/admin/admin-route-pages").then((module) => ({ default: module.AgentLessonsPage })));
const InspirationsPage = lazy(() => import("@/pages/admin/admin-route-pages").then((module) => ({ default: module.InspirationsPage })));
const ChannelsPage = lazy(() => import("@/pages/admin/channels/channels-page"));
const LogicalModelsPage = lazy(() => import("@/pages/admin/logical-models/logical-models-page"));
const AdminPluginsPage = lazy(() => import("@/pages/admin/plugins/plugins-page"));
const AdminSkillsPage = lazy(() => import("@/pages/admin/skills/skills-page"));
const AdminPaymentsPage = lazy(() => import("@/pages/admin/payments/payments-page"));
const ProductOperationsPage = lazy(() => import("@/pages/admin/product-operations/product-operations-page"));
const CustomerServicePage = lazy(() => import("@/pages/admin/customer-service/customer-service-page"));
const LogsPage = lazy(() => import("@/pages/admin/logs/logs-page"));
const RedemptionCodesPage = lazy(() => import("@/pages/admin/redemption-codes/redemption-codes-page"));
const RuntimePolicySettingsPage = lazy(() => import("@/pages/admin/settings/runtime-policy-settings-page"));
const AppearanceSettingsPage = lazy(() => import("@/pages/admin/settings/appearance-settings-page"));
const DrawingEngineSettingsPage = lazy(() => import("@/pages/admin/settings/drawing-engine-settings-page"));
const StorageSettingsPage = lazy(() => import("@/pages/admin/settings/storage-settings-page"));
const ArkPrivateAssetsSettingsPage = lazy(() => import("@/pages/admin/settings/ark-private-assets-settings-page"));
const ResponseInterceptionSettingsPage = lazy(() => import("@/pages/admin/settings/response-interception-settings-page"));
const ThirdPartySettingsPage = lazy(() => import("@/pages/admin/settings/libtv-settings-page"));
const SystemUpdatePage = lazy(() => import("@/pages/admin/settings/system-update-page"));
const SystemPerformancePage = lazy(() => import("@/pages/admin/settings/system-performance-page"));
const StoryboardPromptsPage = lazy(() => import("@/pages/admin/storyboard-prompts/storyboard-prompts-page"));
const UsersPage = lazy(() => import("@/pages/admin/users/users-page"));
const AdministratorsPage = lazy(() => import("@/pages/admin/users/administrators-page"));
const AssetsPage = lazy(loadAssetsPage);
const PromptsPage = lazy(() => import("@/pages/prompts"));
const LoginPage = lazy(() => import("@/pages/auth/login"));
const RegisterPage = lazy(() => import("@/pages/auth/register"));
const ForgotPasswordPage = lazy(() => import("@/pages/auth/forgot-password"));
const CanvasPage = lazy(loadCanvasPage);
const CanvasProjectPage = lazy(loadCanvasProjectPage);
const SharedCanvasPage = lazy(() => import("@/pages/canvas/shared"));
const CreatePage = lazy(loadCreatePage);
const NotFound = lazy(() => import("@/pages/not-found"));
const SkillsPage = lazy(() => import("@/pages/skills"));
const PluginsPage = lazy(() => import("@/pages/plugins"));
const EagleLibraryPage = lazy(() => import("@/pages/plugins/eagle"));
const TasksPage = lazy(() => import("@/pages/tasks"));
const ProjectsPage = lazy(loadProjectsPage);
const ProjectDetailPage = lazy(loadProjectDetailPage);
const SettingsPage = lazy(() => import("@/pages/settings"));
const ProfilePage = lazy(() => import("@/pages/profile"));
const TestVoiceRecording = lazy(() => import("@/pages/test-voice-recording"));
const UserLayout = lazy(() => import("@/layouts/user-layout"));
const RequireFeature = lazy(() => import("@/components/auth/require-feature").then((module) => ({ default: module.RequireFeature })));

function deferred(element: ReactNode) {
    return <Suspense fallback={<WorkspaceRouteLoader />}>{element}</Suspense>;
}

function fullScreenDeferred(element: ReactNode) {
    return <Suspense fallback={<FullScreenLoader label="正在打开创作空间" detail="准备当前页面" />}>{element}</Suspense>;
}

function adminRoute(permission: AdminPermission, element: ReactNode) {
    return <RequireAdminPermission permission={permission}>{element}</RequireAdminPermission>;
}

function AuthenticatedWorkspaceLayout() {
    const { pathname } = useLocation();
    const isCanvasProjectRoute = pathname.startsWith("/canvas/");
    const fallback = isCanvasProjectRoute ? <CanvasRefreshShell /> : <FullScreenLoader label="正在打开创作空间" detail="准备当前页面" />;
    return <RequireAuth><Suspense fallback={fallback}><UserLayout><Outlet /></UserLayout></Suspense><CustomerServiceWidget /></RequireAuth>;
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
    const DirectorReproLab = lazy(() => import("@/pages/dev/director-repro-lab"));
    return [
        { path: "/dev/folders", element: fullScreenDeferred(<FolderPreviewLab />), errorElement: <RouteErrorPage /> },
        { path: "/dev/director-repro", element: fullScreenDeferred(<DirectorReproLab />), errorElement: <RouteErrorPage /> },
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
            { path: "/", element: <RequireAuth>{deferred(<CreatePage />)}</RequireAuth> },
            { path: "/create", element: <RequireAuth>{deferred(<CreatePage />)}</RequireAuth> },
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
                path: "/projects/:projectId",
                element: (
                    <RequireAuth>
                        <RequireFeature feature="shortDramaEnabled">{deferred(<ProjectDetailPage />)}</RequireFeature>
                    </RequireAuth>
                ),
            },
            {
                path: "/projects/:projectId/:view",
                element: (
                    <RequireAuth>
                        <RequireFeature feature="shortDramaEnabled">{deferred(<ProjectDetailPage />)}</RequireFeature>
                    </RequireAuth>
                ),
            },
            {
                path: "/projects/:projectId/chapters/:chapterId",
                element: (
                    <RequireAuth>
                        <RequireFeature feature="shortDramaEnabled">{deferred(<ProjectDetailPage />)}</RequireFeature>
                    </RequireAuth>
                ),
            },
            {
                path: "/projects/:projectId/workflow/:unitId/:stage",
                element: (
                    <RequireAuth>
                        <RequireFeature feature="shortDramaEnabled">{deferred(<ProjectDetailPage />)}</RequireFeature>
                    </RequireAuth>
                ),
            },
            { path: "/canvas", element: <RequireAuth>{deferred(<CanvasPage />)}</RequireAuth> },
            { path: "/canvas/:id", element: <RequireAuth><CanvasProjectPage /></RequireAuth> },
            {
                path: "/admin",
                element: <RequireAuth>{deferred(<AdminPage />)}</RequireAuth>,
                children: [
                    { index: true, element: <AdminLanding>{adminRoute("admin.analytics.overview", <AnalyticsPage />)}</AdminLanding> },
                    { path: "users", element: adminRoute("admin.users.accounts", <UsersPage />) },
                    { path: "administrators", element: <RequireFullAdmin><AdministratorsPage /></RequireFullAdmin> },
                    { path: "channels", element: adminRoute("admin.platform.channels", <ChannelsPage />) },
                    { path: "models", element: adminRoute("admin.platform.logical_models", <RequireFeature feature="frontendModelsEnabled"><LogicalModelsPage /></RequireFeature>) },
                    { path: "plugins", element: adminRoute("admin.platform.plugins", <AdminPluginsPage />) },
                    { path: "skills", element: adminRoute("admin.platform.skills", <AdminSkillsPage />) },
                    { path: "payments", element: adminRoute("admin.finance.payment_providers", <AdminPaymentsPage view="providers" />) },
                    { path: "payment-orders", element: adminRoute("admin.finance.payment_orders", <AdminPaymentsPage view="orders" />) },
                    { path: "payment-reconciliation", element: adminRoute("admin.finance.reconciliation", <AdminPaymentsPage view="reconciliation" />) },
                    { path: "product-operations", element: adminRoute("admin.commerce.products", <ProductOperationsPage />) },
                    { path: "prompt-templates", element: adminRoute("admin.platform.prompt_templates", <StoryboardPromptsPage />) },
                    { path: "storyboard-prompts", element: <Navigate to="/admin/prompt-templates" replace /> },
                    { path: "announcements", element: adminRoute("admin.content.announcements", <AnnouncementsPage />) },
                    { path: "customer-service", element: adminRoute("admin.users.customer_service", <CustomerServicePage />) },
                    { path: "inspirations", element: adminRoute("admin.content.inspirations", <InspirationsPage />) },
                    { path: "banner-announcements", element: adminRoute("admin.content.banner_announcements", <BannerAnnouncementsPage />) },
                    { path: "agent-lessons", element: adminRoute("admin.users.agent_lessons", <AgentLessonsPage />) },
                    { path: "resources", element: adminRoute("admin.storage.resources", <StorageResourcesPage />) },
                    { path: "credit-operations", element: adminRoute("admin.finance.credits", <CreditOperationsPage />) },
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
