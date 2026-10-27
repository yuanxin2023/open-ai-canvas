import { describe, expect, test } from "bun:test";

import { modelQuoteDescription } from "../src/lib/model-quote-description";
import { quoteModel, type LogicalModelQuote } from "../src/services/api/logical-models";
import { apiClient } from "../src/services/api/request";

const quote: LogicalModelQuote = { logicalModelId: "model", billingMode: "per_second", quantity: 5, amountMicrocredits: 50_000, estimated: false };

describe("measured media quote contract", () => {
    test("explains measured seconds, refunds, original ceiling and review", () => {
        const text = modelQuoteDescription({ ...quote, settlementBasis: "measured_duration", chargeLimitMicrocredits: 50_000 });
        for (const value of ["预计消耗 0.05 积分", "实际输出时长", "向上取整到秒", "退回差额", "授权上限 0.05 积分", "转待核对", "不自动超额补扣"]) expect(text).toContain(value);
    });

    test("preserves zero authorization", () => {
        const text = modelQuoteDescription({ ...quote, amountMicrocredits: 0, settlementBasis: "measured_duration", chargeLimitMicrocredits: 0 });
        expect(text).toContain("授权上限 0 积分");
    });

    test("ordinary video and fixed audio keep snapshot description", () => {
        expect(modelQuoteDescription(quote)).toBe("本次消耗 0.05 积分");
        expect(modelQuoteDescription({ ...quote, billingMode: "fixed_request" })).toBe("本次消耗 0.05 积分");
    });

    test("token billing keeps its distinct usage contract", () => {
        const text = modelQuoteDescription({ ...quote, billingMode: "token", estimated: true });
        expect(text).toContain("实际用量结算");
        expect(text).not.toContain("实际输出时长");
    });

    test("invalid or absent ceiling is not displayed as a fabricated number", () => {
        for (const limit of [undefined, Number.NaN, -1]) {
            const text = modelQuoteDescription({ ...quote, settlementBasis: "measured_duration", chargeLimitMicrocredits: limit });
            expect(text).toContain("授权上限以订单记录为准");
            expect(text).not.toContain("NaN");
            expect(text).not.toContain("undefined");
        }
    });

    test("audio quote API preserves measured policy and zero authorization", async () => {
        const original = apiClient.defaults.adapter;
        const response: LogicalModelQuote = { ...quote, estimated: true, amountMicrocredits: 0, settlementBasis: "measured_duration", chargeLimitMicrocredits: 0 };
        let request: unknown;
        apiClient.defaults.adapter = async (config) => {
            request = { url: config.url, method: config.method, data: JSON.parse(config.data) };
            return { config, headers: {}, status: 200, statusText: "OK", data: { code: 0, data: { quote: response }, msg: "ok" } };
        };
        try {
            const intent = { capability: "audio" as const, inputs: { audio: 0 }, options: {} };
            expect(await quoteModel({ channelId: "system", modelKey: "audio", intent })).toEqual({ quote: response });
            expect(request).toMatchObject({ url: "/model-catalog/quote", method: "post", data: { channelId: "system", modelKey: "audio", intent } });
        } finally {
            apiClient.defaults.adapter = original;
        }
    });
});
