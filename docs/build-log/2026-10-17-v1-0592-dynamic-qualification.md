# 2026-10-17 V1-0592: dynamic runner qualification matrix

Outcome. All 22 dynamic profiles re-ran their P/F/S/Z/C/R/T cases through
`corvint-test-runner plan`/`run` and the shared executor. Gap matrix before (base `8cd20e92b`):
witness 75, parser-only 7, none 61, not applicable 11. After: witness 136 (4 of them fail-closed
retry witnesses), parser-only 0, none 7 (Detox 6, storybook-test-runner retry 1), not applicable 11.
The matrix, tuples and retained blockers are in the "Dynamic qualification matrix 2026-10-17
(V1-0592)" section of `docs/specs/test-runner-execution-v0.md`. Evidence root:
`/private/tmp/cem10-build/dynamic-2026-10-17/<profile>/<case>/` (request, plan, receipt, raw
reports, phase output, `summary.json`).

Defects found by the actual runs and repaired in the dynamic parsers, each with a pre-repair
evidence directory and a focused test over the retained raw report
(`internal/testrunner/dynamic/qualification_0592_test.go`):

- TRE-V0-060 Node's synthetic file-level event counted as a passing or failing test, so a file
  with zero tests or a top-level throw yielded a complete run.
- TRE-V0-061 AVA's `# N tests remaining` after a failing `before` hook yielded a complete run.
- TRE-V0-062 Bun 1.3.11 JUnit suite `line` attributes were refused, and Bun's unnamed hook case
  became a FAILED test.
- TRE-V0-063 The Storybook test-runner's skipped `no-op` placeholder for a story-less CSF file
  became a complete SKIPPED run with exit 0.

No new problem codes; no shared executor, plan or receipt change. The four requirements are
accepted under the owner's standing auto-accept of lane requirements (2026-10-11).

Corrections. The earlier TestCafe tuple label "Chrome for Testing 154 headless" is wrong for the
qualified remote runs: they used Playwright's chrome-headless-shell 153.0.8010.12; Chrome for
Testing 154 `--headless=new` never connected (`testcafe/remote-cft154-noconnect`). Storybook
telemetry may have been attempted once during the first storybook-vitest run, before
`core.disableTelemetry` and `STORYBOOK_DISABLE_TELEMETRY=1` were set; every later run disabled it.

Installs (coordinator-approved 2026-10-17; user space under
`/private/tmp/cem10-build/toolchains-2026-10-17/`; no sudo, global package or PATH change):

- npm, per directory: `npm install --save-exact --cache $T/npm-cache --no-audit --no-fund
  <pkg@version>`, verified by lockfile integrity. Lockfile sha256: jest 30.5.2 `62fd338e…9cb3b`;
  vitest 5.0.3 + vite 8.3.4 `dac58d3e…fd48b`; mocha 12.0.3 `22e4a262…77f19`; ava 8.0.1
  `ca3114cf…9808d`; @playwright/test 1.63.0 `95cbf2c6…d00453`; jasmine 7.0.0 / jasmine-core 7.0.2
  `bd61e5bf…e19b3`; WebdriverIO 9.32.0 (cli, local-runner, mocha-framework, json-reporter)
  `0ad7dc33…c781c`; nightwatch 3.16.0 `6b20e0af…c744`; testcafe 3.7.6 `54f275d9…f01d`;
  cypress 16.1.1 `1409dfe9…0cd30`; Storybook 10.6.1 set (html-vite, addon-vitest,
  @storybook/test-runner 0.26.0, @vitest/browser-playwright 5.0.3, playwright 1.63.0)
  `a0edd91d…ddf7d4`.
- Cypress 16.1.1 binary: npm postinstall from download.cypress.io, checksum verified by the CLI;
  executable `3e66d4cb…cd40`.
- Chrome for Testing 154.0.8037.59 mac-arm64 from
  `storage.googleapis.com/chrome-for-testing-public/154.0.8037.59/mac-arm64/`, GCS md5 verified:
  `chrome-mac-arm64.zip` sha256 `c812ad7488f4d65b35c81a8b2acd3bf9be8731fe5b2a57ec4b6c0e1e37dd7b8f`
  (191,431,557 bytes); `chromedriver-mac-arm64.zip` sha256
  `f7c643949fc328216545571382555d1b54865cad0ad055a2dc02ccf4db07822b` (9,303,021 bytes);
  chromedriver `9263a5ed1b6b46c0ec8c4c8547eec48ddc94072a348f307ea06905f1a4ba9622`.
- Playwright 1.63.0 browsers: `PLAYWRIGHT_BROWSERS_PATH=$T/browsers/ms-playwright npx playwright
  install chromium firefox webkit` from cdn.playwright.dev (no published checksums; executable
  sha256 recorded): chromium-1243 (153.0.8010.12) `8319963f…668a`, chromium_headless_shell-1243
  `a0bfe7b4…8282`, firefox-1543 (155.0) `e75d92f6…fbf5a`, webkit-2359 (26.6) `7ba0926c…af9f`.
- pytest 8.4.2 with iniconfig 2.1.0, packaging 25.0, pluggy 1.6.0, pygments 2.19.2:
  `pip install --require-hashes --only-binary=:all: -r requirements-cp313.txt` into a copy of
  python-build-standalone CPython 3.13.15; pytest wheel sha256
  `872f880de3fc3a5bdc88a11b39c9710c3497a547cfa9320bc3c5e62fbf272e79`.
- RubyGems, sha256 checked against rubygems.org: rspec-core 3.13.6 `a8823c64…828b2d`,
  rspec-support 3.13.6 `2e8de370…f8afb2`, rspec-expectations 3.13.5 `33a4d3a1…58836`,
  diff-lcs 1.6.2 `9ae0d2cb…909962`.
- Node 22.23.3 per the orchestrator manifest. Rails ran in the existing local image
  `corvint-cem10-rails:qualification` (`sha256:9c84b5b5…7aa1`) with `--network none`.

Launchers (task-local, because the executor clears the environment): `browsers/webkit-launch.sh`
`3b8b1fa8…6e55f52`, `testcafe/remote-launch.js` `79c3ccca…0bd133`, `storybook/sb-runner-launch.cjs`
`a2e13c72…b69d18`, `cypress/launch.js` `0611d578…a00f0b`, `ruby-gems/rspec-launcher.rb`
`1bc92ceb…d5a55f`. Every browser, driver and static server started by the lane was stopped.

Retained blockers: Detox (no Detox CLI, instrumented app, `applesimutils` or booted
simulator/emulator); TestCafe local browsers (macOS Screen Recording permission, not granted);
storybook-test-runner retry (no admitted Jest config, no native retry flag); fail-closed retry
history for Vitest, storybook-vitest, Bun and WebdriverIO. NOT_RUN: `make gate`, full
`go test ./...`.

Rollback: revert the parser changes in `internal/testrunner/dynamic/{native,parse,validation}.go`,
the test and its fixtures; the spec matrix stays as retained evidence.
