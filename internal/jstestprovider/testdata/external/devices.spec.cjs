const { test, expect } = require('@playwright/test');

test('standard desktop chrome device', async ({page}) => {
  const response = await page.goto(process.env.CORVINT_FIXTURE_URL);
  expect(response.ok()).toBeTruthy();
});
