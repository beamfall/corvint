# V1-1558: contended store writes never report MALFORMED

Date: 2026-10-17. Base: `072a306f1dbd89c2f29b20a3434a0f25864ebbf7` (origin/main).
Ticket: V1-1558, from GitHub issue 1026. Spec: `docs/specs/corvint-tasks-attempt-runner-v0.md`,
ATR-V0-050..053, accepted under the owner's standing auto-accept of lane requirements
(2026-10-11). No accepted requirement is amended. The exit table rows for 127 and 1 gain
cross-references only.

## Problem

`wire.CodeOf` maps every uncoded error to `MALFORMED`. The authority lock functions return the
bare `context.DeadlineExceeded` or `context.Canceled` when the caller's context ends during a lock
wait, and the store write paths passed it through. The attempt runner gives its pre-launch writes
one 60-second budget, and the runner's lock wait is 30 seconds. Under heavy contention, the
heartbeat, read and renew together outlived that budget. The run then exited 1 with `MALFORMED`,
the warning `context deadline exceeded` and no item, as issue 1026 reports. The detached claim's
ceiling and an interrupt took the same path.

## Decisions

- Code. Reuse `LOCK_TIMEOUT`, with no new wire code. Every context check in a store write runs
  before that write commits, and request-id replay makes a same-id retry safe. Its retry condition
  now names an ended context. `wire.ContextEnded` wraps the coded error and the context error
  with two `%w`, so `errors.Is` callers, including the existing continuation and preparation-lock
  tests, still see the context error.
- Boundary. The mapping lives at `administrativeWriteWith`, which every store write passes
  through, and at the runner's pre-launch steps (which include `AttemptRecord` reads) and the
  lease command. `authority.AcquireLock` and `AcquirePreparation` are unchanged:
  `preparation_lock.go` and their tests rely on the bare context error.
- Outcome write. One request id, with retryable codes retried at a 250 ms backoff inside the one
  60-second write budget (`runWriteBudget`, which tests can shorten). The injected write fault
  still fails once, so the existing 127 tests keep their speed.
- Detached interrupt. The child now takes SIGINT/SIGTERM handling before any store work, rather
  than inside `leaseCommand`. An interrupt during its lock wait then keeps a `LOCK_TIMEOUT` result,
  where it used to kill the child (`SUPERVISOR_LOST`).

## Evidence

- The writer-contention tests (`internal/tasks/cli/attempt_contention_test.go`) re-execute the test
  binary as a helper process. The helper holds the store writer lock and the preparation gate,
  because lease writes commit under the gate on the writer-checkpoint route. Tests:
  - `TestATRV0050_ContendedPrelaunchWriteIsLockTimeout`
  - `TestATRV0051_ContendedTimedOutRunExits127WithOneItem`
  - `TestATRV0051_OutcomeWriteRetriesUntilTheLockFrees`
  - `TestATRV0053_DetachedClaimCeilingAndInterruptAreLockTimeout`
- Against origin/main 072a306f1, with only the write-budget test hook ported, three of the four
  fail:
  - pre-launch: `MALFORMED`, `context deadline exceeded`, items `[]`, which is the issue 1026
    symptom
  - timed-out run: exit 127 with `LIMIT_EXCEEDED` and `MALFORMED`
  - detached ceiling: `MALFORMED`, `context deadline exceeded`
  - detached interrupt: `SUPERVISOR_LOST`

  The retry test passes on both.
- `TestATRV0050_ContextEndedIsRetryableLockTimeout` (`internal/tasks/wire`) covers the mapping.
- Linux: NOT_RUN.

## Review fix: same-id retry of a kept LOCK_TIMEOUT (ATR-V0-066)

The coordinator review found a gap. A detached `claim` or `pool acquire` keeps its `LOCK_TIMEOUT`
in `result.json` (ATR-V0-053). The launcher returned early whenever `result.json` existed, so a
repeat `--detach` with the same request id replayed that timeout and never retried.

Decision: keep the result, so `--attach` still reports the retryable timeout. Before a relaunch:

- A launcher that finds a kept `LOCK_TIMEOUT` moves it to `result.1.json`, then starts a child.
- It does so only when no child holds the record's running lock.
- The store's request-ID replay keeps the claim exactly once.
- Any other kept result still answers verbatim.

The other option was a mandatory new request id. It was rejected because the `LOCK_TIMEOUT`
warning tells callers to retry with the same id.

Evidence:

- `TestATRV0066_KeptLockTimeoutRelaunchesOnSameRequestID` (`internal/tasks/cli`) covers three steps:
  1. A contended detached claim times out.
  2. Once the lock is released, the same-id `--detach` claims and the timeout is kept as `result.1.json`.
  3. A third `--detach` replays the OK result.
- The ATR-V0-020..029 and ATR-V0-053 detach tests pass with it, 3 times each under load.
- Linux: NOT_RUN.
