import { square } from "../src/calc.ts";
import { assertEquals } from "./assert.ts";

Deno.test("squares", () => {
  assertEquals(square(3), 9);
});

Deno.test("halves", () => {
  assertEquals(4 / 2, 2);
});
