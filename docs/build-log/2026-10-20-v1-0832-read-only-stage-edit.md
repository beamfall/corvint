# A read-only stage that edits the candidate waits for the operator (V1-0832)

Date: 2026-10-20 (lane dispatch-supervisor). Requirement: ATR-V0-170 in
`docs/specs/corvint-tasks-attempt-runner-v0.md`, accepted under the owner's standing auto-accept
(2026-10-11).

## Decision

The ticket is a confirmed defect. `Workflow.stage` returned "read-only stage changed candidate" as a
plain error before journaling the stop, so the attempt stayed `STOPPING` with `worker=true` and the
program kept its owner; a rerun was refused `RESOURCE_COLLISION`. The fix records the change, journals
the stop unaccepted with no claims and an operator question, finishes the program, and only then
returns a non-retryable `DIRTY_WORKTREE`. The existing supervisor transition (STOPPED with a question
goes to `WAITING`, worker false) carries the recovery, so `transaction/supervisor.go` is unchanged.
`SURVIVORS` from CAL-V0-086 still takes precedence, because the check runs only for a clean stop.

## Evidence

- `TestATRV0170_ReadOnlyStageEditIsTypedAndRecoverable` uses a new fixture flag, `review-edit`. Without
  the fix it fails with `MALFORMED read-only stage changed candidate` (the previous file was restored
  with a scratch copy; no stash). With the fix it passes, including the restore, answer and rerun to
  `READY_FOR_INTEGRATION`.
- Darwin run of `internal/tasks/store` with `-run 'TestCALV0086|TestCALV0197|Program|Workflow|TestATRV0170|Composite|ReadOnly'`
  passes (232 s). Linux: NOT_RUN.
