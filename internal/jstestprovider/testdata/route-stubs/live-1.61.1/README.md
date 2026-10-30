# route-stubs live run (Playwright 1.61.1)

Retained, unedited output of one local run of `internal/jstestprovider/route-stubs.cjs` on
2026-10-15 (LPCV-V0-075): Playwright 1.61.1, Node v22.23.3, the already-installed bundled
chromium-headless-shell 1228, darwin arm64, against the loopback app in `server.cjs`. No browser
was downloaded. Command, run in this directory with `node_modules` temporarily linked to an
existing Playwright 1.61.1 install (the link and `test-results/` were removed afterwards):

    node node_modules/playwright/cli.js test --config playwright.config.cjs

`report.json` is the JSON reporter output and `run.log` the console output. The suite ran from
this directory, so `tests/fixtures.cjs` requires the producer by the same relative path it has
here. Three tests pass: one stub is hit, two stubs never match while the test still passes (the
page reads the real `/api/items`), and one test takes no browser fixture and publishes no record.
`route_stubs_test.go` reads `report.json`; it does not rerun Playwright.
