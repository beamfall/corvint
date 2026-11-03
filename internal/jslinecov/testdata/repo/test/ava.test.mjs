import test from "ava";
import lib from "../lib.js";
import calc from "../out/calc.js";

test("adds", (t) => {
  t.is(lib.add(2, 3), 5);
});

test("squares", (t) => {
  t.is(calc.square(3), 9);
});
