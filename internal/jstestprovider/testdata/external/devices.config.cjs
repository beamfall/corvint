const { defineConfig, devices } = require('@playwright/test');

module.exports = defineConfig({
  testDir: '.',
  projects: [{name: 'chromium', use: {...devices['Desktop Chrome']}}]
});
