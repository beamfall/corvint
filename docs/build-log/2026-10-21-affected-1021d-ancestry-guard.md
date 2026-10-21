# 2026-10-21 lane affected-1021d: shared grafts/shallow ancestry guard

Tickets: V1-1757 (delivered in part). Already in flight elsewhere: V1-2053 and V1-0752 in #1421,
V1-1766 in #1420 (both inside batch #1430); this lane's duplicate V1-2053 commit was reverted.
V1-2071 skipped. EEP-V0-028 is accepted under the owner's standing auto-accept of lane
requirements (2026-10-11).

## Problem

- V1-1757: Git honours `info/grafts` even with `GIT_NO_REPLACE_OBJECTS=1`, and a shallow boundary
  cuts parents. Only appflows refused such checkouts (AFU-V1-068).

## Decisions

- EEP-V0-028: the appflows guard moved unchanged into `internal/gitancestry.Rewritten`; appflows
  calls it, and external-evidence freshness degrades to `revision-unavailable` under it.
- V1-1757 audit (ancestry reads that gate evidence or selection), guarded now: appflows
  repository binding, extevidence freshness. Still unguarded, residual: appflows/selection.go,
  dogfoodflow check/batch/batch_dropped/migration/finishlane/flow, postmergeconnector/git.go,
  postmergemetrics/git.go, witness/witness.go, contextindex/range_impact.go, cmd/corvint
  runs_baseline.go, breakagemap/compile.go, dashboard/repository/trace.go, appflows/review.go,
  localcompletion/storage.go, sharedpath, upgradechanges, tracerecordrepo, gitnotes, genesis
  guidance and the Tasks store ancestry reads (outside this lane's scope). Root-commit identity
  reads (`rev-list --max-parents=0`) are also graft-sensitive. Dogfood guarding needs care: a CI
  job checks out with `fetch-depth: 2`, so refusing on shallow there must be decided first.
- V1-2071 skipped: affected already reads an AGENTS.md `## Verify` fenced block; the ticket's
  repositories declare an inline `**Verify:**` line or a `## Build / gate` heading, which needs a
  declaration-grammar decision, and impact's receipt command list is a wire contract. It did not
  fit the remaining lane budget.

## Evidence

- `TestFreshnessDistrustsRewrittenAncestry_EEPV0028` fails without the guard (`provider-ahead`,
  `unrelated-history`) and passes with it; `TestRewrittenNamesGraftsShallowAndUnreadable_EEPV0028`
  and `TestAFUV1068AncestryRewrittenRefused` pass.

## NOT_RUN

`make gate` (owner preference for scoped lanes); full package suites beyond the focused tests.

## Rollback

Revert the lane commits; the guard move is behaviour-neutral for appflows.
