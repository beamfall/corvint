# 2026-10-20 Process-group Owner: the other callers repeat the group SIGKILL while a member is live (V1-2111)

Human-owned intent: `PGO-V0-017` in `docs/specs/process-group-owner-v0.md`, accepted under the
owner's standing auto-accept (2026-10-11). GitHub #1116 family; follows V1-2103
(`2026-10-20-v1-2103-darwin-fork-group-kill.md`), which added `Primitives.RepeatKillWhileLive` and
opted only the attempt runner in (`ATR-V0-153`).

BASE: `9a63cf8f787eb50bf749296c80df1c36c196c7f7`.

## Problem

On Darwin a group SIGKILL can miss a member that a fork running alongside it adds to the group (3 of
300 stops in the V1-2103 probe). Every other `groupreap.Owner` caller sent exactly one signal, so a
missed member kept the pre-reap quiet proof live until the bound expired and the caller HOLDed.

## Decision, per caller

- Dispatch reader `stateCommand` (dispatcher and standalone work-state reader): opts in. The reader
  command is operator-configured and may fork.
- CEM Git runner, one-shot `runOwned` and session start: opts in through `stableState.primitives`.
  Git forks helpers (pack, credential, hooks).
- Dogfood aggregate worker `runAggregateOwnedProcess`: opts in. The worker is a Corvint child that
  may run Git.
- Linux user-service helper retirement (`SERVICE500-007`): keeps the single signal of `PGO-V0-001`.
  It is Linux-only, and Linux restarts a fork that a group signal reaches mid-copy, so the child
  never appears in the group without the signal. Its quiet proof stays `/proc` based (`PGO-V0-006`).

The repeat stays pre-reap only, while the unreaped leader pins the group ID; a failed repeat HOLDs.
Production hosts are unchanged apart from the flag; tests inject a fake host through a test-only
field (`Dispatcher.readerHost`, `aggregateWorkerHost`, the gitrun seam).

## Evidence

Each opted-in caller has a fake-host test (`TestPGOV0017*` in `internal/cem/gitrun`,
`internal/tasks/dispatch`, `internal/dogfoodflow`) whose quiet proof reports a live member once and
then refuses quiet until a second group kill was sent. With the opt-in commented out, each test
failed (gitrun: ProcessContainment HOLD; reader: QUIESCENCE_UNPROVED / owner HOLD; aggregate worker:
aggregate-cleanup-hold); with it restored, each passed.

## Limits

- The fake host proves the opt-in reaches the Owner; it does not reproduce the Darwin kernel race.
- Linux runtime of the changed callers is `NOT_RUN` locally (macOS host); CI covers it.
- V1-2103's hosted release-gates rerun on macos-15 is owner-only: `NOT_RUN`.

## Rollback

Revert this change. Callers return to the single group SIGKILL of `PGO-V0-001`; the attempt
runner's `ATR-V0-153` opt-in is unaffected.
