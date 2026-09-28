const {test, expect} = require('@playwright/test');

test.beforeEach(async ({}, info) => {
  if (info.retry === 0) throw new Error('deliberate fixture failure');
});

test('infrastructure then assertion', async ({page}) => {
  await page.goto(process.env.CORVINT_FIXTURE_URL);
  expect('actual', 'named assertion fails after fixture recovery').toBe('expected');
});
