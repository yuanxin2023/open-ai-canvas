import { lazy, Suspense, useCallback, useMemo, useRef, useState, type ReactNode } from "react";
import { App, Tabs } from "antd";
import { useLocation, useNavigate } from "react-router";
import { BrandLogo } from "@/components/brand/brand-logo";
import { AppModal } from "@/components/ui/product/app-modal";
import { useAppearanceStore } from "@/stores/use-appearance-store";
import { useUserStore } from "@/stores/use-user-store";
import { AuthFormContext, type AuthView } from "./auth-form-navigation";
import { WorkspaceAuthContext, type WorkspaceAuthRequest } from "./workspace-auth-context";

const LoginForm = lazy(() => import("@/pages/auth/login"));
const RegisterForm = lazy(() => import("@/pages/auth/register"));
const RecoveryForm = lazy(() => import("@/pages/auth/forgot-password"));

export function WorkspaceAuthProvider({ children }: { children: ReactNode }) {
    const [request, setRequest] = useState<{ view: AuthView; next: string } | null>(null);
    const prepareDraft = useRef<(() => Promise<void>) | null>(null);
    const opening = useRef(false);
    const navigate = useNavigate();
    const location = useLocation();
    const { message } = App.useApp();
    const brandName = useAppearanceStore((state) => state.appearance.brandName);
    const openAuth = useCallback(async (options: WorkspaceAuthRequest = {}) => {
        if (opening.current || request) return;
        const next = options.next ?? `${location.pathname}${location.search}${location.hash}`;
        if (useUserStore.getState().user) {
            void navigate(next);
            return;
        }
        opening.current = true;
        try {
            await prepareDraft.current?.();
            setRequest({ view: options.view ?? "login", next });
        } catch (error) {
            message.error(error instanceof Error ? error.message : "保存创作草稿失败，请重试");
        } finally {
            opening.current = false;
        }
    }, [location, message, navigate, request]);
    const context = useMemo(() => ({ openAuth, prepareDraft }), [openAuth]);
    const formContext = useMemo(() => request ? {
        next: request.next,
        navigate: (to: string) => {
            const path = to.split("?")[0];
            if (path === "/login" || path === "/register" || path === "/forgot-password") {
                setRequest((current) => current ? { ...current, view: path.slice(1) as AuthView } : null);
                return;
            }
            setRequest(null);
            if (to !== `${location.pathname}${location.search}${location.hash}`) void navigate(to);
        },
    } : null, [request, location, navigate]);
    return <WorkspaceAuthContext.Provider value={context}>
        {children}
        <AppModal flush open={Boolean(request)} onCancel={() => setRequest(null)} footer={null} centered
            title={null} aria-label="登录或注册" width="min(520px, calc(100vw - 32px))">
            {request && <AuthFormContext.Provider value={formContext}>
                <div className="max-h-[calc(100dvh-64px)] overflow-y-auto rounded-[var(--r-xl)] bg-surface p-6 text-foreground sm:p-10">
                    <div className="mb-8 flex items-center justify-center gap-2 pr-4"><BrandLogo className="size-8 object-contain" fallback={<span aria-hidden>{brandName.slice(0, 1)}</span>} /><span className="text-xl font-semibold">{brandName}</span></div>
                    <h2 className="mb-6 text-center text-xl font-semibold">{request.view === "forgot-password" ? "找回密码" : `欢迎来到 ${brandName}`}</h2>
                    {request.view !== "forgot-password" && <Tabs activeKey={request.view} centered items={[{ key: "login", label: "登录" }, { key: "register", label: "注册" }]} onChange={(view) => setRequest({ ...request, view: view as AuthView })} />}
                    <Suspense fallback={<p role="status" className="py-8 text-center">正在加载…</p>}>
                        {request.view === "login" ? <LoginForm /> : request.view === "register" ? <RegisterForm /> : <RecoveryForm />}
                    </Suspense>
                </div>
            </AuthFormContext.Provider>}
        </AppModal>
    </WorkspaceAuthContext.Provider>;
}
