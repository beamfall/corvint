# 2026-10-14 CI cancellation waste after measurement (V1-0809)

Human-owned intent: the owner's 2026-10-05 CI audit request ("only run the tests that need
running") and its first step, stopping cancellation waste (V1-0809). The lever, its contract
(AFP-V0-027) and its rollback are recorded in `2026-10-05-ci-cancel-waste-shard-balance.md`. This
entry records the after measurement that V1-0809 asks for. It changes no workflow, test or
requirement.

## Method

The same method as the before figure: the 100 most recent completed runs of `.github/workflows/ci.yml`
(workflow id 361349783), listed on 2026-10-14 with `actions/workflows/361349783/runs?status=completed`.
For each run, `actions/runs/<id>/jobs?filter=all` (every attempt) gives each job's `started_at` and
`completed_at`; runner time is their difference summed over jobs, so jobs that never started count
nothing. Three runs had a second attempt (38494366263, 38476173623, 38371433595), and both attempts
count. The per-run table is `evidence/v1-0809/after-runs.tsv`.

## Result

The window runs from 2026-10-12T16:34:07Z to 2026-10-14T11:54:12Z (about 43 hours). The before window
was about 11 hours on 2026-10-05.

| Event | Conclusion | Runs | Runner-minutes | Share |
| --- | --- | --- | --- | --- |
| `pull_request` | success | 33 | 5,358 | 44.8% |
| `pull_request` | cancelled | 8 | 820 | 6.9% |
| `pull_request` | failure | 3 | 458 | 3.8% |
| `merge_group` | success | 25 | 3,244 | 27.1% |
| `merge_group` | failure | 2 | 283 | 2.4% |
| `push` | success | 22 | 1,366 | 11.4% |
| `push` | cancelled | 7 | 420 | 3.5% |
| Total | | 100 | 11,948 | |

Cancelled runs used 1,240 of 11,948 runner-minutes (10.4%). Before, they used 3,101 of 7,063
(43.9%; the ticket's 42% adds the rounded 31% `pull_request` and 11% `push` shares).
`go-product-shard` jobs account for 10,409 minutes (87%). The median successful run used 153 runner-minutes on `pull_request`, 139 on `merge_group` and
20 on `push` (the AFP-V0-024 and V1-0760 reuse paths).

## Criteria

1. After measurement: the table above. Cancelled share comes from the jobs API, not an estimate.
2. Lever: AFP-V0-027 (`ci:batched`) and its rollback are recorded in the 2026-10-05 entry. No pull
   request has carried the label (`gh pr list --label ci:batched --state all` is empty), so its
   hosted behavior is still `NOT_OBSERVED`. The fall in cancelled share therefore cannot be credited
   to it. The 2026-10-05 decision also told batching agents to stop merging the batch branch back
   into constituents, but this measurement does not check whether they did. The merge queue took
   over the landing path on 2026-10-12, and the race matrix now has eight shards
   (`REUSE_SHARDS: "8"`). This measurement cannot separate those causes.
3. `main` coverage: all 7 cancelled `push` runs were `main` pushes, and each commit has a
   successful `merge_group` run of the same `head_sha`: b9b10b42d, b8c22fb35, 3a1980dc1, 2e32f63d5,
   6ca5aa6c3, 21758ccd8 and 8fb53b265. No `main` tree lost its full-suite evidence.
4. Required checks: ruleset 23699808 still requires `go-product`, `ci-control-plane` and `doc-gates`
   (read back 2026-10-14). This ticket narrows no test.

## Limits

- The two windows differ in length (11 hours against 43) and in pipeline. The after window has a
  merge queue (2026-10-12) and the eight-shard matrix, and every merge runs a `merge_group` run, so
  total runner-minutes per run (71 before, 119 after) are not comparable as a cost figure. Only
  the cancelled share is compared.
- The `ci:batched` hosted path stays `NOT_OBSERVED`.
- The run list is the API's view on 2026-10-14. Run logs and job records expire, so the per-run
  table is the retained evidence.

## Rollback

Delete this entry and `evidence/v1-0809/`. Nothing reads them.
