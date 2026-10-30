import { test, expect } from '@playwright/test';
import { ShopPage } from '../pages/shop.page';

test('buys a gift card', async ({ page }) => {
  const shop = new ShopPage(page);
  await shop.buyGiftCard();
  await expect(page.getByTestId('receipt')).toBeVisible();
});
