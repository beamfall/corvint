# Affected and CLI Consistency V0

Owner: Russell Lewis
Date: 2026-11-01
Requirement prefix: `ACX-V0`
Intent status: experimental under the owner's standing lane auto-accept (2026-10-11; lane aff-w12)
Delivery status: experimental
Authoritative inputs: native tickets V1-3583, V1-3327, V1-3578, V1-3726, V1-3727, V1-3045,
V1-2907, V1-3734, V1-3731 and V1-3736. These requirements refine the
[affected plan](affected-plan-v0.md) and the command-line surface around it; they live here to
keep that spec's Requirements section under the 256-obligation OCM limit (`maxObligations`).

## Agent digest
- Claim: Affected source walks, worktree path plans, test-file readers, revspec bases and the CLI flag, error and Makefile surface follow one contract.
- Status: experimental under the owner's standing lane auto-accept (2026-10-11; lane aff-w12).
- Exists: `Source.Files` and `ByName` in `internal/liveverify/affected/source.go`; `--worktree` in `cmd/corvint/affected.go`.
- Blocked on: nothing.
- Read next: Requirements; Failure modes; Acceptance evidence and traceability.

## Intent and scope

Intent (orchestrator lane request aff-w12, 2026-11-01): close ten friction tickets in
`corvint affected`, test selection and the CLI surface, each with the smallest complete slice that
meets its acceptance criteria. Affected user: an agent or engineer planning tests for a change or
driving Corvint from scripts.

## Requirements

- **ACX-V0-850:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3583)
  `Source.Files` and `Source.FilesIncluding` SHALL call `accept` with each walked file's
  repository-relative slash path and its base name, on a supplied filesystem, a disk source and a
  context-scoped disk source alike, so a directory-scoped rule such as the Gradle default version
  catalog `gradle/libs.versions.toml` matches. `ByName` SHALL adapt a name-only predicate, and
  every existing name-only caller SHALL keep its selection unchanged.
- **ACX-V0-851:** (experimental, amending AFP-V0-094, under the owner's standing lane
  auto-accept, 2026-10-11; V1-3327) `corvint affected --worktree` with `--path` or `--paths-from`
  SHALL plan the named paths against the working tree, dirty paths and ignored files included,
  instead of refusing `unsupported-affected-mixed-worktree`. Its receipt SHALL carry the
  degradation `worktree-path-plan` and a warning that names HEAD and the dirty count and says the
  plan is worktree-derived and is not pinned evidence. `--worktree` without `--path` or
  `--paths-from`, or with `--require-clean`, `--snapshot` or `--base`, SHALL be an argument
  error. Without `--worktree` the AFP-V0-094 refusal SHALL stay the default.
- **ACX-V0-852:** (experimental, narrowing AFP-V0-021, under the owner's standing lane
  auto-accept, 2026-10-11; V1-3578) For a dirty `_test.go` path that a Go unit owns, another Go
  unit SHALL be a `PATH_LITERAL_READER` only when one of its path tokens names the file itself:
  a component run of the token that matches the path's file name, whole or as the run's
  trailing fragment. A token that names only an ancestor directory of the path SHALL NOT select
  it: Go compiles a `_test.go` file only into its own package's test binary, so a directory
  literal of another package is no read of it. A non-Go unit, a declared read scope
  (AFP-V0-023), a glob reader (AFP-V0-401) and every other dirty path keep their rules.
- **ACX-V0-853:** (experimental, amending AFP-V0-496, under the owner's standing lane
  auto-accept, 2026-10-11; V1-3726) `corvint affected` SHALL advise the focused
  `corvint test --base BASE --dry-run` command only when `.github/cishards/test-commands.json`
  is a regular file at the root, because without it that command falls back to a FULL scope with
  no units. Otherwise the advice SHALL omit it and SHALL carry the unknown entry
  `FOCUSED_TEST_COMMAND_UNDECLARED`, which names the missing declaration and points to the
  declared mandatory checks (the Makefile gate or the AGENTS.md Verify commands) as the next step.
- **ACX-V0-854:** (experimental, amending AFP-V0-230, under the owner's standing lane
  auto-accept, 2026-10-11; V1-3727) `corvint affected --base` SHALL also accept a revision such
  as `HEAD~1` or `origin/main`: a value of 1 to 256 printable non-space ASCII characters that is
  not all hex, does not start with `-` or `:` and holds no `..`. It SHALL be resolved once with
  `git rev-parse --verify --quiet --end-of-options VALUE^{commit}` and replaced by the full
  commit id before planning, so `range.base` stays pinned. An all-hex value keeps the AFP-V0-230
  rules. Every value that does not resolve to one commit SHALL be refused with
  `invalid-arguments` carrying the same hint, `git rev-parse --verify 'VALUE^{commit}'`; an
  ambiguous prefix keeps `ambiguous-affected-base` (AFP-V0-232).
- **ACX-V0-855:** (experimental, under the owner's standing lane auto-accept, 2026-10-11;
  V1-3045) When two Python files share one dotted name, as `pkg.py` and `pkg/__init__.py` do,
  the Python plugin SHALL keep both files in the one unit `python:pkg`, with the imports of both,
  so an edit to either has that owner and selects its importers. Neither file SHALL be dropped
  from the graph and become an unowned dirty path.

## Non-goals

Pinning a worktree-derived plan as evidence; a repository-wide flag rename.

## Failure modes

(1) A caller that still compares the first `accept` argument with a base name would match nothing;
the signature change makes every such caller a compile error. (2) A `--worktree` plan can be
read as evidence by a consumer that ignores `degradations`; the warning and the degradation name
it. (3) A Go test that walks a directory holding another package's `_test.go` files and reads
them as data is no longer selected by that directory literal alone on a test-only edit; it is
selected when it declares the read in `.corvint/test-read-scopes.json` (AFP-V0-023), and full CI
still runs it.

## Acceptance evidence and traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| ACX-V0-850 | `Source.Files`, `Source.FilesIncluding` and `ByName` in `internal/liveverify/affected/source.go`; the catalog filters in `internal/liveverify/affected/kotlin/kotlin.go` | `TestSourceFilesOffersRelativePath_V1_3583`; the plugin package tests |
| ACX-V0-851 | `Worktree` in `parseAffectedOptions` and `compileAffectedGraph`, `affectedWorktreeWarning` in `cmd/corvint/affected.go` | `TestACXV0851WorktreePathPlan` |
| ACX-V0-852 | `foreignTestFile` and `namesFileName` in `internal/liveverify/affected/readers.go` | `TestTestFileEditSelectsOnlyFileNamingReaders_ACX_V0_852`; the V1-3578 repro counts in `docs/build-log/2026-11-01-aff-w12.md` |
| ACX-V0-854 | `setAffectedBase` in `cmd/corvint/affected.go`, `resolveAffectedAbbrevBase` and `affectedBaseHint` in `cmd/corvint/affected_abbrev_base.go` | `TestACXV0854RevisionBase`; `TestAFPV0230AbbreviatedBaseAndTrailingRoot` |
| ACX-V0-855 | `UnitsSource` in `internal/liveverify/affected/python/python.go` | `TestModuleAndPackageInitShareOneOwner_ACX_V0_855` |
| ACX-V0-853 | `withFocusedNextStep` in `cmd/corvint/affected.go` | `TestAffectedAdviceFocusedNextStep_AFPV0496`; `TestAffectedAdviceOmitsFocusedCommandWithoutDeclaration_ACXV0853` |

## Rollback

Revert the commits naming these requirements; each is local to its ticket's files and no stored
state depends on them.
