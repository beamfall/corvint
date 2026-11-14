# Change proposal: per-test durations in every receipt

Change name: `per-test-durations`
Ticket: none filed yet; to be filed in `.taskman/` before dispatch (not filed by this proposal).
Product requirement served: PROD-TEST-020 (run result record) and the G4 cost figure in
`docs/specs/PRODUCT-PLAN-1.0.md`.
Owning capability spec: a new spec `docs/specs/per-test-durations-v1.md` (prefix `PTD-V1`). It does
not exist yet; the lane creates it in its first commit.
Requirement IDs this change adds: PTD-V1-001 to PTD-V1-009. `PTD-` has no entry in
`docs/specs/REQUIREMENTS.tsv` and appears in no other folder under `docs/specs/openspec/changes/`
(checked 2026-11-09).
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes.

## Needs first

- `canonical-test-id`: a duration is recorded per canonical test ID.

It lands with `run-evidence`, whose rows carry the duration. Changes `fault-injection-harness`
(FIH-V1-012) and `narrowing-measurement` (NMS-V1-002) consume it.

## Why

Owner decision R19 (2026-11-09) requires "per-test durations in every runner's receipt". G4 (owner
answer) measures median selected time from recorded per-test durations, excluding build and setup:
Go, Vitest, Jest and pytest must narrow at least 60 percent of PRs with median selected time at most
50 percent; Playwright, RSpec and Minitest publish their numbers with no bar.

### What exists at integ 9ec3a29d0

- The `corvint test` receipt records elapsed time per adapter invocation
  (`ElapsedMilliseconds` on `Invocation`) and nothing per test: an `Outcome` holds only outcome,
  slice and unit (`internal/sessiontest/run.go`, lines 71-92 and 739, integ 9ec3a29d0).
- The generic runner adapter's test record has no duration field
  (`internal/testrunner/types.go`, lines 140-150, integ 9ec3a29d0), and its Jest and Vitest
  assertion record does not read `duration` (`internal/testrunner/dynamic/parse.go`, lines 185-189,
  integ 9ec3a29d0).
- `flows ingest` already keeps a duration per attempt for Playwright results
  (`internal/appflows/runingest.go`, lines 135, 308 and 400, integ 9ec3a29d0).
- The Ruby line-collection hook records elapsed nanoseconds per test for RSpec, Minitest and
  test-unit (`internal/rubylinecov/linecov.rb`, line 140, integ 9ec3a29d0). The Minitest reporter
  used to run tests records no time (`internal/testrunner/dynamic/reporters/minitest_reporter.rb`,
  integ 9ec3a29d0).

## What changes

- Each per-test receipt outcome gains add-only `durationMs` and `durationSource` (PTD-V1-001).
- One named source per runner, read from the runner's own report (PTD-V1-002).
- A missing duration is absent and marked `NOT_MEASURED`, never estimated (PTD-V1-003).
- File-granularity outcomes carry the file's duration and are marked as such (PTD-V1-004).
- Invocation elapsed time stays separate and is never divided among tests (PTD-V1-005).
- Retried tests record each attempt and the sum (PTD-V1-006).
- Invalid values become `NOT_MEASURED` with `duration-invalid` (PTD-V1-007).
- Run evidence rows copy the duration (PTD-V1-008).
- The cost figure and the fault-injection harness read only recorded durations and report coverage
  (PTD-V1-009).

## Non-goals

- No duration estimates, averages from history or invocation time split across tests.
- No use of durations to narrow or order a selection in 1.0.
- No build or setup timing in the cost figure (G4).
- No change to `Invocation.ElapsedMilliseconds`.

## Impact

- New spec `docs/specs/per-test-durations-v1.md`; generated rows in `docs/specs/REQUIREMENTS.tsv`.
- Receipt schema: add-only fields on per-test outcomes (D25 add-only rule).
- Each runner adapter reads its runner's duration field; the Minitest reporter gains a time field.
- Changes `run-evidence`, `fault-injection-harness`, `narrowing-measurement` and `jest-provider` cite
  these fields.
