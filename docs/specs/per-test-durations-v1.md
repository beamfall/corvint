# Per-test durations V1

Owner: Russell Lewis
Date: 2026-11-11
Intent status: accepted
Delivery status: partial
Health: KEEP
Authoritative inputs: change folder `docs/specs/openspec/changes/archive/2026-11-13-per-test-durations/` (`proposal.md`,
`design.md`, `tasks.md` and `specs/per-test-durations/spec.md`), `docs/specs/PRODUCT-PLAN-1.0.md`
(PROD-TEST-016, PROD-TEST-020) and `docs/decisions/0596-accept-product-plan-1-0-2026-11-10.md`
(owner decisions R7, R19 and R38; G4).

## Agent digest
- Claim: Receipts record each test's measured time from the runner's own report and mark a missing or invalid time NOT_MEASURED, never estimated.
- Status: accepted / partial. Change `per-test-durations` auto-accepted under G14 (decision 0596); delivered under V1-5449.
- Exists: `internal/testduration`; duration members on `corvint-test-run/0` outcomes (Go, Vitest, Playwright, Jest, pytest and RSpec measured at file granularity); the `internal/testrunner` adapters' report durations; run evidence rows copy them.
- Blocked on: Minitest receipt outcomes stay NOT_MEASURED until change `ruby-providers` adds a Minitest contract adapter; `attemptDurationsMs` is observation only; the cost figure and harness adopt `testduration.Cost` in their own changes.
- Read next: Surfaces; Traceability.

## User and boundary

The G4 cost figure compares the median time of the selected tests with the full suite, excluding
build and setup. Before this change a `corvint test` receipt recorded only the elapsed time of each
adapter launch, build included, and no runner adapter kept a per-test time.

This spec is the canonical home of requirements PTD-V1-001 to PTD-V1-009, copied from the change's
`specs/per-test-durations/spec.md`. Design decisions, failure modes and rollback stay in the
change's `design.md`, which owns them; this file adds only the delivery record (Surfaces,
Traceability).

## Requirements

- `PTD-V1-001`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5449) Duration fields on every per-test outcome

Every per-test outcome in a `corvint test` receipt MUST carry `durationSource`, and MUST carry
`durationMs` (integer milliseconds) when the duration was measured. The fields MUST be add-only.

**Scenario: Vitest receipt**
- GIVEN a Vitest run where `math > adds` passed in 3 ms
- WHEN the receipt is written
- THEN its outcome has `durationMs` 3 and `durationSource` `vitest-json`

- `PTD-V1-002`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5449) One named source per runner

Each runner MUST take durations from its own report: Go from `go test -json` `Elapsed`, Vitest and
Jest from the JSON report `duration`, Playwright from the sum of attempt durations, pytest from the
JUnit `time` attribute, RSpec from the JSON formatter `run_time`, and Minitest from a reporter field
filled from `Minitest::Result#time`.

**Scenario: Minitest receipt**
- GIVEN a Minitest run through Corvint's reporter
- WHEN the receipt is written
- THEN each Minitest outcome has `durationMs` and `durationSource` `minitest-reporter`

- `PTD-V1-003`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5449) Missing durations are not estimated

When a runner's report has no duration for a test, the outcome MUST omit `durationMs` and set
`durationSource` to `NOT_MEASURED`. Corvint MUST NOT estimate, average or copy a duration from
another run.

**Scenario: report without times**
- GIVEN a pytest JUnit report whose `testcase` elements have no `time`
- WHEN the receipt is written
- THEN each pytest outcome has `durationSource` `NOT_MEASURED` and no `durationMs`

- `PTD-V1-004`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5449) File-granularity durations are marked

An outcome recorded per file rather than per test MUST carry `durationGranularity: file`.

**Scenario: legacy package unit**
- GIVEN an outcome for legacy unit `go:./internal/x`
- WHEN the receipt is written
- THEN it carries the package duration with `durationGranularity: file`

- `PTD-V1-005`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5449) Invocation time is not divided among tests

`Invocation.ElapsedMilliseconds` MUST keep measuring the whole launch and MUST NOT be divided among
tests or used as a per-test duration.

**Scenario: Go run with build time**
- GIVEN a Go run whose build took 40 seconds and whose tests took 2 seconds
- WHEN the receipt is written
- THEN the per-test durations sum to about 2 seconds and the invocation time includes the build

- `PTD-V1-006`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5449) Retries record each attempt

A test run more than once in one invocation MUST record `attemptDurationsMs` and a `durationMs` equal
to their sum.

**Scenario: Playwright retry**
- GIVEN a Playwright test that failed in 900 ms and passed on retry in 700 ms
- WHEN the receipt is written
- THEN `attemptDurationsMs` is [900, 700] and `durationMs` is 1600

- `PTD-V1-007`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5449) Invalid values are named

A negative, non-numeric or above-bound duration MUST be recorded as `NOT_MEASURED`, and the receipt
MUST list the test under `duration-invalid`.

**Scenario: negative duration**
- GIVEN a Jest report with `duration` -5 for one test
- WHEN the receipt is written
- THEN that test is `NOT_MEASURED` and listed under `duration-invalid`

- `PTD-V1-008`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5449) Run evidence carries durations

Each run evidence row (RVD-V1-006) MUST copy `durationMs`, `durationSource` and, when present,
`durationGranularity` from the receipt.

**Scenario: stored run**
- GIVEN a `corvint test` run with measured durations
- WHEN its record is appended to the run evidence store
- THEN each row holds the test's duration and source

- `PTD-V1-009`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5449) Consumers use only measured durations

The cost figure (NMS-V1-002) and the fault-injection harness (FIH-V1-012) MUST use only measured
per-test durations and MUST report the number of selected and full-suite tests that were
`NOT_MEASURED`. A figure with any unmeasured test MUST be labelled incomplete.

**Scenario: one unmeasured test**
- GIVEN a full baseline run where one test is `NOT_MEASURED`
- WHEN the median selected time is computed
- THEN the figure is labelled incomplete and names the unmeasured count

## Surfaces

Delivered by ticket V1-5449 (build-log `docs/build-log/2026-11-11-per-test-durations.md`):

- `internal/testduration` (shared layer, standard library only) holds the seven source names, the
  NOT_MEASURED rule, the 24-hour bound (the change's Open item 1, as proposed), the attempt sum, and
  `Cost` and `Median`: the measured-only cost figure with its selected and full-suite NOT_MEASURED
  counts, labelled incomplete with no ratio when any test is unmeasured (PTD-V1-003, PTD-V1-006,
  PTD-V1-007, PTD-V1-009).
- `corvint-test-run/0`: each outcome gains `durationSource`, `durationMs` when measured and
  `durationGranularity`, and the receipt gains `durationInvalid`, all omitempty so a receipt
  written before them reads strictly with its bytes unchanged. `corvint test` writes
  `durationSource` on every outcome. Its units are packages and files, so a measured outcome is
  marked `file` (PTD-V1-004). `Invocation.elapsedMilliseconds` is unchanged (PTD-V1-005). A Go
  package records its `Elapsed` only when its tests ran (PASS, FAIL, TIMED_OUT); a build failure or
  a package with no tests is NOT_MEASURED (PTD-V1-003). A provider (Vitest, Playwright) file unit
  records the sum of its tests' times from the provider receipt, a retried Playwright test counting
  each attempt; it is NOT_MEASURED when a test has no time (no `durationMs`, or the receipt's
  `durationMissing` mark, which the Vitest parser sets for an absent, null or non-numeric
  assertion `duration`), a retry kept only its last attempt's
  time, or a test cannot be placed in one selected file (several Vitest files, whose passing tests
  name no file). The receipt outcome has no `attemptDurationsMs`: a file outcome sums many tests,
  so `attemptDurationsMs` is observation only, on `corvint-test-runner` (PTD-V1-006).
- Added by V1-5542 (2026-11-13, build-log `docs/build-log/2026-11-13-jest-pytest-on-the-contract-path.md`):
  a contract (Jest, pytest, RSpec, Vitest) file unit of `corvint test` records the sum of the times
  the runner's report gave its tests, at granularity `file`, under the runner's source. The
  contract parsers mark a result whose report time is absent or invalid (`Result.DurationMissing`,
  `Result.DurationInvalid`, not serialized), so a missing time makes the file NOT_MEASURED and an
  invalid one lists it under `durationInvalid`; a selected test left NOT_RUN makes it NOT_MEASURED.
  A file-level result (a Jest file that failed to load, a pytest collection error on the JUnit or
  pytest-json-report path) carries no time of its own, so it is marked missing and its file is
  NOT_MEASURED, never a measured 0 ms (review P2-a, 2026-11-13). Playwright and Go contract units stay NOT_MEASURED: their contract parsers keep no such mark.
  Minitest runs its full command (RNC-V1-004), so its receipt outcome is NOT_MEASURED.
- `internal/testrunner` (`corvint-test-runner`): each observed test gains `durationMs`,
  `durationSource` and `attemptDurationsMs`, and the observation `durationInvalid`; an invalid
  time does not make the observation incomplete.
- Run evidence rows copy `durationMs`, `durationSource` and `durationGranularity` from the receipt
  (PTD-V1-008, RVD-V1-006). A flows report that converts a stored row with no `durationMs` marks
  the attempt not measured rather than reading its zero `duration_ms` as a time.
- Consumers: the cost figure (NMS-V1-002) and the fault-injection harness (FIH-V1-012) are not on
  this tree; `testduration.Cost` and `Median` are the rule they call (PTD-V1-009).

Per runner:

| Runner | Report field read | `corvint test` receipt | `corvint-test-runner` observation |
| --- | --- | --- | --- |
| Go | `go test -json` package-level `Elapsed` | measured, `go-test2json`, granularity `file` | not extended: the `go-test` plugin reports `RunnerResult`, which has no duration |
| Vitest | assertion `duration` | measured, `vitest-json`, granularity `file`: the sum of the provider receipt's test `durationMs` when one file is selected; NOT_MEASURED for several files, since a passing Vitest test names no file | measured, `vitest-json` |
| Jest | assertion `duration` | measured, `jest-json`, granularity `file`: the contract adapter's sum of the file's assertion durations; NOT_MEASURED when one is null or a selected test is NOT_RUN (V1-5542) | measured, `jest-json` |
| Playwright | sum of result `duration`, each attempt kept | measured, `playwright-json`, granularity `file`: the sum of the file's tests, each the sum of its `attemptDetails` | measured, `playwright-json`, with `attemptDurationsMs` |
| pytest | JUnit `testcase` `time` | measured, `pytest-junit`, granularity `file`: the contract adapter's sum of the file's testcase times (the call phase, `junit_duration_report=call`); NOT_MEASURED as Jest; a time that is not a duration is invalid, not a refusal of the report (V1-5542) | measured, `pytest-junit` |
| RSpec | JSON `run_time` | measured, `rspec-json`, granularity `file`: the contract adapter's sum of the file's examples' `run_time`; NOT_MEASURED when one has none (V1-5542) | measured, `rspec-json` |
| Minitest | reporter `DurationMs` from `Minitest::Result#time` | NOT_MEASURED: the contract has no Minitest selection (RNC-V1-004), so Minitest runs its full command, whose output the receipt does not read; a Minitest contract adapter belongs to change `ruby-providers` | measured, `minitest-reporter` |

## Traceability

| Requirement | Evidence |
|---|---|
| PTD-V1-001 | `TestPTDV1001ReceiptDurationFieldsRoundTrip`, `TestPTDV1001OlderReceiptsAndReaders`, `TestPTDV1001VitestProviderReceiptDuration` ("Vitest receipt", at granularity `file`) (`internal/sessiontest`), `TestRNCV1019ContractRunRecordsPerTestResults` (Jest, `internal/sessiontest`), `TestPTDV1005GoReceiptDurationAndInvocationTime` (`cmd/corvint`); the Minitest receipt is NOT_MEASURED (`TestPTDV1003MinitestFullCommandIsNotMeasured`) |
| PTD-V1-002 | `TestPTDV1002GoPackageElapsed`, `TestPTDV1002ProviderReceiptDurations` (`internal/sessiontest`), `TestPTDV1002RunnerReportDurations`, `TestPTDV1002MinitestReporterDuration`, `TestPTDV1002MinitestReporterOutput` (`internal/testrunner/dynamic`); the "Minitest receipt" scenario's receipt half is not yet built (`ruby-providers`); Jest, pytest and RSpec receipts (V1-5542): `TestPTDV1002ContractFileDurations`, `TestPTDV1002PytestReceiptDurations`, `TestRNCV1019ContractRunRecordsPerTestResults` (`internal/sessiontest`), `TestPTDV1007JestReportDurations`, `TestPTDV1002RSpecReportDurations`, `TestPTDV1002PytestJUnitTimes` (`internal/testrunner/contract`) |
| PTD-V1-003 | `TestPTDV1007InvalidAndMissingValues` (`internal/testduration`), `TestPTDV1001ReceiptDurationFieldsRoundTrip`, `TestPTDV1003BuildFailedPackageNotMeasured`, `TestPTDV1002ProviderReceiptDurations`, `TestPTDV1003VitestNativeReportWithoutDurationIsNotMeasured` (`internal/sessiontest`), `TestPTDV1003VitestMissingDurationIsMarked` (`internal/jstestprovider`), `TestPTDV1002RunnerReportDurations` (`internal/testrunner/dynamic`), `TestPTDV1008StoredRowWithoutDurationIsNotMeasured` (`internal/appflows`), `TestPTDV1002ContractFileDurations`, `TestPTDV1003FileLevelResultIsNotMeasured`, `TestPTDV1003MinitestFullCommandIsNotMeasured` (`internal/sessiontest`), `TestPTDV1002PytestJUnitTimes`, `TestPTDV1002RSpecReportDurations` (`internal/testrunner/contract`) |
| PTD-V1-004 | `TestPTDV1002GoPackageElapsed` (`internal/sessiontest`), `TestPTDV1005GoReceiptDurationAndInvocationTime` (`cmd/corvint`), `TestPTDV1002ContractFileDurations`, `TestPTDV1002PytestReceiptDurations` (`internal/sessiontest`) |
| PTD-V1-005 | `TestPTDV1005GoReceiptDurationAndInvocationTime` (`cmd/corvint`) |
| PTD-V1-006 | `TestPTDV1006AttemptSum` (`internal/testduration`), `TestPTDV1002ProviderReceiptDurations` (receipt `durationMs` sums the attempts) (`internal/sessiontest`); `attemptDurationsMs` is observation only: `TestPTDV1002RunnerReportDurations` (`internal/testrunner/dynamic`) |
| PTD-V1-007 | `TestPTDV1007InvalidAndMissingValues` (`internal/testduration`), `TestPTDV1001ReceiptDurationFieldsRoundTrip`, `TestPTDV1002ProviderReceiptDurations` (`internal/sessiontest`), `TestPTDV1002RunnerReportDurations`, `TestPTDV1002MinitestReporterDuration`, `TestPTDV1007NonNumericTimeKeepsTheReport` (Jest, Vitest, Playwright and RSpec time fields decoded on their own) (`internal/testrunner/dynamic`), `TestPTDV1007JestReportDurations`, `TestPTDV1002PytestJUnitTimes` (`internal/testrunner/contract`), `TestPTDV1002PytestReceiptDurations` (`internal/sessiontest`, listed under `durationInvalid`) |
| PTD-V1-008 | `TestPTDV1005GoReceiptDurationAndInvocationTime` (`cmd/corvint`), `TestRVDV1006RowsFromBothProfiles` (`internal/runevidence`), `TestPTDV1008StoredRowWithoutDurationIsNotMeasured` (`internal/appflows`) |
| PTD-V1-009 | `TestPTDV1009CostFigureUsesOnlyMeasured` (`internal/testduration`); the consumers themselves are not on this tree |

## Non-goals

No duration estimates, averages from history or invocation time split across tests; no use of
durations to narrow or order a selection in 1.0; no build or setup timing in the cost figure (G4);
no change to `Invocation.ElapsedMilliseconds`.

## Rollback

Revert the change's commits. Receipts already written keep the extra fields; readers that do not
know them ignore them. Consumers that need durations report them as `NOT_MEASURED`. PTD-V1-001 to
PTD-V1-009 stay listed as retired if the change is withdrawn and are never reused.
