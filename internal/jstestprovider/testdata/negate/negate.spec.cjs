// Synthetic step-negation matrix fixtures (LPCV-V0-070).
const {test, expect} = require('@playwright/test');

const loaded = async page => {
  await page.goto('/');
  await page.waitForFunction(() => document.querySelector('#total').textContent !== 'loading');
};

test.describe('negate', () => {
  test('network text', async ({page}) => {
    await test.step('open', async () => { await loaded(page); });
    await test.step('total', async () => { await expect(page.locator('#total')).toHaveText('42 apples'); });
  });

  test('static text', async ({page}) => {
    await test.step('open', async () => { await loaded(page); });
    await test.step('static', async () => { await expect(page.locator('#static')).toHaveText('Static Title'); });
  });

  test('caught assertion', async ({page}) => {
    await test.step('open', async () => { await loaded(page); });
    await test.step('caught', async () => {
      try { await expect(page.locator('#total')).toHaveText('42 apples', {timeout: 500}); } catch {}
    });
  });

  test('unproven steps', async ({page}) => {
    await test.step('open', async () => { await loaded(page); });
    await test.step('pattern', async () => { await expect(page.locator('#total')).toHaveText(/\d+ apples/); });
    await test.step('click', async () => { await page.locator('#go').click(); });
  });

  test('soft steps', async ({page}) => {
    await test.step('open', async () => { await loaded(page); });
    await test.step('total', async () => { await expect.soft(page.locator('#total')).toHaveText('42 apples'); });
    await test.step('static', async () => { await expect.soft(page.locator('#static')).toHaveText('Static Title'); });
  });

  test('hard steps', async ({page}) => {
    await test.step('open', async () => { await loaded(page); });
    await test.step('total', async () => { await expect(page.locator('#total')).toHaveText('42 apples'); });
    await test.step('static', async () => { await expect(page.locator('#static')).toHaveText('Static Title'); });
  });

  test('collateral', async ({page}) => {
    await test.step('open', async () => { await loaded(page); });
    await test.step('total', async () => { await expect.soft(page.locator('#total')).toHaveText('42 apples'); });
    await test.step('again', async () => { await expect.soft(page.locator('#audit')).toHaveText('audit ok'); });
  });

  test('flaky', async ({page}) => {
    await page.goto('/');
    await test.step('flaky', async () => { await expect(page.locator('#flaky')).toHaveText('steady'); });
  });

  test('third party', async ({page}) => {
    await test.step('open', async () => { await loaded(page); });
    await test.step('partner', async () => { await expect(page.locator('#partner')).toHaveText('partner data'); });
  });

  test('slow', async ({page}) => {
    await test.step('open', async () => { await page.goto('/'); });
    await test.step('slow', async () => { await page.evaluate(() => fetch('/api/slow')); await expect(page.locator('#total')).toHaveText('42 apples'); });
  });
});

const broken = test.extend({
  broken: async ({}, use) => { throw new Error('fixture setup failed'); },
});

broken('failing fixture', async ({page, broken}) => {
  await test.step('open', async () => { await page.goto('/'); });
});
