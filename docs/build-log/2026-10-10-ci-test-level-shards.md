# V1-1096: test-level slices of oversized packages across CI shards

Date: 2026-10-10

## Problem

AFP-V0-022 partitions whole packages across the six full-CI shards, so the slowest package sets a
floor under the slowest shard. Main run
[38055182050](https://github.com/beamfall/corvint/actions/runs/38055182050) tested
`e0ef9aa891edbd84b19b1f30f12bc6ed13635a96`. Its 320 terminal package outcomes sum to 7,684.8 s,
which gives an ideal share of 1,280.8 s per shard. `internal/tasks/store` alone took 1,871.8 s,
and its shard summed to 2,100.8 s. `cmd/corvint` took 764.6 s, which is below the ideal share.

## Decision

The new requirement is AFP-V0-041 in `docs/specs/affected-plan-v0.md`. It is proposed and
experimental, pending owner acceptance, and it does not add an owner decision.

**Exactly once.** A split package runs as K invocations:

- K-1 named slices, each `-run '^(A|B|...)$'` over plain top-level names;
- one catch-all, `-skip` over the anchored union of every named slice.

`go test` splits a pattern on `/`, and on `|` outside parentheses. Grouping the alternation keeps
it one element, and a one-element pattern matches only top-level names, so subtests always run
with their parent. The anchors keep `TestA` from matching `TestAB`. The same flags filter examples
and fuzz seeds. `-list` also prints benchmarks, so the generator drops them.

What happens to tests that change after generation:

- a test added or renamed after generation runs once, in the catch-all;
- a removed name matches nothing.

`TestAFPV0041SlicesRunEveryTestExactlyOnce` runs real slices over a fixture module containing
`TestMain`, subtests, a parallel test, a fuzz seed, an example, a benchmark, and added, removed and
renamed names. With the end anchor removed, it fails with "TestAB ran 2 times across slices".

**Authority and fallbacks.** A package splits only when both of these hold:

- the project-owned allow-list `.github/cishards/test-split-allow.json` names it, with a reason;
- the slice file `.github/cishards/test-slices.json` splits it validly.

The two allowed packages are `cmd/corvint` and `internal/tasks/store`. Their `TestMain` functions
change only their own process (environment, re-exec roles, `sync.Once` roots), and no test is known
to depend on order. Each of the following keeps packages whole:

- an unusable file;
- an unlisted or unknown package;
- an invalid name or cost;
- a pattern over 64 KiB;
- invalid package costs.

None of them drops a test.

**Placement.** Units are placed longest first: whole packages at their AFP-V0-022 cost, and slices
at their recorded cost. Each unit goes to the least-loaded shard, and slices avoid shards that
already hold a sibling. Without a split, the plan equals `Partition`, and
`TestAFPV0041PlanEqualsPartitionWithoutSplits` checks this on 200 random universes.

**Identity.** The slice code, the allow-list and the slice file are copied into the protected helper
module and covered by its digest. The digest domain is now `corvint-ci-partition/2`. The digest is
part of the AFP-V0-014 frozen identity. The package-level PR driver cannot run slices, so it refuses
sharded execution while any package of the runtime universe is split. The workflow's driver path
does not fall back, so the refusal fails the shard closed. A pinned older driver fails on the
digest mismatch.

**Workflow.** The FULL path runs whole packages in one invocation as before. It then runs each
slice from `ci-shards --slices` as its own `go test`, with the same `-exec` confinement and
`-json -p 1 -count=1 -race -timeout 50m`. The shard fails if any invocation fails, before the
AFP-V0-024 tested-tree record. The CI-side change is small:

- the helper `cp` line gains the four slice files;
- the test step gains the slice loop.

A parallel lane wraps the test command in a `run_tests` function with `tee`. On merge, the slice
loop belongs inside that function.

## Generation

The generator is `tools/ci-test-slices generate`. It requires a clean checkout at the recorded
revision. It enumerates with `go test -race -count=1 -list .`, without workspace or `GOFLAGS`
redirection, and splits each allowed package that is slower than the target. The target defaults
to the ideal share. Named slices minimise the largest slice over sorted names, weighted by observed
top-level time; a name the run lacks takes the package median.

The committed file was generated at `1f68f622e8ee3af9cd39a57696881e2e9b18c7ac` (the implementation
commit), from run 38055182050, on darwin/arm64 with go1.27.1:

- `cmd/corvint` stays whole: 764.6 s is within the 1,280.8 s target.
- `internal/tasks/store` splits in two:
  - named slice: 143 names, 920.5 s planned;
  - catch-all: 951.4 s planned.

Enumeration listed 361 store tests and 7 benchmarks. The run had 359 top-level outcomes. The two
names the run lacks (`TestCALV0026_InPlaceIntentEditAfterCommitSweep`,
`TestCALV0026_LeaseCommitSweepsOutsideWriterLock`) were added after `e0ef9aa8` and took the median
weight.

## Replay of run 38055182050

`tools/ci-test-slices replay` places the run's 320 packages with and without the slice file. Whole
packages count at their observed time. A slice counts at its package's observed time, scaled by its
tests' share of the package's summed top-level test time. That scaling is an inference.
`internal/tasks/store` runs its tests serially (top-level sum about 1,870.7 s against 1,871.8 s
elapsed), so the share is close for it. `cmd/corvint` runs in parallel (2,485.8 s summed against
764.6 s elapsed), so a share of it is less reliable.

| Placement costs | Slices | Per-shard sums (s) | Max (s) |
|---|---|---|---|
| (observed hosted run) | none | 1113.2 1148.7 1145.4 2100.8 1312.8 863.9 | 2100.8 |
| committed `package-costs.json` | none | 2100.8 1113.2 1148.7 863.9 1145.4 1312.8 | 2100.8 |
| committed `package-costs.json` | committed (store x2) | 1266.5 1261.2 1291.7 1236.0 1162.5 1466.9 | 1466.9 |
| observed run as costs | none | 1871.8 1162.8 1162.8 1162.8 1161.8 1162.8 | 1871.8 |
| observed run as costs | committed (store x2) | 1282.3 1280.3 1281.3 1280.3 1280.3 1280.3 | 1282.3 |
| committed `package-costs.json` | `--target 400s` (store x5, cmd/corvint x2) | 1348.9 1169.7 1244.8 1362.8 1342.5 1216.0 | 1362.8 |
| observed run as costs | `--target 400s` | 1280.8 1281.2 1281.2 1281.2 1281.2 1279.3 | 1281.2 |

With the committed cost table and no slices, replay reproduces the observed per-shard sums exactly,
as a permutation. With the committed slices, the predicted maximum falls from 2,100.8 s to
1,466.9 s on the committed table, and to 1,282.3 s once the table matches the run. The residual
above the ideal share on the committed table comes from cost-table drift, not from a package floor.

`--target 400s` gains about 104 s on the committed table and nothing on a current one. It also
multiplies the per-slice overhead and loses `cmd/corvint`'s cross-test parallelism, so the
committed file keeps the default target.

## Local slice execution

The two store slices that the built helper prints for shards 0 and 1 ran through the workflow's
loop on darwin, without the Linux confinement wrapper, as `go test -json -p 1 -count=1 -race`:

| Shard | Slice | Tests | Hosted sum of the same tests (s) | Predicted (s) | Local wall (s) | rc |
|---|---|---|---|---|---|---|
| 0 | `-skip` catch-all | 218 | 951.8 | 952.4 | 1,885 | 1 |
| 1 | `-run` named | 143 | 918.9 (141 with hosted times) | 919.5 | 1,253 | 0 |

`go test -list` printed 361 tests and 7 benchmarks. The two outputs hold exactly one terminal
top-level outcome for each of the 361 tests (350 pass, 10 skip, 1 fail):

- none is missing, duplicated or extra;
- every named test ran in the named slice, and every other test ran in the catch-all;
- no benchmark ran.

The one failure was `TestCALV0197_RunRoleSelectsAnsweredWaitOfItsStage` ("turns 5, want 4"), in
the catch-all. It took 185.1 s there, against 42.9 s on the hosted run. Run alone with the same
flags, it passed in 49.2 s. It builds its own fixture, and its assertion counts supervisor turns
against a stage wall clock. The machine had 12 cores at a load average of 35 to 63, with other
lanes running this package's tests at the same time. This failure is therefore read as load-timing
sensitivity, not as a slicing defect; that reading is an inference. Local wall times are not hosted
evidence.

## Limits and integration

- **Lost interleavings.** Splitting loses cross-slice parallel interleavings, including race
  detection between tests in different slices.
- **Per-slice overhead.** Every slice repeats the package's build or link, process start and
  `TestMain`. This overhead is NOT_OBSERVED on hosted runners.
- **Cost refresh blocked.** While a package is split:
  - `tools/ci-shard-costs refresh` refuses the run, because the package has "two terminal outcomes";
  - the parallel lane's strict drift check abstains, because the package "starts twice".

  So the cost table cannot be refreshed from a sliced run until those tools sum slice outcomes per
  package. That follow-up is not part of this change.
- **Stale slice file.** A stale slice file can only move time between shards.
- **Selective PR pins.** The workflow's selective PR pins (`CORVINT_PR_TOOL_SOURCE` and the
  qualification pins) are empty, so every run takes the FULL path that runs slices. If those pins
  are set while a package is split, the driver's refusal fails every sharded PR run closed. Enabling
  them therefore first needs an empty slice file, or a driver that runs slices.

## Rollback

Choose either:

- set `packages` to `{}` in `.github/cishards/test-slices.json`, after which the plan equals
  AFP-V0-022 and the driver accepts sharding again;
- revert the workflow slice loop and the helper files.

## Not run

- A hosted sliced run.
- Paired hosted timings.
- Per-slice overhead.
- Linux enumeration (darwin was used).
- The parallel lanes' merged `ci.yml`.
- `make gate`.
- Owner acceptance of AFP-V0-041.
