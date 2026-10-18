# V1-1671: relaunch after a kept LOCK_TIMEOUT raced the finishing child

Date: 2026-10-18
Ticket: V1-1671 (no GitHub issue)
Requirement: ATR-V0-095 (`docs/specs/corvint-tasks-attempt-runner-v0.md`, section "V1-1671
relaunch after a kept LOCK_TIMEOUT"), accepted under the owner's standing auto-accept of lane
requirements (2026-10-11). It refines ATR-V0-066 and leaves PSR-V0-111 unchanged.

## Problem

`TestPSRV0111_CeilingRelaunchesOnSameRequestID` failed once on CI (PR #1059, run 39083146468, job
117544304684): the repeat `claim --detach` after the ceiling returned `SUPERVISOR_LOST` with state
`LOST` and the first child's pid, where ATR-V0-066 expects a relaunch.

## Root cause: a product race, not test timing

The detached child writes `result.json`, then its deferred release drops the running lock, then
the process exits and is reaped. The first `--detach --wait` returns as soon as `result.json`
reads, so a prompt repeat can land in either window:

1. Running lock still held: `leaseDetachKept` took the one-shot probe as "a child still runs the
   record" and returned the kept `LOCK_TIMEOUT`.
2. Running lock released, process not yet gone: `leaseDetachKept` moved the result aside, but
   `leaseDetachChildLive` then found the recorded pid alive under its recorded identity (on Linux a
   zombie still has `/proc/PID/stat`), so the launcher started nothing. The attach loop then saw no
   result, no running lock and, once the process was reaped, no live child, and reported `LOST`.

Window 2 is the CI failure. Evidence: injecting a 500 ms sleep after the child's running-lock
release reproduced the exact envelope (`SUPERVISOR_LOST`, state `LOST`, the first child's pid) in
3 of 3 runs on the base `d35b0ee33`. Without injection, 50 of 50 runs passed on an idle host; under
`heavy.sh` beside 16 CPU-bound processes on a 12-core host, 8 of 50 runs failed with
`SUPERVISOR_LOST`.

## Decision

`leaseDetachKept` waits up to the launch-lock bound (10 s) for the running lock when it finds a kept
`LOCK_TIMEOUT`, moves the result while it holds that lock, and reports the move. After a move the
launcher starts a new child without the ATR-V0-027 liveness check: the recorded child released the
running lock after keeping the result and only exits. The test was not changed; synchronising it on
the recorded state would have hidden a race that any caller can hit.

## Evidence

- `TestATRV0095_RelaunchWaitsOutTheFinishingChild` holds the running lock for 300 ms and keeps
  every recorded child visible under its identity. It passes with the fix and fails on the base
  with the kept `LOCK_TIMEOUT`.
- With the fix, the 500 ms post-release and pre-release injections both pass 3 of 3.
- Under `heavy.sh` beside the same 16 CPU-bound processes, `-count=50` of
  `TestPSRV0111_CeilingRelaunchesOnSameRequestID` and of `TestATRV0095_RelaunchWaitsOutTheFinishingChild`
  passed 50 of 50 each (base: 42 of 50).

Rollback: revert the commit. Records and wire shapes are unchanged.

## Review fix

Codex review of PR #1095 (head `a2f91ff20`) raised two P2 findings, both fixed in an added commit:

1. The relaunch's running-lock wait always used 10 s, and the `--wait` deadline only started after
   it, so `--detach --wait 0` could block about 10 s, against ATR-V0-024. One deadline, set when
   the command starts, now bounds both the running-lock wait (at most 10 s, never past the
   caller's `--wait`) and the wait for the result. When it expires with the lock still held,
   nothing is moved or started and the kept `LOCK_TIMEOUT` answers verbatim. ATR-V0-095 is amended
   in place to say so, under the same standing auto-accept (2026-10-11); ATR-V0-024 is unchanged.
   `TestATRV0095_RunningWaitIsBoundedByWait` holds the running lock and checks that `--wait 0` and
   no `--wait` return the kept result at once and `--wait 1` within its bound, with no
   `result.1.json`, then that the same `--detach` relaunches once the lock frees. On the previous
   head it fails: `--wait 0` returned after 10.0 s.
2. `TestATRV0095_RelaunchWaitsOutTheFinishingChild` took the running lock with one nonblocking
   `flock` right after the first `--detach` returned, racing the real child's deferred unlock. It
   now acquires the lock with a bounded retry (10 s deadline, `acquireRunning`) before holding it.

A concurrent launcher of the same record still waits up to 10 s for the launch lock outside the
caller's `--wait`; this predates ATR-V0-095 and is left as a residual.

Evidence: under `heavy.sh`, `-count=20` of `TestPSRV0111_CeilingRelaunchesOnSameRequestID` and both
`TestATRV0095_` tests passed 60 of 60; the focused detach set (ATR-V0-021, 023, 024, 066, 095,
PSR-V0-111) passed.
