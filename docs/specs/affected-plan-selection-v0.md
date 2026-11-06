# Affected Plan Selection Precision V0

Owner: Russell Lewis
Date: 2026-11-01
Requirement prefix: `AFP-V0`
Intent status: per requirement, each experimental under the owner's standing lane auto-accept (2026-10-11)
Delivery status: experimental
Authoritative inputs: `docs/specs/affected-plan-v0.md` (parent contract: wire, authority,
failure codes and acceptance gates), `docs/specs/affected-plan-python-playwright-v0.md` (the
Python adapter), `docs/specs/corvint-self-development-v0.md` (ASD-V0-895, the
`//corvint:fixture-path` marker), `internal/liveverify/affected`, `AGENTS.md` invariants 2, 4
and 8. The parent's Requirements section is at the 256-obligation OCM limit, so lane aff-w26's
requirements live here under the parent's `AFP-V0` prefix; see build-log entry
`2026-11-01-aff-w26`. Lane selprec (2026-11-03; native tickets V1-4409, V1-4362, V1-4173,
V1-4278, V1-3864) adds AFP-V0-2200 to AFP-V0-2206 here for the same reason; see build-log entry
`2026-11-03-selprec-selection-precision`. Lane selmark (2026-11-03; native tickets V1-2662,
V1-1184, V1-1189, V1-2277) adds AFP-V0-2480 to AFP-V0-2489 here for the same reason; see
build-log entry `2026-11-03-selmark-owned-selection`. V1-1184 and V1-1189 name the e2e-safe
profile's AFU-V1-019 to AFU-V1-023; their must-run and order rules apply to the unit plan every
consumer reads, so they live here, and AFP-V0-2486 carries must-run into that profile.
Lane selbugs (2026-11-03; V1-3489, V1-4363, V1-3480,
V1-4281) adds AFP-V0-2400 to AFP-V0-2406 here for the same reason; see build-log entry
`2026-11-03-selbugs-selection-correctness`.
Lane
fr-scripts (2026-11-04; V1-4532, V1-4512) adds AFP-V0-2540 to AFP-V0-2542 here for the same reason.

## Agent digest
- Claim: `corvint affected` names Git timeouts, narrows fixture, temp-root and `go list` reads, marks deletions, and honours declared test owners and must-run tests.
- Status: per requirement, each experimental under the owner's standing lane auto-accept (2026-10-11); delivery experimental.
- Exists: `fixtureOnlyClaim` in `internal/liveverify/affected/select.go`, `deadlineCode` in `internal/liveverify/affected/dirty.go`, `withWholePackages` in `cmd/corvint/affected_whole.go`, `fixtureTokens` in `internal/liveverify/affected/golang/golang.go`, the measured rounds of `TestIncrementalSelectionMeetsTheLiveBudget`, `TestScanImportsAgreesWithAst_V13322`; `goListScanFile` and `ImportSetRecords` in `internal/liveverify/affected/golang`, `importGraphReadersOf` in `internal/liveverify/affected/importgraph.go`, `anchoredWithin` in `readers.go`, `tempRoots` and `ExtensionlessScript` in `shell/shell.go`, `affectedDeletedPaths` in `cmd/corvint/affected.go`; `readTestSelection`, `ownersOf` and `mustRunOf` in `internal/liveverify/affected/testselection.go`, `affectedFailFastOrder` in `cmd/corvint/affected_order.go`; `droppedDeclarations` in `tools/unbounded-readers` and `droppedUnrecorded` in `tools/test-read-scopes-check`; `leafPattern` in `internal/liveverify/affected/golang/unbounded.go` and `internal/gateselect/readers.go`.
- Blocked on: the parent spec's gates.
- Read next: Requirements; the parent spec's Non-goals and authority and Failure modes.

## User and current state

The affected user and the plan wire are the parent's: an engineer or agent that wants the tests a
change plausibly needs before the mandatory gate. These requirements tighten how a plan reports a
Git deadline, a TestMain change and a fixture-only path literal, and how two existing checks
measure. None of them drops a unit the change can reach.

## Requirements

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
  `GOFLAGS` or `GOENV`, and make every other `os/exec` call with string-literal arguments only, so
  no listed name feeds another command; the call, its command, its output and every value derived
  from them must stay in that function: none is returned, sent on a channel, stored anywhere but a
  local of the function, or passed to a call other than a builtin or conversion, a function of a
  standard-library import, or a method of a derived value or of a `*testing.T`, `*testing.B`,
  `*testing.F` or `testing.TB` parameter of a function enclosing the call, so no helper or caller
  can build or run what it lists; and no file of the unit may name `GOFLAGS` or `GOENV`. Each literal pattern of a proven call names the directory it
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
  graph. A path under `testdata` or a directory whose name starts with `_` or `.` is no package
  file. Without the comparison (a `--path` plan, an unreadable base, a caller of `affected.Select`
  that supplies none) the selector SHALL assume every dirty `.go` file of a Go unit changes its
  unit's import set and every unowned one, or a module file, changes the module's, so no reader is
  lost. Measured (build-log entry `2026-11-03-selprec-selection-precision`): deleting
  `cmd/corvint-lsp/swift_impact.go`, an import-set change in a binary that
  `internal/swiftimpact`'s `TestNGIV0988TasksClosureExcludesPlannerAndImpactBinariesLinkReader`
  names only by a computed string, selected 57 units without that package before (its test fails
  on the change) and selects it now, 58 units.

- **AFP-V0-2203:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-4173; amending `AFP-V0-072` (2)) For a Go unit, an anchored path token (one that starts with
  `/`: a module-path literal anchored at its module's directory, a fake absolute root such as
  `"/repo/.git"`, or a fragment joined to a root variable such as `root + "/testdata/"`) SHALL
  name a dirty path only when, joined to the repository root, the unit's own directory or one of
  its ancestors (where the unit's module directory lies), it is the path or an ancestor of it, the
  last of two or more components being allowed to be a fragment; or when a run of it matches the
  path after leading components that, below one of those bases, are each a whole component of one
  of the unit's tokens or one the Go lexicon cannot keep whole, the AFP-V0-072 placement test. A
  token that climbs out of the repository from every base, and a token with no component run
  (`"/"`, `"/."`), names nothing. The unit's own subtree, climbing unanchored tokens, runs of two
  or more components, non-Go units, the CEM sidecar rule and declared read scopes are unchanged.
  Measured on the PR 86 tree with `affected --by-path --path
  internal/jslinecov/testdata/repo/lib.js`: the Go path-literal readers named through `"/repo"`,
  `"///repo"`, `"/testdata/"` and a bare `"/"` (`internal/capcatalog`, `internal/gitstatus`,
  `cmd/corvint` and 15 others: 24 Go readers become 6) are no longer readers; the six that name
  `internal/` by `"../../internal"` or `internal/jslinecov` stay.

- **AFP-V0-2204:** (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-4173) The shell plugin SHALL add no path token for a literal path that follows the
  expansion of a temp root: a variable assigned exactly once in the file, whose every other
  occurrence is an expansion that cannot assign or text joined to a `/`, `.` or `-`, and whose
  value is one `$(mktemp -d ...)` substitution or another temp root followed only by a literal
  path without `..` (`repo="$tmp/repo"`). The assignment is a command of its own at the file's
  top level, not after `&&` or `||` and not in a pipeline or the background, and no expansion of
  the name precedes it, so no value from the environment stands in. A file that defines or aliases
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

## Non-goals

The parent contract's non-goals and authority rules apply. AFP-V0-551 does not narrow by suffix
literals, a runtime read trace, or symbol-level reachability inside a hub package (V1-3282,
V1-2744). It does not narrow a test-only edit's unbounded readers (V1-4143). The nested-module
reader path (`nested.go`) honours the marker only since AFP-V0-831. AFP-V0-552 does not add a Python tokenizer to the
adapter. AFP-V0-2200 does not narrow a declared read scope or an unbounded read: the repository's
`go list` guards whose scopes declare `internal/` (`internal/liveverify/provider/interlock`,
`internal/analyzerstructured`, `cmd/corvint-analyzer-rust`, `cmd/corvint-analyzer-shader`) stay
selected on any `internal/` edit by AFP-V0-023, and a header-only scope would be a new owner
declaration. AFP-V0-2203 and AFP-V0-2204 are not ported to the CI selector's rule (c)
(`internal/gateselect`), which stays broader than the planner as before. AFP-V0-2205 does not
sniff extensionless files elsewhere in the tree. AFP-V0-2480 to AFP-V0-2484 add no annotation or
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
error the confined CI shard surfaces. An owner mapping that names the wrong tests for a path it
owns is an author error: the path's real readers are still selected by the reader rules, and only
the unowned-path unknown is lifted, as for a documentation declaration (AFP-V0-160). A mapping
whose tests no longer exist refuses the build (AFP-V0-2483). A must-run entry makes every plan
larger; it never makes one smaller. An unreadable `corvint test` receipt leaves the order with
that receipt `UNREADABLE` and contributes no failure; it never fails the plan.
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

## Acceptance evidence and traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| AFP-V0-548 | `deadlineCode` and its three calls in `internal/liveverify/affected/dirty.go` | `TestAFPV0548DeadlineExpiryIsGitTimeout` in `internal/liveverify/affected/dirty_deadline_test.go` |
| AFP-V0-549 | `selectionBudgetRounds`, `selectionRound`, `measureIncrementalSelection` in `internal/liveverify/affected/measure_test.go`; immutable `Graph.pathTokenCache` prepared by `finish` | `TestIncrementalSelectionMeetsTheLiveBudget` (three consecutive runs, build log `2026-11-01-aff-w26`); `TestPreparedPathTokensPreserveSelection` (2026-11-05 repair, exact cached/uncached plan parity) |
| AFP-V0-550 | `affectedWhole`, `withWholePackages`, `testMainSource`, `testMainOf` in `cmd/corvint/affected_whole.go`; `WholePackages` in `cmd/corvint/affected.go`; `cmd/corvint/affected_compact.go` | `TestAFPV0550TestMainChangeMakesThePackageWhole` in `cmd/corvint/affected_whole_test.go` |
| AFP-V0-551 | `FixturePathMarker`, `fileScan.fixture`, `fixtureOnly`, `scannedLiteral`, `fixtureTokens`, `pathTokens` and `observeDirectory` in `internal/liveverify/affected/golang/golang.go` | `TestAFPV0551FixtureMarkedLiteralsNameNoRead` in `internal/liveverify/affected/golang/fixture_marker_test.go`; before/after counts in build log `2026-11-01-aff-w26` |
| AFP-V0-552 | `astImports`, `differentialCorpus` in `internal/liveverify/affected/python/differential_test.go` | `TestScanImportsAgreesWithAst_V13322` |
| AFP-V0-553 | `ownDirectory` and the `+` operators of `braced` in `internal/liveverify/affected/shell/computed.go`; `commandStrings` in `internal/liveverify/affected/shell/shell.go` | `TestInvocationThroughVariables_AFP_V0_553` in `internal/liveverify/affected/shell/invocation_test.go`; `TestHeavyShEditKeepsShellReaders_V14170` in `internal/liveverify/affected/replay_v14170_test.go` |
| AFP-V0-830 | the integrations README listing in `script/gen-documentation-paths.sh`; `.corvint/documentation-paths.json` | `TestIntegrationReadmesAreDocumentation_AFP_V0_830` in `internal/liveverify/affected/integration_readme_documentation_repo_test.go`; `script/gen-documentation-paths_test.sh` case 6 |
| AFP-V0-831 | the `fixtureOnly` check on the token loop of `observeDependentModule` in `internal/liveverify/affected/golang/nested.go`; the marker in `tools/local-authority/codec_package_test.go` | `TestNestedModuleHonoursFixtureMarker_AFP_V0_831` in `internal/liveverify/affected/golang/fixture_marker_test.go`; `TestIntegrationReadmesAreDocumentation_AFP_V0_830` (repository plan) |
| AFP-V0-2200 | `goListScanFile`, `goListArguments`, `importGraphTemplate`, `isolatedList`, `literalEnvironment`, `goListUnits` in `internal/liveverify/affected/golang/importgraph.go`; the `listed` skip in `usedCommandLiterals` (`binaryexecs.go`); `Unit.ImportGraphReads` and `ImportGraphReadsAll` in `unit.go`, `graph.go` and `graphcache.go` | `TestGoListReadersAreImportGraphReaders_AFPV02200` in `internal/liveverify/affected/golang/importgraph_test.go`; `TestRepositoryImportGraphReaders_V14409_V14362` in `internal/liveverify/affected/importgraph_repo_test.go` |
| AFP-V0-2201 | `importGraphReadersOf`, `importClosure`, `WitnessImportGraphReader` in `internal/liveverify/affected/importgraph.go`; `terminalWitness` in `select.go` | `TestImportGraphReadersFollowImportSetChanges_AFPV02201` (body-only narrowing; import-added, out-of-closure, `go.mod` and no-comparison controls); `importGraphWitnessResolves` in `measure_test.go`; the fault-injection runs in build-log entry `2026-11-03-selprec-selection-precision` |
| AFP-V0-2202 | `ImportSetChangesAt`, `ImportSetRecords`, `packageHeader` in `internal/liveverify/affected/golang/importsets.go`; `unknownImportSets`, `ModuleFile`, `GoIgnoredPath` in `importgraph.go`; `planAffectedGraph` and `affectedMeasuredCommit` in `cmd/corvint/affected.go` | `TestImportSetRecordsCompareHeaders_AFPV02202` in `internal/liveverify/affected/golang/importgraph_test.go` |
| AFP-V0-2203 | `anchoredWithin`, `runOffsets` and the anchored branch of `namesFileOnlyWithin` in `internal/liveverify/affected/readers.go` | `TestAnchoredTokensResolveFromModuleBases_AFPV02203` in `internal/liveverify/affected/golang/anchored_test.go`; the amended cases of `TestFileNameOnlyReadersMustPlaceThePath_AFPV0072` and `TestChangeEvidenceReadersAreNarrowed_V1_0230` |
| AFP-V0-2204 | `tempRoots`, `unconditionalAssignment`, `populates`, `assignedOnce`, `firstAssignment`, `underTempRoot`, `tempExpansion` and the temp step of `pathTokens` in `internal/liveverify/affected/shell/shell.go` | `TestTempRootPathsAreNoTokens_AFPV02204` (plain root dropped; recursive copy, tree copy, archive, worktree, link, conditional and used-first controls keep the token); `TestWriteOnlyWordsAreNoPathTokens_NSAV0016` |
| AFP-V0-2205 | `ExtensionlessRoots`, `ExtensionlessScript`, `shellShebang`, `shellExecutables` and `Owns` in `internal/liveverify/affected/shell/shell.go` | `TestOwnsOnlyShellSuffixes_NSAV0006`; `TestExtensionlessShellExecutablesAreUnits_AFPV02205` in `internal/liveverify/affected/shell/shell_test.go` |
| AFP-V0-2206 | `Plan.Deleted` in `internal/liveverify/affected/select.go`; `affectedDeletedPaths` and its call in `planAffectedGraph` (`cmd/corvint/affected.go`); `cmd/corvint/affected_compact.go` | `TestAffectedPlanMarksDeletedPaths_AFPV02206` in `cmd/corvint/affected_deleted_test.go` |
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
| AFP-V0-832 | `affectedDocumentationOnly` and its two checks in `compileAffectedGraph` (`cmd/corvint/affected.go`, `cmd/corvint/affected_freshness.go`); the `--path` paragraph of `corvint help affected` | `TestPathListToleratesDirtyDocumentation_AFP_V0_832` in `cmd/corvint/affected_path_test.go` |
| AFP-V0-2440 | `unownedUnknown` and `fixtureOnlyClaim` in `internal/liveverify/affected/select.go` | `TestFixtureOnlyClaimIsNoUnindexedSource_AFPV2440` in `internal/liveverify/affected/fixtures_test.go` (two reclassified shapes; five kept: unhosted, indexed below, outside testdata, a Go claim, a second indexing claimant); `TestRubyFixturesOfANewGoPackageAreNoUnindexedSource_V14446` in `internal/liveverify/affected/replay_v14446_test.go` (real plugins; a no-`go.mod` negative control); both fail with the rule disabled; live replay of PR 85 in build log `2026-11-03-mqfix` |
| AFP-V0-2400 | `internal/liveverify/affected/shell/testinputs.go` (`readShellTestInputs`, `applyShellTestInputs`) and its call in `shell.go`; `shellTestInputsChanged` in `internal/liveverify/affected/readers.go` and its call in `select.go`; `.corvint/shell-test-inputs.json`; the `tools/declaration-compat-check` entry | `TestShellTestInputsDeclaration_AFPV02400` and `TestShellTestInputsDeclarationChanged_AFPV02400` in `internal/liveverify/affected/shell/testinputs_test.go`; the `inputs changed` case of the `tools/shell-test-select` step test; the before/after and fault-injection runs in build-log entries `2026-11-03-selbugs-selection-correctness` and `2026-11-04-selbugs-review-round-d` |
| AFP-V0-2401 | `tools/shell-test-select/main.go`; `make shell-test-affected` | `TestAFPV02401SelectsTheShellTestStepsThePlanSelects`, `TestAFPV02401RepositoryStepsMapToTheirScripts`, `TestAFPV02401IncompleteReceiptsRunEveryStep` in `tools/shell-test-select/main_test.go`; the fault-injected `script/batch-regen.sh` run |
| AFP-V0-2402 | `testPlanRunSet` in `cmd/corvint/test.go` | `TestAFPV02402RunSetIsThePlanGoSelection` in `cmd/corvint/test_whole_reasons_test.go`; ten merged ranges in the build-log entry |
| AFP-V0-2403 | `checkAgreement`, `nestedModuleUnits`, `Summary.TestExtra`, `Summary.TestOmitted`, `Summary.TestMisses` and `Failures` in `tools/selection-baseline/main.go` | `TestSelectionBaselinePinsTestAffectedAgreement_AFPV02403`; `TestAFPV02403NothingManifestMissesFailingPackages` (a NOTHING manifest's failing package is a test miss while the CI selector selects it; a NOTHING manifest without failures whose plan selects a package fails the gate; an unrecognized scope pinned); `TestAFPV02403ManifestOmitsPlannedPackage` (a NARROWED manifest that omits a planned package on a change with no failure fails the gate, a nested-module unit and a non-Go plan unit excepted); `TestAFPV02403NestedModuleUnitIsNotOmitted` (an excluded dependent nested module unit with a go.mod is not omitted; an excluded root package, a path outside the root module and one escaping the clone stay compared); `make selection-baseline-check` |
| AFP-V0-2404 | `.corvint/runtime-readers.json` | the per-path plan summaries in the build-log entry |
| AFP-V0-2405 | `driverInputUnownedHint`, `driverInputHints` in `internal/liveverify/affected/documentation.go` and its call in `select.go` | the `Makefile` plan in the build-log entry |
| AFP-V0-2406 | `frontierSites`, `planFrontierSites` in `internal/liveverify/affected/graph.go`; `frontierUnknowns` returning the participants in `select.go` | `TestAFPV0381PlanCountsRepositoryFrontierSites` (amended), `TestFixtureFrontierBearsOnlyWhereAGoUnitHostsIt_AFPV0446`; regenerated `cmd/corvint/testdata/affected-selection-*.golden` |

## Rollback

Revert the lane's commits for the requirement and delete its row here. For AFP-V0-551, removing
`fixtureTokens` makes marked literals read paths again; the markers are inert comments. For
AFP-V0-830, drop the integrations listing from `script/gen-documentation-paths.sh` and run
`make documentation-paths`. For AFP-V0-2200 to AFP-V0-2202, delete
`internal/liveverify/affected/importgraph.go`, `golang/importgraph.go` and `golang/importsets.go`,
the two `Unit` fields and the `WithImportSetChanges` call: every `go list` literal is a command
edge again. For AFP-V0-2203, restore the `namesPath` branch of `namesFileOnlyWithin`; for
AFP-V0-2204, call `pathTokens` without temp roots; for AFP-V0-2205, drop `ExtensionlessScript`
from `Owns` and the file walk; for AFP-V0-2206, drop `Plan.Deleted`. For AFP-V0-2440, make `fixtureOnlyClaim` return false; such paths are
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
