import { test, expect } from '@playwright/test';
import { clubWithSlots } from '../scenarios/club-with-slots';

test('signs in', async ({ page }) => {
  await page.goto('/#!/login');
  await page.goto(`/#!/clubs/${clubWithSlots.id}/settings`);
  await expect(page.getByText('Settings')).toBeVisible();
});
