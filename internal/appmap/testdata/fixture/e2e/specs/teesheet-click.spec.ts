import { test, expect } from '@playwright/test';
import { HomePage } from '../pages/home.page';
import { TeeSheetPage } from '../pages/teesheet.page';

test('books a slot reached by clicking', async ({ page }) => {
  const home = new HomePage(page);
  await home.goToTeeSheets();
  const sheet = new TeeSheetPage(page);
  await sheet.selectSlot();
  await sheet.book();
  await expect(await sheet.status()).toHaveText('Booked');
});
