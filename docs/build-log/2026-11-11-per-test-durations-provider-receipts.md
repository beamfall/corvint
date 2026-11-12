# 2026-11-11: per-test durations in provider receipts and the review repairs (V1-5449)

Change `per-test-durations`, task 3 in part. Requirements PTD-V1-001 to PTD-V1-004, PTD-V1-006 to
PTD-V1-008 in `docs/specs/per-test-durations-v1.md`. Follows `2026-11-11-per-test-durations.md`.

## Decisions

1. The provider receipt corvint-js-test-provider prints already names each test's `durationMs`
   and each Playwright attempt's in `attemptDetails`; the earlier entry's statement that it names
   no per-test time was wrong. A Vitest or Playwright file unit of a `corvint test` receipt now
   records the sum of its tests' times, granularity `file`, as the Go package `Elapsed` is.
2. A Vitest passing test names no file. With one selected file every test is its own; with more,
   the file sums are NOT_MEASURED rather than guessed (PTD-V1-003).
3. A retried Playwright test counts the sum of its attempt details. One with retries but only its
   last attempt's time is NOT_MEASURED. The receipt outcome carries no `attemptDurationsMs`, since
   it sums many tests; that member is observation only, on `corvint-test-runner` (PTD-V1-006).
4. Jest, pytest, RSpec and Minitest run through the declared-command adapter in `corvint test`,
   which reads no report. Their receipt adapters belong to changes `jest-provider`,
   `pytest-provider` and `ruby-providers`; none is built here.

## Independent review findings repaired

- A Go package whose build failed was recorded with a measured time from its `fail` event. Only a
  package whose tests ran (PASS, FAIL, TIMED_OUT) is measured now.
- A stored receipt row with no `durationMs` became a flows attempt with a `duration_ms` of 0 that
  read as measured. The attempt is now marked not measured.
- The Minitest reporter line is reduced to one guarded assignment from `Minitest::Result#time`
  (Ruby is not installed on the delivering host, so it is not executed), and a Go test pins the
  reporter source and parses its exact output shape.
