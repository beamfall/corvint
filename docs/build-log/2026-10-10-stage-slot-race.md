# V1-0825: unlocked audits and export wait out a writer's staging slots

Date: 2026-10-10

## Problem

CI run 38058826139 (job 114232890487) failed `TestCALV0089_DrainStopsContinuation/at_the_wall`
with `MALFORMED: staging/a00: unassigned stage slot`. The owner hook read `ProgramRecords`
(`readLeaseProof`, an unlocked journal audit) while `RequestProgramControl` was committing the
DRAIN transition under the writer lock.

Root cause: the native writer publishes through staging slots that have no `staging/active.json`
descriptor. `publish` in `internal/tasks/store/store.go` and `advanceHead` in
`internal/tasks/store/redo.go` prepare `staging/aNN`, link or rename it into place, and remove a
linked slot in a deferred `RemoveStage`. The slots therefore exist, unchanged, for most of the
publish (fsyncs, the linked window). `readStage` in `internal/tasks/journal/stage_read.go` refused
any slot without a descriptor as `MALFORMED`. A slot stable across both of an audit's captures
passed `sameObservation`, so `audit` returned the refusal instead of `SNAPSHOT_MOVED` and nothing
retried it. Only a slot that appeared or vanished between the captures was retried.

## Decision

This adds `CTS-V0-008` (proposed) to `docs/specs/corvint-tasks-store-init-v0.md` beside the
CTS-V0-006 read wait. It also amends the leases spec's killed-writer failure row and the V1-0772
non-goal to point at it.

- `readStage` marks the unassigned-slot refusal as `slotsInFlight` only when there is neither
  `active.json` nor `active.json.tmp`. That is the native writer's shape, and also a killed writer's
  orphan shape. A slot beside a descriptor or temp is still refused at once.
- `audit` (all `Audit`, `AuditForWrite`, `RequestIndex.Lookup` and mutation-audit reads) pauses on
  `slotsInFlight` with the CTS-V0-006 backoff (25 ms doubling to 400 ms) and audits again, up to
  `stagePatience` (2 s) of pauses. It then returns the wrapped `*wire.Error` unchanged: same code,
  path and text. Moved attempts keep their own four-attempt bound.
- The backoff and budget are `snapshot.StageSlotWait` and its constants, shared with the archive
  export, not `snapshot.DefaultPatience`. The fixture package zeroes that variable in every test
  binary, and the store tests that flaked are test binaries.
- A caller that holds the writer lock sets `journal.Reader.WriterLocked` through the store's
  `lockedJournalReader` (reconcile, barrier, policy, release, import, `mutateLocked` and redo). No
  other writer can be publishing then, so the audit refuses descriptor-less slots at once instead
  of holding the lock for the budget. Unlocked lease, preparation, checkpoint-refresh, pool-sweep
  replay and cli reads keep waiting.
- `archive/stage_read.go` marks the same shape and `readArchive` waits for it with the same
  `StageSlotWait`. A pause spends neither one of its four moved attempts nor its CTS-V0-006
  deadline, and the slot's `MALFORMED` (at `aNN`, as before) is returned unchanged, without the
  CTS-V0-006 wait annotation, once the budget is spent.
- The writer-checkpoint audit (`AuditForWriter`) is unchanged: it already declines staging as a
  checkpoint miss.

Cost: a store with real orphan slots now takes 2 s longer to refuse an unlocked read or an export.
Writes do not wait: they either clear orphans under the lock first (CAL-V0-019) or audit as
writer-locked and refuse at once, as before.

## Evidence

- `TestCTSV0008_AuditWaitsForWriterHeldStageSlot` uses the capture hook to plant a descriptor-less
  slot between the first attempt's captures and holds it across the second. The sleep hook removes
  it during the single pause. Against the unfixed `audit.go`/`stage_read.go`, the test fails with
  the CI text `MALFORMED: staging/a00: unassigned stage slot`. With the fix it passes.
- `TestCTSV0008_OrphanStageSlotStillMalformedAfterBoundedWait`: orphans `a03` and `a00` still
  refuse `MALFORMED` at `staging/a00` as a plain `*wire.Error`, after pauses summing to exactly 2 s.
- `TestCTSV0008_UnassignedSlotBesideDescriptorRefusedAtOnce`: zero pauses beside `active.json` or
  `active.json.tmp`.
- `TestCALV0114_DivergentProjectionRefusalIsDeterministic` stubs the sleep. Its orphan case is
  otherwise unchanged.
- `TestCTSV0008_WriterLockedAuditRefusesOrphanSlotWithoutPause`: `Audit`, `RequestIndex.Lookup`
  and `AuditForMutation` on a writer-locked reader make zero pauses; a moved first observation is
  retried once and the stable orphan is then refused. It fails without the `WriterLocked` check.
- `TestCTSV0008_LockedReconcileRefusesOrphanSlotWithoutWaiting`: `store.Reconcile` beside an
  orphan refuses `MALFORMED` at `staging/a00` in about 0.2 s; with `reconcile.go` reverted to the
  unlocked reader it took 2.1 s and failed.
- `TestCTSV0008_MovedObservationsAndSpentWaitCompose` (journal) and
  `TestCTSV0008_ArchiveMovedAttemptsAndSpentWaitCompose`: moves while waiting, after the budget is
  spent, and across both; the pauses still sum to exactly 2 s, a move after the budget costs no
  pause, and four moves in all end in `SNAPSHOT_MOVED`.
- `TestCTSV0008_ArchiveWaitsForWriterHeldStageSlot`, `..._ArchiveOrphanSlotMalformedAfterBoundedWait`
  and `..._ArchiveSlotBesideDescriptorRefusedAtOnce` mirror the journal cases through `Export`.
- `TestCTSV0008_ArchiveStagePauseLeavesPatienceForPending`: with a positive 500 ms CTS-V0-006
  patience, a slot held for 600 ms and then a 150 ms `REDO_PENDING` window, the export succeeds;
  before `readArchive` pushed its deadline forward by each stage pause it reported `REDO_PENDING`
  at once.
  Against the unfixed `archive/stage_read.go` the first fails with `MALFORMED: a00: unassigned
  stage slot`. `TestCTSV0008_StageSlotWaitBackoffAndBudget` pins the shared pause sequence.

## Related tickets

These share the root cause by code path; neither is reproduced here.

- V1-0467: `TestCALV0019_RacingLeaseVerbsLeaveOneConsistentHead` racer `submit-a` was refused
  `staging/a03`. `prepareLease` runs `AuditForWrite` under the preparation gate, not the writer
  lock, while a racing writer holds slots `a00`..`a03`.
- V1-0949: `TestATRV0008_DetachedRunSurvivesItsLauncher` polls `store.AttemptRecord`
  (`readLeaseProof`, unlocked) and fails on any read error. Meanwhile the detached supervisor
  process journals heartbeats.

## Review follow-up

An independent Codex review of the first commit found two P2 gaps, both fixed above. Writer-lock
holders (reconcile reaches `audit` through `RequestIndex.Lookup`) waited the whole budget while
holding the lock. The archive export kept its own immediate refusal. The spec text now names both
readers and the writer-locked exception.
A second review round found that the export's stage pauses still ran down its CTS-V0-006
deadline, because the pause did not move it. `readArchive` now adds each measured pause to the
deadline.

## Rollback

Set `StageSlotPatience` to zero in `internal/tasks/snapshot/stage_slot_wait.go`. Both readers then
refuse at once and `WriterLocked` has no effect. No state format, wire code or envelope changes.
