# 2026-10-21 proc-1021d: Darwin `?N` process state in the descendant observer (PGO-V0-021, V1-0692)

Lane `proc-1021d` (process supervision, cleanup and host adapters), base origin/main `0106d06cd`.

## Problem

V1-0692 reported that a private verification supervisor (`code_supervisor.py`, 2026-10-03) refused a
whole `ps` snapshot because an unrelated row showed Darwin's `?N` state, which its
`[A-Z][A-Za-z0-9+<>-]*` state pattern did not admit. The ticket asked to separate that temporary
harness behaviour from production procgroup and to prove, with mocked snapshots, that an unrelated
`?N` is accepted, an owned `?N` is signalled as potentially live and a zombie-only identity is skipped,
while duplicate and malformed tables stay refused.

## Decision

Production is not affected and is not changed. `internal/procgroup` `parseDescendantSnapshot`
validates only the row shape, identity and duplicates, and `descendantObserver.finish` treats only a
`Z` prefix as exited, so `?N` already counts as potentially live. `PGO-V0-021` (accepted under the
owner's standing auto-accept of 2026-10-11) records that contract, and two tests pin it:

- `TestV10689SnapshotParsing` gains a `Darwin unknown state` case (`?N` row accepted); its existing
  duplicate and malformed cases still refuse.
- `TestPGOV0021UnknownStateIsPotentiallyLive` injects a snapshot in which an owned identity reads
  `?N`: `finish` signals it (it is reaped with SIGKILL) and reports it absent once it leaves the
  snapshot, while an unrelated `?N` row with parent 1 is neither adopted nor signalled. Both signal
  targets are the test's own `sleep` children, so a wrong signal cannot reach another process, and the
  test reaps both. The zombie-only skip stays covered by `TestObservedExitedZombieIsNotSurvivor`.

Other production `ps` readers (`internal/tasks/dispatch/process_unix.go`, `internal/tasks/cli/doctor.go`,
`internal/opencodequalification/witness.go`) also test only a `Z` prefix and do not refuse an unknown
state; they were read, not changed.

## Evidence

- Focused: `go test -run 'TestPGOV0021UnknownStateIsPotentiallyLive|TestV10689SnapshotParsing|TestObservedExitedZombieIsNotSurvivor' ./internal/procgroup/` passes on Darwin.
- Scratch mutation: making `finish` skip a `?`-prefixed state as exited fails the new test
  (`owned ?N identity not retired: calls=1`). The mutation was reverted.

## NOT_RUN and limits

- The private supervisor amendment itself is NOT_RUN: `code_supervisor.py` is not in this repository
  and its raw evidence directory (`/private/tmp/corvint-0689-repair-proof`) is no longer on this host.
  V1-0692's first and third criteria (retain the raw row and original HOLD; independently verify the
  owned command cleanup before resuming admitted work) stay with that harness.
- Linux runs of the new test come from PR CI only.

## Other lane tickets

V1-1553 criterion 1, V1-0799 and V1-0950 were already on main; V1-1122, V1-1387 and V1-1853 are
delivered by open peer PRs (#1429, #1407). See the PR body.

## Rollback

Revert the commit; it adds tests and spec text only, with no store or wire format change.
