# Spec delta: per-test-durations

Capability spec: `docs/specs/per-test-durations-v1.md` (new). This delta is a proposal only; the
canonical spec is updated in the same change (G14).

## ADDED Requirements

### Requirement: PTD-V1-001 Duration fields on every per-test outcome
Source: owner decision R19 (2026-11-09).
Status: proposed 2026-11-09; delivery not-started.
Every per-test outcome in a `corvint test` receipt MUST carry `durationSource`, and MUST carry
`durationMs` (integer milliseconds) when the duration was measured. The fields MUST be add-only.

#### Scenario: Vitest receipt
- **GIVEN** a Vitest run where `math > adds` passed in 3 ms
- **WHEN** the receipt is written
- **THEN** its outcome has `durationMs` 3 and `durationSource` `vitest-json`

### Requirement: PTD-V1-002 One named source per runner
Source: owner decision R19 (2026-11-09); G4 (owner answer).
Status: proposed 2026-11-09; delivery not-started.
Each runner MUST take durations from its own report: Go from `go test -json` `Elapsed`, Vitest and
Jest from the JSON report `duration`, Playwright from the sum of attempt durations, pytest from the
JUnit `time` attribute, RSpec from the JSON formatter `run_time`, and Minitest from a reporter field
filled from `Minitest::Result#time`.

#### Scenario: Minitest receipt
- **GIVEN** a Minitest run through Corvint's reporter
- **WHEN** the receipt is written
- **THEN** each Minitest outcome has `durationMs` and `durationSource` `minitest-reporter`

### Requirement: PTD-V1-003 Missing durations are not estimated
Source: every unknown is named; product invariant 2.
Status: proposed 2026-11-09; delivery not-started.
When a runner's report has no duration for a test, the outcome MUST omit `durationMs` and set
`durationSource` to `NOT_MEASURED`. Corvint MUST NOT estimate, average or copy a duration from
another run.

#### Scenario: report without times
- **GIVEN** a pytest JUnit report whose `testcase` elements have no `time`
- **WHEN** the receipt is written
- **THEN** each pytest outcome has `durationSource` `NOT_MEASURED` and no `durationMs`

### Requirement: PTD-V1-004 File-granularity durations are marked
Source: owner decision R19 (2026-11-09).
Status: proposed 2026-11-09; delivery not-started.
An outcome recorded per file rather than per test MUST carry `durationGranularity: file`.

#### Scenario: legacy package unit
- **GIVEN** an outcome for legacy unit `go:./internal/x`
- **WHEN** the receipt is written
- **THEN** it carries the package duration with `durationGranularity: file`

### Requirement: PTD-V1-005 Invocation time is not divided among tests
Source: G4 (owner answer: excluding build and setup).
Status: proposed 2026-11-09; delivery not-started.
`Invocation.ElapsedMilliseconds` MUST keep measuring the whole launch and MUST NOT be divided among
tests or used as a per-test duration.

#### Scenario: Go run with build time
- **GIVEN** a Go run whose build took 40 seconds and whose tests took 2 seconds
- **WHEN** the receipt is written
- **THEN** the per-test durations sum to about 2 seconds and the invocation time includes the build

### Requirement: PTD-V1-006 Retries record each attempt
Source: owner decision R19 (2026-11-09).
Status: proposed 2026-11-09; delivery not-started.
A test run more than once in one invocation MUST record `attemptDurationsMs` and a `durationMs` equal
to their sum.

#### Scenario: Playwright retry
- **GIVEN** a Playwright test that failed in 900 ms and passed on retry in 700 ms
- **WHEN** the receipt is written
- **THEN** `attemptDurationsMs` is [900, 700] and `durationMs` is 1600

### Requirement: PTD-V1-007 Invalid values are named
Source: every unknown is named.
Status: proposed 2026-11-09; delivery not-started.
A negative, non-numeric or above-bound duration MUST be recorded as `NOT_MEASURED`, and the receipt
MUST list the test under `duration-invalid`.

#### Scenario: negative duration
- **GIVEN** a Jest report with `duration` -5 for one test
- **WHEN** the receipt is written
- **THEN** that test is `NOT_MEASURED` and listed under `duration-invalid`

### Requirement: PTD-V1-008 Run evidence carries durations
Source: owner decision R7 (2026-11-09); PROD-TEST-020.
Status: proposed 2026-11-09; delivery not-started.
Each run evidence row (RVD-V1-006) MUST copy `durationMs`, `durationSource` and, when present,
`durationGranularity` from the receipt.

#### Scenario: stored run
- **GIVEN** a `corvint test` run with measured durations
- **WHEN** its record is appended to the run evidence store
- **THEN** each row holds the test's duration and source

### Requirement: PTD-V1-009 Consumers use only measured durations
Source: owner decision R19 (2026-11-09); G4 (owner answer).
Status: proposed 2026-11-09; delivery not-started.
The cost figure (NMS-V1-002) and the fault-injection harness (FIH-V1-012) MUST use only measured
per-test durations and MUST report the number of selected and full-suite tests that were
`NOT_MEASURED`. A figure with any unmeasured test MUST be labelled incomplete.

#### Scenario: one unmeasured test
- **GIVEN** a full baseline run where one test is `NOT_MEASURED`
- **WHEN** the median selected time is computed
- **THEN** the figure is labelled incomplete and names the unmeasured count

## MODIFIED Requirements

None.

## REMOVED Requirements

None.
