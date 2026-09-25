import { describe, expect, test } from "bun:test";

import { creditProductActiveSegments } from "../src/components/payments/credit-product-card";

describe("credit product meter", () => {
    test("scales each product against the largest available package", () => {
        expect(creditProductActiveSegments(100, 100)).toBe(28);
        expect(creditProductActiveSegments(10, 100)).toBe(3);
        expect(creditProductActiveSegments(10, 1_000)).toBe(1);
    });

    test("keeps empty values unfilled and caps the meter at its segment count", () => {
        expect(creditProductActiveSegments(0, 100)).toBe(0);
        expect(creditProductActiveSegments(100, 0)).toBe(0);
        expect(creditProductActiveSegments(120, 100)).toBe(28);
    });
});
