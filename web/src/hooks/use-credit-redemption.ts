import { App } from "antd";
import { useEffect, useRef, useState } from "react";

import { redeemCredits } from "@/services/api/wallet";
import { useUserStore } from "@/stores/use-user-store";

export function useCreditRedemption(userId: string) {
    const { message } = App.useApp();
    const [code, setCode] = useState("");
    const [redeeming, setRedeeming] = useState(false);
    const pending = useRef(false);
    const alive = useRef(true);
    const normalizedCode = code.trim().toLowerCase();
    const current = () => alive.current && useUserStore.getState().user?.id === userId && useUserStore.getState().features.creditsEnabled;
    useEffect(() => {
        alive.current = true;
        return () => {
            alive.current = false;
        };
    }, []);

    const redeem = async () => {
        if (pending.current || !current()) return;
        if (normalizedCode.length !== 32) {
            message.error("请输入完整的 32 位兑换码");
            return;
        }
        pending.current = true;
        setRedeeming(true);
        try {
            await redeemCredits(normalizedCode);
            if (!current()) return;
            setCode("");
            window.dispatchEvent(new CustomEvent("wallet:updated"));
            message.success("兑换成功，积分已到账");
        } catch (error) {
            if (current()) message.error(error instanceof Error ? error.message : "兑换失败");
        } finally {
            pending.current = false;
            if (current()) setRedeeming(false);
        }
    };
    return { code, setCode, normalizedCode, redeeming, redeem };
}
