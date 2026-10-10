# 2026-10-09: takeover after an owner dies between dispatch and FINISHED (V1-0795)

## Intent

Native ticket V1-0795 carries a known limit retained by V1-0755 under CAL-V0-074. A supervised
program owner can die after a stage dispatch and before it writes the program's `FINISHED`
record. The program then stays `SPAWNING` or `STOPPING`. The attempt keeps its claim and
reservation, and a replacement owner was refused `FENCED` with `prior program owner not proved
stopped`. Expected: a replacement recovers the attempt without store surgery, and an unproved
stop still fails closed.

## Root cause

`planProgram` (`internal/tasks/transaction/program.go`) admits an owner change only from a safe
phase (`FINISHED`, `ADMITTED`, `WORKTREE_ADD`, `READY`) or as a worker recovery. Worker recovery
requires `PreviousWorkerClean`, which the store sets only for an attempt that still has a worker
and a retained leader boot record that `RecoverHost` proves stopped. A refused launch writes no
boot record. Its cancelling `NO_EXEC` settlement stops the attempt (`STOPPING`, then a clean
`STOPPED`) before it records the program `FINISHED`. In that window the attempt record already
proves the stop (no worker, no lane, quiescence `PROVED`), but no rule admitted it.

## Decision

The ticket left the choice open between a combined attempt+program transition and a takeover
rule. This lane takes the takeover rule, recorded as `CAL-V0-210` (accepted by decision 0473) (V1-0795;
`docs/specs/corvint-tasks-agent-leases-v0.md`). A combined transition would change the journal
write shape. The takeover rule reuses the existing program record and the existing fencing:
expected-inventory binding, `PreviousOwnerGone` from a dead owner identity, epoch plus one and an
unchanged worktree.

- Transaction (`planProgram`): an owner change from `SPAWNING` or `STOPPING` to `FINISHED` with
  proved quiescence is admitted when `BoundStoppedAttempt` holds for the attempt record that the
  program's current assignment names. That predicate requires the same attempt ID and generation,
  supervision by this program, no worker, no lane and quiescence `PROVED`. The transaction decides
  this from its own attempt state, not from the caller.
- Store (`programTransition`): when the prior owner is gone and no worker attempt was recovered,
  the store applies the same predicate to the proof's attempt record and rewrites the next phase
  to `FINISHED`/`PROVED`. A move from `STOPPING` whose result class is not `NO_EXEC` records
  `usageKnown=false`, because the stage's host output was lost. This matches the transaction's
  existing usage-derivation rule for an empty output.
- Unchanged: an attempt with a worker still needs `RecoveryBoot` and `RecoverHost`; any other
  attempt state is refused as before. `RUNNING`, the supervisor transitions and the attempt record
  are untouched.
- The non-cancelling `NO_EXEC` settlement (stage admission refused, prelaunch preparation failed)
  is intended to leave a dispatched attempt `WAITING`, with its claim held and the owner released,
  for the operator to answer, drain or cancel. This is now documented on `noExec` and in the
  amendment. Only a launch refusal, which no retry clears while the runtime stays unlaunchable,
  cancels.

A new test hook point, `dispatched`, runs before a cancelling settlement writes anything. It is
nil in the product.

## Evidence

- `TestCALV0074_NoExecCrashTakeover` (`internal/tasks/store`) now covers four crash points, each
  in a genuinely separate child process:
  - `dispatched`: the program is `SPAWNING` and the attempt has an unproved worker. The takeover
    is still refused (`recovery boot unavailable`), the program is unchanged and the claim is held
    (`ATTEMPT_LIVE`). This is the retained refusal.
  - `stopped`: the program is `STOPPING` and the attempt is `WAITING`, stopped. The takeover now
    settles `FINISHED`/`PROVED` at epoch plus one, with usage unobserved and counters unchanged.
    The replacement keeps the same attempt, cancels it, and the ticket is reclaimed.
  - `cancel` and `release`: the existing recoveries are unchanged. This change also tightens
    their post-takeover assertion: it previously checked the pre-takeover record.
- Fails on base: with only the updated test and the hook point applied to base `29cc7fd4`, the
  `stopped` case fails with `native transition refused ... Detail:prior program owner not proved
  stopped`. The other three cases pass there. With the change, all four pass.
- `go test -race -p 1 -count=1 -timeout 30m ./internal/tasks/transaction`: ok (52 s).
- `go test -race -p 1 -count=1 -timeout 30m -run 'TestCALV007[0-9]|TestCALV008[0-9]|TestCALV0197|Program|Supervis|Workflow|NoExec' ./internal/tasks/store`:
  ok (1428 s on a shared, loaded host). The whole `internal/tasks/store` package under `-race`
  exceeded the 30-minute per-package hang detector. It was still running
  `TestCALV0078_SupervisedRunAfterCommitIsNotRetryable` at the time, with no failure reported
  before the timeout. A full-package race run is therefore `NOT_RUN` to completion.
- The requirement, traceability, line-citation and receipt doc-gates passed, as did
  `go test ./internal/specindex`.

## Limits

- `CAL-V0-210` was accepted by the owner on 2026-10-09 (decision 0473). Live host qualification
  remains `NOT_RUN`.
- No transaction-level unit test forges a `FINISHED` takeover over an unproved attempt, because no
  public path reaches `planProgram` with one. The store refuses the worker case first. The
  transaction predicate is exercised only through the store test.
- Live Claude Code or Codex qualification: `NOT_RUN`. `make gate`: `NOT_RUN` (lane policy).
- Suspected defect, not changed and not reproduced: the existing worker-recovery path from
  `STOPPING` with a non-`NO_EXEC` result keeps `usageKnown` unchanged while sending no output. By
  reading, the transaction's usage-derivation rule would then refuse it `MALFORMED` whenever usage
  was known. It is reported for separate triage.
