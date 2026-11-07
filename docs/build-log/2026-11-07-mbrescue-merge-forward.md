<!-- SPDX-License-Identifier: Apache-2.0 -->
# mbrescue merge-forward of origin/main 538e823f4

Lane `claude/mbrescue` (PR #126), bound unsealed at `a01248c31` over base `4052b854`, merged
`origin/main` `538e823f4a1d220a3fb5b12c0529248f18924823` (PR #124, `claude/dogfood-friction`)
forward with `git merge --no-ff` in merge commit `1fc5b1357`. The lane binds and seals the whole
range from that mainline commit, by the route in `docs/DOGFOOD.md` "Rebinding after a conflicted
merge-forward" (`DWC-V0-7601`, `DWC-V0-7602`). No `-X ours` or `-X theirs`, no rebase, no amend.

## Conflicted paths

`script/dogfood-merge-base.sh --paths` names these 11 paths. Each resolution keeps both sides.

- `.corvint/shell-foreign-callers.json`: main added `script/dogfood-merge-base_test.sh` to the
  mentions of `script/batch-precheck.sh` and `script/dogfood-check.sh`, with a sentence to each
  reason saying the test runs only a scratch stub of that name. The lane added
  `script/merge-drivers.sh` to the same two mention lists. The merged lists hold both, sorted. The
  lane's file was already 65,535 of the 65,536 bytes `MaxShellForeignCallerBytes` admits, so the
  plain merge is 65,935 bytes. `TestRepositoryShellForeignCallersComplete_TSP_V0_005` then fails
  with `repository walk exceeds its bound`. Every `callers` and `mentions` array now drops the
  space after each comma. This changes only whitespace (the parsed JSON is identical), and the file
  is 64,910 bytes. `make shell-foreign-callers-check` passes.
- `Makefile`: `dogfood-closeout-repin-test` (lane) and `dogfood-merge-base-test` (main) both
  join `GATE_STEPS`, `shell-test` and their own rules. The lane's `closeout_step` recipe for
  `dogfood-closeout` is kept, and its step 1 takes main's `BASE="$(or $(DOGFOOD_BASE),$(BASE))"`,
  so the spec checks see the derived SHA under `BASE=merge-forward`.
- `internal/dogfoodflow/finishlane.go`: both imports, `io/fs` (main) and `maps` (lane).
- `script/dogfood-cache_test.sh`: the lane's repin-first ordering and SEAL=0 resume case is
  kept. Its recipe assertion now expects main's derived-BASE spec-check step, and its comment
  gains main's `DWC-V0-7601` clause.
- `docs/specs/compat-replay-runner-v0.md`, `eol-policy-gate-v0.md`, `go-archive-gate-v0.md` and
  `release-artifact-integrity-v0.md`: each `Makefile:N@sha` citation is recomputed against the
  merged Makefile. The cited bytes match the anchor hash, and every line moves down by one:
  - 392 is `spec-requirements-check`;
  - 502-508 is the EOL phony block;
  - 97-98 is the EOL `GATE_STEPS` pair;
  - 246 is `go-archive-gate`;
  - 155 is `gate`.
  `make line-citations-check` passes on the staged merge.
- `docs/specs/daily-change-evidence-workflow-companion-v0.md`: the Exists line names both
  sides' code. The failure-mode and trace tables hold the lane's `DWC-V0-745` to `DWC-V0-3732` rows,
  then main's `DWC-V0-7600` to `DWC-V0-7603` rows. The rollback line covers both. Each of the 35
  `DWC-V0` IDs is defined once and has one trace row.
- `docs/specs/REQUIREMENTS.tsv`: regenerated from the staged specs by
  `make spec-requirements-resolve`, never edited by hand. `make spec-requirements-check` passes.
- `script/dogfood-merge-base_test.sh`: no text conflict. Main's probe expects exactly four stub
  calls, but the lane's `DWC-V0-758` DONE line in `dogfood-closeout-precheck` asks
  `batch-precheck.sh --verify-receipt HEAD` after each passing precheck. The expectation now lists
  that call too. The test fails at line 127 before this change and passes after it.

## Checks on the merged tree

The checks below all pass on the staged merge, before the merge commit:

- `script/dogfood-cache_test.sh`;
- `script/dogfood-merge-base_test.sh`;
- `script/dogfood-closeout-repin_test.sh`;
- `script/batch-precheck_test.sh`;
- `go test` through `script/heavy.sh`: whole `./internal/dogfoodflow` and
  `./internal/liveverify/affected`, and `-run '^TestDTLV0004'` on `./cmd/corvint`.

`corvint affected --worktree` over the 28 paths the merge changed selects 419 units. Most are
`driver:make` targets, because the Makefile changed. Tier 0 holds three Go packages and three
shell tests, and all six ran. The tier 1 and tier 2 units did not run (`NOT_RUN`): the
orchestrator asked for the touched packages and scripts.
