import { afterEach, expect, test } from "bun:test";

import { ApiError } from "../src/services/api/request";
import { isInsufficientCreditsError, notifyImageCreditsShortfall, IMAGE_CREDITS_SHORTFALL_EVENT } from "../src/lib/image-credit-guidance";

const originalWindow = globalThis.window;

afterEach(() => {
    Object.defineProperty(globalThis, "window", { configurable: true, value: originalWindow });
});

test("only the backend insufficient_credits reason opens image purchase guidance", () => {
    const events = new EventTarget();
    Object.defineProperty(globalThis, "window", { configurable: true, value: events });
    let prompts = 0;
    events.addEventListener(IMAGE_CREDITS_SHORTFALL_EVENT, () => prompts++);

    expect(notifyImageCreditsShortfall(new ApiError("积分不足，请充值后重试", { status: 400, code: 40001, reason: "insufficient_credits" }))).toBe(true);
    expect(notifyImageCreditsShortfall(new ApiError("积分不足，请充值后重试", { status: 400, reason: "invalid_argument" }))).toBe(false);
    expect(notifyImageCreditsShortfall(new Error("积分不足，请充值后重试"))).toBe(false);
    expect(isInsufficientCreditsError(new ApiError("免费模型失败", { reason: "unavailable" }))).toBe(false);
    expect(prompts).toBe(1);
});
