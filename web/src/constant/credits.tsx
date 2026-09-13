import type { ComponentProps } from "react";
import { Coins } from "lucide-react";

export { requestCreditCost } from "@/lib/model-pricing";

export function CreditSymbol({ className, ...props }: ComponentProps<"span">) {
    return (
        <span {...props} className={`inline-flex items-center justify-center ${className || ""}`}>
            <Coins className="size-[1em]" strokeWidth={2.2} />
        </span>
    );
}

export const CREDIT_FRACTION_DIGITS = 2;
export const MICRO_CREDITS_PER_CREDIT = 1_000_000;

export function formatCredits(value: number, maximumFractionDigits = CREDIT_FRACTION_DIGITS) {
    return (value / 1_000_000).toLocaleString("zh-CN", { maximumFractionDigits });
}
