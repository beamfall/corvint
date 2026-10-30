// Synthetic step-negation fixture config (LPCV-V0-070). The application is
// the Go test's locally owned httptest server; its URL arrives through
// CORVINT_NEGATE_URL.
module.exports = {
  testDir: '.',
  testMatch: /.*\.spec\.cjs/,
  timeout: 20000,
  expect: {timeout: 2000},
  use: {baseURL: process.env.CORVINT_NEGATE_URL, headless: true},
  projects: [{name: 'chromium', use: {browserName: 'chromium'}}],
};
