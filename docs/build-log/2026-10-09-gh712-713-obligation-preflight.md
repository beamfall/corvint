# 2026-10-09: expected-fail witnessing and obligation preflight (GitHub #712, #713)

## Intent

GitHub beamfall/corvint#712 and #713 extend the experimental obligation ledger
(`docs/specs/corvint-tasks-obligation-ledger-v0.md`, decision 0456; V1-1022).

- #712: a Playwright `test.fail` test should not read as a failed obligation. An ordinary
  obligation named inside one can never pass, and the run should say so. A run whose post-check
  failed should credit nothing.
- #713: an explicit, read-only check before a long run. It covers obligations no test names,
  `test.fail` tests that mix ordinary obligations, split tests, the plan check and the
  lint/contract gates on a clean tree.

This change adds TOL-V0-022..027 as proposed requirements, pending owner acceptance. It records
no decision, and decision 0456 does not cover them.

## Change

- Report reader (`internal/tasks/obligation/report.go`, `expected_fail.go`):
  - A test is an expected failure when it has a `fail` annotation (test or retry-0 result) or
    `expectedStatus: failed`.
  - Matches inside it never pass.
  - An obligation that is expected-fail there and failed is reported in `defectConfirmed`. It is
    expected-fail when its title says so, or a defect id appears in the fail description, the
    title or the step error. The entry carries the defect id and the error's first line, bounded
    to 240 bytes and secret-screened.
  - Any other obligation is reported in `mixedExpectedFail`, with its test ids and the remedy.
  - Witness still never sets `DEFECT` (TOL-V0-011), and the WITNESS event is unchanged.
- Witness (`internal/tasks/cli/obligations.go`):
  - New flags `--post-check LOG --post-check-status N`.
  - A non-zero status refuses `GATE_FAILED` with the new detail prefix
    `OBLIGATION_POST_CHECK_FAILED:` (`internal/tasks/ticket/obligations.go`). The refusal quotes
    the log's first actionable line and happens before any read of the store or write.
  - No new result code.
- Preflight (`internal/tasks/cli/preflight.go`, `internal/tasks/obligation/source.go`,
  `internal/tasks/store/preflight.go`):
  - Command: `corvint-tasks preflight <ticket> [--commit OID] [--path PREFIX ...] [--plan PATH]
    [--deep --gate GATE ...]`.
  - It lists spec files with `git ls-tree` at the commit (at most 4,096) and reads them with the
    existing `FilesAtCommit` (1 MiB per blob), so it needs no checkout.
  - A heuristic tokenizer finds:
    - test, describe and step titles;
    - `test.fail` forms, including describe- and file-level `test.fail(...)` calls;
    - `isolated` annotations;
    - `<contract>:<id>` contract annotations.
  - It reports `UNNAMED`, `MIXED_EXPECTED_FAIL` (file:line), `SPLIT_TESTS`, the TOL-V0-020 plan
    findings, `PLAN_MISSING` and `DEEP_CHECK_FAILED`, each with a remedy.
  - `--deep` resolves the gates under the lease runner's COMMAND gate rules. It runs them in one
    temporary detached worktree of the commit and removes that worktree on every path.
  - Any finding refuses with the item status `PREFLIGHT_FAILED`. The verb is a `ReadVerbs` member
    and writes no queue state.
- Help: `command_help.go` gives the new witness flags, the preflight usage and notes.

## Verification

- Failing before, on the base code:
  - `TestTOLV0022_ExpectedFailDefectConfirmed`: `defectConfirmed` was absent, and the expected-fail
    ids were `failed`.
  - `TestTOLV0023_MixedExpectedFail`: `mixedExpectedFail` was absent.
  - `TestTOLV0024_PostCheckRefusesCredit`: the result was `unknown flag --post-check`.
  - `TestTOLV0025_*`, `TestTOLV0026_*` and `TestTOLV0027_*`: the result was `unknown verb`.
- Passing after:
  - those tests, plus `TestTOLV0025_ScannerSkipsCommentsRegexAndInterpolation`
    (`internal/tasks/obligation`);
  - the existing TOL-V0 and help tests in `internal/tasks/cli`;
  - `internal/tasks/obligation`, `internal/tasks/store` and `internal/tasks/ticket`.
- `go vet` on the four touched packages, and gofmt.
- The local doc gates listed in the task.

## NOT_RUN and limits

- The live Playwright 1.63 `test.fail` fixture is `NOT_RUN`. The qualified version list stays
  empty, so a real report still refuses `UNSUPPORTED_VERSION`.
- The pool-acquire hook is a documented follow-up, not implemented. Preflight is explicit only.
- `defectConfirmed` is reported, not persisted, and the post-check is not recorded in the event.
- The source scan is not a JavaScript parser:
  - a name built only by `${}` interpolation, or outside a string literal, is not seen;
  - the defect-id shape `[A-Z][A-Z0-9]{0,15}-N` is a heuristic.
- A cross-ticket prefix collision in shared spec files is narrowed only by `--path`.
- `--deep` creates a transient worktree administrative entry. `SIGINT`/`SIGTERM` removes it, but
  `SIGKILL` can leave the worktree, and `git worktree prune` repairs it.

## Round-1 review (Codex) and fixes

Codex round 1 returned FAIL with six findings; the orchestrator decided to fix all six and amend
only the proposed TOL-V0-025 and TOL-V0-027 wording. Each fix has a test that failed on `eb9c69c5`
and passes after.

1. P1, hooks: `--deep` worktree creation ran the repository's `post-checkout` hook. Every Git
   command preflight runs now passes `-c core.hooksPath=/dev/null -c core.fsmonitor=false`. Checked
   on git 2.54 that this form suppresses `post-checkout` on `worktree add`.
   `TestTOLV0027_PreflightDeepRunsNoRepositoryHooks` failed with the hook's marker written.
2. P1, interrupt: the deep path now runs under `signal.NotifyContext` (INT, TERM). On
   cancellation, the lease runner's `execute` kills the gate's process group and waits for it, then
   the worktree is removed and preflight refuses `UNSUPPORTED`.
   `TestTOLV0027_PreflightDeepInterruptRetiresGateAndWorktree` uses a subprocess, a `sleep 600`
   gate and SIGTERM. It failed with the gate's group still alive after the process died.
3. P2, source-changing gate: after each gate, the worktree's HEAD, HEAD tree and porcelain status
   must still match the commit, as the lease runner requires. Otherwise the gate is a
   `DEEP_CHECK_FAILED` finding naming the change, and no later gate runs.
   `TestTOLV0027_PreflightDeepGateThatChangesSourceFails` failed because the truncating gate
   passed and the later gate ran.
4. P2, describe titles: a test's describe titles now count as its title path for
   `MIXED_EXPECTED_FAIL`, with the defect-id rule applied to that title, and for `SPLIT_TESTS`. In
   SPLIT_TESTS, a test inside an id-bearing describe names that id, so it is no longer a
   single-id test. `TestTOLV0025_DescribeTitleIDsReachNestedTests` failed, reporting a spurious
   SPLIT_TESTS where `MIXED_EXPECTED_FAIL:AC-1` was expected.
5. P2, tree listing: `--path` prefixes are passed to `git ls-tree` as literal pathspecs, and the
   output is streamed. The 4,096-file limit and a 64 KiB per-path bound are enforced while reading,
   and the listing stops at the first violation. `TestTOLV0025_PreflightPathNarrowsTheTreeListing`
   uses a commit whose unrelated subtree object is absent. It failed with a git observation error.
6. P2, oversized blobs: the `--batch-check` call now also answers `%(objectsize)`. `FilesAtCommit`
   requests content only for blobs of at most 1 MiB, and the know-how callers ignore the new size.
   `TestTOLV0025_PreflightSkipsOversizedSpecUnread` uses a 2 MiB blob whose loose object keeps only
   its header, so its content cannot be streamed. It failed with `git answered short`.
   The answer parser now requires all three fields, so `TestKHNV0015_CommitRaceResolvesOneCommit`,
   which starts its own `cat-file`, now uses the same three-field format.

Not added: a bound on the total number of entries scanned without `--path`. The listing is
streamed in constant memory, but its time still grows with the tree.
