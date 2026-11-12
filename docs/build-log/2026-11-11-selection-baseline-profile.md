# Selection-baseline profile, source-slicing plan bound, archive-gate limits — 2026-11-11

Lane gatespeed; base `b55635262788687c3eea9a08a94378e4ab8856a4`. Tickets V1-5162, V1-4733,
V1-4365 and V1-5363. Requirements: `ASC-V0-782` (new), the 2026-11-11 amendment of `TSP-V0-006`,
`GLS-V0-035` (new). This entry records the measurements and design decisions; raw logs stayed
outside the repository under `~/.cache/corvint-tmp/sb*` on the WSL lane host.

## V1-5162: where the selection-baseline replay spends its time

Host: WSL2, 26 CPUs, 31 GiB, Go 1.27.3, under the lane environment `GOMAXPROCS=4` and
`SELECTION_BASELINE_JOBS=2` (two replay workers, each subprocess capped at four threads, close to
a 4-vCPU runner's budget). Every run used `SELECTION_BASELINE_CACHE=off`, a fresh
`XDG_CACHE_HOME` (cold `corvint/affected` graph cache, as in CI), `SELECTION_CONTRACT_CHECK=1`
(the doc-gates mode, `AFP-V0-9498`) and `-trimpath -buildvcs=false` binaries.

**One-shot baseline** (origin/main binaries, the 58-change set, `--check`): 1119.1 s wall,
3805.3 s user + 285.9 s system = 4091 CPU-s; `narrow=36/43 merged ... misses=0`, `OK floor
narrow>=33 misses<=0`. The 2026-11-05 entry (`2026-11-05-selection-baseline-gating.md`) measured
the same replay at 2147 CPU-s: the per-change cost nearly doubled in six days.

**Cause, with evidence.** Not a cache miss and not the worker count:

- CI persists no cache for doc-gates (`actions/setup-go` with `cache: false`, no cache step), so
  every CI replay is cold: the `ASC-V0-778` result cache and the affected graph cache start empty.
  The result cache key holds the planner's bytes, so a pull request that changes any Go input of
  `cmd/corvint` would miss even with a persisted cache. GitHub's cache scoping also keeps a pull
  request's entries from a merge-queue run.
- The 4-vCPU runner already runs four workers (`HEAVY_P`=4 through `heavy.sh --admit-only`); the
  replay is CPU-bound, so more workers cannot help: 4091 CPU-s / 4 is about 1020 s at this host's
  per-core speed, and 1331–1684 s on hosted runners matches slower cores plus the concurrent doc
  gates.
- The growth is per-change planner work. `perf` on a typical change (`pr-47`) attributes 55% of
  the selector's and of `corvint test --dry-run`'s CPU to
  `internal/liveverify/affected/golang.Language.observeDirectories` (`pathTokens`,
  `testMetadataLiterals` parsing every Go file), which the selector's qualifying proof
  (`affected.BuildPartial`) and the test command each recompute uncached on the head tree. On the
  large changes (`pr-41`, 567 dirty paths) 68% of `corvint test --dry-run` was `affected.Select`
  called from `cmd/corvint`'s `testSourceEligible`: one full plan per `DIRECT_SOURCE_CHANGE`
  candidate, 102 of them rejected as also selected.

**Per-step attribution** (`ASC-V0-782` TIMING lines; the set run in three contiguous chunks of
20, 20 and 18 changes so each command stayed under ten minutes, one shared cold graph cache):

| Step (worker wall s, summed) | Before | After |
|---|---:|---:|
| checkout | 18.7 | 19.1 |
| affected | 382.1 | 363.3 |
| selector | 434.7 | 403.7 |
| test (`--dry-run --profile ci`) | 1486.2 | 993.3 |
| guard + universe | 3.9 | 3.8 |
| Chunk wall (sum of three) | 1236.5 s | 952.1 s |
| CPU (user + system, sum) | 4299.8 s | 3650.0 s |

Slowest changes before: `pr-41` 135.8 s, `ci-cbc790640` 134.5, `ci-0df97c8c7` 133.0, `pr-39`
99.0, `ci-af9d23f80` 94.3 (the same five the 2026-11-05 entry named). After: `pr-41` 60.2,
`ci-0df97c8c7` 59.6, `pr-43` 58.4, `ci-cbc790640` 55.5, `ci-8db625151` 55.5. On `pr-41` alone
`corvint test --dry-run --profile ci` went from 83.8 s / 92.0 s to 33.0 s / 33.4 s wall (CPU
98–114 s to 52–54 s) with the identical unit set (309 units, every `(adapter, unit, slice)` equal).

**Fix (`TSP-V0-006` amendment).** The spec already bounded source slicing to 16 plans of the other
dirty paths; `testSourceEligibles` counted only kept candidates. It now counts every plan.
Verdicts, narrow counts, misses and test agreement are unchanged in all three chunks (15/19, 14/14
and 7/10 narrowed, 0 misses, 0 test-extra, -omitted and -misses). Recorded precision cost: on the
three FULL-CI changes of base `9bb16e1ff` (`pr-12`, `ci-e09dae910`, `ci-d2115e21f`) one package
that was sliced is now whole past the bound (corpus `sliced` 2 to 1), and units that now hit the
bound carry `direct-source-slice-bound` instead of `whole-also-selected`
(`whole-source-and-tests` 27 to 147). Both outcomes run the package whole.

**Not reached.** The charter's target of five minutes on this host is not met: after the fix the
replay still needs about 3650 CPU-s, and its remaining cost is the uncached whole-tree Go
observation in `internal/liveverify/affected/**`, outside this lane's paths, plus the selector's
authority proof, which by invariant 4 may not read the user graph cache. Proposals for the owner
of that package: memoize `pathTokens` and `testMetadataLiterals` per file blob within a process,
and share one head-tree observation between the plan graph and `BuildPartial` inside
`corvint test`. A persisted cross-run cache would need a `.github/**` cache step and would still
miss on every planner change. The CI half (a doc-gates run citing its `doc-gate-timing` line inside
the job limit) is NOT_RUN here; the job limit is already 45 minutes (`7a1022340`).

## V1-4733: archive gate

Target-level concurrency already landed in `c2e9330b8` (`GAG-V0-9960..9963`, 525.6 s to 191.5 s
on 32 CPUs). The gate stays CPU-bound: 20 cold builds and the verifier cost 3400–3800 CPU-s
there, at least about 850–950 s on four vCPUs, against an 8-minute target. Measured here: a cold
`go build -trimpath ./cmd/corvint` for `linux/amd64` with `GOMAXPROCS=4` takes 16.3 s wall,
91 CPU-s; building a second export of the same commit with that cache takes 0.76 s and gives
identical bytes. Sharing cache lineages (loose A with archive A, loose B with archive B) would
keep two independent lineages compared byte-for-byte and remove about half the build CPU, but
the archive pair would then prove only that the cache was reused. `ARTIFACT-GO-V0-005`,
`GAG-V0-9963` and the conformance README require separate cold caches for both pairs. Decision:
no change; changing those requirements is the owner's call (handback NOT_DONE).

## V1-4365: cross-package fault-injection fixture

Already delivered on origin/main: the `GLS-V0-030..034` harness, the single-package fixture with
every site kind and a seeded miss, and the qualification record refreshed in `3df938810` (seed
20261109, 20 mutants, 13 killed, 0 misses, 2 controls passed, 29 source blobs equal at
origin/main). Added under `GLS-V0-035`: a lib/app fixture where faults in `lib.Grade` and
`lib.Map` fail only tests of the importing package. Every site of the fixture's order (the test requires at least 10 valid mutants)
(case, else-if, loop, closure and statement sites; delete, negate and constant operators) report
zero misses and the control passes. A seeded index that moves Grade's blocks from
`app.TestReport` to `lib.TestTotal` fails qualification naming `app.TestReport`. A first attempt
that only zeroed those blocks passed, because `GLS-V0-030` samples only executed functions; the
fixture follows the single-package test in moving the blocks instead. The per-language corpora of
the acceptance (Beamfall repositories per family, a public Ruby project) are not attempted.

## V1-5363

`testPlanGit` now passes `-c maintenance.auto=false -c gc.auto=0`, as the package's other fixture
helpers do; `TestTestPlanGitDisablesAutoMaintenance` failed without it (`git config --get
maintenance.auto` exit 1). The read-only snapshot excludes nothing new.
