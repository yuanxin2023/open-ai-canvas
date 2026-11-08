import { App } from "antd";
import { useEffect, useRef, useState } from "react";

import { createClientUUID } from "@/lib/client-id";
import { rememberPaymentReturnPath } from "@/lib/workspace-wallet";
import { closePaymentOrder, createPaymentOrder, getPaymentOrder, queryPaymentOrder, refreshPaymentCheckout, type PaymentOrder } from "@/services/api/payments";
import { useUserStore } from "@/stores/use-user-store";

/** One checkout controller for purchases and payment returns, independent of its presentation. */
export function usePaymentOrderFlow({ onGranted }: { onGranted: (order: PaymentOrder) => Promise<void> | void }) {
    const { message } = App.useApp();
    const actorId = useUserStore((state) => state.user?.id);
    const alive = useRef(true);
    const revision = useRef(0);
    const busy = useRef(false);
    const [order, setOrder] = useState<PaymentOrder | null>(null);
    const [creating, setCreating] = useState(false);
    const [querying, setQuerying] = useState(false);
    const [clock, setClock] = useState(Date.now());
    const idempotencyKey = useRef("");
    const grantedOrderIds = useRef(new Set<string>());
    const onGrantedRef = useRef(onGranted);
    onGrantedRef.current = onGranted;
    const current = (requestRevision: number) => alive.current && revision.current === requestRevision && Boolean(actorId) && useUserStore.getState().user?.id === actorId && useUserStore.getState().features.creditsEnabled;

    useEffect(() => {
        alive.current = true;
        return () => {
            alive.current = false;
            revision.current++;
        };
    }, []);

    const accept = async (next: PaymentOrder, requestRevision: number) => {
        if (!current(requestRevision)) return;
        setOrder(next);
        if (next.status !== "credited" || grantedOrderIds.current.has(next.id)) return;
        grantedOrderIds.current.add(next.id);
        await onGrantedRef.current(next);
        if (current(requestRevision)) message.success("支付成功，积分已到账");
    };

    const redirect = (next: PaymentOrder) => {
        if (next.status !== "credited" && next.checkout.mode === "redirect" && next.checkout.url) {
            rememberPaymentReturnPath();
            window.location.assign(next.checkout.url);
        }
    };

    const resetIdempotency = () => {
        idempotencyKey.current = "";
    };

    const start = async (productId: string, providerId: string) => {
        if (busy.current || !current(revision.current)) return;
        const requestRevision = revision.current;
        busy.current = true;
        setCreating(true);
        try {
            if (!idempotencyKey.current) idempotencyKey.current = createClientUUID();
            const result = await createPaymentOrder({ productId, providerId, idempotencyKey: idempotencyKey.current });
            if (!current(requestRevision)) return;
            idempotencyKey.current = "";
            await accept(result.order, requestRevision);
            if (current(requestRevision)) redirect(result.order);
        } catch (error) {
            if (current(requestRevision)) message.error(error instanceof Error ? error.message : "创建支付订单失败");
        } finally {
            if (current(requestRevision)) {
                busy.current = false;
                setCreating(false);
            }
        }
    };

    const showOrder = async (orderId: string) => {
        const requestRevision = ++revision.current;
        busy.current = true;
        setOrder(null);
        setCreating(false);
        setQuerying(true);
        try {
            const result = await getPaymentOrder(orderId);
            await accept(result.order, requestRevision);
        } catch (error) {
            if (current(requestRevision)) message.error(error instanceof Error ? error.message : "读取支付结果失败");
        } finally {
            if (current(requestRevision)) {
                busy.current = false;
                setQuerying(false);
            }
        }
    };

    const update = async (operation: "query" | "close" | "retry" | "poll") => {
        if (!order || busy.current || !current(revision.current)) return;
        const requestRevision = revision.current;
        busy.current = true;
        if (operation !== "poll") setQuerying(true);
        try {
            const action = operation === "close" ? closePaymentOrder : operation === "retry" ? refreshPaymentCheckout : operation === "poll" ? getPaymentOrder : queryPaymentOrder;
            const result = await action(order.id);
            await accept(result.order, requestRevision);
            if (!current(requestRevision)) return;
            if (operation === "retry") redirect(result.order);
            if (operation === "poll" || result.order.status === "credited") return;
            if (result.order.status === "closed") message.info(result.order.closurePending ? "订单已在站内关闭，渠道结果待确认；若已支付，将按原订单到账" : "渠道已确认关闭订单");
            else if (operation === "query") message.info("支付渠道尚未确认，请稍后再试");
        } catch (error) {
            if (current(requestRevision) && operation !== "poll") message.error(error instanceof Error ? error.message : "更新支付订单失败");
        } finally {
            if (current(requestRevision)) {
                busy.current = false;
                setQuerying(false);
            }
        }
    };
    const updateRef = useRef(update);
    updateRef.current = update;

    useEffect(() => {
        if (!order || !["created", "pending", "closing"].includes(order.status)) return;
        const timer = window.setInterval(() => void updateRef.current("poll"), 2_000);
        return () => window.clearInterval(timer);
    }, [order?.id, order?.status]);

    useEffect(() => {
        if (!order) return;
        setClock(Date.now());
        const timer = window.setInterval(() => setClock(Date.now()), 1_000);
        return () => window.clearInterval(timer);
    }, [order?.id]);

    const close = () => {
        revision.current++;
        busy.current = false;
        setOrder(null);
        setCreating(false);
        setQuerying(false);
    };

    return { order, creating, querying, clock, start, showOrder, resetIdempotency, close, refresh: () => update("query"), cancel: () => update("close"), retry: () => update("retry") };
}
