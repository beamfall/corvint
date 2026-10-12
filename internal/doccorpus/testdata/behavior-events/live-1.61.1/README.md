# behavior-events live run (Playwright 1.61.1)

Retained, unedited output of one local run of `internal/doccorpus/behavior-events.cjs` on
2026-10-12: Playwright 1.61.1, Node v22.23.3, bundled chromium-headless-shell 1228 (149.0.7827.55),
darwin arm64, against the loopback app in `server.cjs`. Command, from the suite directory with
`node_modules` linked to an existing Playwright 1.61.1 install:

    node node_modules/playwright/cli.js test --config playwright.config.cjs

`report.json` is the JSON reporter output and `run.log` the console output. The run's
`fixtures.cjs` required the producer by the absolute path of the worktree that ran it; the retained
copy requires it relative to this directory (`../../../../behavior-events.cjs`), which resolves to
the same file inside the repository, so the suite does not name a local path. `behavior_events_producer_test.go` reads
these files; it does not rerun Playwright.
