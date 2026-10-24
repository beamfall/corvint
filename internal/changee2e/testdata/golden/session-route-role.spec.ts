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

const test = base.extend<{ fx1: Record<string, any> }>({
  fx1: async ({ page }, use) => {
    const clubPage = new ClubPage(page);
    const out = await clubPage.seedOpenSlots();
    for (const k of ['clubId']) if (out?.[k] === undefined) throw new Error('corvint-e2e fixture-output-missing ' + k);
    await use(out);
    await clubPage.cleanup();
  },
});
test.use({ trace: 'retain-on-failure' });
test.use({ storageState: process.env.CLUB_ADMIN_STATE });
test.skip(!process.env.CLUB_ADMIN_STATE, 'corvint-e2e session-unset CLUB_ADMIN_STATE');

test('obligation:rebook-slot/rebooked', async ({ page, fx1 }) => {
  const teeSheetPage = new TeeSheetPage(page);
  await test.step('path:entry:screen:admin:app.clubs.teesheets', () => page.goto('/#!/clubs/' + encodeURIComponent(String(fx1.clubId)) + '/teesheets'));
  await test.step('method:e2e/pages/teesheet.page.ts#selectSlot', () => teeSheetPage.selectSlot());
  await test.step('method:e2e/pages/teesheet.page.ts#book', () => teeSheetPage.book());
  await expect(page.getByRole('status', { name: 'Slot status', exact: true }), 'corvint-e2e outcome assert:rebook-slot/rebooked').toHaveText('Booked');
});
