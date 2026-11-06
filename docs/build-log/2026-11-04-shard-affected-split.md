# V1-4355: split internal/liveverify/affected and refresh shard costs

Base: `f13f52b1607222033b089144a744fedae0d35239` (`origin/main`). Ticket: V1-4355.
Requirements: AFP-V0-164 (amendment), AFP-V0-2181, AFP-V0-022.

## Problem

Commit `8046107c7` made placement use the shard's real `-p` parallelism (AFP-V0-2181), but Go-test
shard spread stayed wide: runs 41760796927, 41758354891 and 41756777814 measured max/min 2.26, 1.57
and 2.11. Replay of run 41760796927 (16 shards, revision `f13f52b16`) with its own measured costs
showed why. Placement itself was balanced; the inputs were stale or too coarse:

- `internal/liveverify/affected` took 746.5 s, unsplit, against a committed cost of 532 s and an
  ideal shard of 644.2 s. No placement can bring a shard holding it below 746.5 s.
- `internal/tasks/store` slices were uneven (one 535 s, others 160-270 s) because tests were added
  after the previous generation.
- Other packages had drifted, for example `internal/doccorpus` (365 s measured, 221 s committed).

## Change

1. `internal/liveverify/affected` added to `.github/cishards/test-split-allow.json`. Order review:
   no TestMain; package variables are read-only tables; `sync.Once` fixtures are per process; tests
   use private temporary repositories. A shuffled whole-package `-race` run passed (452.2 s).
2. `package-costs.json` refreshed with `tools/ci-shard-costs refresh` from run 41760796927.
3. `test-slices.json` regenerated with `tools/ci-test-slices generate --target 300s` from the same
   logs. Slices: `cmd/corvint` 4, `internal/liveverify/affected` 3, `internal/tasks/cli` 2,
   `internal/tasks/store` 5; largest slice 294.6 s (AFP-V0-060 bound 400 s). The ideal-share
   default target (644 s) would have left slices above the bound, and a 400 s target left one
   `affected` slice at 435.8 s, so 300 s was used.

## Replay evidence

`tools/ci-test-slices replay --shards 16` over run 41760796927's logs:

| files | min shard | max shard | max/min |
|---|---|---|---|
| observed run | 402.8 s | 959.5 s | 2.38 |
| previous slices and costs | 426.1 s | 880.9 s | 2.07 |
| refreshed slices and costs | 643.3 s | 644.6 s | 1.002 |

Narrowed selections, through `SelectFromParallel` with the refreshed files (random package subsets,
seeds 1-5): at 60 percent, ratios 1.002-1.555; at 30 percent, 1.058-3.431. In every case above
1.25 the slowest shard holds exactly one indivisible unit (`internal/companionrelease` 431.2 s and `internal/doccorpus` 365.3 s, whole packages
outside the allow-list, or 294.6 s and 225.7 s units), so the wall equals the lower bound
max(largest unit, ideal). The previous files under the same subsets gave a 532.4 s floor, set by
the unsplit `affected` package. Max/min is therefore not a sufficient narrowed-plan criterion; the
achieved wall is.

## Limits

- The post-merge CI spread of at most 1.3 is `NOT_OBSERVED` until a FULL 16-shard run on this tree.
- Splitting the remaining large unsplit packages (`internal/companionrelease`, `internal/doccorpus`) would lower narrowed walls
  further, but each needs its own order-independence review; not done here.

Rollback: restore the previous `test-split-allow.json`, `package-costs.json` and `test-slices.json`.
