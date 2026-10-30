'use strict';
const { defineConfig } = require('playwright/test');
module.exports = defineConfig({
  testDir: './tests',
  testMatch: '*.spec.cjs',
  retries: 0,
  workers: 1,
  reporter: [['json', { outputFile: 'report.json' }]],
  use: { baseURL: 'http://127.0.0.1:4880', headless: true },
  projects: [{ name: 'alpha', use: { browserName: 'chromium' } }],
  webServer: { command: 'PORT=4880 node server.cjs', url: 'http://127.0.0.1:4880/ready', reuseExistingServer: false },
});
