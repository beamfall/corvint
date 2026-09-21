const {test, expect} = require('@playwright/test');

test.describe.configure({mode: 'parallel'});

test('failure interrupts peer', async ({page}) => {
  await page.goto(process.env.CORVINT_FIXTURE_URL + '/interrupt-wait');
  expect('actual').toBe('expected');
});

test('interrupted peer', async ({page}) => {
  await page.goto(process.env.CORVINT_FIXTURE_URL + '/interrupt-ready');
  await new Promise(() => {});
});
