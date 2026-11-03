const { defineConfig } = require("cypress");

module.exports = defineConfig({
  video: false,
  screenshotOnRunFailure: false,
  e2e: {
    baseUrl: "http://[::1]:3123",
    specPattern: "cypress/e2e/**/*.cy.js",
    supportFile: false,
  },
});
