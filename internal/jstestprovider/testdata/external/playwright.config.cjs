const inheritedUse = {
  browserName: 'chromium', locale: 'en-CA',
  launchOptions: {executablePath: process.env.CORVINT_FIXTURE_BROWSER_PATH}
};

module.exports = {
	testDir: '.', timeout: 5000, workers: 1, retries: 0,
	use: inheritedUse,
  outputDir: 'results',
  globalSetup: './setup.cjs', globalTeardown: './teardown.cjs',
  webServer: { command: 'node -e "process.exit(87)"', url: 'http://127.0.0.1:1' },
	projects: [
		{name: 'chromium'},
		{name: 'react', use: {viewport: {width: 900, height: 700}}},
    {name: 'broken-browser', use: {browserName: 'chromium', launchOptions: {executablePath: '/nonexistent/corvint-browser'}}}
  ]
};
