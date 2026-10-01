import assert from "node:assert/strict";
import test from "node:test";

import { normalizeUsername, usernameValidationMessage } from "./username";

test("normalizeUsername applies NFKC, trims and lowercases", () => {
    assert.equal(normalizeUsername("　ＡＢＣ１２　"), "abc12");
});

test("usernameValidationMessage accepts supported short usernames", () => {
    for (const value of ["小序", "一二三四五六", "abc", "a12", "小a", "AbC12", "ＡＢＣ"]) {
        assert.equal(usernameValidationMessage(value), "", value);
    }
});

test("usernameValidationMessage rejects unsupported usernames", () => {
    for (const value of ["小", "ab", "abcdefg", "123456", "abc_1", "abc-1", "abc!", "小序😀", "ADMIN", "管理员"]) {
        assert.notEqual(usernameValidationMessage(value), "", value);
    }
});
