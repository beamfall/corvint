# V1-1551: pool sweep prepare step (PSR-V0-024..026)

Date: 2026-10-17. Ticket V1-1551, GitHub #1009. Spec:
`docs/specs/corvint-tasks-pool-safe-reuse-v0.md`. PSR-V0-024..026 are accepted under the owner's
standing auto-accept of lane requirements (2026-10-11).

## Decision

`memberConfig[MEMBER].prepare` is an optional pool command with the health/cleanup shape. It requires
`safeReuse`, because only `pool sweep` runs it. The sweep phase order is cleanup, reset, verify,
prepare, confirm. Prepare is a new owned phase in the existing sweep state machine, not a separate
command after confirmation, for these reasons:

- It runs while the sweep still owns the selected quarantined allocation (CLEANING), so no claim can
  race it. It shares the member's attempt deadline and the sweep budget.
- Pending replay and reconciliation read its child receipt like every other phase. A completed
  prepare is never re-executed.
- Confirmation requires the passed prepare observation as the owned witness. That observation can
  only follow a passed verify.

Failure is terminal. A prepare is not retried even when `maxAttempts` remains. The member stays
QUARANTINED with the reason `CLASS; pool prepare failed`. The issue asked that a failed prepare
leave the member quarantined, and a retry would rerun reset against data that is half prepared.

The issue asked for digest-only output, the same as health/cleanup. Prepare records stdout/stderr
digests, and its sweep log evidence is the empty log. The writer refuses a prepare observation whose
log retains bytes. Reset and verify keep their private logs, unchanged.

Under an ALL barrier, prepare counts as a further phase command, so it never launches. A sweep that
has passed verify ends quarantined with the ALL-barrier reason.

The per-member owned-step loop bound rises from 7 to 8: two cleanup/reset/verify attempts, then
prepare and confirm. Members without prepare never reach the eighth step.

## Compatibility

Snapshot enums for the owner and observation phases gain `prepare`. Policies, pool state and
observations without prepare keep their bytes. Older decoders refuse records that carry prepare,
and that refusal is the recorded compatibility boundary.

## Evidence

- `internal/tasks/intent/pool_prepare_test.go`: `TestPSRV0024PrepareConfig`
- `internal/tasks/store/pool_prepare_test.go`: `TestPSRV0025PrepareRunsOwnedBeforeConfirm`,
  `TestPSRV0025PrepareNeverLaunchesUnderAll`, `TestPSRV0026PrepareFailureStaysQuarantined`,
  `TestPSRV0026PendingReplayNeverRerunsPrepare`
- The existing pool/sweep store tests ran unchanged.
