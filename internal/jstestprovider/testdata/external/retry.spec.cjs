const {test, expect} = require('@playwright/test');

test('retry state', async ({page}, testInfo) => {
  await page.goto(process.env.CORVINT_FIXTURE_URL);
  expect(testInfo.retry).toBe(1);
});
