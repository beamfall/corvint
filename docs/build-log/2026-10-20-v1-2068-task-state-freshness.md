## 2026-10-20 V1-2068: Tasks store projection dirt is source-clean (DIRTY-CACHE-017, proposed)

### Reproduction

Fixture: a Git repository with `go.mod`, `main.go` committed and only `.taskman/queue.json`
modified. Before (base `0617097a8` build): `corvint impact main.go` reported
`freshness.state=mixed-worktree`, `mixed_paths=[".taskman/queue.json"]` and the DIRTY-CACHE-014
MIXED warning; `corvint impact --require-clean main.go` abstained with `NEEDS_WIDENING`,
reason `mixed-worktree`. The dogfood prompt packet behind host hooks reported
`freshness=mixed-worktree` for the same dirt (the session-start hook in this checkout shows it).

After: `state=READY`, `freshness.state=fresh`, `mixed_path_count=0`, no warning or abstention,
and `freshness.task_state={class: TASK_STORE_PROJECTION, path_count: 1, paths: [...]}`. Adding a
source edit restores the full MIXED receipt with every dirty path and no `task_state`.

### Decisions

- The projection layout moves to the leaf package `internal/taskprojection`;
  `affected.IsTaskStoreProjectionPath` delegates, so AFP-V0-088 and DIRTY-CACHE-017 cannot diverge.
- Only an all-projection dirty set is source-clean. A mixed set keeps every path in
  `mixed_paths`; splitting it is not attempted, so no source path can be dropped from the receipt.
- `analyzerSchemaID` is bumped to `corvint-analyzer/131` because the IDX-SNAP-V0-017 audit digests
  every contextindex source; extraction and encoding are unchanged.
- Unchanged by design (DIRTY-CACHE-017 non-goals): trace-learning eligibility, batch fallback,
  `impact --working-tree-untracked`, repository `worktreeState` and handoff dirty digests.
  `corvint context` task packets carry no freshness member, so the ticket's "context packet"
  criterion is met by the query/impact receipt and the dogfood prompt packet only.
- `fresh` now means source-clean, a CCF-V1-005 meaning narrowing; owner acceptance is pending.
