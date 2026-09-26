# Decision 0418 — Shard the go-product root Go suite across five runners

Date: 2026-09-25. Status: proposed (ticket V1-0357); takes effect when the owner merges the
`.github/` change under decision 0390. Adds `AFP-V0-022`; `AFP-V0-013` and `AFP-V0-016` are
unchanged.

## Context

The `go-product` job (`timeout-minutes: 75`) ran the whole root suite on one runner with
`go test -json -p 1 -count=1 -race -timeout 50m ./...`. On main run 36208718703 that step took
69.5 minutes (01:33:15Z to 02:42:42Z), within 6 minutes of the job limit. The per-package
elapsed times sum to 65.6 minutes over the 220 packages that ran tests; the largest are
`cmd/corvint` 460 s, `internal/tasks/authority` 389 s, `internal/doccorpus` 217 s,
`internal/extevidence` 194 s and `internal/lrfrepo` 194 s. PR #245 hit the limit twice. The job
runs on every PR, because the PR narrowing pins are empty.

`-p 1` stays within each runner: decision 0082 serialised package scheduling, and `AGENTS.md`
records that `cmd/corvint` fails its deadline under load. Parallelism has to come from separate
runners.

## Decision

1. The matrix job `go-product-shard` runs five shards with `fail-fast: false`. Each shard does
   the same checkout, setup-go and host-tool provisioning as before, and runs the same
   `go test` command over the packages `script/go-test-shards.sh SHARD 5` assigns it from
   `go list ./...`.
2. The assignment is greedy longest-first over `script/go-test-shard-weights.tsv` (package, whole
   seconds, from run 36208718703; sub-second rounds to 1). Ties go by package name and then to
   the lowest shard. A package without a weight weighs 1 second; a weight for a package that no
   longer exists is ignored. So every listed package is in exactly one shard, and stale weights
   only unbalance the shards. Refresh the weights from a later main run's `go test -json` when a
   shard drifts well above the others.
3. The static, formatting, build and interop steps move unchanged to a separate job,
   `go-product-static`, which runs beside the shards so it adds no wall-clock time. It omits the
   host-tool provisioning and full history: the static steps do not need them, and the
   `go-interop` job already runs the interop tests from a plain shallow checkout.
4. `go-product` stays the required check name (decision 0390). It needs both jobs, runs under
   `if: always()`, and fails unless `needs.go-product-shard.result` and
   `needs.go-product-static.result` are both `success`. So a failed, cancelled or skipped shard
   fails the check; a skipped required check would count as passing.
5. If the trusted PR pins are set, shard 0 alone captures and builds the pinned tools, runs the
   AFP-V0-013 driver (or its full fallback over `./...` when the build failed), and prints the
   Selection audit. Shards 1 to 4 run no tests.

Predicted test time per shard from the weights: 791 to 792 s (13.2 minutes), 49 or 50 packages
each. Each shard also compiles its own race-instrumented dependencies, which the single job
shared; about 4 minutes of the old step was outside package elapsed time, so expect each shard's
test step near 15 to 17 minutes. The expected `go-product` wall time is the slowest shard plus
job start-up, against 71 minutes before.

## Rollback

Revert the `ci.yml` change to the single `go-product` job, and remove `script/go-test-shards.sh`,
its test, the weights file, the Makefile target and `AFP-V0-022`. The ruleset needs no change,
because the required check name stays `go-product`.
