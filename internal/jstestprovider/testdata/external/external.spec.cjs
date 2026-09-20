const {test, expect} = require('@playwright/test');

test('passing page', async ({page}) => {
  await page.goto(process.env.CORVINT_FIXTURE_URL);
  await expect(page.locator('body')).toHaveText('external fixture');
});
test('assertion failure', async ({page}) => {
  await page.goto(process.env.CORVINT_FIXTURE_URL);
  expect(await page.locator('body').innerText()).toBe('wrong text');
});
test('test timeout', async ({page}) => {
  await page.goto(process.env.CORVINT_FIXTURE_URL);
  await new Promise(() => {});
});
test('cancellation', async ({page}) => {
  test.setTimeout(60000);
  await page.goto(process.env.CORVINT_FIXTURE_URL + '/cancel-ready');
  await new Promise(() => {});
});
