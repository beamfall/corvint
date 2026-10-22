# 2026-10-21 tasksperf-1021o: bounded shards for slow Tasks test packages

Tickets: V1-1956, V1-2227. Requirement: ASD-V0-026 (accepted under the owner's standing auto-accept
of 2026-10-11).

## Problem

`./internal/tasks/store` (about 1525 s under load) and `./internal/tasks/cli` (about 616 s) exceed
the 600 s foreground limit of an agent command, so lanes split them into hand-written `-run`
patterns that nothing checked for completeness (V1-1956), and the V1-1499 acceptance run over the
whole store package could not run in one invocation (V1-2227).

## Decisions

- A new `tools/test-shards` command, not the CI slice file: `.github/cishards/test-slices.json` is
  generated from hosted CI timings, covers only allow-listed packages (store, not cli) and sizes
  slices for CI shards; editing it or its allow-list is a `.github` change outside a lane.
- The local split reuses the CI catch-all idea (AFP-V0-041): named shards `-run` their names and the
  last shard `-skip`s the union, so exactly-once holds by construction and the "new test not
  covered" failure mode in V1-1956 criterion 2 cannot occur; `TestASDV0026_*` checks the invariant,
  including unlisted names that are prefixes or extensions of listed ones.
- Placement is by FNV-1a hash of the name, not by cost, so adding a test never moves another and no
  timing file needs maintenance. The cost is imbalance; the measured worst shard is 447 s at N=8.
- `make test-shard` wraps both the listing build and the run in `script/heavy.sh`.

## Evidence

- `go test ./tools/test-shards/` passes; `go vet` and `gofmt -l` clean.
- Host run, 2026-10-21, `-p 4`, one shard per command through the lane semaphore: store N=8 ran
  549/549 listed tests exactly once (package times 235, 262, 268, 242, 301, 447, 191, 141 s); cli
  N=4 ran 619/619 exactly once (130, 132, 154, 151 s). Coverage was computed from the `go test -json`
  pass/fail/skip records against `go test -list .`.
- Store shard 0 failed once in `TestPSRSweepSuccessReplaySuccessor` ("not freed") at load 172; the
  test passed three times alone. Its file records this load-sensitive quiet-host assumption
  (V1-1324); it is unrelated to sharding.

## NOT_RUN

- The CI-scale reference-host timing of every shard: lane measurement only, under shared load.

## Rollback

Revert the commit: it removes `tools/test-shards`, the `test-shard` make target, the
`docs/LANE-ORCHESTRATION.md` paragraph and ASD-V0-026. No product, store or CI state changes.
