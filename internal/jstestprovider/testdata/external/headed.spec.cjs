const {test, expect} = require('@playwright/test');

test.use({launchOptions: {headless: false}});
test('headed bundled browser is unqualified', async ({headless, page}) => {
  expect(headless).toBe(false);
});
