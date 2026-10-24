# 2026-10-24: a free writer lock proves a killed writer's stage slots orphans (V1-2506, CTS-V0-026)

## Decision

An unlocked journal audit that has spent the whole CTS-V0-008 wait beside descriptor-less
`staging/aNN` slots, under a head that did not move, now tests the writer lock
`<git-common-dir>/taskman.lock` once before it refuses `MALFORMED` `unassigned stage slot`.
Every writer holds that lock exclusively for the whole of its publish, so a lock that is free proves
the slots a killed writer's orphans. The audit observes again, setting aside exactly those slots
(bound by file identity, mode, size and modification time), and reads the store. It reports them in
`Result.OrphanStageSlots`, and `receipt audit` warns with their names. A slot that is new or changed
after the proof is a live writer's, and the audit refuses the retryable `SNAPSHOT_MOVED` at
`staging`. A held, absent, symlinked, non-regular or replaced lock file keeps the `MALFORMED`.

The probe lives in `internal/tasks/journal` (`lock_probe.go`), not `authority`, because an
`authority` test imports `journal`. It opens the existing regular file read-only through a
`safeopen` root confined to the common directory, with no create and no symlink following. It checks
the file's identity before and after, and tests `flock(LOCK_SH|LOCK_NB)`, releasing it at once. It
never waits, so a writer polling `LOCK_EX|LOCK_NB` loses at most one poll interval.
`TestCTSV0026_ProbeNamesTheWriterLock` pins the file name against a real `authority.AcquireLock`.

Nothing changes on disk or on the wire beyond one additive warning on `receipt audit`, and reads
still never remove a slot; the next writer removes them (CAL-V0-019). The archive `export` reader is
unchanged.

## Root cause

A gate run that the Pi adapter aborts gets SIGTERM, then SIGKILL 100 ms later
(`integrations/pi/runtime.js`, `process.js`). A SIGKILL inside a publish skips the deferred
`RemoveStage`, which leaves a slot with no descriptor. Before this change every unlocked read waited
two seconds and refused `MALFORMED` until some writer ran, so the wrapper's reconciliation reads all
failed and the adapter returned `reconciliation-unavailable`.

## Evidence (macOS arm64, Go 1.27.2)

- Journal tests: free lock reads; held, absent and symlinked locks stay `MALFORMED`; new or replaced
  slots after the proof refuse `SNAPSHOT_MOVED`; writer-locked and non-native audits never probe.
- Store regression `TestCTSV0026_KilledGateRunLeavesAReadableStore` SIGKILLs a real `gate run`
  subprocess at an artifact publication while one of its slots is occupied. `ticket show`,
  `queue status` and `receipt audit` then exit 0 and the warning names the orphan, and the next gate
  run removes it. With the probe disabled, the same test fails `MALFORMED` at `ticket show`.
- Pi `tasks-host` interruption test (`PWV actual native gate interruption retains uncertainty`):
  no `MALFORMED` since the fix. Its remaining failures, 5 of 12 classified runs under host load, are
  `REDO_PENDING` with staging empty: the SIGKILL landed after the receipt commit and before the head
  apply. That is the designed crash state, and reads never redo (TM-V0-008). V1-2506 acceptance
  item 3 (5 consecutive Pi passes) therefore stays open in a follow-up ticket, which owns the adapter
  kill grace or the reconciliation choice.

## Rollback

Revert the commit. Reads beside a killed writer's orphan slots then refuse `MALFORMED` again until
the next writer runs. There is no state format, code or envelope migration.
