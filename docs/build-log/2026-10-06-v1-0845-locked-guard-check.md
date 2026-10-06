# 2026-10-06: V1-0845 darwin change guard keeps the stat re-read out of the writer lock

## Intent

Ticket V1-0845 (agent-filed from an independent review of the V1-0841 fix): on macOS, files beyond
the kqueue descriptor budget are tracked by a stat tuple that every `ChangeGuard.Check` re-read.
Lease commits call `Check` twice while holding the writer lock, so once a store exceeds the budget
the locked work grew with the number of over-budget files, contrary to CAL-V0-026. Governing
contract: `docs/specs/corvint-tasks-agent-leases-v0.md`, CAL-V0-026 and the descriptor-budget
paragraph of the change-guard section.

## Change

- `changeWatch.changed` is split into `poll` (one non-blocking kqueue or inotify read plus the kept
  result, independent of watched-path count) and `sweep` (re-read the over-budget stat tuples; a
  difference or failed read marks the watch dirty for good). Linux `sweep` has nothing to read.
- `ChangeGuard.Check` is unchanged in meaning (poll, then sweep). New `Sweep` and `CheckEvents`
  (poll only) let a caller do the re-read before taking the lock.
- `commitLease` calls `p.guard.Sweep()` before `AcquireLock` and `CheckEvents` at both locked check
  points. Because the sweep's finding is sticky, the locked check reports it at the same point in the
  locked sequence, so the order of refusals is unchanged.
- The spec's second descriptor-budget limit is replaced by the accepted bound below.

## Decision: sweep before the lock, directory events cover store writers

Of the ticket's directions:

- Raising the budget is not a bound: it is capped by the hard descriptor limit and
  `kern.maxfilesperproc` (10240 on the hosted runner), below a populated store.
- Directory events alone cover only entry changes; an in-place write or `chmod` fires no event on
  the parent, so they are weaker than the stat tuple for those classes.
- Sweeping before the lock and polling under it is chosen, with directory events closing the gap
  between the two. Every store writer changes files in the state directory and intent tree only
  through `authority.Session` (`Mkdir`, `Prepare` into a new `O_EXCL` stage, `Link`, `Replace`,
  `RemoveStage`, `RemoveBarrier`), that is, by creating, linking, renaming or removing entries.
  Each of these fires `NOTE_WRITE` on a watched directory, and every directory is always
  registered. kqueue keeps that event pending from registration until the locked poll reads it, and
  the watch's dirty flag is sticky. So a store write made between the sweep and the locked check is
  not lost, and a change the sweep found is still reported under the lock. The other in-place
  writers in `internal/tasks` are not store writers: admission slots and lock files live in the
  common directory, the retained checkpoint outside the state directory, and dispatch and service
  state under their own configured roots.

Accepted bound: an in-place write or mode change to an over-budget file by an actor that does not
take the writer lock (an editor or tool writing a ticket file in place), made after the sweep read
that file and before the locked check, is not seen by that check. Such an actor is not ordered
against the writer lock; under V1-0841 the same write made just after the locked check was equally
unseen, so the stat re-read under the lock only moved where that window began, by the lock wait.
The edit is met as an edit made after the check is (pre-apply binding and the next audit). Within
the budget, and on Linux, nothing changes.

## Evidence

- Maintained darwin tests in `internal/tasks/authority/change_guard_darwin_test.go`, using the
  injectable budget and a counting `sweepLstat`:
  `TestCALV0026_LockedCheckIndependentOfOverBudgetFiles` (budget 2, 9 and 257 files: the sweep and
  an unlocked `Check` each read 7 and 255 tuples, two `CheckEvents` read none) and
  `TestCALV0026_SweepThenLockedCheckLosesNoChange` (a change before the sweep and a rename, link,
  remove or sibling create after it are all reported by `CheckEvents` with no stat read; the
  in-place write after the sweep is the recorded bound and a later `Check` reports it).
- Existing darwin guard tests updated to the split methods and passing; the broken-watch test now
  covers `CheckEvents`.
- Targeted store lease tests (`TestCALV0026_*` audit/guarded inventory/reuse/preparation,
  `TestCALV0019_Racing*`, `TestCALV0011_*`, `TestCALV0048_*`, `TestGH494_*`) pass, and
  `TestCALV0026_GuardedInventoryPopulated5000` passes under a 10240 soft descriptor limit.
- `go vet` for darwin, linux and windows on `internal/tasks/authority` and `internal/tasks/store`.
- Not run: the repository gate (a release-candidate gate was running on this host), Linux runtime
  tests, a live lock-hold measurement on an over-budget store.

## Rollback

Revert the commit. The guard returns to re-reading over-budget tuples under the lock, which is
correct but violates CAL-V0-026's locked-work bound beyond the budget.
