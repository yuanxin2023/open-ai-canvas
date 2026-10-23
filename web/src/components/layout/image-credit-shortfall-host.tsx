import { Button, Input } from "antd";
import { CheckCircle2, ChevronDown, Coins, CreditCard, TicketCheck } from "lucide-react";
import { useEffect, useRef, useState, type FormEvent } from "react";

import { AppModal } from "@/components/ui/product/app-modal";
import { IMAGE_CREDITS_SHORTFALL_EVENT } from "@/lib/image-credit-guidance";
import { openWorkspaceCreditProducts } from "@/lib/workspace-wallet";
import { redeemCredits } from "@/services/api/wallet";

export function ImageCreditShortfallHost() {
    const [open, setOpen] = useState(false);
    const [redeemExpanded, setRedeemExpanded] = useState(false);
    const [code, setCode] = useState("");
    const [redeeming, setRedeeming] = useState(false);
    const [redeemError, setRedeemError] = useState("");
    const [redeemed, setRedeemed] = useState(false);
    const openRef = useRef(false);
    const redeemingRef = useRef(false);
    const dismissedAt = useRef(0);

    useEffect(() => {
        const onShortfall = () => {
            if (openRef.current || Date.now() - dismissedAt.current < 2000) return;
            openRef.current = true;
            setOpen(true);
        };
        window.addEventListener(IMAGE_CREDITS_SHORTFALL_EVENT, onShortfall);
        return () => window.removeEventListener(IMAGE_CREDITS_SHORTFALL_EVENT, onShortfall);
    }, []);

    const close = () => {
        if (redeemingRef.current) return;
        openRef.current = false;
        dismissedAt.current = Date.now();
        setOpen(false);
        setRedeemExpanded(false);
        setCode("");
        setRedeemError("");
        setRedeemed(false);
    };

    const redeem = async (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        if (redeemingRef.current) return;
        const normalized = code.trim().toLowerCase();
        if (normalized.length !== 32) {
            setRedeemError("请输入完整的 32 位兑换码");
            return;
        }
        redeemingRef.current = true;
        setRedeeming(true);
        setRedeemError("");
        try {
            await redeemCredits(normalized);
            setCode("");
            setRedeemed(true);
            window.dispatchEvent(new CustomEvent("wallet:updated"));
        } catch (error) {
            setRedeemError(error instanceof Error ? error.message : "兑换失败，请检查兑换码后重试");
        } finally {
            redeemingRef.current = false;
            setRedeeming(false);
        }
    };

    return <AppModal
        flush
        open={open}
        centered
        width="min(440px, calc(100vw - 24px))"
        title={null}
        footer={null}
        rootClassName="image-credit-shortfall-modal"
        aria-labelledby="image-credit-shortfall-title"
        closable={!redeeming}
        maskClosable={!redeeming}
        keyboard={!redeeming}
        onCancel={close}
    >
        <div className="image-credit-shortfall-shell">
            <div className={`image-credit-shortfall-icon${redeemed ? " is-success" : ""}`} aria-hidden>
                {redeemed ? <CheckCircle2 /> : <Coins />}
            </div>
            <div className="image-credit-shortfall-copy">
                <span className="image-credit-shortfall-eyebrow">{redeemed ? "积分已到账" : "创作已暂停"}</span>
                <h2 id="image-credit-shortfall-title">{redeemed ? "兑换成功，可以继续创作" : "积分不足，暂时无法生成"}</h2>
                <p>{redeemed ? "返回原任务后，点击“重新生成”继续。" : "当前图片任务需要更多积分。补充积分后，回到原任务点击“重新生成”即可继续。"}</p>
            </div>
            {redeemed ? (
                <Button type="primary" size="large" block className="image-credit-shortfall-main-action" onClick={close}>返回创作</Button>
            ) : (
                <div className="image-credit-shortfall-actions">
                    <Button type="primary" size="large" block icon={<CreditCard aria-hidden />} className="image-credit-shortfall-main-action" disabled={redeeming} onClick={() => {
                        if (redeemingRef.current) return;
                        close();
                        openWorkspaceCreditProducts();
                    }}>购买积分</Button>
                    <Button size="large" block icon={<TicketCheck aria-hidden />} aria-expanded={redeemExpanded} aria-controls={redeemExpanded ? "image-credit-shortfall-redeem" : undefined} className="image-credit-shortfall-redeem-toggle" disabled={redeeming} onClick={() => { setRedeemExpanded((current) => !current); setRedeemError(""); }}>
                        使用兑换码 <ChevronDown className={redeemExpanded ? "is-expanded" : ""} aria-hidden />
                    </Button>
                    {redeemExpanded ? <form id="image-credit-shortfall-redeem" className="image-credit-shortfall-redeem" onSubmit={(event) => void redeem(event)}>
                        <label htmlFor="image-credit-shortfall-code">兑换码</label>
                        <Input id="image-credit-shortfall-code" autoFocus autoComplete="off" value={code} placeholder="输入 32 位兑换码" aria-invalid={Boolean(redeemError)} aria-describedby={redeemError ? "image-credit-shortfall-error" : undefined} disabled={redeeming} onChange={(event) => { setCode(event.target.value.replace(/\s/g, "").slice(0, 32)); setRedeemError(""); }} />
                        {redeemError ? <p id="image-credit-shortfall-error" className="image-credit-shortfall-error" role="alert">{redeemError}</p> : <p className="image-credit-shortfall-hint">兑换成功后，积分会立即存入当前账户。</p>}
                        <Button htmlType="submit" size="large" block loading={redeeming} disabled={!code.length}>确认兑换</Button>
                    </form> : null}
                </div>
            )}
        </div>
    </AppModal>;
}
