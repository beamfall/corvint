# PR #1631 review repair: exclusive worktree removal, bounded export roots, rollback guidance

Date: 2026-10-23. Branch claude/lucid-ramanujan-kpyf2j (PR #1631), merged with origin/main first.
PR #1631 is kept over the duplicate PRs #1629 and #1630. Requirements `CAL-V0-367` to `CAL-V0-371`
(`docs/specs/corvint-tasks-agent-leases-v0.md`) and `CTS-V0-023`, `CTS-V0-024`
(`docs/specs/corvint-tasks-store-init-v0.md`) are accepted under the owner's standing auto-accept
(2026-10-11).

## Findings applied to #1631's code

The independent reviews of #1629, #1630 and #1631 were checked against this branch's
`WORKTREE_REMOVE` code. Every one applied here:

- A live cleanup could be taken over (all three reviews). Completed stages leave `ownerReleased`
  true, and cleanup never cleared it, so a second cleanup passed the live-owner check. The takeover
  write and the write entering `WORKTREE_REMOVE` now record `ownerReleased` false, and the
  transition refuses a released `WORKTREE_REMOVE` entry (CAL-V0-367).
- A dead owner's `git worktree remove` child could outlive the takeover (#1629, #1630). The child is
  now `/bin/sh` gated on its stdin that `exec`s Git once released, so the process identity recorded
  in the `WORKTREE_REMOVE` write (`removerPid`, `removerStarted`) is the Git process, and an owner
  that dies first closes the gate. A takeover refuses `REMOVER_LIVE` while that identity runs and
  `REMOVER_UNRECORDED` without one (CAL-V0-368). Both refusals come before any inspection.
- Legacy `removed:true` records were filtered only in memory (#1629, #1630). They are now dropped by a
  durable same-phase write first; the transition admits `ADMITTED` to `ADMITTED` only for that exact
  compaction, since that phase has no other self-transition (CAL-V0-369).
- A rolled-back removal was retried in the same invocation (#1629), so one cleanup reported
  `REMOVAL_ROLLED_BACK` and a removal for one path. The settled record is now skipped until the next
  cleanup (CAL-V0-370). The CAL-V0-365 test had asserted the old retry; it now asserts the skip.
- A moved worktree's record was dropped on `ENOENT` (#1629, #1631). The record is now kept whenever
  another existing registered worktree has its directory or private Git-directory identity, or has an
  identity that cannot be read (CAL-V0-371).

Each test was checked against a mutation that removes only its fix. With every reclaim removed,
`TestCALV0367_LiveCleanupOwnerIsNotTakenOver` fails: B is then refused only by `REMOVER_LIVE`, so
the remover check also covers the live-owner case. Without the skip, the moved-identity check,
the compaction write or the remover check, the matching CAL-V0-365, 371, 369 and 368 tests fail.

## V1-0903 second cause: one directory root per listed directory

Ported from #1630 (branch claude/relaxed-sagan-s3ubgt) with its reviewed defect fixed. `archiveRead`
kept every listed directory open, 262 roots on a 10k store, so a low `ulimit -n` failed export with
`RESOURCE_EXHAUSTED`. #1630 evicted after opening, so the peak was 17 roots, and the temporary
binding root was not counted. Here a slot is reserved before every open, including the binding
root, so the peak is at most 16 (CTS-V0-023). A test hook observes the held count after each open.
Moving the reservation after the open makes the test observe 17. The stream is compared
byte-for-byte with an export that has no effective bound.

## Rollback guidance (CTS-V0-024)

The CTS-V0-020 rollback text said that an older writer's stage decoder refuses any store holding an
`IMPORT_PLAN` receipt. That is wrong. The stage decoder reads only active staging descriptors, and
the non-fixture writer stages none. Historical receipts go through the journal reader, whose closed
kind set already held `IMPORT_PLAN`. An interrupted amendment is completed by redo.
`internal/tasks/journal` and `internal/tasks/store/redo.go` are unchanged from origin/main, so the
test runs the pre-amendment path. No older binary was run. This item corrects documentation; its
test passes on the tree before the correction.

## Not done

#1630's review findings 3 and 4 (source-queue renames, duplicate ticket mappings) concern import-map
amendment, not `WORKTREE_REMOVE`, and were outside this repair. CTS-V0-021 already refuses a ticket
mapped twice. It still permits renaming an old ticket's source item, which the CTS-V0-021 non-goal
names, and that leaves the renamed item un-importable.
