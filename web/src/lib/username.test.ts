import assert from "node:assert/strict";
import test from "node:test";

import { normalizeUsername, usernameValidationMessage } from "./username";

test("normalizeUsername applies NFKC, trims and lowercases", () => {
    assert.equal(normalizeUsername("　ＡＢＣ１２　"), "abc12");
});

test("usernameValidationMessage accepts supported short usernames", () => {
    for (const value of ["小序君", "一二三四五六七八九", "abc", "a12", "小a1", "AbC123456", "ＡＢＣ"]) {
        assert.equal(usernameValidationMessage(value), "", value);
    }
});

test("usernameValidationMessage rejects unsupported usernames", () => {
    for (const value of ["小", "小序", "ab", "abcdefghij", "123456", "abc_1", "abc-1", "abc!", "小序😀", "ADMIN", "管理员"]) {
        assert.notEqual(usernameValidationMessage(value), "", value);
    }
});
