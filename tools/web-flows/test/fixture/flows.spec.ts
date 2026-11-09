import { test, expect } from '@playwright/test';

test('create and reload', async ({ page }) => {
  await page.goto('/editor');
  await page.locator('#name').fill('sample');
  await page.locator('#save').click();
  await page.reload();
  await expect(page.locator('#items')).toHaveText('sample');
});

test('save without checking its outcome', async ({ page }) => {
  await page.goto('/editor');
  await page.locator('#name').fill('draft');
  await page.locator('#save').click();
});
