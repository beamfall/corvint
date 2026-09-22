const {test, expect} = require('@playwright/test');
test.use({browserName: 'firefox', viewport: {width: 321, height: 456}});
test('effective overrides', async ({browserName, viewport}) => {
  expect(browserName).toBe('firefox');
  expect(viewport).toEqual({width: 321, height: 456});
});
