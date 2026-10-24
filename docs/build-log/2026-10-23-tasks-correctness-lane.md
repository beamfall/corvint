# corvint-tasks correctness lane: archive patience, cleanup takeover, import map amendment

Date: 2026-10-23. Lane: claude/lucid-ramanujan-kpyf2j (base origin/main 7b3a8e55). Tickets V1-0903,
V1-2368, V1-2400, V1-0907 and V1-0775. Requirements `CTS-V0-020` to `CTS-V0-022`
(`docs/specs/corvint-tasks-store-init-v0.md`) and `CAL-V0-365` to `CAL-V0-366`
(`docs/specs/corvint-tasks-agent-leases-v0.md`) are accepted under the owner's standing auto-accept
(2026-10-11). The reserved IDs were free on origin/main and in every open pull request when numbered.
Go 1.27.2 was fetched with `GOTOOLCHAIN=go1.27.2`; the host's local toolchain is go1.24.7.

## V1-0903: archive export rc=1 at 10k tickets

Cause: the CTS-V0-006 patience budget was a wall-clock deadline taken at the start of the read, in
both `snapshot.Reader.Read` and the export's `readArchive`. The spec bounds pauses ("a patience
budget of two seconds"; CTS-V0-008 says "two seconds of pauses"), but the body's own run time spent
the deadline. A 10k-ticket export body takes seconds (about 6.5 s in process, about 18 s for the
built binary including staging and verification), so by its re-probe no budget was left. A writer
caught between receipt link-in and head rename at that probe, or at the next attempt's first probe
after a moved snapshot, then failed the export at once with `REDO_PENDING` and no pause: exit 1.
A small store's body ends well within two seconds, which is why the existing CTS-V0-006 tests passed.

Evidence:

- Store: `TestRC3_SyntheticStore` with 10,000 tickets and 22,000 receipts (about 300 MB; the CLI
  generator `tasks-scale gen` ran at 1 to 2 tickets per second and slowing, so it was stopped at
  about 1,000 tickets).
- 22 runs of the built `archive export`, each followed by `archive verify`: 5 quiet runs and 6 under
  a looping `pause`/`unpause` writer, for each of the origin/main binary and the fixed binary. All
  exited 0 and verified, at about 24 s each. A real writer's pending window is milliseconds, so
  this bounds the failure rather than reproducing it: it does not happen on a quiet store, and it is
  rare under a writer that commits about once every 33 s.
- Reproduction: an uncommitted in-process test on the same 10k store planted a writer's pending
  receipt for 1.5 s at the end of the first export body (6.5 s). The old code failed with
  `REDO_PENDING` after 8.1 s and one body; the fixed code paused, re-read and exported after
  24.7 s and two bodies. `head.json` and the receipt set were restored byte-identical after each run.
- Regression tests `TestCTSV0022_LongBodyKeepsPausePatience` (snapshot) and
  `TestCTSV0022_SlowExportStillWaitsOutWriter` (archive) fail on the old code with `REDO_PENDING`
  and pass with the fix.

Fix (CTS-V0-022): both readers charge the budget with pause durations only. Nothing else changes:
the budget value, backoff, attempt bound, codes and envelope stay the same, and the staging-slot
wait still does not draw on the budget.

## V1-2368: program cleanup takeover and removed-worktree records

Reproduced: a child process stands in for the cleanup owner and exits right after the
`WORKTREE_REMOVE` write. The parent's `CleanupProgram` was then refused, first `ATTEMPT_LIVE`
(`cleanupPhase`) and, with that check widened, `FENCED` "prior program owner not proved stopped"
(the transaction's safe-takeover phase list). The fix (CAL-V0-365) settles the parked removal in the
takeover write itself, with typed outcomes `REMOVAL_COMPLETED` (Git no longer registers the worktree
and its path is gone; the record is dropped) and `REMOVAL_ROLLED_BACK` (the record is kept for the
checked retry). The transaction admits an owner change from `WORKTREE_REMOVE` back to `FINISHED`
(proved terminal attempt) or `ADMITTED` (no attempt). `OpenWorkflow` settles the same way before a
reassign. CAL-V0-366 drops a removed worktree's record at each cleanup rather than marking it, so
records do not accumulate within an assignment. Tests fail on the old code: `ATTEMPT_LIVE`, then
`FENCED` with only the transaction change reverted, and six marked-removed records left behind.

## V1-2400: import map amendment

Decision: amend through the reserved, previously unused receipt kind `IMPORT_PLAN`, so the closed
§3.4 receipt-kind set does not change. The amendment adds a stage operation of the same name
(7 artifacts, a 1838-byte descriptor held exactly by the stage codec test) and the CLI verb
`import amend-map`. "Reviewed" is implemented as OWNER-only plus binding the journaled map digest
the owner reviewed (`--expected-map-sha256`); a stale digest is `REVISION_CONFLICT`. The identity
rules (CTS-V0-021) keep every mapped ticket mapped and every mapped item on its ticket, map no ticket
twice, and refuse a new entry for an existing ticket unless that ticket is that item's import. Any
conflict is `DUPLICATE_ID` naming each one, with nothing written. Residual: the importer still
derives a ticket from its source item ID and never consults the map, so recording a source rename
does not make a re-import follow it (CTS-V0-017 refuses the renamed item). That is recorded as a
CTS-V0-021 non-goal, and import-side rename following needs its own ticket. A first draft let
`importChain` accept a source change the map recorded. It was withdrawn, because `importer.Plan`
never targets the mapped ticket, so the change could not take effect.

## Independent review

One independent review agent read the diff. It found no blocking defects and five non-blocking
ones. All five were fixed, with tests where behaviour changed:

1. With pause-only patience, a moved re-read paused 25 ms each time, because the stall reset after
   every clean probe. A store moving under every 18 s body could therefore be re-read about 80
   times. Now the backoff keeps doubling across re-reads (CTS-V0-022), and
   `TestCTSV0006_MovedReadsPauseBetweenAttempts` holds the doubling sequence.
2. `AfterWait` called the elapsed time "pauses totalling". It now names the pause count and the
   time since the read began.
3. `settleRemoval` compared the recorded path with Git's resolved listing, so a symlinked parent
   never matched. The parent's symlinks are now resolved as well.
4. An interruption inside `git worktree remove` leaves the path gone while Git still lists the
   worktree. Such a removal now settles as `REMOVAL_COMPLETED`, with a reason naming
   `git worktree prune`. Before, it was rolled back and stayed stuck on every cleanup. It is tested
   as the "half" mode.
5. An amendment could map a source item that an existing ticket imports to a different ticket.
   That is now a `DUPLICATE_ID` conflict (CTS-V0-021), tested as "imported item". The test fails
   without the check.

## V1-0907: skipped as already delivered

The review fold already runs at most once per call path on origin/main:
`internal/tasks/cli/external_review.go:84` (`completionOffers` folds once per `ticket show` or
`plan preview` call), `:668` (`gate state`, one fold), `:124` (`receipt audit` reuses the fold of a
complete audit), and `internal/tasks/cli/dispatch.go:967-969` (the dispatcher folds once per tick
and carries an incremental `ReviewFold`, CAL-V0-138). A new test of that would not fail on the old
code, so none was added. Remaining limit: a one-shot CLI read still folds receipts 1..head, so its
cost grows with history. Bounding that needs a persisted fold carrier, which is a wire and design
decision for the owner.

## V1-0775: confirmed real; fix blocked on an owner decision

`TestATRV0241_MappedWriteAfterLeaseAuditRefusedByNextWrite` (already on main, ATR-V0-241) was run on
Linux ext4 and on tmpfs (`TMPDIR=/dev/shm`): in all four cases the claim commits under a `MAP_SHARED`
write made at the `sweep` or `lock` stage, and the next write refuses the store. The lease complete
route builds its inventory from the audit's digests (`internal/tasks/store/scan.go:307-313`), and
under the lock `commitLease` checks only the change-guard events and the head digest. ATR-V0-241
records this window and leaves closing it, a content re-read of every audited file per lease write,
to the owner, because CAL-V0-263 keeps that work out of the lock. The attempt-runner spec now records
the Linux evidence. No fix was made.
