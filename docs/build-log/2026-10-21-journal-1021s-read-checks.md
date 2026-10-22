# 2026-10-21 journal-1021s: read-checkpoint request duplicate, stale-projection warning, release recovery

Lane journal-1021s (branch `claude/journal-1021s`). Tickets: V1-0917, V1-0324, V1-1964 (as narrowed
by decision 0552), plus decision 0552 itself. Requirements PSR-V0-190, PSR-V0-191, ATR-V0-280 and
ATR-V0-281 are accepted under the owner's standing auto-accept (2026-10-11).

## Problem

- V1-0917: a read resumed from the read checkpoint walks only the tail, and the read checkpoint
  retains no `requests/` entries. A tail receipt that posted a request ID already posted before the
  checkpoint therefore passed the fast route (`Audit` returned AGREES) while the complete audit
  refuses `JOURNAL_FORKED`.
- V1-0324: unaudited reads served a projection that differs from the journal head silently; TCP-00
  §3.3 and crash matrix C6 disagreed on whether reads refuse.
- V1-1964: a supervised attempt's recovery named no command, and a `STALE_HOLDER` recovery did not
  say the stale heartbeat is advisory.

## Decisions

- PSR-V0-190: in `step` on a resumed walk, a `requests/` post with a non-null pre-state the
  checkpoint does not retain declines the checkpoint (`errCheckpoint`), so the complete audit
  classifies it. Zero extra reads. A forged duplicate that declares a null pre-state is still only
  caught by the writer route's request index and by the complete audit; recorded as a residual.
- PSR-V0-191: `success()` compares the head receipt's `intent/` afterimages (already read and
  digest-bound to `head.json` by the snapshot) with the loaded store's digests and adds a
  `PROJECTION_STALE` warning (bounded to four paths plus a count). No new §11 code. Audited reads
  (`ticket show`) and writers keep `INTENT_DIVERGED`. The external TCP-00 C6 wording edit is pending.
- ATR-V0-280/281: every recovery without a reap names the exact `release` command; a supervised
  attempt's recovery explains the `null` leaseExpiresAt; `STALE_HOLDER` is reported as advisory. No
  automatic reap (CAL-V0-121 kept).
- Requirement homes: PSR (journal audit on a moving journal already lives there) and ATR, per the
  orchestrator's instruction; no CAL-V0 IDs.

## Evidence

- `TestPSRV0190_CheckpointTailRefusesPrefixRequestDuplicate` (journal): failed before the fix with
  `got <nil>; want JOURNAL_FORKED`; passes after, with the CAL-V0-059/061 and TM-V0-006 neighbours.
- `TestPSRV0191_StaleProjectionReadWarns` (cli): failed with the call disabled
  (`[queue status]: stale projection read without PROJECTION_STALE`); passes after.
- `TestATRV0280_RecoveryNamesReleaseForEveryHolderWithoutReap` (transaction): four `detail lacks`
  failures against the previous `attempt_detail.go`; passes after.
- Affected selection, pre-push traps and doc gates: recorded in the PR body.

## NOT_RUN

- Full `internal/tasks/store` and `internal/tasks/cli` packages (24 and 10 minutes) and the
  `corvint affected` tier 2 units: left to batch CI.
- V1-1663, V1-1568, V1-0926, V1-1662 and V1-2202 S6b: not started in this lane (see the PR's "Not
  delivered" list).

## Rollback

Revert the commit. Reads again serve a stale projection silently, the read checkpoint again relies
on the complete audit for a prefix request duplicate, and recoveries return to the previous text.
No stored format changes.
