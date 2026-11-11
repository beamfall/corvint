# Per-test durations V1

Owner: Russell Lewis
Date: 2026-11-11
Intent status: accepted
Delivery status: not-started
Authoritative inputs: change folder `docs/specs/openspec/changes/per-test-durations/` (`proposal.md`,
`design.md`, `tasks.md` and `specs/per-test-durations/spec.md`), `docs/specs/PRODUCT-PLAN-1.0.md`
(PROD-TEST-016, PROD-TEST-020) and `docs/decisions/0596-accept-product-plan-1-0-2026-11-10.md`
(owner decisions R7, R19 and R38; G4).

## Agent digest
- Claim: Receipts record each test's measured time from the runner's own report and mark a missing or invalid time NOT_MEASURED, never estimated.
- Status: accepted / not-started. Change `per-test-durations` auto-accepted under G14 (decision 0596); delivered under V1-5449.
- Exists: nothing yet.
- Blocked on: nothing.
- Read next: Requirements.

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

Not yet built.

## Traceability

Not yet recorded.

## Non-goals

No duration estimates, averages from history or invocation time split across tests; no use of
durations to narrow or order a selection in 1.0; no build or setup timing in the cost figure (G4);
no change to `Invocation.ElapsedMilliseconds`.

## Rollback

Revert the change's commits. Receipts already written keep the extra fields; readers that do not
know them ignore them. Consumers that need durations report them as `NOT_MEASURED`. PTD-V1-001 to
PTD-V1-009 stay listed as retired if the change is withdrawn and are never reused.
