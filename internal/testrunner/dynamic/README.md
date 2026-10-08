# Experimental dynamic runner profiles

These adapters build an explicit invocation and parse the runner's own result format.
They do not discover or execute tests during analysis. The common executor admits the
executable, immutable inputs, generated reporters, report manifest and bounded process.
`Complete` means a structurally complete observed report, never adequate tests, a covered
requirement, a verified mutation kill, browser/application identity or release qualification.

## Runner matrix

| Runner ID | Execution/report profile | Locally observed evidence (2026-10-01, macOS arm64) |
|---|---|---|
| `mocha` | fixed Mocha event reporter, `--posix-exit-codes` | Mocha 12.0.3 pass/fail/skip/retry through common executor |
| `jasmine` | Jasmine CLI `--reporter=` fixed event-recorder shim (Jasmine 7 has no file reporter), selector reconciliation | Jasmine 7.0.0 / jasmine-core 7.0.2 on Node 22.23.3 (2026-10-08): pass/fail/xit/pending, hook, global, focused, empty and missing-selector reports; live pass/fail/skip, missing selector and load-error refusal through common executor; parallel **NOT_RUN** |
| `node-test` | Node `--test`, fixed native event reporter | Node 22.23.3 pass/fail/skip through common executor and reporter live test |
| `jest` | explicit Jest binary, `--json --runTestsByPath` | Jest 30.5.2 pass/fail/skip, collection error and retry |
| `vitest` | explicit Vitest binary, `run --reporter=json` | Vitest 5.0.3 pass/fail/skip, collection error and retry gap |
| `ava` | explicit AVA binary, native TAP stdout | AVA 8.0.1 pass/fail/skip and collection error |
| `bun-test` | Bun `test`, native JUnit output | Bun 1.3.11 pass/fail/skip |
| `deno-test` | Deno `test`, native JUnit output | Deno 2.9.7 pass/fail/skip |
| `playwright` | explicit Playwright binary, native JSON | Playwright 1.63.0 pass/fail/skip/retry; no browser fixture used |
| `cypress` | fixed Cypress per-attempt reporter, one fresh artifact per instance | Cypress 16.1.1 / Chrome 154.0.8037.59 actual browser pass/fail/skip/retry and collection error through common executor |
| `webdriverio` | pinned configuration containing official JSON reporter; explicit shard manifest | WDIO/JSON reporter 9.32.0 with real Chrome/ChromeDriver 154.0.8037.59 pass/fail/skip |
| `testcafe` | explicit browser in `Project`, native JSON | TestCafe 3.7.6 native remote-browser API produced pass/fail/skip in Chrome 154; local browser CLI blocked by macOS Screen Recording permission; common executor **NOT_OBSERVED**, permission unchanged |
| `nightwatch` | native JSON, explicit shard manifest | Nightwatch 3.16.0 pass/fail/runtime-skip and retry-to-pass with `start_session:false`; browser session **NOT_OBSERVED** |
| `detox` | explicit device configuration, Jest JSON, external Detox retries disabled | Invocation contract checked; existing task-local Detox application/device profile absent; execution **NOT_OBSERVED** |
| `storybook-test-runner` | explicit complete story set, Jest JSON | Native 0.26.0 CLI corrected; Storybook 10.6.1/Jest runtime fails collection (`module.register` unsupported); compatibility qualification **BLOCKED** |
| `storybook-vitest` | pinned Storybook Vitest config, Vitest JSON | Storybook 10.6.1 Vitest addon, Vitest 5.0.3 and Playwright Chrome 154 actual story pass/fail/skip |
| `pytest` | explicit Python `-m pytest`, native JUnit | Python 3.9.6, pytest 8.4.2 pass/fail/skip/setup-error |
| `unittest` | fixed `TestResult` hooks, explicit names or full discovery on invocation | Python 3.9.6 pass/fail/skip; reporter live test |
| `rspec` | explicit RSpec binary, native JSON | RSpec core 3.13.6 pass/fail/pending |
| `minitest` | explicit Ruby files, fixed Minitest reporter | Ruby 2.6.10, Minitest 5.11.3 pass/fail/skip; reporter live test |
| `test-unit` | explicit Ruby files, fixed Test::Unit reporter | Ruby 2.6.10, Test::Unit 3.2.9 pass/fail/omission; reporter live test |
| `rails-test` | explicit Rails test files and pinned boot helper in `Config`, Minitest reporter | Ruby 3.4.11 / Rails 8.1.4 / Minitest 6.0.6 on Linux arm64; real application boot, HTTP route, pass/fail/skip and collection error through common executor |

Missing runtime/device qualification blocks a release support claim. Fixture success does not
qualify other runner versions, operating systems, browsers, workers, child dependencies,
all lifecycle cases or full application execution. All profiles remain experimental. No
generic JUnit parser acceptance is counted as execution: the retained XML fixtures were
actually emitted by the specifically named pytest, Bun and Deno versions.

## Input and outcome boundaries

`Executable` must name the actual already-installed runner entry point or runtime. There is
no `npx`, installation, shell expansion or caller argv passthrough. `Config` is the runner's
configuration except for Rails, where it is an explicit boot helper. Legacy Storybook uses native config discovery: pin those files in `InputFiles`; custom `Config` is rejected. Ruby Minitest, Rails and
Test::Unit require exact selected files; legacy Storybook accepts a complete story set only.
Playwright literal file selections are regex-escaped and anchored. Bun paths receive an
explicit relative-path prefix. Vitest substring selection cannot prove exact-file selection
and is rejected; use pinned configuration for its test set.
Browser/device configuration is explicit in `Project`. Configurations, lockfiles, executable
scripts and their interpreter/package dependencies need the common caller's identity binding;
pinning one entry-point script alone cannot establish the full dependency closure.

WebdriverIO configuration must direct its JSON reporter into the fresh report directory.
`ReportFiles` lists every expected worker/module report. Nightwatch uses the same explicit
manifest. Cypress uses the profile-owned `cypress-*.json` pattern under the fresh directory,
subject to the common 32-report limit; Mocha uses the corresponding `mocha-*.json` pattern. Missing/extra expected workers, collection failures,
unknown status, inconsistent counts, absent completion markers, no tests, contradictory exit,
timeout, interruption and overflow cannot become a complete successful result.

The parsers reject duplicate/case-aliased JSON keys, unrecognized XML structure, contradictory
category counts and terminal-attempt conflicts. Native errors cannot disappear behind passing
rows. Nightwatch assertion `lastError` is accepted only with a retained failed attempt.

Playwright and Jest preserve reported retry attempts. Jest retry cardinality must match native
`invocations`; missing history remains a problem. Vitest 5.0.3 can emit `passed` plus earlier
failure messages without attempt ordinals: this is `FLAKY` with a missing-history problem.
Bun/Deno/pytest native JUnit, AVA TAP, RSpec JSON, TestCafe and WDIO JSON do not prove complete
retry history, and retain `NOT_REPORTED`. Nightwatch retains supplied retry data, checking its
cardinality. A final passing report never retroactively erases process errors.

## Original format evidence

- [Node native reporter events](https://github.com/nodejs/node/blob/main/doc/api/test.md)
- [Jest JSON CLI](https://github.com/jestjs/jest/blob/main/docs/CLI.md)
- [AVA TAP CLI](https://github.com/avajs/ava/blob/main/docs/05-command-line.md)
- [pytest JUnit output](https://github.com/pytest-dev/pytest/blob/main/doc/en/how-to/output.rst)
- [unittest result hooks](https://github.com/python/cpython/blob/main/Doc/library/unittest.rst)
- [RSpec JSON formatter](https://github.com/rspec/rspec/blob/main/rspec-core/lib/rspec/core/formatters/json_formatter.rb)
- [TestCafe JSON reporter](https://github.com/DevExpress/testcafe-reporter-json/blob/master/src/index.js)
- [WDIO JSON mapping](https://github.com/webdriverio/webdriverio/blob/main/packages/wdio-json-reporter/src/utils.ts)
- [Nightwatch JSON reporter](https://github.com/nightwatchjs/nightwatch/blob/main/lib/reporter/reporters/json.js)
- [Cypress reporter selection](https://github.com/cypress-io/cypress-documentation/blob/main/docs/app/run-tests/command-line.mdx)
- [Mocha native reporter events](https://github.com/mochajs/mocha/blob/main/docs/src/content/docs/explainers/third-party-reporters.mdx)
- [Detox runner forwarding](https://github.com/wix/detox/blob/master/docs/config/testRunner.mdx)
- [Storybook test runner CLI](https://github.com/storybookjs/test-runner/blob/next/README.md)

Context7 verified the applicable official API docs during implementation. Ruby reporter hook
APIs were also checked against the exact installed framework source. The Jasmine shim
`reporters/jasmine.cjs` copies native reporter-event fields only; its hash and the npm acquisition
pins are in `testdata/jasmine/provenance.json`, read from the installed 7.0.0 package source. Mocha execution support
does not imply Mocha affected-analysis routing: that separate capability remains unimplemented. Tests retain actual raw
runner reports in `testdata`; browser schema cases are explicitly labelled synthetic contract
tests. `TestProfileOwnedReportersLive` runs disposable source only and reports a skipped test
when an optional runtime/framework is absent. It does not silently claim qualification.

## Cypress and Rails runtime qualification

`testdata/runtime-qualification.json` retains the exact local runtime and tool pins. Cypress
16.1.1 was invoked through the common executor using a pinned task-only launcher to locate
its isolated binary/cache and explicit Node, Chrome and macOS `arch` tools. The native
`run` subcommand precedes `--posix-exit-codes`. Cypress's `retry` event is a serialized
object; the fixed reporter consumes `test:after:run` reconstructed tests for every native
attempt instead. Spec identity comes from `invocationDetails.absoluteFile`. Synthetic root
collection tests have no spec source and originate under `/__cypress/runner/`; they remain
incomplete collection errors. Other missing source metadata also remains unresolved.

Rails booted an actual application with an HTTP integration route in a network-disabled
container derived from the pinned official Ruby image. Its deliberate collection exception
produced exit 1 without a report, retained as incomplete. The raw mixed reports and collection
report/stderr are fixtures. Full dependency closure remains `NOT_OBSERVED`; a tool or image
pin does not establish test adequacy or qualify other runtimes/applications.
