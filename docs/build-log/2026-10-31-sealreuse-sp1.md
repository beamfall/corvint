# Seal reuse and timing lane: dogfood check phases, shared verifier cache, one closeout precheck — V1-3625, V1-3627, V1-3550

Lane sealreuse-sp1, base `8daec98ce4349c1583186692726d2c3a782ac647`. Requirements: `DCW-V0-942`,
`DCW-V0-943`, `DCW-V0-944` in `docs/specs/daily-change-evidence-workflow-v0.md`; `ASD-V0-943`,
`ASD-V0-944` in `docs/specs/corvint-self-development-v0.md`.

## V1-3625: dogfood check per-phase timings

Problem: a check-seal of 1296 s left a timings TSV accounting for 7 s. The finish-lane clock
(`DCW-V0-115`) records only commands its wrapped runners execute. An enrolled aggregate check runs
its verifiers through `runAggregateVerifier`, which execs the worker directly and bypasses
`runner.Run`. The in-process report admission, the aggregate local-outcome reacquisition
(`verifyAggregateOutcome`), the binding work and the operation-lock wait were never timed.
`script/dogfood-check.sh` printed nothing about its two verifier builds.

Decision: the check owns a contiguous phase clock (`internal/dogfoodflow/check_timing.go`). Every
instant belongs to exactly one running phase, so the rows sum to the wall time by construction, not
by sampling. Nested phases (`report` inside `precheck`, each verifier call inside `binding`) end the
outer segment and resume it afterwards. Verifier rows are taken in `check.verify`, which both the
runner and the aggregate worker paths pass through, so every gate replay gets a
`verifier-base|tree|override <command label>` row. The wrapper times `setup`, `build-tree` and
`build-base`, each with `reuse` or `build`, into `dogfood-check.build.tsv` bound to `$$`. `exec`
keeps the PID, so the Go check adopts the file only when it names `os.Getpid()`, and it starts its
own first phase where the wrapper's rows end, which counts the exec gap. Both files are excluded
from the `DCW-V0-347` inputs digest, so writing them never defeats seal reuse.

The first draft also printed a `dogfood-check: TIMING total` stderr line. The filtered
`cmd/corvint` dogfood tests compare check stderr exactly
(`TestDogfoodDailyPathRunsFromBinaryInForeignRepository`,
`TestDogfoodDailyPathCompletesWithDeclaredNoIntent`,
`TestDogfoodFinishLaneDCWV0060MatchesManualRecipe`), and the line is not needed for the AC. The
file is the only record; the spec commit was amended before any code commit to say so.

## V1-3550: dogfood-closeout runs batch-precheck twice

Measured on this worktree at BASE (an unchanged tree, before any edit): one `make batch-precheck`
took 501 s wall (742 s summed). `selection-baseline-check` took 362 s of it, the 8-package drift
union 49 s, the two `@own` companion builds 50 s and 88 s, and `capability-catalog-check` 59 s. The
closeout ran the whole set again on the sealed head. The tree there differs from the first run's
only by the bind and seal commits, yet every tree-keyed ledger step missed (`GL-V0-051` keys an
undeclared salted step on the whole tree).

Decision: keep both runs, and make the second one select. `BATCH_PRECHECK_SINCE_RECEIPT=1` reads
the receipt before the run removes it. A passing receipt for the clean HEAD ends the run. One for a
strict ancestor R replaces the base, for selection and for the base-present rerun alike, so only
the steps R..HEAD can reach run under the existing `ASD-V0-050`/`ASD-V0-051` rules. R passed every
step, so the narrowing has the same soundness as a selection against a passing base. Carrying the
receipt forward without running anything was rejected: `test-read-scopes-check` reads
`.corvint/change.cem.json`, and `TestCurrentTasksSourceArchiveBuildsOffline` archives the whole
tree, so "the seal touches only evidence" is not a safe premise. The affected plan already knows
which units read `.corvint/` paths.

`selection-baseline-check`, the dominant step, also gets declared inputs (`ASD-V0-944`): the Go
source of the packages its recipe builds (`@go`), top-level files (`@root`, which includes the
Makefile), `.github/cishards/` and `benchmarks/selection-baseline/`. It replays fixed historical
commits with binaries built from those inputs, so a doc-only or script-only diff skips it under any
base, which is what brings a script/doc-only closeout well under 600 s.

## V1-3627: one base binary per base per host, shared gate results

ALREADY-DELIVERED (a): the verifier build cache (`DCW-V0-074`,
`script/dogfood-build-corvint.sh`) was already per user and host and outside the repository
(`${XDG_CACHE_HOME:-$HOME/.cache}/corvint-dogfood-bin`), keyed by BASE's root tree plus the
`go env` toolchain and target. A second worktree on the same BASE already reused the base verifier.
What was missing: any record showing it, and any bound. The cache on this host held 107 entries,
5.4 GB. The `build-base reuse|build` row now shows it (the second-worktree case in
`script/dogfood-bin-reuse_test.sh`), and `dogfood_corvint_prune` keeps the
`CORVINT_DOGFOOD_BIN_CACHE_MAX` (default 16) most recently stored or reused entries. A reuse
touches its entry to refresh recency.

ALREADY-DELIVERED (b): gate results with an identical input digest are shared across a user's
worktrees on one host by `tools/gate-ledger` (`docs/specs/gate-ledger-v0.md`, `ASD-V0-042`):
`~/.cache/corvint/gate-ledger`, a content key that never names the worktree or commit, 4096
records, and a per-key lock that rechecks for a pass recorded while it waited. A digest change
misses and reruns. No second ledger was added. The check's own `DCW-V0-347` pass receipt binds HEAD,
so sharing it across worktrees would never hit.

(c), concurrency without a lock: every store renames private temporary files into place, and every
reuse verifies the copied bytes against the recorded object ID, so a half-written or half-pruned
entry costs a build, never a wrong binary. The test runs two concurrent writers over eight keys with
a bound of two. Every surviving entry verifies; the count stays within the bound plus one per
concurrent writer, and a later serial store restores the bound. A reuse racing a prune printed a
`cat` error inside the writer; that read is now quiet.

## Evidence

- `TestDCWV0942CheckWritesPhaseTimings` (`internal/dogfoodflow`): a foreign-PID build file is
  ignored and kept; the matching one is prepended and consumed; the rows come out in order, one per
  verifier command label, and sum to the total from the wrapper's start within 5%; the timing files
  are not reuse inputs. `internal/dogfoodflow` whole package PASS.
- `cmd/corvint`, filtered to every test in `dogfood*_test.go` and `local_completion_test.go`: PASS.
- `script/dogfood-bin-reuse_test.sh` (DCW-V0-942/943 cases; fails on BASE with no build timings),
  `script/batch-precheck_test.sh` group 15 (fails on BASE at the closeout recipe),
  `script/dogfood-cache_test.sh`, `script/dogfood-change_test.sh`,
  `script/dogfood-bind-range_test.sh`: PASS.

## Friction

- Host /tmp (a 16 GB tmpfs on this WSL host) filled to 100% during the lane. It holds an 11 GB
  `/tmp/corvint-go-build-cache` from another caller, which put ENOSPC in two batch-drift steps
  (FLAKY), `batch-precheck_test.sh` group 9, 19 `cmd/corvint` tests, `make spec-requirements`
  (exit 28) and the harness's own shell. Repro: `df -h /tmp; du -sh /tmp/corvint-go-build-cache`,
  then `make batch-precheck` without `TMPDIR`. Workaround: `TMPDIR=$HOME/.cache/corvint-tmp`.
- `script/batch-precheck_test.sh` group 9 asserts that no low-tmp-space warning appears above the
  floor, so it fails on any host whose /tmp is nearly full, whatever the change. Repro: run it with
  /tmp under about 2 GB free.
- The verifier cache had grown to 5.4 GB (107 entries of about 51 MB) with no bound (fixed here).
  Repro on main: `du -sh ~/.cache/corvint-dogfood-bin`.
- `heavy.sh` refuses a whole-package run of `internal/dogfoodflow` until a `HEAVY_WHOLE_REASON` is
  copied from `corvint test --dry-run`, and that dry run takes about a minute for a reason string
  the planner already knows. Repro: `~/lanes/heavy.sh env go test ./internal/dogfoodflow`.
- The frozen Intent-status header of `corvint-self-development-v0.md` refuses any new header text,
  while its digest still lists earlier amendments, so the first `make spec-requirements` failed on
  a status line written in the style of its neighbours. Repro: append an `ASD-V0-9xx (...)` clause
  to line 5 and run `make spec-requirements`.
