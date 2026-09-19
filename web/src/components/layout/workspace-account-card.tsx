import { Button } from "antd";
import { ArrowUpRight, ChevronDown, Coins, HardDrive, LogOut, RefreshCw, Settings, ShieldCheck } from "lucide-react";
import { Link } from "react-router";
import { useAccountFileStorageUsage } from "@/hooks/use-account-file-storage-usage";
import { useWalletBalance } from "@/hooks/use-wallet-balance";
import { useWorkspaceLogout } from "@/hooks/use-workspace-logout";
import { accountStorageMeter } from "@/lib/account-storage-usage";
import { useUserStore } from "@/stores/use-user-store";
import { UserAvatar } from "./user-avatar";
import "./workspace-account-card.css";

/** 账户菜单卡片；余额、积分购买与退出均复用真实服务。 */
export function WorkspaceAccountCard({ onBuyCredits, onNavigate }: { onBuyCredits: () => void; onNavigate: () => void }) {
    const user = useUserStore((state) => state.user);
    const creditsEnabled = useUserStore((state) => state.features.creditsEnabled);
    const { availableMicrocredits, refreshing, refresh } = useWalletBalance(user?.id, creditsEnabled);
    const storageQuery = useAccountFileStorageUsage(Boolean(user));
    const storageMeter = accountStorageMeter(storageQuery.data);
    const { handleLogout, loggingOut } = useWorkspaceLogout();
    if (!user) return null;
    return <section className="workspace-account-card" aria-label="我的账户">
        <header className="workspace-account-card-identity">
            <UserAvatar user={user} className="workspace-account-card-avatar" />
            <div><strong>{user.displayName || user.username}</strong><span>@{user.username}</span></div>
            <em>{user.role === "admin" ? "管理员" : "创作者"}</em>
        </header>
        {creditsEnabled ? <div className="workspace-account-card-wallet">
            <div className="workspace-account-card-balance"><span><Coins />可用积分</span><strong>{availableMicrocredits === null ? "—" : (availableMicrocredits / 1_000_000).toLocaleString("zh-CN", { maximumFractionDigits: 2 })}</strong></div>
            {availableMicrocredits === null ? <Button size="small" loading={refreshing} icon={<RefreshCw />} onClick={() => void refresh()}>刷新余额</Button> : <button type="button" onClick={onBuyCredits}>购买积分<ArrowUpRight /></button>}
        </div> : null}
        <details className={`workspace-account-card-storage is-${storageMeter.tone}`}>
            <summary>
                <HardDrive />
                <span>存储用量</span>
                <small>{storageQuery.data ? `${storageMeter.usedLabel} / ${storageMeter.totalLabel}` : storageQuery.isError ? "暂不可用" : "读取中"}</small>
                <ChevronDown className="workspace-account-card-storage-chevron" />
            </summary>
            <div className="workspace-account-card-storage-detail">
                {storageQuery.isError && !storageQuery.data ? (
                    <div className="workspace-account-card-storage-error">
                        <span>容量统计暂时不可用</span>
                        <button type="button" onClick={() => void storageQuery.refetch()}>重新加载</button>
                    </div>
                ) : (
                    <>
                        <div className="workspace-account-card-storage-heading"><span>账号文件容量</span><strong>{storageQuery.data ? storageMeter.percentLabel : "—"}</strong></div>
                        <span className="workspace-account-card-storage-track" role="progressbar" aria-label="账号文件容量使用进度" aria-valuemin={0} aria-valuemax={storageQuery.data?.totalBytes ?? 0} aria-valuenow={storageQuery.data ? Math.min(storageQuery.data.usedBytes, storageQuery.data.totalBytes) : 0}>
                            <span style={{ width: `${storageMeter.percent}%` }} />
                        </span>
                        <div className="workspace-account-card-storage-meta">
                            <span>已用 {storageQuery.data ? storageMeter.usedLabel : "读取中"}</span>
                            <span>剩余 {storageQuery.data ? storageMeter.remainingLabel : "—"}</span>
                        </div>
                    </>
                )}
            </div>
        </details>
        <nav className="workspace-account-card-actions" aria-label="账户操作">
            <Link to="/settings" onClick={onNavigate}><Settings /><span>账户与设置</span><ArrowUpRight /></Link>
            {user.role === "admin" ? <Link to="/admin" onClick={onNavigate}><ShieldCheck /><span>管理员后台</span><ArrowUpRight /></Link> : null}
            <Button danger type="text" icon={<LogOut />} loading={loggingOut} onClick={() => void handleLogout()}>退出登录</Button>
        </nav>
    </section>;
}
