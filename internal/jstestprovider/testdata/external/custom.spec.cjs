const base = require('@playwright/test');
const test = base.test.extend({page: async ({page}, use) => use(page)});

test('custom page fixture keeps the launch identity', async ({page}) => {
  await page.goto(process.env.CORVINT_FIXTURE_URL);
});
