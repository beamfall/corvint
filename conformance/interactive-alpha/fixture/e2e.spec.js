import { expect, test } from "@playwright/test";

test("increments the counter", async ({ page }) => {
  await page.goto("http://127.0.0.1:4173/");
  await page.getByRole("button", { name: "Increment" }).click();
  await expect(page.locator("#count")).toHaveText("1");
});
