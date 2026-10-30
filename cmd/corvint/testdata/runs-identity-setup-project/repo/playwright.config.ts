// Mirrors GitHub #1866: setup tests live in a project testDir beside the top-level one.
import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  projects: [
    { name: 'setup', testDir: './setup', testMatch: /.*\.setup\.ts/ },
    { name: 'chromium', dependencies: ['setup'] },
  ],
});
