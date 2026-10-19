# 2026-10-19 V1-1919, V1-1930, V1-1931, V1-1932: outcome, cleanup, input and vacuity rules

Tickets: V1-1930 (GitHub #1191, `EITHER-OR-OUTCOME`), V1-1919 (GitHub #1185,
`CLEANUP-HANDLE-OUTLIVED`), V1-1931 (GitHub #1192, `UNSTUBBED-OUTBOUND-INPUT` and
`ENV-UNSTUBBED-CALL`) and V1-1932 (GitHub #1193, `VACUOUS-EMPTY-EQUALITY`).

## Decision

`TSA-V0-160` to `TSA-V0-176` are added to `docs/specs/test-source-analysis-v0.md` and accepted
under the owner's standing auto-accept of lane requirements (2026-10-11). No accepted requirement is
amended. The slice adds four static rules to `corvint tests validity` on the existing TSA-V0-062
token reader, and one fail-closed Playwright infrastructure reason.

- Severity: `EITHER-OR-OUTCOME` is invalid, since an assertion accepting two outcomes cannot detect
  a regression between them; it makes the test `INELIGIBLE` and the report refuses. The other three
  are advisory: each depends on facts the file may not state (a fixture's real lifetime, a stub
  declared elsewhere, seeding outside the test), so they warn without changing hygiene.
- Dispatch: a JS/TS test file gets all four rules. Another JS/TS file (a fixture file) gets
  `CLEANUP-HANDLE-OUTLIVED` only. A Markdown, YAML or JSON file named with `--files` is read as a
  behaviour contract for `expected` keys whose value is `X or Y` or `X/Y`. Each rule abstains when
  the file does not state what it needs (an imported or non-literal-scoped registry, a non-literal
  stub matcher).
- `passed-cleanup-failed` (TSA-V0-168): `ParsePlaywrightJSON` now returns an infrastructure failure
  when the report has suites, a run-level error and only passing, flaky or skipped outcomes. The
  basic E2E runner already drops tests on an infrastructure failure, so such a run is no longer
  proof. The qualified external path already fails closed on `reporter-global-error`, so the
  appflows wire (AFU-V1-042) is not changed.
- Run-time parts stay NOT_PRODUCED with named reasons, because no receipt carries the data:
  `ENV-UNSTUBBED-CALL` (`server-outbound-log-not-produced`), the run-time vacuous reading
  (`assertion-operands-not-produced`), proposing the single value from a passing receipt, and
  row-citation drift at a pinned commit.
- New code lives in new files with feature-prefixed helpers (`eo`, `ch`, `uo`, `ve`). Shared edits
  are one `ValidityFindings` hook, one `ParsePlaywrightJSON` return and four help lines. This keeps
  the change merge-friendly with the open test-validity PRs.

## Evidence

- At BASE `714393264` the rules do not exist, so every fixture gives 0 findings. After the change:
  - either-or: 7 JS forms, 2 annotation survivors and 3 contract lines;
  - cleanup: 3;
  - unstubbed input: 3;
  - vacuous: 3;
  - cleanup gate: 1 infrastructure failure.
  Each rule also has negative and abstain fixtures that give 0.
- Mutations: 15 mutations, all KILLED. They break, in turn:
  - either-or: `toContain`, the annotation, the contract reader and the invalid severity;
  - cleanup: the worker-scope test, `afterAll`, closure rebinding and fixture-file dispatch;
  - unstubbed input: coverage, the non-literal abstain and the nearest-value choice;
  - vacuous: the `.count()` reading and the precondition;
  - the cleanup gate, and keeping a failing test's outcome.
- Focused tests, through `heavy.sh`, one package per command:

  | Package | Result | Load average |
  |---|---|---|
  | `internal/testsource` (`-race`) | ok | 34.96 |
  | `internal/testvalidity` (`-race`) | ok | 39.54 |
  | `internal/jstestprovider` (`-race`) | ok | 37.00 |
  | `internal/testplan` | ok | 43.95 |
  | `internal/testvaliditydoc` | ok | 40.84 |
  | `internal/liveverify/affected/typescript` | ok | 39.65 |
  | `cmd/corvint` (validity and help tests) | ok | 35.55 |

- `internal/jstestprovider` failed once under `-race` at load 38, in
  `TestPWPV0030ExternalTimeoutInterruptsRunnerFirst` ("runner did not see SIGINT"). That test has a
  300 ms timeout and is unrelated to this change. It passed 3 of 3 alone, and the package then
  passed. It is a residual.
- No live qualification on a real suite was run (NOT_RUN).
