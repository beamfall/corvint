const test = require("node:test");
const assert = require("node:assert");
const { add } = require("../lib.js");
const { square } = require("../out/calc.js");

test("adds", () => {
  assert.strictEqual(add(2, 3), 5);
});

test("squares", () => {
  assert.strictEqual(square(3), 9);
});
