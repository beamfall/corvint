# Test Selection Precision V0

Owner: Russell Lewis
Date: 2026-10-30
Requirement prefix: `TSP-V0`
Intent status: experimental under the owner's standing lane auto-accept (2026-10-11; V1-3160); TSP-V0-001 amends AFP-V0-580
Delivery status: experimental
Authoritative inputs: native ticket V1-3160 (related V1-2763, V1-2896). These requirements narrow
when the [affected plan](affected-plan-v0.md) names a unit frontier; that spec's Requirements
section is at the 256-obligation OCM limit (`maxObligations`), so they live here.

## Agent digest
- Claim: build-constraint frontiers bear only on own files; declared gated locators and verified shell callers stop widening every plan.
- Status: experimental under the owner's standing lane auto-accept (2026-10-11; V1-3160); TSP-V0-001 amends AFP-V0-580.
- Exists: `FrontierBearsOnOwnFiles` in `internal/liveverify/affected/unit.go`; `ownFilesBear` in `internal/liveverify/affected/select.go`; `observeDirectory` in `internal/liveverify/affected/golang/golang.go`; `applyGatedLocators` in `internal/liveverify/affected/golang/gatedlocators.go`; `applyForeignCallers` in `internal/liveverify/affected/shell/callers.go`; tests named in the traceability table.
- Blocked on: nothing; Open questions names the residual shell frontier.
- Read next: Requirements; Failure modes; Open questions.

## Intent and scope

Intent (orchestrator lane request, 2026-10-30; V1-3160): the owner asked that every run select only
the minimum safe tests. At `fd51f53a2` every non-empty Corvint plan was `UNKNOWN`, and therefore
FULL in CI, because three global frontiers bore on it: `go:build-constraint-variants`,
`go:nested-module-frontier` and `shell:foreign-invocation-unmodelled`. Each frontier SHALL be scoped
to the changes it can actually bear on, and SHALL still make the plan `UNKNOWN` wherever it applies.

## Requirements

- **TSP-V0-001:** (experimental, amending AFP-V0-580, under the owner's standing lane auto-accept,
  2026-10-11; V1-3160) A unit MAY declare `frontierBears: OWN_FILES`. Its frontier SHALL then be
  named only on a plan with a dirty path whose directory is the unit's directory (the directory of
  its first source or test file) or an ancestor of it, whatever reached the unit. A unit with no source or test file
  SHALL bear as before. `OWN_FILES` SHALL apply in both the reached-unit and the narrowed-reader
  frontier loops of selection. The Go plugin SHALL declare `OWN_FILES` on every unit it gives
  `go:build-constraint-variants`. The model: a Go package's files all live in its directory, so a
  file is added, deleted, or gains or loses a constraint only by a dirty path there; the module
  and workspace manifests and the build configuration that choose the toolchain and tags (`go.mod`,
  `go.work`, `Makefile`) lie in an ancestor; every
  variant's imports are already graph edges, so a change elsewhere reaches every variant's
  dependents through the same union edges; and one test run compiles one host variant whichever
  packages it selects, so naming the frontier for a change outside the directory added
  unreached packages to the run without exercising any other variant.
- **TSP-V0-002:** (experimental, under the owner's standing lane auto-accept, 2026-10-11; V1-3160)
  A disk walk drops a symbolic link, though Go compiles a symlinked `.go` file's target, so a
  package's own files need not all be in its directory. The Go plugin SHALL therefore NOT declare
  `OWN_FILES` on a constrained package whose directory has a symlinked `.go` entry, or whose
  directory it cannot list; its frontier then bears whenever the package is reached, as before
  TSP-V0-001. `Source.LinkedEntries` SHALL list a directory's symlinked entries, record the
  directory unfollowed for the graph cache, and answer none on an immutable tree source, whose walk
  already refuses every non-regular entry.
- **TSP-V0-003:** (experimental, under the owner's standing lane auto-accept, 2026-10-11; the
  orchestrator's go:nested-module-frontier decision, 2026-10-30; V1-0872) The project MAY declare
  gated root locators in `.corvint/gated-root-locators.json`, the closed JSON object
  `{"profile": "corvint-gated-root-locators/0", "packages": {DIR: {"gate": ABS, "reason": TEXT}}}`
  (at most 64 KiB and 64 packages): DIR a clean relative package directory other than `.`, ABS a
  clean absolute path other than `/`, TEXT non-empty printable text of at most 512 bytes. A
  declared package's root-locating code SHALL run only after it admits a protected document read
  under the gate, a path outside every checkout, so a test that links it reads the repository only
  on a host where the protected document names that checkout. The Go plugin SHALL clear
  `LocatesRoot` on each declared package, which keeps its own unbounded-read reason and test
  selection, so its dependents (including a dependent nested module's unit) are no longer followed
  on every dirty path (AFP-V0-012 rule (d)) but only when a change reaches them. The declaration
  SHALL declare nothing, and every plan SHALL name `go:gated-root-locators-invalid`, when it is
  unreadable, oversized, not a regular file, has an unknown member or a wrong profile, or names a
  package that is not observed, does not locate the root, or whose non-test sources do not carry
  the gate as a Go string literal. A plan with the declaration dirty SHALL name
  `go:gated-root-locators-changed`, so it is `UNKNOWN`. Each declared package SHALL carry a test
  that proves the gate from its own non-test sources and fails when the gate or the declaration
  drifts; for `internal/authoritystore` it is `TestRootLocatorGated_TSP_V0_003`.
- **TSP-V0-004:** (experimental, under the owner's standing lane auto-accept, 2026-10-11; the
  orchestrator's V1-3325 decision, 2026-10-30) The shell plugin SHALL treat a `source` or `.` of
  `$NAME/REST` or `${NAME}/REST` as a read of a runtime file outside the repository, and not as a
  dynamic source, only when REST is literal, non-empty and has no `..` segment; NAME occurs in the
  file exactly once as an assignment and otherwise only as `$NAME` or `${NAME...}` expansions other
  than `${NAME=...}` and `${NAME:=...}`; that assignment is a command of its own whose value is one
  `$(mktemp ...)` substitution with `-d` or `--directory`, no `-u` or `--dry-run`, and no list,
  pipeline, redirection or backquote inside; it precedes the source in the same block (one function
  body, brace group or conditional arm, never a subshell or command substitution), does not follow
  `&&`, `||`, `|` or `&` and does not precede `|` or `&`; and no `source`, `.`, `load` or `eval`
  runs between them. Every other non-literal source SHALL stay `shell:dynamic-source`.
- **TSP-V0-005:** (experimental, under the owner's standing lane auto-accept, 2026-10-11; the
  orchestrator's option (c) decision, 2026-10-30) The project MAY declare, in
  `.corvint/shell-foreign-callers.json` (profile `corvint-shell-foreign-callers/0`), for a non-test
  shell script, every repository file that names it, split into sorted, unique, disjoint `callers`
  (files that run it) and `mentions` (files that only name it), with a non-empty reason. The shell
  plugin SHALL verify the declaration against the source universe: it holds only when every
  declared script is an observed single-file non-test shell unit and, for each script, the files
  other than the script among the shell sources, the planned languages' sources, Make files
  (`Makefile`, `makefile`, `GNUmakefile`, `*.mk`) and YAML files whose text contains the script's
  file name are exactly its callers and mentions, and every one of them is readable. A verified
  script SHALL carry its callers and SHALL NOT carry `shell:foreign-invocation-unmodelled`; each
  caller that is a test file of a unit SHALL add a test-user edge from the script to that unit, and
  each other owned caller SHALL make its unit a dependent of the script; a mention SHALL add no edge.
  A missing declaration SHALL change nothing; a present one that is unreadable, invalid or not
  exactly complete SHALL declare nothing and name `shell:foreign-callers-invalid`, so every script
  keeps the frontier. A plan with the declaration dirty SHALL name `shell:foreign-callers-changed`.
  The repository's declaration SHALL be checked by a test that fails on any tracked file naming a
  declared script, other than prose and data (`.md`, `.json`, `.tsv`), that is not listed.

## Non-goals

Evaluating build constraints for a host or running non-host variants; scoping the frontier to the
dirty file's own constraint (a plain file in the directory can be what a variant calls, which
AFP-V0-580's review withdrew a narrower scoping over); selecting a dependent nested module's tests
(AFP-V0-582); mirroring the gated declaration in `internal/gateselect`.

## Failure modes

(1) A non-host variant that calls code in another package can break when that package changes,
and the plan is now `BOUNDED`; a FULL run on the same host compiles the same host variant, so it
would not have caught it either; the cross-compiling checks (`make windows-vet`, the CI Windows vet
step) remain the evidence for that variant. (2) A dirty path in a constrained package's directory
or an ancestor still names the frontier, including a non-Go file, an added variant and a deleted one;
a root-level change (`README.md`) therefore keeps it for every constrained package it reaches.
Build configuration outside an ancestor (`.github/**`, which forces FULL in CI; a script under
`script/` that passes `-tags`) is not modelled; no repository script passes `-tags` today. (3) A plan
from an older planner, or a unit graph cached before this change, carries no `OWN_FILES` and bears
as before; the graph cache is bound to the program identity, so a new binary rebuilds it.
(4) TSP-V0-003: a gated package whose gate admits the checkout (an operator whose protected root
names this repository) runs its locating code in a dependent's test; the planner then misses the
read only if the dependent is not otherwise reached, the declared gate is unchanged and the
package's proof test still passes, which the test's call-order and constructor checks are there to
prevent. The proof is static over the package's own sources: a reflective or `unsafe` call path
it does not model is out of its reach. `internal/gateselect` (the CI PR tier) does not read the
declaration; it keeps the wider closure, which over-selects but never under-selects.
(5) TSP-V0-004: the rule is static over one file's text. A sourced runtime file that rebinds NAME
is excluded by refusing any source after the first; an `eval` of computed text naming NAME, a
nameref (`declare -n`) or an indirect `${!x}` assignment it cannot see would let the read escape,
and only a textual occurrence of NAME in another form refuses. `mktemp -d` that fails leaves NAME
empty, so the read is of an absolute path, still outside the repository.
(6) TSP-V0-005: the scan is textual. A caller that builds the script's file name from parts (no
file in the repository does today) is missed, so the declaration can verify while a caller is
unlisted; the scan cannot prove the absence of such a construction. The caller or mention split is
the project's own per-file claim: a file listed as a mention that does run the script loses its
edge, so its tests are not selected for a change to the script. A caller in a file outside the
candidate suffixes (a `.md` runbook an operator pastes from) is not a code path and is not
scanned. A declared script stays an unbounded reader when it is one, so its callers' tests are
selected on every plan; that is the declared price of the narrowing. Each graph build reads every
candidate file once more for the scan. Undeclared non-test shell scripts keep the frontier, so a
plan that reaches one still bears it.

## Open questions

These need an owner decision because no existing project-owned model bounds them; each frontier
stays as it was:

1. Resolved by TSP-V0-003 (the orchestrator chose option (a), 2026-10-30, rejecting unenforced
   read scopes for dependent-module keys and accepting AFP-V0-650's failure mode):
   `go:nested-module-frontier` bore on every plan because the `tools/local-authority` unit
   inherited root locating from `internal/authoritystore`. Its tests still run only in the job
   V1-3329 adds.
2. Resolved by TSP-V0-004 and TSP-V0-005 (the orchestrator chose option (c), 2026-10-30):
   `shell:foreign-invocation-unmodelled` bore on every plan through `script/heavy.sh`, an unbounded
   reader followed on every dirty path. Its callers, and those of the scripts that run it, are now a
   verified declaration. The frontier still bears through the undeclared non-test shell scripts
   that read a dirty path by a literal path (the `docs/specs/**` checkers and the CEM scripts under
   `script/`); declaring them would make many Go packages that only name them dependents.

## Acceptance evidence and traceability

The tests below, failing on base `fd51f53a2`; the measured plans in build log
`docs/build-log/2026-10-30-v1-3160-frontier-scoping.md`.

| Requirement | Implementation | Evidence |
|---|---|---|
| TSP-V0-001 | `FrontierBearsOnOwnFiles` and `validFrontierBears` in `internal/liveverify/affected/unit.go`; `ownFilesBear` and its two call sites in `internal/liveverify/affected/select.go`; `observeDirectory` in `internal/liveverify/affected/golang/golang.go` | `TestBuildConstraintFrontierBearsOnOwnFiles_TSP_V0_001` (a dirty plain file, variant, absent variant or non-Go file in the directory, alone or with another path, and a root or ancestor file the package reads, bears and stays `UNKNOWN`; a dirty package the variant imports, a named data path and an unrelated package do not, the variant's tests stay selected, and the imported package's plan is `BOUNDED`), the narrowing, V1-0289 and V1-1818 cases in `internal/liveverify/affected/golang/golang_test.go` |
| TSP-V0-002 | `LinkedEntries` in `internal/liveverify/affected/source.go`; `observeDirectory` in `internal/liveverify/affected/golang/golang.go` | `TestBuildConstraintFrontierSymlinkedSource_TSP_V0_002` (a constrained package reached through an imported package, with a dirty symlink target outside its directory: the frontier bears and the plan is `UNKNOWN` with the link, and is not named without it) in `internal/liveverify/affected/golang/golang_test.go` |
| TSP-V0-003 | `readGatedLocators`, `applyGatedLocators` and `carriesLiteral` in `internal/liveverify/affected/golang/gatedlocators.go`; their call in `internal/liveverify/affected/golang/golang.go`; `gatedLocatorsChanged` in `internal/liveverify/affected/readers.go` and its call in `internal/liveverify/affected/select.go`; `.corvint/gated-root-locators.json` | `TestGatedRootLocators_TSP_V0_003` in `internal/liveverify/affected/golang/gatedlocators_test.go` (undeclared: the dependent is selected on an unrelated change; declared: it is not and the plan is `BOUNDED`, the locator's own tests stay selected and a change to the locator still selects it; a gate not carried, a package that is not a locator or not present, a relative gate, an unknown member and unparsed JSON each name the invalid frontier and keep it selected; a dirty declaration is `UNKNOWN`); `TestRootLocatorGated_TSP_V0_003` in `internal/authoritystore/gated_locator_test.go` |
| TSP-V0-004 | `freshDirectorySource`, `makesDirectory` and `onlyExpanded` in `internal/liveverify/affected/shell/fresh.go`; block identity and separators in `parse` in `internal/liveverify/affected/shell/lex.go`; `plainAt` and the source case of `observe` in `internal/liveverify/affected/shell/shell.go` | `TestFreshTemporaryDirectorySourceIsBounded_TSP_V0_004` in `internal/liveverify/affected/shell/shell_test.go` (a heavy.sh-shaped `serve` is bounded; another value, a second assignment, a `for`, `read`, `local` or `unset` rebinding, `${w:=}`, a `local w=` declaration, another function, a subshell, an `if` arm, `&&`, a pipeline, an inner pipeline, a dynamic, `..` or empty rest, `mktemp` without `-d` or with `-u`, a prefix assignment, an `ENVFILE` source, a source between, a second fresh source, a source before the assignment and backquotes each stay dynamic) |
| TSP-V0-005 | `readForeignCallers`, `applyForeignCallers` and `foreignCallersComplete` in `internal/liveverify/affected/shell/callers.go` and their call in `internal/liveverify/affected/shell/shell.go`; `Unit.Callers` in `internal/liveverify/affected/unit.go`; `callerEdges` in `internal/liveverify/affected/graph.go`; `foreignCallersChanged` in `internal/liveverify/affected/readers.go`; `.corvint/shell-foreign-callers.json` | `TestForeignCallerDeclaration_TSP_V0_005` in `internal/liveverify/affected/shell/callers_test.go` (declared: the frontier is gone, the plan on an unrelated change is `BOUNDED` and selects the caller, test-file caller and upstream dependents' tests but not a mention's; undeclared keeps it bearing; an unlisted Go, shell, Make, Python or workflow caller or mention, a stale caller or mention, unsorted lists, a script that is no unit, an empty reason, a bad profile, an unknown member, unparsed JSON and a file both caller and mention each name the invalid frontier and keep it bearing; a dirty declaration is `UNKNOWN`); `TestRepositoryShellForeignCallersComplete_TSP_V0_005` in `internal/liveverify/affected/shell_callers_repo_test.go`; `TestBatchReplayDeclaredRuntimeReaders_V12941` frontier floor in `internal/liveverify/affected/replay_v12941_test.go` |

## Rollback

Remove `frontierBears = affected.FrontierBearsOnOwnFiles` in `observeDirectory`; the constant and
`ownFilesBear` are then inert. No stored state depends on the value. TSP-V0-002: drop the
`LinkedEntries` check in `observeDirectory` only together with TSP-V0-001. TSP-V0-003: delete
`.corvint/gated-root-locators.json` (a change that itself plans `UNKNOWN`); the package is then a
root locator again and its dependents are followed on every dirty path. TSP-V0-004: make
`freshDirectorySource` return false; every such source is then dynamic again, as before.
TSP-V0-005: delete `.corvint/shell-foreign-callers.json` (a change that itself plans `UNKNOWN`) or
make `applyForeignCallers` return at once; every script then carries the frontier again and the
`Callers` edges are empty.
