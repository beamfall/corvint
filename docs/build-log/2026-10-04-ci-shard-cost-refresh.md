# 2026-10-04: CI shard cost table refresh and drift check (V1-0714)

## Finding

`.github/cishards/package-costs.json` was written once (960ee04c, 2026-10-01) and had no
regeneration path. Against hosted run
<https://github.com/beamfall/corvint/actions/runs/37198363933> (main at `e5d47da8`, 295 packages,
5,635s of package test time) it listed 253 packages. `internal/testacceptance` was recorded at
3.2s and observed at 953.5s; `internal/behaviorfalsify` at 910s and observed at 98s; 42 executed
packages had no entry and were charged the median. Observed shard test sums were
1706/1544/1135/1248s against an ideal of 1409s, so the slowest shard set the PR wall clock about
five minutes above what the same work allows.

## Change

- `tools/ci-shard-costs refresh` rebuilds the table from the terminal `go test -json` package
  outcomes of one complete hosted run (raw streams or `gh run view RUN --job JOB --log` output)
  and records the run and revision. It refuses a failed or repeated package, logs lacking a package
  the current table lists (unless `--allow-removed`), and any table the partition would reject, so a refresh cannot silently put CI on the lexical fallback.
- `tools/ci-shard-costs check` prints `drift`, `missing` and `stale` lines and exits 1 when any
  exist. Default bounds: factor 2, ignoring differences under 10s.
- `cishards.Costs` exposes the partition's own table admission so the tool and the partition
  cannot disagree about validity. Membership logic is unchanged.
- The table is refreshed from run 37198363933. Replaying that run's package times through the new
  partition gives shard sums of 1408.6/1409.6/1408.2/1408.6s.

## Evidence and limits

- Independent review found no correctness defect. Its robustness findings (partial logs accepted,
  `--factor NaN` suppressing drift, non-atomic table write) are fixed here. One is retained: a log
  line whose prefix contains `{` before the JSON object is skipped, which `check` later reports
  as `missing`.
- Before refresh, `check` on the four shard logs reported 4 `drift` and 42 `missing` lines; after
  refresh it reports none.
- The balanced sums are a replay of one run's times, not a hosted measurement of the new
  partition: hosted timing after this change is `NOT_OBSERVED` until a run on the merged table.
- The check is not wired into a workflow. It is operator evidence, run when shard times diverge.
- `internal/testacceptance` is expected to get faster under V1-0715; `check` will report it and
  the table should then be refreshed again.

## Rollback

Revert this change. The previous table and partition bytes return; an invalid or missing table
still yields the complete lexical round-robin partition.
