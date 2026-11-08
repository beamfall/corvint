# Affected Plan Selection Precision V0

Owner: Russell Lewis
Date: 2026-11-01
Requirement prefix: `AFP-V0`
Intent status: per requirement, each experimental under the owner's standing lane auto-accept (2026-10-11)
Delivery status: experimental

## Agent digest
- Claim: Affected plans name timeouts and deletions, narrow fixture and go-list reads, retain runtime-rooted Go readers, match CI, and honour owners and must-run tests.
- Status: per requirement, each experimental under the owner's standing lane auto-accept (2026-10-11); delivery experimental.
- Exists: `fixtureOnlyClaim` in `internal/liveverify/affected/select.go`, `deadlineCode` in `internal/liveverify/affected/dirty.go`, `withWholePackages` in `cmd/corvint/affected_whole.go`, `fixtureTokens` in `internal/liveverify/affected/golang/golang.go`, the measured rounds of `TestIncrementalSelectionMeetsTheLiveBudget`, `TestScanImportsAgreesWithAst_V13322`; `goListScanFile` and `ImportSetRecords` in `internal/liveverify/affected/golang`, `importGraphReadersOf` in `internal/liveverify/affected/importgraph.go`, `resolves`, `namesAny` and `anchoredWithin` in `readers.go`, `tempRoots` and `ExtensionlessScript` in `shell/shell.go`, `affectedDeletedPaths` in `cmd/corvint/affected.go`; `parentBackslash` in `internal/gateselect/readers.go`; `readTestSelection`, `ownersOf` and `mustRunOf` in `internal/liveverify/affected/testselection.go`, `affectedFailFastOrder` in `cmd/corvint/affected_order.go`; `droppedDeclarations` in `tools/unbounded-readers` and `droppedUnrecorded` in `tools/test-read-scopes-check`; `leafPattern` in `internal/liveverify/affected/golang/unbounded.go` and `internal/gateselect/readers.go`; `internal/liveverify/affected/golang/typedreach`, `testTypedSlices` in `cmd/corvint/test_typed_slices.go`, `tools/typed-reach-measure`; `boundedJoinConsumed` in `internal/liveverify/affected/golang/locatorproof.go` and `internal/gateselect/locatorproof.go`, `driverGoBuildOnlyCommands`, `driverXargs` and `scanShellOutput` in `internal/liveverify/affected/driver_shell.go`, `RequireLiveTier` in `internal/testsupport/livetier.go`, `LiveTests` in `.github/cishards/live.go`, `Size` in `.github/cishards/size.go` and `.github/forward-rows/github-guard.sh`.
- Blocked on: the parent spec's gates.
- Read next: Requirements; the parent spec's Non-goals and authority and Failure modes.

Authoritative inputs: `docs/specs/affected-plan-v0.md` (parent contract: wire, authority,
failure codes and acceptance gates), `docs/specs/affected-plan-python-playwright-v0.md` (the
Python adapter), `docs/specs/corvint-self-development-v0.md` (ASD-V0-895, the
`//corvint:fixture-path` marker), `internal/liveverify/affected`, `AGENTS.md` invariants 2, 4
and 8. The parent's Requirements section is at the 256-obligation OCM limit, so lane aff-w26's
requirements live here under the parent's `AFP-V0` prefix; see build-log entry
`2026-11-01-aff-w26`. Lane selprec (2026-11-03; native tickets V1-4409, V1-4362, V1-4173,
V1-4278, V1-3864) adds AFP-V0-2200 to AFP-V0-2206 here for the same reason; see build-log entry
`2026-11-03-selprec-selection-precision`. Lane selprec2 (2026-11-03; native tickets V1-4437,
V1-4173, V1-4284) adds AFP-V0-2240 to AFP-V0-2244, and its third round (V1-4173, V1-3282)
AFP-V0-2245 to AFP-V0-2247, of which its fourth round withdrew AFP-V0-2245 and its fifth round
withdrew AFP-V0-2246; see build-log
entries `2026-11-03-selprec2-ci-selector-parity`, `2026-11-03-selprec2-round3-typescript-anchors`
and `2026-11-03-selprec2-round4-withdraw-typescript-anchors`, with round 5 recorded in
`2026-11-03-selprec2-round5-withdraw-shell-canonicalization` and round 6, which amends
AFP-V0-2203, AFP-V0-2240, AFP-V0-2242 and AFP-V0-2243 in place, in
`2026-11-04-selprec2-round6-runtime-roots`. Round 7 amends AFP-V0-2203, AFP-V0-2204, AFP-V0-2240
and AFP-V0-2243 in place; see `2026-11-04-selprec2-round7-file-names-and-redirections`. Round 8
amends AFP-V0-2203 and AFP-V0-2240 to AFP-V0-2242 in place: no Go path literal is narrowed by
placement any more, in the planner or the CI selector; see
`2026-11-04-selprec2-round8-runtime-roots-retained`. Round 9 amends AFP-V0-2241 and AFP-V0-2242 in
place, so a lexicon rewrite only adds tokens beside the written ones, and disables AFP-V0-2243 and
AFP-V0-2244 for 1.0 under the owner-approved last-round cap; see
`2026-11-04-selprec2-round9-additive-lexicon`.

Lane selmark (2026-11-03; native tickets V1-2662,
V1-1184, V1-1189, V1-2277) adds AFP-V0-2480 to AFP-V0-2489 here for the same reason; see
build-log entry `2026-11-03-selmark-owned-selection`. V1-1184 and V1-1189 name the e2e-safe
profile's AFU-V1-019 to AFU-V1-023; their must-run and order rules apply to the unit plan every
consumer reads, so they live here, and AFP-V0-2486 carries must-run into that profile.
Lane selbugs (2026-11-03; V1-3489, V1-4363, V1-3480,
V1-4281) adds AFP-V0-2400 to AFP-V0-2406 here for the same reason; see build-log entry
`2026-11-03-selbugs-selection-correctness`.
Lane
fr-scripts (2026-11-04; V1-4532, V1-4512) adds AFP-V0-2540 to AFP-V0-2542 here for the same reason.
Lane gcr-w20 (2026-11-01; V1-3912) adds AFP-V0-920
here for the same reason; see build-log entry `2026-11-01-gcr-w20`.
Lane selaudit (2026-11-06; the selection audit, build-log entry `2026-11-06-selection-audit`) adds
AFP-V0-2610 here for the same reason. Lane selgo (2026-11-06; native tickets V1-4931, V1-4932,
V1-4933, build-log entry `2026-11-06-selgo`) adds AFP-V0-2740 to AFP-V0-2750 here for the same
reason, including the rows that amend the CI selector (CIN-V0-860) and `corvint test` (STR-V0-064);
its review fix rounds add AFP-V0-2746 to AFP-V0-2750.
Lane ci-speed (2026-11-06; V1-4815) adds AFP-V0-2700 to AFP-V0-2702 here for the same reason; see
build-log entry `2026-11-06-ci-speed`. Lane selgh (2026-11-06; V1-4930, V1-4934) adds AFP-V0-2720
to AFP-V0-2726 here for the same reason; see build-log entry `2026-11-06-selgh`.
Lane floor (2026-11-03; V1-0643, V1-2744) adds
AFP-V0-2360 to AFP-V0-2363 and AFP-V0-2370 to AFP-V0-2378 here for the same reason; see build-log
entry `2026-11-03-floor-typed-reach`.
Lane selmin (2026-11-07; V1-5029, V1-5026, V1-5030) adds AFP-V0-9009 to AFP-V0-9011
here for the same reason; see build-log entry `2026-11-07-selmin`.

## User and current state

The affected user and the plan wire are the parent's: an engineer or agent that wants the tests a
change plausibly needs before the mandatory gate. These requirements tighten how a plan reports a
Git deadline, a TestMain change and a fixture-only path literal, and how two existing checks
measure. None of them drops a unit the change can reach.

## Requirements

### Located runner locality (V1-4679)

- **AFP-V0-2600:** (accepted under the owner's standing lane auto-accept (2026-10-11); V1-4679) Amending AFP-V0-511 (4): for a plugin that owns no dirty source, has no reached
  unit, reads no arbitrary path and has no changed resolution input, a located
  `AncestorBoundFrontiers` entry MAY be omitted when every unowned path is provably outside
  its hidden runner/build edges. This optional plugin declaration MUST name only fixture-closed reasons whose hidden edges
  bind the plugin's own sources or ancestor configuration, not arbitrary runner assets or
  runtime reads. Build MUST validate the namespace/category grammar and fixture-closed subset;
  malformed declarations fail with `ErrInvalidLanguage`. Without the declaration the existing
  conservative behavior applies. TypeScript declares only unknown test runners,
  whose classification inspects source imports and ancestor manifests/configs. The declaration
  classifies plugin code, is reconstructed on cache reads and does not change the graph digest.
  The site MUST be a source or test path actually owned by
  that plugin; bare, summarized (`+N`) and unresolved sites stay unknown. Each unowned path
  MUST independently satisfy either the existing Go-hosted fixture rule, or be outside the
  site's directory and every ancestor directory, with no modelled read/dependency edge
  reaching the plugin. The latter proof applies only to paths outside `testdata`; unhosted
  fixtures retain their existing uncertainty. A build or CI driver input MUST NOT use this
  proof, even when it is outside the site's ancestors: it can drive every job. Resolution
  inputs, path-open reasons (including arbitrary dynamic loads), reached readers and
  dependents retain the existing frontier rules. One unrelated path MUST NOT invalidate
  another path's fixture proof when each independently meets these conditions.
  `UNOWNED_DIRTY_PATH`, selections, wire members and graph digest are unchanged. A declared
  non-source output remains unowned for this rule, but its plan can be `BOUNDED` when no
  remaining uncertainty bears; missing evidence always retains `UNKNOWN`. Non-goals: closing
  arbitrary dynamic loads, narrowing selected tests, or treating exclusions as authority.
  Acceptance: regressions for sibling data, mixed fixture/data and declared output, plus
  fail-closed controls for CI drivers, ancestor configs, unknown sites, resolution inputs
  and real readers/dependents. Rollback: remove the located-runner proof, restoring AFP-V0-511's
  conservative cascade.

- **AFP-V0-548:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-3414)
  When a `DirtyPaths` Git read ends because the read's own deadline elapsed, the returned error
  MUST carry `cemcode.GitTimeout`, not `cemcode.GitCancelled` or a bare context error. Git's runner
  sees only a done context, so `deadlineCode` maps a cancelled or context-ended read to
  `GitTimeout` when the deadline context reports `DeadlineExceeded`. A caller's cancellation
  without a deadline keeps `GitCancelled`. `ErrStatusUnavailable` still wraps both.

- **AFP-V0-549:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-2728)
  Amended under the owner's standing auto-accept, 2026-11-04, V1-4576, TSP-V0-7318:
  `TestIncrementalSelectionMeetsTheLiveBudget` MUST measure up to three rounds of 32 interleaved
  samples per representative edit, bracketing each selection with calibration probes before and
  after it. It MUST normalize each selection against the mean of the two adjacent probes, stop at the first
  round within the budget, and enforce the round with the lowest normalized p95. A slower Select is over the budget in
  every round and still fails; a transient spike from a concurrent heavy.sh run does not. The
  budget, the probe and the reference ratio are unchanged.

- **AFP-V0-550:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-3688)
  For each selected Go unit with a dirty `_test.go` file that adds, removes or changes a
  `TestMain` between the base (or HEAD when no base is given) and the tree, `corvint affected`
  MUST add a `whole_packages` entry `{unit, path, reason}` stating that TestMain wraps every test
  of the package. A test file that cannot be read or parsed on either side keeps its unit whole
  with that reason. The plan's selection is unchanged; the entry replaces a hand-written
  `HEAVY_WHOLE_REASON`. The compact form names the same entries.

- **AFP-V0-551:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4173,
  V1-3392) The Go plugin's path-literal reads MUST honour the ASD-V0-895 marker
  `//corvint:fixture-path`. A token held only by string literals of a `_test.go` file that start
  or end on a line carrying the marker names no `PATH_LITERAL_READER` read for that file. Any
  unmarked literal in the same file that holds the token keeps the read. A marker in a non-test
  file, or on a line the literal neither starts nor ends on, has no effect. The marker never hides
  an unbounded read: a marked `".."` or a climbing literal still makes the file an
  `UNBOUNDED_READER`. `FilePathTokens`, which `tools/test-read-scopes-check` applies the marker to
  itself, still returns every token. Measured on this lane (build log `2026-11-01-aff-w26`): a
  Makefile-only edit selects 58 units, down from 60 (`internal/liveverify/affected/cfamily` and
  `tools/ci-forward-row` leave). A Makefile plus `.gitignore` edit selects 77, down from 79. Every
  unit that remains names its witness and cause, for example the `".."` root readers of
  `internal/liveverify/affected/golang` and `tools/test-read-scopes-check`.

- **AFP-V0-552:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-3322)
  A differential test MUST compare the Python adapter's `scanImports` with the imports that
  Python's own `ast` finds, over a corpus of tricky inputs: f-strings, t-strings, bytes and raw
  prefixes, nested and triple quotes, escapes, line continuations, compound bodies and semicolon
  chains. Every import `ast` finds must either be read with the same module and relative level,
  or the file must raise a frontier. A silent miss fails the test. An input the installed
  `python3` rejects fails the test, unless the corpus lists it as needing a newer Python. The test
  skips without `python3`. On this lane's corpus there are no silent misses. Three imports that
  follow a closing multi-line triple-quoted string are missed but covered by
  `python:conditional-import`. They are logged and reported as an over-wide frontier, not fixed.

- **AFP-V0-553:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4170)
  The shell plugin MUST link a script that a file runs through a variable in three more forms: a
  default that names the script's own directory (`heavy=${H-$(dirname "$0")/heavy.sh}`), a
  `${NAME:+word}` or `${NAME+word}` command word, whose values are empty or the word's first
  field, and a `sh -c` or `bash -c` command string held in a variable, which is parsed once for
  each value the file gives it. Any other `$(...)` part stays unknown, so it never reads as a
  literal path. Each form only adds edges. With it, `script/analyzer-schema_test.sh` and
  `script/codex-review_test.sh` inherit `shell:dynamic-source` from `script/heavy.sh`, as
  `.corvint/unbounded-readers.json` records, and stay selected for an unrelated edit.

- **AFP-V0-830:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-3509,
  amending `AFP-V0-160` (6) and `AFP-V0-465`) This repository's `.corvint/documentation-paths.json`
  MUST also declare `integrations/README.md` and every tracked `integrations/HOST/README.md` one
  directory down, and `script/gen-documentation-paths.sh` MUST generate them, so `--check` keeps
  the list current. A host README describes a host package: no build or test reads it except by a
  path literal or its enclosing directory, which `AFP-V0-160` (3) keeps selecting, and a package
  `files` list ships it without running it. A README-only edit is then `DECLARED_DOCUMENTATION`,
  not `UNOWNED_DIRTY_PATH`, and admits no language plugin. Target, measured on this repository
  with `affected --path integrations/opencode/README.md`: no `UNOWNED_DIRTY_PATH` entry and no
  frontier of a plugin that neither owns, reads nor reaches the path (cfamily, dotnet and the
  others; 76 entries before), with `internal/opencodequalification`, which hashes that README,
  still selected. The TypeScript tests that name `README.md` stay selected as path-literal
  readers, so that plugin still takes part and keeps its own entries under `AFP-V0-511` (3).
  Non-goal: any other file under `integrations/`, which stays a source or unowned path, and the
  TypeScript literal rule itself.

- **AFP-V0-831:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-3509,
  V1-4258; amending the `AFP-V0-551` non-goal) The dependent nested-module reader
  (`AFP-V0-582`, `nested.go`) MUST honour the `//corvint:fixture-path` marker as the package reader
  does: a token only marked literals of the module's `_test.go` files hold is no path token of the
  module's unit. The token still adds its import edge, and a marked literal still counts toward
  the module's unbounded-read reasons. Motivation: `tools/local-authority`'s
  `codec_package_test.go` builds a Git tree object whose entry is named `README.md`; the bare token
  made that module a reader of every `README.md` and brought `go:nested-module-frontier` into a
  host README plan. Target: an `integrations/claude-code/README.md` edit beside a Go change adds no
  unknown entry to the Go change's own plan (`AFP-V0-830`).

- **AFP-V0-832:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4259,
  amending `AFP-V0-094`) `affected --path` MUST NOT refuse a dirty worktree whose every dirty path
  is a declared documentation path (`AFP-V0-160`, read from the worktree's own
  `.corvint/documentation-paths.json`, which must itself be clean) that no unit of the built graph
  owns: no plugin reads such a file as source, so the graph and the plan are HEAD's, and the
  receipt is byte-identical to the one a clean worktree gives. Any other dirty path, an unreadable
  declaration, or a dirty path a unit owns keeps the `unsupported-affected-mixed-worktree`
  refusal; `--require-clean` is unchanged. Motivation: a lane's untracked build-log entry made
  every `--path` probe refuse until the entry was committed. Non-goal: tolerating any other
  untracked file.

- **AFP-V0-2200:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-4409, V1-4362) The Go plugin SHALL record a unit's import-graph reads. A file of the unit
  that calls `exec.Command` or `exec.CommandContext` (through an unshadowed `os/exec` import) with
  the command literal `"go"` and the first argument literal `"list"` makes the unit an
  import-graph reader. The arguments are the call's own, or a local `[]string` variable that is
  only declared by a composite literal, extended by `name = append(name, ...)` and passed as the
  call's variadic argument. The call is *proven* when every flag is `-deps`, `-e`, `-f TEMPLATE`
  or `-f=TEMPLATE` with a literal template whose field references are only `ImportPath`,
  `Imports`, `Deps`, `Name`, `Standard` or `DepOnly` and which holds no bare dot (`{{.}}`) and no
  variable (`$`), and every other argument is a package pattern: a literal, or a `+`
  concatenation whose first operand is a literal or a file constant that does not start with `-`.
  The outermost function holding the call must also assign no `Args` field, assign an `Env`
  field only `append(os.Environ(), ...)` or `[]string{...}` of string literals none of which sets
  `GOFLAGS` or `GOENV`. At every environment proof site, the `Environ` receiver SHALL
  resolve to an unshadowed standard `os` import (including an explicit alias), not merely
  have the spelling `os`; unknown receivers and environment forms leave the call unproven.
  It must also make every other `os/exec` call with string-literal arguments only, so
  no listed name feeds another command; the call, its command, its output and every value derived
  from them must stay in that function: none is returned, sent on a channel, stored anywhere but a
  local of the function, or passed to a call other than a builtin or conversion, a function of a
  standard-library import, or a method of a derived value or of a `*testing.T`, `*testing.B`,
  `*testing.F` or `testing.TB` parameter of a function enclosing the call, so no helper or caller
  can build or run what it lists; and no file of the unit may name `GOFLAGS` or `GOENV`.
  Every builtin or predeclared conversion used by these proofs SHALL have unshadowed identity:
  conservatively, any declaration or import shadowing a predeclared name anywhere in the unit,
  a dot import, or an unreadable/unparseable file leaves every listing unproven.
  Each literal pattern of a proven call names the directory it
  resolves to as `commandDirectory` resolves a command literal (AFP-V0-402); a pattern with
  `...`, the `all` pattern, a computed pattern or a directory that holds no observed package
  makes the unit a reader of the whole module (`Unit.ImportGraphReadsAll`), and `std` and `cmd`
  name nothing. A literal that is only an argument of a proven call SHALL add no command edge
  (AFP-V0-037): it lists an import graph and runs no binary. A `go list` call that is not proven
  makes the unit a reader of the whole module and keeps its literals as ordinary uses, so their
  command edges stay. The targets are `Unit.ImportGraphReads` (unit ids), part of the graph
  digest and the graph cache entry when present.

- **AFP-V0-2201:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-4409, V1-4362) A plan SHALL select an import-graph reader with the witness
  `IMPORT_GRAPH_READER` exactly when the change can alter the import graph it lists: the module's
  import graph changed (AFP-V0-2202), or the reader reads the whole module and some import set
  changed, or a unit it names depends, through import edges, on a unit whose import set changed
  (the changed unit itself included). The witness chain runs from the changed unit through its
  importers to the listed unit, then the reader; a module-wide change names the reader alone with
  the module file as its dirty path. Like a binary-exec runner it is selected one edge past what
  it reads and the traversal does not continue from it (a terminal selection, AFP-V0-560), and
  additive narrowing (ADN-V0-003) prunes no edge of the closure. A body-only edit changes no
  import set and selects no reader through this edge; an edge the unit also holds (an import, a
  command edge, a path literal, a declared read scope, an unbounded read) selects it as before.

- **AFP-V0-2202:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-4409) `corvint affected`, `corvint test` and a review's snapshot plan SHALL compare, when the
  graph holds an import-graph reader and a dirty path is a `.go` file the go tool may build or a
  module file, each such directory's package header at the commit the change is measured from
  with the observed tree: for every `.go` file of the directory, test files included, its name,
  its `//go:build` and `// +build` lines, its `//go:embed` lines, its package clause and its
  sorted import paths. A dirty data file present on one side only below a Go unit that embeds
  changes that unit's import set as well, since `go list` fails on an embed pattern that matches
  nothing (the unknown comparison counts every dirty data file below an embedding unit). A
  directory whose header differs, or cannot be read or parsed on either side, changed its unit's
  import set; a changed directory that holds no unit of the graph (a removed package), a dirty
  `go.mod`, `go.sum`, `go.work`, `go.work.sum` or `vendor/modules.txt` changes the module's import
  graph. A path under `testdata` is no package file. (selbatch fix round 1, 2026-11-06,
  AZU-V0-005) A `.go` path below a directory whose name starts with `_` or `.`, which `./...`
  skips but an import still builds and the Go plugin indexes, SHALL be a package file in both
  the comparison and the assumption below, unless no unit owns it and the repository declares it
  non-source output (AZU-V0-9921) or it lies below `.git`, `.hg` or `.svn`, so an import change in
  an indexed hidden package selects the readers that list an importer of it. Unconstrained test files
  fold into their package's test import sets (AFP-V0-9010). Without the comparison (a `--path` plan, an unreadable base, a caller of `affected.Select`
  that supplies none; library planners that measure from a base supply it, AFP-V0-9011) the selector SHALL assume every dirty `.go` file of a Go unit changes its
  unit's import set and every unowned one, or a module file, changes the module's, so no reader is
  lost. Measured (build-log entry `2026-11-03-selprec-selection-precision`): deleting
  `cmd/corvint-lsp/swift_impact.go`, an import-set change in a binary that
  `internal/swiftimpact`'s `TestNGIV0988TasksClosureExcludesPlannerAndImpactBinariesLinkReader`
  names only by a computed string, selected 57 units without that package before (its test fails
  on the change) and selects it now, 58 units.

- **AFP-V0-2203:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-4173; amending `AFP-V0-072` (1) and (2) and V1-0290 for Go units; round 8) A Go unit's path
  literal SHALL NOT be narrowed by placement: outside the CEM sidecar and sealed-CEM paths, every
  token of a Go unit, anchored, climbing, lone or multi-component, SHALL name a dirty path by the
  component-run relation at any depth, so a lone token also names a directory component, not only
  the file name, and a unit that a token names this way is a `PATH_LITERAL_READER`. Placement may
  drop a reader only when every root a token can be joined to is a literal or proven-bounded
  repository-relative root, and no such proof is sound for Go source: a root can come from
  `os.Getenv`, a flag, a variable, a helper in another package, a changed working directory or a
  walked directory (`filepath.WalkDir(os.Getenv("FIXTURES") + "/repo", ...)` with `FIXTURES` set
  to `app/x` reads every file below `app/x/repo`), and no literal of the unit spells it. A Go unit
  therefore keeps every reader its tokens name; only a non-Go unit whose plugin certifies its
  roots (`Unit.PlacesFileNames`, AFP-V0-400) keeps the AFP-V0-072 placement test, the CEM rules
  (V1-0230, AFP-V0-931) are unchanged, and a declared read scope still bounds a unit. Rounds 6
  and 7 retained multi-component and file-name runs of a runtime-rooted token; round 8 closes the
  class by retaining every run. The PR 86 measurement of the withdrawn placement (24 Go readers of
  `internal/jslinecov/testdata/repo/lib.js` became 6) no longer holds: those 24 readers are
  selected again, as before this lane.

- **AFP-V0-2204:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-4173) The shell plugin SHALL add no path token for a literal path that follows the
  expansion of a temp root: a variable assigned exactly once in the file, whose every other
  occurrence is an expansion that cannot assign or text joined to a `/`, `.` or `-`, and whose
  value is one `$(mktemp -d ...)` substitution or another temp root followed only by a literal
  path without `..` (`repo="$tmp/repo"`). The assignment is a command of its own at the file's
  top level, not after `&&` or `||` and not in a pipeline or the background, carries no
  redirection (round 7: a redirection that fails, `3>&9` in dash, skips the assignment and leaves
  an inherited value in place), and no expansion of the name precedes it, so no value from the
  environment stands in. A file that defines or aliases
  `mktemp` has no temp root, nor does one that can fill a directory with repository files no token
  names: a recursive (`r`, `R` or `a` anywhere in a short option group, as in `cp -rf`), archive
  or `--parents` `cp`, `rsync`, `tar`, `cpio`, `pax`, `unzip`,
  `ditto`, `ln`, `mv`, `xargs`, or a `git` `clone`, `worktree`, `archive`, `checkout-index` or
  `--work-tree`. One that assigns `PATH` before the variable must call `command -p mktemp`. A temp
  root's directory is new when created, so a path below it is what the script itself put there;
  a single file it copies there is read through the copy's source path, which keeps its token. A
  path with `..` after the root keeps its tokens. Measured with the by-path probe below: the shell
  path-literal readers of `internal/jslinecov/testdata/repo/lib.js` fall from 28 to 23; the rest
  copy trees, assign the name more than once or assign it in a function.

- **AFP-V0-2205:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-4278; amending `NSA-V0-006`) The shell plugin SHALL own each file directly under a top-level
  `bin/`, `script/` or `scripts/` directory whose name has no dot, and SHALL build a unit for one
  whose first line is a shebang running `sh`, `bash`, `dash`, `zsh` or `ksh`, directly or through
  `env`. The unit is modelled as any shell file (links, sources, foreign-invocation frontier,
  unbounded reasons), so a caller that runs it by path links to it. An owned file with another
  interpreter gets no unit, so an edit to it is `UNINDEXED_SOURCE_PATH`. Measured: an edit to
  `script/release-checklist` drops from 75 unknown entries (74 `LANGUAGE_FRONTIER` and the
  `UNOWNED_DIRTY_PATH`) to 2 (`shell:foreign-invocation-unmodelled` and the honest
  `NO_SELECTABLE_TEST`: its test copies it into a temporary repository), with 51 units selected.

- **AFP-V0-2206:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-3864) A plan SHALL name, in `plan.deleted` in `plan.dirty` order, each dirty path that is
  absent from the tree the graph was observed from (`Lstat` reports it missing): a file the
  change deletes or renames away, including a path whose ancestor is now a file. The member is
  omitted when no dirty path is absent, in a sparse checkout, where an absent path may lie outside
  the cone (AFP-V0-821), and in a `--path` plan, whose paths are probes rather than changes; the
  compact form carries the same list. Selection is unchanged.
- **AFP-V0-2480:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-2662, V1-1184) A repository MAY declare test ownership in `.corvint/test-selection.json`, a
  closed JSON object of at most 64 KiB with profile `corvint-test-selection/0`, an optional
  `owners` list and an optional `mustRun` list. An owner is `{"paths": PATTERN, "tests": [TEST,
  ...]}` with 1 to 16 tests; a must-run entry is `{"tests": TEST, "reason": TEXT}`. There are at
  most 256 owners and 256 must-run entries; owners are strictly ascending by `paths`, each owner's
  tests strictly ascending, and must-run entries strictly ascending by `tests`. A TEST is a
  repository-relative test file, or a directory when it ends in `/`, in the AFP-V0-160 entry
  grammar, never `.corvint` or under it; a reason is 1 to 200 bytes of printable UTF-8 without a
  control character. The declaration is project-owned authority (`AGENTS.md` invariant 3) that
  only adds selections: it never removes a unit, never narrows any other rule, and changes no
  unknown except as AFP-V0-2482 states. A declaration that is unreadable, not a regular file,
  oversized or invalid in any member refuses the graph build, naming the declaration and the
  member. Its entries are part of the graph digest only when present, so graphs without it keep
  their digest, and of the graph cache entry. When the declaration itself is a dirty path no unit
  names, it is a `CORVINT_DECLARATION` (AFP-V0-443).
- **AFP-V0-2481:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-2662) An owner's PATTERN is at most 256 bytes of `/`-separated segments, with no leading or
  trailing `/` and no empty, `.` or `..` segment. A segment that is exactly `**` matches zero or
  more whole path segments, and at most four segments are `**`. In any other segment `*` matches
  any run of characters other than `/` and `?` one such character; `[`, `]`, `{`, `}` and `\`
  are refused, so no character class, alternation or escape exists. A pattern matches a dirty
  path when it matches the whole repository-relative path, case-sensitively. No pattern matches a
  path that is `.git` or `.corvint` or lies under either, whatever its segments.
- **AFP-V0-2482:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-2662) For each dirty path an owner's pattern matches, every unit holding a test file equal
  to or under one of that owner's tests MUST be selected. A unit not already reached gets the
  terminal witness `DECLARED_TEST_OWNER`: the first such dirty path in plan order, `via` the unit
  alone, and `glob` the pattern. Like a declared read scope (AFP-V0-023) it is not traversed. The
  plan then removes each `UNOWNED_DIRTY_PATH` unknown whose path an owner pattern matches and
  whose owner resolves to at least one unit. It lists those paths, in plan order, as one
  `nonSource` group of class `DECLARED_TEST_OWNER` with a stated reason, after the
  `DECLARED_RUNTIME_READERS` groups and before `CORVINT_DECLARATION`. The reader rules have
  already selected every unit that names such a path, and the declaration states that its owners
  are the tests that exercise it. An `UNINDEXED_SOURCE_PATH` (a path a plugin claims as source),
  a `LANGUAGE_FRONTIER` entry and every other unknown are never removed (AFP-V0-2488).
- **AFP-V0-2483:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-2662, V1-1184) A declared TEST that holds no test file of any unit contradicts the observed
  graph. In a graph built from every plugin, such an owner test or must-run entry refuses the
  graph build fail-closed, naming the declaration and the entry, so a stale mapping can never
  turn an unowned path into a selection of nothing. A partial graph, built from a subset of the
  plugins (AFP-V0-819), cannot tell a stale entry from one an absent plugin observes. It skips
  that entry instead. An owner left with no resolving test keeps its unknown entries.
- **AFP-V0-2484:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-1184) When a plan has at least one dirty path, every unit holding a test file equal to or
  under a must-run entry's TEST MUST be selected, whatever else the plan decides. No exclusion
  basis removes it: not `NO_DEPENDENCY_PATH_TO_DIRTY_UNIT`, an additive narrowing
  (`goAdditiveExcluded`, ADN-V0-005), a documentation-only or other non-source classification, or
  a frontier left out (AFP-V0-560). A unit nothing else reaches gets the witness `MUST_RUN`, with
  the first dirty path and `via` the unit alone. A unit something else reaches keeps that witness.
  The plan names each must-run unit in a `mustRun` member, sorted by unit id and then TEST, with
  `unitId`, `tests` (the entry) and `reason`. The member is omitted when there is none, and the
  compact form carries the same list. The step runs after the frontier and unknown steps and is
  not part of the reached universe (`ReachedUnitIDs`), so a must-run unit adds no unknown, never
  changes `scope`, and never counts as reached by the change. A plan with no dirty path selects
  nothing.
- **AFP-V0-2485:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-1184, V1-2662) The `DECLARED_TEST_OWNER` and `MUST_RUN` witnesses rank in tier 0
  (AFP-V0-162). `corvint test` MUST run a Go unit named in `plan.mustRun` or
  `plan.declaredOwners` whole, never as a STR-V0-063 or TSP-V0-006 slice. Its slice reason is
  `whole: MUST_RUN (REASON)`, or `whole: DECLARED_TEST_OWNER via PATH` for an owner-selected unit. The CI selector
  (`internal/gateselect`) and `corvint test` start from the plan's Go packages, so both run every
  must-run and owner selection.
- **AFP-V0-2486:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-1184) Under `--selection-profile e2e-safe`, an inventory test whose file a unit in
  `plan.mustRun` owns MUST be selected with the reason `declared-must-run`, an additive value of
  the AFU-V1-019 `reasons` list, so no exclusion proof can omit it. Such a unit puts no path into
  the obligation closure through its must-run selection alone, so a must-run test is never
  labelled `static-reach` and selects no linked test by itself. The `strict` and `coverage`
  profiles are unchanged (AFU-V1-024).
- **AFP-V0-2487:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-1189; amended under the owner's standing auto-accept, 2026-11-04, V1-4472,
  TSP-V0-7305; amended under the owner's standing auto-accept, 2026-11-05, self-review repair) `corvint affected --fail-fast-order` MUST add an `order` member to the compact plan.
  `order.key` is `["recent-local-failure", "plan-order"]`. `order.units` lists every
  `plan.selected` unit once, as `{unitId, recentFailure, planIndex}`: units with a recent local
  failure first, each group in `plan.selected` order. That order is already AFP-V0-007's: via
  length, then proximity, then unit id. `order.failures` names the evidence. Its `receipts` list
  holds each `corvint test` receipt read from `<git-dir>/corvint/test-run.json` and
  `test-run.<n>.json`, at most 64: probe the newest 64 mtime-ordered candidates (ties by name),
  plus the unsharded `test-run.json` when outside that prefix. Retain the newest 64 of that pool
  by valid RFC3339Nano `completedAt` from bounded 64 KiB headers, with mtime fallback and name
  as tie breaker. List retained receipts in name order. At most 65 headers are probed; no
  completion-time recency is claimed for older shards outside the pool. Metadata Info
  failures MUST be disclosed in `order.failures.detail` even outside the retained 64 and make
  no recency claim; they set the aggregate state to `UNREADABLE`. Each retained receipt is
  read by the STR-V0-010 strict reader. Each entry has the
  `path` relative to the Git directory, `state` `OBSERVED` or `UNREADABLE` (with `detail`), and
  for an observed receipt its `sha256`, `revision` and `failed` (units with a `FAIL`,
  `BUILD_FAILED` or `TIMED_OUT` outcome, sorted). `order.failures.state` is `OBSERVED` when a
  canonical receipt was observed and no candidate metadata failed. It is `UNREADABLE` with a
  `detail` when `<git-dir>/corvint` exists but cannot be listed (no receipts), or when candidate
  metadata fails (other observed receipts remain). It is `NOT_OBSERVED` otherwise, including
  an absent directory. The order only reorders: `plan.selected`, the
  tiers and every other member are byte-identical with and without the flag. Without the flag
  nothing is read. The flag is refused with `--full`, `--summary`, `--by-path`,
  `--format text` and `--playwright-config`, whose documents carry no compact plan member.
- **AFP-V0-2488:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-2662) An owner declaration only adds. A dirty path in the `DECLARED_TEST_OWNER` group still
  counts as a path no plugin owns for the frontier rules: it raises every non-participating
  plugin's frontier entries as AFP-V0-511 raises them for an unowned path, and it turns the
  AFP-V0-560 terminal rule off, so no plugin's frontier moves to `frontierExcluded` because of
  the declaration. Every `LANGUAGE_FRONTIER` entry a plan holds without the declaration it also
  holds with it, and so does its `UNKNOWN` scope when a frontier bears; the declaration removes
  only the `UNOWNED_DIRTY_PATH` entry AFP-V0-2482 names.
- **AFP-V0-2489:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-2662, V1-1184) The plan and its compact form name, in a `declaredOwners` member, every
  unit an owner pattern selects for the plan's dirty paths, whatever witness reached it first:
  `{unitId, paths, dirtyPath}` with the pattern and the first dirty path in plan order it
  matches, once per unit and pattern, sorted by unit id and then pattern. The member is omitted
  when no owner pattern matches a dirty path. Selection, scope and unknowns are unchanged. The
  AFP-V0-162 tiers rank in tier 0 every unit the plan names in `mustRun` or `declaredOwners`,
  whatever its first witness. Only that witness decides whether the unit raises a parent
  neighbourhood: a declared unit reached by its own source raises one as before, and one reached
  only as a declared or must-run unit raises none.

- **AFP-V0-2440:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4446)
  A plugin's claim on a dirty path no unit lists SHALL be a fixture-only claim, not a source claim,
  when the plugin is not Go, the path lies below a `testdata` directory a Go unit hosts (the
  AFP-V0-446 hosting rule, outermost `testdata` component) and the plugin lists no path below that
  directory. A path whose every claim is fixture-only SHALL be `UNOWNED_DIRTY_PATH`, never
  `UNINDEXED_SOURCE_PATH`, with the hint `claimed only as fixture data by <plugins>: the path lies
  below <directory>, a testdata directory a Go unit hosts, below which that plugin lists no path
  (AFP-V0-2440)`. The path keeps its claimants for every frontier rule: it is not an unowned path
  for AFP-V0-511 or the AFP-V0-560 terminal rule, so other plugins' frontiers are not raised for it
  (a claimed path raises its claimants' whole frontiers instead). A claim by the
  Go plugin, a path outside `testdata`, an unhosted directory, and a plugin that lists a path below
  the directory keep `UNINDEXED_SOURCE_PATH`. Motivation: the Ruby plugin drops `testdata` files in
  a Go-module root (RUBY-AFFECTED-V0-041) but still claims `*.rb`, so PR 85's new package's Ruby
  fixtures made the CI selector fall back to every shard (run 41714153849). The Go toolchain builds
  nothing below `testdata`; the hosting package's tests read the files as bytes, which the CI
  selector's rules (b) to (d) attribute to Go packages as they do for an unowned path.
- **AFP-V0-2540:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4532)
  When `unbounded-readers-check` reports a package unrecorded and HEAD's committed
  `.corvint/test-read-scopes.json` declares it while the working tree's copy does not, the report
  SHALL add a `likely cause` line naming the removed declaration and both repairs: restore the
  entry, or record the package. Other unrecorded packages keep the existing report, and without
  Git, a commit or a readable committed declaration no line is added. Motivation: the refusal named
  only the unbounded-readers record, so a lane that dropped a declaration edited the wrong file.
- **AFP-V0-2541:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4532)
  `test-read-scopes-check` SHALL fail for each package that HEAD's committed declaration lists and
  the working tree's copy does not, when the package's Go tests are now unbounded readers and
  `units` of `.corvint/unbounded-readers.json` does not list it. The failure names the package, both
  files and both repairs, citing AFP-V0-025, whose ratchet compares sets. A listed package, a
  package no commit declares, and an unreadable record (left to `unbounded-readers-check`) are not
  reported.
- **AFP-V0-2542:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4512)
  AFP-V0-012 rule (d) SHALL NOT make a Go test file an unbounded reader for a relative package
  pattern, `"./..."` or `"./NAME/..."` whose components are plain names, when the pattern's subtree,
  resolved against the test's package directory, holds no other directory with a Go file outside
  `testdata` and no nested `go.mod`. Such a pattern reaches no more than `"."`, which rule (d)
  already accepts. The planner's Go plugin and the CI selector (`internal/gateselect`) SHALL apply
  the same rule and agree on every fixture case. A subtree with another package or a nested module,
  an import-path pattern, a component that is empty, `.`, `..` or holds `...`, and any non-test file
  keep rule (d). The exemption SHALL hold only while no Go file of the package moves the working
  directory: a `Dir` assigned or keyed to anything but the literal `"."` or `""`, a `Chdir` or
  `Fchdir` identifier, or a string literal with a `cd` or `pushd` word or a `-C` or `--chdir`
  option keeps rule (d) for every such pattern in the package, as when a test finds the
  repository root through `runtime.Caller`, sets `cmd.Dir` there and runs `go build ./...`.
  Motivation: a leaf package whose test passed `./...` to a Go command was selected on every
  change, though the pattern reaches nothing outside the package directory.

- **AFP-V0-2400:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-3489) A repository MAY declare, in `.corvint/shell-test-inputs.json` (the closed object
  `{"profile":"corvint-shell-test-inputs/0","tests":{TEST:{"inputs":[ENTRY...],"reason":R}}}`,
  at most 64 KiB, 1 to 64 tests, 1 to 64 sorted unique inputs each, a printable reason of at most
  512 bytes), the paths a shell test reads by a word the shell plugin does not resolve as a path. An
  `ENTRY` is a repository file, a directory ending in `/` (AFP-V0-023 grammar, never `.git`), or
  such a directory followed by one last component holding a `path.Match` wildcard
  (`script/*.sh`). Each input SHALL join the declared test unit's path globs anchored at the root
  (`/Makefile`, `/integrations`, `/script/*.sh`), so a dirty path it holds selects the test with a
  `PATH_GLOB_READER` witness naming that glob (AFP-V0-401). The declaration only adds: the unit
  keeps every literal, edge and unbounded read the plugin observed. A declaration that is
  unreadable or invalid adds nothing and names the plugin frontier `shell:test-inputs-invalid`; an
  entry naming no observed shell test (a unit with one test file and no source) adds nothing and
  names `shell:test-inputs-invalid:TEST is not an observed shell test`. Because the plugin reads
  the declaration when the graph is built, a plan whose dirty paths hold
  `.corvint/shell-test-inputs.json`, a deletion included, SHALL be UNKNOWN with the
  `LANGUAGE_FRONTIER` detail `shell:test-inputs-changed` (review round d), so an edit that drops an
  input together with a change that input covered never narrows a selection, and
  `tools/shell-test-select` runs every step. The repository declares
  `script/no-python-runtime-dependency_test.sh` (it prints the bare word `Makefile` and lists
  `script/*.sh` and `integrations/` with `find`).
- **AFP-V0-2401:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-3489) `tools/shell-test-select --plan PLAN [--makefile Makefile]` SHALL print, one per line,
  the `shell-test` prerequisites whose recipe runs a `script/*_test.*` file the plan selects as a
  unit, and every prerequisite whose recipe names no such file. It SHALL print every prerequisite,
  with `shell-test-select: FULL N of N steps: REASON` on stderr, when the plan is unreadable or not
  a successful `affected-plan/1`, omits or nulls any of the `dirty`, `selected` or `unknown`
  arrays, contains a malformed member, names `Makefile` as dirty, or carries an unknown other than a
  `LANGUAGE_FRONTIER` `gateselect.AllowedUnknown` admits, a `NO_SELECTABLE_TEST` or a well-formed
  `UNSELECTED_BINARY_CONSUMER`; an `UNOWNED_DIRTY_PATH` and every `shell:` frontier but
  `shell:foreign-invocation-unmodelled` (an invalid or changed shell declaration can hide a shell
  test's edge or input) run every step. Each dirty path MUST be a nonempty relative slash path
  with no dot or parent element, backslash, whitespace or control character; each selected entry
  MUST have a `unitId` comprising a lowercase language token (`[a-z][a-z0-9-]*`), a colon and
  such a path (the root `.` is admitted). Each unknown MUST have nonempty `reason` and `detail`
  strings. Missing, null or wrongly typed members SHALL take FULL; complete empty arrays are
  valid. The FULL reasons SHALL name incomplete arrays (`the plan is incomplete: dirty, selected
  and unknown must be arrays`), a malformed dirty path (`the plan has a malformed dirty path`),
  incomplete unknown entries (`the plan has an incomplete unknown entry`), or invalid selected
  IDs (`the plan has a missing or malformed selected unitId`), as applicable. It SHALL print
  `shell-test-select: NARROWED K of N steps` otherwise, and name each selected shell test no step
  runs (`selected PATH is no shell-test step; run it by its own driver`). `make shell-test-affected
  BASE=SHA` SHALL build `corvint` and the tool from the checkout, plan with `corvint affected
  --base`, and run the printed steps with `--keep-going`; a failed plan runs every step.
- **AFP-V0-2402:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-4363; amends STR-V0-061 and STR-V0-002) The AFP-V0-012 attribution `corvint test` computes
  SHALL still decide `FALLBACK` and `NOTHING`, but a `run` verdict SHALL run exactly the plan's
  own Go selection (`provider.go.packages`), so `corvint test` and `corvint affected` select the
  same Go packages. The attribution's rule (a) to (c) lines add packages the plan's reviewed
  narrowings exclude (the lone-component and directory-resolved literal rules of AFP-V0-021 and
  AFP-V0-072, namer edges from any path token naming a package directory) and no longer enter the
  run set or the manifest. A `run` verdict over a plan that selects no Go package SHALL be
  `NOTHING the plan selects no Go package; the AFP-V0-012 attribution names only packages the
  plan's rules exclude (AFP-V0-2402)`. The CI PR tier's deciding selector (`gateselect.Select`) is
  unchanged, so CI's executed set keeps the attribution.
- **AFP-V0-2403:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-4363) `tools/selection-baseline` SHALL compare, for every replayed change whose `corvint
  test --dry-run` manifest is `NARROWED`, the manifest's Go units (`go:` units, nested-module units
  excepted) with the plan's selected units and the change's failing packages, recording
  `testExtra` (run but not selected), `testOmitted` (a root Go package the plan selects, a `go:`
  unit, that the manifest does not run; review round e P2) and `testMisses` (failing but not run)
  per change and in the summary, and `make selection-baseline-check` SHALL fail when any change has
  a `testExtra` or `testOmitted` entry, whether or not the change records a failing package, or
  the `testMisses` total exceeds the floor's `misses`. Only a `FULL` manifest SHALL be exempt
  from the `testOmitted` and `testMisses` comparisons: a `NOTHING` manifest, or one of any other scope, SHALL be
  compared as an empty run set whatever units it lists, so each failing package of its change is
  a `testMisses` entry even when the CI selector selects that package (review round c P2), and
  each root Go package its plan selects is a `testOmitted` entry. A dependent nested module's
  synthetic `go:` unit (AFP-V0-582) is no root package: it SHALL be left out of `testOmitted` only
  when the manifest's `excluded` list names it (STR-V0-069) and its directory below the root module
  path holds a regular `go.mod` at the replayed head; any other excluded unit stays compared. The
  verdict line SHALL end `test-extra=N test-omitted=N test-misses=N`.
- **AFP-V0-2404:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-3480) This repository's `.corvint/runtime-readers.json` (AFP-V0-602) SHALL declare
  `LICENSING.md`, `protocol/` and each Corvint declaration that is neither self-classifying nor a
  driver input: `.corvint/evidence-reach.json`, `existence-readers.json`,
  `gated-root-locators.json`, `runtime-edges.json`, `shell-foreign-callers.json`,
  `shell-read-scopes.json`, `shell-test-inputs.json`, `test-binary-consumers.json`,
  `test-binary-execs.json`, `test-nested-sandboxes.json`, `test-read-scope-literals.json` and
  `work-queue-policy.json`, each with the readers its reason names. A change to one alone is then a
  `DECLARED_RUNTIME_READERS` path whose literal, scoped and unbounded readers are selected, with no
  `UNOWNED_DIRTY_PATH` and no frontier fan-out; a `*-changed` frontier such a declaration already
  raises (`go:gated-root-locators-changed`, `shell:read-scopes-changed`,
  `shell:foreign-callers-changed`, `shell:test-inputs-changed`) is unchanged. `Makefile`,
  `.github/` workflow and action files, `.corvint/documentation-paths.json`,
  `.corvint/runtime-readers.json` and
  `.corvint/unbounded-reader-exclusions.json` stay `UNOWNED_DIRTY_PATH`: no sound mapping exists,
  because a driver input runs every step it defines and a declaration would classify itself.
- **AFP-V0-2405:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-3480) An `UNOWNED_DIRTY_PATH` entry whose path is a driver input (a make file, a
  `.github/workflows/` or `.github/actions/` file) and that carries no hint SHALL carry the hint
  `build or CI driver input: every recipe or job it defines runs through it, so no declaration maps
  it to its consumers (AFP-V0-602, AFP-V0-2405); ...`, naming why it keeps the entry. Selection and
  scope are unchanged.
- **AFP-V0-2406:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-4281; amends AFP-V0-381) A plan's `frontierSites` SHALL count the repository sites, reached or
  not, of each plugin the plan involves (`participants`) or whose frontier one of its unknown
  entries names, and no other plugin's; it is omitted when those plugins have none. Adding,
  removing or changing a plugin the change does not involve SHALL leave every other byte of the
  plan unchanged but the graph digest and the exclusion universe that names it.

- **AFP-V0-2240:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-4437; round 8) `internal/gateselect`'s rule (c), in `Select`, `SelectStructural` and the
  `-bounds` attribution, SHALL NOT narrow an anchored token by placement: as in the planner
  (AFP-V0-2203), any token of a package names a dirty path by the component-run relation at any
  depth, whatever root it may be joined to at run time. This is the base rule (c) of AFP-V0-012;
  the round 1 to 7 placement filter (`keepsReader`) is removed. AFP-V0-2204 has no counterpart:
  this index holds only Go packages. The CEM sidecar and sealed-CEM rule (`resolvingReaders`) is
  unchanged.

- **AFP-V0-2241:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-4437; round 8) The same rule (c) SHALL NOT apply the lone-token file-name restriction
  (V1-0290) or the AFP-V0-072 (1) placement test to any other token: a single-component run of an
  unanchored token names any matching component of the path, so a reader that joins `"lib.js"`
  or `"repo"` to a runtime root stays selected. After the printf-verb blanking this index SHALL
  keep every token of the literal as written and add the tokens of its `..\` to `../` rewrite,
  exactly as the planner does (AFP-V0-506; round 9, replacing round 8's substitution):
  `os.Getenv("PREFIX") + strings.ReplaceAll("foo..\\notes", "\\", "/")` keeps `foo..` and `notes`
  and adds `foo../notes`, so it names `app/prefoo../notes`, and
  `os.Getenv("FIXTURES") + strings.Split("foo..\\notes", "\\")[1]` keeps `notes`, so it names the
  root file `notes`, in the verdict and in `-bounds` alike. Round 8 kept only `foo../notes`, so a
  change to `notes` skipped the Split reader with no fallback. A `..\x` literal keeps a bare `..`,
  which rule (d) reads as reaching the repository root from a test file deep enough. The
  narrowing measured on PR 86's range in rounds 1 to 7 (`reader` lines 79 to 28, the run 257 to
  209 packages; AFP-V0-2240 alone 79 to 78) is withdrawn with it, and rule (c) selects as base.

- **AFP-V0-2242:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-4437) The CI selector's rule (c) SHALL stay a superset of the planner's Go path-literal
  readers. On a shared fixture of anchored, runtime-rooted, directory-walking, climbing, lone,
  multi-component, `..\x`, prefixed `foo..\notes` and closing-tag (`"</repo/page.html>"`,
  `"&lt;/repo/page.html&gt;"`) tokens, a test SHALL build both the planner's graph and the CI
  selector's index and fail when, for any probed dirty path, the planner's `PATH_LITERAL_READER`
  Go units differ from rule (c)'s readers, and, since round 9, from the packages rule (c) or
  rule (d) selects counted together with the planner's `UNBOUNDED_READER` units. Since round 9 a
  lexicon rewrite only ever adds a token, in both lexicons: the `..\` to `../` rewrite, the
  `/`-anchored spelling of an import path under the module and the closing-tag spelling without
  its `/` are each kept beside the token as written, never in its place, so every token either
  lexicon read before a rewrite was added is still read. The fixture holds the `strings.Split`
  reader of `foo..\notes` and a walker of a runtime root joined to the module's own import path
  (`filepath.Join(os.Getenv("SRC"), "example.com/fixture")`), each with a control that names
  another path. Wherever the two lexicons differ (AFP-V0-765), this index keeps a token the planner
  drops or a broader one; `..\x` is no difference. A token whose `/` directly follows `<` or an
  HTML-escaped `&lt;` in any letter case is kept in both lexicons as written and without the `/`
  (round 6 here and round 9 in the planner; amending AFP-V0-505, AFP-V0-840 and AFP-V0-765
  difference (4)), so its first component may end a longer name.

- **AFP-V0-2243:** (disabled for 1.0 in round 9, 2026-11-04: an orchestrator-made, owner-delegated
  decision under the owner-approved last-round cap; the ID is kept and not reused, and the rest of
  this entry is the round 6 to 8 text, historical. Since round 9 a `&&`, `||`, `|` or `&` after
  a temp root's assignment refuses it, as at base AFP-V0-2204, because no test this round proves
  where such a list ends: an `&` after a compound command, a continued line or an empty compound
  can background the assignment. The kept refusals (a multi-word assignment command, a refusing
  operator before a compound closer) only widen selection. Repro:
  `tmp=$(mktemp -d) || exit 1` then `cat "$tmp/repo/internal/x/y.json"` keeps the token.)
  (Formerly experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-4173; amending `AFP-V0-2204`) A temp root's assignment MAY be followed by `&&` or `||`: it is
  still the list's first command and always runs in the file's own shell, so
  `tmp=$(mktemp -d ...) || exit 1` makes a temp root as a line of its own does. An assignment
  after `&&` or `||`, in a pipeline or in the background still makes none, and a second
  assignment on the right side refuses as any second assignment does. A list the assignment leads
  that ends in `&` runs in the background, assignment included, and makes none. An assignment
  after a compound command joined by `&&` or `||` (`{ :; } || tmp=...`, `esac && tmp=...`) is
  after `&&` or `||`. A list the assignment leads that continues into a compound command (a brace
  group, subshell, `if`, `while`, `until`, `for`, `case`, `select`, `time`, `coproc` or a function
  definition) makes none (round 6): whether such a list ends in `&` is not visible to the command
  scan (`tmp=$(mktemp -d) && { :; } &`). A list member whose lead operator is not the previous
  member's trail also makes none (round 7): an empty compound (`case x in esac`) leaves no command
  of its own, so `tmp=$(mktemp -d) && case x in esac & wait` shows only the `&` that backgrounds
  it. The lexer reads `&&` and `||` whole, so the next command's lead
  is `&&` or `||`, not a second `&` or `|`. If mktemp fails and the
  right side does not exit, the value is empty and a path below it is absolute below `/`, which is
  no repository path. Measured with the by-path probe below: the shell path-literal readers of
  `internal/jslinecov/testdata/repo/lib.js` fall from 23 to 16; the rest copy trees
  (`cp -R`, `ln`, `mv`, `tar`, `git clone`, `git worktree`) or assign the name twice. None of the
  23 is kept only because it assigns the name in a function.

- **AFP-V0-2244:** (disabled for 1.0 in round 9, 2026-11-04: an orchestrator-made, owner-delegated
  decision under the owner-approved last-round cap; the ID is kept and not reused, and the rest of
  this entry is historical. `.corvint/shell-foreign-callers.json` is the base declaration again, so
  `internal/dogfoodflow` stays a caller of `script/batch-regen.sh`: whether every test that
  imports it, transitively (`cmd/corvint`, `internal/localcompletion`,
  `tools/capabilities-check`), reads only the committed copy is not proven by a test. Repro: a
  `script/heavy.sh` edit over the lane base selects the `BINARY_EXEC` units the measurement below
  removed again (inferred: 52 units back to 71, not re-measured).)
  (Formerly experimental: an orchestrator-made, owner-delegated decision, 2026-11-03;
  V1-4284; amending the use of `TSP-V0-005`) A file that names a declared shell script only to run
  a committed copy of it read from Git history (a parent commit's blob checked out in a private
  clone), or a fixture's own copy, SHALL be listed among the script's `mentions`, not its
  `callers`, with the declaration's reason naming it a history input: its tests never read the
  worktree file, so a worktree edit of the script reaches them only once committed and replayed
  as history. `internal/dogfoodflow` (`batch.go`, `regenerated.go`, `regenerated_test.go`), which
  replays `script/batch-regen.sh` from a parent commit (DCW-V0-401), is the first such entry in
  `.corvint/shell-foreign-callers.json`. The declaration's shape is unchanged, so a planner built
  from the CI pin reads it (AFP-V0-710). Measured on a `script/heavy.sh` edit over the lane base
  with the declaration committed: 71 units to 52, `BINARY_EXEC` 25 to 0.

- **AFP-V0-2245:** (withdrawn 2026-11-03, rejected by the orchestrator under owner delegation:
  unsound: data- and filesystem-derived directories escape the certificate (re-review
  2026-11-03). The ID is not reused and no code implements it; the text is historical.) The
  TypeScript plugin was to record each module's runner directories and certify modules whose
  anchored tokens could then resolve as a Go unit's did under AFP-V0-2203 before round 8. A re-review found
  counterexamples that the certificate cannot see (listed under Failure modes), and TypeScript keeps
  the component-run relation of AFP-V0-072 (3). The counterexamples are regression tests that a
  future attempt must keep passing (`TestTypeScriptAnchoredCounterexamplesStaySelected_AFPV02245`).

- **AFP-V0-2246:** (withdrawn and superseded by the pre-canonicalization `AFP-V0-2204` rule:
  orchestrator-made, owner-delegated decision, 2026-11-03; V1-4173; ID retained, never reused)
  The canonicalization-through-`pwd` narrowing is withdrawn entirely. Independent review found
  that compound-keyword prefixes hide refused commands, expansion can remove a command word's
  slash, scripts can signal their own traps, and line continuations can hide function names.
  These bypasses invalidate the shell allowlist; extending it is not an accepted repair.
  A temp-root token SHALL be dropped only by the earlier `AFP-V0-2204` rule: a literal path
  suffix written directly after a proven temp-root expansion in the script text. A path derived
  through `pwd`, `cd`, a command substitution other than the original `mktemp -d` root creation,
  or a variable assigned from those operations MUST keep its path tokens. In particular,
  `NAME=$(cd "$NAME" && pwd)` (also `pwd -P`) remains an assignment, so the name is assigned
  twice and no longer qualifies as a temp root. The old allowlist and canonicalization helpers
  MUST be removed. The counterexamples remain regression tests for any future proposal.

- **AFP-V0-2247:** (experimental: an orchestrator-made, owner-delegated decision, 2026-11-03;
  V1-3282; amending `LGP-V0-027`) The TypeScript plugin's impact profile (its probes, its dynamic
  load pattern and its static-load rule) SHALL live in `internal/webdialect`, a TypeScript plugin
  package with no package-level variable whose initializer calls a function (its patterns are
  compiled when `webdialect.Impact` builds the profile, so ADN-V0-004's initializer rule holds), and
  core SHALL register `webdialect.Impact(flowWebCodeLines)` for each declared `module-imports`
  impact dialect, its web mask staying in `internal/contextindex`. Behavior is unchanged. Measured
  on a `flowWordBefore` body edit: the `contextindex` record no longer reaches the profile's var
  initializer, but still falls back on the registering `init`
  (`CHANGE_REACHES_PACKAGE_INITIALIZATION`), 151 units as before. The fallback is the real reach:
  `Build` reaches `nonGoImpactUnknowns`, whose `dialect.LookupImpact` runs the registered code
  lines, which run `flowWordBefore`; a registration rule could at most select 147.

- **AFP-V0-920:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3912)
  When `corvint affected` cannot create a scratch directory under the temporary directory (a full
  `/tmp` with no `TMPDIR`, or a `TMPDIR` that is missing or unwritable), it SHALL exit 2 with the
  stable code `temp-unavailable` on stderr. The message SHALL name the temporary directory it tried
  and tell the caller to set `TMPDIR` to a writable directory with free space. The `affected audit`
  scratch clone is the only temporary directory the command creates; the plan itself writes no
  temporary file, and a failed receipt write already exits 2 with `output-failed`.

Non-goals: no free-space probe before the work starts and no fallback directory. Failure mode: a
child process (`git`) that writes its own temporary files reports its own error, which the audit
names as that change's error. Acceptance: `TestAffectedAuditTempUnavailableNamesTMPDIR_AFPV0920`.
Rollback: restore the `cannot create a scratch clone` message in `auditAffectedChange`.

### Deleted Go sources of a surviving package (selection audit 2026-11-06)

- **AFP-V0-2610:** (experimental under the owner's standing lane auto-accept (2026-11-06); lane
  selaudit, the 2026-11-06 selection audit; amends AFP-V0-620) A deleted or renamed-away non-test
  `.go` path SHALL drop its `UNINDEXED_SOURCE_PATH` entry under exactly AFP-V0-620's conditions:
  only the Go plugin claims it, it is absent from the disk checkout the graph was observed from, no
  directory the Go plugin reports as unobserved encloses it, and its directory still holds a Go
  package the graph observes. Rule (a) of AFP-V0-004 (`goSourceRule`) already attributes such a
  path to that package with a `DIRECT_SOURCE_CHANGE` witness, which is a traversal root, so every
  importer and test importer of the package is selected as for an edit to one of its files; the
  ADN-V0 additive narrowing never applies to it, because a deleted file is a `DELETED_FILE`
  additive fallback (ADN-V0-004). A file of the directory compiles into no package other than the
  directory's own, whatever its build constraints, so what other packages compile against changes
  only through that package. Readers of the path are attributed before this rule, as for any
  unowned path (AFP-V0-600, AFP-V0-601). This removes the local/CI divergence in which
  gate-affected-select narrowed the same plan (CIN-V0-860 (4)) while `corvint test` fell back to
  FULL, so the AFP-V0-098 superset guard forced the whole CI suite on every pull request that
  deleted or moved a Go source out of a surviving package (beamfall/corvint#114). Every exclusion
  AFP-V0-620 keeps still applies: a path present on disk, a graph with no disk root, and a deleted
  path whose directory holds no observed package keep the entry (a wholly deleted directory is
  AFP-V0-821's). Failure mode: as AFP-V0-620's, a graph observed from another checkout reads
  absence from the wrong tree. Falsifier: a deleted non-test Go path whose removal changes a test
  outcome in a package the plan excludes. Acceptance:
  `TestMovedGoTestIsAttributedToItsBasePackage_AFPV0620` and
  `TestDeletedGoSourceIsAttributedToItsPackageAndImporters_AFPV02610`. Rollback: restore the
  `_test.go` suffix check in `deletedGoSource`; the path raises `UNINDEXED_SOURCE_PATH` again.

### Deleted fixtures, nested modules on the ci profile, and record entries (lane selgo, 2026-11-06)

Intent (owner goal, 2026-11-06: every local and CI run executes only the minimum tests the change
can affect, every time, and never fewer). The 2026-11-06 selection audit found three causes of
whole-suite or over-wide runs that these rows remove: beamfall/corvint#114 (a moved TypeScript
fixture below `cmd/corvint/testdata` forced FULL), #105 (the `corvint test` shadow refused a nested
module that the CI selector admits, so the AFP-V0-098 superset guard forced FULL), and the readers
that a sealed CEM and a build-log entry select on every pull request (#106).

- **AFP-V0-2740:** (experimental under the owner's standing lane auto-accept (2026-11-06); V1-4931;
  amends AFP-V0-2440) A dirty path that a non-Go plugin claims as source SHALL raise no
  `UNINDEXED_SOURCE_PATH` when it is absent from the disk checkout the graph was observed from, lies
  below a `testdata` directory a Go unit hosts (AFP-V0-446), and no claimant lists a path of its
  fixture subtree (the first directory below that `testdata` on the path, or the `testdata`
  directory itself for a file directly in it) or reports the path inside a directory it does not
  observe (AFP-V0-191). That is AFP-V0-2440's fixture-only claim for a deleted or moved-away file:
  Go builds and runs nothing below `testdata`, the hosting package's tests read the file as bytes,
  and no unit of the claimant survives in that fixture to have held it. The path stays unowned:
  rule (b) selects the hosting package, rule (c) selects every unit, in any language, whose import
  or path literal names it (AFP-V0-021), and `frontierUnknowns` treats it as any other unowned path
  (AFP-V0-511) although the claimant's `Owns` still claims it. The claimant takes part as the owner
  of a dirty path, and every other plugin's unowned-path checks see the path, so a path-open entry
  of a third plugin, such as a test that reads files by a computed name, still bears. A path
  present on disk, a graph with no disk root, a sparse checkout, a Go claim, a path no plugin
  claims, a `testdata` directory no Go unit hosts, and a fixture subtree the claimant still lists
  keep the entry. Failure mode: a plugin unit in a sibling fixture that loaded the file
  by a name no literal states is not selected. Falsifier: a deleted fixture file whose removal
  changes a test outcome in a unit the plan excludes. Acceptance:
  `TestDeletedFixtureIsFixtureData_AFPV2740`, and
  `TestDeletedFixtureStaysUnownedForOtherPlugins_AFPV2740` for the third plugin's entry (review fix
  round 3). Rollback: drop `deletedFixture` from `admittedDeletion`, which `seed` and
  `participants` share (AFP-V0-2749).
- **AFP-V0-2741:** (experimental under the owner's standing lane auto-accept (2026-11-06); V1-4931;
  CI selector, CIN-V0-860) `gate-affected-select` and `corvint test` SHALL give the same decision
  for a deleted fixture of AFP-V0-2740 because both read the one plan: the plan carries no unknown
  for the path itself (a frontier entry the path bears under AFP-V0-511 is the same unknown in
  both), `corvint test` narrows, and the CI selector's rules (b) and (c) select the hosting
  package and every Go package whose literal names the path, a superset of the plan's Go selection
  as for every dirty path. The CI selector admits no new unknown for it: a plan that still carries
  the entry (an AFP-V0-2740 exclusion) is FULL in both. Acceptance: the beamfall/corvint#114
  replay in build-log entry `2026-11-06-selgo` (both selectors narrow; `cmd/corvint` and
  `internal/command/flows` are selected by both). Rollback: as AFP-V0-2740.
- **AFP-V0-2742:** (experimental under the owner's standing lane auto-accept (2026-11-06); V1-4932;
  amends STR-V0-064 for the `ci` profile; related V1-0872, V1-4363) On the `ci` runner profile a
  well-formed AFP-V0-650 `go:unobserved-module:<directory>:<path>` entry SHALL keep the run at the
  scope the rest of the plan earns, as the deciding driver does for the same entry (CIN-V0-860),
  and SHALL add to `checks` the module's own suite as a check still required and not run
  (`sessiontest.ModuleCheck`: `go -C <directory> test ./...` and the reason). The `ci` profile
  launches no `go-module:` unit, because its confined runner is not qualified for a nested module;
  CI runs each nested module's suite in a job of its own outside the Go shards (`go-static` runs
  the tests and vet of `interop/cem01-go` and `tools/local-authority`, CIN-V0-008, AFP-V0-380). A malformed detail and every other
  unknown keep FULL; the local profile is unchanged. Failure mode: a CI workflow that stopped
  running the module's suite would leave the check unrun; the check line names it. Falsifier: a
  `ci` dry run over a plan whose only non-allowlisted entry is a well-formed module frontier that is
  FULL, or that names no module check. Acceptance: `TestCIProfileNamesTheNestedModuleSuite_AFPV2742`
  and the amended `TestSTRV0064NestedModuleUnit`; the beamfall/corvint#105 replay. Rollback:
  restore the `RunnerProfile != ProfileCI` condition on the module branch of `Build`.
- **AFP-V0-2743:** (experimental under the owner's standing lane auto-accept (2026-11-06); V1-4933;
  amends AFP-V0-152 and AFP-V0-072; V1-1820) A record entry is a sealed CEM directly under
  `.corvint/changes/` or a Markdown file directly under `docs/build-log/`; nearly every change adds
  one of each. For a record entry a Go unit's path token SHALL select it as a literal reader only
  when the token, resolved against the unit's directory, or against the repository root when the
  unit holds a parent-only or `/`-anchored token or locates the root at run time (it is in the
  unbounded-reader closure), names the entry itself, the record directory, or a fragment completed
  at the entry's position. A token that resolves to the repository root or to a proper ancestor of
  the record directory (`.corvint`, `docs`) does not select by itself. AFP-V0-2746 keeps a unit
  whose other literals join it down to the record directory. Otherwise the token reaches the entry
  only through a name computed or walked at run time, which the unbounded-reader rule and declared
  read scopes cover; AFP-V0-2745 and AFP-V0-2747 ratchet every package that names such an
  ancestor. A `/`-anchored token
  is resolved from the root. Build-log entries are thereby narrowed as sealed CEMs already were, so
  a fixture-root token (`docs/build-log/` in a package that never climbs to the root) no longer
  selects its package. Declared read scopes, globs, embeds, unbounded readers and every other path
  keep their rules. Falsifier: a test outside the plan whose outcome a new record entry changes.
  Acceptance: `TestRecordEntriesSelectOnlyTheirReaders_AFPV2743` and `TestRecordDirectory_AFPV2743`.
  Rollback: drop the record branch of `resolves`.
- **AFP-V0-2744:** (experimental under the owner's standing lane auto-accept (2026-11-06); V1-4933;
  CI selector rule (c), CIN-V0-860, AFP-V0-012) `gate-affected-select` SHALL apply AFP-V0-2743 to
  its own rule (c): a record entry's readers are `resolvingReaders`, whose resolution must lie at
  or below the record directory (`resolvesRecord`), in selection and in the `-bounds` output the
  gate ledger keys on (GL-V0-009). Because the CI selector always starts from the plan's Go
  selection, its run stays a superset of `corvint test`'s. Acceptance:
  `TestRecordEntryReadersResolveToTheDirectory_AFPV2744` and the amended
  `TestSelectPackagesNarrowsSealedChangeEvidenceReaders`; the beamfall/corvint#106 replay. Rollback:
  make `resolvedReaders` return `resolvedEvidence` and drop `resolvesRecord`.
- **AFP-V0-2745:** (experimental under the owner's standing lane auto-accept (2026-11-06); V1-4933)
  `make unbounded-readers-check` (`tools/unbounded-readers`) SHALL also fail, exit 1, when a Go test
  package without a declared read scope reads a record directory, by a path literal or glob that
  selects it for a probe entry no literal names (`Graph.RecordReaders`) or by a token that resolves
  to an ancestor of the directory other than the root, and the closed record
  `.corvint/record-readers.json` (`{"profile":"corvint-record-readers/0","directories":
  {DIRECTORY:{PACKAGE:REASON}}}`, DIRECTORY `.corvint/changes/` or `docs/build-log/`, every reason
  non-blank) does not list it. The message names the package and the routes: declare its reads in
  `.corvint/test-read-scopes.json` (with the directory when its tests really read it), mark a
  fixture literal with `//corvint:fixture-path`, or record it with its reason. A listed package that
  no longer reads the directory is reported on stderr without failing, as AFP-V0-025 treats a stale
  entry; a malformed record exits 2; an absent record is the empty set. The record is a ratchet,
  not a selection input. AFP-V0-2747 says how a literal or glob is matched against new entries and
  which packages the record cannot admit. Acceptance: `TestAFPV2745RecordDirectoryReadersRatchet`;
  the repository check passes with the baseline record. Rollback: delete `records.go`, its call in
  `run` and the record.
- **AFP-V0-2746:** (experimental under the owner's standing lane auto-accept (2026-11-06); V1-4933;
  review fix round 1, finding P1; amends AFP-V0-2743 and AFP-V0-2744) For a record entry, the
  planner (a Go unit) and `gate-affected-select` (a package, in selection and in `-bounds`) SHALL
  still select a literal reader whose path token resolves, by the rules of AFP-V0-2743 and
  AFP-V0-2744, to the repository root or to a proper ancestor of the record directory, when every
  component of the record directory below that ancestor is a whole `/`-separated component of one
  of the unit's own tokens. Such separate literals can be joined in any order
  (`os.ReadFile(filepath.Join("../docs", "build-log", name))` in package `reader`) to reach every
  entry of the directory. The literal index does not reconstruct the expression, so any token of the
  unit witnesses a component. A unit whose literals join the ancestor only to another directory
  (`docs/build`, `.corvint/change`) or file stays narrowed. A unit that computes the directory's
  name at run time is a walker, which AFP-V0-2747 refuses to record. Each selector reads its own
  lexicon, so `gate-affected-select`, whose tokens include fixture-marked literals and whose lone
  components compose beside a climbing token, stays a superset of the planner. Failure mode: a
  record directory component spelt only by concatenation (`"build" + kind`) is no whole component
  of any token, so the unit is a walker, not a reader. Falsifier: a test outside the plan that reads
  a new record entry through literals it joins. Acceptance:
  `TestRecordEntryReadersJoinSeparateLiterals_AFPV2746` and
  `TestRecordEntryJoinedReadersSelectedByBothSelectors_AFPV2746` (a fixture whose test really reads
  each entry through `filepath.Join` of separate literals; it fails with the join removed from
  either selector). Rollback: make `reachesRecord` return `resolvesRecord` and drop the `joins` pass
  of `resolvingReaders`.
- **AFP-V0-2747:** (experimental under the owner's standing lane auto-accept (2026-11-06); V1-4933;
  review fix round 1, finding P2; amends AFP-V0-2745) The ratchet SHALL match each literal and glob
  of a package against the record directory's entry namespace, not against one synthetic name. The
  namespace is every name with at least one byte before the entry suffix (`.cem.json`, `.md`). Two
  kinds of package read new entries:
  - one with a glob whose last component holds a wildcard (`*`, `?` or a class) and admits a name of
    the namespace (`2026-*.md`, decided by `globEntry` over `path.Match` syntax);
  - one with a literal whose last component follows the record directory's own last component as a
    fragment that does not end with the entry suffix (`docs/build-log/2026-`).
  A glob or literal that names one entry, by having no wildcard or by ending with the suffix, does
  not read new entries, and neither does a glob whose names cannot end with the suffix
  (`*.json`). `Graph.RecordReaders` takes the record directory and returns the readers and,
  apart, the walkers. A walker names a proper ancestor of the record directory below the root and
  joins no literal that reaches the directory (AFP-V0-2746). It does not locate the root at run time
  and declares no read scope. A walker SHALL fail the check even when the record lists it: no
  selector selects it for a new entry, so a record entry would only hide a reader. Its routes are a
  declared read scope (AFP-V0-023) or the fixture marker. A unit that locates the root and names an
  ancestor is a reader, because the unbounded-reader rule selects it on every change. AFP-V0-2748
  keeps a walker refused whatever narrower literal or glob it also reads by. Acceptance:
  `TestRecordReadersMatchTheEntryNamespace_AFPV2747` (fails when `recordProbes` returns only its
  fixed probe) and `TestAFPV2747RecordRatchetMatchesTheEntryNamespace` (fails with the fixed probe
  alone, and when walkers are returned among readers); the repository check passes with the
  unchanged baseline record. Rollback: return only the fixed probe from `recordProbes` and return
  walkers among the readers.
- **AFP-V0-2748:** (experimental under the owner's standing lane auto-accept (2026-11-06); V1-4933;
  review fix round 2, finding P1; amends AFP-V0-2747) A package whose literal names a proper
  ancestor of a record directory below the root (`../docs`) SHALL stay a walker, and fail the check
  even when the record lists it, unless the reader rules select it for every name of the entry
  namespace. That holds in three cases:
  - a token resolves to the record directory itself, or to the root or a proper ancestor of it from
    which the package's literals join the rest of the way (AFP-V0-2746);
  - a glob matches the directory or one of its ancestors, or matches the directory with its leading
    components and admits every name of the namespace with its last one (`*`, `*.md`, `*.json` for
    `.cem.json`), as `globCovers` decides over `path.Match` syntax;
  - the package locates the root at run time (AFP-V0-2747).
  A reader match through a narrower literal or glob never masks the walker. Such a match is a
  fragment (`docs/build-log/f`), an entry name, or a glob constrained in the name
  (`../docs/*/2026-*.md`, `[0-9a-f]*.cem.json`), and it selects the package for some entries only.
  A walk of `../docs` still reads `docs/build-log/2027-entry.md`, for which no selector selects the
  package. `gate-affected-select` has no glob reader and no walker classification, so the ratchet
  decides this once for both selectors, neither of which selects such a package for that entry.
  `globCovers` searches every rune class its pattern tells apart; to that end it adds each class
  bound and the runes beside it to `globEntry`'s alphabet, which `globEntry` then uses too.
  Failure mode: a package that walks `docs` and globs only some of its entries can no longer be
  recorded; it needs a declared read scope (AFP-V0-023) or the fixture marker. Falsifier: a package
  the check admits that names an ancestor of a record directory and that no selector selects for
  some new entry. Acceptance: `TestRecordWalkerNotMaskedByNarrowerReads_AFPV2748` (fails when a
  narrower read masks the walker), `TestAFPV2748RecordRatchetRefusesAMaskedWalker` (real Go source
  that walks `../docs` and calls `filepath.Glob("../docs/*/2026-*.md")`, listed in the record; it
  passes the check without the fix) and `TestRecordWalkerWithGlobSkippedByBothSelectors_AFPV2748`
  (neither selector selects that package for `docs/build-log/2027-entry.md`, and it is a walker);
  the repository check passes with the unchanged baseline record. Rollback: classify a walker with
  any reader match as a reader again, dropping `coversRecord`, `globCoversRecord` and `globCovers`.
- **AFP-V0-2749:** (experimental under the owner's standing lane auto-accept (2026-11-06); V1-4931;
  review fix round 4, finding P1; extends AFP-V0-2740's unowned-path rule to AFP-V0-2610 and
  AFP-V0-821) Every dirty path the plan admits as a deletion without `UNINDEXED_SOURCE_PATH`
  although a plugin's `Owns` claims it SHALL stay unowned for `frontierUnknowns` (AFP-V0-511): a
  deleted Go source or `_test.go` file of a surviving package (AFP-V0-620, AFP-V0-2610), a file of a
  wholly deleted Go package (AFP-V0-821), and a deleted fixture (AFP-V0-2740). `seed` and
  `participants` decide this through one predicate, `admittedDeletion`, so no path is admitted by
  one and left Go-owned by the other. The claimant still takes part as the owner of a dirty path.
  Every other plugin's unowned-path checks see the path, so a path-open entry of a plugin that
  neither owns a dirty path nor reaches a changed unit, such as a Ruby spec that reads repository
  files by a computed name, still bears with a hint naming the deleted path, as it did through the
  `UNINDEXED_SOURCE_PATH` fallback before the deletion was admitted. Source-bound entries still do
  not bear (a Go path is no Ruby source), and an edit to a file a unit still declares is owned, so
  this adds no entry to an ordinary edit. The extra entries are `LANGUAGE_FRONTIER` unknowns of
  non-Go plugins; both selectors add each one's language suite to execution (AFP-V0-2750), so
  `corvint test` and `gate-affected-select` keep giving the same decision. Failure mode: a deleted Go path now
  makes the plan `UNKNOWN` wherever another plugin has a path-open frontier entry; the selection is
  unchanged. Falsifier: a Go deletion `seed` admits that `participants` does not list among the
  unowned paths, or a path-open entry of another plugin that does not bear on it. Acceptance:
  `TestDeletedGoSourceStaysUnownedForOtherPlugins_AFPV2749` (a Ruby computed-name reader bears on a
  deleted source, a deleted `_test.go` file and a wholly deleted package, and not on an edited
  declared source; fails with `participants` testing only `deletedFixture`) and
  `TestAdmittedDeletionsStayUnownedInParticipants_AFPV2749` (`participants` lists exactly what
  `admittedDeletion` admits on a disk checkout, a sparse checkout and a graph with no disk root,
  and `seed` raises no entry for those paths). Rollback: in `participants`, call `deletedFixture`
  in place of `admittedDeletion`; deleted Go paths are Go-owned for the frontier checks again.
- **AFP-V0-2750:** (experimental under the owner's standing lane auto-accept (2026-11-06); V1-4931;
  review fix round 5, finding P1; the orchestrator's fail-closed decision; amends AFP-V0-982, the
  STR-V0-004 language set of `corvint test` and the CIN-V0-860 audit lines) Every
  `LANGUAGE_FRONTIER` the plan names bears on the change: `frontierUnknowns` names an entry only
  through a participating plugin, a reached or narrowed unit, a dirty path no plugin owns, or a
  cross-language entry of a terminal plugin, and a clean plan names none. So no non-Go frontier is
  unrelated to a changed path, and the blanket non-Go allowance SHALL be removed from
  `gateselect.AllowedUnknown`. Its `LANGUAGE_FRONTIER` case SHALL admit only
  `go:build-constraint-variants`, `go:nested-module-frontier` and a `driver:` frontier: driver units
  are make targets and CI jobs that no adapter or declared command runs, so neither a narrowed nor a
  FULL run has a suite to add for them (the `NO_SELECTABLE_TEST` rule already excludes the driver
  namespace, AZU-V0-9704). Every other `LANGUAGE_FRONTIER` whose detail is `<language>:...` with a
  plain lowercase language name and no `go:` prefix SHALL be read by `gateselect.FrontierSuite`
  for both selectors and SHALL add that language's declared suite to execution without widening Go
  selection:
  - `corvint test` (`sessiontest.Build`) adds the language to the set it runs through
    `declaredLanguage`, as it does for a `NO_SELECTABLE_TEST` unit: outside the `ci` profile the
    declared full command runs as `declared:<language>`; on the `ci` profile it is named as a check
    still required; a declared `none` records the project's no-suite assertion; and a language the
    declaration does not name leaves the run `INCOMPLETE` (STR-V0-004). The run stays `NARROWED`.
  - `gate-affected-select` (`gateselect.Select`) admits the entry and writes one
    `suite <language> <- <detail>` audit line per language, naming the first such entry, before
    the verdict; the Go verdict is unchanged. The CI PR tier runs non-Go suites outside the Go
    shards, and this requirement does not change those jobs: `host-adapters` runs the declared
    TypeScript family on every pull request, `shell-test-selected` keeps its own shell rule, and a
    language the declaration marks `none` has no suite.
  A detail with no language prefix, an empty detail, or a malformed language name is outside both
  rules and still forces FULL. `tools/shell-test-select` keeps its own shell rule and admits the
  other non-Go frontiers it admitted before. The repository declares `cfamily` as `none` in
  `.github/cishards/test-commands.json`, since its only C sources are fixtures, so a deleted Go
  source that raises a `cfamily` entry stays complete here; `tools/test-commands-check` names the
  C and C++ runners it looks for in `make gate` to hold that assertion (STR-V0-006). A historic
  tree still has no `cfamily` declaration, so `tools/selection-baseline` reads an `INCOMPLETE`
  `corvint test` dry run (exit 3 with its manifest written) as that verdict, not a failed replay,
  and records its reasons as `testIncomplete`. Failure mode: a narrowed run now carries
  each bearing non-Go language's declared suite, or its no-suite assertion; a plan naming a
  language the declaration omits is `INCOMPLETE` locally and FULL in CI through the AFP-V0-098
  shadow check. Falsifier: a plan with a non-Go, non-driver frontier whose `corvint test` manifest
  omits that language, or whose `gate-affected-select` output names no `suite` line for it.
  Acceptance: `TestFrontierSuiteRunsInCorvintTest_AFPV2750` and
  `TestFrontierSuiteNamedByGateAffectedSelect_AFPV2750` in `internal/sessiontest` (a declared Ruby
  suite whose computed reader bears on a deleted Go source runs as `declared:ruby` in a `NARROWED`
  manifest, is a check on the `ci` profile, and is named by a `suite ruby` line in
  `gate-affected-select` output with the Go verdict unchanged; both fail with the blanket allowance
  restored) and the updated `TestDriverAndShadowShareOneUnknownAllowlist_AFPV0982`; the
  replays in build-log entry `2026-11-07-selgo-fix5`. Rollback: restore the non-Go clause of
  `AllowedUnknown`, delete `FrontierSuite` and its calls, and drop the `cfamily` declaration.

### Root locators and runtime sources are not reads (V1-4283)

Measured on 2026-11-06 at `4052b8540b5fff818a819db37b55371ea5428c2c`: a one-line edit of
`docs/TICKET-PRIORITY.md` selected 117 `UNBOUNDED_READER` units under 30 causes: 53 through a
`package main` unit's non-test root locator (`cmd/corvint`, `benchmarks/runner`,
`cmd/corvint-tasks`, `tools/gate-ledger`, `tools/test-commands-run`, `tools/cem-stable-regen`), 2
through a prose literal (`internal/perfbudget`), 39 through two shell scripts the plugin marked
`shell:dynamic-source` (`script/heavy.sh`, `script/dogfood-finish.sh`) and 3 toolchain-running
shell tests with no declared read scope. None of those units reads the edited file. The
requirements below make AFP-V0-012 rule (d) and the shell plugin's source classification state
what is read, and keep every unit whose reads are genuinely unbounded, with a witness that says why;
the before and after plans are in `docs/build-log/2026-11-06-unbounded-readers.md`. The independent
review of 2026-11-07 found four of the first versions too broad (a joined path a later
`filepath.Dir` widens, a stop at every command, a line break taken as proof of text, an
environment-named source taken as external); the requirements below are the corrected rules, each
of which keeps the broader selection where its proof does not hold
(`docs/build-log/2026-11-07-unbounded-readers-review-fix.md`). The second review of 2026-11-07
found two more gaps: a `go list` whose output a later command runs (`go list ./lib | xargs -I PKG
go test PKG`) was a build-only edge, and a standard stream the program itself redirects to a pipe
made printed text a path it opens. AFP-V0-2956 to AFP-V0-2960 close them by keeping the
uncertainty (`docs/build-log/2026-11-07-unbounded-readers-review-fix-2.md`). The third review of
2026-11-07 found that a helper package imported by the command's test could still capture its
stream. A printed literal can be read back anywhere in the process, by the Go test runtime, or by
any process that runs the command, so AFP-V0-2962 withdraws the printed-text exemption rather than
narrow it a third time (`docs/build-log/2026-11-07-unbounded-readers-review-fix-3.md`).

- **AFP-V0-2950:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4283;
  amends AFP-V0-012 rule (d) and AZU-V0-9930; review fix 2026-11-07) Locating the working directory
  is not a read. An `os.Getwd` call (under the local name the file's import gives `os`) whose two
  results are bound by one `:=` or `=` statement to a local variable and a local variable or blank
  SHALL NOT make its package an unbounded reader only after every initializer and assignment is
  certified: the closed allowlist is a two-result call of the imported `os.Getwd`, a single local
  string declaration with no initializer, or a single local declaration initialized to the empty
  string literal. Arbitrary calls, parameters, other literals, unresolved receivers and all other
  write forms retain the root-locator claim, even before a conditional fallback. Every other use
  of the path variable must be (a) an
  operand of `==` or `!=` against a string literal, or (b) the first argument of a `filepath.Join`
  or `path.Join` call (under the local name the file's import gives `path/filepath` or `path`) whose
  every other argument is a string literal whose path components are plain names, not empty, `.`,
  `..` or a glob, and whose result only a read at or below the joined path consumes: the join is
  the one argument of `os.ReadFile`, `os.Stat`, `os.Lstat` or `os.ReadDir`, or the value of a `:=`
  or `var` binding of one local variable, never rebound or addressed, whose every use is such an
  argument or an operand of `==` or `!=` against a string literal (`boundedJoinConsumed`). The
  joined path becomes a root-anchored rule (c) token of the package (`/docs/guide.md`), so a change
  to the file it names (or below a directory it names) selects the package as
  `PATH_LITERAL_READER`; a change elsewhere does not. Any other use of the root (a return, an
  assignment to another variable, a struct field, an argument to any other function,
  `filepath.Dir`, a `Join` with a computed, empty, `.` or `..` argument) and any other consumer of
  the joined path (`filepath.Dir(p)`, a further join, a return, `os.Open`, whose file reports the
  path, a walk, whose callback receives it, or any other call) keeps the package a root locator
  exactly as before, as does a `--show-toplevel` literal: the subprocess that answers it reads the
  repository's Git metadata, and its output flows through text the lexical pass cannot follow. The
  planner's Go plugin (`comparisonOnlyLocators` in
  `internal/liveverify/affected/golang/locatorproof.go`) and the CI selector
  (`internal/gateselect/locatorproof.go`) SHALL apply the same proof and agree on every fixture case.
- **AFP-V0-2951:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4283;
  amends AFP-V0-012 rule (d); review fix 2026-11-07) A rule (d) chain from a root locator SHALL NOT
  follow a build-only edge, and SHALL follow every other edge. A build-only edge runs from a driver
  unit (a make target, CI job or action) to a unit whose owned paths the driver names only as the
  package patterns of `go vet` or `go list` whose standard output feeds no other command and whose
  arguments no feeder supplies (AFP-V0-2956, AFP-V0-2957; the unit's `buildPathImports`) and by no other path
  import, declared import or declared caller (TSP-V0-005): those commands read the packages without
  running any of their code or leaving a binary a later step could run, so the driver reads nothing
  a root locator in them would. `go build`, `go install`, `go run`, `go test`, `go generate` and
  every other path import stay execution edges: a later step, or a target that depends on this
  one, may run what they build. A Go unit whose non-test files declare `package main` and locate the
  root SHALL stay `locatesRoot`, so every unit that runs the command through an execution edge is
  an unbounded reader with the command as its cause. A build-only edge is still a dependency edge:
  a change to a vetted or listed unit selects the driver as before. The first version of this
  requirement stopped every chain at a command; a target that runs a repository-walking command
  over the live checkout was then selected only when its recipe or the command's code changed,
  which the review rejected. The CI selector was unchanged here: its rule (d) closure followed Go
  import edges only (AFP-V0-464) until AFP-V0-2994 added execution edges, and this requirement's
  Go-side claim held only for driver edges until AFP-V0-2990.
- **AFP-V0-2952:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4283;
  amends AFP-V0-012 rule (d); review fix 2026-11-07; superseded by AFP-V0-2962, review fix 3,
  2026-11-07) Superseded by AFP-V0-2962. This requirement exempted a `..`-climbing token that a
  file held only inside line-broken literals, each reaching only a standard stream (`fmt.Print`,
  `fmt.Printf` or `fmt.Println`; `fmt.Fprint*` or `io.WriteString` to `os.Stdout` or `os.Stderr`;
  `os.Stdout.WriteString`), as text the file prints, not a path it opens. The three reviews found
  three ways to read such text back and open it: a formatted value, a pipe in the same package and
  a helper imported into the process. Printed text also reaches every process that runs the
  command. No repository unit used the exemption.
- **AFP-V0-2953:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4283;
  review fix 2026-11-07) The shell plugin SHALL keep `shell:dynamic-source` for a `source` or `.`
  whose operand is an expansion of a name the environment gives (`$NAME`, `${NAME}`, or `${NAME`
  followed by `/`, `#` or `%` and a pattern), whether or not the file ever assigns NAME: a file
  that gives the name no value proves nothing about where its caller's value points, and a caller
  that names a repository file (`ENVFILE=lib.sh`) makes the sourcing file read it. Only a literal
  link (NSA-V0-008, AFP-V0-2954) or a TSP-V0-004 fresh-directory source bounds a source. The first
  version of this requirement admitted such a source when the file never assigned the name and
  sourced no repository file; the review rejected it, so `script/heavy.sh`, whose remote half
  sources `"${ENVFILE/#\~/$HOME}"`, stays a dynamic source, and so does every unit that runs it.
- **AFP-V0-2954:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4283;
  amends NSA-V0-008) A variable that leads a `source`, `.`, `load` or run target SHALL resolve as a
  directory form when it is assigned more than once only if every assignment is a plain
  `NAME=VALUE` word with byte-identical VALUE text and the file never otherwise declares, reads into,
  appends to or iterates the name: whichever assignment ran, the value is the same. A second
  assignment with different text (`DIR=$(dirname "$0")` then `DIR=/tmp`), a `local`, `read`, `for`
  or `+=` form, or a `${NAME=...}` default keeps the target unresolved, as before. TSP-V0-004's
  fresh-directory proof still requires exactly one assignment.
- **AFP-V0-2955:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4283;
  review fix 2026-11-07) The repository SHALL declare in `.corvint/shell-read-scopes.json`
  (TSP-V0-950) the repository paths `script/qualify-tasks-basic_test.sh` (`cmd/corvint-tasks/`,
  `go.mod`, `go.sum`, `internal/`) reads outside `script/`, with the reason its toolchain run needs
  them, and `.corvint/unbounded-readers.json` SHALL list exactly the units the corrected
  AFP-V0-2950 to AFP-V0-2954 leave on the floor, each with its reason.
  `script/dogfood-cache_test.sh` declares no scope: it names the dogfood wrappers, whose chain
  reaches `script/heavy.sh`'s environment-named source (AFP-V0-2953), and TSP-V0-950 refuses a
  declaration over a test that reaches a dynamic source. `script/lane-scripts_test.sh` stays recorded:
  it sources a `sed`-filtered copy of `script/heavy.sh` it writes into its temporary directory,
  a read of its own directory that no proof links (TSP-V0-004 admits no write between the
  `mktemp` and the source), so TSP-V0-950 refuses a declaration over it. `script/heavy.sh` SHALL
  keep its remote half provable under TSP-V0-004 (the payload directory variable is assigned once
  and no other function or embedded program reuses its name) and its toolchain probe a literal
  binary name (`"$sdk/go"`), so the shell plugin links or bounds every source and run target it
  holds but the environment-named source AFP-V0-2953 keeps dynamic.
- **AFP-V0-2956:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4283;
  review fix 2, 2026-11-07; amends AFP-V0-2951) A `go vet` or `go list` whose standard output feeds
  another command SHALL record its package patterns as path imports (execution edges), not build
  path imports: its output feeds a command when a pipe (`|` or `|&`, not `||`) follows it, or a
  command substitution (`$(...)`, backticks), a process substitution (`<(...)`, `>(...)`) or a make
  `$(shell ...)` encloses it, at any nesting (a `sh -c` script inside such a context inherits it).
  The consumer may run what the command lists: `go list ./lib | xargs -I PKG go test PKG` runs
  lib's tests, so a root locator in lib reaches the driver. Buffered listing output can also be
  executed as a named program (`go list -f 'go test {{.ImportPath}}' ./lib > /tmp/check; sh /tmp/check`).
  A listing without a known direct consumer keeps build-only classification only for the closed
  output allowlist: default package paths, or the exact single-field `{{.Dir}}` and `{{.ImportPath}}`
  formats. The understood options are `f`, `C`, `tags`, `mod`, `modfile`, `overlay`, `compiler`,
  `gcflags`, `asmflags`, `ldflags`, `pkgdir`, `e`, `deps`, `test`, `find`, `race`, `msan`, `asan`,
  `trimpath`, `modcacherw` and `buildvcs`. Any other option or output format retains execution edges
  to every possible owner and marks the invocation computed: an emitted program may execute a
  package other than the listed input
  (`go list -f 'go test ./{{printf "%s%s" "rea" "der"}}' ./lib` runs `reader`, not `lib`).
  Do not infer that a later file consumer must be fed or expanded. This is conservative over
  arbitrary templates whether or not the scanner sees their eventual consumer. The shell scanner
  SHALL NOT take the word after `<(` or `>(` as a redirect target.
- **AFP-V0-2957:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4283;
  review fix 2, 2026-11-07) A command that `xargs`, `parallel`, or `find`'s `-exec`, `-execdir`,
  `-ok` or `-okdir` runs takes arguments the scanner cannot read. A go package command it runs
  SHALL import every owner (the pattern `...`, as an expanded pattern does; a build-only edge for
  `go vet` and `go list` unless AFP-V0-2956 applies); a make call it runs SHALL be computed (every
  target of its make file); and an interpreter it runs, a command word holding the replacement
  string that `-I`, `-J`, `-i` or `--replace` names, or a `{}` command word after `find -exec` (any
  `{` word after `parallel`) SHALL mark the site a computed invocation. An xargs option missing its
  value keeps the site computed (AZU-V0-001).
- **AFP-V0-2958:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4283;
  review fix 2, 2026-11-07) A shell or interpreter in the driver scanner's interpreter list that
  reads its program from standard input SHALL mark the site a computed invocation: one with no
  script operand and no inline program, a shell given `-s` (alone or in an option cluster) or `-`,
  as in `... | sh`, `bash -s` and a heredoc-fed `bash <<EOF`, whose body the scanner skips. A shell's
  `-o NAME` and `+o NAME` take their value; `+x`-style options are options, not the script.
- **AFP-V0-2959:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4283;
  review fix 2, 2026-11-07; superseded by AFP-V0-2962, review fix 3, 2026-11-07) Superseded by
  AFP-V0-2962. This requirement limited the AFP-V0-2952 exemption to a `package main` file, on the
  claim that a command's directory is all the code that shares its process. The claim was wrong:
  the packages the command's tests import run in the same test process.
- **AFP-V0-2960:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4283;
  review fix 2, 2026-11-07; superseded by AFP-V0-2962, review fix 3, 2026-11-07) Superseded by
  AFP-V0-2962. This requirement further required that no Go file of the command's directory give
  evidence of capturing a standard stream (a `Pipe`, `Dup2`, `NewFile` or `Output` identifier, an
  assignment to `os.Stdout`, a `//go:linkname`). The third review's imported helper
  (`capture.Read(main)`) carries that evidence outside the directory, so the check never saw it.
- **AFP-V0-2962:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4283;
  review fix 3, 2026-11-07; amends AFP-V0-012 rule (d); supersedes AFP-V0-2952, AFP-V0-2959 and
  AFP-V0-2960) No output proof SHALL exempt a `..`-climbing token from rule (d): the token gives
  the reason and, in a non-test file, the `locatesRoot` it gave before AFP-V0-2952, whatever the
  literal holding it reaches (printed or stored, line-broken or not, in a command or a library).
  Every unit that runs such a command through an execution edge is then an unbounded reader
  (AFP-V0-2951). A standard stream feeds code neither analyzer can bound. (1) Any package in the
  import closure of the command or of its test packages shares the process and can redirect the
  stream: the third review's helper reads the command's `"../config%s\n"` back through
  `capture.Read(main)` and opens `../config.go`. (2) The standard library: `fmt` imports `os`, which
  defines `Pipe` and `NewFile`, and every test binary links `testing`, which sets `os.Stdout` to an
  `os.Pipe` writer to check examples. Under the AFP-V0-2960 evidence rule every command's closure
  therefore carries capture evidence, and a closure proof would exempt nothing without a
  per-release list of standard packages proven inert. The CI selector reads only repository
  sources and cannot inspect a module or standard package at all. (3) Any process that runs the
  command, such as a make target `cd quiet && cat "$(go run .)"` or a test that runs the binary and
  opens what it printed. No import closure includes that process. A sound exemption would have to
  bound every reader of the stream, so it is withdrawn instead. Over-selection is the accepted cost;
  no repository unit used the exemption when it was withdrawn. The planner's Go plugin and the CI
  selector SHALL apply the same rule and agree on every fixture case, and the selector's scan memo
  SHALL carry no per-file exemption state (`textOnly`, `main`, `captures`), so a warm scan agrees
  with a cold one.

### Rule (d) and runtime traversals follow execution edges (V1-4983)

The fix-only review of 2026-11-07 found that rule (d) skipped Go binary consumers (V1-4983):
`Graph.unboundedReaders` followed import edges and the drivers' path imports, but no `execs` edge,
and the binary-consumer step ran before the unbounded readers were added. A command that prints
`"../config%s\n"` keeps rule (d) (AFP-V0-2962), yet a test in another package that ran it with
`exec.Command("go", "run", ".../tool")`, trimmed the output and opened that path was not selected
by an edit of `config.go`; round 3 found the same gap with an `os.Getwd` command and an unrelated
edit. AFP-V0-464 assumed that such a runner is its own unbounded reader because it runs `go`; no
scan marks that. The requirements below follow execution edges wherever a traversal follows code,
keep a test-only run one level past what it runs, as a test-only import is, and measure the cost
in `docs/build-log/2026-11-07-binary-runners.md`.

- **AFP-V0-2990:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4983)
  Amending AFP-V0-012 rule (d), AFP-V0-464 and AFP-V0-2951, a Go unit that runs the built binary of
  a unit the rule (d) closure walks SHALL be an unbounded reader whose cause is the closure's root
  locator, selected on any dirty path with an `UNBOUNDED_READER` witness, unless its own declared
  read scope bounds it (AFP-V0-023). The run may come through an `execs` edge from a path literal
  (`go run`, `go build`, a binary path), from `.corvint/test-binary-execs.json` or from a declared
  runtime edge (AFP-V0-819). In the review's case, where `tool` prints `"../config%s\n"` and a
  separate package's test runs it with `go run`, trims the output and opens that path, an edit of
  `config.go` SHALL select the test's package as well as `tool`. In round 3's case, where `tool`
  calls `os.Getwd` and hands the root on and another package's test runs it, an unrelated edit
  SHALL select that package.
- **AFP-V0-2991:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4983)
  The Go plugin SHALL record a unit's `testOnlyExecs`: the members of `execs` that only its
  `_test.go` files use. The sources are searched first, so a command a non-test file uses is never
  test-only, and an unparsed source uses every candidate. A `.corvint/test-binary-execs.json` edge
  is never test-only. A declared runtime edge that the unit's code does not already run is
  test-only, because a test file holds it. A unit whose `testOnlyExecs` is not a subset of `execs`
  is invalid; the member is part of the AFP-V0-005 projection when non-empty. The rule (d) closure
  SHALL follow a code-run edge (an `execs` member not in `testOnlyExecs`) as it follows an import,
  so the runner's importers and code runners follow transitively. It SHALL add a unit whose tests
  alone run a walked unit's binary one level past that unit, as it adds a test user, and never
  walk past it. A unit that rule (d) reaches only that way reads nothing unbounded with its own
  code. For AFP-V0-560 plugin participation it is a terminal selection, as a `BINARY_EXEC` runner
  is, so a Playwright spec that runs a root-locating Go command through a declared runtime edge
  brings no TypeScript frontier into an unrelated plan.
- **AFP-V0-2992:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4983)
  Amending AFP-V0-495, the traversal from a test-less reader of an unowned path SHALL follow the
  same edges: it walks code runners like importers and adds test-only runners one level past every
  walked unit. A test-less command's runners are the only checks of its read, as a test-less
  library's importers are. That traversal SHALL NOT select a unit whose declared read scope
  excludes the path (AFP-V0-161): its tests read nothing there, whatever code or binary they run.
  The walk still passes through the unit.
- **AFP-V0-2993:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4983)
  Amending AFP-V0-037, the dependency traversal of rules (a) and (b) SHALL walk a code-run edge as
  it walks an import edge. A unit whose code may run the binary of a command the traversal reaches
  is reached with a `BINARY_EXEC` witness, and its importers and code runners are reached from it,
  so its declared binary consumers (AFP-V0-762) and its frontier (AFP-V0-581) bear as they do for
  any traversed unit. A unit whose tests alone run the binary stays selected one edge past the
  command and is not expanded. The CI selector's rules (a) and (b) already follow transitively a
  non-test literal that names a package directory (`namers`, AFP-V0-464). Before this requirement,
  `corvint affected` and `corvint test` stopped one edge past a rebuilt command and could select
  fewer packages than CI.
- **AFP-V0-2994:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4983)
  The CI selector (`internal/gateselect`) SHALL apply the rule (d) closure of AFP-V0-2990 and
  AFP-V0-2991 over the execution edges its literal index can see. A directory is a command when a
  non-test file declares `package main`. A package that holds a path token able to name a command
  directory runs that command: from its code when a non-test file holds the token, from its tests
  alone otherwise. The token names the directory as the plugin's `commandDirectory` resolves it,
  or through any of its component runs. Each `.corvint/test-binary-execs.json` entry is a code run.
  A declaration the selector cannot apply (unreadable, oversized, malformed, or with a member that
  names no package or no command) makes every package a rule (d) unit. The selector cannot follow
  a literal to its use as the plugin does, so its set may exceed the planner's. It SHALL hold every
  planner rule (d) unit on every fixture case, and the parity tests SHALL hold the two sets equal
  where the plugin's position rule changes nothing. The scan memo SHALL carry each file's
  `package main` flag (`command`). Like the file's imports, the flag is a function of its bytes, not
  exemption state, so AFP-V0-2962's memo clause holds and a warm scan agrees with a cold one.
- **AFP-V0-2995:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4983)
  `.corvint/unbounded-readers.json` SHALL list the 24 units that AFP-V0-2990 to AFP-V0-2992 add to
  the floor, each with its chain and cause. Nineteen of them reach the floor through
  `cmd/corvint-go-test-provider`, whose import of `internal/liveverify/parentverify` locates the
  root, and a directory literal of the language manifests in `internal/langplugin` and
  `internal/langplugin/decl`. Neither analyzer can prove that literal never reaches a process. That
  one edge class accounts for nearly all of the measured widening (`docs/build-log/2026-11-07-binary-runners.md`).
  Removing it needs a precision change or an owner declaration, not a weaker rule.
  `TestPrecisionReplay_AFPV09003` keeps its bound of 86 on the heavy.sh-only plan's units beyond
  the rule (d) floor, because this record holds the floor unit by unit (AFP-V0-025).

### Additive narrowing follows every traversal class (V1-4983)

The fix-only review of 2026-11-07 (round 5) found that additive narrowing missed the code-run class
that AFP-V0-2993 added to the dependency walk. `traverseWhere` walks four edge classes, but
`reachedThrough`, which demotes a `NARROWED` record whose package the walk reaches from another
changed unit (ADN-V0-004), checked only the three import classes. Take a change that edits a
command that locates nothing and adds an unused exported function to a library whose existing
`Run` executes that command. The library kept its `NARROWED` record, and an importer that calls
`Run` but names no new declaration was excluded, although its tests observe the changed command.
The requirements below close the gap in both analyzers. They derive the walk and the check from
one list, so a later class cannot reopen it.

- **AFP-V0-3140:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4983)
  Applying ADN-V0-004's walk clause to AFP-V0-2993's code-run class, a `NARROWED` record whose
  package the dependency walk reaches by a code-run edge from another reached unit SHALL fall back with `REACHED_THROUGH_ANOTHER_CHANGED_UNIT` naming
  that unit, as it does for an import, advisory or route-dispatch edge, and the plan SHALL follow
  the package's importers. In the review's case, `harness`'s code runs `quiet` through
  `exec.Command("go", "run", ...)` or a `.corvint/test-binary-execs.json` entry. An edit of `quiet`
  together with an unused `Fresh` added to `harness` SHALL select `consumer`, which calls
  `harness.Run`.
- **AFP-V0-3141:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4983)
  A record whose package no followed edge reaches SHALL keep its narrowing. A unit whose tests alone
  run a changed command is added one level past the walk (`execUsersOf`), never through a followed
  edge. So when only `harness`'s tests run `quiet`, a change of `quiet` together with `harness`'s
  additive change keeps the record `NARROWED` and leaves `consumer` out, as `harness`'s additive
  change alone does.
- **AFP-V0-3142:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4983)
  The edge classes the dependency walk follows SHALL be one list (`traversalEdges`): imports,
  advisory imports, route-dispatch imports and code runs. `traverseWhere` and `reachedThrough` SHALL
  both take their classes from it, so a class that one follows cannot be missing from the other. A
  test SHALL add, for each listed class, one edge from a changed unit to a narrowed package. It SHALL
  require the fall-back and the selection of every importer and test user the record would exclude.
  It SHALL fail when the list changes length, until this row and the test name the new class.
- **AFP-V0-3143:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4983)
  Amending AFP-V0-2993's CI-selector clause and ADN-V0-006, the CI selector's rules (a) and (b)
  SHALL walk a command's code runners (AFP-V0-2994's `codeRunners`, declared runs included) as they
  walk its importers and namers. `attributeNarrowed` SHALL start from a narrowed package's code
  runners as well as its referencing importers and namers, because a record prunes import edges
  only, as the planner's `follows` does. The plugin never narrows a `main` package (ADN-V0-004), so
  that start matters for a stale or forged receipt, which the selector does not trust to prove it.
  Before this requirement, a run declared in
  `.corvint/test-binary-execs.json` was no namer edge. Once the runner's own path was narrowed, a
  changed command therefore missed the runner's importers. On the review's case, with the record
  `NARROWED` as the planner emitted it before AFP-V0-3140, `SelectStructural` SHALL select every
  package the planner selects. `Select` keeps the whole attribution and takes the same walk, which
  can only add packages.
### Shell run targets and conditionals keep their commands (V1-4283)

- **AFP-V0-3144:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4283;
  review fix 6, 2026-11-07; amends TSP-V0-960 in `docs/specs/test-selection-precision-v0.md`) The
  shell plugin MAY leave a computed run target or interpreter operand (TSP-V0-960) unexpanded when
  its file name is literal (no `$`, backquote or brace), carries no owned shell suffix, and is the
  file name of no shell unit's file (an owned extensionless file with no shell shebang has no unit,
  and an edit of it is `UNINDEXED_SOURCE_PATH`, AFP-V0-2205). No value of such a target can link an
  edge or build a script name, so a 64-value bound that cuts its values drops nothing, and the unit
  SHALL NOT become `shell:dynamic-source` for it; `script/heavy.sh`'s toolchain probe `"$sdk/go"` is
  one (AFP-V0-2955). A target whose file name an owned extensionless script has (AFP-V0-2205), such
  as `"$dir/check"` beside `script/check`, SHALL be expanded as TSP-V0-960 requires: each fully
  literal value that names the script is an import edge, so `for dir in script; do "$dir/check";
  done` imports `shell:script/check` and an edit of `script/check` selects the tests that reach the
  caller, and a value bound that cuts its values makes the unit `shell:dynamic-source`.
- **AFP-V0-3145:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4283;
  review fix 6, 2026-11-07; amends NSA-V0-008 in `docs/specs/native-shell-affected-test-v0.md`) The
  shell lexer SHALL take `[[` as opening a conditional expression, whose `&&`, `||`, `|`, `<`, `>`
  and parentheses belong to the expression and not to the command list, only in command position:
  the first word of a command, or a word that only the reserved words `if`, `while`, `until`, `!`,
  `time`, `then`, `do`, `else` and `elif` precede in it. A `;`, `&&`, `||`, `|`, newline, `{`, `(`,
  case arm or command substitution starts a new command. The expression ends at the first `]]`
  after the `[[`. A `[[` in any other position is an argument (`echo [[ marker && . "$ENVFILE"`):
  the operator after it SHALL still end the command, so the next command is scanned, an
  environment-named source there keeps `shell:dynamic-source` (AFP-V0-2953) and a literal source
  keeps its edge.
- **AFP-V0-3146:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4283;
  review fix 7, 2026-11-07; amends TSP-V0-960 in `docs/specs/test-selection-precision-v0.md`) A
  name that a file gives more than 16 values (TSP-V0-960's per-name bound) SHALL be a cut, as the
  64-value bound is: the expansion stands the unknown part in for every value the file gives, so the
  unit SHALL set `UnboundedReads` to `shell:dynamic-source` (a non-test unit also `LocatesRoot`)
  rather than read the unknown part as an environment input. A test that runs `"$dir/check"` from
  `for dir in a b c d e f g h i j k l m n o p script` beside an owned extensionless `script/check`
  (AFP-V0-3144) SHALL then be selected by an edit of `script/check` and by an edit any dynamic
  source `script/check` holds may read, such as a configuration file it sources through
  `. "$ENVFILE"`. A name given 16 values or fewer is expanded and links its values as before.
- **AFP-V0-3147:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4283;
  review fix 7, 2026-11-07; amends TSP-V0-960) Every other bound of that expansion that drops values
  the file gives SHALL be a cut in the same way: depth 4 reached through a variable chain, a nested
  `-`, `:-`, `=` or `:=` default, or a `+` or `:+` alternative, and the per-name or 64-value bound
  reached while expanding a command string held in a variable (`sh -c "$cmd"`), whose dropped strings
  may run scripts. An unknown part that drops nothing the file gives (a name the file never sets, a
  positional or special parameter, a command substitution, an operator the model does not follow, an
  empty `:+` alternative) is no cut and keeps TSP-V0-960's treatment. A name whose value refers to
  itself reaches depth 4 through that cycle and is a cut too, unless AFP-V0-3149 unrolls it.
- **AFP-V0-3148:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4283;
  review fix 7, 2026-11-07; refines TSP-V0-960) The per-name bound SHALL count distinct values: a
  value the file gives a name more than once, verbatim (the same word list in several `for` loops),
  expands to the same values each time and SHALL count once, so a name given 16 or fewer distinct
  values stays expanded and bounded however often it repeats them. Seventeen distinct values remain
  a cut (AFP-V0-3146).
- **AFP-V0-3149:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4283;
  review fix 7, 2026-11-07; refines TSP-V0-960 and AFP-V0-3147) A value of a name that refers to that
  name SHALL be expanded once, with no cut, only when every self-referential assignment
  belongs to this closed allowlist:
  - the plain identity `$name` or `${name}`;
  - a standalone `-`, `:-`, `=` or `:=` default with a literal operand containing no `$` or
    backquote (`dir=${dir:-script}`);
  - a plain reference with exactly the working-directory prefix `$PWD/` or `${PWD}/`
    (`corvint_bin="$PWD/$corvint_bin"`), with no suffix, whose complete bounded expansion is
    exactly one unknown part followed by `/`. Other prefixes and appended suffixes are unproven:
    wildcard-prefixed extensionless execution targets are not resolved by `constructed`.

  Each reference takes all non-self-referential values, every allowed default's literal operand,
  and the unknown part for earlier values. Thus one default cannot discard another default's
  reachable target. The unrolled value expands at its name's depth, because its reference cannot
  recur on itself. References inside opaque command substitutions or backquotes do not recurse
  through the value expander and SHALL NOT be classified as self-reference cycles; they retain
  TSP-V0-960's existing opaque-result treatment. Existing working-directory normalization chains stay within depth 4 and retain their edges and
  constructed names. Any self-reference outside this allowlist SHALL retain the dynamic-source
  protection of AFP-V0-3147, whether or not expansion reaches a numerical bound. This includes
  composed alternatives (`x=${x:-script}; x=${x:+$x/check}`), unknown transformations
  (`x=${x%/unused}`), unknown-prefix references with suffixes (`x="$PWD/$x/check"`), literal
  wrappers, nested self-references and cycles through other names. A prefix that adds a directory
  (`x=check; x="$PWD/script/$x"`), an opaque command-result prefix, or an in-file `PWD` value
  that adds a directory SHALL retain the same protection. Unknown prefix forms fail closed.
  Failure mode: discarding a default or certifying an arbitrary braced reference can omit a
  reachable extensionless script and its dynamic sources. Acceptance: the reported default/alternative
  composition, prefix-directory composition and unknown prefix/transform forms select the wrapper
  for configuration and target changes; composed defaults and plain working-directory normalization
  preserve the literal execution edge. Rollback: disable this shortcut
  and retain dynamic-source protection for every self-reference; never restore unchecked unrolling.

### Live test tier (V1-4815)

- **AFP-V0-2700:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4815)
  A Go test that reads the live repository at HEAD or builds release binaries MAY join the live
  test tier by calling `testsupport.RequireLiveTier(t)` as its first statement. The helper SHALL
  run the test by default, and SHALL skip it, naming AFP-V0-2700 and the job that runs it, only when
  the environment holds exactly `CORVINT_SKIP_LIVE_TIER=1` and the test binary was built without the
  `live` build tag (a tagged binary always runs the tier). Every run that does not set the variable
  (local `go test`, `corvint test`, the consumers of `.github/cishards/test-commands.json`, `make
  gate`, batch-precheck and nightly gates) therefore runs the tier, and a change that breaks a live
  test fails there, before the merge queue (V1-4923). A live-tier test SHALL keep passing when run;
  the tier changes where it runs, never what it asserts. A test that selection or a gate names as always selected, such as
  `TestOwnershipAuditRatchet_AZU_V0_002` (AZU-V0-002), SHALL NOT join the tier.
- **AFP-V0-2701:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4815)
  The tier SHALL be discovered from source, never listed by hand: `cishards.LiveTests` lists each
  top-level `TestXxx` in a tracked root-module `_test.go` file (outside `testdata`, `vendor`,
  hidden and `_` directories and nested modules) whose body calls `RequireLiveTier` through a
  selector, grouped by package. `make live-test` runs `tools/live-test-run`, which runs each
  package's tier with `go test -tags live -count=1 -timeout T -run '^(A|B)$'`, exits 1 on any
  failure and 2 on an unreadable or empty tier. `live-test` is not a `GATE_STEPS` entry: `make gate`
  and the nightly release gates (which run `make -k gate`) already run the tier through `go-test`.
  `script/batch-precheck.sh` runs `make live-test` as an extra target before every batch push.
- **AFP-V0-2702:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4815)
  PR shards SHALL exclude the tier: the Go test step of the `go-product-shard` job in
  `.github/workflows/ci.yml` sets `CORVINT_SKIP_LIVE_TIER: "1"`, no other line of the workflow or of
  `.github/cishards/test-commands.json` sets it, and no `go test` line passes a `-tags` value naming
  `live`, so every live test skips in the shards and runs everywhere else. The
  merge queue SHALL run the tier in a `go-live` job (`make live-test`) on `merge_group` events,
  which the required `go-product` result requires to succeed there and to be skipped on other
  events. The job runs without the race detector, which the PR shards used for these tests
  before; a race in a live-tier test surfaces only where `-race` runs it. The workflow half takes effect only after the owner's ci-control-plane review of its own
  PR; until then PR shards run the tier as before.

### Narrowing-input guard and sized shard matrix (V1-4930, V1-4934)

- **AFP-V0-2720:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4930;
  amends AFP-V0-098 and AFP-V0-933) `narrow.sh` SHALL force FULL for a `.github/` change only when
  the range BASE..TESTED_MERGE changes a trusted narrowing input: a path the trusted narrowing reads
  or executes. `github-guard.sh`, beside `narrow.sh` in the trusted checkout (the
  `CORVINT_PR_TOOL_SOURCE` tools of a pull request, the queue base's `forward-rows` of a merge-queue
  entry), names the inputs: `.github/forward-rows`, `.github/cishards`, `.github/testconfine`,
  `.github/actions`, `.github/workflows/ci-forward-bar.yml`,
  `.github/workflows/ci-control-plane.yml`, and the narrowing sections of
  `.github/workflows/ci.yml` (AFP-V0-2721). It reads the range with
  `git diff --name-only -z --no-renames` over those pathspecs (raw NUL-separated paths, so a
  non-ASCII path is never display-quoted away, and both sides of a rename), prints `0`, or `1`
  followed by the changed inputs one per line, or `unreadable` and its reason, and exits 0 in every
  case. `narrow.sh` records `0` as no `.github/` reason; `1` as the FULL reason "the change touches
  .github/ narrowing inputs (AFP-V0-2720): INPUTS", the first five inputs also in the record's
  `githubInputs` list; and anything else (`unreadable`, a missing guard, a guard that exits
  non-zero, a base that was not proven) as the unreadable FULL reason, as before. Every other
  `.github/` path (another workflow, `ci-cadence`, `macos`, `flake-report`, `nightly-signal`,
  `gate-reuse`, `qualifications`, a document) does not force FULL: such a change narrows when every
  other AFP-V0-098 condition holds (a MET verdict, the driver a superset of the shadow). The tested
  tree's copies of the guard, of its list and of ci.yml are never read for the decision: the list
  lives in the trusted copy, and the compare reads Git objects of both revisions.
- **AFP-V0-2721:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4930)
  For `.github/workflows/ci.yml` the guard SHALL compare sections, not the file. When
  `git diff --quiet` says the file changed, it reads both revisions with `git show` and compares,
  for each of `docs-plan`, `narrowing-verdict`, `shard-plan`, `go-product-shard`,
  `ci-tested-tree-record`, `ci-narrowed-tree-record` and `go-product`, the job's block (its
  two-space key line to the next two-space key or top-level key), and, as the section `-`, the text
  outside the members of `jobs:` (every top-level key: `on`, `concurrency`, `permissions`, `env`),
  naming each differing section as `.github/workflows/ci.yml (NAME)`; another job's block may
  change, be added or be deleted. It fails closed: a YAML anchor (`&name`) in either revision is a
  change (`(a YAML anchor)`), since an anchor lets one block rewrite another; a `jobs:` key that is
  not a plain block mapping compares the whole file; a job key not written as exactly two spaces,
  the name and a colon leaves its lines in the preceding section, so a guarded block that moves or
  vanishes differs from the base; a deleted or renamed ci.yml compares as empty; an unreadable base
  file is `unreadable`.
- **AFP-V0-2722:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4930)
  Fault-injection evidence. Tests SHALL hold, through the real `narrow.sh`, `github-guard.sh` and
  git over a repository, that a pull request that edits `narrow.sh`, one that edits the
  `test-confine` wrapper and one that edits the selection line of the `go-product-shard` tests
  step each record FULL naming that input as the one `.github/` reason, and that one that adds an
  unrelated workflow, one that edits `ci-cadence` data and a document, and one that edits another
  job of ci.yml each record NARROWED with an empty `githubInputs`. The merge-queue path
  (`mq-narrow.sh`, then the same `narrow.sh`) applies the same guard over the queue range.
- **AFP-V0-2723:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4934)
  `cishards.Size` (`SizeFrom`) SHALL return the fewest shards from `min` to `max` whose AFP-V0-590
  balanced placement of the selection (`SelectFromParallel`, at the shards' `go test -p`) keeps
  every shard's predicted wall time (the AFP-V0-2181 `Wall` over the parallelism) within a budget
  in milliseconds, or within the longest unit of the `max`-shard placement when that unit alone
  exceeds the budget (a whole package no slice file splits cannot be brought under the budget by
  any count; a split package's slices are its units, so a count too small to split it is not
  accepted at the whole package's cost); `max` when no count does; `min` for a selection that
  names no package of the universe; and `max` for a cost table the placement would discard, so a
  job is never traded on an unpriced guess. The helper takes `--size-budget-ms MS` with
  `--select FILE` (`--min-shards N`, default `MinNarrowedShards` = 2; `--shards` as the maximum;
  `--parallel`) and prints the count alone; a refusal is an error, never a count. The workflow
  passes `DefaultWallBudgetMs`, 400 s, the AFP-V0-060 slice bound, so a sized shard is never
  planned longer than the longest slice a full shard runs; the floor of two bounds the fallback
  (half the suite at the shards' parallelism when every shard's own gate records FULL).
- **AFP-V0-2724:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4934;
  amends AFP-V0-590 (4) and AFP-V0-591) The `shard-plan` job of `.github/workflows/ci.yml` SHALL
  size the `go-product-shard` matrix of a pull request before any shard starts. It runs only for a
  same-repository `pull_request` with `CORVINT_PR_NARROWING` on and a MET verdict, and it needs only
  `narrowing-verdict`, never `docs-plan`, so its checkout, builds and narrowing run beside docs-plan
  rather than after it. It reads no docs-plan output: on a DOCS or REUSE decision no consumer reads
  its outputs (`go-product-shard` does not start, `go-product`'s AFP-V0-591 guard requires no
  marker and `ci-narrowed-tree-record` does not run), so its count is unused there. It resolves and builds the trusted tools as a shard does (`pin-main`,
  the pinned planner, selector, driver and shadow, `ci-shards` and `test-confine` from the tested
  checkout) and, after `pin-lag.sh`, runs the trusted `shard-plan.sh` from the
  `CORVINT_PR_TOOL_SOURCE` checkout. `shard-plan.sh` runs `narrow.sh` beside it once, as shard 0 of
  `FULL_SHARDS` (16) with every input a shard's gate gets, and only on a NARROWED record sizes the
  record's `selected` with `ci-shards --shards FULL_SHARDS --min-shards MIN_SHARDS (2)
  --size-budget-ms WALL_BUDGET_MS (400000) --parallel SHARD_TEST_P (4)`; it writes the step
  outputs `count` and `matrix` (the JSON list `[0, …, count-1]`), keeps `FULL_SHARDS` for a FULL
  decision, a missing or unreadable record or selection, a sizing failure and a count outside
  1..FULL_SHARDS, and never fails the job. The job's outputs default to sixteen on a failed step or
  a skipped job, and `go-product-shard` takes
  `shard: fromJSON(needs.shard-plan.outputs.matrix || '[0, …, 15]')`, so a `main` push keeps sixteen
  shards (a `merge_group` entry is sized by AFP-V0-3260). `SHARDS` stays `strategy.job-total` in every shard step, so the
  complete-universe partition (AFP-V0-022), the balanced placement (AFP-V0-590), each record and
  each coverage marker name the sized count, and `go-product`'s AFP-V0-591 guard and
  `ci-narrowed-tree-record`'s `shards=` read `needs.shard-plan.outputs.count || '16'`. The shards
  decide for themselves: a shard whose own gate records FULL runs its complete-universe partition
  over the sized matrix, so no package is lost and the superset rule is unchanged; the plan job's
  own record is retained as `ci-shard-plan`, never as `ci-narrowing-*`. The narrowing pins
  (`CORVINT_PR_TOOL_SOURCE`, `CORVINT_TEST_SHADOW_SOURCE`, `CORVINT_PR_SAMPLE_K`,
  `CORVINT_PR_SAMPLE_SEED`) move to the workflow `env`, which both jobs read unchanged. Wall-clock
  budget: a sized shard is planned within 400 s of predicted test wall time (or its longest unsplit
  unit), the bound a full shard's slices run under, plus the shard's fixed setup (about 110 s
  measured on run 42212454082). The plan job (about 80 s: the median of the same steps in the
  sixteen shards of that run, 56 s to 89 s) starts when `narrowing-verdict` ends (17 s into that
  run) and runs beside docs-plan (which ended at 72 s), so the first shard starts at the later of
  docs-plan and narrowing-verdict plus the plan job: about 100 s into the run, at most about 110 s,
  instead of 72 s unsized. A sized run with the same predicted longest shard therefore finishes
  about 30 s later than the unsized run (650 s on that run), in exchange for its fewer jobs; a plan
  job that waited for docs-plan would start the first shard at about 154 s.
- **AFP-V0-2725:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4934;
  amends AFP-V0-099 and AFP-V0-734) Consumers SHALL accept the selection run's own shard count.
  `row.jq` admits one count `$m`, named by every readable selection document and narrowing record
  of the selection run, when it equals the full matrix `$n` or a record with that count says
  NARROWED; the narrowing records and selection documents then validate against `$m` and must cover
  shards 0..`$m`-1, a pull-request row's outcomes take `$m` and a queue row's outcomes the full queue
  run's `$n`, and the row records `selectionShards`. Any other count fails the shard-count shape, as
  before. `tools/ci-reuse-plan` admits a narrowed record (`corvint-ci-tested-tree/2`) of any count
  from 1 to 64 when every `go-product-shard (i)` job of that count, `docs-plan`, `narrowing-verdict`
  and `go-product` passed, and its REUSE decision names that count in `shards` with one record per
  shard; a full record still needs the asked count. The docs-plan REUSE step accepts a decision
  whose `records` length equals its `shards`, and whose `shards` equals `REUSE_SHARDS` unless the
  scope is NARROWED.
- **AFP-V0-2726:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4934)
  Every isolated helper build in `.github/workflows/*.yml` (a run step that sets a variable to a
  `$RUNNER_TEMP/...` module directory, copies `.github/cishards/` or `.github/testconfine/` files
  into it with `cp ... "$VAR/"` and `cp ... "$VAR/cmd/"`, and writes its `go.mod` with `printf`)
  SHALL copy every file its packages and their `//go:embed` directives need. A Go test in
  `.github/cishards` reads every workflow, attributes every `cp .github/cishards/` and
  `cp .github/testconfine/` line to such a module (an unattributed one fails), replays each
  distinct module from the checkout into a temporary directory, checks that every `//go:embed`
  pattern of every copied Go file names a copied file, and runs `go vet ./...` there and
  `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...`, with the same `GOENV=off`,
  `GOWORK=off`, `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off` and empty `GOFLAGS` the
  workflows set. Because `./...` skips `.github` and the isolated modules do not copy this test,
  the `go-static` job runs it in the checkout beside the AFP-V0-2701 and AFP-V0-2702 tests, on
  every pull request, merge-queue entry and main push that runs go-static.

- **AFP-V0-3260:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-4353;
  amends AFP-V0-2724) The `shard-plan` job SHALL also size the `go-product-shard` matrix of a
  `merge_group` entry whose base is `refs/heads/main`, when `CORVINT_MQ_NARROWING` is on and the
  verdict is MET. Its `pin-main` step resolves `origin/main` as the shards do, and its plan step takes
  `shard-plan.sh` and `pin-lag.sh` (`git archive` of `.github/forward-rows`) from the same commit the
  shards' queue gate takes `mq-narrow.sh` from: `QUEUE_BASE` when it is on `origin/main`, else its
  merge base with `origin/main`, never the entry. A copy without `mq-narrow.sh` fails the step, which
  keeps sixteen. With `QUEUE_BASE` set, `shard-plan.sh` runs `mq-narrow.sh` instead of `narrow.sh`,
  with the queue variables a shard's gate gets and `MQ_DELTA` empty, so its selection is the whole
  queue range: a superset of every shard's own selection, which a stacked entry's delta can only
  narrow, so the sized count never undersizes a shard. It prices placement at the queue shards'
  `SHARD_TEST_P` (1), by the same expression the gate and tests steps use. A queue entry that docs-plan
  decides DOCS or REUSE starts no shard, as before. The full fixed-count paths stay sixteen: the
  docs-plan `REUSE_SHARDS`, `ci-tested-tree-record` (which runs only when all sixteen shards record
  FULL) and the forward-row collector's `SHARDS`. `row.jq` (amending AFP-V0-2725) gives a queue row's
  outcomes the one count every readable outcomes document of the queue run names, when it is within
  1..`SHARDS`, and still requires them to cover shards 0..count-1: a sized queue run whose shards all
  fall back to FULL ran the complete universe over that count, so it is a complete row, not a false
  INCOMPLETE that counts toward the narrowing kill switch. Any other count keeps `SHARDS`.

- **AFP-V0-2360:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-0643, owner direction 2026-11-03) `internal/companionrelease` SHALL be declared in
  `.corvint/test-read-scopes.json` (AFP-V0-023) and leave `.corvint/unbounded-readers.json`. Its
  tests that exported the enclosing checkout's `HEAD` through Git SHALL instead export one
  disposable fixture repository per test binary (`currentSourceFixture`): the Tasks subset
  (`tasksExportFiles`, `tasksExportPrefixes`), `.gitattributes`, `.gitignore` and
  `extensions/vscode/` copied from the working tree, the repository's own ignore rules applied,
  committed with `closedGitEnv`, no hooks and fixed identities, outside the repository root. The
  production `exportSource`, `tasksExport` and every assertion are unchanged; the content is the
  working tree's bytes of the declared paths, so in CI's clean checkout it is `HEAD`'s. Builds from
  the repository root pass `-buildvcs=false` and read only declared entries.
- **AFP-V0-2361:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-0643; amended in batch-1107x: the later rescue-c fail-closed decision PUB-V0-671 keeps this
  package on the `UNBOUNDED_READER` floor with no read-scope declaration, pinned by
  `TestPUBV0671HostArchiveProofKeepsTheUnboundedFloor`; the disposable-repository rule below still
  holds) `conformance/release-artifact-v0` SHALL be declared the same way. Its tests that ran
  Git against the enclosing checkout SHALL run it against a disposable repository: the shared
  archive repository for destination validation, and one committing the package's own
  `manifest.json` for the cancelled archive build; fixture repositories are created from a
  temporary parent directory, never with the checkout as Git's working directory. Plain reads of
  committed files stay `../..` reads of declared entries, and the two evidence trees the
  historical-version sweep walks (`.agent-evidence/`, `benchmarks/results/`) are declared whole so
  the sweep keeps its strength.
- **AFP-V0-2362:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-0643) The darwin/arm64 Corvint host archive proof (`TestCorvintHostArchivePartialProof`),
  which exports and builds the checkout's `HEAD` through its `.git`, SHALL run only when
  `CORVINT_HOST_ARCHIVE_PROOF=1`, which the `release-gates` workflow's legs set, and SHALL skip
  naming the variable otherwise. It is the one read outside the package's declaration and runs
  only in a whole-suite release gate, never in a planned selection.
- **AFP-V0-2363:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-0643; amended in batch-1107x: in effect for `internal/companionrelease` only, since
  `conformance/release-artifact-v0` stays on the floor under PUB-V0-671) With the declarations of
  AFP-V0-2360 and AFP-V0-2361, `corvint affected` SHALL NOT
  select either package for a dirty path outside its subtree and declared entries (no
  `UNBOUNDED_READER` witness), and SHALL still select it, with `DECLARED_READ_SCOPE`,
  `PATH_LITERAL_READER`, `BINARY_EXEC` or a dependency witness, for a path inside them. The
  measured per-merge saving is retained in build-log entry `2026-11-03-floor-typed-reach`.
- **AFP-V0-2370:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-2744) `corvint test` SHALL narrow a Go package of the allow-list `typedReachPackages`
  (`cmd/corvint`, `internal/tasks/cli`, `internal/tasks/store`) that the plan selects by
  `DEPENDENCY_PATH` to the test functions a type-checked call graph of its host test binary
  (`internal/liveverify/affected/golang/typedreach`) cannot prove unable to reach the change.
  Every other package, and every allow-listed package any AFP-V0-2372, AFP-V0-2374 or AFP-V0-2377
  condition refuses, keeps its whole-package selection with the refusal named.
- **AFP-V0-2371:** (experimental; V1-2744) The graph SHALL load every module package of the test
  binary from source under the executor's build configuration, type-check it with
  `go/types` (standard-library packages from that configuration's `GOROOT` without bodies), and give each function or
  method body, with its function literals, an edge to: every function it calls or names as a
  value; every method of every module named type structurally inside a value it converts to an
  interface (call arguments, assignments, returns, composite elements, sends, comparisons,
  explicit conversions, `append`) or passes as a type argument, which covers standard-library and
  reflective calls; every module method of a name it calls on a type parameter; and the code a
  package variable it mentions holds. A package variable initializer's calls run at
  initialization; the functions, literals and converted values it only stores, and those an
  `init` function only assigns to a package variable, are held by that variable. Source discovery
  SHALL use the configuration's cgo setting; cgo files it selects are refused by AFP-V0-2372.
  Build constraints (`//go:build`, legacy `// +build`, file-name suffixes) SHALL be evaluated
  under the executor's exact build configuration (`typedreach.ProbeConfig`): `go env GOFLAGS`
  and the `context` of `go list -e [-tags, -race, -msan, -asan] unsafe`, run with the
  environment and `go test` flags the run uses (`sessiontest.GoExecutor`: the declared Go
  environment and flags, or the CI profile's), so `GOFLAGS` from the environment or from
  `go env -w`, `CGO_ENABLED`, `GOOS`, `GOARCH`, `GOEXPERIMENT` and feature levels such as
  `GOAMD64` are read as the go command reads them. The configuration is `GOROOT`, `GOOS`,
  `GOARCH`, `CgoEnabled`, `Compiler`, `UseAllFiles`, build, tool and release tags; its identity
  (`sha256:` of its canonical encoding) SHALL be named in every narrowed reason, and the
  executing shard recomputes the plan under its own environment, so a run whose configuration
  differs from the planner's refuses the supplied plan. A `go test` flag or `GOFLAGS` entry
  the model does not classify (any build flag other than `-tags`, `-race`, `-msan`, `-asan` and
  `-mod=readonly|mod`, such as `-toolexec`, `-overlay`, `-gcflags`, `-pgo`, `-coverpkg`,
  `-mod=vendor`, a switch with an explicit value such as `-race=1`, `-race=false` or `-race=true`,
  a build switch (`-tags`, `-race`, `-msan`, `-asan`) set both in `GOFLAGS` and in the flags, or a
  positional argument; the explicit-value and both-places forms are disabled for 1.0), a configuration
  that cannot be read, or a compiler other than `gc` SHALL keep every allow-listed package whole,
  naming that the executor build configuration cannot be read. As in the go command, a flag's
  last occurrence wins. Without a configuration (`typedreach.Load`, `LoadBase`) every explicit
  constraint SHALL still refuse narrowing with `UNMODELLED_SOURCE`.
- **AFP-V0-2372:** (experimental; V1-2744) The always-run code is every module package's
  initialization (variable initializer calls and `init` bodies) and `TestMain`, less each
  `TestMain` `if` statement without `else` whose condition requires `os.Args[N] == "C"` (N at
  least 1, C a non-empty constant not starting with `-`, alone or in a conjunction) and whose body
  ends by calling `os.Exit`: `go test` passes the binary only `-test.*` flags, so the branch runs
  only in a child the binary starts as itself, which only a process start can do, and that start
  keeps its test by AFP-V0-2373. The condition itself runs in the parent and stays always-run. When it reaches the change the package SHALL stay whole with
  `ALWAYS_RUN_REACHES_CHANGE`, and when it starts an unproved process with
  `ALWAYS_RUN_STARTS_PROCESS`. A module package with cgo, assembly or other non-Go source, a
  `//go:linkname` directive, an import outside the module and `GOROOT`, or any type error SHALL
  keep the package whole with `UNMODELLED_SOURCE` or `TYPE_CHECK_FAILED`.
- **AFP-V0-2373:** (experimental; V1-2744) A test function (`Test`, `Benchmark`, `Fuzz`, `Example`
  names `go test` runs) SHALL be kept with `REACHES_CHANGE` when its closure reaches a changed
  declaration (a changed function by edge, a changed constant, variable or type by mention, a
  changed type by any expression of a type holding it), and with `STARTS_PROCESS` when its closure
  may start a process: any use of package `os/exec` (a `Cmd` however made, by `Command`,
  `CommandContext`, `new`, a literal or a variable declaration, and its `Start`, `Run`, `Output`
  or `CombinedOutput`), `os.StartProcess`, `syscall.Exec`, `ForkExec`, `StartProcess`,
  `plugin.Open`, or any of these taken as a value, whatever the program: a constant `git` or shell
  can still run module code through configuration, aliases or hooks. The changed set is closed
  over declarations: a constant, type or variable whose type, value or type parameters name a
  changed constant or type, in any package, and each later member of a changed constant's group,
  is changed too; a function's signature (an array length, a parameter type) counts as part of the
  function. Every other test function SHALL be dropped.
- **AFP-V0-2374:** (experimental; V1-2744) A dropped test whose closure reads a package variable
  a kept test's closure may write SHALL be kept with `READS_STATE_A_KEPT_TEST_WRITES`, to a
  fixpoint. A dropped test whose closure may write a package variable a kept test's closure reads
  SHALL be kept with `WRITES_STATE_A_KEPT_TEST_READS` in the same fixpoint, whatever the order
  the tests run in: unchanged setup that sets a flag a kept test branches on decides whether the
  kept test reaches a broken branch, so the kept test run alone could pass where the package
  fails. Such a writer's reads count as a kept test's reads in turn; its writes count as a kept
  test's writes only once its own closure reads state a test kept by `REACHES_CHANGE`,
  `STARTS_PROCESS` or `READS_STATE_A_KEPT_TEST_WRITES` may write, since until then it writes at
  head what it wrote at base. While any test is kept, a dropped test whose closure calls a
  process-wide setter (the `processState` functions below, called or taken as a value) SHALL be
  kept that way too, and so keeps the package whole, since that state has no per-reader pseudo
  variable and no kept test can be proved not to read it; a file write at a constant path also
  writes the file-system pseudo variable, so it is kept only when a kept test may read files.
  A write is an assignment, increment, address-taking, or `append`/`copy`/`clear`/`delete`
  destination rooted at the variable, and for a module variable also a pointer-receiver method
  call on it or passing it, when of map, slice, pointer, channel or interface type, as an argument,
  through any selector, index, slice, dereference or type assertion; the
  read-only methods of `regexp.Regexp`, `strings.Replacer` and the template types' execution are
  not writes (`sync.Mutex`/`RWMutex` locks, `sync.Once.Do` and `sync.Pool` are, since lock state
  can outlive a test). Any other use of a module variable holding a map, slice, pointer, channel or interface (directly
  or in a struct or array field) than
  an index, field or range yielding no such reference, a `len`/`cap` argument or a comparison
  operand is a write, since a copy of the reference can be written through. A file write
  (`os.WriteFile`, `Create`, `OpenFile` unless its constant flags only read, `Mkdir*`, `Remove*`,
  `Rename`, `Symlink`/`Link` at their link path, `Ch*`, `Truncate`, `os.CopyFS`,
  `io/ioutil.WriteFile`) whose path mentions a module
  package variable writes that variable; a bound pointer-method value taken on a package variable
  is a call for this rule. The file system is a third pseudo variable: every file write or read
  (`os.ReadFile`, `Open`, `Stat`, `Lstat`, `ReadDir`, `Readlink`, `DirFS`, `filepath.Walk*`,
  `Glob`, `EvalSymlinks`, `fs.ReadFile`, `io/ioutil.ReadFile`, `io/ioutil.ReadDir`, a read-only
  `OpenFile`) writes or reads it unless its path is private: a call of `TempDir`, `os.MkdirTemp`,
  `os.CreateTemp`, `io/ioutil.TempDir` or `io/ioutil.TempFile`, a local variable a body assigned
  once from a private path (a range clause key or value is an assignment, so a ranged
  variable is never private; a range or assignment target that is not a bare identifier, such
  as a parenthesized one, is not modelled and ends the privacy of every variable in the body,
  disabled for 1.0), `filepath.Join` or `path.Join` of a private first element and constant
  elements naming no `..`, or a private path plus a constant starting with `/` naming no `..`; any
  other expression, a helper call included, is not private. Temporary creation by `os.MkdirTemp`,
  `os.CreateTemp`, `io/ioutil.TempDir` or `io/ioutil.TempFile` also writes the file system unless
  its parent directory is provably private: a private result still changes a shared parent's
  directory entries. These creators taken as values write the file system too. Any use of
  `os.OpenRoot`, `os.OpenInRoot` or an `os.Root` method reads and writes it;
  and every state reader or writer taken as a value reads or writes its pseudo variable. A closure
  that may start a process (AFP-V0-2373) writes the file system, environment and signal pseudo
  variables, since a child's writes are not bounded. Standard-library state read without being
  named counts as read: a call into `fmt` reads `os.Stdout` and `os.Stderr`, into `log` reads
  `os.Stderr` and the log default (written by `log.SetOutput`, `SetFlags`, `SetPrefix`, and
  `(*log.Logger).SetOutput`, `SetFlags`, `SetPrefix`, including bound method values and method
  expressions). Without a receiver ownership proof, these methods SHALL count as default-logger
  writes even when the receiver is a local alias or a call result such as `log.Default()`. Into
  `log/slog` also the slog default (written by `slog.SetDefault`, `SetLogLoggerLevel`), and into
  `time` reads `time.Local`. Output through a process stream or a logger (`fmt.Print*`, the `log`
  `Print*`, `Fatal*`, `Panic*` and `Output` functions and `*log.Logger` methods, and the `log/slog`
  `Debug`, `Info`, `Warn`, `Error`, `Log`, `LogAttrs` functions and `*slog.Logger` methods with their
  `Context` forms), called or taken as a value, SHALL write the file-system pseudo variable, since
  the stream or the logger writer may be a file a later test reads. Input from the process
  standard input (`fmt.Scan`, `Scanf`, `Scanln`, called or taken as a value) SHALL read it,
  since an earlier test or `TestMain` may have redirected `os.Stdin` to a file; `Fscan*` on
  `os.Stdin` reads `unknown` through its `io.Reader` argument. The standard library is otherwise closed by an allow-list: a use of a
  standard-library function or method that the model neither records above nor finds in
  `purePackages` (value-only packages such as `strings`, `encoding/json`, `regexp`, `sync`,
  `testing`, `time`, `fmt`, less `impureFunctions` and the package-level `math/rand` functions) or
  `pureOS` makes its closure read and write a fourth pseudo variable, `unknown`, as do a process
  start, any write of a standard-library variable, `os.Symlink` and `os.Link` (a link can lead a
  private path anywhere), a call of an abstract method of an `io`, `io/fs`, `net`, `os`,
  `net/http`, `database/sql` or `bufio` interface (it may dispatch to a file or connection), and a
  standard-library call handing such an interface parameter anything but a module type or an
  in-memory `bytes.Buffer`, `strings.Builder`, `bytes.Reader` or `strings.Reader`. The same
  argument check SHALL apply to an indirect call whose callee the model cannot resolve, through
  its static signature, and a standard-library function or method taken as a value with such an
  interface parameter SHALL itself be unknown, since it may later be called with any argument; `fmt`'s
  `Fprint*` and `Fscan*` are pure only with such a buffer. `runtime` is admitted per function
  (`Caller`, `Callers`, `CallersFrames`, `FuncForPC`, `Gosched`, `KeepAlive`, `Version`,
  `NumCPU`); `crypto` and `archive/zip` (their registries) are not admitted; `time.LoadLocation`,
  the template and `go/parser` file-loading entry points and `filepath.Abs` are impure. `os.TempDir`,
  `UserHomeDir`, `UserCacheDir`, `UserConfigDir`, `MkdirTemp`, `CreateTemp`, `io/ioutil.TempDir`
  and `io/ioutil.TempFile` read the
  environment; `signal.NotifyContext` writes signal dispositions; `slog.SetDefault` writes both
  default loggers; a function taken as a value carries its implicit reads, and so SHALL a stored
  bound method value or method expression (such as `log.Default().Prefix` or
  `(*log.Logger).Prefix` held in a package variable), whose holder reads what its call reads; an
  `os.Process` method value reads signal dispositions and an `os.Root` method value reads and
  writes the file system. `unknown` on either side conflicts with any
  pseudo or standard-library variable on the other: an `os.File` method, `net`, `net/http`, a
  listener on a fixed port or any unlisted package therefore keeps every dropped test that reads
  such state when a kept test may write it. The environment (`os.Setenv` family; read by `os.Getenv` family and every process
  start) and signal dispositions (`signal.Notify`, `Ignore`, `Reset`, `Stop`, as calls or values) are two
  pseudo variables under the same rule: `signal.Stop` deregisters a channel, so a signal a later
  test sends itself can take its default action and end the suite. A `signal.Stop` call SHALL NOT erase a `Notify` write without proof of
  matching channels and restoration on every exit. The current model retains the write even
  with an unconditional deferred Stop; mismatched channels, conditional cleanup and early returns
  cannot establish restoration. A kept
  test whose closure calls another process-wide setter (`os.Chdir`, `runtime.GOMAXPROCS` with an
  argument other than a constant below 1, `flag.Set`/`Parse`, `net/http.Handle*`, the
  `runtime/debug` setters except the goroutine-local `SetPanicOnFault`, and the others
  `processState` lists), a file write at a constant path, or one of these functions taken as a
  value, SHALL keep the package whole with `KEPT_TEST_WRITES_PROCESS_STATE`.
- **AFP-V0-2375:** (experimental; V1-2744) A narrowed package SHALL run `-run ^(KEPT...)$` with
  reason `selected: the tests reaching <changed files> (typed reach: N of M test functions cannot
  reach it)` (code `SELECTED_REACHING_CHANGE`), or `-run ^$` with `selected: no test reaches
  <changed files>; build and vet only` when none is kept; a pattern past `cishards.MaxPatternBytes`
  or every test kept (`EVERY_TEST_REACHES_CHANGE`) keeps it whole. A refusal keeps the plan's
  whole reason and appends ` (typed reach: <refusal>)`.
- **AFP-V0-2376:** (experimental; V1-2744) The changed declarations SHALL be read per changed
  non-test Go file by comparing base and head top-level declarations by key (function name,
  `Type.Method`, each declared name; a constant group as a whole) on printed text, either side
  absent counting as changed (`typedreach.DiffFile`). A name Go lets repeat SHALL keep one key
  per occurrence: each `init` function, and each blank (`_`) declaration by its ordinal among the
  file's blank declarations, so no blank initializer hides an edit of another and each keeps its
  own initialization status. The declarations' package SHALL be the
  module path joined with the file's directory (`typedreach.ImportPath`): a file beside `go.mod`
  belongs to the module path itself, never `module/.`, in `corvint test` and the measurement tool
  alike, so a root-package change is resolved and base-checked like any other. Compiled source positions are observable:
  a declaration whose file, line or column positions move SHALL also be changed, even when its
  printed text is identical, including positions adjusted by `//line` directives and positions
  inside a declaration. A preceding declaration's growth therefore seeds unchanged functions
  that move, retaining their dependent-package callers. An `init` function, a package variable
  whose initializer calls or is blank, a package clause change or a parse failure SHALL be an
  initialization change, which keeps whole every test binary linking that package and bears on no
  other. Before narrowing, the changed files' base bytes SHALL also be type-checked over the
  current source closure, omitting files absent at base, without modifying the worktree. An
  unreadable, deleted or incompatible base closure, a changed file absent at head, a changed
  file whose build constraint lines differ between base and head, or (without an executor
  configuration) any constrained base file keeps the package whole with
  `BASE_SOURCE_UNPROVED`; the base closure is selected under the same configuration as head. If any changed function or method present at base, a method of a
  changed type, or the code a changed package variable present at base holds (a function literal,
  function value or method value its initializer stores, which a test reading the variable runs)
  can write any shared state, start a process or call a process-wide setter, the
  package SHALL stay whole with `BASE_MAY_WRITE_SHARED_STATE`, naming the declaration. This
  conservative check includes calls and held values: state the edit stopped writing still counts.
  The measurement tool SHALL apply this same base check. A changed name absent at head (removed)
  SHALL count every declaration of its package as
  changed; an added or removed method counts its receiver type as changed. An import change SHALL
  count every declaration of the file using a path imported under a new name, or newly imported,
  as changed; an added path no other file of the package imports, or a removed path the binary no
  longer links, SHALL be an initialization change; a blank or dot import change always is.
- **AFP-V0-2377:** (experimental; V1-2744) A package SHALL be refused narrowing, with the reason
  named, when it is an unbounded reader, when one of its globs matches a dirty path
  (`Graph.GlobRead`), when it embeds files, reads the import graph or is a declared existence
  reader, when its path literals name a dirty path (`Graph.NamesPath`), when its declared read
  scope holds one, when a dirty path is its own file, when a dirty path lies in or under the
  directory of a package that embeds files, when a dirty path other than a non-test `.go` file
  (a data file, test data, or a `.go` file under `testdata`) lies in or under the directory of any
  Go package, whether embedded or read at run time, when a changed file's base cannot be read, or
  when the change shows no changed Go declaration at all: an empty change set never slices.
  Another package's `_test.go` files are never linked and are left out; a path outside every Go
  package directory reaches a test only through a read the conditions above refuse.
- **AFP-V0-2378:** (experimental; V1-2744) `tools/typed-reach-measure` SHALL replay the narrowing
  of one package for the Go change between a base and a checked-out head and, with `-oracle`, run
  each test function alone in a test binary built at head and in one with
  `panic("typed-reach fault")` at the entry of every changed function, classifying each outcome
  from `-test.v` output (`pass`, `skip`, `fail`, `timeout`, `none`), naming every test whose
  outcome differs and every such test the plan dropped; `-oracle-dropped-only` runs only the
  dropped tests (a missed observer can only be one of them) plus `-oracle-also` controls. Recall
  against observed outcomes is retained in the build-log entry.

### Test-file import edges, line-less runtime sites and library planning (V1-5029, V1-5026, V1-5030)

- **AFP-V0-9009:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-5026; amending AZU-V0-8902) The file of a located
  `typescript:e2e-runtime-dependency:` site SHALL be the site text with its trailing backquoted
  token (a space and a backquoted spelling, cut at the last space-backquote) removed, then a trailing `:LINE` of digits
  removed. A line-less site such as `` dir/tests/items.spec.cjs `playwright` `` therefore reaches
  a directory runtime edge (`.corvint/runtime-edges.json`) that declares its test, selects the
  declared caller and closes, as a site with a line already did. A site ending in a backquote
  with no space-backquote, a truncated `+N` entry, or one whose file no unit owns as a test still
  names no declared caller and stays unknown. When the whole site text is itself a test path (a
  file whose name ends in a backquoted word or `:digits`), that test is the site. When the parsed
  file is a test as well, the site is ambiguous and stays unknown, so the parse never moves a site
  onto another declared test. Measured on this tree with `corvint affected
  --path extensions/vscode/src/extension.ts`: the three live-1.61.1 Playwright specs
  (`internal/doccorpus/.../behavior-events/live-1.61.1/tests/items.spec.cjs` and the two
  `internal/jstestprovider/.../route-stubs/live-1.61.1*/tests/*.spec.cjs`) become selections
  (264 to 267 selected units), and their three `e2e-runtime-dependency` unknowns close (67 to 64).

- **AFP-V0-9010:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-5029; amending AFP-V0-2202) A test file adds an import edge to no other package unless it
  changes its package's test import set. The package header compared by AFP-V0-2202 SHALL
  therefore fold every *unconstrained* test file of a directory into one summary line, when
  the directory's non-test files declare exactly one package clause N that neither ends in
  `_test` nor is `documentation`, and every unconstrained test file declares N or `N_test`. The
  summary holds the sorted set of import paths of the N test files, that of the `N_test` files,
  and the sorted `//go:embed` lines of both. A test file is unconstrained when it has no
  `//go:build` or `// +build` line, imports no `"C"`, and its name names no GOOS or GOARCH
  (`go/build.Context.MatchFile` accepts it for a platform no name names). A new, edited or
  removed test file that leaves both role sets and the embed lines as they were changes no import
  set and selects no *proven* import-graph reader outside its package. A test file that adds an
  import path to its role's set, moves one between roles, shrinks a set, adds an embed line,
  declares another package clause, carries a constraint or a platform name, or imports `"C"`
  still changes it. So does any test file of a directory without exactly one non-test clause,
  which keeps its own header line. The fold, and every header comparison, narrows only readers
  proven to observe the import graph alone. A reader (a unit with a `go list` call) SHALL be
  marked `Unit.ImportGraphReadsFiles` (set only with `ImportGraphReadsAll`, part of the graph
  digest and the graph cache entry when set) unless it is proven: every `go list` call is proven
  (AFP-V0-2200; a `-json` call, a template naming `Doc`, `TestGoFiles` or another field outside
  the AFP-V0-2200 list, or a unit that names `GOFLAGS` or `GOENV` is not), and no file of the
  unit may observe package metadata or source text by another channel. The channel classifier is
  a closed allowlist that fails closed: a file is proven only when every construct in it is one
  the allowlist names, and anything it cannot classify is observing. A file is proven when it
  can be read and parsed; every `//go:` line is `build`, `generate`, `noinline` or `fix` (so
  `embed`, `linkname`, `wasmimport` and any unknown directive are not); every function has a body
  (one without is implemented in assembly); every import is either a standard package on the
  inert list (`inertImports`: packages such as `bytes`, `context`, `encoding/json`, `fmt`, `io/fs`,
  `runtime`, `strings`, `testing` and `time`, whose API cannot read a file, start a process or
  report build metadata from the values a file can build without the classified packages) or a
  classified package (`os`, `io/ioutil`, `path/filepath`, `reflect`, `os/exec`), named by a local
  name no declaration of the file shadows, never by a dot import; and no selector names `Chdir`,
  `Setenv`, `Unsetenv` or `Clearenv`, whatever the receiver (`t.Chdir` and `t.Setenv` included),
  because a working-directory move redirects every relative read and `go list` target, and an
  environment change can set `GOFLAGS` or `GOENV` by a computed name. A classified package's
  members are allowed one by one (`classifiedImports`). `reflect` allows only `DeepEqual`. Among
  the other packages, only explicitly classified non-observing members, including path arithmetic
  (`filepath.Join`, `Dir`, `Rel` and the like) and environment reads, are allowed. Filesystem
  mutators are observing: their errors can reveal source existence or metadata. No mutator
  is certified here, including one targeting fixtures or temporary outputs, because mutation-path
  confinement is not proven. Unknown members also remain observing. `ReadFile`, `Open`, `ReadDir`, `Stat` and `Lstat` of `os` (and
  `ReadFile` and `ReadDir` of `io/ioutil`) are allowed only as a call whose path is a literal, or
  a `path` or `path/filepath` `Join` of literals, naming the package's own `testdata` directory or
  a path below it. This path proof SHALL reject climbing components and admit only
  regular files and directories in unfollowed source-directory entries, checking every ancestor
  before descent. An absent entry after checked ancestors is harmless; a symlink, other entry
  mode, metadata failure or unknown filesystem result is observing. Any other member makes the file observing: `filepath.Walk`, `WalkDir`, `Glob`,
  `os.DirFS`, `OpenRoot` and `StartProcess` among them. An `os/exec` member is allowed only as the
  selector of a `go list` call that AFP-V0-2200 proves (recorded by byte offset), and only inside
  a function. Every other reference makes the file observing: an alias (`run := exec.Command`), a
  function or method value, an `exec.Cmd` or `exec.ExitError` type, a struct literal, a
  package-level command, or any other command. The command such a call builds SHALL stay confined
  to its outermost function. It is either the receiver of a call of `Output`, `CombinedOutput`,
  `Run`, `Start`, `Wait`, `String`, `Environ` or a pipe method, or the sole value of a
  `name := call` (or `var name = call`). Every other use of name SHALL be such a method call; an
  assignment of `Stdin`, `Stdout`, `Stderr`, `WaitDelay`, `Cancel`, `Dir`, or `Env` from a literal
  environment proven by the closed AFP-V0-2200 forms and import identity; a read of `Stdin`,
  `Stdout`, `Stderr`, `Process` or `ProcessState` that does not take its address; or another definition by an `os/exec` call. Any other use leaves the call
  unproven: an argument (`json.Unmarshal(data, c)` included), a stored or returned value, an
  address, a `Path` or `Args` reference (`c.Args[4] = ...` included) or a reassignment. A `Dir`
  assignment SHALL keep the call proven only for the closed directory-origin allowlist:
  the literals `""` and `"."`; a direct fresh `TempDir()` on an unshadowed testing parameter
  used only as a direct method receiver; or the module root derived from this source file's
  `runtime.Caller(0)` physical filename through `filepath.Dir`, `Clean` and `Join` with relative
  literals. This filename origin SHALL be certified only without line remapping: either
  `//line` or `/*line ...*/` directives anywhere in the file invalidate this origin.
  Root derivations may use uniquely declared, preceding top-level locals only when
  every use belongs to the derivation (no assignment, address, alias or escape), and only when
  resolving the call from the root preserves its target set. Every unknown origin or form
  makes the call unproven, retaining whole-module source observation as well as import reads.
  Directory provenance alone SHALL NOT add a binary-execution edge for the listing's literal
  targets: when the arguments, command confinement, environment and output uses otherwise
  satisfy the closed AFP-V0-2200 proof, those literals are still only listing arguments.
  An unknown command use, mutation or escape keeps them ordinary uses and retains any binary
  edge. A later known directory assignment SHALL NOT erase an earlier unknown one.
  The comparison SHALL record
  `ImportSetChanges.Source` (`Graph.SourcePath`), whatever the headers: the least dirty path
  that the go command or a source parser may observe. That is a `.go` file (paths the go tool
  ignores included), a file the go command lists by another suffix (`.c`, `.h`, `.s`, `.syso`
  and the rest), or any file below an embedding Go unit's directory, whose content its build
  reads. Every `ImportGraphReadsFiles` reader is selected through it when no import set changed, with the witness chain owning unit (when a Go unit owns the path),
  reader. So a package comment, a body edit, a kept-set test file, an assembly file or an
  embedded file's content selects every unproven reader and no proven one; a path of none of
  these kinds (a README) selects none through this edge. A reader's file reads of other paths
  stay governed by its read scopes. With no comparison (nil), `Source` is computed from the dirty paths
  the same way.

- **AFP-V0-9011:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-5030) Library planning from a known base SHALL compare import sets as `corvint affected
  --base` does. `languages.SelectAt(ctx, root, head, commit, graph, dirty)` compares at a
  pinned commit read from the repository under root, and `languages.SelectBetween(base, head,
  graph, dirty)` compares two source trees; both then call `affected.Select`. The library callers
  that measure a change from a base use them: `prove`'s affected-test rows, `flows impact`
  (`appflows.FlowImpactAt`) and `delta.Compile`. So does `corvint affected --snapshot`, which
  pins its base. An unpinned or unreadable base (or a nil base tree) leaves the comparison
  unknown, and the plan counts every dirty Go file as an import-set change, exactly as a
  `--path` plan and plain `affected.Select` do. A caller with no base keeps that conservative
  fallback; none drops a reader.

### Driver input and file-set reads (lane avdrv, 2026-11-08)

- **AFP-V0-9400:** (accepted under the owner's standing auto-accept (2026-10-11); V1-4973, V1-5034) A driver shell scanner that skips a here-document body SHALL retain
  `driver:computed-invocation` for its target or job. A wrapper may pass that body to
  an interpreter, save it for later execution, or pass a nested body's output to another
  command; a literal first command word does not prove the input inert. Quoted and
  tab-stripped delimiters and unterminated bodies retain the same fallback. Literal
  `<<` inside a quoted argument is not a redirection. The existing direct
  `sh <<'A' | sh` nested-body case remains conservative. This rule does not execute
  the body, infer its language, or claim to have found its hidden execution edges.
- **AFP-V0-9401:** (accepted under the owner's standing auto-accept (2026-10-11); V1-5033) A literal driver invocation of `git ls-files`, `git grep`, `find`, or `rg`, and a
  modelled `go run` invocation, SHALL retain a dependency on every repository file.
  It SHALL use the existing anchored ancestor glob `/*` in `Unit.PathGlobs` when the
  target or job has no computed-invocation fallback. The file-set edge matches additions,
  deletions, unindexed paths, dot paths and tested source without requiring the path to
  exist in the scanner's inventory. Flags, shell expansions in operands, changed working
  directories and file or directory symlinks SHALL NOT narrow that dependency. A Go
  program's package dependencies do not bound its runtime reads or its subprocess
  `Cmd.Dir`; these commands keep the whole-tree bound until a separate proof can narrow
  it. A pure command mentioning an enumerator as data does not itself enumerate.
  Ordinary Go build, vet and test invocations retain their existing package edges.
- **AFP-V0-9402:** (accepted under the owner's standing auto-accept (2026-10-11); V1-5033) A reached driver file-set reader SHALL also reach its driver dependents,
  including make prerequisites and workflow callers, even when a dirty path has an
  already tested owner. Existing computed fallbacks and their `FRONTIER_SITE` witnesses
  SHALL remain intact; an enumerator edge is not a proof that another command in the
  same target has bounded reads. An empty change selects no file-set reader. Unknown
  ownership and other frontier reasons remain visible. No new graph fields, plan kinds,
  reason codes or execution authority are introduced.

The owner-requested lane builds on `205d188261990927b4118f73fdd1c232ccc0e2c7` and retains
`a7d38e73236cea3a237ce972f3bd50e11e723c29` as its requested evidence base. V1-4684,
V1-4687 and V1-4690 already have the AZU driver, verified caller and runnable-consumer
implementations at that head. V1-5087 is left out: changing the stale-runtime-edge refusal
would violate an existing assertion, and integration into the trusted CI caller is outside
this lane's allowed paths. These facts and the pending integration checks are recorded in
`docs/build-log/2026-11-08-avdrv.md`; no runtime-edge compatibility relaxation is claimed.

## Non-goals

The parent contract's non-goals and authority rules apply. AFP-V0-551 does not narrow by suffix
literals, a runtime read trace, or symbol-level reachability inside a hub package (V1-3282,
V1-2744). It does not narrow a test-only edit's unbounded readers (V1-4143). The nested-module
reader path (`nested.go`) honours the marker only since AFP-V0-831. AFP-V0-552 does not add a Python tokenizer to the
adapter. AFP-V0-2200 does not narrow a declared read scope or an unbounded read: the repository's
`go list` guards whose scopes declare `internal/` (`internal/liveverify/provider/interlock`,
`internal/analyzerstructured`, `cmd/corvint-analyzer-rust`, `cmd/corvint-analyzer-shader`) stay
selected on any `internal/` edit by AFP-V0-023, and a header-only scope would be a new owner
declaration. AFP-V0-2205 does not sniff extensionless files elsewhere in the tree. AFP-V0-2240
and AFP-V0-2241 port no non-Go reader to the CI selector, and change neither its rules (a), (b)
and (d), the CEM evidence rule, nor declared read scopes (TSP-V0-980). No TypeScript token is narrowed: AFP-V0-2245
is withdrawn, and TypeScript keeps the component-run relation (AFP-V0-072 (3)). AFP-V0-2246 is
withdrawn; shell tokens derived through canonicalization stay selected. AFP-V0-2243 and
AFP-V0-2244 are disabled for 1.0: no temp root is accepted before a list operator, and
`batch-regen.sh` keeps its base callers. Round 9 does not change the planner's V1-1818
short-component rule (`wholeShortEnds`), which the CI selector does not apply. AFP-V0-2247 does not move the web mask out of
`internal/contextindex`, and the symbol rule for a registering `init` (V1-3282) is not changed:
measured, a sound rule would select 147 of 151 units for a contextindex body edit, because
contextindex's own registry lookups reach the registered profile (build-log entries
`2026-11-03-selprec2-ci-selector-parity` and `2026-11-03-selprec2-round3-typescript-anchors`).
AFP-V0-2480 to AFP-V0-2484 add no annotation or
tag inside test source: the declaration is one reviewed file, and a must-run or owner entry names a
test file or directory, so a Go entry selects the whole package that holds it. A pattern that
matches no file is not refused, because an owner may be declared before the files it owns exist.
AFP-V0-2487 does not change what `corvint test` runs or in which order; it orders the plan for a
runner or agent that reads it. It reads no CI history and no remote failure data, learns no
weights, and does not order the e2e-safe `selected` list.
AFP-V0-2400 infers no input from a test's
text; AFP-V0-2401 does not run Python, Node or other non-shell steps beyond the `shell-test` list,
and AFP-V0-2402 does not change the CI deciding selector. AFP-V0-2404 gives `Makefile` no consumer
mapping.
AFP-V0-2542 does not narrow an
import-path or climbing pattern, and the nested-module reader path (`nested.go`) keeps rule (d) for
every `/...` token. Its directory-move check reads only the package's own tokens: a helper in
another package that changes the directory, or a directory built by `filepath.Join` or string
concatenation into a command argument other than `Dir`, `-C` or a `cd` word, is not seen.
sniff extensionless files elsewhere in the tree. AFP-V0-2370 does not trace process argument
values through data structures (a gate or pool command a test defines keeps its test by
`STARTS_PROCESS`), does not model files shared through paths computed at run time (other than from a package
variable), child processes left running, or function and interface values one test stores in
shared state for another to call outside a package variable the second reads; the
`companionrelease` fixture applies the repository's ignore rules, which equals `HEAD`'s tracked set
only while no tracked file matches an ignore rule (none does on 2026-11-03), and does not narrow `cmd/corvint` or `internal/tasks/cli`, which stay unbounded
readers (AFP-V0-025) and keep their whole selection with that refusal named.
AFP-V0-2743 to AFP-V0-2745 do not narrow the unbounded readers a build-log entry selects:
`internal/doccorpus/selfcorpus` inventories the checkout's documentation, so `docs/build-log/` gets
no AFP-V0-772 exclusion. They do not narrow the CI selector's own rule (d) closure, which every
change selects (V1-2162), nor the declared `internal/` read scopes a `_test.go` edit selects. The
baseline record lists the current literal readers; giving them declared read scopes is follow-up
work. AFP-V0-2742 does not run a nested module on the `ci` profile.
AFP-V0-3143 does not add a command's test-only runners to the CI selector's rules (a) and (b): the
planner selects them one edge past a rebuilt command (`execUsersOf`, AFP-V0-037), and CI unions the
plan's packages. AFP-V0-3140 to AFP-V0-3143 do not narrow a code-run edge itself: a record prunes
import edges only, so a narrowed package's code runners are always followed.
AFP-V0-2990 to AFP-V0-2995 do not narrow AFP-V0-402's rule for which literal positions reach a
process, and they keep the language manifests' directory literals as code runs. They do not make a
path-literal, glob, declared-scope, existence or import-graph reader that has its own tests select
its runners, just as AFP-V0-021 does not select such a reader's importers. The runners of a command
that only an enclosed path reaches stay one edge past it (AFP-V0-037), as its importers stay
unwalked.
AFP-V0-3144 does not link a computed target whose file name is a glob (`"$dir"/chec?`): as under
TSP-V0-960, such a value is neither an edge nor a constructed name. AFP-V0-3146 and AFP-V0-3147 do
not model an environment input: a value whose unknown part is a name the file never sets
(`"$TOOLS/check"`) stays outside the model as TSP-V0-960 and failure mode (12) there state, because no
bound dropped a value the file gives. AFP-V0-3145 does not recognise `[[` after `time -p` or
`coproc`; the lexer splits such an expression at its operators and scans the pieces as commands. No
repository script uses either form.

## Failure modes

The parent contract's failure codes apply. A fixture marker on a literal the test really reads is
an author error. It narrows that read the same way ASD-V0-895 does for `test-read-scopes-check`, so
the marker is reviewable in the diff. A `whole_packages` entry the parser cannot compute keeps the
unit whole. A missing `python3` skips AFP-V0-552 rather than passing it. A `go list` reader whose call
the AFP-V0-2200 grammar does not recognise (arguments passed through a helper, a slice built in
another function) keeps whatever edges it had before and, if a `"list"` literal sits in the
calling function, reads the whole module; one with no recognisable call at all is not an
import-graph reader, as before. A test that sets `cmd.Dir` to another directory lists that
directory's packages while its targets are resolved against its module, as AFP-V0-402 resolves
command literals. A shell test that reads a repository file through a temp-root path it did not
create (a variable the AFP-V0-2204 rule accepts but a later redirection rebinds) is an author
error the confined CI shard surfaces. Since round 8 no Go reader is narrowed by placement, so a
reader whose root is supplied at run time stays selected in the planner, the CI selector and the
`-bounds` attribution of `tools/gate-ledger` (GL-V0-009) alike. Two placement rules remain and
are suspected gaps of the same class, recorded for their own tickets: the V1-0230 and AFP-V0-931
narrowing of CEM paths, and the V1-0290 file-name prefilter for a non-Go unit whose plugin does
not certify its roots. Since round 9 a literal that spells `..\x` keeps its bare `..`, so a test
package deep enough that `..` climbs to the repository root is an unbounded reader in the planner
and unresolved in the CI selector (fail closed); measured on this repository's lane tree, the CI
selector's unresolved packages are the same 39 as round 8's, each already unresolved by another
literal or a root-locating call. The
withdrawn AFP-V0-2245 dropped these TypeScript tests for `app/x/repo/lib.js`, each of which joins
an anchored `"/repo"` (or a glob) to a directory taken from data, the filesystem or a launcher its
literals do not spell, so it can read that path (cases 2, 6 and 12 show the shape; their fixtures
resolve the directory elsewhere), and each of which the component-run relation selects (the
re-review's list has no case 10): (1) `globSync('**/repo/*')`; (2)
`readdirSync(process.cwd())[0] + '/repo/lib.js'`; (3) an environment variable other than `PWD`,
`OLDPWD` and `INIT_CWD` (`FIXTURES=app/x node --test`); (4) a directory parsed from JSON
(`JSON.parse(readFileSync(...dirs.json)).app`); (5) a launcher outside the manifest's script text
(a shell script, a Makefile, a CI `working-directory`, an nx target's `cwd`); (6) a
`.mocharc.yml` whose `spec` climbs out of its directory (YAML, which the config check lexed as
script); (7) a Vitest `setupFiles` or `globalSetup` module or a Mocha `--require` module that
calls `process.chdir`; (8) a dynamic `import()` of a template literal, or a package `"imports"`
alias (`#dirs`), which the import closure did not follow; (9) identifiers turned into components
(`Object.keys({ app: 1, x: 2 }).join('/')`); (11) a case transform (`'APP/X'.toLowerCase()`); and
(12) a directory taken from `module.parent.filename` or an `Error().stack`. The withdrawn
AFP-V0-2246 allowlist missed (1) `if`, `!`, `while`, `for` and `time` prefixes before `eval`,
source, `enable`, `set -T`, DEBUG traps or dash aliases; (2) backticks and nested parameter
expansions whose slash vanishes; (3) an INT trap fired by `kill -INT $$`; (4) function names or
the `function` keyword split by a backslash-newline; and (5) a missing standalone `set -T`
regression. All these cases, and plain canonicalization without any override, MUST keep the
`internal/x/y.json` token. The CI PR
and merge-queue selector runs the pinned `CORVINT_PR_TOOL_SOURCE` build, so AFP-V0-2240 and
AFP-V0-2241, including round 8's removal of their placement filter, reach CI only with the next
re-pin; until then a pin from rounds 1 to 7 can drop a runtime-rooted reader.
An owner mapping that names the wrong tests for a path it
owns is an author error: the path's real readers are still selected by the reader rules, and only
the unowned-path unknown is lifted, as for a documentation declaration (AFP-V0-160). A mapping
whose tests no longer exist refuses the build (AFP-V0-2483). A must-run entry makes every plan
larger; it never makes one smaller. An unreadable `corvint test` receipt leaves the order with
that receipt `UNREADABLE` and contributes no failure; it never fails the plan. A test file
whose header cannot be parsed on either side changes its directory, as any unparsable file
does (AFP-V0-9010). A base that `SelectAt` cannot resolve or read never narrows a plan: it
falls back to the unknown comparison (AFP-V0-9011).
A stale AFP-V0-2400 declaration misses an input the
test later reads, as before the declaration; it can never narrow. An AFP-V0-2402 run set misses a
package only where the plan's reviewed rules exclude a real reader, which AFP-V0-2403 measures on
the failed-CI corpus; CI's decider still runs the attribution. A wrong AFP-V0-2404 reason is a
project authority error; the reader rules still select every literal, scoped and unbounded
reader.
A package added
below a leaf package makes the leaf's pattern an unbounded read again, so `unbounded-readers-check`
fails on the unrecorded unit until the record or the test changes (AFP-V0-2542). A package whose
tests set `Dir` for an unrelated command loses the exemption for its leaf patterns too; that is
conservative, and the read is recorded or the pattern rewritten as `"."`.
A literal that names a command directory but is only data, such as a language manifest, still
makes its holder a code runner whose importers rule (d) and the dependency traversal walk
(AFP-V0-2991, AFP-V0-2993). That over-selection is measured, not removed. A runner whose declared
read scope omits a read is an author error that the confined CI shard surfaces. On an unusable
binary-execs declaration the CI selector's rule (d) widens to every package, and the planner's
module-level `go:test-binary-execs-invalid` frontier sends the CI tier to its full run.
A live-tier test that regresses passes every PR shard and fails in any run without
`CORVINT_SKIP_LIVE_TIER=1`: the local selected run, batch-precheck's `make live-test`, the merge
queue, the nightly gates or a local `make gate`. A test that calls `RequireLiveTier` through a local
helper, or not in its own body, is not discovered by `make live-test`, but still runs in every
untagged run without the variable; AFP-V0-2701 reads only the test's own body. A CI step other than
the shard Go test step that copied the variable would silently skip the tier; AFP-V0-2702's
single-occurrence check refuses that.
A change to a trusted narrowing input that the guard admits by mistake would narrow a pull
request against its own machinery; the list is a reviewed file of the trusted checkout, the
AFP-V0-2722 fault injections hold it to the three inputs the charter named, and the merge queue,
which runs `mq-narrow.sh` from main, retests the merged tree. A pull request whose shard-plan
narrowed but whose shards record FULL (an input that differed between the two runs) runs the whole
suite over the sized matrix: slower, never smaller. A sized run leaves fewer `ci-outcomes-*`
artifacts than `ci-shard-cost-drift` expects, so that advisory report abstains, as it does for
every selected run; `ci-tested-tree-record` runs only when all sixteen `full<index>` outputs say
FULL, so a sized run never records a full tree. A selection run of another count that did not
narrow leaves its queue row INCOMPLETE, as before AFP-V0-2725. A pull request with narrowing on and
a MET verdict whose docs-plan decides DOCS or REUSE still runs the plan job (about 80 runner
seconds), because the job cannot read docs-plan's mode without waiting for it; nothing reads its
count. A file that an isolated helper build needs but does not copy fails that workflow's helper
build; AFP-V0-2726 catches it before merge, on the pull request that adds the file or the
dependency.
Had AFP-V0-3144's shortcut been kept for every literal non-shell file name, an edit of an
extensionless script that a caller runs through a computed directory would miss the caller's tests;
had AFP-V0-3145's `[[` been taken as a test anywhere in a command, an argument `[[` would hide the
next command, and an environment-named source there would lose its `shell:dynamic-source`. Each
has a fault-injection test. AFP-V0-3144 scans the shell units' paths once for each target it would
skip, which is linear in the shell file count and bounded by the targets a file holds.
Had AFP-V0-3146 and AFP-V0-3147 left a per-name or depth bound silent, its unknown part would read as
an environment input: a value naming an owned extensionless script would be dropped with no edge and
no `shell:dynamic-source`, and an edit of that script, or of anything a dynamic source it holds may
read, would miss the caller's tests. A file that legitimately gives a name more than 16 values now
selects its tests on every change; its precision cost is visible as an `UNBOUNDED_READER` witness
and in `.corvint/unbounded-readers.json`, never as a missed test.

## Acceptance evidence and traceability

The avdrv additions are build-only candidates; their new regressions have not been executed
in the lane. The integration branch must run the complete affected package and independent
review before promotion. See `docs/build-log/2026-11-08-avdrv.md` for check results and limits.

| Requirement | Implementation | Regression witness (pending execution) |
|---|---|---|
| AFP-V0-9400 | `driverShellScanner.script` in `internal/liveverify/affected/driver_shell.go` | `TestDriverHeredocConsumers_AFPV09400` in `internal/liveverify/affected/driver_avdrv_test.go`: wrapper, generated script, substitution, nested output, unclosed and tab-stripped body; workflow selection and quoted-text controls |
| AFP-V0-9401 | `driverShellScanner.simple`, `goInvocation`, `driverUnitBuilder.unit` | `TestDriverEnumerationSelectsFilesAndCallers_AFPV09401`, `TestDriverEnumerationDoesNotInventReaders_AFPV09401`, `TestDriverEnumerationSymlinkTargets_AFPV09401`: direct and nested commands, new/deleted/unindexed files, unknown directories, file and directory links, negative controls |
| AFP-V0-9402 | `Graph.driverSites` in `internal/liveverify/affected/driver.go` | `TestDriverEnumerationSelectsFilesAndCallers_AFPV09401`, `TestDriverEnumerationKeepsComputedFallback_AFPV09402`: tested-owner callers, unrelated exclusions, empty change, retained computed witnesses |

| Requirement | Implementation | Evidence |
|---|---|---|
| AFP-V0-2600 | `unownedOutsideRunnerSite`, `frontierUnknowns` in `internal/liveverify/affected/select.go` | `TestUnownedRunnerSiteLocality_AFPV02600`, `TestDeclaredOutputOutsideRunnerSite_AFPV02600`, `TestUnboundedRunnerInputsFailClosed_AFPV02600` (`frontier_unowned_locality_test.go`); `docs/build-log/2026-11-05-affected-unknown-frontier.md` |
| AFP-V0-548 | `deadlineCode` and its three calls in `internal/liveverify/affected/dirty.go` | `TestAFPV0548DeadlineExpiryIsGitTimeout` in `internal/liveverify/affected/dirty_deadline_test.go` |
| AFP-V0-549 | `selectionBudgetRounds`, `selectionRound`, `measureIncrementalSelection` in `internal/liveverify/affected/measure_test.go`; immutable `Graph.tokens` prepared by `finish` | `TestIncrementalSelectionMeetsTheLiveBudget` (three consecutive runs, build log `2026-11-01-aff-w26`); `TestPreparedPathTokensPreserveSelection` (2026-11-05 repair, exact cached/uncached plan parity) |
| AFP-V0-550 | `affectedWhole`, `withWholePackages`, `testMainSource`, `testMainOf` in `cmd/corvint/affected_whole.go`; `WholePackages` in `cmd/corvint/affected.go`; `cmd/corvint/affected_compact.go` | `TestAFPV0550TestMainChangeMakesThePackageWhole` in `cmd/corvint/affected_whole_test.go` |
| AFP-V0-551 | `FixturePathMarker`, `fileScan.fixture`, `fixtureOnly`, `scannedLiteral`, `fixtureTokens`, `pathTokens` and `observeDirectory` in `internal/liveverify/affected/golang/golang.go` | `TestAFPV0551FixtureMarkedLiteralsNameNoRead` in `internal/liveverify/affected/golang/fixture_marker_test.go`; before/after counts in build log `2026-11-01-aff-w26` |
| AFP-V0-552 | `astImports`, `differentialCorpus` in `internal/liveverify/affected/python/differential_test.go` | `TestScanImportsAgreesWithAst_V13322` |
| AFP-V0-553 | `ownDirectory` and the `+` operators of `braced` in `internal/liveverify/affected/shell/computed.go`; `commandStrings` in `internal/liveverify/affected/shell/shell.go` | `TestInvocationThroughVariables_AFP_V0_553` in `internal/liveverify/affected/shell/invocation_test.go`; `TestHeavyShEditKeepsShellReaders_V14170` in `internal/liveverify/affected/replay_v14170_test.go` |
| AFP-V0-830 | the integrations README listing in `script/gen-documentation-paths.sh`; `.corvint/documentation-paths.json` | `TestIntegrationReadmesAreDocumentation_AFP_V0_830` in `internal/liveverify/affected/integration_readme_documentation_repo_test.go`; `script/gen-documentation-paths_test.sh` case 6 |
| AFP-V0-831 | the `fixtureOnly` check on the token loop of `observeDependentModule` in `internal/liveverify/affected/golang/nested.go`; the marker in `tools/local-authority/codec_package_test.go` | `TestNestedModuleHonoursFixtureMarker_AFP_V0_831` in `internal/liveverify/affected/golang/fixture_marker_test.go`; `TestIntegrationReadmesAreDocumentation_AFP_V0_830` (repository plan) |
| AFP-V0-2200 | `goListScanFile`, `goListArguments`, `importGraphTemplate`, `isolatedList`, `literalEnvironment`, `goListUnits` in `internal/liveverify/affected/golang/importgraph.go`; the `listed` skip in `usedCommandLiterals` (`binaryexecs.go`); `Unit.ImportGraphReads` and `ImportGraphReadsAll` in `unit.go`, `graph.go` and `graphcache.go` | `TestGoListReadersAreImportGraphReaders_AFPV02200` in `internal/liveverify/affected/golang/importgraph_test.go`; `TestRepositoryImportGraphReaders_V14409_V14362` in `internal/liveverify/affected/importgraph_repo_test.go` |
| AFP-V0-2201 | `importGraphReadersOf`, `importClosure`, `WitnessImportGraphReader` in `internal/liveverify/affected/importgraph.go`; `terminalWitness` in `select.go` | `TestImportGraphReadersFollowImportSetChanges_AFPV02201` (body-only narrowing; import-added, out-of-closure, `go.mod` and no-comparison controls); `importGraphWitnessResolves` in `measure_test.go`; the fault-injection runs in build-log entry `2026-11-03-selprec-selection-precision` |
| AFP-V0-2202 | `ImportSetChangesAt`, `ImportSetRecords`, `packageHeader` in `internal/liveverify/affected/golang/importsets.go`; `unknownImportSets`, `ModuleFile`, `GoPackageFile` in `importgraph.go`; `planAffectedGraph` and `affectedMeasuredCommit` in `cmd/corvint/affected.go` | `TestImportSetRecordsCompareHeaders_AFPV02202`, `TestHiddenPackageImportSetSelectsObserver_AFPV02202` in `internal/liveverify/affected/golang/importgraph_test.go` |
| AFP-V0-2203 | the `goUnit && !resolvedEvidence` any-depth argument of `namesAny` in `readers` and the Go branch of `resolves` in `internal/liveverify/affected/readers.go`; `namesAt`; `namesFileOnlyWithin` only for certified non-Go units | `TestAnchoredTokensResolveFromModuleBases_AFPV02203`, `TestRuntimeRootedRunsKeepComponentRuns_AFPV02203`, `TestRuntimeRootedFileNamesStaySelected_AFPV02203` and `TestRuntimeRootedDirectoryWalkStaysSelected_AFPV02203` (round 8: an `os.Getenv("FIXTURES")+"/repo"` walker and a `filepath.Join` walker stay selected; `/elsewhere` control) in `internal/liveverify/affected/golang/anchored_test.go`; the amended cases of `TestFileNameOnlyReadersMustPlaceThePath_AFPV0072`, `TestFileNameOnlyReadersKeepWhatTheyCanReach_AFPV0072` and `TestChangeEvidenceReadersAreNarrowed_V1_0230`; `TestReadersMatchesDirtyMajorScan` (certified-unit narrowing) |
| AFP-V0-2204 | `tempRoots`, `unconditionalAssignment`, `populates`, `assignedOnce`, `firstAssignment`, `underTempRoot`, `tempExpansion` and the temp step of `pathTokens` in `internal/liveverify/affected/shell/shell.go` | `TestTempRootPathsAreNoTokens_AFPV02204` (plain root dropped; recursive copy, tree copy, archive, worktree, link, conditional and used-first controls keep the token); the redirection cases of `TestEmptyCompoundsAndRedirectedAssignmentsKeepTokens_AFPV02243` (round 7: `3>&9`, `2>/dev/null` and leading redirections keep the token, run under dash); `TestWriteOnlyWordsAreNoPathTokens_NSAV0016` |
| AFP-V0-2205 | `ExtensionlessRoots`, `ExtensionlessScript`, `shellShebang`, `shellExecutables` and `Owns` in `internal/liveverify/affected/shell/shell.go` | `TestOwnsOnlyShellSuffixes_NSAV0006`; `TestExtensionlessShellExecutablesAreUnits_AFPV02205` in `internal/liveverify/affected/shell/shell_test.go` |
| AFP-V0-2206 | `Plan.Deleted` in `internal/liveverify/affected/select.go`; `affectedDeletedPaths` and its call in `planAffectedGraph` (`cmd/corvint/affected.go`); `cmd/corvint/affected_compact.go` | `TestAffectedPlanMarksDeletedPaths_AFPV02206` in `cmd/corvint/affected_deleted_test.go` |
| AFP-V0-2240 | `readers` and `pathReaders` in `internal/gateselect/readers.go` (plain `namesPath`, no placement filter); `packageBounds` in `select.go` | `TestRuleCAnchoredTokens_AFPV02240`, `TestRuleCRuntimeRootedRuns_AFPV02240`, `TestRuleCRuntimeRootedFileNames_AFPV02240` and `TestRuleCRuntimeRootedDirectoryWalk_AFPV02240` (round 8: walker and joined walker in readers, verdict and `-bounds`; `/elsewhere` control) in `internal/gateselect/naming_test.go` |
| AFP-V0-2241 | `parentBackslash` and its additive use in `pathTokens` in `internal/gateselect/readers.go` | `TestRuleCLoneTokens_AFPV02241` (lone, directory-shaped, climbing, prefixed `foo..\notes` and round 9 `split` tokens; `-bounds` keeps `app/prefoo../notes` and `notes`); `TestRuleCParentBackslashKeepsWrittenTokens_AFPV02242` (round 9: the `strings.Split` reader of `foo..\notes` is selected for `notes` in readers, the verdict and `-bounds`, with a `foo..\other` control); the `..\x` cases of `TestFastTierLiteralLexiconDiffers_AFPV0765`; the fail-before runs in build-log entry `2026-11-04-selprec2-round9-additive-lexicon` |
| AFP-V0-2242 | the test itself; `pathTokens`, `spelledTokens`, `appendSpellings`, `parentBackslash`, `moduleAnchored` and `closesTag` in `internal/gateselect/readers.go`; `literalTokens`, `spelledTokens` and `appendSpellings` in `internal/liveverify/affected/golang/golang.go` | `TestRuleCMatchesThePlanner_AFPV02242` in `internal/gateselect/naming_test.go` (round 6 adds the closing-tag packages; round 8 the walker, joined walker and `prefix` packages; round 9 the `split` and `srcwalk` packages and the unbounded and unresolved sets, with no intended difference); `TestRuleCParentBackslashKeepsWrittenTokens_AFPV02242` and `TestParentBackslashKeepsWrittenTokens_AFPV0506` (round 9, in `internal/liveverify/affected/golang/anchored_test.go`); the fault-injection runs in build-log entries `2026-11-03-selprec2-ci-selector-parity`, `2026-11-04-selprec2-round6-runtime-roots`, `2026-11-04-selprec2-round8-runtime-roots-retained` and `2026-11-04-selprec2-round9-additive-lexicon` |
| AFP-V0-2243 | disabled for 1.0 (round 9): the base trail refusal of `&&`, `\|\|`, `\|` and `&` in `unconditionalAssignment` in `internal/liveverify/affected/shell/shell.go`; kept, only widening: the one-word assignment refusal there, the refusing-operator lead after a compound closer in `endCommand` and the whole-operator advance (`lex.go`) | the `or exit` and `and cd` cases (kept since round 9) and the `piped`, `background`, `list in background`, `or list in background`, `or reassign` and `or read` cases of `TestTempRootPathsAreNoTokens_AFPV02204`; `TestBackgroundedCompoundListsKeepTokens_AFPV02243` (round 6: brace group, subshell, `if`, `while`, `for`, `case`, `time` and after-compound cases; round 9: every list after the assignment is kept, with lone-assignment controls); `TestEmptyCompoundsAndRedirectedAssignmentsKeepTokens_AFPV02243` (round 7: empty `case` lists in the background under bash and dash; round 9: own-line, newline-list and right-redirection lists kept, with a lone-assignment control); the fault-injection runs in build-log entry `2026-11-04-selprec2-round7-file-names-and-redirections` |
| AFP-V0-2244 | disabled for 1.0 (round 9): `.corvint/shell-foreign-callers.json` is the base declaration | the repository declaration check (`TestRepositoryShellForeignCallersComplete_TSP_V0_005` in `internal/liveverify/affected/shell_callers_repo_test.go`); `tools/declaration-compat-check`; the `heavy.sh` replay in the build log |
| AFP-V0-2245 | withdrawn; no implementation | `TestTypeScriptAnchoredCounterexamplesStaySelected_AFPV02245` in `internal/liveverify/affected/typescript/anchored_counterexamples_test.go` (the 17 counterexample cases stay selected; each fails against the withdrawn implementation at `7f7458dc9`) |
| AFP-V0-2246 | Withdrawn: canonicalization helpers removed; `tempRoots` and `pathTokens` in `internal/liveverify/affected/shell/shell.go` retain the pre-canonicalization rule | `TestTempRootPathsAreNoTokens_AFPV02204` (all canonical cases kept; literal temp-root controls unchanged); `TestCanonicalizationCounterexamplesStaySelected_AFPV02246` (all five review classes and sub-cases, standalone `set -T`, direct and assigned derived paths, shell variants) |
| AFP-V0-2247 | `Impact` and `StaticLoad` in `internal/webdialect/webdialect.go`; `flowWebCodeLines` in `internal/contextindex/flowlexical.go` and the registration in `dialect_hooks.go`; the TypeScript plugin's paths in `internal/langplugin` and `internal/langplugin/decl`; the Tasks export prefix in `internal/companionrelease/export.go` | `TestImpactProfileIsUnchanged_AFPV02247` in `internal/webdialect/webdialect_test.go`; the unchanged `internal/contextindex` and `internal/langplugin` suites; the contextindex replay in the build log |
| AFP-V0-2480 | `DeclaredSelectionPath`, `readTestSelection`, `validTestSelectionReason` in `internal/liveverify/affected/testselection.go`; the `testSelection` member of `declarations`, `digestBody` and `Canonical` (`graph.go`) and of `cacheEntry` (`graphcache.go`); the declaration list of `declarationPath` (`documentation.go`) | `TestTestSelectionDeclarationGrammar_AFPV02480` in `internal/liveverify/affected/testselection_test.go` |
| AFP-V0-2481 | `validOwnerPattern`, `ownerPatternMatches` in `internal/liveverify/affected/testselection.go` | `TestOwnerPatternGrammar_AFPV02481` |
| AFP-V0-2482 | `ownersOf` (called at the end of `reach`), `ownedPaths` (called in `selectPlan`), `WitnessDeclaredTestOwner`, `ClassDeclaredTestOwner`; `terminalWitness` in `select.go` | `TestDeclaredOwnerSelectsItsTests_AFPV02482` (the `docs/contracts/**/*.json` example; negative controls: an unmatched path, an `UNINDEXED_SOURCE_PATH`); `TestDeclaredTestOwnersWithoutDeclaration_AFPV02482` |
| AFP-V0-2488 | `frontierNonSource` in `selectPlan` (`select.go`), which passes `frontierUnknowns` the `nonSource` groups without the `DECLARED_TEST_OWNER` group | `TestOwnerKeepsFrontierUnknowns_AFPV02488` in `internal/liveverify/affected/testselection_test.go` (two frontier-bearing plugins; negative control without an owner; fault injection: passing the owned group makes the plan `BOUNDED` with the ts frontier excluded) |
| AFP-V0-2489 | `DeclaredOwner`, `declaredOwnersOf` in `testselection.go`, `Plan.DeclaredOwners`; `DeclaredOwners` in `affectedCompactPlan`; `tierAffectedPlan` and `tierDeclaredSelections` in `cmd/corvint/affected_tiers.go` | `TestDeclaredOwnerSelectsItsTests_AFPV02482` (member, with an unmatched control); `TestMustRunAndOwnerAreTierZero_AFPV02485` (compact member); `TestDeclaredUnitsAreTierZeroWhateverReachedThem_AFPV02489` in `cmd/corvint/affected_testselection_test.go` (undeclared control) |
| AFP-V0-2483 | `resolveTestSelection` in `testselection.go`, called by `build` in `graph.go` | `TestContradictedTestSelectionRefuses_AFPV02483` (full build refuses; partial build skips) |
| AFP-V0-2484 | `mustRunOf` and `Plan.MustRun`; the hook before the selection loop of `selectPlan` | `TestMustRunSurvivesEveryExclusion_AFPV02484` (no dependency path, documentation-only change, empty dirty set control) |
| AFP-V0-2485 | the tier-0 case of `tierAffectedSelections` (`cmd/corvint/affected_tiers.go`); `MustRun` in `affectedCompactPlan`; the must-run and declared-owner branches of `testGoSlices`, `testDeclaredOwnerReason` (`cmd/corvint/test_slices.go`) | `TestMustRunAndOwnerAreTierZero_AFPV02485` in `cmd/corvint/affected_testselection_test.go`; `TestMustRunUnitRunsWhole_AFPV02485`; `TestDeclaredOwnerUnitRunsWhole_AFPV02485` (undeclared control) |
| AFP-V0-2486 | the `declared-must-run` reason and the `mustRun` skip of `selected` in `internal/appflows/selection.go` | `TestE2ESafeKeepsMustRunTests_AFPV02486` in `internal/appflows/selection_mustrun_test.go` |
| AFP-V0-2487 | `affectedFailFastOrder`, `readFailFastReceipts` in `cmd/corvint/affected_order.go`; the `--fail-fast-order` parse and its refusals in `cmd/corvint/affected.go`; `Order` in `affectedCompactReceipt` | `TestFailFastOrderPutsRecentFailuresFirst_AFPV02487`, `TestFailFastOrderKeepsTheMostRecentReceipts_AFPV02487` (the 64 newest by time, ties by name) and `TestFailFastOrderUnlistableDirectoryIsUnreadable_AFPV02487` (absent-directory control) in `cmd/corvint/affected_order_test.go` |
| AFP-V0-2540 | `droppedDeclarations` and the `likely cause` line of `run` in `tools/unbounded-readers/main.go` | `TestAFPV2540RefusalNamesTheDroppedDeclaration` in `tools/unbounded-readers/main_test.go` (a dropped declaration; controls: a package HEAD never declared, no commit) |
| AFP-V0-2541 | `droppedUnrecorded`, `droppedDeclarations` and the `dropped` report of `run` in `tools/test-read-scopes-check/main.go` | `TestDroppedDeclarationNamesTheRatchetEntry_AFP_V0_2541` in `tools/test-read-scopes-check/main_test.go` (controls: a package the record lists, no commit) |
| AFP-V0-2542 | `leafPattern`, `directoryMove` and `packageDirectories` in `internal/liveverify/affected/golang/unbounded.go`, their calls in `observeDirectory` and `pathTokens` (`golang.go`); `leafPattern`, `directoryMove`, `goDirectories`, their calls in `record`, `scanSource` and `indexRepositoryMemo` in `internal/gateselect/readers.go`, and the `movesDirectory` memo field in `scanmemo.go` | `TestLeafPackagePatternParity_AFPV02542` in `internal/gateselect/leaf_pattern_test.go` (both analyzers agree, and a cold and a warm scan memo agree; bounded: a leaf, a data-only subtree, Go files only under `testdata`, a `Dir` set to `"."`; unbounded: a package below, a nested module, an import-path, a climbing, a deep and a wildcard pattern, and a leaf pattern in a package that sets `cmd.Dir` to a `runtime.Caller` root, sets `Dir` in another file or in a composite literal, calls `t.Chdir`, or passes `-C` or a `cd` shell word) and `TestAFPV2542LeafPackagePatternIsNoUnboundedRead` in `tools/unbounded-readers/main_test.go` (a package added below makes the read unbounded again) |
| AFP-V0-920 | `affectedTempUnavailable` and `auditAffectedChange` in `cmd/corvint/affected_audit.go` | `TestAffectedAuditTempUnavailableNamesTMPDIR_AFPV0920` (a missing `TMPDIR` exits 2 with `temp-unavailable`, the path and the `TMPDIR` advice) |
| AFP-V0-832 | `affectedDocumentationOnly` and its two checks in `compileAffectedGraph` (`cmd/corvint/affected.go`, `cmd/corvint/affected_freshness.go`); the `--path` paragraph of `corvint help affected` | `TestPathListToleratesDirtyDocumentation_AFP_V0_832` in `cmd/corvint/affected_path_test.go` |
| AFP-V0-2440 | `unownedUnknown` and `fixtureOnlyClaim` in `internal/liveverify/affected/select.go` | `TestFixtureOnlyClaimIsNoUnindexedSource_AFPV2440` in `internal/liveverify/affected/fixtures_test.go` (two reclassified shapes; five kept: unhosted, indexed below, outside testdata, a Go claim, a second indexing claimant); `TestRubyFixturesOfANewGoPackageAreNoUnindexedSource_V14446` in `internal/liveverify/affected/replay_v14446_test.go` (real plugins; a no-`go.mod` negative control); both fail with the rule disabled; live replay of PR 85 in build log `2026-11-03-mqfix` |
| AFP-V0-2400 | `internal/liveverify/affected/shell/testinputs.go` (`readShellTestInputs`, `applyShellTestInputs`) and its call in `shell.go`; `shellTestInputsChanged` in `internal/liveverify/affected/readers.go` and its call in `select.go`; `.corvint/shell-test-inputs.json`; the `tools/declaration-compat-check` entry | `TestShellTestInputsDeclaration_AFPV02400` and `TestShellTestInputsDeclarationChanged_AFPV02400` in `internal/liveverify/affected/shell/testinputs_test.go`; the `inputs changed` case of the `tools/shell-test-select` step test; the before/after and fault-injection runs in build-log entries `2026-11-03-selbugs-selection-correctness` and `2026-11-04-selbugs-review-round-d` |
| AFP-V0-2401 | `tools/shell-test-select/main.go`; `make shell-test-affected` | `TestAFPV02401SelectsTheShellTestStepsThePlanSelects`, `TestAFPV02401RepositoryStepsMapToTheirScripts`, `TestAFPV02401IncompleteReceiptsRunEveryStep` in `tools/shell-test-select/main_test.go`; the fault-injected `script/batch-regen.sh` run |
| AFP-V0-2402 | `testPlanRunSet` in `cmd/corvint/test.go` | `TestAFPV02402RunSetIsThePlanGoSelection` in `cmd/corvint/test_whole_reasons_test.go`; ten merged ranges in the build-log entry |
| AFP-V0-2403 | `checkAgreement`, `nestedModuleUnits`, `Summary.TestExtra`, `Summary.TestOmitted`, `Summary.TestMisses` and `Failures` in `tools/selection-baseline/main.go` | `TestSelectionBaselinePinsTestAffectedAgreement_AFPV02403`; `TestAFPV02403NothingManifestMissesFailingPackages` (a NOTHING manifest's failing package is a test miss while the CI selector selects it; a NOTHING manifest without failures whose plan selects a package fails the gate; an unrecognized scope pinned); `TestAFPV02403ManifestOmitsPlannedPackage` (a NARROWED manifest that omits a planned package on a change with no failure fails the gate, a nested-module unit and a non-Go plan unit excepted); `TestAFPV02403NestedModuleUnitIsNotOmitted` (an excluded dependent nested module unit with a go.mod is not omitted; an excluded root package, a path outside the root module and one escaping the clone stay compared); `make selection-baseline-check` |
| AFP-V0-2404 | `.corvint/runtime-readers.json` | the per-path plan summaries in the build-log entry |
| AFP-V0-2405 | `driverInputUnownedHint`, `driverInputHints` in `internal/liveverify/affected/documentation.go` and its call in `select.go` | the `Makefile` plan in the build-log entry |
| AFP-V0-2406 | `frontierSites`, `planFrontierSites` in `internal/liveverify/affected/graph.go`; `frontierUnknowns` returning the participants in `select.go` | `TestAFPV0381PlanCountsRepositoryFrontierSites` (amended), `TestFixtureFrontierBearsOnlyWhereAGoUnitHostsIt_AFPV0446`; regenerated `cmd/corvint/testdata/affected-selection-*.golden` |
| AFP-V0-2360 | `currentSourceFixture` in `internal/companionrelease/source_fixture_test.go`; its use in `exported_current_test.go`; `-buildvcs=false` in `companion_cli_contract_test.go` and `tasks_export_closure_test.go`; `.corvint/test-read-scopes.json`; `.corvint/unbounded-readers.json` | `go test -exec test-confine ./internal/companionrelease/` (build log); `TestCurrentTasksSourceArchiveBuildsOffline`, `TestCurrentExportedTasksSmokeGenerator` |
| AFP-V0-2361 | `manifestArchiveTestRepository`, `sharedArchiveTestRepository`, `initArchiveTestRepository` in `conformance/release-artifact-v0/archive_gate_test.go`; `newPublicationFixture` in `publication_status_test.go`; `.corvint/test-read-scopes.json` | `go test -exec test-confine ./conformance/release-artifact-v0/` (build log); `TestArchiveDestinationRequiresTrustedAbsentExternalChild`, `TestBuildArchivesCancellationRemovesScratchStageAndOutput` |
| AFP-V0-2362 | the `CORVINT_HOST_ARCHIVE_PROOF` check in `conformance/release-artifact-v0/corvint_archive_test.go`; the job env of `.github/workflows/release-gates.yml` | `TestCorvintHostArchivePartialProof` (skips naming the variable) |
| AFP-V0-2363 | the two declarations and `.corvint/unbounded-readers.json` | `make unbounded-readers-check`; planner runs and per-merge saving in build-log entry `2026-11-03-floor-typed-reach` |
| AFP-V0-2370 | `typedReachPackages`, `testTypedSlices` in `cmd/corvint/test_typed_slices.go`; its call in `testGoSlices` | `corvint test --dry-run` replays in build-log entry `2026-11-03-floor-typed-reach` |
| AFP-V0-2371 | `buildGraph`, `funcDecl`, `packageVar`, `walkInitializer`, `heldAssignment`, `visitExpr`, `call`, `composite`, `convertTo` in `internal/liveverify/affected/golang/typedreach/graph.go`; `Load`, `runnerSourceParity` in `load.go`; `namedIn` in `analyze.go` | `TestAFPV02371ConversionsAndHeldValuesCarryReach`, `TestAFPV02371RunnerConfigurationRefusesHiddenSource` |
| AFP-V0-2372 | `childOnly`, `moduleHazard`, `hasLinkname`, `Analyze`; `Config`, `BuildFlags`, `ProbeConfig`, `LoadFor`, `LoadBaseFor` in `typedreach`; `sessiontest.GoExecutor`; `typedReachExecutor` in `cmd/corvint/test_typed_slices.go` | `TestAFPV02372AlwaysRunReachKeepsThePackageWhole`, `TestAFPV02372LoadRefusesUnmodelledSource`, `TestAFPV02372ExecutorConfigSelectsConstrainedSource`, `TestAFPV02372BuildFlagsClassifiesExecutorFlags`, `TestAFPV02372ProbeConfigReadsTheExecutorEnvironment`, `TestAFPV02372DryRunBindsExecutorBuildConfiguration`, `TestAFPV02372RunRefusesAnotherConfiguration`, `TestAFPV02372ExplicitSwitchValueRefuses`, `TestAFPV02373KeepsOnlyTheTestsReachingAChangedFunction` (negative control) |
| AFP-V0-2373 | `Analyze`, `reach`, `provenTool`, `externalTools`, `processStarters` | `TestAFPV02373KeepsOnlyTheTestsReachingAChangedFunction`, `TestAFPV02373RoundThreeRegressions` (constants, `os/exec` uses, ioutil and `os.Root`) |
| AFP-V0-2374 | `Analyze`, `conflict`, `sharedVar`, `fileWrite`, the `setters` of a node, `streamOutput`, `streamInput`, `assignedElsewhere`, `opaqueArguments`, `opaqueParameter`, `stateWriters`, `stateReaders`, `processState`, `exemptStateMethods`, `readOnlyReceivers`, `queryOnly`, `rootVar` | `TestAFPV02373KeepsOnlyTheTestsReachingAChangedFunction` (`TestRead`); `TestAFPV02374ReviewRegressions`, `TestAFPV02374ReReviewRegressions`, `TestAFPV02373RoundThreeRegressions` (process-start writes, `os.Stdout` and `fmt`), `TestAFPV02374RoundFourRegressions` (interface-held references, discarded-temp privacy, `os.File` handles, `net/http` defaults, fixed-port listeners), `TestAFPV02374AllowListRegressions`, `TestAFPV02374LoggerReceiverSettersKeepOrderedReaders`, `TestAFPV02374SignalCleanupCannotEraseWrites`, `TestAFPV02374StoredMethodValuesCarryImplicitReads`, `TestAFPV02374SignalStopWritesSignalState`, `TestAFPV02374StreamOutputWritesFiles`, `TestAFPV02374IndirectStdCallsKeepOpaqueArguments`, `TestAFPV02374RangeAssignmentEndsPrivacy` (value, key, parenthesized value, key and assignment), `TestAFPV02374ScanReadsFiles`, `TestAFPV02374SetupWritersOfKeptReadsAreKept` (a setup flag, through a helper, transitive setup, a process-wide and a fixed-path setup; the full fixture fails, the kept test alone passes, in build-log entry `2026-11-07-mbapproved-setup-writers`) |
| AFP-V0-2375 | the slice and whole forms in `testTypedSlices` | `corvint test --dry-run` replays (build log) |
| AFP-V0-2376 | `DiffFile`, `FileChange.Declarations`, `ImportPath`, `fileDecls` in `typedreach/diff.go`; `resolve`, `BaseEffects` in `analyze.go`; its use in `testTypedSlices` and `tools/typed-reach-measure` | `TestAFPV02376DiffFileNamesChangedDeclarations`, `TestAFPV02376MovedPositionsKeepDependentReaders`, `TestAFPV02376ImportPathOfRootPackageFiles`, `TestAFPV02376DryRunReadsRootPackageChanges`, `TestAFPV02376BaseConstraintChangeRefuses`, `TestAFPV02376BaseEffectsPreserveRemovedWrites`, `TestAFPV02376BaseEffectsSeeHeldCallbacks`, `TestAFPV02376BlankInitializersKeepOwnKeys` |
| AFP-V0-2377 | `typedReachChangedPaths`, `typedReachUnitRefusal`; `Graph.NamesPath`, `Graph.GlobRead` in `internal/liveverify/affected/readers.go` | `corvint test --dry-run` replays (build log); `TestAFPV02370DryRunNarrowsAnAllowListedDependent` (embedded and run-time data files) |
| AFP-V0-2378 | `tools/typed-reach-measure/main.go` | fault-injection oracle runs in build-log entry `2026-11-03-floor-typed-reach` |
| AFP-V0-2740 | `deletedFixture`, `fixtureSubtree` in `internal/liveverify/affected/deletedfixtures.go`, the call in `seed` and the unowned-path listing in `participants` (`select.go`) | `TestDeletedFixtureIsFixtureData_AFPV2740` in `internal/liveverify/affected/deletedfixtures_test.go` (no unknown, hosting package and literal reader selected; seven kept cases; fails with the call removed); `TestDeletedFixtureStaysUnownedForOtherPlugins_AFPV2740` there (a deleted Python fixture still bears a third plugin's path-open entry; fails with `participants` reading `claimantsOf` alone); the PR #114 replay in build-log entry `2026-11-06-selgo` |
| AFP-V0-2741 | the shared plan; rules (b) and (c) of `internal/gateselect` unchanged | the PR #114 replay in build-log entry `2026-11-06-selgo` (driver and shadow narrow, shadow a subset of the driver) |
| AFP-V0-2742 | the module branch of `Build` and `ModuleCheck` in `internal/sessiontest/plan.go`; the `UnobservedModule` comment in `internal/gateselect/allowlist.go` | `TestCIProfileNamesTheNestedModuleSuite_AFPV2742` in `internal/sessiontest/module_ci_afpv2742_test.go` (fails with the profile condition restored); the amended ci case of `TestSTRV0064NestedModuleUnit`; `TestDriverAndShadowShareOneUnknownAllowlist_AFPV0982`; the PR #105 replay |
| AFP-V0-2743 | `recordDirectory`, `resolvesRecord`, `BuildLogDirectory` and the record branch of `resolves` in `internal/liveverify/affected/readers.go` | `TestRecordEntriesSelectOnlyTheirReaders_AFPV2743` and `TestRecordDirectory_AFPV2743` in `internal/liveverify/affected/records_test.go` (both fault injections fail it); the PR #106 replay |
| AFP-V0-2744 | `recordDirectory`, `resolvedReaders` in `internal/gateselect/select.go`; `resolvesRecord` and the record branch of `resolvingReaders` in `internal/gateselect/readers.go` | `TestRecordEntryReadersResolveToTheDirectory_AFPV2744` in `internal/gateselect/records_afpv2744_test.go` (both fault injections fail it); the amended `TestSelectPackagesNarrowsSealedChangeEvidenceReaders`; the PR #106 replay |
| AFP-V0-2745 | `Graph.RecordReaders`, `namesRecordAncestor` in `internal/liveverify/affected/recordreaders.go`; `tools/unbounded-readers/records.go` and its call in `run`; `.corvint/record-readers.json` and its `notPlanner` entry in `tools/declaration-compat-check` | `TestAFPV2745RecordDirectoryReadersRatchet` in `tools/unbounded-readers/records_test.go`; the `RecordReaders` cases of `TestRecordEntriesSelectOnlyTheirReaders_AFPV2743`; `make unbounded-readers-check` on the repository |
| AFP-V0-2746 | `reachesRecord`, `recordAncestor`, `joinsComponents` and the record branch of `resolves` in `internal/liveverify/affected/readers.go`; `recordAncestor`, `heldComponents` and the `joins` pass of `resolvingReaders` in `internal/gateselect/readers.go` | `TestRecordEntryReadersJoinSeparateLiterals_AFPV2746` in `internal/liveverify/affected/records_test.go` (fails with the join removed from `reachesRecord`); `TestRecordEntryJoinedReadersSelectedByBothSelectors_AFPV2746` in `internal/gateselect/records_afpv2746_test.go` (real Go source; fails with the join removed from either selector); the PR #106 replay in build-log entry `2026-11-07-selgo-fix1` |
| AFP-V0-2747 | `Graph.RecordReaders`, `recordProbes`, `globEntry`, `globItems` and `suffixPrefix` in `internal/liveverify/affected/recordreaders.go`; `recordDirectories` and the walker refusal of `checkRecordReaders` in `tools/unbounded-readers/records.go` | `TestRecordReadersMatchTheEntryNamespace_AFPV2747` in `internal/liveverify/affected/records_test.go` (fails with `recordProbes` reduced to its fixed probe); `TestAFPV2747RecordRatchetMatchesTheEntryNamespace` in `tools/unbounded-readers/records_test.go` (fails with the fixed probe alone and with walkers admitted); `make unbounded-readers-check` on the repository |
| AFP-V0-2748 | `Graph.RecordReaders`, `coversRecord`, `globCoversRecord`, `globCovers`, `globAlphabet` and `classBounds` in `internal/liveverify/affected/recordreaders.go` | `TestRecordWalkerNotMaskedByNarrowerReads_AFPV2748` in `internal/liveverify/affected/records_test.go` (fails with a narrower read masking the walker); `TestAFPV2748RecordRatchetRefusesAMaskedWalker` in `tools/unbounded-readers/records_test.go` (fails without the fix); `TestRecordWalkerWithGlobSkippedByBothSelectors_AFPV2748` in `internal/gateselect/records_afpv2748_test.go`; `make unbounded-readers-check` on the repository |
| AFP-V0-2749 | `admittedDeletion` in `internal/liveverify/affected/deletedtests.go`, its calls in `seed` and `participants` (`select.go`) | `TestDeletedGoSourceStaysUnownedForOtherPlugins_AFPV2749` and `TestAdmittedDeletionsStayUnownedInParticipants_AFPV2749` in `internal/liveverify/affected/deletedsources_test.go` (both fail with `participants` testing only `deletedFixture`); the one-file deletion replays in build-log entry `2026-11-07-selgo-fix4` |
| AFP-V0-2750 | `AllowedUnknown` and `FrontierSuite` in `internal/gateselect/allowlist.go`; the `FrontierSuite` case and `suite` lines of `unknownNarrowing` and `selectPackagesFrom` in `internal/gateselect/select.go`; `frontierSuite` and its call in `Build` (`internal/sessiontest/plan.go`); the shell-test-select allowance; the `cfamily` entry of `.github/cishards/test-commands.json` and of `languageRunners` in `tools/test-commands-check/main.go`; `dryRunManifest` and `testIncomplete` in `tools/selection-baseline/main.go` | `TestFrontierSuiteRunsInCorvintTest_AFPV2750` and `TestFrontierSuiteNamedByGateAffectedSelect_AFPV2750` (`internal/sessiontest/frontiersuite_afpv2750_test.go`; both fail with the blanket allowance restored); `TestDriverAndShadowShareOneUnknownAllowlist_AFPV0982`; `TestSelectionBaselineReadsAnIncompleteDryRun_AFPV2750`; the replays in build-log entry `2026-11-07-selgo-fix5` |
| AFP-V0-2610 | `deletedGoSource` (was `deletedGoTest`) in `internal/liveverify/affected/deletedtests.go` and its call in `seed` (`select.go`); `goSourceRule` in `structure.go` | `TestDeletedGoSourceIsAttributedToItsPackageAndImporters_AFPV02610` and the amended `TestMovedGoTestIsAttributedToItsBasePackage_AFPV0620` in `internal/liveverify/affected/golang/deletedtests_test.go` (fails before the change: `UNINDEXED_SOURCE_PATH provider/extra.go`); amended to the new rule: `TestDeletedGoSourceSelectsItsPackageAndImporters_V1_0340` (no unindexed entry), the two AFP-V0-191 tests in `golang/unobserved_test.go` and the Go case of `TestSeamWidensRatherThanNarrowingOnUnknownInput` (a new unindexed file is written to disk), and the two abstention rows of `TestUseCaseHostileChangeConsequence` (a directory left with no package); the PR #114 replay and a one-file deletion replay in build-log entry `2026-11-06-selection-audit` |
| AFP-V0-2950 | `comparisonOnlyLocators`, `boundedJoin`, `boundedJoinArgument`, `boundedJoinConsumed`, `boundedRead` and `literalComparison` in `internal/liveverify/affected/golang/locatorproof.go` and the identical `internal/gateselect/locatorproof.go`; their calls in `pathTokens` (`golang.go`) and `scanSource` (`gateselect/readers.go`) | `TestBoundedJoinLocatorIsNoRead_AFPV02950` in `internal/liveverify/affected/golang/locator_selection_test.go` (a join read through `os.ReadFile`, `os.Stat`, `os.ReadDir` or a bound variable is selected only by the path it names; returned, re-assigned, `Dir`-climbed, computed-join, `..`-join controls stay rule (d), and so do the review's `Dir`-widened joined path, a returned join, an `os.Open` join, a rebound joined variable and a joined path handed to a function); `TestUnprovenLocatorInitializerSelection_AFPV02950`, `TestLocatorInitializerAllowlist_AFPV02950` and `TestUnprovenInitializerParity_AFPV02950` cover environment, unknown-call, parameter and unknown-write roots; `TestBoundedJoinLocatorParity_AFPV02950` in `internal/gateselect/locator_parity_test.go` (both analyzers agree; cold and warm memo agree); counts in build logs `2026-11-06-unbounded-readers` and `2026-11-07-unbounded-readers-review-fix` |
| AFP-V0-2951 | `driverGoBuildOnlyCommands`, `goInvocation`, `goPattern` and `embedded` in `internal/liveverify/affected/driver_shell.go`; `buildPaths` in `driver.go`; `Unit.BuildPathImports` (`unit.go`, `graphcache.go`, `graph.go`); `buildOnly` in `resolvePathImports` and `callerEdges` and the edge filter of `unboundedReaders` (`graph.go`); `traverseWhere` in `select.go` | `TestCommandLocatorReachesItsRunners_AFPV02951` in `internal/liveverify/affected/golang/locator_selection_test.go` (a real Makefile: `go run`, `go build` and its dependent target and a vet-and-run target are selected on an unrelated change through the command's runtime read; `go vet` and `go list` targets are not, and a vet target is still selected by an edit of what it vets); `TestRepositoryScriptsAreBoundedSources_V14283` (the five commands locate the root); counts in build log `2026-11-07-unbounded-readers-review-fix` |
| AFP-V0-2952 | Superseded by AFP-V0-2962; the implementation was removed in review fix 3, and its history is in build logs `2026-11-06-unbounded-readers`, `2026-11-07-unbounded-readers-review-fix` and `2026-11-07-unbounded-readers-review-fix-2` | `TestPrintedClimbKeepsRuleD_AFPV02962` and `TestPrintedClimbParity_AFPV02962` pin the replacement |
| AFP-V0-2953 | the source branch of `observe` (`internal/liveverify/affected/shell/shell.go`), which marks every source no literal or fresh directory resolves dynamic | `TestEnvironmentNamedSourceStaysDynamic_AFPV02953` in `internal/liveverify/affected/shell/shell_test.go` (five environment-named forms stay dynamic, and a test containing only `. "$ENVFILE"` is selected by an edit of the repository's `lib.sh`); the `envfile.sh` near miss of `TestFreshTemporaryDirectorySourceIsBounded_TSP_V0_004`; `TestRepositoryScriptsAreBoundedSources_V14283` in `internal/liveverify/affected/unbounded_readers_v14283_repo_test.go`; `TestHeavyShEditKeepsShellReaders_V14170` |
| AFP-V0-2954 | `conflicting` in `resolver`, its assignment in `newResolver` and the count check of `base` (`internal/liveverify/affected/shell/shell.go`) | `TestSameValuedReassignmentResolves_AFPV02954` in `internal/liveverify/affected/shell/shell_test.go` (the dogfood-finish.sh form links; controls: differing values, `local`, `read`, `for`, `+=`); `TestReassignedVariableIsNotADirectoryForm_NSAV0008` (unchanged) |
| AFP-V0-2955 | `.corvint/shell-read-scopes.json`, `.corvint/unbounded-readers.json`, `unwrap_shell` in `script/heavy.sh` | `TestRepositoryScriptsAreBoundedSources_V14283` (heavy.sh dynamic only through its environment-named source, dogfood-finish.sh linked, qualify-tasks-basic_test.sh read-scoped, dogfood-cache_test.sh on the floor); `make unbounded-readers-check`; `make test-read-scopes-check`; `make selection-baseline-check`; the plans in build logs `2026-11-06-unbounded-readers` and `2026-11-07-unbounded-readers-review-fix` |
| AFP-V0-2956 | `feeds`, `flush` and the substitution stack of `script`, the `$(...)` and backtick branches of `word`, `scanShellOutput` and the `buildOnly` condition of `goInvocation` in `internal/liveverify/affected/driver_shell.go`; `scanBothOutput` in `internal/liveverify/affected/driver.go` | `TestBufferedListingProgramSelection_AFPV02956` and `TestDriverListOutputAllowlist_AFPV02956` cover buffered programs, unknown templates/options and certified path formats; `TestDriverListingOutputKeepsExecution_AFPV02956` in `internal/liveverify/affected/driver_shell_test.go` (the review's `go list ./lib \| xargs -I PKG go test PKG`, `\|&`, `cd`, `$(...)`, quoted `$(...)`, backticks, a while-read loop, a process substitution, piped `go vet`, a piped `sh -c` all give lib an execution edge; output to a file, `\|\|` and `&&` keep the build-only edge); `TestListingOutputReachesItsRunner_AFPV02956` in `internal/liveverify/affected/golang/locator_selection_test.go` (a real Makefile: the review's target, a backtick target and a while-read target are selected through lib's root locator on an unrelated change; a `go list > pkgs.txt` target is not); `TestBufferedListingOtherPackageSelection_AFPV02956` (cross-package buffered template and unknown option retain the driver for runtime configuration edits; all three certified formats preserve precision) |
| AFP-V0-2957 | `driverXargs`, the `xargs` branch of `command`, the replacement check and the `s.fed` condition of `simple`, `embedded`, the `fed` parameter of `goInvocation` and `makeInvocation` in `internal/liveverify/affected/driver_shell.go` | `TestDriverListingOutputKeepsExecution_AFPV02956` (xargs, `xargs -a`, `parallel` and `find -execdir` go runs import every owner; a fed `go vet` keeps the build-only class; a fed make call is computed); `TestDriverStdinProgramIsComputed_AFPV02958` (an interpreter xargs runs, an xargs replacement command word, `find -exec {}`, `parallel {}`); `TestDriverTruncatedWrapperOptions_AZU_V0_001` (`xargs -I` stays computed) |
| AFP-V0-2958 | `operand` and the `-s`, `-`, `-o`/`+o` and `+` branches of the interpreter loop in `simple` (`internal/liveverify/affected/driver_shell.go`) | `TestDriverStdinProgramIsComputed_AFPV02958` (`\| sh`, a heredoc-fed `bash`, `bash -s`; a named script, an inline `-c` program and `bash -eu script` stay uncomputed) |
| AFP-V0-2959 | Superseded by AFP-V0-2962; the implementation was removed in review fix 3, and its history is in build log `2026-11-07-unbounded-readers-review-fix-2` | `TestPrintedClimbKeepsRuleD_AFPV02962` (the commands `printed`, `streamed`, `raw` keep rule (d)) pins the replacement |
| AFP-V0-2960 | Superseded by AFP-V0-2962; the implementation was removed in review fix 3, and its history is in build log `2026-11-07-unbounded-readers-review-fix-2` | `TestImportedCaptureKeepsClimb_AFPV02962` and `TestCapturedStreamKeepsClimb_AFPV02962` pin the replacement |
| AFP-V0-2962 | rule (d) marks of `observeDirectory` and the literal loop of `pathTokens` (`internal/liveverify/affected/golang/golang.go`) and of `record`, `scanFile` and `scanSource` (`internal/gateselect/readers.go`), which hold no printed-text branch; `scanMemoEntry` (`internal/gateselect/scanmemo.go`), which carries no exemption state; both `outputproof.go` files removed | `TestImportedCaptureKeepsClimb_AFPV02962` in `internal/liveverify/affected/golang/text_literal_test.go` (the review's exact example: `tool` prints `"../config%s\n"`, its test calls the imported `capture.Read(main)`; `tool` keeps rule (d) with `locatesRoot` and an edit of `config.go` selects it); `TestPrintedTextReachesItsRunner_AFPV02962` (a make target that runs a printing command and opens what it printed is an unbounded reader on an edit of `config.go` and on an unrelated edit); `TestPrintedClimbKeepsRuleD_AFPV02962` and `TestCapturedStreamKeepsClimb_AFPV02962` (every earlier fixture keeps rule (d), the formerly exempt commands and `quiet` included); `TestImportedCaptureParity_AFPV02962`, `TestPrintedClimbParity_AFPV02962` and `TestCapturedStreamParity_AFPV02962` in `internal/gateselect/locator_parity_test.go` (both analyzers agree, cold and warm memo agree, and the selector's verdict for an edit of `config.go` runs `tool`); each new regression fails at `c083f6927`; counts in build log `2026-11-07-unbounded-readers-review-fix-3` |
| AFP-V0-2990 | the `codeRunners` class of `traverseWhere` and `testRunnersOf` (`internal/liveverify/affected/select.go`); their use in `unboundedReaders` (`graph.go`) | `TestRuleDReachesGoRunRunner_AFPV02990` (the review's exact `go run` example: an edit of `config.go` selects `runner` as `UNBOUNDED_READER` caused by `tool`) and `TestRuleDReachesGetwdRunner_AFPV02990` (round 3's `os.Getwd` example) in `internal/liveverify/affected/golang/binary_runner_selection_test.go`; both fail at `c80f5059c`; counts in build log `2026-11-07-binary-runners` |
| AFP-V0-2991 | `Unit.TestOnlyExecs` (`unit.go`, `graph.go`, `graphcache.go`); the source-first split of `literalExecs` and `applyBinaryExecs` (`golang/binaryexecs.go`); the test-only runtime edge of `applyRuntimeEdges` (`runtime_edges.go`); `codeRunners`, `testRunners` and `runnerOnly` (`graph.go`); the `runnerOnly` branch of `participants` (`select.go`) | `TestRuntimeUncertaintyFollowsExecutionEdges_AFPV02991` (a wrapper command, a code-running library and its importer and a test-only runner are reached; the test-only runner's importer and a non-locating command's runner are not); `TestExecsSplitCodeFromTestRuns_AFPV02991`; `TestDeclaredRuntimeEdgeSelectsTheSpec_AFP_V0_819` (unchanged); `TestBatchReplayDeclaredRuntimeReaders_V12941` (a documentation plan gains no TypeScript frontier); `viaEdges` in `measure_test.go` accepts a code-run hop |
| AFP-V0-2992 | `testlessReaderUsers` and `scopeExcludes` (`internal/liveverify/affected/readers.go`) | `TestRuntimeUncertaintyFollowsExecutionEdges_AFPV02991` (the runner of the test-less `reporter` is selected by an edit of `data/report.txt`); `TestTestlessReaderRunnersKeepReadScopes_AFPV02992` (a runner whose read scope excludes the path is not selected; one whose scope holds it is); `TestDeclaredReadScopesNarrowSpecAndRequirementsPlans_TSP_V0_900` (unchanged); `TestSealOnlyCEMDeltaStaysBounded_V13571` (a seal selects the runners of a CEM reader's dependent command) |
| AFP-V0-2993 | the `codeRunners` class of `traverseWhere`, which `traverse` and so `additiveTraverse` take; `execUsersOf` (`execs.go`) | `TestBuildReachWalksCodeRunners_AFPV02993` (a library change that rebuilds `tool` selects a code runner's importer and the runner of a command whose code runs `tool`; a test-only runner's importer is not selected); `TestSelectionOnTheLiveDirtyWorktree` and `TestAppmapOnlyChangeSelectsOnlyJustifiedUnboundedReaders_V1_0999` resolve every chain |
| AFP-V0-2994 | `internal/gateselect/runners.go` (`linkRunners`, `commandTargets`, `applyBinaryExecs`, `readBinaryExecs`, `runReach`); `command` in `scanFile`, `record` and `goPackage` (`readers.go`); the `execsErr` branch and `runReach` call of `unresolved`; the memo's `command` entry (`scanmemo.go`) | `TestGoRunRunnerParity_AFPV02990`, `TestGetwdRunnerParity_AFPV02990`, `TestExecutionEdgeParity_AFPV02992` (both analyzers agree, cold and warm memo agree, the verdict runs each runner), `TestBinaryExecsDeclarationWidens_AFPV02994` and `TestRunnerEdgesNeverNarrowerThanThePlanner_AFPV02994` in `internal/gateselect/binary_runner_parity_test.go`; `TestReachMatchesThePlannerGraph_V12727` (unchanged) |
| AFP-V0-2995 | `.corvint/unbounded-readers.json` | `make unbounded-readers-check`; `TestPrecisionReplay_AFPV09003`; the before and after plans in build log `2026-11-07-binary-runners` |
| AFP-V0-3140 | the code-run class in `reachedThrough` (`internal/liveverify/affected/additive.go`) | `TestAdditiveNarrowingFollowsCodeRuns_AFPV03140` in `internal/liveverify/affected/golang/additive_exec_test.go` (the review's case, by a `go run` literal and by a declared run: `harness` falls back naming `quiet`, `consumer` is selected); both cases fail at `ec11d63c3`; build log `2026-11-07-additive-code-runs` |
| AFP-V0-3141 | `reachedThrough` follows only the walk's classes; `execUsersOf` adds a test-only runner past the walk | `TestAdditiveNarrowingKeepsUnrunRecords_AFPV03141` (an additive change alone and a test-only run keep `NARROWED` and leave `consumer` out) |
| AFP-V0-3142 | `traversalEdges` and the class constants in `internal/liveverify/affected/select.go`; their use in `traverseWhere`, `reachedThrough` and `follows` | `TestReachedThroughCoversEveryTraversalClass_AFPV03142` in `internal/liveverify/affected/additive_classes_test.go` (one injected edge per listed class demotes the record; it fails for the code-run class when `reachedThrough` drops it) |
| AFP-V0-3143 | the `codeRunners` edges of `reach` (`internal/gateselect/readers.go`) and the code-runner start of `attributeNarrowed` (`internal/gateselect/select.go`) | `TestNarrowedRunnerFollowsTheChangedCommand_AFPV03143`, `TestNarrowedRunnerKeepsItsNarrowing_AFPV03143` and `TestNarrowedCommandKeepsItsRunners_AFPV03143` in `internal/gateselect/additive_runner_parity_test.go` (the structural verdict holds the planner's narrowed selection; the declared case and the narrowed command each fail without their mechanism); `TestReachMatchesThePlannerGraph_V12727` and `TestSelectStructuralHonorsAdditiveNarrowing_ADNV0006` (unchanged) |
| AFP-V0-3144 | the shortcut and `ownedExtensionless` in `internal/liveverify/affected/shell/computed.go` | `TestComputedTargetKeepsOwnedExtensionlessScripts_AFPV03144` in `internal/liveverify/affected/shell/computed_extensionless_test.go` (a loop over `script` and a file-relative value link `script/check`, its edit selects the test that runs it and not another script's, a cut list naming `check` is a dynamic source; `go` and `check.py` stay bounded, and fail when the shortcut is deleted); the before/after plans in build log `2026-11-07-unbrd-fix6` |
| AFP-V0-3146 | `named` and `truncated` in `internal/liveverify/affected/shell/computed.go`; the `computed` closure of `observe` (`internal/liveverify/affected/shell/shell.go`) | `TestPerVariableBoundKeepsTheCallerSelected_AFPV03146` in `internal/liveverify/affected/shell/computed_bounds_test.go` (the review's reproducer: a 17-word loop running `"$dir/check"` beside a `script/check` that sources `"$ENVFILE"` is `shell:dynamic-source`, and edits of `config/settings.env` and of `script/check` select the wrapper test; it fails at `aa16cbf1a`); the 16-value control in `TestEveryExpansionBoundIsACut_AFPV03147` |
| AFP-V0-3147 | `truncated` at the depth bounds of `braced` and `named` in `computed.go`; `commandStrings` and its use in `observe` (`shell.go`) | `TestEveryExpansionBoundIsACut_AFPV03147` in `computed_bounds_test.go` (17 plain and braced assignments, a five-name chain, a five-deep default, a five-deep `:+` alternative and a 17-valued `sh -c "$cmd"` each make the test `shell:dynamic-source`, and each fails at `aa16cbf1a`; an environment-input directory stays bounded, and a 16-value loop links `script/check` with no unbounded witness); build log `2026-11-07-unbrd-fix7` |
| AFP-V0-3148 | the distinct-value `add` of `variableValues` in `computed.go` | `TestRepeatedValuesCountOnce_AFPV03148` in `computed_bounds_test.go` (six loops over the same three words link `script/check` with no unbounded witness; it fails when `add` keeps duplicates); `script/gen-spec-requirements_test.sh`, whose `mode` repeats 6 distinct flags across 7 loops, stays bounded in build log `2026-11-07-unbrd-fix7` |
| AFP-V0-3149 | `unrolled`, `unrollSeed`, `selfReference`, `topLevel` and their use in `named` (`computed.go`) | `TestComposedSelfReferencesKeepSelection_AFPV03149` in `computed_bounds_test.go` (default/alternative, unknown-transform, prefix-directory, opaque-prefix and unknown-prefix suffix selection, composed-default and working-directory execution edges); `TestSelfReferenceUnrollsOnceWhenNoValueIsLiteral_AFPV03149` in `computed_bounds_test.go` (`b="$PWD/$b"`, `dir=${dir:-script}` and two unrolled names in one chain stay bounded, the second linking `script/check`; a single unrolling keeps the `script/run.sh` edge and the constructed name's frontier; `x="../$x"`, `${y:-abc$x}` and a two-name cycle are `shell:dynamic-source`; the bounded cases fail without `unrolled`); `TestRepositoryScriptsAreBoundedSources_V14283`, `TestRepositoryInboundCallers_AZU_V0_8901` and `TestDeclaredReadScopesNarrowSpecAndRequirementsPlans_TSP_V0_900` (unchanged) keep the dogfood wrappers bounded |
| AFP-V0-3145 | `openTest` and `testLeaders` in `internal/liveverify/affected/shell/lex.go` | `TestConditionalOpensOnlyInCommandPosition_AFPV03145` in `internal/liveverify/affected/shell/conditional_position_test.go` (an argument `[[` before `&&`, `||` or `\|` keeps the next source; the test after every leader keeps its operators, and fails without the leaders); `TestCasePatternsAndConditionalsAreNotCommands_NSAV0008_NSAV0009` (unchanged) |

| AFP-V0-2700 | `RequireLiveTier`, `liveTierSkipped`, `LiveTierSkipEnv` in `internal/testsupport/livetier.go`; `liveTier` in `livetier_default.go` and `livetier_live.go`; the `RequireLiveTier` calls in 25 tests of `internal/liveverify/affected`, `internal/companionrelease` and `cmd/corvint` | `TestRequireLiveTierRunsUnlessThePRShardSkips_AFPV02700` and the fault-injection `TestBrokenLiveTestFailsAnOrdinaryRun_AFPV02700` in `internal/testsupport/livetier_test.go` (with and without `-tags live`); before/after package seconds in build-log entry `2026-11-06-ci-speed` |
| AFP-V0-2701 | `LiveTests`, `rootModuleSource`, `liveTestsInFile` in `.github/cishards/live.go`; `tools/live-test-run/main.go`; the `live-test` target in `Makefile`; `live-test` in `EXTRA_TARGETS` of `script/batch-precheck.sh` | `TestLiveTestsInFileFindsMarkedTopLevelTests_AFPV02701`, `TestRootModuleSourceMatchesGoTestReach_AFPV02701` (including a module-root file) in `.github/cishards/live_test.go`; the `make -s live-test` assertion in `script/batch-precheck_test.sh`; `TestArgvRunsOnlyTheLiveTestsWithTheTag_AFPV02701`, `TestRunExecutesEachLivePackage_AFPV02701` in `tools/live-test-run/main_test.go` |
| AFP-V0-2702 | `CORVINT_SKIP_LIVE_TIER` in the `go-product-shard` Go test step, the `go-live` job and the `go-product` needs in `.github/workflows/ci.yml` | `TestRepositoryLiveTierIsExcludedFromPRShards_AFPV02702` in `.github/cishards/live_test.go`; the workflow diff; an observed merge-queue run is pending the owner's ci-control-plane review |
| AFP-V0-2720 | `.github/forward-rows/github-guard.sh` (the input list and the `-z` diff); the guard call, `githubInputs` and the reason in `.github/forward-rows/narrow.sh` and `narrow.jq` | `TestAFPV02720GuardNamesOnlyTheNarrowingInputs_AFPV02720` in `tools/ci-forward-row/github_guard_test.go`; the `PR-owned github edit stays full` and `an unreadable range stays full` cases of `TestAFPV0098SupersetGuardSidecar`; the `.github/` cases of `TestAFPV0731MergeQueueSelection`; replay of PR #120 (run 42294716102) and PR #111 (run 42233489680) in build-log entry `2026-11-06-selgh` |
| AFP-V0-2721 | `section` and the `jobs` list in `.github/forward-rows/github-guard.sh` | `TestAFPV02721GuardComparesTheNarrowingJobsOfCIYML_AFPV02721` (every narrowing job, the top-level text, a moved key, a deleted file, an anchor; another job's edit, a new job and a deleted job narrow) |
| AFP-V0-2722 | the real-git harness `guardRepo` in `tools/ci-forward-row/github_guard_test.go` | `TestAFPV02720NarrowShFaultInjection_AFPV02720` in `tools/ci-forward-row/narrowing_sized_test.go` (narrow.sh, the confine wrapper and the selection step each FULL; an unrelated workflow, ci-cadence data, another job and code-only each NARROWED) |
| AFP-V0-2723 | `Size`, `SizeFrom`, `PredictedWallMs`, `MinNarrowedShards`, `DefaultWallBudgetMs` in `.github/cishards/size.go`; `--size-budget-ms` and `--min-shards` in `.github/cishards/cmd/main.go`; `size.go` in `profileNames` | `TestAFPV02723SizeIsTheFewestShardsWithinTheBudget_AFPV02723`, `TestAFPV02723UnitsAboveTheBudgetWidenTheBound_AFPV02723`, `TestAFPV02723SizeRefusesOrKeepsTheMaximumWhenItCannotPrice_AFPV02723` in `.github/cishards/size_test.go`; `TestAFPIsolatedBuildIgnoresModuleRedirection` (the isolated build carries `size.go`); the PR #106, #111 and #120 sizings in build-log entry `2026-11-06-selgh` |
| AFP-V0-2724 | the `shard-plan` job, the `go-product-shard` matrix and `needs`, the workflow-level pins, the `SHARDS` of `go-product` and `ci-narrowed-tree-record` in `.github/workflows/ci.yml`; `.github/forward-rows/shard-plan.sh` | `TestAFPV02724ShardPlanSizesTheMatrix_AFPV02724` and `TestAFPV02724ShardPlanOverlapsDocsPlan_AFPV02724` (the plan job needs only `narrowing-verdict`, and every reader of its outputs skips or ignores them on DOCS and REUSE) in `tools/ci-forward-row/narrowing_sized_test.go`; `TestAFPV0097WorkflowBindings`, `TestAFPV0591CoverageGuard`, `TestCINV0011ToolsBuildFromComparedMain` and `TestCINV0010PinLagRefusesNarrowing` (`pinlag_test.go`), `TestAFPV0734NarrowedRecordJob` and `TestAFPV0024WorkflowShardCountsAgree` (`tools/ci-reuse-plan`) on the workflow text; an observed sized pull-request run is pending the owner's ci-control-plane review and a `CORVINT_PR_TOOL_SOURCE` that carries `shard-plan.sh` |
| AFP-V0-2725 | `$counts`, `$m` and `$no` in `.github/forward-rows/row.jq`; `narrowedRecordOf`, `recordedNarrowed` and `recordShards` in `tools/ci-reuse-plan/main.go`; the REUSE check of the docs-plan reuse step in `ci.yml` | `TestAFPV02725RowsAcceptTheSelectionRunsShardCount_AFPV02725` in `tools/ci-forward-row/narrowing_sized_test.go`; the sized cases of `TestAFPV0734ReusesANarrowedPullRequestRunOfTheSameRange` in `tools/ci-reuse-plan/main_test.go` |
| AFP-V0-2726 | the `size.go` copy in `.github/workflows/linecov-index.yml`; the go-static `-run` pattern in `.github/workflows/ci.yml` | `TestIsolatedHelperBuildsCopyWhatTheyNeed_AFPV02726` in `.github/cishards/isolated_copies_test.go` (it fails with `partition.go embeds size.go` and the isolated vet error when `size.go` is dropped from the linecov copy list) |
| AFP-V0-9009 | `inboundRuntimeSiteFile`, `ownedTest` and their calls in `inboundRuntimeSite` (`internal/liveverify/affected/inbound_runtime.go`) | `TestLinelessInboundRuntimeSite_AFP_V0_9009` (declared directory edge selects the caller and closes the entry; the undeclared fault injection keeps it unknown) `TestInboundRuntimeSiteFile_AFP_V0_9009` and `TestInboundRuntimeSiteAmbiguity_AFP_V0_9009` in `internal/liveverify/affected/inbound_runtime_test.go`; `TestDeclaredInboundRuntime_AZU_V0_8902`; the before/after repro in build-log entry `2026-11-07-selmin` |
| AFP-V0-9010 | `packageHeader`, `readHeaderFile`, `platformFree`, `testSummary` and `Source` in `ImportSetRecords` (`internal/liveverify/affected/golang/importsets.go`); `goListFile.unproven` and `goListFile.proven`, the `confinedCommand` check of `goListScanFile` and the `observesSource` pass of `goListUnits` (`golang/importgraph.go`) and the allowlist `observesSource`, `inertImports`, `classifiedImports`, `allowedDirectives`, `observingMethods`, `confinedCommand` and `ownTestdata` (`golang/sourcereaders.go`), and `commandDirectoryProven` (`golang/commanddirectories.go`); `ImportSetChanges.Source`, `Graph.SourcePath`, `unknownImportSets` and the `ImportGraphReadsFiles` case of `importGraphReadersOf` (`internal/liveverify/affected/importgraph.go`); `Unit.ImportGraphReadsFiles` in `unit.go`, `graph.go` and `graphcache.go` | `TestTestFileImportEdges_AFP_V0_9010` in `internal/liveverify/affected/golang/importsets_tests_test.go` (kept sets change no import set and select no proven reader, but a new kept-set file or a test body edit still selects the unproven `-json` reader wide; added import, role move, shrink, constraint, platform name, foreign clause, embed, cgo and test-only directory each still change it and select guard, slice and wide); `TestFoldedTestFilesKeepUnprovenReaders_AFP_V0_9010` (a duplicate-import test file added, or a test file renamed, selects the unproven file-name reader names and not the proven guard; dropping `Source` loses it; an added import selects both); `TestPackageDocReaderSelected_AFP_V0_9010` (a package comment edit selects the computed-target `{{.Doc}}` reader with chain lib, docs and not guard; dropping `Source` loses it); `TestUnknownReaderFallsBackToRun_AFP_V0_9010` (34 channels each make a proven reader unproven and selected by a body-only edit, among them an `os.Symlink` into testdata, an aliased `exec.Command` and `exec.CommandContext`, a function value of `exec.Command`, an `exec.ExitError` type, a package-level command, an `Args` index swap, a command passed to `json.Unmarshal`, and the unclassified `net/http/cgi` import, `reflect.ValueOf`, an unknown `//go:` directive, `t.Chdir` and a computed `t.Setenv`; an own-testdata read, an environment read and a confined command with literal `Env`, `Dir`, `Stderr`, `ProcessState` and `reflect.DeepEqual` stay proven and unselected); `TestReaderEnvironmentIdentity_AFP_V0_9010` (custom and shadowed receivers and an unknown environment producer retain folded-test-addition selection; standard, aliased and literal controls); `TestReaderBuiltinIdentity_AFP_V0_9010` (package/local append, argument append, shadowed conversion and unknown environment retain folded-test-addition selection; builtin control); `TestCallerFilenameProvenance_AFP_V0_9010` (line/block/column directives and unknown filename transform retain remapped-target import selection; physical filename control) |
| AFP-V0-9010 | Same implementation as the preceding AFP-V0-9010 row | `TestCommandDirectoryFallsBack_AFP_V0_9010` (computed byte directory and unknown origins retain import and body selection; derived root and fresh temporary directory controls); `TestUnknownDirectoryListingIsNotExecution_AFP_V0_9010` (computed directory, Git stdout and unknown origins retain import/body reads without a false execution edge; unknown command methods and escapes retain that edge); `TestExistingTestdataSymlinkFallsBack_AFP_V0_9010` (existing directory, root and file symlinks retain body selection); `TestTestdataUnknownEntryFailsClosed_AFP_V0_9010` (regular-file control, unknown filesystem modes and lookup error); `TestFilesystemMutatorsObserveSource_AFP_V0_9010` (filesystem mutators and unknown members fail closed as calls and values); `TestMutatorSeesFoldedTestFileAddition_AFP_V0_9010` (computed-path mutator errors and an unknown operation retain selection on a kept-import test addition, with import-only and dropped-Source controls); `TestSourcePath_AFP_V0_9010` (`.go`, testdata `.go`, `.s`, `.syso`, `.c` and embedded-file paths give the Source, a README or a non-embedded text file none; an assembly file selects the unproven reader wide and a README does not); `TestImportSetRecordsCompareHeaders_AFPV02202`; the file-set reader case of `TestBuildRejectsUnitsThatBreakTheSeamContract` in `internal/liveverify/affected/affected_test.go` |
| AFP-V0-9011 | `SelectAt`, `SelectBetween` in `internal/liveverify/affected/languages/select.go`; their calls in `affectedTestRows` (`cmd/corvint/prove.go`), `FlowImpactAt` (`internal/appflows/impact.go`), `Compile` (`internal/delta/compile.go`); the `ImportSetChangesAt` call in `compileSnapshotAffected` (`cmd/corvint/affected.go`) | `TestSelectAtComparesImportSets_AFP_V0_9011` (real Git base: body edit drops the reader, import edit keeps it, no base falls back) and `TestSelectBetweenComparesImportSets_AFP_V0_9011` in `internal/liveverify/affected/languages/select_test.go` |
| AFP-V0-3260 | the `shard-plan` job's `if:`, `pin-main` and plan steps in `.github/workflows/ci.yml`; the gate choice in `.github/forward-rows/shard-plan.sh`; `$qcounts` and `$no` in `.github/forward-rows/row.jq` | `TestAFPV03260ShardPlanSizesMergeQueueEntries_AFPV03260` and the sized queue cases of `TestAFPV02725RowsAcceptTheSelectionRunsShardCount_AFPV02725` in `tools/ci-forward-row/narrowing_sized_test.go`; `TestCINV0011ToolsBuildFromComparedMain` (the plan and shard `pin-main` bodies are equal and the plan runs for both events) and `TestAFPV0540PullRequestParallelismBinding`; an observed sized queue run is pending merge |

## Rollback

For AFP-V0-9400..9402, revert the avdrv scanner, builder and driver-dependent traversal
changes together. No persisted wire migration or declaration rewrite is required. Reversion
restores the known wrapper-heredoc and literal enumerator selection gaps; keep those cases
UNKNOWN or run the full affected scope until a replacement passes integration verification.

Revert the lane's commits for the requirement and delete its row here. For AFP-V0-9009, cut the
site at its first `:` again. For AFP-V0-9010, make `packageHeader` add every test file's own
line, so every test-file header change counts again; keep `ImportSetChanges.Source`, `observesSource`
and `Unit.ImportGraphReadsFiles`, which only add selections. For AFP-V0-9011, call `affected.Select`
directly in the four callers: they fall back to the unknown comparison, which drops no reader. For AFP-V0-2720 to
AFP-V0-2722, restore the `git diff --quiet ... -- .github/` check in `narrow.sh` and delete
`github-guard.sh`: every `.github/` change is FULL again. For AFP-V0-2723 to AFP-V0-2725, delete
`shard-plan.sh`, `size.go` and the `shard-plan` job, restore the literal sixteen-shard matrix and
`SHARDS: "16"` in `go-product` and `ci-narrowed-tree-record`, and restore the `== $n` shard-count
checks in `row.jq` and the `shards` equality in `narrowedRecordOf`; the pins may stay at the
workflow level. Either half reverts alone. For AFP-V0-2726, delete
`.github/cishards/isolated_copies_test.go` and its alternative in the go-static `-run` pattern; the
`size.go` copy in `linecov-index.yml` stays while `partition.go` embeds it. For AFP-V0-3260, drop the
`merge_group` clause from the `shard-plan` job's `if:`: queue entries keep sixteen shards again. The `$qcounts` choice in `row.jq` may stay: an unsized queue run names sixteen. For AFP-V0-2370 to
AFP-V0-2378, emptying `typedReachPackages` restores whole-package selection; the package and the
tool can then be deleted. For AFP-V0-2360 to AFP-V0-2363, deleting a declaration and restoring its
`.corvint/unbounded-readers.json` row returns the package to the floor; the fixtures stay correct. For AFP-V0-551, removing
`fixtureTokens` makes marked literals read paths again; the markers are inert comments. For
AFP-V0-830, drop the integrations listing from `script/gen-documentation-paths.sh` and run
`make documentation-paths`. For AFP-V0-2200 to AFP-V0-2202, delete
`internal/liveverify/affected/importgraph.go`, `golang/importgraph.go` and `golang/importsets.go`,
the two `Unit` fields and the `WithImportSetChanges` call: every `go list` literal is a command
edge again. AFP-V0-2203 and AFP-V0-2240 to AFP-V0-2242 need no rollback of round 8, which only
widens selection: reverting its commit restores round 7's Go placement in `readers.go` and
`internal/gateselect/naming.go`, which drops runtime-rooted readers. For AFP-V0-2204, call `pathTokens` without temp roots; for AFP-V0-2205, drop `ExtensionlessScript`
from `Owns` and the file walk; for AFP-V0-2206, drop `Plan.Deleted`; for AFP-V0-2243, restore `&&` and `||` to the trail
refusal (round 9 did so). Rounds 6 to 9 only widen selection, so reverting a round's commit
restores the previous round's relation (round 7's lexer change alters only command leads, which
every consumer refuses alike); for AFP-V0-2244, move the three `internal/dogfoodflow` files back to
`callers` (round 9 did so). Reverting round 9's code commit restores round 8's substituting
lexicon, which skips the `strings.Split` reader of `foo..\notes`. AFP-V0-2245
and AFP-V0-2246 are withdrawn and need none; for AFP-V0-2247, move `Impact` back into
`internal/contextindex` as the `webImpact` variable and delete `internal/webdialect`. Run
`make spec-requirements` after deleting this file.
For AFP-V0-2440, make `fixtureOnlyClaim` return false; such paths are
`UNINDEXED_SOURCE_PATH` again and the CI selector falls back to FULL for them. Run
`make spec-requirements` after deleting this file. For AFP-V0-2480 to AFP-V0-2486, AFP-V0-2488 and AFP-V0-2489, delete
`internal/liveverify/affected/testselection.go`, the `testSelection` declaration member, the two
`selectPlan` hooks, `Plan.MustRun`, `Plan.DeclaredOwners`, `tierAffectedPlan` and the e2e-safe reason; a repository's
`.corvint/test-selection.json` then refuses nothing and selects nothing, because no reader reads it,
and its paths fall back to `UNOWNED_DIRTY_PATH`. For AFP-V0-2487, drop the flag and
`cmd/corvint/affected_order.go`; plans without the flag are unchanged either way.
For AFP-V0-2400, delete
`shell/testinputs.go`, `shellTestInputsChanged` and its call, and `.corvint/shell-test-inputs.json`; for AFP-V0-2401, delete
`tools/shell-test-select` and the `shell-test-affected` target; for AFP-V0-2402, restore
`testAttributionReasons`; for AFP-V0-2403, drop `checkAgreement`; for AFP-V0-2404, remove the
entries; for AFP-V0-2405, drop `driverInputHints`; for AFP-V0-2406, count `graph.sites` for every
plugin again and regenerate the two goldens.
For AFP-V0-2540
and AFP-V0-2541, delete `droppedDeclarations` from both tools and `droppedUnrecorded`; the refusals
then name only the unbounded-readers record again. For AFP-V0-2542, delete both `leafPattern`
and `directoryMove` functions, `packageDirectories`, `goDirectories`, the `leafReads` and
`movesDirectory` fields and the memo's `movesDirectory` entry; such patterns are unbounded reads
again.
For AFP-V0-2740 and AFP-V0-2741, drop `deletedFixture` from `admittedDeletion`; such paths are
`UNINDEXED_SOURCE_PATH` again and both selectors fall back to FULL. For AFP-V0-2742, restore the
profile condition on the module branch of `Build`. For AFP-V0-2743 and AFP-V0-2744, drop the record
branch of `resolves` and make `resolvedReaders` return `resolvedEvidence`. For AFP-V0-2745, delete
`tools/unbounded-readers/records.go`, its call and `.corvint/record-readers.json`. For AFP-V0-2746,
make `reachesRecord` return `resolvesRecord` and drop the `joins` pass of `resolvingReaders`; joined
readers of a record entry are dropped again. For AFP-V0-2747, return only the fixed probe from
`recordProbes` and return walkers among the readers. For AFP-V0-2748, classify a walker with any
reader match as a reader and drop `coversRecord`, `globCoversRecord` and `globCovers`. For
AFP-V0-2749, call `deletedFixture` in place of `admittedDeletion` in `participants`. For
AFP-V0-2750, restore the non-Go clause of `AllowedUnknown`, delete `FrontierSuite` and its calls,
and drop the `cfamily` declaration from `.github/cishards/test-commands.json` and its
`languageRunners` entry, and make `dryRunManifest` in `tools/selection-baseline` refuse exit 3.
For AFP-V0-2990 to AFP-V0-2995, drop the `codeRunners` class of `traverseWhere`, `testRunnersOf`,
`runnerOnly` and the scope bound of `testlessReaderUsers`; drop `TestOnlyExecs` and the
source-first split of `literalExecs`; delete `internal/gateselect/runners.go`, its two calls, the
`command` flag and the memo's `command` entry, and restore `reach(false, …)` in `unresolved`; then
remove the 24 record entries. Rule (d) then stops at a command again and misses its runners.
For AFP-V0-3140 to AFP-V0-3143, inline the class list back into `traverseWhere`, drop the code-run
class from `reachedThrough`, and drop `codeRunners` from the CI selector's `reach` and from
`attributeNarrowed`'s start. Additive narrowing then misses a changed command that a narrowed
package's code runs, and `corvint test` misses that package's importers.
For AFP-V0-3144 and AFP-V0-3145, revert the review-fix-6 commit: the shortcut again skips every
literal non-shell file name, and any `[[` in a command again opens a test; delete the two tests.
For AFP-V0-3149, make `named` expand each value with `values` instead of `unrolled`; the four
dogfood wrappers' `corvint_bin` then becomes a cut, they become dynamic sources, and
`script/dogfood-change_test.sh`'s read scope must move to `.corvint/unbounded-readers.json`.
For AFP-V0-3148, drop the `slices.Contains` test from `variableValues`' `add`; a repeated word list
then counts toward the bound again, which is sound but makes such a file a dynamic source.
For AFP-V0-3146 and AFP-V0-3147, revert the review-fix-7 commit: `named`, `braced` and
`commandStrings` again return the unknown part at their per-name and depth bounds without the cut,
so a value they drop is read as an environment input; delete `computed_bounds_test.go`.
For AFP-V0-2700 to AFP-V0-2702, delete the `RequireLiveTier` calls and imports, the three
`internal/testsupport/livetier*.go` files, `.github/cishards/live.go`, `tools/live-test-run`, the
`live-test` target and its batch-precheck `EXTRA_TARGETS` entry, the `CORVINT_SKIP_LIVE_TIER`
line of the shard step and the `go-live` job; every test then runs in PR shards again. Deleting only
the shard step's `CORVINT_SKIP_LIVE_TIER` line is the one-line rollback of the PR-shard exclusion.
