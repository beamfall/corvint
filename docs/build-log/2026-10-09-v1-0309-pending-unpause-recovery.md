# V1-0309: supported recovery of a pending UNPAUSE receipt

Ticket V1-0309 (panel F2, audit addendum STO-01). Base `29cc7fd4e257ae1549848fc165ac98f4a70109d9`.

## Prior work found

- Issue 433 (`2026-10-01-tasks-read-waits-for-in-flight-writer.md`, `CTS-V0-006`): reads now wait
  out a writer's §5.2 pending window with a two-second patience and name the wait as retryable.
  This covers the finding's concurrent-writer symptom without a reader lock; it does not add the
  `WRITER_ACTIVE` code the finding proposed.
- V1-0325 (`2026-10-04-v1-0325-intent-worktree.md`, `CTW-V0-002`..`CTW-V0-007`): a writer on a
  feature branch routes intent writes, and the redo inside `Mutate`, to a linked intent-branch
  worktree, and `Mutate` refusals carry the repair text. Barrier and redo keep their text
  without the repair (`CTW-V0-007`).
- Nothing settled a pending receipt whose post deletes the barrier: `redoPosts` refused it as
  `UNSUPPORTED`, and `Barrier` never ran redo, so retrying the unpause returned `REDO_PENDING`.

## Decision

- Redo now settles the receipt-authorized barrier deletion after the other posts and before
  the head, matching first-write order. It deletes only the exact pre bytes under the session
  CAS and syncs the deletion. An already absent barrier is accepted as the post state and synced
  as absent. A third value is refused as `JOURNAL_FORKED` with nothing written.
- `Barrier` runs §5.2 redo under its own writer guards before the request lookup, as `Mutate`,
  `MutateBatch`, `PolicyUpdate`, `Release` and `Import` already do. An ALL barrier admits
  UNPAUSE, so retrying the same unpause settles the pending receipt and then replays it. As a
  result, `Barrier` no longer refuses an unrelated pending receipt; it settles it first.
- The intent-branch guard in redo now applies only when the pending receipt posts an `intent/`
  path. A receipt that writes only state-directory files, such as UNPAUSE, is settled from any
  branch, as the barrier command already is. Receipts that write the intent projection keep the
  guard (`TestTMV0009_AS11_WriterGuardsBeforeRedo` branch/pending still passes).
- Rejected: a new top-level `recover` verb. The closed command set and help profile would need
  a contract amendment, which needs an owner decision. Existing writers are the supported
  recovery path.

## Evidence

`internal/tasks/store/barrier_redo_test.go`:

- `TestV10309_PendingUnpauseRecovers` covers 6 cases. It runs ADMISSION and ALL barriers, with a
  fault before or after the barrier unlink (before the head), and recovers by barrier retry or
  by mutation. Each case checks that the identical receipt settles the head, that the barrier is
  gone, that the audit passes and is not pending, and that a second recovery leaves the store
  digest unchanged.
- `TestV10309_PendingUnpauseRecoversOffIntentBranch`,
  `TestV10309_BarrierSettlesPendingMutation`: both fail on base.
- `TestV10309_PendingUnpauseChangedBarrierRefuses` and
  `TestV10309_PendingUnpauseAfterHeadReplays` pass on base and after the change. They guard the
  refusal and the settled case.

On base, all 6 `PendingUnpauseRecovers` subtests fail with `REDO_PENDING` or
`UNSUPPORTED ... redo of a deletion`, and so do the off-branch and pending-mutation tests. All
pass after the change. Two existing assertions recorded the old limitation and were updated:
`TestTMV0009_AS11_BarrierPublicationReturnedFaults/predelete` now expects the retry to settle
and replay, and the `pending` case moved out of `TestTMV0009_AS11_BarrierRefusalsPreserveStore`.

## Limits

- "After head publication" is tested as the settled state. No returned fault after the head
  rename was injected; the store package has no hook there. No process-kill or power-cut run
  was made (`NOT_RUN`).
- The `WRITER_ACTIVE` reader code and a dedicated `recover` verb from the finding's Fix are not
  implemented. Issue 433 covers the reader symptom. The verb is left for an owner decision.
- `make gate`: `NOT_RUN` (lane policy). Focused `internal/tasks/store` and
  `internal/tasks/cli` barrier tests were run.
