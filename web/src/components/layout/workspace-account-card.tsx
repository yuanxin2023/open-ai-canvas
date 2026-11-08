import { Button } from "antd";
import { ArrowUpRight, ChevronDown, Coins, Gift, HardDrive, LogOut, RefreshCw, Settings, ShieldCheck } from "lucide-react";
import { Link } from "react-router";
import { useTranslation } from "react-i18next";
import { currentLocale } from "@/i18n";
import { useAccountFileStorageUsage } from "@/hooks/use-account-file-storage-usage";
import { accountStorageMeter } from "@/lib/account-storage-usage";
import { progressFillStyle } from "@/lib/progress-fill";
import { useWalletBalance } from "@/hooks/use-wallet-balance";
import { useWorkspaceLogout } from "@/hooks/use-workspace-logout";
import { useUserStore } from "@/stores/use-user-store";
import { UserAvatar } from "./user-avatar";
import { UserGroupAvatarRing, UserGroupBadge } from "./user-group-badge";
import "./workspace-account-card.css";

/** 顶部账户卡片集中展示余额、文件容量和账户操作。 */
export function WorkspaceAccountCard({ onWallet, onNavigate }: { onWallet: () => void; onNavigate: () => void }) {
    const user = useUserStore((state) => state.user);
    const creditsEnabled = useUserStore((state) => state.features.creditsEnabled);
    const { availableMicrocredits, refreshing, refresh } = useWalletBalance(user?.id, creditsEnabled);
    const storageQuery = useAccountFileStorageUsage(Boolean(user));
    const meter = accountStorageMeter(storageQuery.data);
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
                    <span>@{user.username}</span>
                </div>
                <em>{user.role === "admin" ? t("shell.account.admin") : t("shell.account.creator")}</em>
            </header>
            {creditsEnabled ? (
                <div className="workspace-account-card-wallet">
                    <div className="workspace-account-card-balance">
                        <span>
                            <Coins />
                            {t("shell.account.availableCredits")}
                        </span>
                        <strong>{availableMicrocredits === null ? "—" : (availableMicrocredits / 1_000_000).toLocaleString(currentLocale(), { maximumFractionDigits: 2 })}</strong>
                    </div>
                    {availableMicrocredits === null ? (
                        <Button size="small" loading={refreshing} icon={<RefreshCw />} onClick={() => void refresh()}>
                            {t("shell.account.refreshBalance")}
                        </Button>
                    ) : (
                        <button type="button" onClick={onWallet}>
                            {t("shell.account.topup")}
                            <ArrowUpRight />
                        </button>
                    )}
                </div>
            ) : null}
            <details className={`workspace-account-card-storage is-${meter.tone}`}>
                <summary>
                    <HardDrive aria-hidden />
                    <span>{t("local.ui.storageUsage")}</span>
                    <small>{storageQuery.data ? `${meter.usedLabel} / ${meter.totalLabel}` : storageQuery.isError ? t("local.ui.unavailable") : t("account.layout.storage.counting")}</small>
                    <ChevronDown className="workspace-account-card-storage-chevron" aria-hidden />
                </summary>
                <div className="workspace-account-card-storage-detail">
                    {storageQuery.data ? (
                        <>
                            <div className="workspace-account-card-storage-heading">
                                <span>{t("local.ui.storageUsage")}</span>
                                <strong>{meter.percentLabel}</strong>
                            </div>
                            <span
                                className="workspace-account-card-storage-track"
                                role="progressbar"
                                aria-label={t("account.layout.storage.progress")}
                                aria-valuemin={0}
                                aria-valuemax={storageQuery.data.totalBytes}
                                aria-valuenow={Math.min(storageQuery.data.usedBytes, storageQuery.data.totalBytes)}
                                aria-valuetext={t("account.layout.storage.summary", {
                                    used: t("account.layout.storage.used", { size: meter.usedLabel }),
                                    remaining: t("account.layout.storage.remaining", { size: meter.remainingLabel }),
                                    total: t("account.layout.storage.total", { size: meter.totalLabel }),
                                })}
                            >
                                <span style={progressFillStyle(meter.percent)} />
                            </span>
                            <div className="workspace-account-card-storage-meta">
                                <span>{t("account.layout.storage.used", { size: meter.usedLabel })}</span>
                                <span>{t("account.layout.storage.remaining", { size: meter.remainingLabel })}</span>
                            </div>
                            {meter.full ? <p role="status">{t("account.layout.storage.full")}</p> : null}
                        </>
                    ) : (
                        <span>{storageQuery.isError ? t("local.ui.storageUnavailable") : t("account.layout.storage.counting")}</span>
                    )}
                    {storageQuery.isError ? (
                        <button type="button" className="workspace-account-card-storage-retry" disabled={storageQuery.isFetching} onClick={() => void storageQuery.refetch()}>
                            {t("common.retry")}
                        </button>
                    ) : null}
                </div>
            </details>
            <nav className="workspace-account-card-actions" aria-label={t("shell.account.actions")}>
                <Link to="/settings?section=profile" onClick={onNavigate}>
                    <Settings />
                    <span>{t("shell.account.settings")}</span>
                    <ArrowUpRight />
                </Link>
                {creditsEnabled ? (
                    <Link to="/referrals" onClick={onNavigate}>
                        <Gift />
                        <span>{t("shell.account.referral")}</span>
                        <ArrowUpRight />
                    </Link>
                ) : null}
                {user.role === "admin" ? (
                    <Link to="/admin" onClick={onNavigate}>
                        <ShieldCheck />
                        <span>{t("shell.account.adminConsole")}</span>
                        <ArrowUpRight />
                    </Link>
                ) : null}
                <Button danger type="text" icon={<LogOut />} loading={loggingOut} onClick={() => void handleLogout()}>
                    {t("shell.account.logout")}
                </Button>
            </nav>
        </section>
    );
}
