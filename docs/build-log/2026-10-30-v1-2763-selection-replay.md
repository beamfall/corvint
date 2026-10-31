# Selection cost on replayed batches before and after TSP-V0-001 to 005 — V1-2763

## Question

V1-2763 asks whether `corvint affected` still over-selects on real traffic (baseline median 0.867 of
full Go test cost) and for before and after numbers. Lane ci-plan-w8 scoped the global frontiers
(V1-3160, TSP-V0-001 to 005). This entry measures what that changed on real batched change sets.

## Method

- Change sets: the changed paths of the last 60 first-parent merges of the primary checkout's local
  `main` (old-lineage history), keeping sets of 1 to 400 paths. 59 sets qualified; 3 of them touch `.github/`.
- Planner: `corvint --root ROOT affected --no-cache --paths-from SET` from two binaries, one built at
  base `fd51f53a2` (run against a detached worktree of the base) and one at lane commit `7f37edd`
  (TSP-V0-005), run against this lane's tree.
- Cost: the selected `go:` units' share of the summed milliseconds in
  `.github/cishards/package-costs.json`. Shell and other non-Go units are not costed. "CI cost" applies
  the CI rule: an `UNKNOWN` plan or a `.github/` change runs FULL, cost 1.0.

## Result

| Measure | Base `fd51f53a2` | Lane `7f37edd` |
|---|---|---|
| Sets with scope `UNKNOWN` | 59 / 59 | 59 / 59 |
| CI cost, median | 1.0 | 1.0 |
| Plan cost (selected Go share), median | 0.576 | 0.585 |
| Plan cost, mean | 0.552 | 0.560 |
| Selected units, median | 100 | 102 |

The plan cost rose by about 0.01 because the verified `script/heavy.sh` declaration (TSP-V0-005)
makes its declared callers' units dependents of the script, so `internal/dogfoodflow` and its
importers are now selected wherever the script is followed. That is the price of replacing an
`UNKNOWN` frontier with explicit edges; it does not change CI cost while the plans stay `UNKNOWN`.

Unknown causes named on the lane binary (number of the 59 sets each one appears in):

| Cause | Sets |
|---|---|
| `shell:foreign-invocation-unmodelled` | 59 |
| `go:build-constraint-variants` | 51 |
| `go:nested-module-frontier` | 47 |
| `typescript:*` (six reasons, together) | 45 |
| `cfamily`, `dotnet`, `swift`, `ruby` frontiers | 13 |
| `Makefile` as a changed root file | 4 |

If the shell frontier were removed and nothing else changed, only 4 of the 59 sets (`16403cbf6`,
`548ce13ce`, `575939667`, `8fa297eae`) would become `BOUNDED`; 52 have another cause. Real batched
traffic is therefore still 100% FULL in CI after V1-3160, and the single-path improvements recorded in
`2026-10-30-v1-3160-frontier-scoping.md` do not reach batches.

## What still keeps batches UNKNOWN

1. `go:build-constraint-variants` bears on the own files of every constrained package (by design,
   AFP-V0-580). 51 of 59 batches change at least one constrained package, so this is the largest lever.
   Narrowing it needs a per-constraint variant model (for example GOOS/GOARCH variants CI does run),
   which is new design, not a scoping fix.
2. `shell:foreign-invocation-unmodelled` from 42 path-literal shell scripts that a replayed batch
   followed. 132 tracked files name them. 4 of them (`examples/evidence-provider/v0/authoring-proof.sh`,
   `measure-import-cal018.sh`, `measure-worktree-index-share.sh`, `tools/check-analyzer-go.sh`) are
   named by no candidate file, which TSP-V0-005's "at least one caller or mention" rule rejects.
   Declaring them all would need that rule relaxed (a spec amendment) and a per-file caller or mention
   claim for the 132 files, for at most 4 of 59 batches.
3. `go:nested-module-frontier` through `tools/local-authority`, reached via `cmd/corvint/affected.go`.
4. The TypeScript, C-family, .NET, Swift and Ruby frontiers from the adapters' own fixture trees.

## Outcome

- V1-2763's before/after numbers are measured: CI cost median 1.0 before and after on 59 real
  batches; plan cost median 0.576 to 0.585. Over-selection on batched traffic is not reduced, because
  every batch is still `UNKNOWN`.
- AC3 (recall and cost from forward shadow rows) is not measured here: no forward shadow rows exist
  for the lane binary, and the shadow pin is outside this lane (V1-3309, V1-3314).
- The ordering of further levers (build-constraint variants first, the shell residual second) is a
  decision for the orchestrator.

## Rollback

This entry changes no behaviour.
