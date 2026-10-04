# Unbounded test reader ratchet and selected-share report (V1-0719)

Human-owned intent: ticket V1-0719, recorded as owner-directed, proposed AFP-V0-025. Selection
promotion stays blocked (V1-0616); nothing here narrows what CI runs.

## Finding

The ticket's figures were stale. Measured 2026-10-04 at `2b533372` with a planner built from the
tree: 51 test packages were unbounded readers (rule (d)), holding 2150 s of the 5518 s the
partition's cost estimates price (39%, not 75%). The installed planner (1.0.0-rc.1 build 163)
predates the AFP-V0-023 declarations and still reports 108. Nothing failed when the count grew, and
no run reported how much of the suite a plan selected.

## Change

- `tools/unbounded-readers` and `make unbounded-readers-check` (a `doc-gates` and `make gate`
  step): build the planner's unit graph and fail when the unbounded test units exceed the ceiling
  in `.corvint/unbounded-readers.json`, when a recorded reason names a package that is not
  unbounded, or when the record is absent or malformed. `Graph.UnboundedReaders` is exported for it.
- `.corvint/test-read-scopes.json` declares six more packages, each run to a pass under the
  Landlock wrapper with exactly the committed scope: `internal/tasks/authority`,
  `internal/tasks/intent`, `internal/tasks/store` (`go.mod` only), `internal/analyzernativebridge`,
  `tools/corvint-pr-tests` and `cmd/corvint-go-test-provider`. The last two build other packages in
  a subprocess, so their scopes are the coarse subtrees `cmd/`, `internal/`, `tools/` plus the
  module files; that is wide, and still takes them off the floor for documentation and workflow
  changes.
- `.corvint/unbounded-readers.json`: ceiling 45, with reasons for the costly packages that cannot
  be declared: `cmd/corvint` (a test opens the filesystem root `/`, an ancestor the wrapper
  cannot grant without the whole repository), `internal/companionrelease` (its tests run git against the
  repository, and `.git` is never grantable), `internal/contextindex` and
  `internal/liveverify/affected` (both read the whole repository by design).
- `ci-shards --share PLAN` and two advisory steps in shard 0 of `go-product-shard`: the estimated
  time of the plan's selected packages as a share of the universe, written to the step summary and
  retained as the `affected-share` artifact (`corvint-ci-selected-share/0`).

## Evidence and limits

- Unbounded test packages 51 to 45; their estimated time 2150 s to 1276 s (39% to 23% of the
  priced suite). Of the ten costliest on the ticket's date, six are declared and four carry a
  reason.
- A one-line documentation edit, planned with the tree's planner and priced by `--share`: 54
  packages and 2170 s selected before (387 permille), 48 packages and 1296 s after (231 permille).
- `TestAFPV0025RatchetFailsAboveTheRecordedCeiling`, `TestAFPV0025RatchetWithoutARecordRefuses`
  and `TestAFPV0025ShareReportsSelectedEstimatedTime` pass; the helper also passes in the isolated
  module the workflow builds. `actionlint` and `ci-least-privilege-check` pass.
- Scopes were verified in a local Linux container (kernel 6.8, Landlock ABI 4, `/tmp` on tmpfs)
  without `-race`; hosted CI runs them with `-race` and is the first hosted observation. A scope
  that is too narrow fails that package in full CI rather than under-selecting.
- The hosted share report is `NOT_OBSERVED` until this change's own pull-request run. The estimates
  are one retained main run, and 45 packages are unpriced and take the median.
- Declared scopes change no CI selection today: pull-request CI still runs the complete universe.
- Found on the way and filed: V1-0739 (`TestSelectionOnTheLiveDirtyWorktree` rejects a valid
  witness whenever `.github/cishards` is dirty; clean trees are unaffected).

## Rollback

Remove the `unbounded-readers-check` step and target, the record, and the two share steps. The six
declarations can be deleted individually; each deletion returns that package to rule (d).
