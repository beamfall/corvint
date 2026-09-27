## 2026-09-26 V1-0357: go-product runs as four test shards behind one required check

The `go-product` job ran the whole race suite in one job. On main (run 36276711799, job
108500814020) the tests took 58 min 22 s, the static checks 45 s and interop 4 s, against a
75-minute job timeout. The 248 root packages sum to 55.2 minutes of package time. The largest are
`cmd/corvint` (407 s), `internal/tasks/authority` (234 s), `internal/extevidence` (191 s),
`internal/doccompiler` (178 s) and `internal/lrfrepo` (166 s).

Decision: `.github/workflows/ci.yml` runs the tests in a matrix job, `go-product-shard`, with four
shards. Each shard runs the unchanged `go test -json -p 1 -count=1 -race -timeout 50m` argv over the
packages whose 1-based line number in `go list ./...` is congruent to `strategy.job-index` modulo
`strategy.job-total`. Every package lands in exactly one shard, and the shard count comes only from
the matrix. Shard 0 also runs the static, formatting, Windows build and interop steps. The required
check `go-product` is now a job that needs every shard and fails unless the matrix result is `success`.
It runs under `if: always()`. Without that, a failed shard would skip it, and a skipped required
check counts as passing.

From the recorded package durations, the shards carry 7.7, 12.8, 15.9 and 18.7 minutes of package
time, plus per-shard setup and compilation. A greedy split by duration would give about 14 minutes
each, but it needs a duration table that goes stale as packages change; the round-robin split needs
no table.

Pinned driver (AFP-V0-013): the trusted pins are empty today. When they are set, only shard 0
captures and builds the pinned tools. It runs the driver, or the whole root suite when the driver did
not build, exactly as the single job did. The other shards exit without testing. A fallback therefore
still runs the full root suite in one job, and main, static, vet, format, build and interop stay full.

Unchanged: the provisioning step, the 75-minute job timeout (the 50-minute per-package hang detector
must fit inside it, decision 0082), the three least-privilege properties
(`script/check-ci-least-privilege.sh` passes), and the required check names in ruleset `main`.

Set aside: raising the timeout, which would not shorten a PR run; splitting race from non-race
packages, since every package runs under `-race`; a committed duration table for greedy balancing.

Acceptance: `go-product` on main finishes with at least 25% headroom on three consecutive main runs,
NOT_OBSERVED until after merge. This is a `.github/` change, so it merges only after the owner posts
`ci-control-plane` success on the exact head (decision 0390). Rollback: revert the change, and
`go-product` returns to one job.
