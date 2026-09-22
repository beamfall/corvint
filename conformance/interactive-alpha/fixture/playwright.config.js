import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: ".",
  testMatch: "e2e.spec.js",
  retries: 0,
  workers: 1,
  use: { headless: true },
});
