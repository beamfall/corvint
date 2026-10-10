# V1-0825: unlocked audits wait out a writer's staging slots

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
- The budget is a journal package variable, not `snapshot.DefaultPatience`. The fixture package
  zeroes that variable in every test binary, and the store tests that flaked are test binaries.
- The writer-checkpoint audit (`AuditForWriter`) is unchanged: it already declines staging as a
  checkpoint miss.

Cost: a store with real orphan slots now takes 2 s longer to refuse an unlocked read, and so does a
barrier or reconcile write, which does not clear orphans. Every other write still clears orphans
under its lock first (CAL-V0-019), so it does not wait.

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

## Related tickets

These share the root cause by code path; neither is reproduced here.

- V1-0467: `TestCALV0019_RacingLeaseVerbsLeaveOneConsistentHead` racer `submit-a` was refused
  `staging/a03`. `prepareLease` runs `AuditForWrite` under the preparation gate, not the writer
  lock, while a racing writer holds slots `a00`..`a03`.
- V1-0949: `TestATRV0008_DetachedRunSurvivesItsLauncher` polls `store.AttemptRecord`
  (`readLeaseProof`, unlocked) and fails on any read error. Meanwhile the detached supervisor
  process journals heartbeats.

## Residual

`internal/tasks/archive/stage_read.go` keeps its own copy of the unassigned-slot check. An
`archive export` racing a writer can still be refused this way.

## Rollback

Set `stagePatience` to zero in `internal/tasks/journal/audit.go`. No state format, wire code or
envelope changes.
