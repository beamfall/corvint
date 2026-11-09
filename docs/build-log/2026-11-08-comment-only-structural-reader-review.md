# Preserve structural coverage gaps and source-reader callers

V1-5176 / PR 153 follow-up, based on `f39aa4706e5754a572bd4536048b338cb096c767`.
The owner requested exactly two review repairs under AFP-V0-3322 and AFP-V0-3325.

`commentOnlyUnit` now checks all three Go structural result sets as well as current
ownership. A retained comment-only seed cannot hide a deleted or renamed-away source,
an added source, or a direct/enclosing data input. Testless literal/glob readers now
carry callers for admitted comment-only paths even when their owner has tests, since
compilation propagation no longer supplies that coverage. The spec clarifies both rules.

Two regressions use real Go adapter discovery and BASE/HEAD comment classification:

- `TestCommentOnlyStructuralCoverageGap_AFP_V0_3325` appends a comment to `lib/a.go`
  and deletes `lib/b.go` from a package with no tests or callers. Before the fix,
  `NO_SELECTABLE_TEST` disappeared. Direct/nested data cases also failed; addition
  and rename controls retained their coverage gap.
- `TestCommentOnlyTestlessReaderOfTestedOwner_AFP_V0_3322` makes a testless `reader`
  import the tested `lib` and read `lib/a.go`; a tested `consumer` imports `reader`.
  Before the fix, only `lib` was selected. The fixed plan selects `consumer` through
  the independent reader. A parent-relative literal initially triggered a separate
  unbounded root-reader rule; the final fixture uses `lib/a.go` to isolate this finding.

The two new tests failed before production edits. Afterward, the requested heavy-wrapper
test command with `-run 'TestCommentOnlyStructuralCoverageGap|TestCommentOnlyTestlessReaderOfTestedOwner|CommentOnly'`
and `GOTOOLCHAIN=local go vet ./internal/liveverify/affected/` passed with Go 1.27.3.
Raw fail-before and passing logs are retained locally in `.lane-tmp/v1-5176-*.log`.
`make spec-requirements` passed with no generated changes. `make spec-requirements-check`
and `make changed-checks BASE=f39aa4706e5754a572bd4536048b338cb096c767` passed after
removing the alternate-index environment from their child Git fixture tests; the first
attempts failed because that environment contaminated fixture indexes. The changed checks
retain the existing non-failing `internal/tasks/cli` unbounded-reader advisory.

Corvint context and affected planning ran; installed Core build 136 reported stale source.
The official version check retained freshness UNKNOWN because sandbox process inspection
failed. No update was installed. Tasks, network writes, subagents, full gate and post-commit
bind/seal are outside this explicitly local review repair. Git's normal index write is
blocked by `index.lock: Operation not permitted`; a worktree-local alternate index retains
the staged edits for spec generation. No commit or native ticket completion is claimed here.
Rollback is to revert these two guards; there is no stored-state or wire-format change.
