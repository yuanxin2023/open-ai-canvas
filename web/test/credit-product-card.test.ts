import { describe, expect, test } from "bun:test";

import { creditProductActiveSegments, normalizeProductAccentColor, productAccentForeground } from "../src/components/payments/credit-product-card";

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

describe("credit product card accent", () => {
    test("normalizes configured colors and rejects unsafe CSS values", () => {
        expect(normalizeProductAccentColor(" #d8ff4f ")).toBe("#D8FF4F");
        expect(normalizeProductAccentColor("red; color: transparent")).toBe("#D8FF4F");
        expect(normalizeProductAccentColor()).toBe("#D8FF4F");
    });

    test("chooses readable text for light and dark accents", () => {
        expect(productAccentForeground("#D8FF4F")).toBe("#111111");
        expect(productAccentForeground("#1B2A54")).toBe("#FFFFFF");
    });
});
