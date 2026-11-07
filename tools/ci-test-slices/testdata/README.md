<!-- SPDX-License-Identifier: Apache-2.0 -->
# Parallel placement sample

`run-42106672783-costs.json` retains milliseconds rounded from the 17 longest
passing, unsplit package outcomes in PR #99 run
[`42106672783`](https://github.com/beamfall/corvint/actions/runs/42106672783),
head `e468668f2d2ea198271ce26717f8f8dd6e998203`, merged as lane base
`28a3ff86a2c3d17662765da12cc9251c06466f5f`. Packages with multiple outcomes
(test slices) and failures are excluded. The source archive SHA-256 is
`62a5dd13fd868750203ef09c0783775fb7553bd6fb85a7618719fccc4f122742`.
Raw logs stay in `~/lanes/placement-evidence/`.

The full run includes a failed `internal/tasks/store` slice. This deliberately
small regression sample is not a complete passing run and cannot qualify an
AFP-V0-040 cost-table refresh. Tests create two synthetic streams from these
values solely to exercise replay; those streams are not source CI logs.

`run-42106672783-p1.golden` records the existing serial LPT placement of this
sample over two shards: costs descending, package-name ties, least summed load,
then lowest shard index. It freezes package membership at `p=1` independently
of the new replay options. At `p=4`, with a common 360-second allowance,
parallel-aware placement predicts a max of 723.828 s versus 729.270 s for that
legacy placement evaluated under the same model (CIN-V0-7620).
