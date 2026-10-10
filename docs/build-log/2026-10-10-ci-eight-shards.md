# CI go-product shards from 6 to 8: V1-1094

Contracts: AFP-V0-022 (complete-universe partition; the count is a matrix parameter) and AFP-V0-024
(main-push reuse needs one record per shard of the current count). No requirement text changes:
AFP-V0-022 already makes the partition digest and shard count part of the AFP-V0-014 frozen
identity without naming a count, and AFP-V0-024 speaks of "the current shard count".

## Change

- `.github/workflows/ci.yml`: the `go-product-shard` matrix becomes `[0..7]`, `REUSE_SHARDS` and the
  `ci-shard-cost-drift` `SHARDS` become `"8"`, in the same commit.
- `.github/workflows/pr-tests-qualification.yml`: the frozen `shards` input defaults to `8` (it still
  said `4`, stale since the 2026-10-05 move to 6). This is the count a V1-0039 campaign binds.
- `.github/cishards/package-costs.json`: refreshed by `tools/ci-shard-costs refresh` from run
  38072785874 (tested merge `b9a965b705e9c58b6e5b661f9eea130d9acd20f0`, parents `8b6af3ee`, the
  `internal/tasks/store` speedup, and the V1-0639 head), all 320 packages. The previous table still
  carried the pre-speedup store cost (1,872 s; observed now 1,055 s).
- `tools/ci-reuse-plan/workflow_test.go`: `TestAFPV0024WorkflowShardCountsAgree` fails when
  `REUSE_SHARDS`, the drift report's `SHARDS`, or the qualification default and its remark differ
  from the matrix length. Negative controls: `REUSE_SHARDS: "6"` against an 8-entry matrix, and the
  6-entry matrix against the new qualification default, each fail it.

## Re-pinned identity

Partition profile digest (`ci-shards --profile`, `corvint-ci-partition/1`):
`92d94f46fbd748c7c1be4672cb4f8d2f81b9783988c22fb9967379391c3a86b3` (6 shards, table from run
38055182050) becomes `f7f976f78675c72d2763838ca25cf6538149784663c20b41702d76a47e3a6da4` with 8
shards. The partition sources are unchanged; only the table bytes move the digest. No selection pin
is set (`CORVINT_PR_TOOL_SOURCE` and `CORVINT_PR_QUALIFICATION_SOURCE` are empty) and no qualification
row exists (V1-0039 not run; V1-0811 forward rows not recorded), so nothing qualified is invalidated;
a future campaign binds this digest and 8.

## Replay (inference, not hosted timing)

Package elapsed times of run 38072785874 placed by each table (seconds of test time per shard; the
refreshed table is in-sample):

| table | 6 shards max | 8 shards max | 8 shards, non-store max |
| --- | --- | --- | --- |
| previous (run 38055182050) | 1,313 | 1,055 | 939 |
| refreshed (run 38072785874) | 1,163 | 1,055 | 845 |

On this tree the whole `internal/tasks/store` package (1,055 s) is the 8-shard floor, not
`cmd/corvint` (688 s in this run): about 19 min per slowest job with the observed ~100–165 s of
per-shard setup, against about 22–25 min today. The ticket's 17 min target needs the store split of
AFP-V0-041 (PR #736): with it the ideal is about 6,977/8 = 872 s per shard.

## Composition with PR #736 (AFP-V0-041)

#736 changes neither the matrix, `REUSE_SHARDS`, `SHARDS` nor the cost table, so the merge is
textual-clean. Its slice placement takes the shard count at run time (`PlanFrom(…, total, …)`,
at most `total` slices per package), so its 2-slice store split is valid at 8. Its committed
`test-slices.json` and the tool defaults (`--shards 6`) were derived for 6 shards from the
pre-speedup run 38055182050 (920 s and 951 s store slices), so after #736 lands the slices are
re-derived with `ci-test-slices generate --shards 8` from one complete 8-shard hosted run and the
defaults move to 8.

## Limits and rollback

- During the switch, a `main` push whose pull request was tested with 6 shards finds no
  `…-of-8` records and runs FULL once; later merges reuse as before.
- About 2.5 runner-minutes of setup and race compile per added shard: about 5 runner-minutes per
  full run.
- Hosted timings come only from this change's own pull-request runs.
- Rollback: restore the `[0..5]` matrix, `REUSE_SHARDS: "6"`, `SHARDS: "6"`, the qualification
  default, and the previous table bytes, in one commit.
