# Delta reintegration on current main and owner docs-only deferral

Issue: #389 (part of #388) / native V1-0536. Owning contract: `docs/specs/immutable-delta-v0.md`.
New base: `cdd2e31fffd732419eb9ade531ccf289259f3243` (origin/main on 2026-10-01).
Imported source: `codex/issue389-public-integration` head `73e76c35188adc1e1fa65799b3002469f08cfc87`,
whose base `7bd7f5e03ad177e7496ce5dbd1563b8466f591a4` is 105 commits behind the new base.

## Decisions

1. **Reuse the existing branch (owner, 2026-10-01).** The owner directed "reuse the existing codex
   branch for 389". The branch was squash-imported onto the new base in a plain clone. The original
   ref was neither moved, rewritten nor deleted. The import merged cleanly: `docs/specs/INDEX.json`,
   `README.md` and `REQUIREMENTS.tsv` were auto-merged. The Codex bind map
   `.corvint/change.cem.json` was dropped because it binds the old base, and this change is rebound
   against the new base. The three use-case implementation receipts were repinned from origin/main
   with `script/repin-use-case-receipts.sh`. That keeps `repositoryRevision` reachable from this
   branch instead of naming the unpublished Codex commit `1e09757b`. The `cmd/corvint/main.go`
   digest is the same.
2. **Docs-only deferred (owner, 2026-10-01).** The owner decided "keep docs-only as tests-needed
   for now" for the V1-0579 strict-provider conflict. The classification is unchanged. Native
   `docs-only` is an accepted interim limit. V1-0579 stays open as the follow-up. The spec digest,
   acceptance section and README row record this. The closed schema still carries `docs-only`, and
   its wire vector still passes.
3. **Root-help conflict.** After the import, `origin/main` had pinned CCF-V1-008 in
   `TestRootHelpLabelsEveryVerbWithMaturityAndOwner` and `TestInvalidChoiceNamesEveryDispatchedTopLevelVerb`.
   Those tests require every dispatched verb to appear in root help, labelled Core or
   Experimental with an indexed owner prefix. The imported `delta` verb was dispatched but not
   listed, so both tests failed. Root help now lists `delta` as Experimental and labels it
   `delta (DLT-V0)`. It is not a Core verb.
4. **Requirement anchors (V1-0583).** Each DLT-V0 requirement now has exact-ID Go subtests
   (`t.Run("DLT-V0-NNN <words>", ...)`) on tests whose assertions establish it. The spec's anchoring
   table lists them. Whole-function tests use a two-line wrapper that leaves their body unchanged.
   The fixture-merge test splits its existing assertions into labelled subtests and adds a binding
   check (base, head, tree, build, digest, changed paths). A new test,
   `TestDeltaIncompleteProviderRequiresFullSuite`, covers the issue's "incomplete provider coverage
   yields run-full-suite with the reason" criterion at compile level. A malformed record yields
   `external-coverage-incomplete`. An unreadable file yields `provider-capture-unavailable`. Both
   set `runFullSuite` and `findings`.

## Limits retained

Earlier Codex receipts, enrollments, native qualifications and `/tmp` evidence are history for the
old base. None of them transfers to the new base. Fresh focused checks and this change's own CEM
binding are the evidence for the new base. V1-0564 (full affected-package live-worktree oracle),
V1-0565 (flowdocs bounded HTML), V1-0581 and V1-0582 remain open and are not addressed here.
Runtime behaviour remains unknown.

Rollback: revert this change's commits. The original Codex branch and its retained evidence are
untouched.
