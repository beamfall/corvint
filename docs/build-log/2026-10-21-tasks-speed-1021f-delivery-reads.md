# Delivery screen and batch-delivered reads (V1-2251, V1-2254)

Date: 2026-10-21. Lane: tasks-speed-1021f. Spec: `docs/specs/corvint-tasks-closeout-debt-v0.md`
TCD-V0-159..164, accepted under the owner's standing auto-accept of 2026-10-11.

## Problem

About 40% of the tickets dispatched to lanes in batches 1021c and 1021d were already delivered or in
flight in another PR (V1-2251), and delivered tickets stayed OPEN after their batch merged
(V1-2254). The orchestrator screened with a scratch Python script.

## Decisions

- Both reads live in `corvint-tasks ticket` next to `closeout-debt`, reusing its ticket index,
  ref resolution and bounded commit scan (`closeoutRef` and `parseScanLimit` are now shared).
- GitHub facts come only from operator `gh --json` export files (`--prs`, `--runs`). The binary
  never queries a network service (invariant 7). Without an export the screen says NOT_OBSERVED,
  never CLEAR, and `greenRuns` is NOT_OBSERVED.
- The delivered-versus-mentioned rule ports the scratch screen: a PR line with a Refs, Closes,
  Fixes or Delivered lead (or DELIVERED) delivers unless it says not started, not delivered,
  skipped, residual, duplicate, blocked or already. Commit lines deliver unless vetoed.
- `batch-delivered` builds on the CI-evidence gate of #1436 (V1-2216, merged into this lane): it
  lists ticket, lane PR, batch PR, merge commit and green run ids for a closer to feed to
  `gate run --ci-run` and `complete`. It completes nothing.

## Evidence

`TestTCDV0159_DeliveryScreenVerdicts` and `TestTCDV0163_BatchDeliveredListsLaneTicketsWithGreenRuns`
pass. With the NOT_OBSERVED branch and the success filter removed, both fail.

## NOT_RUN

- V1-2251 criterion 2 (dispatch claims a second dispatch sees) and criterion 4 (the next sweep
  below 10% duplicates): not in this lane.
- V1-2254 closing step and the open-and-referenced count reaching 0: the closer runs
  `gate run --ci-run` and `complete`; `closeout-debt` already reports the count.

## Rollback

Revert the change. The verbs are additive reads with output-only profiles.
