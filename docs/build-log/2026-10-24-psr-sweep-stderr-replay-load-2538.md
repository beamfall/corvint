# 2026-10-24 PSR sweep stderr and replay tests under CPU contention (V1-2538)

Base `025120e1a7f14bce336ebe401ef6f2f99e8a3cb1`. Linux/amd64 cloud container, 4 CPUs. The load came
from `nproc*2` (8) `yes > /dev/null` burners at nice 0. They were started and stopped by the same
command that ran the tests at `nice -n 15`. Only `internal/tasks/store/pool_sweep_test.go` changed;
no product deadline, budget or allowance was changed.

## Root cause

- `TestPSRSweepStderrIsNotStdout` and `TestPSRSweepSuccessReplaySuccessor` used `psrFixture` and
  `psrRun`. That means the fixture's fixed 3 s safe-reuse attempt bound and the product's fixed
  observation bounds (a 15 s sweep budget scales them only 1.5x, PSR-V0-023).
- One attempt runs the reset and verify commands, plus the cleanup command in the replay test.
  Around each phase it makes three Git source observations before and three after. Every one of
  those processes is a bounded child with its own process-group cleanup proof. That is about a
  dozen spawns plus `ps`/`kill` probes inside 3 s.
- Under CPU contention the attempt deadline expired first. Verify recorded `TIMEOUT` instead of
  `STDOUT_MISMATCH`, and the replay test's member stayed quarantined ("not freed"). Neither test
  is about those bounds: `TestPSRDeadlineRetryMatrix` and the probe tests own them.
- No child waits on a lock or an undrained pipe. Every observed failure was the attempt
  `TIMEOUT`.

## Fix

- Both tests use the V1-1324 load-tolerant bounds that their sibling tests already use:
  `psrFixtureConfigured(..., psrLoadTolerant)`, which sets a 120 s attempt bound, and
  `psrRunLoaded`, which sets a 300 s sweep budget and the lifted observation bounds. Each test
  waits on the sweep's own bounded return.
- The replay runs with the same budget as the original sweep, so the request digest, and with it
  the replay, is unchanged.
- No assertion changed. The "not freed" failure now prints `psrEntryStates`, so a remaining
  host-load failure is classified (V1-1153).
- Remaining quiet-host assumption: unchanged from V1-1324. The group-cleanup proof (2 s
  liveness loop, 1 s pipe drain, 1 s `WaitDelay`) is never lifted.

## Evidence

See the PR body for the exact commands and counts.
