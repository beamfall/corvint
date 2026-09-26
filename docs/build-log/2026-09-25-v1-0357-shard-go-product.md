## 2026-09-25 V1-0357, AFP-V0-022 (proposed), decision 0418: shard the go-product Go suite

Baseline: on main run 36208718703 the `go-product` test step took 69.5 minutes of the job's 75
(01:33:15Z to 02:42:42Z); static checks 58 s and interop 5 s followed. Per-package `go test
-json` elapsed times summed to 65.6 minutes over 220 packages, led by `cmd/corvint` 460 s,
`internal/tasks/authority` 389 s and `internal/doccorpus` 217 s.

`ci.yml` now runs the suite as five `go-product-shard` runners, `-p 1` within each, over the
packages `script/go-test-shards.sh` assigns by greedy longest-first on
`script/go-test-shard-weights.tsv` (those 220 elapsed times, whole seconds). Static, build and
interop steps moved unchanged to a parallel `go-product-static` job. `go-product` keeps the
required check name, needs both, and fails unless both results are `success`. With trusted PR
pins set, only shard 0 runs the driver.

Over the current 248 packages of `go list ./...` the partition is complete and disjoint, with
predicted test time 791 to 792 s (13.2 minutes) per shard, 49 or 50 packages each. Each shard
compiles its own race-instrumented dependencies, and about 4 minutes of the old step was outside
package elapsed time, so the expected shard test step is about 15 to 17 minutes. Evidence:
`make go-test-shards-test` (4 cases), `actionlint`, `make ci-least-privilege-check
ci-least-privilege-test`, and the documentation checks. The hosted sharded run is NOT_RUN until
the PR's CI; the ticket's acceptance needs three consecutive main runs with 25% headroom.
