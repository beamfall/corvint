# 2026-10-12 Citation baseline split by class at the V1-1276 cutover (V1-1281 criterion 3)

Branch `claude/v1-1281-criterion3` from `origin/main` 9deae4838 (the V1-1276 merge, #837, after
which `cem prepare` writes `cem/1.0`). V1-1281 acceptance criterion 3 asked for the class-split
citation baseline (`DCW-V0-049`, `CEM-V1-106`) to be rerun over the first 30 post-cutover maps.
At the merge commit exactly one committed map is post-cutover, so this entry retains the rerun
over every committed map at that commit and records the shortfall as a limit instead of waiting.

## Method

`GOTOOLCHAIN=local go run ./tools/cem-citation-baseline --root . --maps .corvint/changes --summary
docs/build-log/evidence/v1-1281/citation-baseline-summary.json >
docs/build-log/evidence/v1-1281/citation-baseline-maps.jsonl` from the repository root of a plain
clone at 9deae4838. The tool runs `cem status --citation` in process for each map and takes a
`cem/1.0` map's class from the status envelope's `citationClass`; a `cem/0.x` map is classed from
its bind range. Retained evidence: `evidence/v1-1281/citation-baseline-maps.jsonl` (one row per
map, the V1-1262 row shape plus `class`) and `evidence/v1-1281/citation-baseline-summary.json`
(aggregate for all maps and per class).

The first run read the one `cem/1.0` map as an error row, `artifacts-required`: the stable
verifier requires an explicit absolute `--artifacts` root, which the bind loop passes
(`CEM-V1-147`) but the tool did not. The tool now appends `--artifacts <absolute root>` for a map
declaring `cem/1.0`; `TestBaselineDCWV0049SplitsRowsByCitationClass` binds a `cem/1.0` map with an explicit
`--spec cem/1.0` prepare and requires its row to be valid, class `change`, with its hunk counted. The two
runs differ in that row only.

## Result

| Measure | Value |
| --- | --- |
| Maps | 615 (614 `cem/0.2`, 1 `cem/1.0`), 614 with a citation block |
| Class | 615 `change`, 0 `chore` |
| Hunks classified | 21749 |
| `held` / `mechanical-proved` | 0 / 0 |
| `anchored` | 1235 (372 maps have at least one) |
| `cited` | 20475 |
| `unknown` | 39 hunks in 33 maps |
| Annotation hunks | 31 in 23 maps |
| Invariant maps | 193 of 614 (184 of them cite exactly one evidence item) |
| Specificity | 222 positive, 346 zero, 46 negative |
| Max fan-out ≥ 100 | 19 maps (largest 291) |

The post-cutover map `860f59ea` (V1-1276's own sealed map) is valid, class `change`, 156 hunks all
`cited` against 156 distinct evidence items, max fan-out 1, specificity 0, not invariant. The 593
V1-1262 rows reproduce byte for byte apart from the added `class` member; the 22 further rows are
the maps committed between the two runs. `f0fda542` still fails `git-budget-exceeded` before any
citation is computed, as in V1-1262.

## Retained limits and NOT_PRODUCED

- One of the first 30 post-cutover maps existed at 9deae4838; the remaining 29 accrue one per
  sealed lane. The criterion is PARTIAL here, not met: V1-1277 reruns the tool after it migrates
  every committed map to `cem/1.0` (`evidence/v1-1277/`), which measures the migrated set, and the
  first-30 rerun needs a later entry once 30 post-cutover maps are committed.
- No committed map carries a `role`, so the `chore` class is empty: the bind and seal chore commits
  of a lane are not separately mapped, and `markStableChoreMap` writes a role only for a change whose
  whole range touches CEM map paths. The split is exercised by the tool's test, not by this data.
- The measurement is the native consumer only; `interop/cem01-go` has no citation block to compare.
