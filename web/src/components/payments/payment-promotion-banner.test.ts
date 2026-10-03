import assert from "node:assert/strict";
import test from "node:test";

import { promotionClockOffset, promotionCountdown } from "./payment-promotion-banner";

test("promotionCountdown formats days through seconds", () => {
    const now = Date.parse("2026-10-02T00:00:00+08:00");
    const result = promotionCountdown("2026-10-05T01:02:03+08:00", 0, now);
    assert.deepEqual(result, { days: 3, hours: 1, minutes: 2, seconds: 3, totalMs: 262_923_000 });
});

test("promotionCountdown applies server clock offset and clamps at zero", () => {
    const receivedAt = Date.parse("2026-10-01T15:59:50Z");
    const offset = promotionClockOffset("2026-10-01T16:00:00Z", receivedAt);
    assert.equal(offset, 10_000);
    assert.equal(promotionCountdown("2026-10-01T16:00:09Z", offset, receivedAt).totalMs, 9_000);
    assert.equal(promotionCountdown("2026-10-01T15:59:59Z", offset, receivedAt).totalMs, 0);
});
