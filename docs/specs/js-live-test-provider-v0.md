# JS/TS Live-Test Provider V0

**Owner:** Russell Lewis
**Date:** 2026-09-11
**Intent status:** proposed
**Delivery status:** experimental
**Profiles:** none (no wire protocol frozen yet; this is a pre-qualification experiment)
**Document kind:** pre-qualification experiment profile (prose intent only; no numbered
requirement clauses, so OCM must abstain rather than invent coverage). Decision 0179
(2026-09-13) holds this profile prose-only permanently; a hypothesis gains `PREFIX-NNN`
clauses only via a separate accepting slice's own requirements, never by relabelling this
prose.

revision 2 (2026-09-13: `exit-status-unexplained` infrastructure failure for a Vitest run whose
report is all-passed but which exited nonzero, closing the gap that let an error thrown outside a
test's own run publish as green)

## Agent digest
- Claim: An experimental JS/TS provider binds Vitest and Playwright adapters whose receipts pin test identity and separate test failures from infrastructure failures.
- Status: proposed; experimental; no qualification decision, frozen wire protocol, or promotion to policy input.
- Exists: `internal/jstestprovider` (adapters, receipt/projection types, testdata unit tests) and retained source package `cmd/corvint-js-test-provider`, built standalone as `corvint-js-test-provider` (`unit`, `e2e`); evidence fixture app lives outside the repository.
- Blocked on: a numbered accepting decision, a frozen canonical wire codec, sandboxed/CI qualification parallel to AT-14, coverage observation, and any repository-gate evidence claim.
- Read next: Decision and boundary; Receipt shape and execution states; Non-goals.

## 1. Decision and boundary

`internal/jstestprovider` binds two adapters over two npm-ecosystem test runners: Vitest 5.0.0 for
unit tests and `@playwright/test` 1.63.0 for browser end-to-end tests. This is experimental
implementation evidence only; it does not qualify this proposed provider, and per AGENTS.md
invariant 8 it does not itself constitute accepted intent.

This specification is a pre-qualification experiment adjacent to
[`go-live-test-provider-v0.md`](go-live-test-provider-v0.md) and, transitively, beneath
[`live-proof-carrying-verification-v0.md`](live-proof-carrying-verification-v0.md). It makes no LPCV
compatibility claim: there is no affected-test selector, no exclusion certificate, no
repository-pass claim. Every run has scope `UNKNOWN` beyond the exact files it bound; passing a
bound test file is useful observation, never proof that omitted tests, packages, browsers, or
platforms are safe.

Both adapters are dependency-free Go orchestration around unmodified, unpatched npm packages
(`vitest`, `@playwright/test`) invoked exactly as documented. Corvint does not fork, vendor, or alter
either runner. Corvint does alter one piece of default runner behavior for the E2E path: it starts and
owns the served application under `internal/procgroup` itself rather than delegating server
lifecycle to Playwright's `webServer` config option, so that server-process cleanup is Corvint's own
verified guarantee rather than a trusted side effect of the child tool exiting cleanly.

## 2. Frozen V0 operation

### 2.1 Unit path (Vitest)

The provider runs, under `internal/procgroup` containment (bounded wall-clock timeout, bounded
stdout/stderr collection):

```text
npx vitest run --reporter=json --outputFile=<tmp-path>
```

in the caller-supplied working directory. A caller-supplied output path is removed before the run,
so a report left there by an earlier run is never read back as this run's: a Vitest process that
exits without writing one yields `report-not-written`. It reads back the JSON report and parses each
`testResults[].assertionResults[]` entry into one `TestOutcome`. Vitest 5.0.0's JSON reporter has no
structured source-location field; `file:line` anchors for a failing test are recovered by matching
the failure's stack trace against `at (\S+\.test\.[jt]sx?):(\d+):(\d+)` (`internal/jstestprovider/vitest.go`).
A `testResults[]` entry whose file-level `status` is `failed` while none of its assertions failed (a
collection error that ran no test, or a file-level hook such as `afterAll` that threw after every
test passed) adds one `infrastructure` outcome named for the file and carrying the file's `message`;
Vitest reports such a run as unsuccessful, and dropping the entry would publish only the passing
assertions. A report that yields no outcome at all (Vitest found no test files and still wrote a
report with an empty `testResults`) sets the run-level `no-suites-collected` failure, as the
Playwright path does. Vitest's own `success`/count fields (checked empirically against the
installed 5.0.0 package's `vitest/dist/chunks/index.B89dZ0-N.js`, not part of this repository) are
computed only from `numFailedTestSuites`/`numFailedTests` and never observe an error raised outside
a test's own execution window - a throw from a timer callback after its test already passed leaves
every parsed outcome
`passed` while the process still exits nonzero, with no signal anywhere in the report. When the
parsed outcomes contain no `failed` or `infrastructure` entry at all and the process nonetheless
exited nonzero, the run sets the run-level `exit-status-unexplained` failure rather than publish an
all-passed receipt (`internal/jstestprovider/runner.go:unexplainedNonzeroExit`).

### 2.2 E2E path (Playwright)

The provider:

1. Starts the caller-supplied app-server command under its own `procgroup.Run` call, in a
   cancelable child context.
2. Polls a caller-supplied readiness URL until it answers with status `< 500`, bounded by a
   readiness timeout.
3. Runs `npx playwright test --config=<path> --reporter=json` (plus any caller-supplied extra
   Playwright arguments) under a second, independent `procgroup.Run` call.
4. Cancels the server's context and confirms `procgroup.Observation.OwnedProcessGroupCleanup` fired
   before returning — this is the receipt's `serverDescendantsGone` field, recorded on every return
   after the server started, including a server that never became ready. Corvint never relies on
   Playwright's own server teardown.
5. Digests the app-build directory (if the caller names one) both before step 2 and after step 3,
   and reports `staleAppBuild = true` only when both digests are known and differ
   (`internal/jstestprovider/runner.go:isStaleAppBuild`). An app-build entry that does not resolve
   to a regular file (a symlink to a directory, a dangling symlink) makes that digest explicit
   unknown rather than an error, so the run still returns its receipt and `serverDescendantsGone`.

Playwright's JSON reporter nests `suites[].suites[]...` → `specs[]` → `tests[]` (one per project) →
`results[]` (one per retry attempt). The adapter classifies from the **last attempt's real
`result.status`** (`passed`/`failed`/`timedOut`/`skipped`/`interrupted`), never from the test's own
aggregate `status` field — a mid-run SIGINT produces an aggregate `status` of `"skipped"` on the
interrupted test even though its last attempt's real status is `"interrupted"`, and classifying from
the aggregate would misreport an operator cancellation as an ordinary skip
(`internal/jstestprovider/playwright.go:playwrightState`).

## 3. Receipt shape and execution states

A `Receipt` (`internal/jstestprovider/receipt.go`) binds:

- `identity`: content digests of every bound test file, the config file, and the combined
  `package.json`+lockfile; the Node version; the runner name/version; the exact command argv; and
  only the caller-declared environment variable names (never the full process environment).
- `appBuildAtStart` / `appBuildAtPublish` / `staleAppBuild`: E2E only. Either identity may be
  explicit `{unknown: true, reason: "..."}` when no app-build directory was named; staleness is
  never inferred across an unknown side.
- `tests[]`: one `TestOutcome` per test, each carrying a fixed `state`, retry count, duration, an
  optional `file:line` anchor, and a failure message. Failure artifacts (trace/screenshot paths)
  are listed by path, never copied into the receipt.
- `infrastructure`: set only when the run itself could not produce a real test observation (runner
  crash, timeout, output overflow, unparseable report, no suites collected). A per-test `failed`
  state is never routed through this field.
- `cancelled`, `serverDescendantsGone`: operator-interruption and cleanup-verification signals.

`ExecutionState` is one of: `passed`, `failed`, `skipped`, `flaky` (passed only after at least one
prior failed attempt), `timedOut`, `interrupted`, `infrastructure`. `internal/jstestprovider/projection.go`
maps every state onto `internal/testvalidity.Axis` values without ever collapsing to a bare boolean:
`flaky` still projects `PASSED` execution plus a nonzero retry count is visible on the outcome;
`timedOut` and `infrastructure` both project `INFRASTRUCTURE`, distinct from `interrupted`, which
projects `CANCELLED`. A run-level `infrastructure` or `cancelled` receipt field overrides the
aggregate `ReceiptRunProjection` the same way, independent of any individual test's outcome.

### Infrastructure failure reasons

The provider sets `infrastructure.reason` to the kebab-case codes below (decision 0100), in
addition to single-word reasons such as `timeout`. Each row cites the first emitting site and
states only the condition checked there.

| Code | First emitting site | At the cited site |
|---|---|---|
| `exit-status-unexplained` | `internal/jstestprovider/runner.go:149` | the Vitest process exited nonzero but none of its parsed outcomes are `failed` or `infrastructure` (e.g. an error thrown outside any test's own run) |
| `no-suites-collected` | `internal/jstestprovider/vitest.go:82` | the Vitest report produced no test outcome; the Playwright report path emits the same code at `internal/jstestprovider/playwright.go:91` when the report has no suites |
| `output-overflow` | `internal/jstestprovider/runner.go:178` | the Vitest process observation reports combined, stdout, or stderr output overflow (checked after timeout) |
| `report-not-written` | `internal/jstestprovider/runner.go:138` | after a Vitest run with no boundary failure, reading the JSON output file failed or it exceeded the 4 MiB bounded-report limit; the detail is the read error or `report-output-overflow` |
| `report-unparseable` | `internal/jstestprovider/runner.go:142` | `ParseVitestJSON` refused the output file; the detail is the parse error |
| `server-not-ready` | `internal/jstestprovider/runner.go:264` | the E2E server did not become ready at the configured URL within the ready limit (15 s when unset); the server is cancelled and its cleanup recorded in `serverDescendantsGone` first, `appBuildAtPublish` is explicit unknown, and a run whose context was cancelled during the wait reports `cancelled` instead |
| `start-failed` | `internal/jstestprovider/runner.go:182` | the Vitest process observation reports it never started (checked after timeout, output overflow, and cancellation); the detail is the observation error; the Playwright test process emits the same code at `internal/jstestprovider/runner.go:328`, checked after timeout and output overflow |
| `wait-not-completed` | `internal/jstestprovider/runner.go:184` | the Vitest process started, did not time out, overflow, or get cancelled, and its wait did not complete; the Playwright test process emits the same code at `internal/jstestprovider/runner.go:330` under the same condition, so its output is never parsed |

The per-test projection also sets one claim reason, outside `infrastructure.reason`:

| Code | First emitting site | At the cited site |
|---|---|---|
| `flaky-retry` | `internal/jstestprovider/projection.go:30` | the outcome state is `flaky`; the claim keeps association `ASSOCIATED`, hygiene `ELIGIBLE` and report state `PASSED`, and this is its only reason |

## 4. Non-goals

- No wire protocol is frozen. Today's CLI JSON output (`receipt` + `testProjections` +
  `runProjection`) is implementation-convenient and may change without notice.
- No claim that a passing bound test file proves anything about unbound files, other browsers,
  other Node versions, or CI parity.
- No network-denial or resource-containment profile beyond `internal/procgroup`'s existing
  timeout/output/descendant-cleanup guarantees.
- No coverage observation.
- No qualification harness parallel to the Go provider's AT-14 fixtures exists yet.
- The provider never fails a run (nonzero CLI exit) because a test failed. Only an infrastructure
  failure — a boundary condition that prevented a real observation — exits nonzero.

## 5. Acceptance evidence (informal, this slice)

Evidence for the states below was recorded on 2026-09-12 in an internal evidence transcript (not part of the public tree)
with the exact commands and receipt excerpts used to produce each
state live against the fixture app: `passed`, `failed` (real Vitest assertion failure and real
Playwright browser-workflow failure with `file:line` anchors), `skipped`, `flaky` (via a real retry),
`timedOut` (live, via a deliberately short Playwright per-test timeout), `interrupted`/`cancelled`
(live SIGINT with `ps`-based proof that no server or browser descendant process survives), an
infrastructure failure (`no-suites-collected`, live), and `staleAppBuild` (live, via mutating the
served app-build directory mid-run). It also records a fix-then-rerun pair showing that resolving a
failing assertion clears only that failure and leaves the pre-existing skip untouched.

Unit tests pin four boundary behaviors: `TestRunUnit_StaleCallerOutputFileIsNotReadBack` (stale
caller output file), `TestE2EBoundaryFailure_WaitNotCompleted` (Playwright wait not completed),
`TestDigestAppBuildDir_SymlinkedDirectoryIsUnknown` (unbindable app-build entry), and
`TestRunUnit_UnhandledErrorOutsideTestIsNotSilentlyGreen` (an all-passed report backed by a nonzero
exit, fixture `testdata/vitest-unhandled-error.json` captured from a real Vitest 5.0.0 run).

## 6. Rollback

Delete `internal/jstestprovider/` and `cmd/corvint-js-test-provider/`. Neither package is imported by
any other production path in this repository; removal has no other blast radius. No spec, decision,
or requirement in this document is binding until a decision number accepts it.
