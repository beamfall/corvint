import { test, expect } from '@playwright/test';
import { bookFirstSlot } from '../workflows/booking';
import { clubWithSlots } from '../scenarios/club-with-slots';

test('books through the workflow', async ({ page }) => {
  await bookFirstSlot(page);
  expect(clubWithSlots.slots).toBeGreaterThan(0);
});
