# 2026-10-06: V1-0845 darwin change guard keeps the stat re-read out of the writer lock

## Intent

Ticket V1-0845 (agent-filed from an independent review of the V1-0841 fix): on macOS, files beyond
the kqueue descriptor budget are tracked by a stat tuple that every `ChangeGuard.Check` re-read.
Lease commits call `Check` twice while holding the writer lock, so once a store exceeds the budget
the locked guard work grew with the number of over-budget files, contrary to CAL-V0-026. Governing
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
- Test seams only: `authority.SetVnodeTestHooks` (darwin; budget and an observer of over-budget
  stat reads) and an unexported `commitStage` field in the store's inventory hooks, called with
  `sweep` before `Sweep` and `lock` before `AcquireLock`.

Scope of the claim: guard polling under the writer lock is now independent of store size. CAL-V0-026
is not yet met for lease commits: `retainCheckpoint` (`lease_write.go` → `journal/checkpoint.go`,
`store/guards.go`) still traverses, sorts and encodes the whole canonical map under the lock. That
cost predates V1-0845 and remains, recorded for a follow-up ticket.

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
take the writer lock (an external editor or tool writing a ticket or `policy.json` in place), made
after the sweep read that file and before the locked check, is not seen by that check. No Tasks
writer edits a watched file in place, so only such external actors reach this window. The real
consequence: the lease commits on the canonical journal content, because `commitLease` rebinds only
`head.json` (it does not call `bindObservation` and ordinary leases pass no `beforeCommit`), and the
next journal audit refuses the edit, `INTENT_DIVERGED` for an intent projection. Such an actor is not
ordered against the writer lock; under V1-0841 the same write made just after the locked check had
the same consequence, so the stat re-read under the lock only moved where that window began, by the
lock wait. Within the budget, and on Linux, nothing changes.

## Evidence

- Maintained darwin tests in `internal/tasks/authority/change_guard_darwin_test.go`, using the
  injectable budget and a counting `sweepLstat`:
  `TestCALV0026_LockedCheckIndependentOfOverBudgetFiles` (budget 2, 9 and 257 files: the sweep and
  an unlocked `Check` each read 7 and 255 tuples, two `CheckEvents` read none) and
  `TestCALV0026_SweepThenLockedCheckLosesNoChange` (a change before the sweep and a rename, link,
  remove or sibling create after it are all reported by `CheckEvents` with no stat read; the
  in-place write after the sweep is the recorded bound and a later `Check` reports it).
  The rename and link cases stage their file outside the watched trees and first assert a clean
  `CheckEvents`, so the single entry operation alone must produce the report.
- Maintained darwin store tests in `internal/tasks/store/lease_guard_darwin_test.go` drive a real
  claim through `Lease` with budget 0:
  `TestCALV0026_LeaseCommitSweepsOutsideWriterLock` probes writer-lock ownership with a second
  `flock(LOCK_EX|LOCK_NB)` on each over-budget stat read and requires 0 reads under the lock and
  more than 0 between the `sweep` and `lock` stages (12 observed). Mutation checks: reverting the
  first locked `CheckEvents` to `Check` (12 locked reads), moving `Sweep` under the lock (12 locked,
  0 in the window) and removing `Sweep` (0 in the window) each fail it.
  `TestCALV0026_InPlaceIntentEditAfterCommitSweep` edits `policy.json` in place: at the `lock` stage
  the claim commits and the next `Lease` is refused `INTENT_DIVERGED`; at the `sweep` stage the claim
  is refused (`INTENT_DIVERGED`) with no receipt.
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
correct but adds store-size guard work under the lock beyond the budget.
