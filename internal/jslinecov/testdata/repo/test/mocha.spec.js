const assert = require("node:assert");
const { add } = require("../lib.js");
const { square } = require("../out/calc.js");

describe("math", () => {
  it("adds", () => {
    assert.strictEqual(add(2, 3), 5);
  });
  it("squares", () => {
    assert.strictEqual(square(3), 9);
  });
});
