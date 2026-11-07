<!-- SPDX-License-Identifier: Apache-2.0 -->
# Shard parallelism replay (V1-4355)

Base: `28a3ff86a2c3d17662765da12cc9251c06466f5f`.
Requirement: `CIN-V0-7620`, under standing approval, not owner read-back.

## Finding and bounded decision

The base already implements `max(longest, sum/p)` placement in
`.github/cishards/slices.go`: `PlanFromParallel`, `SelectFromParallel` and `Wall`
(`AFP-V0-2181`). `ci-shards --parallel` already defaults to `SHARD_TEST_P`.
The missing replay support was in `tools/ci-test-slices`: it always used
`PlanFrom` and reported serial sums. Reuse the existing planner, add replay-only
`--parallel` and `--compile-link`, and preserve default output exactly. A common
compile/link allowance shifts every shard equally, so it cannot improve placement
by itself. It is an explicit estimate, never inferred from package elapsed times.

Production integration is PARTIAL: the charter forbids every `.github/**` edit.
Both `SHARD_TEST_P` settings in `ci.yml` still restrict `4` to self-hosted PRs
(`AFP-V0-540`); PR #100 must set both the narrowing and execution steps to `4`
for hosted PRs while leaving `merge_group` at `1`. The existing Plan/Select
parallel entry points need no new algorithm or wire change. No allowed-path
change can activate a different expression in that workflow. No exception was
assumed, and no workflow, cost table, split file, golden already present at BASE,
or selection authority was changed. Independent review belongs to the orchestrator.

## Inputs and predictions

Source: [PR #99 run 42106672783](https://github.com/beamfall/corvint/actions/runs/42106672783),
head `e468668f2d2ea198271ce26717f8f8dd6e998203`, merged as the base above.
The downloaded archive contains 290 distinct packages and 300 terminal package
outcomes across the sixteen shard logs. The latest jobs response reports Go-test
steps of 510..1059 s, mean 726.625 s; summed package elapsed values are
413.712..792.183 s. The archive includes a failed `internal/tasks/store` slice
(shard 6, 282.877 s). Event elapsed times do not isolate serial compile/link work,
and this failed run cannot refresh costs under `AFP-V0-040`.

Raw evidence is outside Git at `~/lanes/placement-evidence/`. SHA-256 digests:

- `run-42106672783.zip`: `62a5dd13fd868750203ef09c0783775fb7553bd6fb85a7618719fccc4f122742`.
- Base `package-costs.json`: `a9fe8790e22e76d2ba76a286ab350f7cc31e2fd585698b1fc3cfa1d7abc65b5c`.
- Base `test-slices.json`: `e317df7afd878a62e25d570e5974b73cb332e1356f75cbde9f81de406c9ad78d`;
  identical to the source head's file fetched from GitHub.
- Committed 17-package measured fixture: `541705a21fc16dff5f4534ac2599dc60cd50cf4efbb5da5795466daf0e6bd011`.
- New serial placement golden: `bbbfdaef340403b25a6d4857b4eadcb935ef9be8c6002d605314cc9219cda7fe`.

For the PR #99 observed package set, the **unchanged committed** cost/slice
inputs yield 300 units on sixteen shards. The isolated scratch Go replay calls
the actual `PlanFromParallel` implementation; `committed-cost-replay.log` retains
its result. These are model predictions, not new CI measurements:

| Event model | Predicted max, no allowance | With illustrative common 360 s compile/link allowance |
|---|---:|---:|
| Before: `p=1` | 569.504 s | 929.504 s |
| PR after wiring: `p=4` | 431.198 s | 791.198 s |

The predicted max falls 138.306 s (14.9% including the illustrative allowance).
Evaluating the old serial placement at `p=4` also gives a max of 431.198 s: that
unit is the lower bound, so no placement can strictly reduce this set's max
without changing its costs or splitting it. Those changes are outside this lane.
The failed slice's elapsed time was not admitted as a replacement cost.

For a small regression sample, the 17 longest passing, unsplit package outcomes
from that run are retained in testdata. With two shards and both placements
evaluated at `p=4`, serial LPT predicts max 369.270 s, parallel placement
363.828 s, a 5.442 s improvement. Adding the same 360 s gives 729.270 s and
723.828 s. The fixture is explicitly a sample, not a complete passing run.

## Verification and retained limits

- Whole existing package: `TMPDIR=~/lanes/placement.tmp script/heavy.sh go test -count=1 -timeout 30m ./tools/ci-test-slices`
  passed (1.485 s package time); `test-ci-test-slices-terminal.log` retains the
  semaphore result and full command. Tests cover the unchanged serial golden
  and stdout, Plan/Select determinism, improved parallel sample placement, sliced
  replay, upward millisecond rounding and invalid/replay-only options.
- Negative control: the BASE source plus its process helpers, in an isolated
  scratch module, passes the identical serial stdout golden and fails the
  parallel replay expectation with `flag provided but not defined: -parallel`.
  `test-baseline-control-accepted.log` retains this expected failure. Earlier
  scratch harness attempts failed on missing helper files, internal-import
  scope and missing fixture parent directories; none is regression evidence.
- Initial new tests exposed missing fixture source metadata and an incorrect
  expected shard order; the new fixtures were corrected. Existing assertions
  were not weakened. Earlier failures remain in the external test logs.
- `make spec-requirements` regenerated and staged the requirement index and
  README. `script/check-line-citations.sh --repin` found no movable citations;
  `make -s spec-requirements-check line-citations-check` passed.
- Used self-development routes: pre-change context/query, a supplemental `.go`
  impact read, affected advice, and the final CEM workflow. The bound directory
  impact remains `NOT_PRODUCED unsupported-impact-path-suffix`; the later `.go`
  receipt is supplemental, not a repair to that binding. The installed affected
  binary reports `self-binary-stale`; advice remains UNKNOWN. Context omitted
  sources at the result limit; original planner/spec files were read directly.
- Native ticket read needed the provided read-only snapshot; current journal
  state remains NOT_OBSERVED. No Tasks write, push, PR, seal, model-backed review,
  whole-repository test selection or doc-gates was run. The shipped command-tree
  generator does not inventory this source-only tool; its help and developer
  catalog row document the options without changing shipped snapshots.
- Hosted `p=4` qualification, exact compile/link cost and a complete passing
  source refresh are NOT_PRODUCED. Retrieval/learning frozen evaluations and
  optional analyzer/provider/service routes are not applicable to this replay
  change. The orchestrator owns independent review and host-wide verification.

Rollback removes the replay options and their requirement/testdata; production
placement and the merge-queue command are unaffected.
