'use strict';
const path = require('node:path');
const { test, expect } = require('./fixtures.cjs');

const app = 'http://127.0.0.1:4880';
const har = path.join(__dirname, '..', 'items.har');

test('browser.newContext stubs are recorded', async ({ browser }) => {
  const context = await browser.newContext();
  await context.route('**/api/items', route => route.fulfill({ json: { items: [1, 2] } }));
  const page = await context.newPage();
  await page.route('**/api/never', route => route.fulfill({ json: { items: [] } }));
  await page.goto(`${app}/`);
  await expect(page.locator('#count')).toHaveText('2');
  await context.close();
});

test('browser.newPage stubs are recorded', async ({ browser }) => {
  const page = await browser.newPage();
  await page.route('**/api/itmes', route => route.fulfill({ json: { items: [] } }));
  await page.goto(`${app}/`);
  await expect(page.locator('#count')).toHaveText('3');
  await page.close();
});

test('routeWebSocket stubs are recorded', async ({ page }) => {
  await page.routeWebSocket('**/live', ws => { ws.send('mocked'); });
  await page.routeWebSocket('**/never-socket', () => {});
  await page.goto('/ws');
  await expect(page.locator('#message')).toHaveText('mocked');
});

test('routeFromHAR stubs are recorded', async ({ page, context }) => {
  await page.routeFromHAR(har, { url: '**/api/items' });
  await context.routeFromHAR(har, { url: '**/api/never', notFound: 'fallback' });
  await page.goto('/');
  await expect(page.locator('#count')).toHaveText('9');
});

test('no browser fixture', async () => {
  expect(1 + 1).toBe(2);
});
