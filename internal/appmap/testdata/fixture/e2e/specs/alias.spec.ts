import { test, expect } from '@playwright/test';
import { TeeSheetPage } from '@pages/teesheet.page';

test('reaches the tee sheet through a path alias', async ({ page }) => {
  const sheet = new TeeSheetPage(page);
  await sheet.book();
  await expect(page.getByTestId('slot-status')).toHaveText('Booked');
});
