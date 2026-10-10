import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  retries: 1,
  projects: [
    { name: 'alpha' },
    { name: 'beta' },
  ],
});
