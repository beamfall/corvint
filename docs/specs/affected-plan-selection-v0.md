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
`2026-11-03-selprec-selection-precision`. Lane floor (2026-11-03; V1-0643, V1-2744) adds
AFP-V0-2360 to AFP-V0-2363 and AFP-V0-2370 to AFP-V0-2378 here for the same reason; see build-log
entry `2026-11-03-floor-typed-reach`.

## Agent digest
- Claim: `corvint affected` narrows more reads and marks deletions; `corvint test` narrows allow-listed dependency selections by a typed call graph.
- Status: per requirement, each experimental under the owner's standing lane auto-accept (2026-10-11); delivery experimental.
- Exists: `deadlineCode` in `internal/liveverify/affected/dirty.go`, `withWholePackages` in `cmd/corvint/affected_whole.go`, `fixtureTokens` in `internal/liveverify/affected/golang/golang.go`, the measured rounds of `TestIncrementalSelectionMeetsTheLiveBudget`, `TestScanImportsAgreesWithAst_V13322`; `goListScanFile` and `ImportSetRecords` in `internal/liveverify/affected/golang`, `importGraphReadersOf` in `internal/liveverify/affected/importgraph.go`, `anchoredWithin` in `readers.go`, `tempRoots` and `ExtensionlessScript` in `shell/shell.go`, `affectedDeletedPaths` in `cmd/corvint/affected.go`; `internal/liveverify/affected/golang/typedreach`, `testTypedSlices` in `cmd/corvint/test_typed_slices.go`, `tools/typed-reach-measure`.
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
  `TestIncrementalSelectionMeetsTheLiveBudget` MUST measure up to three rounds of 32 interleaved
  selection and probe samples, stop at the first round within the budget, and enforce the budget
  on the round with the lowest selection-to-probe p95 ratio. A slower Select is over the budget in
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
  V1-0643) `conformance/release-artifact-v0` SHALL be declared the same way. Its tests that ran
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
  V1-0643) With the declarations of AFP-V0-2360 and AFP-V0-2361, `corvint affected` SHALL NOT
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
  `-mod=vendor`, a valued switch such as `-race=1`, or a positional argument), a configuration
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
  fixpoint. A write is an assignment, increment, address-taking, or `append`/`copy`/`clear`/`delete`
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
  variable is never private), `filepath.Join` or `path.Join` of a private first element and constant
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
  absent counting as changed (`typedreach.DiffFile`). The declarations' package SHALL be the
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
  `BASE_SOURCE_UNPROVED`; the base closure is selected under the same configuration as head. If any changed function or method present at base, or a method of a
  changed type, can write any shared state, start a process or call a process-wide setter, the
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
sniff extensionless files elsewhere in the tree. AFP-V0-2370 does not trace process argument
values through data structures (a gate or pool command a test defines keeps its test by
`STARTS_PROCESS`), does not model files shared through paths computed at run time (other than from a package
variable), child processes left running, or function and interface values one test stores in
shared state for another to call outside a package variable the second reads; the
`companionrelease` fixture applies the repository's ignore rules, which equals `HEAD`'s tracked set
only while no tracked file matches an ignore rule (none does on 2026-11-03), and does not narrow `cmd/corvint` or `internal/tasks/cli`, which stay unbounded
readers (AFP-V0-025) and keep their whole selection with that refusal named.

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
error the confined CI shard surfaces.

## Acceptance evidence and traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| AFP-V0-548 | `deadlineCode` and its three calls in `internal/liveverify/affected/dirty.go` | `TestAFPV0548DeadlineExpiryIsGitTimeout` in `internal/liveverify/affected/dirty_deadline_test.go` |
| AFP-V0-549 | `selectionBudgetRounds`, `selectionRound`, `measureIncrementalSelection` in `internal/liveverify/affected/measure_test.go` | `TestIncrementalSelectionMeetsTheLiveBudget` (three consecutive runs, build log `2026-11-01-aff-w26`) |
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
| AFP-V0-832 | `affectedDocumentationOnly` and its two checks in `compileAffectedGraph` (`cmd/corvint/affected.go`, `cmd/corvint/affected_freshness.go`); the `--path` paragraph of `corvint help affected` | `TestPathListToleratesDirtyDocumentation_AFP_V0_832` in `cmd/corvint/affected_path_test.go` |
| AFP-V0-2360 | `currentSourceFixture` in `internal/companionrelease/source_fixture_test.go`; its use in `exported_current_test.go`; `-buildvcs=false` in `companion_cli_contract_test.go`; `.corvint/test-read-scopes.json`; `.corvint/unbounded-readers.json` | `go test -exec test-confine ./internal/companionrelease/` (build log); `TestCurrentTasksSourceArchiveBuildsOffline`, `TestCurrentExportedTasksSmokeGenerator` |
| AFP-V0-2361 | `manifestArchiveTestRepository`, `sharedArchiveTestRepository`, `initArchiveTestRepository` in `conformance/release-artifact-v0/archive_gate_test.go`; `newPublicationFixture` in `publication_status_test.go`; `.corvint/test-read-scopes.json` | `go test -exec test-confine ./conformance/release-artifact-v0/` (build log); `TestArchiveDestinationRequiresTrustedAbsentExternalChild`, `TestBuildArchivesCancellationRemovesScratchStageAndOutput` |
| AFP-V0-2362 | the `CORVINT_HOST_ARCHIVE_PROOF` check in `conformance/release-artifact-v0/corvint_archive_test.go`; the job env of `.github/workflows/release-gates.yml` | `TestCorvintHostArchivePartialProof` (skips naming the variable) |
| AFP-V0-2363 | the two declarations and `.corvint/unbounded-readers.json` | `make unbounded-readers-check`; planner runs and per-merge saving in build-log entry `2026-11-03-floor-typed-reach` |
| AFP-V0-2370 | `typedReachPackages`, `testTypedSlices` in `cmd/corvint/test_typed_slices.go`; its call in `testGoSlices` | `corvint test --dry-run` replays in build-log entry `2026-11-03-floor-typed-reach` |
| AFP-V0-2371 | `buildGraph`, `funcDecl`, `packageVar`, `walkInitializer`, `heldAssignment`, `visitExpr`, `call`, `composite`, `convertTo` in `internal/liveverify/affected/golang/typedreach/graph.go`; `Load`, `runnerSourceParity` in `load.go`; `namedIn` in `analyze.go` | `TestAFPV02371ConversionsAndHeldValuesCarryReach`, `TestAFPV02371RunnerConfigurationRefusesHiddenSource` |
| AFP-V0-2372 | `childOnly`, `moduleHazard`, `hasLinkname`, `Analyze`; `Config`, `BuildFlags`, `ProbeConfig`, `LoadFor`, `LoadBaseFor` in `typedreach`; `sessiontest.GoExecutor`; `typedReachExecutor` in `cmd/corvint/test_typed_slices.go` | `TestAFPV02372AlwaysRunReachKeepsThePackageWhole`, `TestAFPV02372LoadRefusesUnmodelledSource`, `TestAFPV02372ExecutorConfigSelectsConstrainedSource`, `TestAFPV02372BuildFlagsClassifiesExecutorFlags`, `TestAFPV02372ProbeConfigReadsTheExecutorEnvironment`, `TestAFPV02372DryRunBindsExecutorBuildConfiguration`, `TestAFPV02372RunRefusesAnotherConfiguration`, `TestAFPV02373KeepsOnlyTheTestsReachingAChangedFunction` (negative control) |
| AFP-V0-2373 | `Analyze`, `reach`, `provenTool`, `externalTools`, `processStarters` | `TestAFPV02373KeepsOnlyTheTestsReachingAChangedFunction`, `TestAFPV02373RoundThreeRegressions` (constants, `os/exec` uses, ioutil and `os.Root`) |
| AFP-V0-2374 | `sharedVar`, `streamOutput`, `streamInput`, `assignedElsewhere`, `opaqueArguments`, `opaqueParameter`, `stateWriters`, `stateReaders`, `processState`, `exemptStateMethods`, `readOnlyReceivers`, `queryOnly`, `rootVar` | `TestAFPV02373KeepsOnlyTheTestsReachingAChangedFunction` (`TestRead`); `TestAFPV02374ReviewRegressions`, `TestAFPV02374ReReviewRegressions`, `TestAFPV02373RoundThreeRegressions` (process-start writes, `os.Stdout` and `fmt`), `TestAFPV02374RoundFourRegressions` (interface-held references, discarded-temp privacy, `os.File` handles, `net/http` defaults, fixed-port listeners), `TestAFPV02374AllowListRegressions`, `TestAFPV02374LoggerReceiverSettersKeepOrderedReaders`, `TestAFPV02374SignalCleanupCannotEraseWrites`, `TestAFPV02374StoredMethodValuesCarryImplicitReads`, `TestAFPV02374SignalStopWritesSignalState`, `TestAFPV02374StreamOutputWritesFiles`, `TestAFPV02374IndirectStdCallsKeepOpaqueArguments`, `TestAFPV02374RangeAssignmentEndsPrivacy`, `TestAFPV02374ScanReadsFiles` |
| AFP-V0-2375 | the slice and whole forms in `testTypedSlices` | `corvint test --dry-run` replays (build log) |
| AFP-V0-2376 | `DiffFile`, `FileChange.Declarations`, `ImportPath`, `fileDecls` in `typedreach/diff.go`; `resolve` in `analyze.go`; its use in `testTypedSlices` and `tools/typed-reach-measure` | `TestAFPV02376DiffFileNamesChangedDeclarations`, `TestAFPV02376MovedPositionsKeepDependentReaders`, `TestAFPV02376ImportPathOfRootPackageFiles`, `TestAFPV02376DryRunReadsRootPackageChanges`, `TestAFPV02376BaseConstraintChangeRefuses` |
| AFP-V0-2377 | `typedReachChangedPaths`, `typedReachUnitRefusal`; `Graph.NamesPath`, `Graph.GlobRead` in `internal/liveverify/affected/readers.go` | `corvint test --dry-run` replays (build log); `TestAFPV02370DryRunNarrowsAnAllowListedDependent` (embedded and run-time data files) |
| AFP-V0-2378 | `tools/typed-reach-measure/main.go` | fault-injection oracle runs in build-log entry `2026-11-03-floor-typed-reach` |

## Rollback

Revert the lane's commits for the requirement and delete its row here. For AFP-V0-2370 to
AFP-V0-2378, emptying `typedReachPackages` restores whole-package selection; the package and the
tool can then be deleted. For AFP-V0-2360 to AFP-V0-2363, deleting a declaration and restoring its
`.corvint/unbounded-readers.json` row returns the package to the floor; the fixtures stay correct. For AFP-V0-551, removing
`fixtureTokens` makes marked literals read paths again; the markers are inert comments. For
AFP-V0-830, drop the integrations listing from `script/gen-documentation-paths.sh` and run
`make documentation-paths`. For AFP-V0-2200 to AFP-V0-2202, delete
`internal/liveverify/affected/importgraph.go`, `golang/importgraph.go` and `golang/importsets.go`,
the two `Unit` fields and the `WithImportSetChanges` call: every `go list` literal is a command
edge again. For AFP-V0-2203, restore the `namesPath` branch of `namesFileOnlyWithin`; for
AFP-V0-2204, call `pathTokens` without temp roots; for AFP-V0-2205, drop `ExtensionlessScript`
from `Owns` and the file walk; for AFP-V0-2206, drop `Plan.Deleted`. Run
`make spec-requirements` after deleting this file.
