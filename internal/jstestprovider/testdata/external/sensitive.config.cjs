module.exports = {
  testDir: '.', timeout: 15000, workers: 1, retries: 0,
  use: {browserName: 'chromium'},
  projects: [{name: 'chromium'}]
};
