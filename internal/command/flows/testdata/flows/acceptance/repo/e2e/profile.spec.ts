import { expect, test } from "@playwright/test";

test("saves", async ({ page }) => {
  await page.goto("/profile");
  await page.getByTestId("display-name").fill("Ada");
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByTestId("profile-status")).toHaveText("saved");
});
