# Darwin shards: no host-load queue, named failures (V1-4732)

Date: 2026-11-06. Requirement: CIN-V0-4625 (amended).

## Finding

The first hosted run of the sharded `go-macos` workflow (run 42173342735) showed two avoidable
costs:

- Every shard spent 120 s in `heavy.sh`'s host-load queue (load 11-25 on a 3-CPU runner) before
  running any test. A hosted runner is dedicated to the job, so the queue protects nothing. That
  is roughly 8 runner-minutes per pull request across four shards.
- On failure, the job printed the last 40 raw `go test -json` lines. Those never named the failed
  tests, so diagnosis required downloading the shard artifact.

## Decision

`shard.py` sets `HEAVY_LOAD_WAIT=0` for its `heavy.sh` invocations and prints a failure summary:
each failed package/test with up to six of its output lines (at most 60 failures), falling back to
the non-JSON tail when no test event failed (a build failure, for example).

The Darwin test failures themselves (Tasks `UNSUPPORTED_FILESYSTEM` under the runner TMPDIR,
`linecov` window order, store intent sweep, local-authority umask, a companionrelease TempDir
cleanup) were already tracked by V1-4701, the local-authority umask ticket and the PUBV0965
flake ticket. This change does not alter them.

## Evidence

`python3 -B .github/macos/shard_test.py` (new `test_CINV04625_failure_summary_names_failed_tests`,
plus a `HEAVY_LOAD_WAIT` assertion) and `select_test.py` pass; the summary was replayed on the
retained shard 0 and shard 3 artifacts and names every failed test.

## Rollback

Revert this commit; shards then wait on host load again and print raw tails.
