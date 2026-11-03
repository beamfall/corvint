<!-- SPDX-License-Identifier: Apache-2.0 -->
# Wall-time shard placement and refreshed tables (V1-4355)

`AFP-V0-2181` is experimental under the owner's standing lane auto-accept (2026-10-11); it is
not an owner read-back. It amends the `AFP-V0-022` placement (and, through it, `AFP-V0-590`) and
`AFP-V0-540`'s single `SHARD_TEST_P` setting. Branch `claude/shard-walltime-balance`, base
`32e93e84d7b2bfc4deeff935b6ffa9c2c84239fe`.

## Diagnosis

FULL run `41568676566` (source `32e93e84d`, sixteen shards) spread its shards from 491.6 s to
884.8 s, max/min 1.80, against an ideal share of 664.1 s. Every hosted shard and the merge queue
run `-p 1` today (`CORVINT_SELF_HOSTED` has been off since 10-28), so the spread is not a `-p`
effect. Replaying the committed tables gives 834.0 s / 491.6 s; the cost table, refreshed from
run `41429718453`, no longer matched the tree. The cause was stale tables.

## Decision

- Placement minimizes the predicted shard wall time `max(longest*p, sum)/p` for the shard's
  `go test -p`. Units are placed longest first, ties go to the lower load and then the lower
  index, and sibling slices still avoid each other. At `p = 1` this is the old least-loaded rule,
  oracle-tested over 300 seeded fixtures, so the change has no effect on the hosted placement
  today. It takes effect on self-hosted pull-request shards at `p = 4`, where least-loaded can
  stack a long unit with a full load. Example: units 80, 50, 50, 50, 40, 40 on two shards at
  `p = 2` give a least-loaded wall of 85 and a wall-time-placed wall of at most 80.
- `ci-shards --parallel` defaults to `SHARD_TEST_P`. The narrowing gate step now sets
  `SHARD_TEST_P` with the tests step's expression, so the pinned narrowed placement equals the
  recomputed one. The `runner` context is not available in job-level `env`, so the value is set
  twice; `TestAFPV0540PullRequestParallelismBinding` now requires exactly the two settings.
- `package-costs.json` was refreshed with `ci-shard-costs refresh` from run `41568676566`: 426
  packages, 10626.3 s. `test-slices.json` was regenerated with `--target 350s`, giving
  `cmd/corvint` 4 slices, `tasks/cli` 2 and `tasks/store` 5, all under the 400 s bound. A 7m
  target was tried first and was refused by the `AFP-V0-060` 350 s rule. The allow-list is
  unchanged.

## Predicted before/after (16 shards, run 41568676566 costs; predictions, not hosted measurements)

| Tables and placement | max | min | max/min |
|---|---|---|---|
| observed run | 884.8 s | 491.6 s | 1.80 |
| old committed tables, replayed | 834.0 s | 491.6 s | about 1.70 |
| refreshed costs, old slices | 790.9 s | 561.0 s | about 1.41 |
| refreshed costs and slices (ci-test-slices replay) | 664.5 s | 663.3 s | about 1.002 |
| refreshed costs and slices (`walltime_test.go` model, `p = 1`) | 664.3 s | 663.3 s | 1.001 |

Frozen replay in `TestAFPV02181Run41568676566Replay`:

| Case | `p` | predicted shard wall | max/min | lower bound | least-loaded max |
|---|---|---|---|---|---|
| FULL | 1 | 663.3 to 664.3 s | 1.001 | 664.1 s | 664.3 s |
| FULL | 4 | 297.4 to 532.4 s | 1.790 | 532.4 s | 532.4 s |
| heaviest 40 packages | 1 | 483.7 to 538.4 s | 1.113 | 532.4 s | 538.4 s |
| heaviest 40 packages | 4 | 238.2 to 532.4 s | 2.235 | 532.4 s | 532.4 s |
| top 3 plus every 12th | 1 | 106.8 to 532.4 s | 4.987 | 532.4 s | 532.4 s |
| top 3 plus every 12th | 4 | 46.5 to 532.4 s | 11.460 | 532.4 s | 532.4 s |

The test asserts max/min of at most 1.25 for FULL and the heaviest-40 subset at `p = 1`. In
every case it asserts a max of at most 1.25 times the lower bound and no worse than least-loaded.

## Limits

- The unsplit `internal/liveverify/affected` takes 532.4 s, which exceeds the per-shard share of
  the `p = 4` and sparse cases. That one package sets the max there, and no placement can reach
  max/min 1.25. Splitting it needs an allow-list entry with a package-isolation justification,
  which is an owner decision.
- The model omits per-invocation build overhead and background slices at `p > 1`.
- `tools/ci-test-slices` replay and generate, `internal/sessiontest` `PlanFrom` and
  `tools/corvint-pr-tests` `planShards` still place at `p = 1`. That is correct for every current
  hosted run.
- Hosted wall-time balance is `NOT_OBSERVED` until a complete new run.

Rollback: revert the V1-4355 change. Placement at `p = 1` is identical, so the refreshed tables
may stay or be reverted with it.
