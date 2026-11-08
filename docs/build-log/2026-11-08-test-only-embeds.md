# V1-5177: test-only embeds

Owner-requested experimental fix, 2026-11-08; base
`19c5a0111d667a11fd3a82ebcc8920f8a848260c`, branch `claude/v1-5177-test-embeds`.

The Go observer ignored `_test.go` embed directives. Editing a non-imported subpackage's
embedded source therefore omitted the embedding unit's tests. Separate `TestEmbeds` and
`TestEmbedFiles` facts now select its tests as byte readers, without adding production
imports or binary consumers. Production embed facts stay separate; independent structural
selection rules are unchanged. Malformed directives and invalid patterns in either scope
retain `go:unparsed-source`. Both scopes populate the resolved index used by AFP-V0-3324.
Test facts are validated, digested and cached; cache profile `/1` rejects old `/0` entries.
AFP-V0-3350 to AFP-V0-3352 record intent and rollback.

Evidence:

- `TestTestOnlyEmbedSelectsNonImportedSource_AFPV03350` failed before the fix in all four
  internal/external-package and code/comment cases: selected only `cat/sub/x_test.go`.
  Temporarily restoring `!isTest` reproduced those same failures; restoring the candidate
  passed the focused Go suite. Logs: `/tmp/v1-5177-before.log`,
  `/tmp/v1-5177-reverted-before.log`, `/tmp/v1-5177-golang.log`.
- Focused `golang` and `affected` suites passed, including malformed patterns, production
  importer/binary isolation, deleted targets, the narrowing guard's index, digest/path
  validation, real cache round trips and old-profile rejection. Commands used `-count=1`,
  `-timeout 30m` and the requested narrow `-run` filters through `script/heavy.sh`.
  Go tests ran on the configured WSL host; affected tests ran locally on Darwin.
- V1-5176 is absent from this base. Its affected-package diff at `f39aa4706` was combined
  with this patch in `/tmp/v1-5177-integration.CYgriv`. Both focused suites passed, including
  `TestCommentOnlyTestEmbedIntegration_AFPV03351`, which calls the actual `WithGoChanges`
  and proves it refuses test-embedded source and retains the source's importers and the
  embedding tests. Evidence: `/tmp/v1-5177-integration.log`; integration fixture retained
  in that disposable checkout. No sibling branch files were changed.
- `GOTOOLCHAIN=local go vet ./internal/liveverify/affected/...` passed with Go 1.27.3.
  Spec generation/check and changed checks passed. `make unbounded-readers-check` passed
  with an existing stale `internal/tasks/cli` inventory warning; unrelated inventory was
  preserved. Independent Astra/medium review found no blockers; its contract wording
  clarification about independent structural rules was incorporated.

Corvint context and affected selection were used, retaining their bounded omissions and
unknowns. Installed Core is `1.0.0-rc.3.dev`, build 136, commit `19c5a0111`; update freshness
is UNKNOWN because sandbox process-group inspection refused the update check. The initial
`make dogfood-change` could not create the worktree Git metadata directory; CEM binding/seal
remain NOT_PRODUCED. Account usage telemetry was unavailable. No Tasks command, push,
whole-repository test or `make gate` was run. This is a local build handoff, not ticket
completion or release qualification. The sibling narrowing implementation must still be
integrated into the eventual delivery branch.
