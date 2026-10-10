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

## Round-2 review (Codex) and fixes

Codex found three issues in round 2. Each fix has a regression test that failed on 7728f366 and
passes after the fix.

1. P1, descendants of earlier gates: `execute` stops watching a gate's process group once the
   gate's own process exits, so a background process the gate started survived later gates and an
   interrupt. Each gate already runs in its own process group (the lease runner's `Setpgid`), and
   `execute` now returns that group's id. Preflight kills the group with `SIGKILL` as soon as each
   gate returns, whether the gate exited or the run was interrupted. The lease runner's own
   behaviour is unchanged.
   - `TestTOLV0027_PreflightDeepInterruptRetiresEarlierGateDescendants` runs Codex's repro: one
     gate leaves `/bin/sleep 600` in the background, then a later gate is interrupted. It failed
     with "the earlier gate's background process ... survived".
   - `TestTOLV0027_PreflightDeepRetiresAFinishedGatesDescendants` checks the same without an
     interrupt and failed in the same way.
   - Both tests read the background pid from a file beside the worktree.
   - Limit: a process that leaves the gate's group, for example with `setsid`, is not tracked.
   - Limit: the group is killed just after the gate is reaped. While any member is alive, the
     group id cannot be reused. Once the group is empty, a kill could in principle reach an
     unrelated new group that reused the id in that window.
2. P2, buffered status: the post-gate `git status --porcelain -z --untracked-files=all` is now
   streamed. The first byte settles that the worktree is dirty, so Git is then killed instead of
   being read to the end.
   - `TestTOLV0027_PreflightStatusStopsAtFirstEntry` uses a command that writes one entry and then
     sleeps for 600 s.
   - Before the fix, with the old buffered `Output()` read applied to the same seam, it failed at
     its 20 s guard.
3. P2, a line break in a spec path: such a path broke the line framing of `cat-file --batch-check`
   and shifted every later answer. In the test, `e2e/checkout.spec.ts` went unread and AC-1 was
   reported `UNNAMED`.
   - Preflight now never asks for such a path. It reports an `UNREADABLE_SPEC_PATH` finding for
     that file (a new finding kind), and the other files read normally.
   - A `--path` or `--plan` value with a line break is a usage `ERROR`.
   - `catFileAtCommit` refuses a path with a line break, so no caller can shift the answers. This
     guard also changes know-how, which previously got corrupted answers for such a path and now
     gets a refusal.
   - `TestTOLV0025_PreflightRefusesALineBreakSpecPath` uses `e2e/a\nb.spec.ts`. It failed with
     `UNNAMED` findings for AC-1, AC-2, AC-3 and AC-5.

## Round-3 review (Codex) and fix

Codex found one P1 in round 3. The first-byte read of the post-gate `git status` watched no
context or deadline.
- Repro: a tracked file has a slow clean filter, and the gate rewrites that file at the same size.
  Git must then hash the file, so status runs the filter and hangs. `SIGINT`/`SIGTERM` cancelled
  only the context, which nothing was reading, so preflight stayed and the worktree was left
  behind.

The fix covers every post-gate Git call: the two `rev-parse` calls and the status.
- Each runs in its own process group (the lease runner's `containGate`).
- Each is watched by the preflight's signal-aware context plus a deadline, and the group is killed
  with SIGKILL when either ends.
- The deadline is the gate's declared `timeoutSeconds`, so it reuses the policy's bound for that
  gate.
- A cancel returns the interrupt refusal. A missed deadline is a `DEEP_CHECK_FAILED` finding ("its
  worktree could not be checked"), never a pass, and no later gate runs.
- The worktree removal (`git worktree remove --force`) does not run the clean filter, and the
  worktree is removed on both paths.
- TOL-V0-027 states the bound. It also states a limit that is out of scope here: creating the
  worktree, including smudge filters, is still unbounded.

Regression tests (both failed on 96d6cb1c):
- `TestTOLV0027_PreflightDeepInterruptStopsAHungStatus` sends SIGTERM while the filter runs and
  before any status byte. It failed with "preflight did not exit after SIGTERM" after 60 s.
- `TestTOLV0027_PreflightDeepStatusPastTheGateTimeoutFails` uses a 2 s gate timeout. It failed by
  hanging until the 100 s test timeout. It now returns in about 2.7 s.
- `TestTOLV0027_PreflightStatusStopsAtFirstEntry` gains a case where a silent command is stopped by
  its context.
