import { translate } from "@/i18n";
import { useTranslation } from "react-i18next";
import { Button } from "antd";
import { ArrowUpRight, ChevronDown, Coins, Gift, HardDrive, LogOut, RefreshCw, Settings, ShieldCheck } from "lucide-react";
import { Link } from "react-router";
import { useAccountFileStorageUsage } from "@/hooks/use-account-file-storage-usage";
import { useWalletBalance } from "@/hooks/use-wallet-balance";
import { useWorkspaceLogout } from "@/hooks/use-workspace-logout";
import { accountStorageMeter } from "@/lib/account-storage-usage";
import { useUserStore } from "@/stores/use-user-store";
import { UserAvatar } from "./user-avatar";
import { UserGroupAvatarRing, UserGroupBadge } from "./user-group-badge";
import "./workspace-account-card.css";

/** 账户菜单卡片；余额、积分购买与退出均复用真实服务。 */
export function WorkspaceAccountCard({ onBuyCredits, onNavigate }: { onBuyCredits: () => void; onNavigate: () => void }) {
    const user = useUserStore((state) => state.user);
    const creditsEnabled = useUserStore((state) => state.features.creditsEnabled);
    const { availableMicrocredits, refreshing, refresh } = useWalletBalance(user?.id, creditsEnabled);
    const storageQuery = useAccountFileStorageUsage(Boolean(user));
    const storageMeter = accountStorageMeter(storageQuery.data);
    const { handleLogout, loggingOut } = useWorkspaceLogout();
    const { t } = useTranslation();
    if (!user) return null;
    return (
        <section className="workspace-account-card" aria-label={t("shell.account.region")}>
            <header className="workspace-account-card-identity">
                <UserGroupAvatarRing>
                    <UserAvatar user={user} className="workspace-account-card-avatar" />
                </UserGroupAvatarRing>
                <div>
                    <strong className="workspace-account-card-name">
                        <span>{user.username}</span>
                        <UserGroupBadge />
                    </strong>
                    <span>{user.email || `用户 ${user.id.slice(0, 8)}`}</span>
                </div>
                <em>{user.role === "admin" ? translate("shell.account.admin") : translate("shell.account.creator")}</em>
            </header>
            {creditsEnabled ? (
                <div className="workspace-account-card-wallet">
                    <div className="workspace-account-card-balance">
                        <span>
                            <Coins />
                            {translate("shell.account.availableCredits")}
                        </span>
                        <strong>{availableMicrocredits === null ? "—" : (availableMicrocredits / 1_000_000).toLocaleString("zh-CN", { maximumFractionDigits: 2 })}</strong>
                    </div>
                    {availableMicrocredits === null ? (
                        <Button size="small" loading={refreshing} icon={<RefreshCw />} onClick={() => void refresh()}>
                            {translate("shell.account.refreshBalance")}
                        </Button>
                    ) : (
                        <button type="button" onClick={onBuyCredits}>
                            {translate("local.ui.buyCredits")}
                            <ArrowUpRight />
                        </button>
                    )}
                </div>
            ) : null}
            <details className={`workspace-account-card-storage is-${storageMeter.tone}`}>
                <summary>
                    <HardDrive />
                    <span>{translate("local.ui.storageUsage")}</span>
                    <small>{storageQuery.data ? `${storageMeter.usedLabel} / ${storageMeter.totalLabel}` : storageQuery.isError ? translate("local.ui.unavailable") : translate("account.settings.diagnostics.loading")}</small>
                    <ChevronDown className="workspace-account-card-storage-chevron" />
                </summary>
                <div className="workspace-account-card-storage-detail">
                    {storageQuery.isError && !storageQuery.data ? (
                        <div className="workspace-account-card-storage-error">
                            <span>{translate("local.ui.storageUnavailable")}</span>
                            <button type="button" onClick={() => void storageQuery.refetch()}>
                                {translate("account.layout.state.reload")}
                            </button>
                        </div>
                    ) : (
                        <>
                            <div className="workspace-account-card-storage-heading">
                                <span>{translate("library.assets.storage.label")}</span>
                                <strong>{storageQuery.data ? storageMeter.percentLabel : "—"}</strong>
                            </div>
                            <span
                                className="workspace-account-card-storage-track"
                                role="progressbar"
                                aria-label={translate("account.layout.storage.progress")}
                                aria-valuemin={0}
                                aria-valuemax={storageQuery.data?.totalBytes ?? 0}
                                aria-valuenow={storageQuery.data ? Math.min(storageQuery.data.usedBytes, storageQuery.data.totalBytes) : 0}
                            >
                                <span style={{ width: `${storageMeter.percent}%` }} />
                            </span>
                            <div className="workspace-account-card-storage-meta">
                                <span>
                                    {translate("local.ui.used")}
                                    {storageQuery.data ? storageMeter.usedLabel : translate("account.settings.diagnostics.loading")}
                                </span>
                                <span>
                                    {translate("local.ui.remaining")}
                                    {storageQuery.data ? storageMeter.remainingLabel : "—"}
                                </span>
                            </div>
                        </>
                    )}
                </div>
            </details>
            <nav className="workspace-account-card-actions" aria-label={translate("shell.account.actions")}>
                <Link to="/settings" onClick={onNavigate}>
                    <Settings />
                    <span>{translate("local.ui.accountManagement")}</span>
                    <ArrowUpRight />
                </Link>
                {creditsEnabled ? (
                    <Link to="/referrals" onClick={onNavigate}>
                        <Gift />
                        <span>{translate("local.ui.referralRewards")}</span>
                        <ArrowUpRight />
                    </Link>
                ) : null}
                {user.role === "admin" ? (
                    <Link to="/admin" onClick={onNavigate}>
                        <ShieldCheck />
                        <span>{translate("shell.account.adminConsole")}</span>
                        <ArrowUpRight />
                    </Link>
                ) : null}
                <Button danger type="text" icon={<LogOut />} loading={loggingOut} onClick={() => void handleLogout()}>
                    {translate("shell.account.logout")}
                </Button>
            </nav>
        </section>
    );
}
