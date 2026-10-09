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
  the head, matching first-write order. Present bytes are unlinked only after the session's
  identity and digest check under the lock finds the pre digest, and the deletion is synced. The
  unlink itself is by name, not bound to the checked inode, so this is cooperative-lock
  protection, not an exact-byte compare-and-swap: a non-cooperating editor that replaces the name
  after the last check is not excluded, as the original barrier contract already states for
  hostile editors. An already absent barrier is accepted as the post state and synced as absent.
  A third value, seen before the removal or by the removal's own check, is refused as
  `JOURNAL_FORKED` with the barrier and head unchanged.
- Review repair: redo of a pending UNPAUSE receipt that posts no `intent/` path now uses
  `journal.Reader.PendingBarrierRemoval`, the fresh UNPAUSE's `TICKETS_NOT_COMPARED` audit with
  the one pending receipt admitted. Before effects it rebinds by a second audit of the same mode
  instead of the intent tree digest, which refuses the empty or malformed ticket edits the fresh
  UNPAUSE admits. Every other pending receipt keeps the strict `PRE_OR_POST` audit.
- Contract: the amendment of the TCP-00 settled-barrier clause ("Pending receipts refuse without
  redo") is recorded as the proposed spec
  `docs/specs/corvint-tasks-pending-unpause-recovery-v0.md` (`PUR-V0-001`..`005`), accepted by
  decision 0478.
- `Barrier` runs §5.2 redo under its own writer guards before the request lookup, as `Mutate`,
  `MutateBatch`, `PolicyUpdate`, `Release` and `Import` already do. An ALL barrier admits
  UNPAUSE, so retrying the same unpause settles the pending receipt and then replays it. As a
  result, `Barrier` no longer refuses an unrelated pending receipt; it settles it first.
- The intent-branch guard in redo now applies only when the pending receipt posts an `intent/`
  path. A receipt that writes only state-directory files, such as UNPAUSE, is settled from any
  branch, as the barrier command already is. Receipts that write the intent projection keep the
  guard (`TestTMV0009_AS11_WriterGuardsBeforeRedo` branch/pending still passes).
- Not done, accepted by decision 0478: a new top-level `recover` verb (the closed command set and help
  profile would need a contract amendment) and the `WRITER_ACTIVE` reader code. Existing writers
  are the supported recovery path.

## Evidence

`internal/tasks/store/barrier_redo_test.go`:

- `TestV10309_PendingUnpauseRecovers` covers 6 cases. It runs ADMISSION and ALL barriers, with a
  fault before or after the barrier unlink (before the head), and recovers by barrier retry or
  by mutation. Each case checks that the identical receipt settles the head, that the barrier is
  gone, that the audit passes and is not pending, and that a second recovery leaves the store
  digest unchanged.
- `TestV10309_PendingUnpauseRecoversOffIntentBranch`,
  `TestV10309_BarrierSettlesPendingMutation`: both fail on base.
- `TestV10309_PendingUnpauseChangedBarrierRefuses` installs a canonical, structurally valid
  barrier that differs only in its actor and asserts `JOURNAL_FORKED` with the barrier and head
  unchanged. It and `TestV10309_PendingUnpauseAfterHeadReplays` pass on base and after the
  change; they guard the refusal and the settled case.
- `TestV10309_PendingUnpauseBarrierChangedBeforeRemoval` writes that third value through a
  test-only hook (`redoBarrierObserved`, nil in production) between redo's observation and the
  removal, and asserts `JOURNAL_FORKED`, the third value preserved and the head unchanged. It
  fails on base, where redo never reaches a removal.
- `TestV10309_PendingUnpauseDivergentTicketsRecover` (ADMISSION and ALL, before the unlink and
  before the head) edits two canonical tickets to empty and malformed bytes while paused, then
  interrupts the UNPAUSE. Retry settles the identical receipt, preserves the edited bytes and
  replays; a second retry changes nothing. All 4 subtests fail on base 96d595f7 with
  `INTENT_DIVERGED: intent/tickets/AT-0002.json`.
- Independent reproduction, 2026-10-09: with the 3726b479 test files and a nil
  `redoBarrierObserved` shim on 96d595f7, `TestV10309_PendingUnpauseBarrierChangedBeforeRemoval`
  fails ("redo never reached the barrier removal") and
  `TestV10309_PendingUnpauseDivergentTicketsRecover` fails (`INTENT_DIVERGED` ... `AT-0002.json`
  projection differs). On 3726b479 all seven `TestV10309` tests pass under `-race`, and
  `internal/tasks/journal` and `internal/tasks/snapshot` pass. A Codex re-review found no P1, P2
  or P3 code defects; it withheld PASS only because it did not execute tests.

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
  implemented; decision 0478 accepts that. Issue 433 covers the reader symptom.
- Barrier deletion is cooperative-lock protection only (see Decision). Hostile-editor
  qualification is `NOT_RUN`, as in the original barrier contract.
- `PUR-V0` is accepted by decision 0478 (owner decisions on the `recover` verb, `WRITER_ACTIVE`
  and the `Barrier` behaviour change included); the external TCP-00 wording edit is not made and
  remains open. Live qualification is `NOT_RUN`.
- `make gate`: `NOT_RUN` (lane policy). Review repair runs: `internal/tasks/journal` and
  `internal/tasks/cli` full packages pass. The full `internal/tasks/store` package hit the 30 m
  per-package timeout on a host shared with other lanes; the panic fired in an unrelated lease
  test, with no assertion failure, so the full store result is `NOT_PASSED (timeout)`. The
  barrier, redo, pending, guard, reconcile, `TMV0006`, `TMV0009` and `TMV0016` store tests and
  `internal/tasks/journal` pass under `-race`.
