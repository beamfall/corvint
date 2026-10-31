# Release gates nightly triage: run 41059652620 on 21fea70c (V1-3055, V1-3177)

The scheduled Release gates run 41059652620 at `21fea70cb` failed five jobs. Each failure was traced
from its job log and checked again on `origin/main` `780f9e970`.

## Findings

- `core-n1-replay-v0.8.1` and `core-n1-replay-v1.0.0-rc.1` (jobs 124075897155, 124075897077):
  `fatal: not a valid object name`. The single-root repository has no pre-1.0 tags. This is already
  repaired on `main` by `9dc17ba` (build log `2026-10-30-n1-first-release.md`). Owner decision
  2026-10-30: those releases are not restored or replayed. One `core-n1-replay` row runs
  `script/core-n1-source.sh --replay` against the previous release published in this repository,
  and for `v1.0.0-rc.3` it reports a visible `NOT_APPLICABLE FIRST_RELEASE`. The CCF acceptance
  evidence bullet still named `v0.8.1`; this change amends it, with no requirement ID change.
- `linux-core` HLQ-V1 (job 124075896619): the exit 2 came from the `n1-archive` step.
  `select-previous-release.sh` refused with `no reachable previous release`, and every later N-1
  and HLQ step was skipped. The ripgrep `case` line near the end of the log is echoed workflow text,
  not a failure. This is also repaired by `9dc17ba`: exit 3 sets `first_release=true`, and HLQ then
  runs without `--base-corvint` and accepts only an exact report. A local run of
  `conformance/host-lifecycle-v1 --host cli` with no base, on a clean `780f9e970` source, wrote
  `case upgrade NOT_RUN no --base-corvint supplied` and
  `SUMMARY status=FAIL passed=8 failed=0 notRun=1`. The workflow's acceptance predicate matches both
  lines exactly. `go run` turns the runner's exit status into 1, which the step expects.
- `full-gate` macos-15 and ubuntu-24.04 (jobs 124075897179, 124075897175): `--- FAIL: TestWd` and
  `workflow_context_test.go:318: refused: CONTEXT_UNAVAILABLE` are not failures. `TestWd` is the
  deliberately failing fixture package that `tools/gate-ledger` `TestGoTestPartialPassRerun` runs.
  The refusals are `t.Logf` lines of the passing subtests of
  `internal/tasks/store` `TestWorkflowContextRefusesIncompleteScopeAndAbsentSource`. Both packages
  report `ok`, and no `FAIL github.com/...` package line appears. The real failures were two
  `make -k gate` steps:
  - `cross-vet`: `syscall.Mkfifo` is undefined for `GOOS=solaris`. `internal/doccompiler` was already
    fixed on `main` (`d66e873`, V1-3196). Current `main` still failed the same way in
    `internal/dogfoodflow/propose_refresh_fifo_unix_test.go` and `verify_quote_fix_unix_test.go`,
    both tagged `//go:build unix`. They now carry the doccompiler constraint
    (`darwin || dragonfly || freebsd || linux || netbsd || openbsd`). The solaris branch of
    `cross-vet` runs only in `make gate`, so a new `unix`-tagged FIFO test can merge unseen.
  - `license-headers-check`: the three public-root-history notice fixtures were exempted on `main`
    by V1-3208. Current `main` failed again on
    `internal/tasks/similar/testdata/corpus-2026-10-30.jsonl`, the frozen task-store ticket corpus of
    the TCL-V0-009 duplicate evaluation. Like `.taskman/**`, it quotes the former terms as
    historical record, and its recall thresholds are measured on those exact records. The check
    exempts exactly that file, `LICENSING.md` records it, and
    `script/check-license-headers_test.sh` proves that a sibling corpus quoting the former header
    still fails.

## Evidence and limits

These pass locally on this change:

- `make cross-vet` (under `script/heavy.sh`)
- `make license-headers-check` and `script/check-license-headers_test.sh`
- the two dogfoodflow FIFO tests on darwin
- `script/select-previous-release_test.sh`, `script/core-n1-source_test.sh` and
  `script/core-n1-replay-default_test.sh`, run under bash as the Makefile runs them

NOT_RUN:

- `make gate`, which lane policy excludes
- a hosted Release gates run
- the HLQ codex and claude-code hosts

The next scheduled run, or a `workflow_dispatch` of Release gates on the merged head, is the
acceptance evidence for V1-3055 and V1-3177.

Rollback: revert this change. The nightly `cross-vet` and `license-headers-check` steps would then
fail again.
