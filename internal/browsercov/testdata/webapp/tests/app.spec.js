const { test, expect } = require("@playwright/test");

test("adds", async ({ page }) => {
  await page.goto("/");
  await page.click("#add");
  await expect(page.locator("#out")).toHaveText("5");
});

test("signs", async ({ page }) => {
  await page.goto("/");
  await page.click("#sign");
  await expect(page.locator("#out")).toHaveText("positive");
});
