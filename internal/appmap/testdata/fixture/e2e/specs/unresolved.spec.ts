import { test } from '@playwright/test';
import { MissingPage } from '../pages/missing.page';

test('imports a page object that does not exist', async ({ page }) => {
  new MissingPage(page);
});
