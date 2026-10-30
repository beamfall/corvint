'use strict';
const { test } = require('./fixtures.cjs');

test.describe('items', () => {
  test.use({ behaviorFixtures: ['seeded-items'] });

  test('lists items', async ({ page, behavior }) => {
    behavior.cleanup(async () => {});
    await page.goto('/items');
    await behavior.expect({ id: 'items-visible', behavior: 'list-items', criterion: 'items-visible' })(page.locator('#count')).toHaveText('2');
    await behavior.negativeControl('empty-hidden')(page.locator('#empty')).toBeHidden();
    await page.locator('#done').click();
    await page.waitForURL('**/done');
  });
});

test('walks a redirect and a popup', async ({ page, behavior }) => {
  await page.goto('/old');
  await page.goto('/opener');
  const [popup] = await Promise.all([page.waitForEvent('popup'), page.locator('#open').click()]);
  await popup.waitForLoadState();
  await behavior.expect({ id: 'popup-text', behavior: 'popup', criterion: 'popup-text' })(popup.locator('p')).toHaveText('popup');
});

test('records no event for inexact or frame assertions', async ({ page, behavior }) => {
  await page.goto('/items');
  const meta = { id: 'count', behavior: 'list-items', criterion: 'count' };
  await behavior.expect(meta)(page.locator('#count')).toHaveText(/2/);
  await behavior.expect(meta)(page.frameLocator('iframe').locator('#inner')).toHaveText('2');
  await behavior.expect(meta)(page.locator('#count')).toHaveText('2', { useInnerText: true });
});
