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

- **AFP-V0-9200:** (accepted by the owner, 2026-11-07; V1-5089 lane charter) Additional
  repository test read scopes MUST be supported by inspection of test literals, fixtures,
  repository walks, Git access, nested builds and subprocess reads, followed by a complete
  package run through the protected AFP-V0-023 Landlock wrapper on Linux. A candidate is
  retained only when that run passes and its scope excludes `.corvint/changes/`,
  `docs/build-log/`, and unrelated `docs/specs/` and `script/` paths. A denied read that is
  tolerated or converted to a skip is not qualification. Tests requiring repository-wide
  enumeration or Git metadata remain undeclared; missing evidence never justifies a scope.
  This declaration-only slice changes neither planner logic, test assertions, nor wire contracts.
  Acceptance records package outcomes and exclusions, source/build identities, and before/after
  Go package and test-file counts by witness kind for the V1-5000 three-path example and its
  pinned 20-merge replay. Historical replay retains each historical declaration; applying new
  scopes to historical tests without confined qualification is not acceptance evidence.
  Evidence: whole-package confined runs and `tools/test-read-scopes-check`, recorded in
  `docs/build-log/2026-11-07-rsdecl.md`. Rollback removes the added scope entries, restoring
  conservative reader selection without changing tests or the confinement wrapper.

### V1-5089 exact-change completion

- `AFP-V0-9201`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5089)
  Inventory the undeclared Go packages selected at the lane's starting HEAD for exactly
  `.corvint/agent-grep-exclude`, `docs/specs/lane-check-tooling-v0.md`, and
  `script/long-lines-check_test.sh`. Record witness kind and test-file count, descending
  by count. The earlier CEM/build-log/sessiontest example is not this inventory.
- `AFP-V0-9202`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5089)
  Preserve rsdecl's five qualified scopes and its settled exclusions. Revisit an
  exclusion only when the exact-change inventory supplies a different reason;
  retain the original evidence and explain the new qualification.
- `AFP-V0-9203`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5089)
  Inspect each new candidate's fixtures, file reads, enumeration, Git operations,
  subprocesses and nested builds. Declare only the observed repository read domain;
  temporary repositories and embedded data are not live checkout reads.
- `AFP-V0-9204`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5089)
  Each retained candidate MUST pass its unchanged whole package under the protected
  CI Landlock wrapper on Linux with the candidate scope. Never split the package or
  change a test to obtain qualification. Inspect skips: a denied read tolerated or
  converted into a skip is not qualifying evidence.
- `AFP-V0-9205`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5089)
  Scopes MUST exclude `.corvint/changes/`, `docs/build-log/`, and unrelated
  `docs/specs/` and `script/` paths. Repository-wide readers, unclear reads and failed
  candidates remain undeclared with explicit reasons. A planner defect exposed by
  a declaration stops this slice; it does not authorize a planner repair.
- `AFP-V0-9206`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5089)
  Add passing scopes with sorted package keys and entries, preserving the existing
  profile and entries. The declaration checker and its existing whole-package tests
  MUST pass. No test assertion, golden or wire contract changes are admitted.
- `AFP-V0-9207`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5089)
  Measure BASE and final declaration-tree binaries on the identical three paths from
  AFP-V0-9201. Report Go packages and test files by witness kind before and after;
  distinguish the merge-base baseline from the inherited rsdecl starting point.
  A path-list measurement is conditional selection evidence, not a full-diff gate.
- `AFP-V0-9208`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5089)
  Append the inventory, qualifying runs, undeclared remainder, comparison and limits
  to the rsdecl build log's rsdecl2 section. Retain raw plans and test logs outside
  the repository, with committed digests and derived counts. Independent review and
  host verification remain separate from local confinement and structural CEM checks.
- `AFP-V0-9209`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5089)
  Rollback removes only rsdecl2's added scope entries, restoring conservative
  selection without changing tests, the wrapper or planner. This is declaration
  qualification, not a new command, wire format or claim of platform-wide coverage.

Acceptance for AFP-V0-9201 through AFP-V0-9209 is the exact-path inventory and
paired plans, unchanged confined package runs, declaration checker suite, and
explicit exclusions retained in `docs/build-log/2026-11-07-rsdecl.md` (rsdecl2).

### V1-5165 over-selection diagnostic

- `AFP-V0-9510`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5165)
  Every `corvint-pr-narrowing/0` record MUST carry `overSelected`, the recorded
  selection's packages that the shadow plan did not select and that were not reused
  (at most 50, sorted), and `overSelectedCount`, their full count. Both are computed
  only when the driver and shadow selections were read and the selection within the universe was computed (`selectedOk`); otherwise the list is empty
  and the count is `null`. The shard summary line names the count and up to five
  packages. The fields are diagnostics only: they never change the mode, reasons,
  placement or executed set, and an over-selection is not a narrowing failure.
  Rollback deletes the two fields and the summary clause. Acceptance is
  `TestAFPV0098SupersetGuard` and `TestAFPV0098SupersetGuardSidecar` in `tools/ci-forward-row/narrowing_test.go`.

### V1-5201 literals joined to a temporary root (lane affprec2)

Audit 2026-11-09 (`origin/main` `8db854618`, selection-baseline narrow 35/43, median cost 0.818): a
Go path literal selects its unit for any dirty path it prefixes (AFP-V0-012 rule (c), AFP-V0-2203
narrows placement only for CEM and record directories), so test fixtures that write
`filepath.Join(t.TempDir(), "docs", ...)` make hub packages readers of every `docs/` change. At
`32140bb3b` the proof below applies to 3164 literal arguments in 689 files of 173 packages.

- `AFP-V0-9810`: (experimental under the owner's standing lane auto-accept; lane affprec2
  charter 2026-11-09; V1-5201) The Go plugin SHALL NOT count a string literal as a path token
  (AFP-V0-021) when it is an argument, other than the first, of a `filepath.Join` or `path.Join`
  call whose first argument is temp-rooted, and every such argument of that call is a plain
  literal (no empty, `.`, `..` or glob component). An expression is temp-rooted when it is
  `X.TempDir()` with `X` a function parameter of type `*testing.T`, `*testing.B`, `*testing.F` or
  `testing.TB` that the function never reassigns or addresses, such a join, or a
  local variable bound exactly once (`:=`, or a one-name `var` with one value) to a temp-rooted
  expression and never reassigned, addressed or declared at package level, through chains of such
  bindings within the file. The joined path lies below a directory the test run created empty,
  so the literal names no repository read whatever consumes the joined value. `os.TempDir()` is
  not temp-rooted: it is the shared directory `TMPDIR` names, which may be or hold a checkout. The
  proof is per file: a literal reaching a join through a helper function, a parameter, a map or
  a slice stays a path token; so does the same text spelled anywhere else in the file, a join
  whose first argument is any other expression (`os.Getwd`, `"."`, a constant), a receiver whose
  type the file does not declare as a testing parameter, a shadowed `filepath`, and every literal
  of a file that does not parse. Failure mode: a test that symlinks or copies a repository path
  under the temporary root still reads the repository through the literal that names that
  repository path, which this rule leaves untouched; the per-file bound and the exact receiver
  types keep a helper-built or type-shadowed root a reader. Rollback deletes `tempjoin.go` and
  its call in `pathTokens`; graph digests then change back and no wire member moves. Acceptance:
  `TestTempJoinComponents_AFP_V0_9810`, `TestTempJoinComponentsNeedsImports_AFP_V0_9810` and
  `TestTempJoinedLiteralSelectsNoReader_AFP_V0_9810` in `internal/liveverify/affected/golang`,
  and the docs-only probe in `docs/build-log/2026-11-09-affprec2.md`, which records that the
  repository's hub packages stay readers of a `docs/specs/` edit through fixture map keys written
  by helpers, a shape this per-file proof does not bind and leaves selected.

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
| AFP-V0-9000 | `testReadKinds`, `readKindAliases`, `Language.observeDirectory` | `TestReadKindsDeclaration_AFPV09000`, `TestReadKindsUnresolvedAlias_AFPV09000`, `TestReadKindsDependencyOverflow_AFPV09000` in `internal/liveverify/affected/golang/readkinds_test.go`; `TestBoundaryAliasCache_AFPV09000` in `internal/liveverify/affected/affprec_repo_test.go` |
| AFP-V0-9001 | `internal/tasks/boundary_test.go` | `TestBoundaryKindBound_AFPV09001`, `TestBoundarySymlinkTarget_AFPV09001` in `internal/liveverify/affected/affprec_repo_test.go` |
| AFP-V0-9002 | `internal/tasks/cli/command_help.go`, embedded usage text | `TestHelpTextProvenance_AFPV09002` in `internal/liveverify/affected/affprec_repo_test.go` |
| AFP-V0-9003 | reader selection and retained uncertainty | `TestPrecisionReplay_AFPV09003` in `internal/liveverify/affected/liverepo/precision_replay_repo_test.go`, `TestTestOnlyImporterPrecision_AFPV09003` in `internal/liveverify/affected/affprec_repo_test.go` |
| AFP-V0-9004 | unchanged dirty-declaration frontier, conditional path measurement | `TestDirtyForeignCallerDeclaration_AFPV09004` in `internal/liveverify/affected/affprec_repo_test.go` |
| AFP-V0-9005 | `testReadKinds`, `internal/liveverify/mutate/export_test.go` | `TestFixtureReadKinds_AFPV09005` in `internal/liveverify/affected/golang/readkinds_test.go` |
| AFP-V0-9200 | `.corvint/test-read-scopes.json` | Whole-package Landlock qualification and declaration-checker results in `docs/build-log/2026-11-07-rsdecl.md`; existing tests and assertions unchanged. |
| AFP-V0-9510 | `.github/forward-rows/narrow.jq` `overSelected`/`overSelectedCount`, `narrow.sh` summary clause | `TestAFPV0098SupersetGuard` (cap, unknown selection) and `TestAFPV0098SupersetGuardSidecar` (summary names five) in `tools/ci-forward-row/narrowing_test.go` |
| AFP-V0-9810 | `tempJoinComponents` in `internal/liveverify/affected/golang/tempjoin.go`, called from `pathTokens` | `TestTempJoinComponents_AFP_V0_9810`, `TestTempJoinComponentsNeedsImports_AFP_V0_9810`, `TestTempJoinedLiteralSelectsNoReader_AFP_V0_9810`; HEAD census and docs-only probe in `docs/build-log/2026-11-09-affprec2.md` |
