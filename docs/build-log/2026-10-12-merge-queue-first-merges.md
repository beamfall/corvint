# 2026-10-12 First merges through the `main` merge queue (CORE-GITHUB-MERGE-QUEUE-COMPATIBILITY-1)

Hosted evidence for the queue recorded in `2026-10-12-merge-queue-enabled.md`, plus two corrections
to that entry from its independent review.

## Successful queued merges

Four pull requests merged through the queue on 2026-10-12. For each one, the `CI` `merge_group` run
on the `gh-readonly-queue/main/pr-<N>-<base>` branch passed `go-product` and `doc-gates` on the
queue commit. The `merge-group` job of a `ci-control-plane` `workflow_run` then posted
`ci-control-plane` `success` on that commit as `github-actions[bot]`, and `main` advanced to the
queue commit itself, which is also the pull request's merge commit.

| PR | Queue commit = merge commit | First parent | `merge_group` run | `ci-control-plane` posted | Merged |
| --- | --- | --- | --- | --- | --- |
| #840 | `8fb53b265e1e2a46eb7fe8534c16c4bc288a96bc` | `d67ee35d4` | 38292100504 | 19:02:23Z (run 38294958759) | 19:02:50Z |
| #847 | `21758ccd84d154114fb0bcef00fd98762ba408d3` | `8fb53b265` | 38292857291 | 19:10:12Z | 19:10:36Z |
| #843 | `6ca5aa6c31959203f4a0703e923cd4b3d5a7fd87` | `21758ccd8` | 38295018459 | 19:24:08Z | 19:24:32Z |
| #848 | `2e32f63d5838ce8b34ce3183b5414972a86d429a` | `6ca5aa6c3` | 38295891294 | 19:31:29Z | 19:31:54Z |

This settles the `NOT_OBSERVED` items in the enablement entry: the queue branch form matches the
`ci-control-plane.yml` pattern, `workflow_run` is delivered for `merge_group` runs, and `main`
becomes the tested queue commit. With two builds allowed, #847's and #848's queue commits were built
on the preceding entry's queue commit before it merged, so each run tested the combined tree
including its predecessor. Queue `CI` runs took 21 to 27 minutes, well inside the 120-minute
timeout.

## Corrections to the enablement entry

- That entry says its own pull request (#848) is the first to merge through the queue. #840 merged
  first; #848 was the fourth.
- That entry says every merge runs a second full `CI` on the `main` push. The push skips the root
  race invocation when a successful pull-request run already tested the identical tree
  (AFP-V0-024). That rarely applies after a queue merge, because the pull-request run tested a
  merge with an older `main`. #840's push decision was `FULL`: "no admitted run retained a full
  record of this exact tree for every shard" (`ci-reuse` artifact of run 38295017392). V1-0760
  removes the second run by reusing the queue run.
- Review also found the longest configured `CI` chain is 115 minutes before scheduling delays
  (15-minute planner, 75-minute shards, 20-minute advisory job, 5-minute control plane). The
  120-minute timeout covers it with little slack. Observed runs used about a fifth of it.

## Observed side effect: cancelled `main` push runs

The queue now merges every 8 to 14 minutes, while a full `main` push run takes about 25. The `CI`
concurrency group for pushes is the constant `refs/heads/main` with `cancel-in-progress`, so each
queued merge cancels the previous push run. Push runs 38295017392 (`8fb53b265`) and 38295889750
(`21758ccd8`) ended `go-product` `failure` with `SHARDS_RESULT=cancelled`. Those commits also carry
the `go-product` `success` of their queue runs, so the merge gate held, but `main` shows a failing
check run on them. V1-0728 owns that fix. Queue runs are not affected, because each queue branch has
its own concurrency group.

## Limits

Failed and stale queue groups did not occur. V1-1319 retains that evidence when ordinary queue
traffic produces it. Deliberately failing a queue entry needs owner authorization first.
