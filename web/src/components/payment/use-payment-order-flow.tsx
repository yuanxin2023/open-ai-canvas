import { translate } from "@/i18n";
import { App, Button } from "antd";
import { CreditCard } from "lucide-react";
import { useEffect, useRef, useState } from "react";

import { PaymentCheckoutCode } from "@/components/payment-checkout-code";
import { AppModal } from "@/components/ui/product/app-modal";
import { useUserStore } from "@/stores/use-user-store";
import { createClientUUID } from "@/lib/client-id";
import { paymentOrderGrantLabel } from "@/lib/payment-order-grant";
import { rememberPaymentReturnPath } from "@/lib/workspace-wallet";
import { closePaymentOrder, createPaymentOrder, getPaymentOrder, queryPaymentOrder, refreshPaymentCheckout, type PaymentOrder } from "@/services/api/payments";

function grantedText() {
    return "支付已确认，积分已经到账";
}

function notGrantedText(order: PaymentOrder) {
    if (order.closurePending) return "订单已在站内关闭，渠道结果待确认；若已支付，将按原订单发放权益";
    return "订单已关闭，未产生积分充值";
}

/**
 * 在线支付下单、扫码、轮询、关单与重建收银台的共用流程。
 * onGranted 对每个订单只触发一次，调用方在其中刷新积分余额。
 */
export function usePaymentOrderFlow({ onGranted }: { onGranted: (order: PaymentOrder) => Promise<void> | void }) {
    const { message } = App.useApp();
    const actorId = useUserStore((state) => state.user?.id);
    const alive = useRef(true);
    useEffect(() => {
        alive.current = true;
        return () => {
            alive.current = false;
        };
    }, []);
    const current = () => alive.current && Boolean(actorId) && useUserStore.getState().user?.id === actorId;
    const [order, setOrder] = useState<PaymentOrder | null>(null);
    const [open, setOpen] = useState(false);
    const [creating, setCreating] = useState(false);
    const [querying, setQuerying] = useState(false);
    const [clock, setClock] = useState(Date.now());
    const idempotencyKey = useRef("");
    const grantedOrderId = useRef("");
    const onGrantedRef = useRef(onGranted);
    onGrantedRef.current = onGranted;

    const settle = async (next: PaymentOrder) => {
        if (!current() || next.status !== "credited" || grantedOrderId.current === next.id) return;
        grantedOrderId.current = next.id;
        await onGrantedRef.current(next);
    };

    useEffect(() => {
        if (!open || !order || !["created", "pending", "closing"].includes(order.status)) return;
        const interval = window.setInterval(() => {
            void refresh(order.id, true);
        }, 4_000);
        return () => window.clearInterval(interval);
    }, [open, order?.id, order?.status]);

    useEffect(() => {
        if (!open) return;
        const interval = window.setInterval(() => setClock(Date.now()), 1_000);
        return () => window.clearInterval(interval);
    }, [open]);

    /** 切换商品或支付方式后必须换新的幂等键，否则后端会拒绝“同键不同单”。 */
    const resetIdempotency = () => {
        idempotencyKey.current = "";
    };

    const start = async (productId: string, providerId: string) => {
        if (!current()) return;
        setCreating(true);
        try {
            if (!idempotencyKey.current) idempotencyKey.current = createClientUUID();
            const result = await createPaymentOrder({ productId, providerId, idempotencyKey: idempotencyKey.current });
            if (!current()) return;
            idempotencyKey.current = "";
            setOrder(result.order);
            await settle(result.order);
            if (result.order.checkout.mode === "redirect" && result.order.checkout.url) {
                rememberPaymentReturnPath();
                window.location.assign(result.order.checkout.url);
                return;
            }
            setOpen(true);
        } catch (error) {
            if (current()) message.error(error instanceof Error ? error.message : "创建支付订单失败");
        } finally {
            if (current()) setCreating(false);
        }
    };

    /** 支付跳转返回后展示指定订单的结果。 */
    const showOrder = async (orderId: string) => {
        try {
            const result = await getPaymentOrder(orderId);
            if (!current()) return;
            setOrder(result.order);
            setOpen(true);
            await settle(result.order);
        } catch (error) {
            if (current()) message.error(error instanceof Error ? error.message : "读取支付结果失败");
        }
    };

    async function refresh(orderId = order?.id, silent = false) {
        if (!orderId || querying) return;
        setQuerying(true);
        try {
            const result = await queryPaymentOrder(orderId);
            if (!current()) return;
            setOrder(result.order);
            if (result.order.status === "credited") {
                await settle(result.order);
                if (!silent) message.success(grantedText());
            } else if (!silent && result.order.status === "closed") message.warning(notGrantedText(result.order));
            else if (!silent) message.info(translate("account.payment.notConfirmed"));
        } catch (error) {
            if (!silent) message.error(error instanceof Error ? error.message : "查询支付结果失败");
        } finally {
            if (current()) setQuerying(false);
        }
    }

    const cancel = async () => {
        if (!order) return;
        setQuerying(true);
        try {
            const result = await closePaymentOrder(order.id);
            if (!current()) return;
            setOrder(result.order);
            if (result.order.status === "credited") await settle(result.order);
            else message.success(translate("account.payment.orderClosed"));
        } catch (error) {
            if (current()) message.error(error instanceof Error ? error.message : "关闭订单失败");
        } finally {
            if (current()) setQuerying(false);
        }
    };

    const retry = async () => {
        if (!order) return;
        setQuerying(true);
        try {
            const result = await refreshPaymentCheckout(order.id);
            if (!current()) return;
            setOrder(result.order);
            if (result.order.checkout.mode === "redirect" && result.order.checkout.url) window.location.assign(result.order.checkout.url);
        } catch (error) {
            if (current()) message.error(error instanceof Error ? error.message : "刷新支付入口失败");
        } finally {
            if (current()) setQuerying(false);
        }
    };

    const close = () => setOpen(false);

    const dialog = (
        <AppModal
            open={open}
            title={order?.status === "credited" ? translate("account.payment.topupDone") : order?.checkout.mode === "qr_code" ? translate("account.payment.scanTitle") : translate("account.payment.confirmTitle")}
            centered
            width={430}
            onCancel={close}
            footer={paymentFooter(order, querying, close, cancel, () => refresh(), retry)}
        >
            {order ? (
                <div className="workspace-wallet-payment">
                    <span className="workspace-wallet-payment-icon">
                        <CreditCard />
                    </span>
                    <strong>¥ {(order.amountFen / 100).toFixed(2)}</strong>
                    <p>
                        {order.productName} · {paymentOrderGrantLabel(order)}
                    </p>
                    {order.status === "pending" && order.checkout.mode === "qr_code" && order.checkout.value ? (
                        <>
                            <PaymentCheckoutCode value={order.checkout.value} />
                            <span>{translate("account.payment.scanHint")}</span>
                        </>
                    ) : null}
                    <PaymentStatus order={order} now={clock} />
                </div>
            ) : null}
        </AppModal>
    );

    return { creating, start, showOrder, resetIdempotency, dialog };
}

function PaymentStatus({ order, now }: { order: PaymentOrder; now: number }) {
    if (order.status === "credited") return <div className="workspace-wallet-payment-status is-success">{grantedText()}</div>;
    if (order.status === "closed") return <div className="workspace-wallet-payment-status">{notGrantedText(order)}</div>;
    if (order.status === "create_failed") return <div className="workspace-wallet-payment-status is-error">{translate("account.payment.createCheckoutFailed")}</div>;
    const remaining = Math.max(0, Math.floor((new Date(order.expiresAt).getTime() - now) / 1000));
    const hours = Math.floor(remaining / 3600);
    const minutes = Math.floor((remaining % 3600) / 60);
    const seconds = remaining % 60;
    return (
        <div className="workspace-wallet-payment-status">
            {translate("local.ui.orderTimeLeft")}
            {String(hours).padStart(2, "0")}:{String(minutes).padStart(2, "0")}:{String(seconds).padStart(2, "0")}
            {translate("local.ui.autoConfirm")}
        </div>
    );
}

function paymentFooter(order: PaymentOrder | null, loading: boolean, close: () => void, cancel: () => Promise<void>, query: () => Promise<void>, retry: () => Promise<void>) {
    if (order?.status === "pending")
        return [
            <Button key="cancel" danger disabled={loading} onClick={() => void cancel()}>
                {translate("account.payment.closeOrder")}
            </Button>,
            <Button key="query" type="primary" loading={loading} onClick={() => void query()}>
                {translate("account.payment.paid")}
            </Button>,
        ];
    if (order?.status === "create_failed")
        return [
            <Button key="close" onClick={close}>
                {translate("account.payment.later")}
            </Button>,
            <Button key="retry" type="primary" loading={loading} onClick={() => void retry()}>
                {translate("account.payment.regenerate")}
            </Button>,
        ];
    return [
        <Button key="done" type="primary" onClick={close}>
            {translate("library.assets.batch.done")}
        </Button>,
    ];
}
