# Dogfood plan and trace caps — V1-0394 / V1-1107

Intent remains proposed, pending owner acceptance: DCW-V0-035..036 in
`docs/specs/daily-change-evidence-workflow-v0.md`. No decision or bound increase.
The coordinator separates citation/OCM-link row overflow from malformed plans and prints
limit=256 with the observed rows. Current, stored and changed-path admission trace lists name their 200-path overflow
and observed distinct count. Existing codes for other invalid input remain unchanged.

The documented staged route keeps one base/patch and map, applies 256 then 23 explicit rows
for 279 hunks, commits only the fully cited sidecar, and reruns/checks the same base. Range
binding cannot stage one oversized commit: its map check requires all unknowns named in one
plan. No unknown is promoted implicitly. Independent trace/OCM-link limits still apply.
Rollback reverts these diagnostics, tests and documentation; no stored format changes.

## Evidence and limits

- Base: `29e1f653486fca13216606683d3a5c2b05cacd36`.
- Corvint query used; receipt `/private/tmp/v10394-query.json` retains ranked omissions and
  test-path exclusions. The query's revision was not a commit in this checkout; the initial
  affected invocation refused `unsupported-affected-revision`. The corrected invocation used
  this checkout's HEAD: `/private/tmp/v10394-affected-current.json`. Its static full-gate advice
  is excluded by the owner's explicit focused-only instruction; no full-suite claim.
- Index refresh used: `/private/tmp/v10394-index.log`. Optional mutation/provider/host routes
  are not applicable. Post-commit CEM/seal and native ticket completion are NOT_RUN: this
  builder is explicitly forbidden to commit or write `.taskman`.
- Installed Core: 1.0.0-rc.3 build 407; Tasks: 0.0.0-tcp01-unverified+build.407.
  Official updater checks for both components left freshness UNKNOWN because the version smoke
  failed (`process-after-start-failed: descendant snapshot unavailable`); no update installed.
  Account usage telemetry is NOT_OBSERVED (no available usage read tool); no spend claim.
- Original worktree staging is blocked: its Git index is outside the writable sandbox.
  Staged-input generation/checks use `/private/tmp/v10394-final-index` and private objects, with the
  original objects read as alternates. Default macOS `mktemp` ignores TMPDIR without a template;
  a temporary PATH shim supplies an explicit permitted template for zero-argument calls.
- PASS: `go test -race -count=1 -timeout 30m ./internal/dogfoodflow ./internal/trace`
  with `-run 'TestPlanRowLimits|TestTracePathLimitDiagnostic|TestNewRecordValidationBounds'`.
  After the admission repair, PASS: `./internal/trace` with
  `-run 'TestTracePathLimitDiagnostic|TestNewRecordValidationBounds|TestAdmissibleCurrentPathsSharesRecordAdmission'`.
- PASS: all requested documentation checks, including the final fresh-index batch
  (`/private/tmp/v10394-doc-gates-final.log`, exit 0). The receipt fixture test ran separately
  with temporary Git-index/object variables unset: five cases passed. The separate
  `sh script/repin-use-case-receipts.sh --check` passed: all 18 receipts current. No main.go
  change, receipt rewrite or full gate. `git diff --cached --check` passed on the final index.
- The first receipt fixture run inherited the temporary index and replaced its staged contents;
  only scratch metadata was affected. Final staging/checks used a fresh index, and the receipt
  fixture test was rerun without those Git environment variables.
- PASS: `go test -race -count=1 -timeout 30m ./cmd/corvint -run
  '^TestDogfoodSingleCommitStagedCitationPlans$'` (`/private/tmp/v10394-staged.log`).
  A real binary prepared one 279-hunk commit; stage 1 published 256 supported/23 unknown;
  stage 2 published 279 supported/0 unknown; after the fully cited sidecar commit, the same-base
  final change completed and `dogfood-check` reported supported=279, unknown=0, mechanical=0
  and PASS. Commits occurred only in the disposable test fixture, never in this worktree.

Independent review (native `/root/review`) found the changed-path admission diagnostic missing
its observed count. Repaired the shared admission message and added 200/201 assertions while
preserving the structured `admitted-path-limit` reason used by aggregate fallback. Readback
review found no remaining material findings. The first row-bound fixture lacked a CEM and
stopped before validation; initializing the CEM proved the path, then the focused rerun passed.
