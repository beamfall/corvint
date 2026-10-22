# 2026-10-21 dispatch-1021u: hand-off note commit and dispatch residuals

Lane `dispatch-1021u` (branch `claude/dispatch-1021u`, base `9ae946a5c`). Tickets V1-2246, V1-1389,
V1-1571, V1-0771 and V1-1091. New and amended requirements are accepted under the owner's standing
auto-accept (2026-10-11).

## Problem

- V1-2246: the dispatcher never set the optional `commit` member of the ATR-V0-104 hand-off note
  (ERG-V0-026), so a `gate packet` checkpoint built from a salvaged note always reported
  `commitBinding` `NOT_OBSERVED`.
- V1-1571: (a) an `opencode` line refused for a repeated or case-folded member made the context
  estimate UNKNOWN but left the CAL-V0-157 usage totals `KNOWN`, although it may have been an
  uncounted `step_finish`. (b) A post-restart detached hand-off lost the `CONTEXT` detail. (c) A
  crossing whose record write failed before a restart is reported only after a fresh line.
- V1-1389: dispatch recover-reader residuals (pressure re-sample, operator event, journal
  attestation, live and Linux qualification).
- V1-0771 (suspected race) and V1-1091 (suspected btime jitter in Linux process identity).

## Decisions

- ERG-V0-028 (new, `docs/specs/corvint-tasks-external-reviews-v0.md`): a salvaged note names the
  attempt's commit. That is the supervised worktree's `HEAD` when the attempt has one, else
  `refs/heads/<branch>` in the work root. The value is resolved through the injected
  `store.ResolveCommit`. An unresolved or invalid commit leaves the member out. The dispatcher
  path and `dispatch salvage` both bind it.
- V1-1571 (a): `WorkerUsage.line` marks the totals malformed for that line, as the Codex and Claude
  paths already do for refused members. ATR-V0-037 is amended. (b) Already delivered by V1-1562
  (`6adb973d6`, ATR-V0-057, `TestATRV0057_DeferredContextHandoffSurvivesRestart`). The stale
  failure-mode bullet now cites ATR-V0-057. (c) Accepted in the spec: reporting the crossing
  earlier would act on an unobserved estimate, which ATR-V0-038 forbids.
- V1-1389: spec rationale in `docs/specs/corvint-tasks-agent-leases-v0.md`. No pressure re-sample
  is needed, because the next start is UNKNOWN until its own sample (CAL-V0-068). The state-dir
  attestation suffices, because no Tasks authority is involved and the program lock already fences
  it. The operator-path event was not built. Emitting it needs either a ledger `eventSeq` write from
  a non-dispatcher, which risks an implicit ledger migration, or a pending-announcement file read
  at the next start. Live and Linux qualification need a live host. Both stay open.
- V1-0771: NOT-REPRODUCED. The named tests passed 70/70 with `-race` (count 15 and 20) and 80/80
  without it (count 40), at load average 11 to 45.
- V1-1091: the in-process fix is already on main (`05fa42d95`,
  `TestV11091IdentityStableAcrossBtimeJitter`). Persisted cross-process identity comparison still
  has no ±1 s tolerance at about 25 `ProcessIdentity` call sites. AC1 needs a GitHub Linux runner.

## Evidence

- `TestERGV0028_CheckpointCommit`, the extended `TestATRV0104_HardStopSalvagesBoundedScreenedNote`
  (`internal/tasks/dispatch`), and `TestERGV0028_SalvagedNoteBindsPacketCheckpoint`
  (`internal/tasks/cli`) fail with the commit resolution forced empty and pass with it.
- `TestCALV0157_UsageVocabulariesAndStates` gains two refused-member rows. Both fail without the
  fix (state `KNOWN` instead of `PARTIAL`) and pass with it.

## NOT_RUN

Live dispatched-lane note binding; a supervised attempt's worktree `HEAD` beyond the fixture; live
recover-reader HOLD, refusal, recovery and start on a real host; Linux boot-identity recovery;
tier-2 `corvint affected` units (left to batch CI).

## Rollback

Revert the lane commits. No profile, ledger, receipt, wire code or analyzer schema changed. The
note's `commit` member was already optional.
