const { defineConfig } = require("@playwright/test");

// IPv6 loopback: some hosts drop (rather than refuse) IPv4 loopback connects
// to a closed port, which hangs the webServer pre-start check.
module.exports = defineConfig({
  testDir: "tests",
  use: { baseURL: "http://[::1]:3123" },
  projects: [{ name: "chromium", use: { browserName: "chromium" } }],
  webServer: { command: "node server.js", url: "http://[::1]:3123", reuseExistingServer: false },
});
