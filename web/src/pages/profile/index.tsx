import { Navigate, useSearchParams } from "react-router";

// Linux.do 身份验证回调仍返回此路径，将其送回账号管理的安全页。
export default function ProfilePage() {
    const [searchParams] = useSearchParams();
    const next = new URLSearchParams(searchParams);
    next.set("section", searchParams.get("section") === "security" ? "security" : "profile");
    return <Navigate to={`/settings?${next.toString()}`} replace />;
}
