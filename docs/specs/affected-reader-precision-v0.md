# Affected Reader Precision V0

Owner: Russell Lewis
Date: 2026-11-05
Requirement prefix: `AFP-V0`
Intent status: experimental
Delivery status: experimental
Authoritative inputs: owner lane charter for affprec, 2026-11-05; `affected-plan-v0.md` (AFP-V0-012, AFP-V0-021, AFP-V0-036).

## Agent digest
- Claim: Affected selection bounds declared source scanners and keeps display-text provenance separate from executable path reads.
- Status: experimental; owner-directed lane implementation, pending independent review.
- Exists: source files, existing reader and dependency contracts, explicit file-kind authority.
- Blocked on: independent review and host verification of retained uncertainty.
- Read next: Requirements, Failure modes and Acceptance evidence below.

## Problem and current state

The six lane tickets report broad UNKNOWN selections on shell-only, test-only
and declaration/spec edits. `internal/tasks/boundary_test.go` walks from a parent
root but skips hidden directories and parses only Go source. Its unbounded lexical
reason loses that file-kind distinction. `internal/tasks/cli/command_help.go`
contains parent-path examples used only as help text; its source literals make
importers root-locator dependents. These facts do not bound unrelated dynamic
readers, which remain selected. The measurable job is fewer false selections with
each same-path true reader retained, without claiming UNKNOWN evidence is absent.

Owner direction: V1-4497, V1-4036, V1-3992, V1-4143, V1-4453 and V1-4535,
2026-11-05 lane charter. This contract is experimental. It does not change the
plan, graph-cache or declaration wire formats, or the independent CI selector.

## Requirements

- `AFP-V0-9000`: A Go test file may declare `//corvint:read-kinds .go go.mod`
  as a standalone line comment before its package clause. This is project-owned
  authority that this file and its test helpers observe repository files only
  by Go source contents and module-boundary existence, including directory
  traversal that ignores other file kinds. Temporary fixtures are outside that
  repository domain. The observer MUST replace only this file's lexical
  root-locator/parent-climb reasons with positive `*.go` and `go.mod` reader
  globs plus reader dependencies on regular in-repository targets of `.go`
  aliases, regardless of target suffix, directory or module. Directory listings
  and alias resolutions MUST participate in graph-cache invalidation. An
  unresolved, external or non-regular target, an unreadable listing, or exhausted
  traversal/dependency bounds MUST retain the file's lexical unbounded reasons.
  Ordinary literals, embeds, imports, execution edges, tokenization
  failures and other files' unbounded reasons MUST remain. A malformed,
  duplicate, misplaced or non-test declaration MUST NOT narrow reads. Adding
  a direct read of any other kind requires removing the declaration in the same edit.
- `AFP-V0-9001`: The Tasks import-direction, know-how authority and error-rewrap scans SHALL use AFP-V0-9000's
  kind declaration: their directory skip rules exclude hidden state and their
  content reads open only Go source. An edit of `.corvint/` JSON alone MUST NOT
  select it unless a scanned source alias targets that JSON; a Go source,
  source-alias target or module-boundary edit MUST still select it. This
  is a selection bound, not a confinement grant or a change to import rules.
- `AFP-V0-9002`: Parent-path examples in Tasks command help SHALL be embedded
  text resources, preserving their exact emitted bytes. The source observer
  MUST retain the embed dependency while those display strings no longer
  supply executable Go parent-climb literals. A new importer MUST NOT become
  unbounded solely because it links the command-help examples. Other real
  root-locating code in Tasks SHALL retain its uncertainty.
- `AFP-V0-9003`: Replays of script-only, test-only and data/spec changes MUST
  report selected counts, every retained frontier and its causal path or
  missing declaration. Each precision regression MUST retain a true reader
  of the same changed path. No extension-wide exemption for arbitrary
  unbounded readers is permitted: a test may read another test as data and
  an executable script may consume JSON or specifications.
- `AFP-V0-9004`: A dirty shell foreign-caller declaration MUST retain
  `shell:foreign-callers-changed`; it cannot authorize its own completeness.
  For an isolated measurement, use `corvint affected --worktree --path script/heavy.sh`
  (or repeated `--path` for the measured paths), without `--base`.
  The existing `worktree-path-plan` degradation makes the mutable input explicit. This observes the current declaration but explicitly excludes
  its edit from the requested change set. It is a conditional measurement,
  never verification of the full dirty change. The full dirty/base plan and
  invalid/stale declaration frontiers MUST remain visible. No scratch commit
  is needed for this conditional comparison.

- `AFP-V0-9005`: A Go test file may instead declare
  `//corvint:read-kinds fixtures` before its package clause. This is project-owned
  authority that this file and its helpers read repository-like paths only in
  disposable fixtures, never in the checkout being selected. Only that file's
  lexical unbounded reasons are removed; all ordinary literals and dependency
  edges remain. The declaration MUST NOT suppress another file's unbounded
  reasons, a non-test root locator or a parse failure. `internal/liveverify/mutate/export_test.go`
  SHALL use it for its fixture exports and negative symlink-target table. A
  true reader of the identical parent path without this declaration MUST stay
  selected. Adding a real checkout read requires removing this declaration.

## Non-goals and failure modes

Non-goals: automatic whole-program read analysis, narrower treatment of arbitrary
root locators, confinement-policy changes, declaration self-approval, or a bounded
claim when completeness is unknown. Failure mode: a stale project declaration
can understate a reader; review of the declared file and paired true-reader
regressions is required. The simpler baseline retains all unbounded readers.
Rollback removes the declaration and restores inline help; no migration or wire
version change is needed. Promotion requires the package tests and orchestration's
independent review; historical replay claims require the original Git objects.

## Compatibility and maintenance

The declaration's positive glob edges use existing Unit members and therefore
participate in graph digests, cache restoration and immutable-source observation.
No declaration, plan or cache JSON member changes. Older observers ignore the
comment and conservatively keep the scanner unbounded. CI's independent selector
is unchanged and may select more. Embedded help remains part of the binary and
is tested byte-for-byte by existing CLI help tests. New reads in a declared file
require re-review; no declaration authorizes a weaker test assertion.

## Acceptance evidence and traceability

For V1-4497's heavy-only replay, the historical below-86 bound measures language
units, before the AZU-V0-001 Make/CI claimant was integrated. Keep the full
`languages.All()` graph and report total, language, Make target and CI job counts
separately. Additional required drivers do not consume that historical language
budget: every runnable driver naming `script/heavy.sh` and every runnable computed-invocation site
must remain selected. Retain all frontiers and the same-path true-reader controls.
This reconciles the measurement domain under the owner's 2026-11-06 self-review
repair direction; it changes no selector behavior or wire contract.

| Requirement | Implementation | Evidence |
| --- | --- | --- |
| AFP-V0-9000 | `testReadKinds`, `readKindAliases`, `Language.observeDirectory` | `TestReadKindsDeclaration_AFPV09000`, `TestReadKindsUnresolvedAlias_AFPV09000` in `internal/liveverify/affected/golang/readkinds_test.go`; `TestBoundaryAliasCache_AFPV09000` in `internal/liveverify/affected/affprec_repo_test.go` |
| AFP-V0-9001 | `internal/tasks/boundary_test.go` | `TestBoundaryKindBound_AFPV09001`, `TestBoundarySymlinkTarget_AFPV09001` in `internal/liveverify/affected/affprec_repo_test.go` |
| AFP-V0-9002 | `internal/tasks/cli/command_help.go`, embedded usage text | `TestHelpTextProvenance_AFPV09002` in `internal/liveverify/affected/affprec_repo_test.go` |
| AFP-V0-9003 | reader selection and retained uncertainty | `TestPrecisionReplay_AFPV09003`, `TestTestOnlyImporterPrecision_AFPV09003` in `internal/liveverify/affected/affprec_repo_test.go` |
| AFP-V0-9004 | unchanged dirty-declaration frontier, conditional path measurement | `TestDirtyForeignCallerDeclaration_AFPV09004` in `internal/liveverify/affected/affprec_repo_test.go` |
| AFP-V0-9005 | `testReadKinds`, `internal/liveverify/mutate/export_test.go` | `TestFixtureReadKinds_AFPV09005` in `internal/liveverify/affected/golang/readkinds_test.go` |
