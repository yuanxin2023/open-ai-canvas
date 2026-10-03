import { useEffect, useMemo, useRef, useState } from "react";

import { paymentPromotionImageUrl, type PublicPaymentPromotion } from "@/services/api/payments";
import { cn } from "@/lib/utils";

export type PromotionCountdown = { days: number; hours: number; minutes: number; seconds: number; totalMs: number };

export function promotionClockOffset(serverTime: string, receivedAt = Date.now()) {
    const parsed = Date.parse(serverTime);
    return Number.isFinite(parsed) ? parsed - receivedAt : 0;
}

export function promotionCountdown(endsAt: string | undefined, clockOffsetMs = 0, now = Date.now()): PromotionCountdown {
    const end = endsAt ? Date.parse(endsAt) : Number.NaN;
    const totalMs = Number.isFinite(end) ? Math.max(0, end - (now + clockOffsetMs)) : 0;
    const totalSeconds = Math.floor(totalMs / 1000);
    return {
        days: Math.floor(totalSeconds / 86_400),
        hours: Math.floor((totalSeconds % 86_400) / 3_600),
        minutes: Math.floor((totalSeconds % 3_600) / 60),
        seconds: totalSeconds % 60,
        totalMs,
    };
}

export function PaymentPromotionBanner({ promotion, serverTime, className, onBoundary }: { promotion: PublicPaymentPromotion; serverTime: string; className?: string; onBoundary?: () => void }) {
    const clockOffsetMs = useMemo(() => promotionClockOffset(serverTime), [serverTime]);
    const [now, setNow] = useState(Date.now());
    const boundaryReported = useRef(false);
    const active = promotion.phase === "active";
    const countdown = promotionCountdown(promotion.endsAt, clockOffsetMs, now);
    const showCopy = active || promotion.inactiveCopyEnabled;
    const title = active ? promotion.activeTitle : promotion.inactiveTitle;
    const subtitle = active ? promotion.activeSubtitle : promotion.inactiveSubtitle;

    useEffect(() => {
        boundaryReported.current = false;
        if (!active) return;
        const interval = window.setInterval(() => setNow(Date.now()), 1_000);
        return () => window.clearInterval(interval);
    }, [active, promotion.endsAt]);

    useEffect(() => {
        if (!active || countdown.totalMs > 0 || boundaryReported.current) return;
        boundaryReported.current = true;
        onBoundary?.();
    }, [active, countdown.totalMs, onBoundary]);

    if (!promotion.visible || !promotion.imageUrl) return null;
    const imageUrl = paymentPromotionImageUrl(promotion);
    const values = [countdown.days, countdown.hours, countdown.minutes, countdown.seconds];
    const labels = ["天", "时", "分", "秒"];

    return (
        <section className={cn("workspace-payment-promotion", !showCopy && "is-image-only", className)} style={{ backgroundImage: `url("${imageUrl}")` }} aria-label={active ? "限时促销活动" : "套餐宣传"}>
            <div className="workspace-payment-promotion-overlay" />
            {showCopy ? (
                <div className="workspace-payment-promotion-copy">
                    {title ? <strong>{title}</strong> : null}
                    {subtitle ? <span>{subtitle}</span> : null}
                </div>
            ) : null}
            {active ? (
                <div className="workspace-payment-promotion-countdown" aria-label={`活动剩余 ${countdown.days} 天 ${countdown.hours} 小时 ${countdown.minutes} 分 ${countdown.seconds} 秒`}>
                    {values.map((value, index) => (
                        <div key={labels[index]}>
                            <strong>{String(value).padStart(2, "0")}</strong>
                            <span>{labels[index]}</span>
                        </div>
                    ))}
                </div>
            ) : null}
        </section>
    );
}
