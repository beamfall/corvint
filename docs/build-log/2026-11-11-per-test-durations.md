# 2026-11-11: per-test durations (V1-5449)

Change `per-test-durations`, tasks 1, 2, 5, 6 and 7; tasks 3 and 4 stay open (see 1). Requirements
PTD-V1-001 to PTD-V1-009 in `docs/specs/per-test-durations-v1.md`. Built after `run-evidence`
(V1-5448), whose rows carry the durations; serves PROD-TEST-016 (G4) and PROD-TEST-020.

## What changed

- `internal/testduration` (shared layer, standard library only; one prefix line in
  `tools/import-ratchet/areas.json`): source names, NOT_MEASURED, the 24-hour bound, attempt sums
  and the measured-only cost figure.
- `corvint-test-run/0` outcomes gain add-only `durationSource`, `durationMs` and
  `durationGranularity`; the receipt gains `durationInvalid`. The Go adapter reads each package's
  `Elapsed`.
- `internal/testrunner` tests gain add-only `durationMs`, `durationSource` and
  `attemptDurationsMs`; the observation gains `durationInvalid`. The Vitest, Jest, Playwright,
  pytest and RSpec parsers read their report field, and Corvint's Minitest reporter records
  `Minitest::Result#time`.
- Run evidence rows copy the receipt's duration members.
- Acceptance-log row `per-test-durations` (kind `change`, mode `auto`).

## Differences from the proposal, followed by intent

1. The design says each runner adapter reads its runner's duration. On this tree the
   `corvint test` receipt has three adapters. The Go adapter parses `go test -json`, and it now
   records time. The provider adapter (Vitest and Playwright through `corvint-js-test-provider`)
   reads a provider receipt whose tests carry no time; adding one is the provider's change. The
   declared-command adapter (Jest, pytest, RSpec, Minitest) reads no report at all. So a
   `corvint test` receipt records NOT_MEASURED for every runner but Go. The report times are read
   where this tree has report parsers: the `internal/testrunner` adapters behind
   `corvint-test-runner`. Following the lane charter, tasks 3 and 4 stay unticked and the spec
   marks each runner's receipt part not yet built (changes `jest-provider`, `pytest-provider` and
   `ruby-providers`). No adapter was invented.
2. Receipt outcomes are per unit (package or test file), not per test, until test-level receipt
   rows exist. Every measured receipt outcome is therefore the package's or file's time with
   `durationGranularity: file`, as design decision 4 and PTD-V1-004 describe. A nested-module unit
   folds many packages and a declared unit is a whole suite; both stay NOT_MEASURED rather than
   take a summed or invocation time.
3. Go reads the package-level `Elapsed`, not a sum of test events. It is the package's own report
   of its tests' time and leaves out the build, which `Invocation.elapsedMilliseconds` keeps
   (PTD-V1-005). The `go-test` plugin of `internal/testrunner` reports `RunnerResult`, which has
   no duration field; it is not extended, since the receipt's Go source is covered.
4. `durationSource` is omitempty on the wire, so a receipt written before this change still passes
   the strict canonical reader. Every outcome `corvint test` writes carries it (PTD-V1-001).
   Strict readers of an older Corvint (`verify-shards`) refuse unknown members, as with ctid's
   `testId`; the D25 add-only rule covers lenient readers, which the older-reader test checks.
5. `duration-invalid` is the JSON member `durationInvalid` (camelCase, like the receipt's other
   members) on the receipt and on the testrunner observation. An invalid time does not make an
   observation incomplete.
6. Playwright's `attemptDurationsMs` is kept only when a test ran more than once; one attempt is
   `durationMs` alone. A missing attempt time makes the test NOT_MEASURED rather than a partial
   sum.
7. The cost figure and the fault-injection harness (NMS-V1-002, FIH-V1-012) are not on this tree.
   Task 6 delivers the rule they share, `testduration.Cost` and `Median`, with the fixture the task
   names (one unmeasured test labelled incomplete). Each consumer adopts it in its own change.
8. A fractional reported time (Jest's 12.4 ms in the fixture) is rounded to the nearest
   millisecond. It is still the reported value, not an estimate.
9. Existing whole-struct outcome assertions in `internal/sessiontest` and `cmd/corvint` gain the
   new members (`durationSource: NOT_MEASURED`, and `durationGranularity: file` for a package or a
   provider file that ran). No frozen golden holds receipt outcomes.
10. `TestSTRV0011ReceiptsHaveNoOtherReader` counts sessiontest functions that take a `Receipt`.
    The stamping helper takes the invocations instead, since it is part of the writer.

## Evidence

- `script/heavy.sh go test -count=1` on `./internal/testduration/`, `./internal/sessiontest/`,
  `./internal/testrunner/dynamic/`, `./internal/testrunner/` and `./cmd/corvint-test-runner/`: ok.
- `./cmd/corvint/` with `-run 'TestPTDV1|TestRVDV1|TestSTRV0|TestCMSV0|TestAFPV02487|FailFast'`: ok.
- `make traceability-tests-check`: ok.

## NOT_RUN

- A live Minitest, Jest, Vitest, Playwright, pytest or RSpec run: none of these runtimes is
  installed on the delivering host, so the report fixtures are written by hand in each runner's
  documented shape. The Minitest reporter change is checked only for its text, and not executed.
- The G4 measurement run (NMS-V1-002): its change is not on this tree.
- `openspec validate per-test-durations`: the OpenSpec CLI is not installed on this host.
