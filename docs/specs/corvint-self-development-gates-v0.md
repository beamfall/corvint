# Corvint Self-Development Gates V0

Owner: Russell Lewis
Date: 2026-11-04
Requirement prefix: `ASC-V0`
Intent status: proposed
Delivery status: experimental
Authoritative inputs: repository-owner requests V1-3615, V1-4166, V1-4239, V1-4242 and V1-3580; `AGENTS.md` invariants 4 and 8; `docs/specs/corvint-self-development-v0.md` `ASD-V0-035`, `ASD-V0-042`, `ASD-V0-925` and `ASD-V0-943`.

## Agent digest
- Claim: Self-development gate requirements beyond the 256-obligation limit of `corvint-self-development-v0.md`, under their own prefix.
- Status: proposed/experimental
- Exists: the receipt keep and step labels in `script/batch-precheck.sh`, and the replay workers and result cache of `tools/selection-baseline`, with their focused tests.
- Blocked on: Nothing; companion of the self-development spec, whose Requirements section is at the 256-obligation limit.
- Read next: Requirements; Acceptance evidence; Rollback.

## User and measurable job

This spec extends `docs/specs/corvint-self-development-v0.md`, whose Requirements section is at the `ocm prepare` limit; new lane-gate requirements go here. The job is a shorter lane closeout: a reseal selects against the last sealed head's receipt, ledger records name their step, and `selection-baseline-check` scales to the host and reuses identical replays. Each requirement keeps the number it was written under in the lane that built it (`ASD-V0-775` to `ASD-V0-779` became `ASC-V0-775` to `ASC-V0-779` when the work was merged, because the parent spec was full).

## Requirements

- `ASC-V0-775`: (experimental under the standing lane auto-accept, 2026-11-01; V1-4239, V1-3475; refines `ASD-V0-035` and `ASD-V0-943`)
  - Before its steps, `script/batch-precheck.sh` MUST keep a well-formed passing receipt that names a strict ancestor of HEAD, and print `batch-precheck: kept the passing receipt for ancestor SHA ...`.
  - It MUST still remove any other receipt, one naming HEAD included, and a passing full run MUST still replace it with a receipt for HEAD.
  - Such a receipt states only that the named commit passed every step, which no run at a later commit changes. `--verify-receipt REV` still refuses it for any REV but its own.
  - Design: on a reseal, the pre-bind `BATCH_PRECHECK_SKIP_DRIFT=1` run removed the old sealed head's receipt and wrote none. The sealed-head `BATCH_PRECHECK_SINCE_RECEIPT=1` run then printed `since-receipt not used: no well-formed passing receipt` and selected against BASE (dsl-w32: a 4-hunk reseal 289 s against 298 s cold). It now selects against the old sealed head, so only the steps the reseal's own commits can reach run.
  - Measured in `docs/build-log/2026-11-01-csp-w33.md`.
  - Non-goals: carrying a receipt across a rewritten history (a non-ancestor receipt is still removed); using the kept receipt in the pre-bind run.
  - Failure mode: none new, since a kept receipt only narrows a later `BATCH_PRECHECK_SINCE_RECEIPT=1` run, exactly as a receipt for that commit always did.
  - Rollback: restore the unconditional removal.
- `ASC-V0-776`: (experimental under the standing lane auto-accept, 2026-11-01; V1-4242; refines `ASD-V0-042`)
  - `script/batch-precheck.sh` MUST pass each ledger step's display name (the step table's name, such as `sync-check` or `go run ./conformance/use-cases-v0`) to `tools/gate-ledger run` as `GATE_LEDGER_LABEL`.
  - The ledger MUST record it, as one line of at most 200 bytes, in the pass record's `label` field.
  - The label MUST NOT enter the key: a pass recorded without a label still reuses, and two labels of one step share one key. The variable is outside the salted GO*, CGO_* and CORVINT_* prefixes, and an older ledger binary ignores it.
  - Design: every non-make step keys as `batch-precheck+SALT`, so the ledger survey could not tell 615 such records (13,002 s) apart.
  - Non-goals: renaming the keyed step; relabelling existing records.
  - Rollback: drop the variable and the field.
- `ASC-V0-777`: (experimental under the standing lane auto-accept, 2026-11-01; V1-4166; amends `ASD-V0-925`) `tools/selection-baseline`'s default worker count MUST be the larger of the CPU count at most 4 (`ASD-V0-925`) and CPUs/3 at most 8, at least 1 and still at most the number of changes to replay, with a positive `SELECTION_BASELINE_JOBS` winning; results and lines keep the change-set order, so the verdict and `--out` are the same for any worker count. A 12-CPU host keeps 4 workers; the 32-CPU WSL lane host gets 8. Measured there (2026-11-01, load 1.6-7.6): 89.2 s with 4 workers, 64.8 s with 8, 60.1 s with 12, identical results. Non-goals: load-adaptive scheduling. Failure mode: eight concurrent planners load the host for those seconds. Rollback: restore the cap of 4, or set `SELECTION_BASELINE_JOBS=4`.
- `ASC-V0-778`: (experimental under the standing lane auto-accept, 2026-11-01; V1-4166, V1-3580) `tools/selection-baseline` MUST read a replayed change's result from its per-user result cache (`SELECTION_BASELINE_CACHE_DIR`, else the user cache directory's `corvint/selection-baseline`, a private directory) only when the record's key matches the digest of the planner, selector and replay-tool binaries' bytes, `.github/cishards/package-costs.json`, the module path, `go version`, the `go env` value of each build and module setting (which reads the `go env -w` file), `git --version`, `git config --list --includes`, every `GO*`, `CGO_*`, `CORVINT_*`, `GIT_*` (except `GIT_TRACE*`) and `SELECTION_BASELINE_*` variable (except the cache switches and `SELECTION_BASELINE_JOBS`) and the whole change entry; each change's base and head are immutable commits, so that key fixes the result. A run that reuses any record MUST print `selection-baseline: REUSE N of M change results ...`, naming the run key (the hex digest of every input above except the change entries). The key MUST hash the exact cost-table bytes the replay parsed, never a second read of the file; any miss, unreadable or mismatched record replays the change and records it; `SELECTION_BASELINE_CACHE=off` or `CORVINT_GATE_LEDGER=off` replays every change; the cache keeps the newest 512 records. `make selection-baseline-check` MUST build its binaries with `-trimpath -buildvcs=false` unless `SELECTION_BASELINE_OUT` is set, so identical Go inputs give identical bytes in any worktree (results then equal the stamped build's, except that the replayed plans lose the replay-only `self-binary-stale` degradation, which no result field reads, and `--out`'s planner names no commit, which is why `--out` keeps the stamp). Measured on the WSL lane host (2026-11-01): a cold `make selection-baseline-check` 75.8 s, the identical rerun 1.65 s; stamped and unstamped `--out` results equal except the planner field. Non-goals: sharing records between users or hosts; a narrower replay. Failure mode: a planner whose output depends on an input outside the key (none known: the replayed commits are immutable and the graph cache is derived from them) would reuse a stale result until the binary changes. Rollback: delete `tools/selection-baseline/cache.go` and its call in `replay`, and the build flags in `Makefile`.
- `ASC-V0-779`: (superseded 2026-11-06 at the mbrescue merge; was experimental under the standing lane auto-accept, 2026-11-01; V1-3615) The obligation is carried by `DTL-V0-006` (bounded-parallel `doc-gate/%` targets under `DOC_GATE_JOBS`, each timed) and `ASD-V0-782` (a failing run ends naming the failed gates), which origin/main delivered first. The lane runner `script/doc-gates.sh`, its test and `doc-gates-test` are not merged. Gap kept open under V1-3615: main's second recipe line, the `conformance/use-cases-v0` run, still does not run when a gate fails, and no rerun-only-the-failed command is printed. Record: `docs/build-log/2026-11-06-mbrescue.md`.
- `ASD-V0-3720`: (moved here from `corvint-self-development-v0.md` at the mbrescue merge, 2026-11-06, because that spec holds 256 obligations; experimental under the standing lane auto-accept, 2026-11-01; V1-3257; amends `ASD-V0-087`) The dogfood tooling set of the `corvint-bin-predates-tree-tooling` check MUST also hold every in-module package of the Go import closure of `internal/dogfoodflow` (`go list -deps`), each as its own directory's non-test files, so a dogfood behaviour change made only in a dependency such as `internal/trace` refuses the override; a package outside that closure keeps it. The closure is computed only when `CORVINT_BIN` is set. Non-goals: the closure of `cmd/corvint`, which is nearly the whole repository. Failure mode: where `go list` fails, the fixed paths of `ASD-V0-087` alone apply, as before. Evidence: `script/dogfood-cache_test.sh` (ASD-V0-3720 case).

## Failure modes

| Condition | Required observation |
|---|---|
| A receipt names a non-ancestor of HEAD | It is removed, as before `ASC-V0-775` |
| A cached selection-baseline record does not match its key | It is ignored and the change is replayed |

## Acceptance evidence

| Requirement | Implementation and actual evidence |
|---|---|
| ASC-V0-775 | the receipt keep before the steps in `script/batch-precheck.sh`; `make batch-precheck-test`: a dirty run after HEAD moved keeps the ancestor receipt and writes none for HEAD; a reseal's pre-bind run keeps the sealed head's receipt and the resealed head selects since it (`since-receipt base`, the planner asked for that base) and writes its own; a receipt naming no ancestor is removed; the reseal timings in `docs/build-log/2026-11-01-csp-w33.md` | implemented; measured |
| ASC-V0-776 | `GATE_LEDGER_LABEL` in `run_one`, `script/batch-precheck.sh`; `record.Label` and `stepLabel` in `tools/gate-ledger`; `TestStepLabelIsRecordedOutsideTheKey_ASCV0776` (an unlabelled pass hits once a label is given, a new record carries the label, two labels share one key); `make batch-precheck-test` 4b (every `batch-precheck+` record carries a label, `sync-check` among them, and with every label stripped each step still reuses) | implemented; shell test PASS |
| ASC-V0-777 | `replayJobs` and `defaultReplayJobs` in `tools/selection-baseline/main.go`; `TestReplayJobsScaleToTheHost_ASCV0777`; timings in `docs/build-log/2026-11-01-csp-w33.md` | implemented; measured |
| ASC-V0-778 | `tools/selection-baseline/cache.go` and its use in `replay`; the build flags of `selection-baseline-check` in `Makefile`; `TestResultCacheEqualsReplay_ASCV0778` (a second run replays nothing and writes byte-identical results; cache off, ledger off, changed planner or selector bytes, environment, a `go env -w` setting, a Git config setting, cost table and change entry each replay), `TestResultCacheRefusesForeignRecords_ASCV0778`, `TestCacheEnvironmentKeepsBehaviourVariables_ASCV0778`, `TestResultCacheKeysTheCostBytesReplayRead_ASCV0778`; timings in `docs/build-log/2026-11-01-csp-w33.md` | implemented; measured |
| ASC-V0-779 | superseded by `DTL-V0-006` and `ASD-V0-782` (origin/main); no implementation of its own | superseded; `docs/build-log/2026-11-06-mbrescue.md` |
| ASD-V0-3720 | `script/dogfood-finish.sh` `tooling_closure`; `script/dogfood-cache_test.sh` (V1-3257): a staged `internal/trace` change imported by `internal/dogfoodflow` refuses the override, an unimported package does not |

## Non-goals and limits

No change to the doc-gate list or its order, to the ledger key, or to the selection-baseline verdict; each requirement states its own non-goals.

## Rollback

Each requirement states its own rollback; none changes a stored format other than the optional ledger `label` field and the selection-baseline result cache, both of which older binaries ignore.

Roll back `ASD-V0-3720` by removing `tooling_closure` and its call from `script/dogfood-finish.sh` and its `dogfood-cache_test.sh` case.
