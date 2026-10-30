# route-stubs residual live run (Playwright 1.61.1)

Retained, unedited output of one local run of `internal/jstestprovider/route-stubs.cjs` on
2026-10-16 (LPCV-V0-079; V1-1475): Playwright 1.61.1, Node v22.23.3, the already-installed bundled
chromium-headless-shell 1228, darwin arm64, against the loopback app in `server.cjs`. No browser
was downloaded. Command, run in this directory with `node_modules` temporarily linked to an
existing Playwright 1.61.1 install (the link and `test-results/` were removed afterwards):

    node node_modules/playwright/cli.js test --config playwright.config.cjs

`report.json` is the JSON reporter output and `run.log` the console output. Five tests pass:

- a context from `browser.newContext()` records one hit context stub and one never-matched page
  stub;
- a page from `browser.newPage()` records one never-matched stub;
- `page.routeWebSocket` records one hit and one never-matched stub with `api` `routeWebSocket`;
  the app has no WebSocket server, so only the stub answers;
- `page.routeFromHAR` and `context.routeFromHAR` (with `items.har`) record one hit and one
  never-matched stub with `api` `routeFromHAR`;
- a test without a browser fixture publishes no record.

`route_stubs_test.go` reads `report.json`; it does not rerun Playwright.
