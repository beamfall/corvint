import { test, expect } from '@playwright/test';
import { OrdersPage } from '../pages/orders.page';

test('orders', async ({ page }) => {
  const orderId = 7;
  const rowId = `order-row-${orderId}`;
  await page.route('**/api/v2/orders*', (route) => route.fulfill({ json: [] }));
  await page.route('**/api/v2/invoices', (route) => route.fulfill({ json: [] }));
  await page.route(/\/api\/v2\/refunds/, (route) => route.abort());
  await page.route('https://api.example.test/api/v2/status/*', (route) => route.fulfill({ json: {} }));
  await page.goto('/home');
  await page.goto(`/orders/${orderId}`);
  await page.goto('/reports');
  await page.goto('/files/a/b');
  await expect(page).toHaveURL(/\/orders\/\d+/);
  await page.waitForURL('**/orders');
  await new OrdersPage(page).save();
  await new OrdersPage(page).cancel();
  await page.getByTestId('order-row-7').click();
  await page.getByTestId(rowId).click();
  await page.locator('#order-list .orders-panel [data-region="orders"]').click();
  await page.locator('.btn.secondary').click();
  await page.getByRole('button', { name: 'Save' }).click();
  await page.locator('xpath=//div').click();
});
