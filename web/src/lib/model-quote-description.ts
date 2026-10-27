import type { LogicalModelQuote } from "@/services/api/logical-models";

export function modelQuoteDescription(quote: LogicalModelQuote) {
    const amount = (quote.amountMicrocredits / 1_000_000).toLocaleString("zh-CN", { maximumFractionDigits: 6 });
    const descriptions = [`${quote.estimated || quote.settlementBasis === "measured_duration" ? "预计" : "本次"}消耗 ${amount} 积分`];
    const estimate = quote.videoTokenEstimate;
    if (estimate) {
        descriptions.push(`公式预估 ${estimate.formulaTokens.toLocaleString("zh-CN")} 视频 Token，平台额外预留 ${estimate.reservationMarginPercent}%（共 ${estimate.reservedTokens.toLocaleString("zh-CN")} Token）`);
        if (estimate.referenceDurationEstimated) descriptions.push("未知的参考视频时长按 15 秒估算");
        if (estimate.dimensionsEstimated) descriptions.push("实际画幅尚未确定，尺寸按当前参数估算");
    }
    if (quote.settlementBasis === "measured_duration") {
        const limit = quote.chargeLimitMicrocredits;
        const limitText = typeof limit === "number" && Number.isFinite(limit) && limit >= 0 ? `授权上限 ${(limit / 1_000_000).toLocaleString("zh-CN", { maximumFractionDigits: 6 })} 积分` : "授权上限以订单记录为准";
        descriptions.push(`按实际输出时长向上取整到秒结算，短于预估退回差额；${limitText}，超出上限或无法确认时长时转待核对，不自动超额补扣`);
    }
    if (quote.billingMode === "token")
        descriptions.push(estimate ? "预估金额并非费用上限，成功任务优先按有效的实际用量结算；未返回用量时按火山引擎公式结算，额外预留不计入公式结算，可能补扣或退回差额" : "预估金额并非费用上限，最终以成功任务返回的实际用量结算，可能补扣或退回差额");
    return descriptions.join("；");
}
