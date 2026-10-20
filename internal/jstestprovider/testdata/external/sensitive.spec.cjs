const { test, expect } = require('@playwright/test');

// PWP-V0-035: a real fill step whose typed value the /2 reporter must redact.
test('redacted fill', async ({page}) => {
  await page.goto(process.env.CORVINT_FIXTURE_URL);
  await page.setContent('<input id="entry">');
  await page.locator('#entry').fill('corvint-fixture-typed-value');
  expect(await page.evaluate(() => document.querySelector('#entry').value.length)).toBeGreaterThan(0);
});
