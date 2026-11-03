const { test, expect } = require("bun:test");
const { add } = require("../lib.js");
const { square } = require("../out/calc.js");

test("adds", () => {
  expect(add(2, 3)).toBe(5);
});

test("squares", () => {
  expect(square(3)).toBe(9);
});
