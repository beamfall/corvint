'use strict';
const { test, expect } = require('./fixtures.cjs');

test('stub is hit', async ({ page }) => {
  await page.route('**/api/items', route => route.fulfill({ json: { items: [1, 2] } }));
  await page.goto('/');
  await expect(page.locator('#count')).toHaveText('2');
});

test('stub never matched but the test passes', async ({ page, context }) => {
  await page.route('**/api/itmes', route => route.fulfill({ json: { items: [] } }));
  await context.route(/\/api\/v2\//, route => route.fulfill({ json: { items: [] } }));
  await page.goto('/');
  await expect(page.locator('#count')).not.toHaveText('loading');
});

test('no browser fixture', async () => {
  expect(1 + 1).toBe(2);
});
