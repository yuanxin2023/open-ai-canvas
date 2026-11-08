import { Button, ConfigProvider, Input, Popover } from "antd";
import { CreditCard, Gift, Sparkles } from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { WorkspaceCreditGiftMark } from "@/components/layout/workspace-credit-gift-mark";
import { formatCredits } from "@/constant/credits";
import { useWalletBalance } from "@/hooks/use-wallet-balance";
import { useCreditRedemption } from "@/hooks/use-credit-redemption";
import { getWorkspaceAntThemeConfig } from "@/lib/app-theme";
import { openWorkspaceWallet } from "@/lib/workspace-wallet";

const HOVER_POINTER_QUERY = "(hover: hover) and (pointer: fine)";
const workspaceCreditTheme = getWorkspaceAntThemeConfig();

export function WorkspaceCreditPopover({ userId, variant }: { userId: string; variant?: "topbar" }) {
    const { t } = useTranslation();
    const topbar = variant === "topbar";
    const { availableMicrocredits, policy } = useWalletBalance(userId);
    const [open, setOpen] = useState(false);
    const [canHover, setCanHover] = useState(() => typeof window !== "undefined" && window.matchMedia(HOVER_POINTER_QUERY).matches);
    const { code, setCode, normalizedCode, redeeming, redeem } = useCreditRedemption(userId);
    const balance = availableMicrocredits === null ? "--" : formatCredits(availableMicrocredits);
    const dailyBonus = policy === null ? "--" : formatCredits(policy.checkinBonusMicrocredits, 2);
    useEffect(() => {
        const media = window.matchMedia(HOVER_POINTER_QUERY);
        const updateCanHover = () => {
            setCanHover(media.matches);
            setOpen(false);
        };
        media.addEventListener("change", updateCanHover);
        return () => media.removeEventListener("change", updateCanHover);
    }, []);
    return (
        <ConfigProvider theme={workspaceCreditTheme}>
            <Popover
                trigger={canHover ? "hover" : topbar ? [] : "click"}
                placement={canHover ? "bottomRight" : "bottom"}
                rootClassName="workspace-credit-popover"
                open={open}
                onOpenChange={setOpen}
                content={
                    <div className="workspace-credit-panel">
                        <div className="workspace-credit-panel-heading">
                            <div>
                                <div className="workspace-credit-panel-title">我的积分</div>
                                <div className="workspace-credit-panel-description">兑换后余额将自动刷新</div>
                            </div>
                            <div className="workspace-credit-balance-chip" aria-label={`当前积分余额 ${balance}`}>
                                <Sparkles aria-hidden />
                                <span>余额</span>
                                <strong>{balance}</strong>
                            </div>
                        </div>

                        <div className="workspace-credit-daily-bonus" aria-label={`每日签到赠送 ${dailyBonus} 免费积分`}>
                            <Gift aria-hidden />
                            <div className="workspace-credit-daily-bonus-copy">
                                <strong>每日免费积分</strong>
                                <span>{policy === null ? "正在读取每日签到赠送额度" : `每天签到赠送 ${dailyBonus} 免费积分`}</span>
                            </div>
                            <strong className="workspace-credit-daily-bonus-amount">{dailyBonus}</strong>
                        </div>

                        <div className="workspace-credit-redeem-heading">
                            <span>
                                <Gift aria-hidden />
                                兑换码兑换
                            </span>
                            <small>32 位兑换码</small>
                        </div>
                        <div className="workspace-credit-actions">
                            <div className="workspace-credit-redeem-row">
                                <Input aria-label="兑换码" autoComplete="off" maxLength={32} placeholder="输入兑换码" value={code} disabled={redeeming} onChange={(event) => setCode(event.target.value)} onPressEnter={() => void redeem()} />
                                <Button type="primary" loading={redeeming} disabled={normalizedCode.length !== 32} onClick={() => void redeem()}>
                                    兑换
                                </Button>
                            </div>
                            <Button
                                className="workspace-credit-purchase-button"
                                type="primary"
                                icon={<CreditCard aria-hidden />}
                                onClick={() => {
                                    setOpen(false);
                                    openWorkspaceWallet();
                                }}
                            >
                                购买积分与套餐
                            </Button>
                        </div>
                    </div>
                }
            >
                <button
                    type="button"
                    className={topbar ? "app-workspace-topbar-credit-pill" : "app-workspace-credit-button"}
                    aria-label={topbar ? t("shell.openWallet", { balance }) : canHover ? `购买积分与套餐，当前余额 ${balance}` : `查看积分与兑换码，当前余额 ${balance}`}
                    aria-expanded={canHover || topbar ? undefined : open}
                    onClick={() => {
                        if (!canHover && !topbar) return;
                        setOpen(false);
                        openWorkspaceWallet();
                    }}
                >
                    {topbar ? (
                        <>
                            <WorkspaceCreditGiftMark />
                            <span>{t("shell.credits")}</span>
                            <strong>{balance}</strong>
                        </>
                    ) : (
                        <>
                            <Sparkles aria-hidden />
                            <span>{balance}</span>
                        </>
                    )}
                </button>
            </Popover>
        </ConfigProvider>
    );
}
