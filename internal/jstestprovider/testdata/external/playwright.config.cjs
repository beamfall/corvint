module.exports = {
  testDir: '.', timeout: 1500, workers: 1, retries: 0,
  outputDir: 'results',
  globalSetup: './setup.cjs', globalTeardown: './teardown.cjs',
  webServer: { command: 'node -e "process.exit(87)"', url: 'http://127.0.0.1:1' },
  projects: [
    {name: 'chromium', use: {browserName: 'chromium'}},
    {name: 'react', use: {browserName: 'chromium', viewport: {width: 900, height: 700}}},
    {name: 'broken-browser', use: {browserName: 'chromium', launchOptions: {executablePath: '/nonexistent/corvint-browser'}}}
  ]
};
