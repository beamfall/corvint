# 2026-10-21 lane affected-1021d: MIXED refusals name paths; shared ancestry guard

Tickets: V1-2053 (delivered in part), V1-1757 (delivered in part), V1-0752 (already delivered),
V1-1766 and V1-2071 (skipped). AFP-V0-162 and EEP-V0-028 are accepted under the owner's standing
auto-accept of lane requirements (2026-10-11).

## Problem

- V1-2053: `corvint affected --require-clean` and `--path` refused a dirty worktree with a count
  only. A path ignored by a global excludes file looks clean to plain `git status` but is dirty to
  Corvint, which reads status without global configuration (decision 0158), so the operator could
  not find it.
- V1-1757: Git honours `info/grafts` even with `GIT_NO_REPLACE_OBJECTS=1`, and a shallow boundary
  cuts parents. Only appflows refused such checkouts (AFU-V1-068).

## Decisions

- AFP-V0-162: both refusals now name up to eight quoted dirty paths, `(+N more)`, and state the
  global-excludes cause and fix (tracked `.gitignore` or `.git/info/exclude`). The AFP-V0-044
  warning stays path-free (decision 0519 text) and the MCP `MIXED` binding stays count-only; both
  are residuals needing an owner amendment or a wire change. The overview refusal was already
  delivered by RGV-V0-050 (013f8338a).
- EEP-V0-028: the appflows guard moved unchanged into `internal/gitancestry.Rewritten`; appflows
  calls it, and external-evidence freshness degrades to `revision-unavailable` under it.
- V1-1757 audit (ancestry reads that gate evidence or selection), guarded now: appflows
  (repositories, selection via the bound pin), extevidence freshness. Still unguarded, residual:
  dogfoodflow check/batch/batch_dropped/migration/finishlane/flow, postmergeconnector/git.go,
  postmergemetrics/git.go, witness/witness.go, contextindex/range_impact.go, cmd/corvint
  runs_baseline.go, breakagemap/compile.go, dashboard/repository/trace.go, appflows/review.go,
  localcompletion/storage.go, sharedpath, upgradechanges, tracerecordrepo, gitnotes, genesis
  guidance and the Tasks store ancestry reads (outside this lane's scope). Root-commit identity
  reads (`rev-list --max-parents=0`) are also graft-sensitive. Dogfood guarding needs care: a CI
  job checks out with `fetch-depth: 2`, so refusing on shallow there must be decided first.
- V1-1766 skipped: AFP-V0-072 already removed the pure basename match; what remains selects through
  anchored or climbing directory tokens AFP-V0-072(2) keeps on purpose (`../../internal` walkers in
  obscorpus and semescalate are real readers). Separating string-predicate literals from path reads
  needs plugin data flow, a design decision. A tried narrowing broke
  `TestSealedChangeEvidenceReadersAreNarrowed_AFPV0152` and was reverted.
- V1-2071 skipped: affected already reads an AGENTS.md `## Verify` block; a repository that
  declares its gate under another heading needs a declaration-grammar decision, and impact's
  receipt command is a wire contract.
- V1-0752 already delivered: 2026-10-20-v1-0752-merge-queue-closes-ratchet-skew.md (merge_group
  CI runs unbounded-readers-check; the main ruleset requires it in the merge queue).

## Evidence

- `TestAffectedMixedRefusalNamesGloballyExcludedPath_AFPV0162` fails without the detail (count-only
  stderr) and passes with it; `TestAffectedDirtyDetailIsBounded_AFPV0162` passes; existing
  require-clean, path-list and mixed-range tests pass.
- `TestFreshnessDistrustsRewrittenAncestry_EEPV0028` fails without the guard (`provider-ahead`,
  `unrelated-history`) and passes with it; `TestRewrittenNamesGraftsShallowAndUnreadable_EEPV0028`
  and `TestAFUV1068AncestryRewrittenRefused` pass.

## NOT_RUN

`make gate` (owner preference for scoped lanes); full package suites beyond the focused tests.

## Rollback

Revert the lane commits; the guard move is behaviour-neutral for appflows.
