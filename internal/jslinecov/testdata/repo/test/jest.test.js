const { add } = require("../lib.js");
const { square } = require("../out/calc.js");

describe("math", () => {
  it("adds", () => {
    expect(add(2, 3)).toBe(5);
  });
  it("squares", () => {
    expect(square(3)).toBe(9);
  });
});
