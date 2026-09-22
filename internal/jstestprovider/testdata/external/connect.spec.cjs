const {test} = require('@playwright/test');

test.use({connectOptions: {wsEndpoint: 'ws://127.0.0.1:1'}});
test('remote browser is unqualified', async () => {});
