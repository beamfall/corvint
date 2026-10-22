# 2026-10-21 localfix-1021x: supervised stage worktree cleanup (V1-0826)

## Problem

V1-0826: `store.Workflow.CleanupWorktrees` had no CLI caller, and an `OpenWorkflow` reassign
(`FINISHED` to `ADMITTED`, assignment + 1) set `Program.Worktrees` to nil. Every prior-assignment stage
worktree therefore stayed on disk and registered in Git with no program record left to find it.

## Decisions

- New operator command `corvint-tasks program cleanup --program ID --config FILE` (ATR-V0-296). It runs
  only for a `FINISHED` program behind a terminal `PROVED` attempt or an idle `ADMITTED` program with no
  attempt, takes ownership the way `OpenWorkflow` does, and never reassigns or claims work.
- Cleanup skips and reports instead of stopping: a worktree that fails the identity, HEAD or clean-tree
  checks, or whose `git worktree remove` fails, stays registered with a reason. Removal of a clean
  worktree whose HEAD is the recorded commit is lossless because candidate refs keep the commit.
- A reassign runs the same cleanup first and keeps the records it could not remove (ATR-V0-297), so
  `program show` reports them and `program cleanup` removes them once they are clean.
- The idle program needed a transition `ADMITTED` <-> `WORKTREE_REMOVE` with no current attempt on
  either side (ATR-V0-298); every other transition rule is unchanged.
- Requirements ATR-V0-296..298 are accepted under the owner's standing auto-accept (2026-10-11).

## Evidence

- `TestATRV0297_ReassignRemovesPriorStageWorktrees`, `TestATRV0296_CleanupRefusesLiveAttempt`
  (`internal/tasks/store`) and `TestATRV0296_ProgramCleanupRefusalsDoNotMutate` (`internal/tasks/cli`)
  pass on Darwin.
- The reassign test fails with the old reassign (`Worktrees = nil`, no cleanup): "reassign kept [], want
  only the dirty ...". It fails with the ATR-V0-298 transition removed: "program transition ADMITTED ->
  WORKTREE_REMOVE [MALFORMED]".

## NOT_RUN

- Linux and live supervised-host qualification; `make gate` and `corvint affected` tier 2 (batch CI).
- Not addressed: resuming a removal interrupted after its `WORKTREE_REMOVE` write (takeover does not
  treat that phase as safe), and pruning removed records inside one assignment.

## Rollback

Revert the commit. Reassign again drops every worktree record, `program cleanup` disappears and the
idle-removal transition is refused; records already kept remain valid program records.
