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

## Agent digest
- Claim: `corvint affected` narrows fixture, temp-root and `go list` reads, keeps Go path-literal readers under any root, and the CI selector follows its relation.
- Status: per requirement, each experimental under the owner's standing lane auto-accept (2026-10-11); delivery experimental.
- Exists: `deadlineCode` in `internal/liveverify/affected/dirty.go`, `withWholePackages` in `cmd/corvint/affected_whole.go`, `fixtureTokens` in `internal/liveverify/affected/golang/golang.go`, the measured rounds of `TestIncrementalSelectionMeetsTheLiveBudget`, `TestScanImportsAgreesWithAst_V13322`; `goListScanFile` and `ImportSetRecords` in `internal/liveverify/affected/golang`, `importGraphReadersOf` in `internal/liveverify/affected/importgraph.go`, `resolves` and `namesAny` in `readers.go`, `tempRoots` and `ExtensionlessScript` in `shell/shell.go`, `affectedDeletedPaths` in `cmd/corvint/affected.go`; `parentBackslash` in `internal/gateselect/readers.go`.
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
| AFP-V0-832 | `affectedDocumentationOnly` and its two checks in `compileAffectedGraph` (`cmd/corvint/affected.go`, `cmd/corvint/affected_freshness.go`); the `--path` paragraph of `corvint help affected` | `TestPathListToleratesDirtyDocumentation_AFP_V0_832` in `cmd/corvint/affected_path_test.go` |

## Rollback

Revert the lane's commits for the requirement and delete its row here. For AFP-V0-551, removing
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
