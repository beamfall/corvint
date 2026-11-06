import { expect, test } from "@playwright/test";

test("creates", async ({ request }) => {
  const response = await request.post("/api/orders", { data: { items: ["book"] } });
  expect(response.status()).toBe(201);
});
