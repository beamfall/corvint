const base = require('@playwright/test');
const test = base.test.extend({page: async ({page}, use) => use(page)});

test('custom page fixture is unknown', async ({page}) => {
  await page.goto(process.env.CORVINT_FIXTURE_URL);
});
