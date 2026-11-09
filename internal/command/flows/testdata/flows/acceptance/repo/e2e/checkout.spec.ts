import { expect, test } from "@playwright/test";

test("pays", async ({ page }) => {
  await page.goto("/checkout");
  await page.getByRole("button", { name: "Pay" }).click();
  await expect(page.getByTestId("order-status")).toHaveText("paid");
});
