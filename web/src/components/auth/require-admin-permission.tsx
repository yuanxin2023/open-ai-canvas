import type { ReactNode } from "react";
import { Link, Navigate } from "react-router";

import { firstAdminPath, hasAdminPermission, type AdminPermission } from "@/lib/admin-permissions";
import { useUserStore } from "@/stores/use-user-store";

export function AdminLanding({ children }: { children: ReactNode }) {
    const user = useUserStore((state) => state.user);
    if (!user || user.role !== "admin") return <Navigate to="/" replace />;
    const target = firstAdminPath(user.adminAccess);
    return target === "/admin" ? children : <Navigate to={target} replace />;
}

export function RequireAdminPermission({ permission, children }: { permission: AdminPermission; children: ReactNode }) {
    const user = useUserStore((state) => state.user);
    if (user?.role === "admin" && hasAdminPermission(user.adminAccess, permission)) return children;
    const target = firstAdminPath(user?.adminAccess);
    return (
        <main data-admin-root className="admin-unauthorized">
            <div className="admin-unauthorized-card">
                <h1>无权限</h1>
                <p>当前管理员没有此模块权限。</p>
                <Link to={target}>前往可用模块</Link>
            </div>
        </main>
    );
}

export function RequireFullAdmin({ children }: { children: ReactNode }) {
    const user = useUserStore((state) => state.user);
    if (user?.role === "admin" && user.adminAccess?.level === "full") return children;
    const target = firstAdminPath(user?.adminAccess);
    return (
        <main data-admin-root className="admin-unauthorized">
            <div className="admin-unauthorized-card">
                <h1>无权限</h1>
                <p>管理员配置仅允许全权限管理员访问。</p>
                <Link to={target}>前往可用模块</Link>
            </div>
        </main>
    );
}
