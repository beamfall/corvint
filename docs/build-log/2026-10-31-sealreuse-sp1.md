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
`build-base`, each with `reuse` or `build`, into `dogfood-check.build.<pid>.tsv`, named and headed
with `$$`. `exec` keeps the PID, so the Go check adopts only its own file, and it starts its own
first phase where the wrapper's rows end, which counts the exec gap. The timings file is written
only while the run holds the local-completion operation lock. Every timing file is excluded from
the `DCW-V0-347` inputs digest, so writing them never defeats seal reuse.

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
commits with binaries built from those inputs.

The first measurement showed that the plan-based skip alone never fires on this repository's real
diffs. `corvint affected` against origin/main carries `LANGUAGE_FRONTIER` unknowns that
`FRONTIER_OK` does not list (TypeScript `dynamic-import`, C-family `link-graph-unmodelled` and
`unresolved-include`, .NET `dynamic-test-discovery`): 86 of them for this lane's diff, and some
even for a one-line doc edit. So `ASD-V0-050` selection is off, and every step runs. The spec was
amended, in its own commit, so that once the diff is read but the plan cannot narrow,
`selection-baseline-check` is decided on the diff alone: it is skipped only when no Go input changed
anywhere and no declared input changed. That needs no plan, since its inputs are fully declared.
Widening `FRONTIER_OK` is a separate decision, left out here (see Friction).

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

## Measurements

All closeout measurements ran on a throwaway local clone of this branch
(`~/.cache/sealreuse-sp1/measure`, never pushed) on the WSL host, with
`TMPDIR=$HOME/.cache/corvint-tmp`.

- Real check and seal (V1-3625 AC b): `make dogfood-finish` with this lane's inputs, 43.9 s wall,
  bound, checked and sealed. Its finish-lane `TIMING check-seal seconds=7.73` step against the
  check's `dogfood-check.timings.tsv` rows, which sum to 7.734 s (0.1%): `binding` 5.469,
  `verifier-tree dogfood-ocm status` 0.735, `verifier-tree cem status` 0.517,
  `verifier-base cem status` 0.504, `precheck` 0.473, `seal` 0.030, the rest under 0.01. Before
  this change, the finish-lane rows named only the three verifier commands (about 1.8 s); the
  5.5 s `binding` phase, in-process OCM, coverage and record work, was the unattributed share.
- Wrapper-level check of the same bind commit: `make dogfood-check`, 8.629 s by `date` around the
  make call, against rows summing to 8.517 s (98.7%; make start and process exit are outside the
  rows). The rows included `setup` 0.023, `build-tree reuse` 0.538 and `build-base reuse` 0.187:
  the clone, a different repository from the worktree that first built this BASE, reused the
  host-shared base verifier (V1-3627 AC a).
- Closeout prechecks for this Go change (V1-3550). The first run (`make batch-precheck
  BASE=<base>` at the work head) took 464.7 s wall (790 s summed; `selection-baseline-check`
  393 s); the plan was not usable, so everything ran. The sealed-head run with
  `BATCH_PRECHECK_SINCE_RECEIPT=1` took 38.4 s: since-receipt base = the work head; 1 of 15 drift
  tests; catalog and selection-baseline checks skipped; 35 passed; receipt written for the sealed
  head. Before, the second run repeated the full set (about 465 s). Closeout for this change by
  parts: 4.5 s inputs-check + 464.7 s + 43.9 s finish + 38.4 s, about 552 s, against about 980 s
  before.
- Doc-only first precheck (a one-line `docs/build-log/README.md` commit on BASE, cold, with
  `CORVINT_GATE_LEDGER=off`): 487.1 s before the diff-only rule (`selection-baseline-check` ran
  because the plan was unusable), 111.3 s after (297 s summed; the longest step was
  `TestCorvintBundledBinariesBuildTwiceIdentically` at 96 s). A warm rerun on the same tree
  reused every ledger step and took 24.9 s. Doc-only closeout by parts: about 5 + 111 + 44 + 38,
  about 200 s, well under 600 s. That sum is assembled from separately measured steps; no single
  end-to-end doc-only `make dogfood-closeout` was timed, because a closeout seals and this lane
  does not seal.

## Independent review

One read-only review subagent found no high- or medium-severity defects. Its four low-severity
findings:

1. The since-receipt base also replaced the base for the base-present rerun, so a failure R would
   now also show would downgrade to a WARN. Fixed: R is only the selection base (`since_base`);
   the base-present rerun and trailer ranges keep the given base.
2. The receipt does not record the base its run selected against. Not fixed: recording it changes
   the five-line `ASD-V0-035` receipt. Recorded as an `ASD-V0-943` failure mode; inside the
   closeout, the first run always rewrites the receipt against the same base.
3. The timings could be written outside the operation lock, and two wrappers in one git directory
   shared one build file. Fixed: the timings file is written only while the lock is held, and the
   build file is per PID.
4. `CORVINT_DOGFOOD_BIN_CACHE_MAX=08` printed a bash base error. Fixed: the bound must match
   `^[1-9][0-9]*$`.

The first measurement precheck also caught five `docs/AUTOMATION.md` line citations whose cited
lines in `script/dogfood-check.sh` and `internal/dogfoodflow/check.go` changed. They were reread
and repinned to the same blocks (the build steps; the report-missing path), whose claims still
hold.

## Evidence

- `TestDCWV0942CheckWritesPhaseTimings` (`internal/dogfoodflow`): a foreign-PID build file is
  ignored and kept; the matching one is prepended and consumed; the rows come out in order, one per
  verifier command label, and sum to the total from the wrapper's start within 5%; the timing files
  are not reuse inputs. `internal/dogfoodflow` whole package PASS.
- `cmd/corvint`, filtered to every test in `dogfood*_test.go` and `local_completion_test.go`: PASS.
- `script/dogfood-bin-reuse_test.sh` (DCW-V0-942/943 cases; fails on BASE with no build timings),
  `script/batch-precheck_test.sh` group 15 (fails on BASE at the closeout recipe; covers the
  since-receipt base, the receipt-at-HEAD skip, a dirty checkout, no receipt, and the
  selection-baseline skip with and without a usable plan),
  `script/dogfood-cache_test.sh`, `script/dogfood-change_test.sh`,
  `script/dogfood-bind-range_test.sh`: PASS.

## Friction

- `FRONTIER_OK` in `script/batch-precheck.sh` lists three `LANGUAGE_FRONTIER` details, but
  `corvint affected` against origin/main now reports TypeScript, C-family and .NET frontier details
  on ordinary diffs, a one-line doc commit included. `ASD-V0-050`/`ASD-V0-051` selection therefore
  never narrows a first precheck on this repository. Repro: on a doc-only commit over origin/main,
  `make batch-precheck BASE=<origin/main sha>` prints `batch-drift runs every drift test ... (the
  plan has unknowns: LANGUAGE_FRONTIER typescript:dynamic-import...`.

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
