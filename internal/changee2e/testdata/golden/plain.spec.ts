// corvint-test e2e compiled scenario: generated, do not edit.
// catalog_digest: <pinned>
// base: <pinned>
// head: <pinned>
// obligation: obligation:book-tee-time/booked
// compiler: corvint-test-e2e-compile/1
// digest: <pinned>
import { test as base, expect } from '@playwright/test';
import { ClubPage } from '../pages/club.page';
import { HomePage } from '../pages/home.page';
import { TeeSheetPage } from '../pages/teesheet.page';

const test = base.extend<{ po: Record<string, any>; fx1: Record<string, any> }>({
  po: async ({ page }, use) => {
    await use({ clubPage: new ClubPage(page), homePage: new HomePage(page), teeSheetPage: new TeeSheetPage(page) });
  },
  fx1: async ({ po }, use) => {
    const { clubPage } = po;
    await clubPage.seedOpenSlots();
    await use({});
    await clubPage.cleanup();
  },
});
test.use({ trace: 'retain-on-failure' });

test('obligation:book-tee-time/booked', async ({ page, po, fx1 }) => {
  const { homePage, teeSheetPage } = po;
  await test.step('path:entry:screen:admin:app.home', () => page.goto('/#!/home'));
  await test.step('method:e2e/pages/home.page.ts#goToTeeSheets', () => homePage.goToTeeSheets());
  await test.step('method:e2e/pages/teesheet.page.ts#selectSlot', () => teeSheetPage.selectSlot());
  await test.step('method:e2e/pages/teesheet.page.ts#book', () => teeSheetPage.book());
  await expect(page.getByTestId('slot-status'), 'corvint-e2e outcome assert:book-tee-time/booked').toHaveText('Booked');
});
