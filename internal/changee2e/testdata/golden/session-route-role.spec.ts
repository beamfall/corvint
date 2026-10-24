// corvint-test e2e compiled scenario: generated, do not edit.
// catalog_digest: <pinned>
// base: <pinned>
// head: <pinned>
// obligation: obligation:rebook-slot/rebooked
// compiler: corvint-test-e2e-compile/1
// digest: <pinned>
import { test as base, expect } from '@playwright/test';
import { ClubPage } from '../pages/club.page';
import { TeeSheetPage } from '../pages/teesheet.page';

const test = base.extend<{ po: Record<string, any>; fx1: Record<string, any> }>({
  po: async ({ page }, use) => {
    await use({ clubPage: new ClubPage(page), teeSheetPage: new TeeSheetPage(page) });
  },
  fx1: async ({ po }, use) => {
    const { clubPage } = po;
    const out = await clubPage.seedOpenSlots();
    try {
      for (const k of ['clubId']) if (out?.[k] === undefined) throw new Error('corvint-e2e fixture-output-missing ' + k);
      await use(out);
    } finally {
      await clubPage.cleanup();
    }
  },
});
test.use({ trace: 'retain-on-failure' });
test.use({ storageState: process.env.CLUB_ADMIN_STATE });
test.skip(!process.env.CLUB_ADMIN_STATE, 'corvint-e2e session-unset CLUB_ADMIN_STATE');

test('obligation:rebook-slot/rebooked', async ({ page, po, fx1 }) => {
  const { teeSheetPage } = po;
  await test.step('path:entry:screen:admin:app.clubs.teesheets', () => page.goto('/#!/clubs/' + encodeURIComponent(String(fx1.clubId)) + '/teesheets'));
  await test.step('method:e2e/pages/teesheet.page.ts#selectSlot', () => teeSheetPage.selectSlot());
  await test.step('method:e2e/pages/teesheet.page.ts#book', () => teeSheetPage.book());
  await expect(page.getByRole('status', { name: 'Slot status', exact: true }), 'corvint-e2e outcome assert:rebook-slot/rebooked').toHaveText('Booked');
});
