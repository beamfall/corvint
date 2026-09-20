const {test, expect} = require('@playwright/test');
test.use({viewport: async ({}, use) => use({width: 321, height: 456})});
test('executable identity is unknown', async ({viewport}) => expect(viewport.width).toBe(321));
