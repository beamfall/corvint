# 2026-10-31 runners-mac test-runner and integration batch

Lane runners-mac worked internal/testrunner, testselect, jstestprovider and integrations/{codex,
gemini,opencode,pi} on the Mac. The base was origin/main `8daec98ce4349c1583186692726d2c3a782ac647`.
Go tests ran through `script/heavy.sh`, locally, with each test pattern in its own run and the
timeout set explicitly. Load averages ran from 23 to 213. No Tasks writes, pushes or Codex runs
were made.

## Delivered

- **V1-2347, V1-2518 (AHI-950):** the OpenCode qualification fix.
  `./internal/opencodequalification` passes when run whole (15 s).
- **V1-2495 (AHI-951):** adds `script/publish-codex-plugin.sh`.
  - `check` refuses a plugin manifest whose skills, mcpServers or hooks path is missing,
    leaves the plugin, or names an empty `.mcp.json`.
  - `publish` takes the plugin from a committed revision and replaces the hooks with the
    qualified rendering. It omits `scripts/` and writes `PUBLISH.json`.
  - `script/publish-codex-plugin_test.sh` passes.
  - AC3, a live Codex install, is BLOCKED because this lane may not run Codex.
- **V1-2977 (AHI-952, AHI-953):** the Gemini 0.2.12 bump. The bump had left out the Gemini hook
  literal; AHI-953 adds it.
- **V1-3040:** passes on darwin. The linux CI half is NOT_RUN.
- **V1-0582:** passed 50 of 50 runs on wsl-lanes and 50 of 50 on darwin.
- **V1-1235 (AFU-V1-950):** `TestAFUV1950PlaywrightDecodersAgree` runs one real Playwright
  1.61.1 report through all three decoders. All three must report the same tests, attempts and
  verdicts. The report includes a `test.fail()` test that passes, which must count as a failure.
- **V1-3409 (AHI-954):**
  - Decision: an explicit exemption, not an earlier check. The ticket's early check would have
    refused inputs that the later stage accepts and records.
- **V1-2185 (MTV-V0-950):**
  - The probe HTTP client moves to `internal/jstestprovider/loopbackhttp`, which registers it
    from `init`. jstestprovider itself imports no package that opens network connections, and a
    probe fails closed when no client is linked.
  - Decision: only the HTTP client moved. The E2E runner and its receipt model stay in
    jstestprovider. Moving the runner would have broken receipt imports in four packages for no
    gain to the boundary.
  - Both allowlists shrink. The core HTTP importers go from {jstestprovider, testrunner} to
    {testrunner}. The test-validity tool's network importers go from {jstestprovider} to empty.
  - `go list -deps ./cmd/corvint` lists no loopbackhttp.
  - These pass: the jstestprovider, corvint-js-test-provider, playwrightminimize and
    testacceptance packages, and `go vet`.
  - changee2e passes in three `-run` splits (48 + 70 + 3 tests); its live tests skip without
    Playwright. The whole-package run timed out at 600 s, and again at 520 s, under load
    170-213. The goroutine stack was in `ledger_test.go:82`, with no jstestprovider frames.

## ALREADY-DELIVERED on the base

- **V1-1672:** PWP-V0-050..054; `TestPWPV0054ReporterExternalNodesMatchTable` passes.
- **V1-1534:** delivered by V1-2182 / TRE-V0-117 (`parseXcode`);
  `TestXcodeBuildFailureAndZeroTests` passes.
- **V1-0606, AC2:** `docs/specs/test-runner-execution-v0.md` and `internal/testrunner/sql/README.md`
  already document the requirement.
- **V1-2149, AC1:** PWV-V0-016; `host-version.test.mjs` passes 2 of 2.

## Measured

- **V1-1731:** checked the frozen corpora in `benchmarks/*manifest*.json`.
  - They are zod@e516c3ba, execa@8017b279, flask@d318b683, cobra@adbc8813, and beamfall/core at
    5dbd4ed6 and at 6e82abd8.
  - None of them has a `*.setup.*` or `*.seed.*` file.
  - Only core has a `playwright.config`. `corvint tests setup-select` on core returns mode `all`
    at both revisions, with 0 setup files and an empty universe. The reasons it gives are
    unresolved `playwright-core/lib/*Bundle` imports in `e2e/lib/archived-visual-parity*`.
  - So no `all` outcome comes from open setups, and no narrowing ticket is needed.
- **V1-1476:** cleared at load 141-213 on darwin.
  - 10 of 10 runs passed for `TestNEAV0006ObservedNestedGroupCancellation` and for each of seven
    testrunner retirement and interrupt tests.
  - No run logged "descendant snapshot unavailable".
  - `TestExecuteRefusesUnprovenRetirement` skipped.

## BLOCKED

- **V1-0606, AC1 and AC3:** sqllogictest-bin 0.29.1 and PostgreSQL are not installed on any lane
  host. Installing them needs owner approval.
- **V1-1536:** neither the Mac nor WSL has GoogleTest, and the earlier evidence directory is
  gone.
- **V1-2149, AC2:** no built image exists, and building one needs a network npm install.
- **V1-2495, AC3:** the live Codex install, as noted under Delivered.

## V1-1669: needs reshape, not consolidated

The two acceptance criteria cannot both be met as written.

- **Argv order.** The callers order the same flags differently. gorunner emits
  `test -json -count=1 -vet=off`, and mutate emits `go test -count=1 [-json] -timeout -run`.
  sessiontest emits `go test -exec confine -json` in CI, and native emits `test -json -count=1`.
  One owning builder has to pick one order, but AC2 requires each caller's existing tests to
  pass unchanged.
- **Receipts.** gorunner records its argv in its run results (`runner.go:119`), so reordering
  changes recorded evidence.
- **Two process-group mechanisms.** native goes through `procgroup`, and mutate goes through
  `groupreap.Run`. gorunner calls `exec.CommandContext` directly. sessiontest only plans argv and
  runs nothing. Merging `procgroup` and `groupreap` is its own cross-platform change.
- **Recommended reshape:**
  1. Share the hermetic environment values (`GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`).
  2. Add a parity test that pins each caller's argv.
  3. Decide on `procgroup` versus `groupreap` separately.

## Friction

- (a) The spec Intent-status freeze is discovered only when `make spec-requirements` runs.
- (b) Mac load ran from 23 to 213.
  - Go's default 600 s timeout fails changee2e at load 200, and heavy.sh does not add
    `-timeout 30m`.
  - Long commands get moved to the background automatically.
- (c) The UserPromptSubmit hook reported `corvint-event-rejected:dogfood-event-deadline`.
- (d) The V1-2495 ticket assumed publication tooling that did not exist.
- (e) heavy.sh needs `HEAVY_REMOTE=0` even when `HEAVY_LOCAL_REASON` is set.
- (f) `host-adapters.test.mjs` needs fixture environment variables that are set up by hand.
- (g) The Gemini bump left out the hook literal (fixed, AHI-953).
- (h) The README command lacked the Pi environment variable (fixed).
- (i) The V1-1235 ticket missed the `test.fail()` false pass. `qualified-reporter.cjs` reads
  `result.status` and ignores `expectedStatus`, so it may have the same false pass. Suspected,
  not confirmed.
- (j) There is no Playwright `.bin` shim and no Playwright 1.63.0 install on the Mac, so the
  playwrightminimize and changee2e live tests skip.
- (k) The interactive `cp` alias stalled a restore that ran in the background.
- (l) heavy.sh refuses a whole-package run without a reason. Corvint selected packages for the
  whole lane, so each run needed its own hand-written reason.
- (m) The installed corvint binary is stale (`dbc1713779fc`), which degrades test selection.
- (n) Three dispatched tickets were already delivered on the base (V1-1534, V1-2149 AC1,
  V1-1672). No check for delivered work runs before dispatch.
- (o) Tickets that need a live witness (V1-1536, V1-2149, V1-0606) need toolchains that no lane
  host has.
- (p) `setup-select` returns `all` with unresolved-import reasons even when there are no setup
  files. Repro: core-history at 5dbd4ed6, `internal/web/app`.
- (q) V1-1669's two acceptance criteria conflict (see above).
