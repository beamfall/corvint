# Test Selection Precision V0

Owner: Russell Lewis
Date: 2026-10-30
Requirement prefix: `TSP-V0`
Intent status: experimental under the owner's standing lane auto-accept (2026-10-11; V1-3160); TSP-V0-001 amends AFP-V0-580
Delivery status: experimental
Authoritative inputs: native ticket V1-3160 (related V1-2763, V1-2896); TSP-V0-006: V1-2806; TSP-V0-007: V1-3050; TSP-V0-860: V1-2892; TSP-V0-861: V1-3348; TSP-V0-940: V1-3346; TSP-V0-960: V1-3347; TSP-V0-864: V1-3348; TSP-V0-950 and TSP-V0-951: V1-2167. These requirements narrow
when the [affected plan](affected-plan-v0.md) names a unit frontier; that spec's Requirements
section is at the 256-obligation OCM limit (`maxObligations`), so they live here.

## Agent digest
- Claim: frontiers bear only where they can; declared locators and shell callers stop widening plans; Go edits run reaching tests; PR reruns reuse passes.
- Status: experimental under the owner's standing lane auto-accept (2026-10-11; V1-3160); TSP-V0-001 amends AFP-V0-580.
- Exists: `FrontierBearsOnOwnFiles` in `internal/liveverify/affected/unit.go`; `ownFilesBear` in `internal/liveverify/affected/select.go`; `observeDirectory` in `internal/liveverify/affected/golang/golang.go`; `applyGatedLocators` in `internal/liveverify/affected/golang/gatedlocators.go`; `applyForeignCallers` in `internal/liveverify/affected/shell/callers.go`; `testSourceSlice` in `cmd/corvint/test_source_slices.go`; `rerun` and `buildPrior` in `tools/corvint-pr-tests/rerun.go`; tests named in the traceability table.
- Blocked on: TSP-V0-007's CI wiring (a `.github` change, Open question 4); Open questions names the residual shell frontier and the source-and-test slice.
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
  `&&`, `||`, `|` or `&` and does not precede `|` or `&`. Every command between them (in that
  block, or a substitution within it; a command of another block fails, and a function defined
  there is skipped) SHALL be on this allow-list, since a link, copy, move, mount, `cd` or write
  there could make the read a repository file: no backquote or process substitution, a redirection
  only to `/dev/null` or a file descriptor, no assignment or prefix other than `command -p`, and the
  command one of `:`, `true`, `false`, `echo`, `printf` without `-v`, `exit`, `return`, `hostname`
  without operands, `base64` with only decode options, a `tar` extraction of standard input
  (`-x`/`--extract`, `-C`/`--directory DIR`, `-f -`, option groups of `xvzjJpom`, no member), a
  `trap` in that block for EXIT, or for INT, TERM, HUP or QUIT with the handler `exit [N]`, or a
  call to a function defined exactly once, earlier, in that block, with a body of allow-listed
  commands and no call or nested block. The proof SHALL fail when the file defines a function
  named `mktemp`, `command`, `tar`, `base64`, `hostname` or an allow-listed builtin, runs `alias`,
  `hash` or `enable`, names TAPE or TAR_OPTIONS other than as an expansion, or runs a `source`,
  `.`, `load` or `eval` before the fresh source; and when the file names PATH other than as an
  expansion, `mktemp`, `tar`, `base64` and `hostname` SHALL each run as `command -p NAME`. Every
  other non-literal source SHALL stay `shell:dynamic-source`.
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
- **TSP-V0-006:** (experimental, under the owner's standing lane auto-accept, 2026-10-11; V1-2806)
  `corvint test` SHALL slice a Go package the plan selects by `DIRECT_SOURCE_CHANGE` to the test
  functions that can reach what changed, and otherwise SHALL keep it whole with a reason naming the
  missing proof. Slicing SHALL require that the package's own dirty paths are existing non-test `.go`
  files present at base, that none of its test files is dirty, that the plan of the other dirty
  paths does not select it (at most 16 such plans), that its directory holds no assembly, C or other
  non-Go source, that no file names `go:linkname` or imports `"C"`, and that every file parses. Each
  changed file with its receiver-less functions other than `init` removed SHALL equal base token for
  token, comments included; the changed names are the functions whose text differs, added or
  removed. No file of the package SHALL write a changed name other than as a call's callee or its
  own declaration; no variable initializer, `init` or `TestMain` SHALL reach a changed name; and no
  changed name SHALL reach a package variable, where a changed function's base body joins the
  closure beside its head body (state the change stopped touching still counts). Each changed
  function SHALL have, in head and base, no type parameters and only parameters and results of a
  predeclared basic value type the package does not redeclare (`bool`, `string`, `byte`, `rune`, the
  sized and unsized integer, float and complex types; not `uintptr`, not variadic), and its file no
  dot import. No declaration the changed functions reach, base bodies included, SHALL write other
  than to `_` or a plain identifier: an assignment, increment or range target through a pointer,
  selector or index, or a call of `copy`, `clear` or `delete`, keeps the package whole. Reach is the closure over top-level declarations by
  identifier, where a method is also reached by its receiver type's name and a selector qualified
  by an unshadowed import name is no use. Every test function of an external (`_test` package) file
  SHALL be kept, and each internal one whose closure holds a changed name. A package with every
  test kept SHALL stay whole; one with none SHALL run `-run ^$` (build and vet only). Dependents of
  the package SHALL stay whole. The base SHALL be read in the same two Git processes as STR-V0-072.
- **TSP-V0-007:** (experimental, under the owner's standing lane auto-accept, 2026-10-11; V1-3050)
  `corvint-pr-tests --mode prior --evidence DIR` SHALL write `prior.json`
  (`corvint-pr-tests-prior/0`) from one completed run's output directories: the PR head, tested
  merge, tree and identity (shard count cleared) on which every directory agrees and which each
  observed, and the packages with a package-level `pass` event, no `fail` event and no slice in any
  directory. It SHALL refuse evidence with no or more than 64 directories, an execution error, a
  selection that recorded any fallback or drift reason (its tests may have run on a tree other than
  the one it names), an unreadable event or disagreeing directories. `--mode plan --prior FILE` SHALL write `rerun.json`
  (`corvint-pr-tests-rerun/0`) beside an unchanged `selection.json`: the packages of `go list ./...`
  at the tested merge M1 that the record does not list as passed, plus the selector's `run` set for
  the planner's `affected --base M0` at M1, where M0 is the record's tested merge. It SHALL instead
  name `./...` and a reason when the tool profile or the deciding plan's topology failed, the record
  is unreadable, malformed (including any data after its one JSON value, which a second decode must
  report as end of input) or carries an unknown field, its identity differs, M0 does not resolve to
  the recorded tree or its second parent is not the recorded head, that head is not an ancestor of
  the current head, M0..M1 changes a `.github` path, the planner or selector fails or gives no `run`
  verdict, the universe is unavailable, or the source drifts after planning.
- **TSP-V0-940:** (experimental, under the owner's standing lane auto-accept, 2026-10-31; V1-3346;
  refines TSP-V0-003 and STR-V0-061) `internal/gateselect` SHALL read the same
  `.corvint/gated-root-locators.json` from the checkout it indexes, with the same validity rules
  (closed object, profile, at most 64 KiB and 64 packages, each package an indexed package whose
  non-test code locates the root and whose non-test sources carry the gate as a Go string
  literal, the gate a clean absolute path other than `/`, the reason non-empty printable text of
  at most 512 bytes). A valid declaration SHALL stop each declared package's rule (d) reach: the
  package keeps its own `unresolved` reason, and its dependents are no longer `unresolved` for
  depending on it, so the selector and the planner agree on which units every dirty path selects,
  a nested-module change included. An absent or invalid declaration SHALL declare nothing (the
  wider selection). A plan that names `go:gated-root-locators-invalid` or
  `go:gated-root-locators-changed` SHALL make the selection `FALLBACK`, as the module-level
  frontiers do.

- **TSP-V0-960:** (experimental, amending TSP-V0-005, under the owner's standing lane auto-accept,
  2026-10-11; V1-3347) The shell plugin SHALL expand a command word or `bash`/`sh` operand that is
  computed (contains `$` or a backquote) through the file's own assignments, `${NAME-default}`,
  `${NAME:-default}`, `${NAME=default}` and `${NAME:=default}` defaults and `for NAME in` word lists,
  bounded to depth 4, 16 values a name and 64 values a word; any other expansion, a positional or
  special parameter, a command substitution and a name the file never sets is unknown. A word whose
  values the 64-value bound cuts short SHALL NOT drop them silently: its kept values stay edges and
  the unit SHALL set `UnboundedReads` to `shell:dynamic-source`, a non-test unit also `LocatesRoot`
  (NSA-V0-009), so a change to a dropped script still selects the tests that reach it. Each fully
  literal value SHALL be resolved as a literal target, adding an import edge when it names a source.
  A value whose file name is still unknown but whose text after its last unknown part ends in an
  owned shell suffix is a constructed script name: its non-test unit SHALL carry
  `shell:foreign-invocation-unmodelled` bearing on dependents, and the TSP-V0-005 completeness scan
  SHALL count the file as naming every declared script whose file name the pattern matches, so an
  unlisted constructor makes the declaration invalid. A declared script that itself constructs a
  script name SHALL keep the frontier while still carrying its callers. A value with no literal shell
  suffix is not a constructed script name.
- **TSP-V0-861:** (experimental, amending TSP-V0-006, under the owner's standing lane auto-accept,
  2026-10-11; V1-3348) A package TSP-V0-006 would keep whole only because its own test files
  changed with its source SHALL instead be sliced when both proofs hold: TSP-V0-006's over its
  changed non-test files, with the head test files (changed ones included) in its reach graph, and
  STR-V0-063's over its changed test files (each exists and parses, none declares `init` or
  `TestMain`, initializes a variable with a call or has a blank or dot import, now or at base; no
  unchanged test file uses a name a changed one declares; the test files' imports together equal
  base's; the changed files declare at least one test function). The slice SHALL run the union of
  the tests TSP-V0-006 keeps and every test function of the changed test files. Either proof
  failing SHALL keep the package whole with that proof's reason; a union holding every test
  function of the package SHALL keep it whole naming that. The plan of the dirty paths outside the
  package's own source and test files SHALL still not select it, and the base reads SHALL stay in
  the same two Git processes (STR-V0-072). TSP-V0-006's `go:linkname` refusal SHALL be decided on
  the parsed comments: a file names it when one of its comments starts with `//go:linkname`, the
  only form the compiler honours, so a string literal or prose naming the directive (this
  slicer's own source does) no longer keeps its package whole.
- **TSP-V0-864:** (experimental, amending TSP-V0-006, under the owner's standing lane auto-accept,
  2026-10-11; V1-3348) A changed file whose residue (the file less its receiver-less functions
  other than `init`) differs from base SHALL still be sliced when, comments ignored, the head
  residue less each wholly new `const` or `type` declaration (none of whose names base declares)
  equals base's residue, and the files' directive comments are the same in source order, each
  paired with the declaration it binds (the package clause for a directive above it, else the
  innermost declaration or grouped spec that begins next, keyed by its keyword and names), so a
  `//go:embed` moved from one `var` to another keeps the package whole. A directive comment is one
  starting `//go:`, `//line ` or `/*line `, or a build constraint as `go/build/constraint`
  classifies it (`IsGoBuild`, `IsPlusBuild`), so `//+build ignore` without a space counts. The new names SHALL seed reach beside the changed
  functions; they are no function, so the value-use, signature and write proofs apply to the
  changed functions alone. A new name that a method's receiver names, a changed or removed
  declaration, a new import, `var`, method or `init`, and a changed package clause SHALL keep the
  package whole as before. An external test SHALL be kept only when a function changed or a name
  is new: a comment-only change is observed by no test, so it runs `-run ^$` (build and vet).
- **TSP-V0-860:** (experimental, under the owner's standing lane auto-accept, 2026-10-11; V1-2892)
  `tools/selection-baseline` SHALL replay a corvint binary and a gate-affected-select binary over
  the fixed change set `benchmarks/selection-baseline/changes.json`
  (`corvint-selection-baseline-changes/0`): every lane change merged to `origin/main` (base its
  first parent) and every merged-branch commit whose pull-request CI run failed (base the newest
  first-parent ancestor on `main`), with the packages each failed run's shard annotations name. It
  SHALL check each head out detached and clean in a `--shared` clone, so the repository's index,
  worktrees and refs are not written, and record per change the plan scope, the dirty, selected,
  unknown and universe counts, the unknown kinds (a language frontier by its name without the
  site) and witness kinds, the selector verdict, whether `.github/` changed, the CI scope (`FULL`
  for a `.github/` change or a verdict other than `run` or `NOTHING`), the selected share of the
  Go packages and of the milliseconds in `.github/cishards/package-costs.json` (1 for `FULL`),
  and the Go packages `corvint test --dry-run` runs whole, sliced, and whole because their source
  and test files both changed. A failing package a `NARROW` or `NOTHING` change does not select
  SHALL be a miss with its cause; a failure naming no package SHALL be counted as unattributed,
  never as covered. The narrow count SHALL only be reported together with the miss count.
  `make selection-baseline-check` SHALL build both binaries from the tree under test, replay them
  and fail when fewer merged changes narrow than the file's `floor.narrowMerged` or any miss
  appears. It SHALL be one of `DOC_GATE_TARGETS`, so `make doc-gates` (the CI `doc-gates` job, which
  checks out full history) and `make batch-precheck` run it (owner ci-control-plane approval
  delegated to the orchestrator, 2026-10-31).
- **TSP-V0-950:** (experimental, under the owner's standing lane auto-accept, 2026-10-11; V1-2167)
  The project MAY declare, in `.corvint/shell-read-scopes.json` (profile
  `corvint-shell-read-scopes/0`, `{"profile", "tests": {TEST: {"reads", "reason"}}}`), for a shell
  test file, the repository paths it reads outside its own directory: `reads` is 1 to 256 sorted,
  unique AFP-V0-023 read-scope entries (a path, or a subtree with one trailing `/`), and `reason` a
  non-empty single-line statement of at most 512 bytes naming what the toolchain run reads. The
  shell plugin SHALL accept the declaration only when every declared test is an observed shell test
  unit whose only unbounded read is NSA-V0-011's `shell:runs-toolchain` and that reaches, through
  its `Imports`, `TestImports` and `Execs` transitively, no unit with unbounded reads or that locates
  the root (a test that sources a dynamic path, itself or through a literal helper, stays
  unbounded). An accepted test's unit SHALL then carry no `UnboundedReads` and
  SHALL be read-scoped to its entries, as an AFP-V0-023 Go package is: it is selected by a dirty path
  in its own directory or in a declared entry, and a literal it names outside them selects nothing.
  A missing declaration SHALL change nothing; a present one that is unreadable, oversized, invalid in
  any member, or names a test that is not such a unit SHALL declare nothing and name
  `shell:read-scopes-invalid`, so every toolchain test stays an unbounded reader. A plan with the
  declaration dirty SHALL name `shell:read-scopes-changed` and be `UNKNOWN`.
  `tools/test-read-scopes-check` SHALL keep checking Go packages only.
- **TSP-V0-951:** (experimental, under the owner's standing lane auto-accept, 2026-10-11; V1-2167)
  `script/check-sql-native-ratchets_test.sh` SHALL be declared under TSP-V0-950 with `Makefile`,
  `go.mod`, `go.sum`, `go.work` and the non-standard packages `go list -deps` gives for
  `./cmd/corvint-go-toolchain-receipt`, and SHALL leave the units of
  `.corvint/unbounded-readers.json`. A repository test SHALL fail when that package gains a
  dependency outside the declared reads or the plugin does not accept the declaration. For a
  `shell:runs-toolchain` unit that is neither declared nor recorded, `tools/unbounded-readers`
  SHALL still refuse with exit 1 and its line SHALL name `.corvint/shell-read-scopes.json` as the
  bounded route before the `units` entry.
- **TSP-V0-900:** (experimental, under the owner's standing lane auto-accept, 2026-10-11; V1-3763,
  V1-3587, V1-3788, V1-3616) This repository SHALL declare an AFP-V0-023 read scope in
  `.corvint/test-read-scopes.json` for each Go test package whose literals outside its directory
  name only temporary fixtures or a bounded toolchain input, and a TSP-V0-950 read scope in
  `.corvint/shell-read-scopes.json` for each `shell:runs-toolchain` script test whose toolchain reads
  a bounded set, and SHALL remove each declared unit from `.corvint/unbounded-readers.json`. A Go
  declaration SHALL hold under `tools/test-read-scopes-check` and under the `.github/testconfine`
  wrapper with every test of the package passing; a shell declaration SHALL pass in a copy of the
  tree holding only `script/` and its declared reads, or fail there exactly as in the full tree. A
  one-spec edit and a `docs/specs/REQUIREMENTS.tsv` edit SHALL then select none of the declared
  units, while a unit that reads every spec or the index stays selected. Measured from 6d4713fb8b5d
  to e00f422d3af1, `corvint affected` units fall from 71 to 44 (spec), 75 to 44 (index), 62 to 38
  (Makefile recipe; 69 unknowns before and after), 58 to 42 (`script/lane-guard.sh`), 59 to 43
  (`script/check-bash32.sh`), 60 to 36 (`docs/DOGFOOD.md`) and 20 to 18 (a new CEM under
  `.corvint/changes/`); for the spec edit `corvint test --dry-run` falls from 48 to 28 units, its
  stdout from 15,343 to 8,802 bytes and its `--summary` stdout from 3,033 to 1,859 bytes. Non-goals:
  row-level selection within `REQUIREMENTS.tsv` (every remaining reader parses the whole file);
  declaring a unit that opens `/` or `.git`, runs Git over the real checkout, launches a nested
  sandbox or walks the whole tree. Failure mode: a declared unit that gains a read outside its scope
  is missed until the scope is widened; the static checker and TSP-V0-901 catch literal drift, and a
  confined run catches runtime drift only when rerun. Rollback: restore the three declaration files
  from the base; the units are unbounded readers again.
- **TSP-V0-901:** (experimental, under the owner's standing lane auto-accept, 2026-10-11; V1-3763,
  V1-3587, V1-3788) A repository test SHALL select each class of narrowed unit for a dirty path its
  declared scope names (a spec file, another package's fixture, a `.github` input, an integrations
  input, a capability tree, a Go package source, the Makefile, an extension file, a tool source and
  a dogfood document) and SHALL fail when the declaration drops that read. Rollback: delete the test
  with TSP-V0-900's declarations.
- **TSP-V0-902:** (experimental, under the owner's standing lane auto-accept, 2026-10-11; V1-3763,
  V1-3788, V1-3616) The plans TSP-V0-900 leaves wide SHALL stay sound; they are recorded, not
  narrowed, and each still names its unknowns. The spec-edit plan's 19 unbounded readers cannot be
  read-scoped: units that open `/` (the dashboard walks, `internal/console`, `internal/lspstdio`,
  `internal/postmergehost`, `internal/postmergeworkflow`), run Git on the real checkout
  (`internal/analyzerhtmlcss`, `internal/analyzerpython`, `internal/doccorpus/selfcorpus`,
  `internal/companionrelease`, `conformance/release-artifact-v0`), launch a nested sandbox
  (`internal/liveverify/mutate`) or read the whole tree (`internal/tasks`, `internal/tasks/cli`,
  `internal/liveverify/affected/golang`), and five shell tests that source `script/heavy.sh`
  dynamically or run `make` or `git` on the real root; with the true spec readers
  (`internal/specindex`, `internal/lrfrepo`, `cmd/corvint`) the safe floor is above 20 units. A
  Makefile recipe edit keeps 68 `LANGUAGE_FRONTIER` entries and the `UNOWNED_DIRTY_PATH`, because
  the Makefile is an unowned driver input that runs `node --test`, `go` and scripts: 58 are
  TypeScript `runtime-flags-unresolved`, `e2e-runtime-dependency`, `executable-config-unresolved`,
  `dynamic-import` and `test-runner-unknown` entries that plugin declares path-open, `cfamily` reads
  a Makefile as a build input, and `dotnet`, `swift` and `ruby` declare no FrontierReach. A declared
  per-driver plugin list was rejected: it would drop 6 entries, leave the plan `UNKNOWN` and go stale
  silently when a recipe gains an invocation. `shell:script/dogfood-cache_test.sh` and
  `shell:script/batch-precheck_test.sh` are in every probe plan as `DEPENDENCY_PATH` because a
  non-test script that names the dirty path carries its TSP-V0-005 callers, which is sound;
  `.corvint/test-binary-consumers.json` names only `cmd/corvint-web-flows`, so no hub command widens
  a plan. Rollback: delete this record; no code depends on it.
- **TSP-V0-710:** (experimental, amending TSP-V0-860, under the owner's standing lane auto-accept,
  2026-10-11; V1-3501) A change of `changes.json` MAY carry `recorded: {ciScope, verdict}`, the CI
  scope and selector verdict of the replay the floor was last measured on. `tools/selection-baseline`
  SHALL print, before the floor verdict, one line
  `selection-baseline: MOVED <id> ci <old> -> <new> (verdict <old> -> <new>[: <reason>])` for every
  change whose replayed CI scope or verdict differs from its record, the reason being the replayed
  selector's fallback or `NOTHING` reason, and SHALL print
  `selection-baseline: UNRECORDED <n> change(s) carry no recorded verdict` when any change lacks one.
  The floor and miss checks are unchanged. Refreshing the record (copying each result's `ciScope`
  and `verdict` from a `--out` file into `changes.json`) is part of moving the floor. Non-goals:
  failing on a move alone; recording per-package selections. Failure mode: a record left stale after
  a deliberate floor change makes every later replay print the same MOVED lines until it is
  refreshed. Rollback: drop `Moved` and its lines in `verdictLines`; the `recorded` members are then
  ignored, so remove them with it.
## Non-goals

Evaluating build constraints for a host or running non-host variants; scoping the frontier to the
dirty file's own constraint (a plain file in the directory can be what a variant calls, which
AFP-V0-580's review withdrew a narrower scoping over); selecting a dependent nested module's tests
(AFP-V0-582).

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
it does not model is out of its reach. `internal/gateselect` (the CI PR tier and
tools/gate-ledger) reads the declaration since TSP-V0-940 and shares this failure mode.
(5) TSP-V0-004: the rule is static over one file's text. A sourced runtime file that rebinds NAME
is excluded by refusing any source after the first; an `eval` of computed text naming NAME, a
nameref (`declare -n`) or an indirect `${!x}` assignment it cannot see would let the read escape,
and only a textual occurrence of NAME in another form refuses. `mktemp -d` that fails leaves NAME
empty, so the read is of an absolute path, still outside the repository. The shadowing and PATH
checks see this file only: a function, alias or PATH inherited from the environment (an exported
bash function, say), a `source` the text places after the fresh one but that runs first at run
time, and another process writing into the new directory concurrently are out of the model; the
`command -p` form bypasses functions and a PATH override. The payload `tar` extracts is run-time
data, so a link inside it is the script's own, as before.
(6) TSP-V0-005: the scan is textual. A caller in another language that builds the script's file name from parts is
missed, so the declaration can verify while a caller is unlisted; a shell caller doing so with a
literal shell suffix fails the scan under TSP-V0-960, whose failure mode (12) names what it still
misses. The caller or mention split is
the project's own per-file claim: a file listed as a mention that does run the script loses its
edge, so its tests are not selected for a change to the script. A caller in a file outside the
candidate suffixes (a `.md` runbook an operator pastes from) is not a code path and is not
scanned. A declared script stays an unbounded reader when it is one, so its callers' tests are
selected on every plan; that is the declared price of the narrowing. Each graph build reads every
candidate file once more for the scan. Undeclared non-test shell scripts keep the frontier, so a
plan that reaches one still bears it.
(7) TSP-V0-006: reach is lexical over identifiers, so it over-approximates (any declaration of a
name is reached, a field shares a method's name) and never sees reflection, `unsafe` or a function
reached through an interface method of the same name declared elsewhere; a changed function is
never a value, so it cannot travel into another package or an interface. Its signature passes no
reference, and no write it reaches goes through a pointer, selector or index, so the state it can
change is its own locals or a package variable, which keeps the package whole; state of another
package or the process it changes by calling that package's functions (`os.Setenv`, a setter, an
`append` into a slice another package exposes) is out of the model, as files and the environment are. Like STR-V0-063, a skipped
test is assumed independent of the kept ones except through package variables, which a changed
function may not reach: a test that observes a changed function only through files, the
environment or another process it starts is missed. The slice keeps external tests whole because a
package they import may call the changed function. A package whose source and own test files both
changed was kept whole (44 of 57 such packages on 29 replayed commits) until TSP-V0-861.
(8) TSP-V0-007: a package's prior pass is reused only when the planner does not select it for
M0..M1, so a missed reader in that delta is missed as it is in the deciding plan; a test that passed
at M0 by chance (flaky, time or network dependent) is not rerun; a record is only as trustworthy as
the run that produced it, which executed PR code, so a run able to forge its own go.json could mark a
package passed; the FULL merge-queue run is the backstop for all three. Packages the prior run did
not execute, ran in slices, or skipped (no test files) are never reused.
(9) TSP-V0-860: the change set is as old as the repository (19 merged changes on 2026-10-31,
fewer than V1-2892's 50, because the 1.0 repository was minted on 2026-10-30); the failures are
package-level (a shard annotation names a failing package, not a test), so a narrowed selection
that keeps the package but slices the failing test away is not seen; a failed job whose annotations
name no package is unattributed; a failure that only a non-host variant or another workflow
(Release gates, `cross-vet`) shows is out of the set. The replay uses the tree under test's
planner on historical trees, so it measures today's selection of yesterday's changes, not what CI
ran then.
(10) TSP-V0-861: the union is sound only as far as each half is: a test of a changed test file runs
whatever it reaches, and a test of an unchanged file is skipped only when it neither reaches a
changed function (TSP-V0-006's lexical reach over the head test files) nor uses a name a changed
test file declares; a test file that is deleted, or a change to a test file of another package, is
outside both proofs and keeps the package whole through the other dirty paths' plan.
(11) TSP-V0-864: a new type or const is observed only by code that names it, which lexical reach
finds; a new type's method set could satisfy an interface, so any method on it keeps the package
whole. A comment is no input to the compiled program except a directive, which must match.
(12) TSP-V0-960: the expansion is static over one file. A value set in another file, the environment,
an argument or a command's output is unknown, so a computed name built only from those, or with no
literal shell suffix (`"$tool"` holding a script path from the environment), is still missed, as is a
construction in a Make, Go or YAML candidate; the frontier is then not named and the scan can verify
while such a caller is unlisted. The frontier bears on the constructing file's dependents, not on the
scripts it may run, so a change to the constructed callee does not select the caller's tests; failing
closed through `UnboundedReads`, as NSA-V0-009 does for a dynamic source, was rejected because it
would select those tests on every plan. A test file is a leaf and carries no frontier for what it
constructs. Unknown parts match any text, so a pattern can count a file as naming a script it never
runs, which can only invalidate the declaration, never verify it.
(13) TSP-V0-950: no wrapper confines a shell test to its declared reads, as `.github/testconfine` does a
Go package, so a declaration is the project's claim; a read the toolchain gains outside it (a new
dependency of a built package) is missed until the declaration is widened. TSP-V0-951's repository
test catches the ratchet test's one such drift, a new dependency of the receipt tool; any other
declared test needs its own check.

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
3. Resolved by TSP-V0-861 (V1-3348): a package whose own test files changed with its source runs the
   union of the TSP-V0-006 and STR-V0-063 slices, each under its own proof.
4. TSP-V0-007 is not wired into CI: `.github/workflows/ci.yml` must keep each PR run's shard output
   directories, fetch the previous run's tested merge for the same PR, build the record with
   `--mode prior` and pass `--prior`, and the narrowing gate must decide from `rerun.json` (its
   superset check against the shadow selection unchanged, its `.github` refusal applied to
   M0..M1). Whether a record produced by a run of PR code may narrow a later run is an owner
   decision. Intersecting `rerun.json` with the base-relative `selection.json` is also sound under
   both proofs and would narrow further; it is not proposed here.

## Acceptance evidence and traceability

The tests below, failing on base `fd51f53a2`; the measured plans in build log
`docs/build-log/2026-10-30-v1-3160-frontier-scoping.md`.

| Requirement | Implementation | Evidence |
|---|---|---|
| TSP-V0-001 | `FrontierBearsOnOwnFiles` and `validFrontierBears` in `internal/liveverify/affected/unit.go`; `ownFilesBear` and its two call sites in `internal/liveverify/affected/select.go`; `observeDirectory` in `internal/liveverify/affected/golang/golang.go` | `TestBuildConstraintFrontierBearsOnOwnFiles_TSP_V0_001` (a dirty plain file, variant, absent variant or non-Go file in the directory, alone or with another path, and a root or ancestor file the package reads, bears and stays `UNKNOWN`; a dirty package the variant imports, a named data path and an unrelated package do not, the variant's tests stay selected, and the imported package's plan is `BOUNDED`), the narrowing, V1-0289 and V1-1818 cases in `internal/liveverify/affected/golang/golang_test.go` |
| TSP-V0-002 | `LinkedEntries` in `internal/liveverify/affected/source.go`; `observeDirectory` in `internal/liveverify/affected/golang/golang.go` | `TestBuildConstraintFrontierSymlinkedSource_TSP_V0_002` (a constrained package reached through an imported package, with a dirty symlink target outside its directory: the frontier bears and the plan is `UNKNOWN` with the link, and is not named without it) in `internal/liveverify/affected/golang/golang_test.go` |
| TSP-V0-003 | `readGatedLocators`, `applyGatedLocators` and `carriesLiteral` in `internal/liveverify/affected/golang/gatedlocators.go`; their call in `internal/liveverify/affected/golang/golang.go`; `gatedLocatorsChanged` in `internal/liveverify/affected/readers.go` and its call in `internal/liveverify/affected/select.go`; `.corvint/gated-root-locators.json` | `TestGatedRootLocators_TSP_V0_003` in `internal/liveverify/affected/golang/gatedlocators_test.go` (undeclared: the dependent is selected on an unrelated change; declared: it is not and the plan is `BOUNDED`, the locator's own tests stay selected and a change to the locator still selects it; a gate not carried, a package that is not a locator or not present, a relative gate, an unknown member and unparsed JSON each name the invalid frontier and keep it selected; a dirty declaration is `UNKNOWN`); `TestRootLocatorGated_TSP_V0_003` in `internal/authoritystore/gated_locator_test.go` |
| TSP-V0-004 | `freshDirectorySource`, `makesDirectory`, `onlyExpanded`, `onlyRead`, `freshShadowed`, `freshCheck.allowed`, `tarExtracts` and `trapAllowed` in `internal/liveverify/affected/shell/fresh.go`; block identity, separators, definition counts and function bodies in `parse` in `internal/liveverify/affected/shell/lex.go`; the `command -p` serve payload commands in `script/heavy.sh`; `plainAt` and the source case of `observe` in `internal/liveverify/affected/shell/shell.go` | `TestFreshTemporaryDirectorySourceIsBounded_TSP_V0_004` in `internal/liveverify/affected/shell/shell_test.go` (a heavy.sh-shaped `serve` is bounded; another value, a second assignment, a `for`, `read`, `local` or `unset` rebinding, `${w:=}`, a `local w=` declaration, another function, a subshell, an `if` arm, `&&`, a pipeline, an inner pipeline, a dynamic, `..` or empty rest, `mktemp` without `-d` or with `-u`, a prefix assignment, an `ENVFILE` source, a source between, a second fresh source, a source before the assignment and backquotes each stay dynamic; a PATH-setting `command -p` variant with a function defined between is bounded; an `ln`, `cp`, `mv`, `mount`, `cd`, write redirection, unlisted command, `env` or assignment prefix, tar member file, substituted or brace-group `ln`, `printf -v`, DEBUG or non-exit INT trap, a called function whose body links, is defined later, twice or with a nested block, a `mktemp`, `tar` or `command` function, an `alias`, `hash`, PATH or TAPE assignment, an unprefixed tar under PATH, and a `.` of another file before the assignment each stay dynamic; all but the `command` function and the unprefixed tar were bounded before this amendment) |
| TSP-V0-005 | `readForeignCallers`, `applyForeignCallers` and `foreignCallersComplete` in `internal/liveverify/affected/shell/callers.go` and their call in `internal/liveverify/affected/shell/shell.go`; `Unit.Callers` in `internal/liveverify/affected/unit.go`; `callerEdges` in `internal/liveverify/affected/graph.go`; `foreignCallersChanged` in `internal/liveverify/affected/readers.go`; `.corvint/shell-foreign-callers.json` | `TestForeignCallerDeclaration_TSP_V0_005` in `internal/liveverify/affected/shell/callers_test.go` (declared: the frontier is gone, the plan on an unrelated change is `BOUNDED` and selects the caller, test-file caller and upstream dependents' tests but not a mention's; undeclared keeps it bearing; an unlisted Go, shell, Make, Python or workflow caller or mention, a stale caller or mention, unsorted lists, a script that is no unit, an empty reason, a bad profile, an unknown member, unparsed JSON and a file both caller and mention each name the invalid frontier and keep it bearing; a dirty declaration is `UNKNOWN`); `TestRepositoryShellForeignCallersComplete_TSP_V0_005` in `internal/liveverify/affected/shell_callers_repo_test.go`; `TestBatchReplayDeclaredRuntimeReaders_V12941` frontier floor in `internal/liveverify/affected/replay_v12941_test.go` |
| TSP-V0-006 | `testSourceEligibles`, `testSourceEligible`, `testSourceSlice`, `testSourcePlainFunctions`, `testSourceValueUse`, `testSourceQualifiers`, `testSourceDeclOf`, `testSourceReferenceSignature` and `testSourceWrite` in `cmd/corvint/test_source_slices.go`; their call and the shared base read in `testGoSlices` in `cmd/corvint/test_slices.go`; the TSP-V0-006 verdict line in `testVerdict` in `cmd/corvint/test.go` | `TestTSPV0006DryRunSlicesSourceChanges` in `cmd/corvint/test_source_slices_tspv0006_unix_test.go` (a called function, a doc comment, a function a method calls and an added function slice to the reaching and external tests; a function no test reaches builds only; every test reaching, an own test file, a value use, a package variable reached, initialization reaching, a method, an import, a variable, a comment outside functions, a dependency also changed, a new file, an assembly file, `go:linkname` and cgo stay whole, and so do a function that stopped reaching a package variable, a pointer parameter, a write through another package's variable in the head or only in the base body, a function reaching such a write, and a dot import; the importer stays whole); `TestTSPV0006PlainFunctionsAndQualifiers` and `TestTSPV0006ReferenceSignatureAndWrites` (basic-only signatures; pointer, slice, map, channel, function, interface, named, struct, array, variadic, `uintptr`, pointer-result, generic and redeclared-basic signatures and writes through a selector, pointer, index or range target or by `copy`, `delete` or `clear` are named) in `cmd/corvint/test_source_slices_tspv0006_test.go`; `TestSTRV0072SliceWalksDoNotGrowWithPackages` (two Git processes with a source change present); the mutation check in `docs/build-log/2026-10-30-v1-2806-source-slices.md` |
| TSP-V0-007 | `readPrior`, `rerun`, `buildPrior` and `readPriorEvents` in `tools/corvint-pr-tests/rerun.go`; `selectSince` and the `--prior` branch of `planOnly` in `tools/corvint-pr-tests/main.go` | `TestTSPV0007RerunReusesPriorPasses` in `tools/corvint-pr-tests/rerun_tspv0007_test.go` (the fixed package alone reruns while the base-relative selection keeps both; nothing passed, everything passed and one passed; a missing, malformed, unknown-field, other-schema or invalid-package record, another identity, tree, unresolvable merge or head, a rewritten head, a planner failure, a non-`run` verdict, an unavailable tool profile, a refused plan and a `.github` delta each run `./...`; a record followed by a stray `}` or a second value is malformed); `TestTSPV0007PriorRecordFromRunEvidence` (only whole-package passes no directory contradicts; no directories, another merge, an unobserved merge, another identity, a source-drift or planner-drift fallback reason, an execution error, an unreadable or missing event stream each refuse); the replay in `docs/build-log/2026-10-30-v1-3050-pr-rerun-reuse.md` |
| TSP-V0-861 | `tests` of `testSourceCandidate`, `testSourceEligible`, `testSourceOwnPaths` and the TSP-V0-861 union in `testSourceSlice` in `cmd/corvint/test_source_slices.go` | `TestTSPV0861DryRunSlicesSourceAndTestChanges` in `cmd/corvint/test_source_slices_tspv0861_unix_test.go` (a new internal test file, an edited external test file and a new test of a function no test reached slice to the union; every test kept, a `TestMain`, an `init`, a new import, a failing source proof and a test file with no test function stay whole; a string literal and a prose comment naming the directive slice, and a `//go:linkname` comment stays whole); the "its test file changed too" case of `TestTSPV0006DryRunSlicesSourceChanges`; the replay in build log `docs/build-log/2026-10-31-ci-select-w1.md` |
| TSP-V0-864 | `testSourceAdditive` and the `added`, `additive` and `seeds` handling in `testSourceSlice` in `cmd/corvint/test_source_slices.go` | `TestTSPV0864DryRunSlicesAdditiveDeclarations` in `cmd/corvint/test_source_slices_tspv0861_unix_test.go` (a file comment, a new const no test names, a new const a changed function uses, and a new type with its new test slice; a new var, import, method on a new type, directive, `//+build ignore` without a space, swapped `//go:embed` bindings and a changed declaration stay whole); the "a comment outside functions" case of `TestTSPV0006DryRunSlicesSourceChanges`; the replay in build log `docs/build-log/2026-10-31-ci-select-w1.md` |
| TSP-V0-860 | `replay`, `measure`, `CIScope`, `checkFailures`, `Summarize` and `Failures` in `tools/selection-baseline/main.go`; `benchmarks/selection-baseline/changes.json`; the `selection-baseline-check` target in `Makefile` | `TestSelectionBaselineGateFailsOnFloorOrMiss_V12892`, `TestSelectionBaselineUnknownKindsAndManifest_V12892` and `TestSelectionBaselineChangeSetIsValid_V12892` in `tools/selection-baseline/main_test.go`; the replays in `benchmarks/results/v2892-selection-baseline-*.json` and build log `docs/build-log/2026-10-31-ci-select-w1.md` |
| TSP-V0-710 | `Moved`, `change.Recorded` and the `replay` call in `tools/selection-baseline/main.go`; the `recorded` member of every change in `benchmarks/selection-baseline/changes.json` | `TestSelectionBaselineNamesMovedVerdicts_V13501` (one recorded change flips each way and one carries no record; a move alone leaves the floor check passing) and the recorded-verdict check of `TestSelectionBaselineChangeSetIsValid_V12892` in `tools/selection-baseline/main_test.go`; the records come from the base replay at 12/19 narrowed, 0 misses (build log 2026-11-01-sel-w10) |
| TSP-V0-960 | `variableValues`, `expand`, `braced`, `named` and `constructed` in `internal/liveverify/affected/shell/computed.go`; the computed command and interpreter cases of `observe` and the `built` map in `internal/liveverify/affected/shell/shell.go`; the `built` argument of `applyForeignCallers` and `foreignCallersComplete` in `internal/liveverify/affected/shell/callers.go` | `TestComputedInvocation_TSP_V0_960` and `TestComputedInvocationValueBoundIsUnbounded_TSP_V0_960` (81 values from two nine-word loops make the test an unbounded reader with its 64 kept edges; 64 values stay bounded) in `internal/liveverify/affected/shell/computed_test.go` (a literal `${NAME-default}` value is an import edge with no frontier; a constructed `"$here/$name.sh"` keeps the frontier of a non-test caller, bearing on dependents, and not of a test; a declared script that constructs a name keeps the frontier; an unlisted literal-parts, pattern or interpreter-operand constructor makes the declaration invalid; a listed one and a pattern naming another script keep it declared) |
| TSP-V0-950 | `readShellReadScopes` and `applyShellReadScopes` in `internal/liveverify/affected/shell/readscopes.go` and their call in `internal/liveverify/affected/shell/shell.go`; `shellReadScopesChanged` in `internal/liveverify/affected/readers.go` and its call in `internal/liveverify/affected/select.go`; the `go:` filter in `tools/test-read-scopes-check/main.go` | `TestShellReadScopeDeclaration_TSP_V0_950` in `internal/liveverify/affected/shell/readscopes_test.go` (undeclared: the toolchain test is selected on an unrelated change; declared: it is not, the plan is `BOUNDED`, and a dirty declared entry or a file in its directory selects it; a test that also sources a dynamic path, one that sources a literal helper sourcing a dynamic path directly or two sources away, a non-toolchain test, a source script, an absent file, unsorted or empty reads, a `.git` entry, an empty or multi-line reason, a bad profile, an unknown member and unparsed JSON each name the invalid frontier and keep it unbounded; a dirty declaration is `UNKNOWN`) |
| TSP-V0-951 | `.corvint/shell-read-scopes.json`; `.corvint/unbounded-readers.json`; the `shell:runs-toolchain` line in `tools/unbounded-readers/main.go` | `TestRepositoryShellReadScopes_TSP_V0_951` and `TestListedDirectoriesKeepSpaces_TSP_V0_951` in `internal/liveverify/affected/shell_readscopes_repo_test.go`; `TestRunsToolchainRefusalNamesTheUnitsEntry` in `tools/unbounded-readers/main_test.go` (an undeclared, unrecorded toolchain test still exits 1 and the line names the declaration) |
| TSP-V0-900 | `.corvint/test-read-scopes.json`, `.corvint/test-read-scope-literals.json`, `.corvint/shell-read-scopes.json` and `.corvint/unbounded-readers.json`, read by `ReadScoped` in `internal/liveverify/affected/readers.go` and `applyShellReadScopes` in `internal/liveverify/affected/shell/readscopes.go` | `TestDeclaredReadScopesNarrowSpecAndRequirementsPlans_TSP_V0_900` in `internal/liveverify/affected/readscope_narrowing_repo_test.go` (fails with the base declarations: 30 narrowed units selected); `TestRepositoryShellReadScopes_TSP_V0_951`; `make test-read-scopes-check`; `make unbounded-readers-check`; confined runs and measured plans in build log `docs/build-log/2026-11-01-rsd-w17.md` |
| TSP-V0-901 | the same declarations | `TestDeclaredReadScopesKeepTrueReaders_TSP_V0_901` in `internal/liveverify/affected/readscope_narrowing_repo_test.go` (fails when the `conformance/mcp-2026-07-28` spec read or the `internal/tasks/ticket` fixture read is dropped) |
| TSP-V0-902 | `driverInput`, `bearsOnUnowned` and `readsResolutionInput` in `internal/liveverify/affected/select.go` (unchanged) | the measured plans in build log `docs/build-log/2026-11-01-rsd-w17.md` |
| TSP-V0-940 | `gatedLocators` and `applyGatedLocators` in `internal/gateselect/gatedlocators.go`, their call in `indexRepository` (`internal/gateselect/readers.go`); the two frontiers in `moduleLevelFrontiers` (`internal/gateselect/select.go`) | `TestGatedRootLocatorsMatchThePlanner_TSP_V0_940` in `internal/gateselect/gatedlocators_test.go`: with a nested module and a dependent of a gated locator, the selector's rule (d) equals the planner's `UnboundedReaders` undeclared (the dependent included) and declared (the locator alone), a nested-module change selects the same root-module units in both, an invalid declaration declares nothing, and a plan naming either frontier falls back. |

## Rollback

Remove `frontierBears = affected.FrontierBearsOnOwnFiles` in `observeDirectory`; the constant and
`ownFilesBear` are then inert. No stored state depends on the value. TSP-V0-002: drop the
`LinkedEntries` check in `observeDirectory` only together with TSP-V0-001. TSP-V0-003: delete
`.corvint/gated-root-locators.json` (a change that itself plans `UNKNOWN`); the package is then a
root locator again and its dependents are followed on every dirty path. TSP-V0-004: make
`freshDirectorySource` return false; every such source is then dynamic again, as before.
TSP-V0-005: delete `.corvint/shell-foreign-callers.json` (a change that itself plans `UNKNOWN`) or
make `applyForeignCallers` return at once; every script then carries the frontier again and the
`Callers` edges are empty. TSP-V0-006: in `testGoSlices`, stop collecting `sources` (leave every
`DIRECT_SOURCE_CHANGE` selection on the `reason` path); every such package then runs whole with its
original reason. No stored state depends on it. TSP-V0-007: stop passing `--prior`; `rerun.json` is then
not written and only `selection.json` decides, as before. TSP-V0-860: delete the
`selection-baseline-check` target; no other target or stored state depends on it. TSP-V0-861: in
`testSourceEligible`, return the fallback again when a test file of the package is dirty. TSP-V0-864: make
`testSourceAdditive` return false; every residue change keeps the package whole again. TSP-V0-960: make
`constructed` return false and pass an empty `built` map; computed words are then ignored again.
No stored state depends on it. TSP-V0-950 and TSP-V0-951: delete `.corvint/shell-read-scopes.json` (a
change that itself plans `UNKNOWN`) and restore the ratchet test's `units` and `reasons` entries in
`.corvint/unbounded-readers.json`; every toolchain shell test is then an unbounded reader again.
TSP-V0-940: drop the `applyGatedLocators`
call in `indexRepository` and the two `moduleLevelFrontiers` entries; the selector then follows a
declared package's dependents on every dirty path again, as before.
