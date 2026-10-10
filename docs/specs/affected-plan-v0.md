# Affected Plan V0

Owner: Russell Lewis
Date: 2026-09-01
Requirement prefix: `AFP-V0`
Intent status: accepted for AFP-V0-008 (decision 0057) and AFP-V0-009 (decision 0052); AFP-V0-013/014/015 accepted (decision 0289); AFP-V0-016/017 accepted (decision 0320), AFP-V0-016 amended (decision 0390); AFP-V0-021 accepted (decision 0376); AFP-V0-009 and AFP-V0-021 amended (decision 0424); AFP-V0-023 owner-directed, proposed (2026-10-01, V1-0246); AFP-V0-031/032/033 accepted (decision 0438); AFP-V0-034 accepted (decision 0439); AFP-V0-035 proposed (V1-0943; no GitHub issue); AFP-V0-036 proposed (V1-0984; no GitHub issue); AFP-V0-037 proposed (V1-0991; no GitHub issue); AFP-V0-038 proposed (V1-0995; no GitHub issue); AFP-V0-039 proposed (V1-0662; no GitHub issue); AFP-V0-040 accepted (decision 0487); other AFP-V0 requirements proposed
Delivery status: experimental
Authoritative inputs: `docs/specs/go-live-test-provider-v0.md` (provider plan wire and non-goals),
`docs/specs/live-proof-carrying-verification-v0.md` (future composer, not-started),
`internal/liveverify/affected` (selector), `AGENTS.md` invariants 2, 4, and 8.

## Agent digest
- Claim: `corvint affected` emits a read-only, non-authoritative affected-test selection plan with provider-ready Go package paths.
- Status: accepted for AFP-V0-008 (decision 0057) and AFP-V0-009 (decision 0052); AFP-V0-013/014/015 accepted (decision 0289); AFP-V0-016/017 accepted (decision 0320), AFP-V0-016 amended (decision 0390); AFP-V0-021 accepted (decision 0376); AFP-V0-009 and AFP-V0-021 amended (decision 0424); AFP-V0-023 owner-directed, proposed (2026-10-01, V1-0246); AFP-V0-031/032/033 accepted (decision 0438); AFP-V0-034 accepted (decision 0439); AFP-V0-035 proposed (V1-0943; no GitHub issue); AFP-V0-036 proposed (V1-0984; no GitHub issue); AFP-V0-037 proposed (V1-0991; no GitHub issue); AFP-V0-038 proposed (V1-0995; no GitHub issue); AFP-V0-039 proposed (V1-0662; no GitHub issue); AFP-V0-040 accepted (decision 0487); other AFP-V0 requirements proposed/experimental
- Exists: `internal/liveverify/affected`, `corvint affected`, `cmd/corvint/affected_test.go`, the `advice` member (AFP-V0-009: repository-declared mandatory checks, one advisory Go command, the unknown frontier), the `--base FULL_COMMIT_ID` range form and `range` member (AFP-V0-010), and the `make gate-affected` fast tier over the receipt (AFP-V0-011: `script/gate-affected.sh`, fail-closed to the full `go-test` run; not the push gate), whose union is attributed per dirty path from a static repository index of imports and path literals (AFP-V0-012), whose literal-reader rule also adds, in the plan itself, selections for every dirty path a package names, without narrowing an unowned path's `UNKNOWN` scope (AFP-V0-021); `tools/corvint-pr-tests` and `.github/workflows/ci.yml` remain full until separately pinned AFP-V0-014 qualification; AFP-V0-022 adds complete advisory CI partitions and a digest-bound experimental sharded PR profile; AFP-V0-023 lets a project-owned `.corvint/test-read-scopes.json` take a root-locating package off the rule (d) floor, enforced in full CI by the Landlock wrapper `.github/testconfine`.
- Blocked on: the LPCV-V0 composer accepting or replacing this wire; genuine 200-row qualification and matching reviewed pins (AFP-V0-014/017); the 201-commit prerequisite is met at `adf8358220769b8d6724ad27d27625602b8a7c62`, but no campaign PASS is implied.
- Read next: Requirements; Non-goals and authority; Failure modes.

## Intent and scope

Before this slice `internal/liveverify/affected` (eight language plugins) was reachable from no
binary, and `cmd/corvint-go-test-provider` ran only the packages named in its bundle. The provider
spec forbids selection inside the provider (`go-live-test-provider-v0.md` §11). This slice connects
the two halves at the operator's hand: selection is produced as a separately labelled, non-
authoritative plan; execution stays explicit. Affected user: an engineer or agent that wants to
run the tests a change plausibly needs before the mandatory gate. Measurable job: produce a
deterministic plan for one dirty worktree in bounded time with an explicit unknown frontier.

## Requirements

- **AFP-V0-001:** The command MUST be read-only: one bounded `git status`, one HEAD identity read,
  one source walk, at most two bounded advice-declaration reads (AFP-V0-009), one bounded
  read-scope declaration read (AFP-V0-023), one bounded binary-exec declaration read
  (AFP-V0-037), with `--base` one
  bounded base identity read and one bounded `git diff --name-only` (AFP-V0-010), no test
  execution, no index, trace, cache, or ledger write, and no `observeUnsupported` call on failure.
- **AFP-V0-002:** The dirty set MUST come from `affected.DirtyPaths` (porcelain v1, NUL-delimited,
  untracked included, ignored excluded, 8 MiB / 10 s bounds) and MUST fail closed on overflow,
  malformed output, or an unavailable Git. The one directory record Git emits under
  `--untracked-files=all`, a nested repository or linked worktree as `?? DIR/`, is admitted as
  the dirty path `DIR`, which no plugin owns, so the plan widens to `UNKNOWN` over it rather than
  the capture failing. (proposed, decision 0398; V1-0314) A status or range path whose Git bytes
  are not UTF-8 is a changed path, not malformed output: it enters the set in display form, each
  invalid byte run replaced by U+FFFD, so its directory prefix still drives ownership and widening.
  The dirty set and HEAD MUST be re-read after the graph
  is built; any difference MUST fail closed as `unsupported-affected-drift`, because the status and
  the graph are two observations of one mutable worktree.
- **AFP-V0-003:** Stdout MUST be one canonical JSON line with exactly the members `advice`
  (AFP-V0-009), `mutates=false`,
  `ok=true`, `plan` (the selector's canonical `affected.Plan`), `profile="affected-plan/0"`
  (this is the `--full` document; the default is its AFP-V0-035 `affected-plan/1` projection),
  `provider.go.{packages,state}`, `range.{base,paths}` (AFP-V0-010), `revision` (HEAD commit id),
  and `tool="affected"`.
  `provider.go.state` MUST be one of `RUNNABLE`, `EMPTY_SELECTION`, `MODULE_PATH_UNRESOLVED`, or
  `PACKAGE_BOUND_EXCEEDED`; `packages` MUST be non-empty only when the state is `RUNNABLE`, and is
  then the sorted, de-duplicated import paths of selections in the `go:` namespace, at most 4,091
  entries (the `go-live-plan/0` argv bound). The module-path state exists because unit identities
  are directories rather than import paths when the Go plugin reports `go:module-path-unresolved`.
- **AFP-V0-004:** The plan is not authority. `plan.scope` MUST be `UNKNOWN` whenever `plan.unknown`
  is non-empty; the provider's plan wire MUST keep `NO_AFFECTED_SELECTION_PROOF`; no consumer may
  treat an exclusion as proof that the excluded test is safe to omit. (proposed, decision 0398;
  V1-0340) A dirty path no Go unit declares MUST follow AFP-V0-012's structural rules (a) and (b)
  rather than exclude the package it touches. A current-tree-unindexed Go path is a
  `DIRECT_SOURCE_CHANGE` of the package observed in its directory, which is then traversed to its
  dependents; with no package there, every unit with an import edge to an absent in-module Go unit
  whose last path component is the directory's name is a changed unit with a `DEPENDENCY_PATH`
  witness; at the module root (directory `.`), every unit importing an absent in-module package is
  selected this way. The Go plugin keeps an import under an observed module path that resolves to
  no package as an edge to that absent `go:` identity for this purpose. Any other unowned or
  non-Go path selects every Go package whose directory encloses it with witness kind
  `ENCLOSING_PACKAGE`; the nearest such package, when the path sits directly in its directory,
  and every enclosing package whose non-test files carry `//go:embed` (`Unit.embeds`) are also
  traversed to their dependents. The `UNINDEXED_SOURCE_PATH` and `UNOWNED_DIRTY_PATH` entries
  stay, so `plan.scope` stays `UNKNOWN`. This replaces the former exclusion reason
  `UNINDEXED_DIRTY_GO_PATH_MAY_BE_DELETED_OR_RENAMED`, which named the package that lost a file
  but left it and its importers unselected. Rollback restores that reason.
  (proposed, decision 0398; V1-0289) A plugin's frontier reason MUST be a `LANGUAGE_FRONTIER`
  entry of every plan the plugin takes part in, and only of those: a plugin takes part when it
  owns a dirty path (a unit of it declares the path, or its `Owns` claims it), when it owns a
  reached unit, when its units read any path (`affected.PathReader`; the Go plugin, whose path
  tokens and rule (d) reads mean a unit it could not observe may read any dirty path), and every
  plugin takes part once a dirty path is owned by none or no path is dirty. A reason that bears only on one unit is that unit's sorted `frontier` member, covered
  by the graph digest, and is named only by a plan that reaches the unit. The Go plugin's
  `go:build-constraint-variants` is such a reason, because every variant's imports are edges, so
  the closure is a superset of each variant's; every other plugin reason stays plugin-wide,
  because it hides edges or units of that plugin. Rollback names every graph frontier reason in
  every plan again.
- **AFP-V0-005:** For a fixed tree, HEAD, and dirty set the document MUST be byte-identical across
  runs; with `--base`, the base commit is part of that fixed input.
  (proposed, decision 0398; V1-0299) `plan.graphDigest` MUST be `affected-graph:sha256:` followed by
  the lowercase hex SHA-256 of the domain tag `corvint-affected-graph/1` and one line feed, then the
  deterministic JSON projection `{"languages","frontier","units"}`: the sorted participating plugin
  names, the sorted graph frontier, and, in unit-id order, each unit's `id`, `sources`, `tests`,
  `imports`, `testImports`, `pathTokens`, `pathTokensBounded`, `embeds`, `unboundedReads`,
  `locatesRoot`, `readScoped`, `readScope` (AFP-V0-023), `execs` (AFP-V0-037) and `frontier`, every
  member always present except `readScoped` and `readScope`, which appear only for a declared unit,
  and `execs`, which appears only when non-empty, so that a graph without them keeps its digest (`digestBody` and `digestUnit` in
  `internal/liveverify/affected/graph.go`). The projection is fixed there, not by the internal
  `Unit` struct, so a new internal field changes the digest only when it is added to the
  projection, and a unit field left out of it fails the test. The value is an identity, not a
  CCF-V1-002 identifier: it has changed between releases on identical input (0.7.0 to 0.8.1, and
  again with this derivation), so a reader compares digests only from one release.
- **AFP-V0-006:** Failures MUST exit 2 with a typed code on stderr and no partial document:
  `unsupported-affected-revision`, `unsupported-affected-status`, `unsupported-affected-graph`
  (including an unreadable subtree, or an accepted source file whose repository-relative path is no
  canonical unit path, such as a name holding a backslash, both of which `affected.SourceFiles` now
  refuses instead of silently narrowing the unit set), `unsupported-affected-drift`, or
  `invalid-arguments`.
- **AFP-V0-007:** `plan.selected` MUST be ordered by exactly three keys: (1) reverse-dependency
  distance from the dirty path, ascending — the length of `witness.via`, so a directly changed
  unit precedes its dependents; (2) within one distance, the number of leading directory
  components the unit's sources or tests share with `witness.dirtyPath` (the longest such run over
  the unit's paths), descending; (3) then `unitId`, ascending. Every key is a function of the graph
  and the dirty set alone, so the order is covered by AFP-V0-005; it is not a ranking claim under
  AFP-V0-004 — a consumer that truncates the plan reads the nearest selections first, never a
  proof about the rest. `plan.excluded` stays in `unitId` order: an exclusion has no witness, so
  keys (1) and (2) do not apply to it.
- **AFP-V0-008:** The Go plugin's unit universe is the root module, or, when the root holds a
  `go.work`, every module a `use` directive names (a `use DIR` line or one parenthesized block,
  read from text; an entry outside the root is skipped and raises the
  `go:workspace-module-outside-root` frontier, because its packages and every edge into them are
  absent from the graph, V1-0327). Each observed module's packages are
  units under that module's own import path, so an edge between two workspace modules resolves
  as an edge inside one does. A `go.mod` below the root that no `use` names is the
  `go:nested-module-frontier`: its packages are absent from the graph, never attributed to the
  module above them (narrowed per module by AFP-V0-031). A listed module whose path cannot be read is `go:module-path-unresolved`.
  Directories named `build`, `dist`, and `target` are explicitly admitted at every depth for Go
  source. Each admitted directory subtree has its own `affected.MaxIncludedDirectoryEntries` bound
  of 20,000 entries. An entry beyond that bound skips the remainder of only that subtree and adds
  `go:included-directory-walk-bounded` to the language frontier, widening the plan to `UNKNOWN`
  without failing graph construction. As the go tool does, the plugin ignores every directory
  named `testdata` below an observed module's root, with its descendants: no unit is built from
  it, a `go.mod` there is not a nested-module frontier, and the plugin does not own a `.go` path
  with a `testdata` component. A changed fixture file is therefore `UNOWNED_DIRTY_PATH`, never an
  untested package under AFP-V0-020, and the package that holds the `testdata` directory is traced
  from its own files as before (V1-0203). `Owns` sees only the repository-relative path, so an
  unindexed `.go` path of a workspace module whose root lies below `testdata` is also
  `UNOWNED_DIRTY_PATH` rather than `UNINDEXED_SOURCE_PATH`; the plan is `UNKNOWN` either way. The
  toolchain is never executed. (proposed 2026-09-25, not accepted; V1-0291) An import only a
  package's `_test.go` files declare is a `testImports` edge, kept apart from `imports` with
  `go list -deps -test` semantics: a change the traversal reaches selects that package's tests one
  edge further, witnessed as `DEPENDENCY_PATH`, and the traversal does not continue to the
  package's importers, which never compile its tests. An import its non-test files also declare
  stays an ordinary edge.
- **AFP-V0-009:** (accepted 2026-09-04 by decision 0052) The receipt MUST carry an `advice` member with exactly the
  members `status="PLAN_ONLY"`, `checks`, `unknown`, and `note`, plus `test_selection` only when
  `--provider` is given (ETS-V0-002, `docs/specs/external-test-selection-v0.md`). Each `checks` entry has exactly
  `command`, `kind` (`mandatory` or `advisory`), `reason` (one sentence), and `source` (a repository
  path or `affected-plan`). A `mandatory` entry MUST come only from a repository-owned declaration
  read from the working tree at the root: a `gate:` target in `Makefile` yields `make gate`, and a
  fenced `sh`/`bash`/`console` block under a heading whose text is exactly "Verify" in `AGENTS.md`
  (amended, see below) yields each of its non-empty command lines with a leading `$ ` stripped. Both reads are bounded at 256 KiB per
  file and the mandatory list is deduplicated in order of appearance and capped at 16 entries; a
  truncated read still yields the declarations inside the bound. When neither declaration exists,
  `checks` MUST carry no mandatory entry and `unknown` MUST gain
  `NO_REPOSITORY_GATE_DECLARED: no Makefile gate target or AGENTS.md Verify block`. The single
  `advisory` entry exists only when `provider.go.state` is `RUNNABLE`, is
  `GOTOOLCHAIN=local go test -count=1 <packages>` over `provider.go.packages` with source
  `affected-plan`, and is otherwise replaced by one `unknown` line naming the state. `checks` MUST
  be ordered mandatory-first in source order, then the advisory entry; `unknown` MUST be the sorted,
  deduplicated projection of `plan.unknown` as `<reason>: <detail>` plus those synthetic lines. The
  advice is static: no check is executed, no entry may be derived from `plan.excluded`, and a
  mandatory check stays required whatever the advisory list says (AFP-V0-004). A fenced line whose
  first non-space character is `#` is a comment, not a command. A read that hits the 256 KiB bound
  adds `unknown` gaining `MANDATORY_DECLARATION_TRUNCATED: <path> exceeded 262144 bytes` and MUST
  NOT also add `NO_REPOSITORY_GATE_DECLARED`; a declaration the 16-entry cap drops adds
  `MANDATORY_DECLARATION_CAPPED: <path> declared more than 16 commands`. The advisory command's
  package arguments are each POSIX single-quoted, with an embedded `'` escaped as `'\''`, so an
  unusual import path cannot break a shell paste.
  (accepted 2026-09-26, decision 0424; from decision 0398; V1-0342) Only a heading whose text,
  without its `#` marks and surrounding space, equals `Verify` in any case declares checks; a
  substring match made every shell line under a heading such as `## Build / run / verify`
  mandatory, app launches included. Each other heading whose text contains `verify` in any case and
  that has such a fence MUST add `MANDATORY_DECLARATION_UNRECOGNIZED: AGENTS.md heading "<text>" is
  not "Verify", so its commands are not checks` to `unknown` instead of yielding checks. A shell
  comment, a `#` at the start of the line or after a space or tab, outside single or double quotes
  and not escaped by a backslash, is removed from each command line with the whitespace before it,
  and a line left empty is no command. A command that ends in a single `&` or whose first word is
  `open` or `xdg-open` does not end on its own: it MUST be an `advisory` entry with source
  `AGENTS.md`, listed after the mandatory entries and before the plan's advisory entry, and counts
  toward the 16-entry cap. Any other command stays `mandatory`, because requiring too much is safe.
  `NO_REPOSITORY_GATE_DECLARED` is added only when no entry is mandatory and neither declaration
  source was truncated. Rollback restores the substring heading match and whole-line commands.
- **AFP-V0-010:** `corvint affected --base FULL_COMMIT_ID` (or `--base=`) MUST join the committed
  range to the dirty set: the paths of one bounded `git diff --name-only -z --no-renames --no-color
  BASE HEAD --` (`affected.RangePaths`, the AFP-V0-002 8 MiB / 10 s bounds; each NUL-delimited
  entry MUST be a valid relative path) are unioned, deduplicated, and sorted with the worktree dirty
  set before selection, so `plan.dirty` is that union and every AFP-V0-004 widening rule applies to
  a committed path exactly as to a dirty one. The receipt's `range` member MUST carry exactly `base`
  (the given commit id; `""` without `--base`) and `paths` (the sorted range paths alone; `[]`
  without `--base`), so a consumer can separate the committed contribution from the worktree's.
  The argument MUST be one full 40- or 64-hex commit id (`invalid-arguments` otherwise, as for any
  other argument); a base that is not a commit in the repository is
  `unsupported-affected-revision`; diff overflow, malformed output, or an unavailable Git is
  `unsupported-affected-status`. The base is immutable content: it is read once and is not part of
  the AFP-V0-002 drift re-read, which still covers the worktree and HEAD.
- **AFP-V0-011:** `make gate-affected` (`script/gate-affected.sh [BASE]`; the Makefile's `BASE`
  defaults to `main`, and `BASE=` tests the dirty worktree alone) is the fast tier of the test gate.
  It MUST run the `go-test` target's own command (`GO_TEST_COMMAND`, one Makefile definition both
  tiers append their package pattern to) over the union AFP-V0-012 derives from
  `provider.go.packages` and every path in `plan.dirty`, then `go-vet`, `go-format-check`,
  `spec-requirements-check`, `decision-numbers-check`, and `line-citations-check`. It MUST fall back
  to the full `./...` run, naming the reason on stderr as `gate-affected: FALLBACK <reason>`, when:
  the base does not resolve to a commit; the plan cannot be produced; `provider.go.state` is neither
  `RUNNABLE` nor `EMPTY_SELECTION`; `plan.unknown` carries a `LANGUAGE_FRONTIER` at module level
  (`go:module-path-unresolved`, `go:included-directory-walk-bounded`, `go:unparsed-source`,
  `go:cgo-frontier`); `go.mod`, `go.sum`, `go.work`, or `go.work.sum` is in `plan.dirty`; a path in
  `plan.dirty` or an entry of `plan.unknown` carries a control character (checked before any line
  echoes it, so no path can forge the verdict line, and a dirty path the plan reports in neither
  `provider.go.packages` nor `plan.unknown` cannot pass unseen); the repository cannot be indexed
  for attribution (AFP-V0-012); a package path is not a plain import path under the root module;
  the union is empty while `plan.dirty` is not; or the selector's last line is not a `run`,
  `FALLBACK`, or `NOTHING` verdict. A document, a testdata fixture, a script, or an unowned path does
  not fall back by itself: AFP-V0-012 names the packages that can read it. Measured 2026-09-12 by
  replaying `corvint affected --base <first parent>` on the 50 first-parent commits ending at
  `09e4bc2f`, with the selector over each commit's tree: this rule falls back on 0 and narrows all
  50, to 112.9 packages on average (minimum 99, median 109, maximum 138) of the 184 that `./...`
  names. The rule it replaces fell back on 48, and its 2 narrowed runs averaged 6.5 packages.
  `b5c04a72` selects `internal/specindex` as a `reader` of `docs/decisions/README.md`. The floor is
  the 95 packages AFP-V0-012 marks unresolved at `09e4bc2f`. An empty union over an empty
  `plan.dirty` runs no test and says so. A hangup, interrupt, or termination signal MUST end the run with a
  nonzero status, never read as a failed plan that starts the fallback. Every run MUST print its audit lines on stdout (`base`,
  `selected <pkg>`, `frontier <pkg> <- <path>`, `data <path>`, `reader <pkg> <- <path>`,
  `unresolved <pkg>: <reason>`, and the `run <command> <packages>` line) so a narrowed run can be
  checked afterwards. The steady-state frontiers `go:build-constraint-variants` and
  `go:nested-module-frontier` do not fall back: one or both are present on nearly every change here
  and name packages the selector never claims. The fast tier is not the push gate: `make gate` is unchanged
  and stays the mandatory check (AFP-V0-009 keeps advising it). The selector MUST refuse a plan
  file above 8 MiB before JSON decoding, so external plan input cannot consume unbounded memory.
- **AFP-V0-012:** (decision 0131) The fast tier's selector MUST attribute every dirty path from a
  static index of the repository under test, built without a build or a Go toolchain. The index
  walks the root module, skipping `.`, `_`, and `testdata` directories and every subtree holding a
  nested `go.mod`, within 200000 entries and 8 MiB per `.go` file. For each `.go` file it reads
  the imports (`go/parser`, imports only) and lexes the string literals outside import declarations
  into path tokens: runs of `[A-Za-z0-9._~@+/-]` once printf verbs are removed, with a root-module
  import path rewritten to a root-anchored path; exact equality names the root package itself. A token's component runs are its maximal runs of
  components other than empty, `.`, and `..`. A package's dependents are the reverse closure over
  two edges: an import of a root-module package, and a token with a component run equal to that
  package's directory (a test that builds `./cmd/corvint` depends on it). For each dirty path the
  union MUST gain:
  (a) for a `.go` path with no `.`, `_`, or `testdata` directory component outside a nested module
  (`frontier`), its directory's package when one exists, and that directory's dependents, which the
  importers of a deleted file or package still reach;
  (b) for any other path (`data`), every package whose directory is an ancestor of the path, the
  nearest one's dependents when the path sits directly in its directory, and the dependents of
  every such ancestor whose non-test files carry `//go:embed` (an embed pattern reaches into a
  subdirectory that is a package of its own);
  (c) for every path (`reader`), each package whose own files carry a token with a component run
  naming it. A run of two or more components matches consecutive path components, its first by
  suffix unless the token starts there and its last by prefix unless the token ends there; a single
  component matches only a whole path component. For the CEM sidecar `.corvint/change.cem.json`
  (decision 0323), derived evidence every dogfooded change commits, such a package is selected
  only when that token, resolved against the package's directory (a root-anchored token against the
  root), can form the sidecar or one of its ancestor directories while preserving the outer
  partial-component matches above. A root-climbing or compatible root-anchored token in the same
  package MAY establish the root for a separate naming fragment because the literal-only index
  cannot prove whether the expressions compose; a `.corvint` or `change.cem.json` token joined only
  to a fixture root selects nothing, and rules (a), (b), and (d) are unchanged;
  (d) whenever `plan.dirty` is non-empty (`unresolved`), every package whose reads no literal bounds
  and that no valid AFP-V0-023 declaration names; a declared one is selected by its scope instead.
  Such a package calls `runtime.Caller` or `os.Getwd` or carries the literal `--show-toplevel` in any
  file, or has a file that does not lex. A call counts under whatever local name the file's own
  import gave the callee package — its default name, an alias (`rt "runtime"`), or unqualified under
  a dot import (`import . "runtime"`) — not only the literal text `runtime.Caller`/`os.Getwd`. It may
  instead have a test file carrying a token that ends
  in `/...`, is made only of `..` components, or resolves from the package directory to the
  repository root or above. It may have a non-test file carrying a token that climbs with `..` to a
  named component, which resolves against whichever test runs it. Or it depends on a package whose
  non-test file meets one of these conditions.
  The selector MUST fall back (AFP-V0-011) when the walk fails or exceeds a bound, or when a `.go`
  file cannot be read, its opened handle does not still match the regular directory entry the walk
  classified, its imports do not parse, or it is a symlink: `go build`/`go test` reads a symlinked
  `.go` file like any other, so the index cannot skip it without under-selecting that file's
  dependents, and does not follow it in place. Residual assumptions, each recorded in decision 0131:
  a dependency's own path literals resolve against a root or directory its caller supplies,
  so they select only that dependency, not its dependents; a bare `..` or `../` in production code
  rejects a path rather than climbing; a root taken from an environment variable, a flag, or a
  subprocess other than `git rev-parse --show-toplevel` is not detected; and a path assembled only
  from fragments shorter than one whole component is not detected.

Simpler baseline: run the full mandatory gate. This slice never replaces it; it proposes a narrower
first pass whose omissions stay `UNKNOWN`.

Trust boundary and resource limits: the command trusts only the local Git worktree and the source
text it walks. It launches Git twice for status and twice for HEAD identity (10 s deadlines, 8 MiB
status bound), with `--base` once more for the base identity and once for the range diff (same
bounds), walks at most `affected.MaxWalkEntries` entries, reads at most
`affected.MaxSourceBytes` per file, and executes nothing else. Every failure is fail-closed.

- **AFP-V0-013:** (accepted by decision 0289) `tools/corvint-pr-tests` MUST execute only
  typed root-module Go package argv from a separately pinned trusted planner and the existing
  AFP-V0-012 selector outside the PR checkout. It MUST verify the exact captured merge commit,
  clean tree, event base as first parent and event head as second parent, with Git replacements
  disabled and grafts refused; validate receipt profile/base/revision and package grammar;
  recheck source before execution; and preserve planner/selector digests, audit, identities,
  fallback reason, exact argv and observed exit outside source. Advice strings MUST NOT execute.
  Missing/malformed/stale qualification, unsupported tool/profile, planning failure or topology
  drift MUST run the full root Go suite. Cancellation MUST exit nonzero and kill owned process
  groups without starting fallback. Main/release, static/security/vet/format/build/interop and
  JavaScript/E2E checks MUST remain full. PR workflow permissions MUST be read-only, with no
  persisted credentials, secrets or pull_request_target execution. Initial empty trusted pins
  MUST stay visibly full-suite; local fixture PASS MUST NOT claim hosted or narrowed CI PASS.
- **AFP-V0-014:** (accepted by decision 0289) PR narrowing MUST require a separately trusted,
  SHA256-pinned qualification artifact matching exact reviewed tool source/binaries, Go 1.27.1 (decision 0293),
  OS/architecture/release and C compiler identities, fixed `go test -json -p 1 -count=1 -race
  -timeout 50m` argv, closed `GOENV=off`, `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOWORK=off`, empty `GOFLAGS`,
  `GOSUMDB=off`, `CGO_ENABLED=1`, owned build cache and empty read-only module cache. Freeze
  exactly 200 distinct ordered contiguous first-parent base/target pairs before execution.
  Capture selection and the complete root package universe before one full invocation per row;
  retain JSON/stderr, argv, elapsed time, exit and raw hashes. Every package MUST have one terminal
  outcome. Timeout, build failure, infrastructure failure or incomplete outcomes MUST invalidate
  the row and qualification; no replacement rows. Any omitted failing package MUST fail
  promotion, without automatic UNKNOWN waiver. Resume MUST validate exact frozen identities and
  raw hashes; measure one row before the remaining serial campaign. A wholly green corpus MUST
  retain its limited counterfactual evidence. Artifact/workflow pins MUST remain separate from
  tool source and precede final release gate freeze. Clearing pins MUST restore full execution.

### Shared isolated container profile

- **AFP-V0-015:** (accepted by decision 0289) The trusted PR driver SHALL support one 1-based indexed row from the unchanged frozen
200-pair corpus. Each container row starts in a fresh owned checkout, with HOME/TMP/GOCACHE
empty immediately before the fixed race invocation; run mode SHALL use the same cold-cache
boundary after planning. Qualification SHALL accept retained read-only rows separately from
its writable output. Missing rows, altered pairs, profile mismatch or incomplete outcomes fail.

The retained `corvint-pr-container/1` profile and its receipt reader remain exact Go 1.27.0.
Current native execution requires Go 1.27.1 under decision 0293; current-source execution in the
retained image is unqualified. The launcher delegates to separately supplied, hashed, read-only
trusted binaries and their immutable source identity. Neither that boundary nor retained receipts
qualifies a current-launcher/legacy-tools pairing. Migrating the image requires separate review and
measurements; the historical image and config identities below remain unchanged.

The optional trusted launcher SHALL use the official Linux amd64 Go 1.27.0 image manifest
`sha256:eef6a67266eeed3c86dd47fd01b32faa8bf0229eb83eb3d4d466e80391bd3820`,
with verified registry config `sha256:ccb6f18cbf10486608b5fea50834a621b9fada5dd869841f08c486ba307155a3`.
Docker's observed image ID MUST equal that exact manifest or its verified config digest,
while the requested image reference MUST remain the exact digest-qualified reference.
The only accepted no-new-privileges inspection spellings are one bare entry or one `:true`
entry; false, duplicate or additional security options fail. The existing /work tmpfs MUST
retain explicit `exec`; this normalization changes no resources or isolation.
Frozen trusted binaries remain outside the tested source and are mounted read-only. Inspect
must establish image, 2 CPUs, GOMAXPROCS=2, 8 GiB memory/no additional swap, 14 GiB /work tmpfs,
read-only root, dropped capabilities, no-new-privileges, disabled network and exact read-only
input mounts. The tmpfs shares the memory ceiling; this is a memory-backed bounded profile,
not a claim of 14 GiB usable disk or equal host performance. The source input is a dedicated
full clone; cloning SHALL use an exclusive owned protected-scope config naming only the exact root and root/.git, removed before tests. Normal/test Git environments SHALL remain unchanged; no user-global config is read or modified. Only the private checkout/runtime/evidence is writable. No engine startup, settings
change, image publication, service, scheduler or automatic 200-row launch is admitted.

The launcher SHALL use typed in-container tar for tmpfs evidence, with at most 16 KiB framing beyond actual member bytes and bounded zero-only trailing drain before process completion. Run exit 1 SHALL require coherent retained execution/selection and failing package evidence; missing or inconsistent evidence remains infrastructure failure. The launcher SHALL own cleanup before container creation, export only fixed bounded regular
files without archive path extraction, and report failed removal as failure. One real fixture,
interruption cleanup and one historical row must establish viability before the 200-row run.
Fake Docker regression success is not real-container success. Hosted execution and protected
workflow admission remain separate and unverified until observed. Rollback removes the launcher
and container qualification; full fallback remains available.

### Hosted control plane and qualification

- **AFP-V0-016:** (accepted by decision 0320; ruleset and consent amended by decision 0390)
  `.github/workflows/ci-control-plane.yml` SHALL run
  on `workflow_run` after `CI`, never check out or execute PR code, and post the commit status
  `ci-control-plane` on the PR head: `failure` when a changed or renamed-from path is under
  `.github/` or the PR exceeds the files API's 3000-file listing, `success` otherwise. An API
  failure MUST post nothing. Its permissions MUST be only `contents: read`, `pull-requests:
  read` and `statuses: write`. The `main` ruleset SHALL require a pull request and the
  `go-product`, `ci-control-plane` and `doc-gates` checks (`doc-gates` is the `ci.yml` job
  added by PR #212, V1-0244), with no bypass actor. Consent to a `.github/` change SHALL be a
  repository admin posting `ci-control-plane` `success` on the exact reviewed head SHA (`gh api
  -X POST repos/beamfall/corvint/statuses/<head sha> -f state=success -f
  context=ci-control-plane -f description="admin reviewed .github change at <sha>"`). The
  latest status per context wins, so consent binds to that one SHA and is logged with its
  creator; `go-product` and `doc-gates` stay binding, so no merge happens while checks run; a
  later push gets a fresh workflow `failure` and needs fresh consent.
- **AFP-V0-017:** (accepted by decision 0320) `.github/workflows/pr-tests-qualification.yml`
  SHALL be `workflow_dispatch` only, with `contents: read` and no persisted credentials. It
  SHALL build the three trusted binaries from the dispatched `tool_source`, freeze the corpus
  ending at `corpus_end`, run each requested row (`1` or all 200) on its own fresh runner
  provisioned as `ci.yml`, and qualify only when every row job succeeded, with the frozen
  driver. Concurrent rows on separate runners satisfy AFP-V0-014's campaign only because every
  row identity must equal the frozen identity. It MUST NOT commit, pin, or publish anything
  but workflow artifacts.
- `AFP-V0-018`: (proposed) `corvint affected --playwright-config PATH` MUST emit the separate
  `playwright-affected/0` profile defined by `TJAA-V0-010..017`. It MUST accept `--base` with the
  same range semantics as `affected-plan/0`, MUST NOT be combined with external `--provider`, and
  MUST preserve the same bounded Git/status/HEAD drift checks and revalidate the bounded source-content
  digest immediately before emission. The receipt has exactly `mutates`,
  `ok`, `plan`, `profile`, `range`, `revision`, and `tool`; unsupported config/source observation
  fails with `unsupported-playwright-affected` and no partial receipt. The default invocation and
  its closed `affected-plan/0` bytes remain unchanged.
  Optional `--playwright-discovery FILE` requires `--playwright-config` and reads only a bounded
  caller-owned `playwright-discovery/0` receipt. The plan MUST reconcile exact project/file pairs
  against immutable HEAD, config and current source bytes before emitting file argv; unproven
  discovery MUST emit empty selected/excluded rows and one complete-config `fallbackArgv` as
  specified by TJAA-V0-015. The input MUST be re-read before emission; drift fails with
  `unsupported-affected-drift`. Corvint MUST NOT execute discovery or config.

- `AFP-V0-019`: (proposed technical contract implementing owner-requested issue #57)
  `affected --snapshot FILE` MUST admit only a closed `corvint-planning-snapshot/0`
  receipt: `schema`, `commitRevision` (current HEAD commit), `treeRevision` (that
  commit's complete source/configuration tree), `baseRevision` (existing commit),
  `changedPaths` (sorted unique canonical repository paths equal to the complete
  no-renames base-to-commit diff), and `changedPathsSha256` (lowercase SHA-256 of
  canonical JSON of that array, without newline). Full lowercase Git object IDs
  are required. Missing, null, duplicate, unknown, incomplete, stale or mismatched
  inputs MUST fail closed; snapshot refusal is `unsupported-planning-snapshot`
  with exit 2 and no partial stdout, never fallback to live files. A valid snapshot
  MUST use immutable blobs for source and mandatory-check declarations, ignoring
  unrelated dirty/untracked paths; HEAD MUST still match before output. The
  optional `snapshot` output member MUST identify `IMMUTABLE_COMMITTED_SNAPSHOT`,
  the exact receipt and `sourceConfigTree`, `worktree=NOT_EVIDENCE`,
  `status=PLAN_ONLY`, and `accepting=false`. Inputs are authoritative only for
  those Git bytes; test selection, exclusions and runtime coverage remain advice.
  The existing output without `--snapshot` MUST remain unchanged. The Playwright
  form remains non-accepting and retains missing-discovery full-suite fallback.
  V0 MUST reject overlays, `--base`, external providers/discovery, symlink/gitlink
  trees and incomplete materialization; it MUST NOT infer a working-tree digest.
  Bounds are 512 KiB receipt, 4,096 changed paths, 8 MiB tree/diff output,
  20,000 regular blobs, 64 MiB batch output, and bounded contained Git calls.
  Private scratch MUST be outside the repository and removed on return/failure;
  no index, checkout, filter, archive attribute, executable source or test runs.
  Rollback removes this explicit opt-in route; existing fail-closed profiles stay.

- `AFP-V0-020`: (proposed; tickets V1-0187, V1-0204) A changed unit (one that owns a changed path)
  with no selectable test MUST be named in `plan.unknown` with reason `NO_SELECTABLE_TEST` and its
  unit id as detail, never silently omitted, so `plan.scope` is `UNKNOWN` (AFP-V0-004). Each plugin
  defines "no selectable test". A Go package has none when it declares no test of its own, because
  the go tool runs a package's tests only against that package. Every other plugin may keep tests in
  the unit itself or in units that depend on it, so its changed unit has none when no unit it
  reaches through the graph, itself included, declares a test. An untested unit that only depends
  on a change, or is not reached, raises nothing. The Playwright plan does not yet name such a
  unit: it widens only on the shared graph's other unknowns, so a changed helper that no spec
  reaches leaves it `BOUNDED` with nothing selected for that helper until ticket V1-0211 decides
  how it reports one. Rollback restores the silent skip.
- `AFP-V0-021`: (accepted, decision 0376; ticket V1-0126) Every dirty path, as in rule (c) whether or not a
  plugin owns it and including a changed source file, MUST also select every unit not otherwise
  reached whose own files carry a path token naming it, with witness kind `PATH_LITERAL_READER`
  and `via` holding that unit alone; the reader is not traversed to its dependents, and a unit
  already reached keeps its seed or `DEPENDENCY_PATH` witness. Tokens and the naming relation are
  AFP-V0-012's rule (c) lexicon: string literals outside import declarations, printf verbs
  removed, runs of `[A-Za-z0-9._~@+/-]`, an import path under the owning module rewritten to a path
  anchored at that module's directory, matched by component run. The Go plugin records them as
  the unit's sorted `pathTokens`, which the graph digest covers. A package with more than
  `affected.MaxPathsPerUnit` distinct tokens keeps none and is marked `pathTokensBounded`; a plan
  with any dirty path then carries `LANGUAGE_FRONTIER` `go:path-token-bound:<unit id>` for each
  such unit nothing else reached, and a clean plan carries none. A Go file that does not lex
  raises `go:unparsed-source`, except under a `testdata` or `_`-prefixed directory, which the go
  tool never builds. A match adds and removes no unknown entry: an unowned path keeps its
  `UNOWNED_DIRTY_PATH` entry, so `plan.scope` stays `UNKNOWN`, because a literal index cannot
  bound a read whose path is built at run time (AFP-V0-012 rule (d)). A reader's witness is the
  smallest dirty path naming it. (accepted 2026-09-26, decision 0424; from decision 0398; V1-0230)
  For the CEM sidecar `.corvint/change.cem.json` (`affected.ChangeEvidencePath`, equal to
  `frontier.ExcludedPath`) a Go unit is a reader only when one of its naming tokens resolves as rule
  (c) narrows it: against the unit's directory, or against the repository root when the unit also
  carries an anchored or parent-only token, to the sidecar or an ancestor, outer components matching
  as fragments. An anchored naming token always counts, because the graph does not record the module
  directory it is anchored at, so this is a superset of rule (c)'s sidecar readers. A non-Go unit
  and any other dirty path keep the component-run relation.
  (proposed 2026-09-25, not accepted; V1-0290) A one-component run of a token that is not anchored
  at a module root and has no `..` in it names only a dirty path's file name, never one of its
  directory components: the `internal/` of `"internal/%03d.go"` would otherwise make its package a
  reader of every path under any `internal` directory, while `"../../.corvint"` climbs to a
  directory. Runs of two or more components, climbing tokens and anchored tokens match as rule (c)
  does, so here `affected` selects a subset of rule (c)'s readers. Rollback
  removes the reader selections and the bound entries; the paths stay unknown as before.
  (accepted 2026-09-26, decision 0424; from decision 0398; V1-0230) Rule (d) is modelled for Go: a
  package that calls `runtime.Caller` or `os.Getwd` (through a plain, aliased or dot import),
  carries the literal `--show-toplevel`, has a file whose source after its import declarations does
  not lex outside a directory the go tool never builds, or carries a literal that climbs with `..`
  to a named component (a non-test file) or that is made only of `..` components, resolves to the
  repository root or above, or is a `/...` pattern (a test file) records the first such reason, a
  non-test one preferred, as the unit's `unboundedReads`, and `locatesRoot` when a non-test file is
  the cause. Both are covered by the graph digest. On any non-empty dirty set, every such unit,
  every dependent of a `locatesRoot` unit, and every test user of a `locatesRoot` unit or of such a
  dependent, not otherwise reached, is selected with witness kind `UNBOUNDED_READER`, `dirtyPath`
  the smallest dirty path and `via` that unit alone, and is not traversed; a clean plan selects
  none. The `UNOWNED_DIRTY_PATH` entry still stays: rule (d) is a lexical heuristic that cannot
  bound every read built at run time, and no plugin but Go records path tokens, so `plan.scope`
  stays `UNKNOWN` for an unowned path. Rollback removes the `UNBOUNDED_READER` selections and the
  two unit members.

### Complete-universe PR partition profile

- **AFP-V0-022:** (owner-directed experimental profile, 2026-10-01; V1-0616/0617) Full CI SHALL
  partition every package from the complete runtime `go list ./...` universe exactly once across
  the matrix. Advisory bounded package costs may change placement, never membership; unknown
  packages use a positive estimate, and unavailable or invalid costs fall back to complete lexical
  round-robin partitioning. Each admitted PR shard SHALL intersect its selection with that same
  complete-universe partition, so independently selected and full fallback shards cannot omit a
  selected package. An empty intersection SHALL retain an explicit audit and execute no Go test.
  The protected partition implementation and cost bytes SHALL be built independently of tested
  module directives; its digest and shard count SHALL be part of the AFP-V0-014 frozen identity.
  A driver/fallback partition digest mismatch SHALL fail the check rather than mix partitions.
  Historical shadow rows SHALL still execute one complete root invocation with all terminal
  package outcomes; the 200-row qualification, exact event topology and mandatory full checks
  remain required. Sharded container execution is unsupported. Promotion of selective hosted CI
  remains blocked until genuine qualification and reviewed immutable pins; local fixtures and
  timing simulations cannot establish hosted speedup. Rollback clears the selection pins and
  reverts partition placement to the previous complete round-robin workflow.
  (V1-0714) The cost bytes SHALL be regenerated by `tools/ci-shard-costs refresh` from the terminal
  `go test -json` package outcomes of one complete hosted run, recording that run and its revision;
  a failed or repeated package outcome, logs that lack a package the current table lists (unless
  the operator states the removal), or a table the partition would reject, SHALL refuse the
  refresh and leave the table unchanged. `tools/ci-shard-costs check` SHALL report each package whose observed time differs from
  its entry by more than the stated factor (default 2, ignoring differences under 10s), each
  executed package the table lacks, and each entry the run did not execute. The check is advisory
  operator evidence: it never changes membership and is not a CI gate.
  (V1-0716) Within a full pull-request shard the affected plan MAY order, never select: the
  protected helper's `--order` moves the plan's selected Go packages to the front of the shard
  (changed units, including a package whose own non-Go file changed, and their dependents, then other bounded witnesses, then unbounded readers, then every
  unselected package), keeping the helper's existing order inside each class. The output SHALL be
  a permutation of the same shard, so no ordering input can add, omit or move a package between
  shards. The planner SHALL be built from the pull-request event base commit, never the tested
  head. A planner that fails to build or run, a missing artifact, or a plan that is oversized,
  unparseable, not `affected-plan/0` or not `ok` SHALL leave the current order. The plan carries
  no selection or skip authority here and needs no AFP-V0-014 qualification. Rollback removes
  `--order` from the workflow.

### Declared confined test read scopes

- **AFP-V0-023:** (owner-directed, proposed, 2026-10-01; V1-0246, V1-0081) A project MAY commit
  `.corvint/test-read-scopes.json`, the closed object `{"profile":"corvint-test-read-scopes/0",
  "packages":{DIR:[ENTRY...]}}`, at most 1 MiB, 4096 packages and 256 entries per package. `DIR` is
  a canonical relative directory, never `.`, holding an observed Go package of the root module; an
  `ENTRY` is a canonical relative path, optionally ending in one `/` to name a subtree, never `.git`
  or below it, strictly ascending within its package. The declaration states that the package's
  test processes read nothing under the repository root except its own directory subtree and the
  declared entries. The planner (`readScoped`/`readScope` unit members, part of the AFP-V0-005
  projection) and the AFP-V0-012 selector MUST then leave that package out of rule (d), whatever
  makes it unbounded, and select it, with witness `DECLARED_READ_SCOPE` or the selector line
  `declared <pkg> <- <path>`, when a dirty path is in its subtree, equals or lies under an entry,
  is an ancestor of an entry, or is the declaration itself. Rules (a) to (c) and every dependency
  edge are unchanged; a package that depends on a declared root locator but is not itself declared
  stays on the floor. A declaration that is present but not a regular file, oversized, not exactly
  this grammar, or names a directory with no package MUST declare nothing: the planner raises the
  module-level frontier `go:test-read-scopes-invalid` and the selector falls back (AFP-V0-011).
  Full CI SHALL run the root Go test command with `-exec` set to the owner-protected wrapper
  `.github/testconfine`, built isolated from tested module directives, which loads the same
  grammar independently, fails the run on any invalid declaration, a Landlock ABI below 2, an
  existing declared directory or entry reached through a symbolic link, or an entry whose trailing `/`
  disagrees with whether it is a directory, never grants a symbolic link outside the root, and
  execs each declared package's test binary under a Landlock ruleset granting read access to every
  path outside the root, the package subtree and the existing declared entries only, with
  `-buildvcs=false` appended to `GOFLAGS` because a nested `go build` cannot read `.git` to stamp
  VCS information; undeclared packages run unconfined. The ruleset also handles and grants link/rename reparenting (REFER)
  everywhere, because an unhandled REFER refuses every cross-directory link or rename; Landlock
  still refuses a reparenting that would widen a file's read access, such as a move out of a denied
  tree. A declaration that is too narrow therefore fails that package's tests
  in full CI rather than silently under-selecting. Residuals, each retained rather than closed:
  Landlock does not mediate `stat`, `access`, `readlink` or the existence of a path, so a test
  whose outcome depends only on metadata or absence under the root is not confined; a test that
  tolerates a denied read is not detected; a subprocess outside the test binary (a `go build` of
  another package) inherits the ruleset, so such a package cannot be declared narrowly; the
  AFP-V0-013 driver's selected and shadow runs are not confined until its frozen argv and
  identity carry the wrapper, which MUST precede any AFP-V0-014 campaign that relies on a
  declaration; and `make gate-ledger` `-unresolved`/`-bounds` stay whole-tree for declared
  packages. Rollback deletes the declaration, which restores rule (d) for every package; removing
  the `-exec` wrapper alone without deleting the declaration is not a supported state.
- **AFP-V0-024:** (owner-directed, proposed, 2026-10-04; V1-0717) A push to `main` MAY reuse a
  passed pull-request result instead of repeating the root Go race invocation, only when all of
  the following hold: the pushed commit is a two-parent merge; a completed, successful
  `pull_request` run of `.github/workflows/ci.yml` in this repository exists for the merged head;
  and that run retained, for every shard of the current shard count, a record named
  `ci-tested-tree-TREE-full-SHARD-of-SHARDS` whose `TREE` is exactly the pushed commit's tree
  id. A shard SHALL retain that record only after its complete package set passed in the full
  branch, so a documentation-only (DCI-V0) or selection-narrowed run can never be reused. Tree
  identity binds every tracked file, including the workflow file blob and the pinned Go
  version; no separate comparison is trusted. REUSE therefore means that the pull-request
  merge-ref commit with the identical tree passed. A fork pull-request run is admitted like any
  other: it runs in this repository, and a record for the merged head can only come from a
  workflow that head, and so the merged tree, contains. The decision SHALL be made by `tools/ci-reuse-plan`, built from
  the pushed `main` commit, over retained public run and artifact listings read with no
  permission beyond `contents: read` (ARTIFACT-V0-008 is unchanged); artifact contents are not
  downloaded. Only the root race invocation is omitted: static, build, interop, documentation
  and artifact checks still run. The decision, with the reused run id and tree id, SHALL be
  retained as the `ci-reuse` artifact. A missing record, an unreachable API, a failed build, any
  other event or any mismatch runs FULL. Limits: the record name is written by the reused run's
  own workflow steps, which the tree identity binds to the merged, owner-protected workflow;
  the commit id and history of the checkout, the event environment, the installed host
  packages and the hosted runner image are not part of the identity and may differ between the
  two runs; the
  hosted reuse path and its effect on completed `main` results are `NOT_OBSERVED` until a
  merge lands with matching records. Rollback removes the `reuse` step, which restores FULL
  for every push.
- **AFP-V0-025:** (owner-directed, proposed, 2026-10-04; V1-0719, V1-0752) The repository SHALL
  commit `.corvint/unbounded-readers.json`, the closed object
  `{"profile":"corvint-unbounded-reader-set/0","units":[DIR...],"reasons":{DIR:REASON}}`, at
  most 64 KiB. `units` names, strictly ascending, the test directories of the admitted
  unbounded test units: units with tests that rule (d) selects on any dirty path because
  neither a literal nor a declared read scope (AFP-V0-023) bounds their reads. `reasons`
  records, for each of those directories (AFP-V0-033), why a package's reads cannot be declared.
  `tools/unbounded-readers` builds the same unit graph the planner builds and
  `make unbounded-readers-check`, a `doc-gates` and `make gate` step, MUST fail, naming the
  directories, when an unbounded test unit is not in `units` or shares its directory with
  another, or when the record is absent or not exactly this grammar, which includes a
  `reasons` directory outside `units`. A `units` directory that is not an unbounded test
  unit passes and is reported for removal; nothing removes it automatically. Adding a
  directory to `units` is an ordinary reviewed edit of the record: the check makes growth
  visible, it does not forbid it. The record is a set and not a count so that it merges the
  way the tree does (V1-0752): two changes that each add an unbounded package with its
  entry, or one that adds such a package while another's last check is stale, merge to a
  record that names both, where two identical edits of a count merged to one increment. Each full pull-request run
  SHALL also report, in the step summary of shard 0 and as the `affected-share` artifact
  (`corvint-ci-selected-share/0`), the estimated time of the packages the advisory AFP-V0-022
  plan selected as a share of the complete universe, priced with the partition's cost
  estimates and the median for an unpriced package, with the part held by packages selected
  only as unbounded readers. The report is a shadow metric: it never narrows what runs, and a
  missing plan or unreadable estimates only omit it. Limits: the estimates are one retained
  hosted run, not this run's measured time; the set is of units, not of their cost; a
  `reasons` entry is a reviewed statement, not a proof that no narrower declaration exists.
  The set does not remove every merge skew, because a unit is also unbounded when a
  dependency's non-test code locates the root: a change that makes a package locate the
  root beside a change that adds a test package importing it, a change whose last check
  ran before this requirement reached its base, a change to how the graph classifies units,
  or two changes to one package that disagree about its reads can still merge to a failing
  record; the push run then fails and names the directories. Only a required up-to-date
  branch or a merge queue, which are repository settings outside this contract, closes
  those.
  Rollback removes the make step and the two workflow steps.
- **AFP-V0-026:** (owner-directed, proposed, 2026-10-04; V1-0752) `.github/workflows/ci.yml`
  SHALL also run on `merge_group` `checks_requested`, so a merge-queue entry reports the
  required `go-product` and `doc-gates` checks on the exact commit that becomes `main`. Such a
  run MUST be FULL: the documentation classifier (DCI-V0), the AFP-V0-013 driver, the
  AFP-V0-022 order plan and the AFP-V0-024 reuse stay bound to their own events and are off
  for it. `.github/workflows/ci-control-plane.yml` SHALL post `ci-control-plane` `success` on
  the head of a completed `merge_group` run of `CI` whose branch begins
  `gh-readonly-queue/main/pr-`, and post nothing for any other branch; it still never checks
  out or executes repository code. That status carries no consent of its own: a pull request
  enters the queue only after the AFP-V0-016 decision on its own head, and a queue commit
  holds only such pull requests on top of `main`. Limits: enabling the queue is the owner's
  ruleset change and hosted queue behavior is `NOT_OBSERVED` until then; each merge runs the
  suite on the queue commit and again on the `main` push unless the pull-request run tested
  the same tree, because AFP-V0-024 admits only pull-request results; the pull-request run
  still tests a possibly stale merge, so only the queue run closes the AFP-V0-025 residual
  skews; the status is posted only after the whole `CI` run completes, so the queue's
  status-check timeout must exceed that run; the queue-entry precondition, the branch form
  and `workflow_run` delivery for `merge_group` runs are GitHub behavior assumed here, and a
  wrong assumption about the latter two posts nothing; the status stays on a queue commit
  after its entry leaves the queue, so a pull request whose head is that commit shows it
  until its own `CI` run completes and AFP-V0-016 replaces it, which can admit only
  `.github/` content already consented to on a queued head. Rollback: disable the queue in the ruleset
  first, then remove the trigger and the `merge-group` job; removing them while the queue is
  on leaves every entry waiting for checks that never report.

- **AFP-V0-027:** (owner-directed, proposed, 2026-10-05; V1-0809) A pull request labelled
  `ci:batched` is a constituent whose commits a batch pull request carries and tests; the
  `go-product-shard` and `go-static` jobs SHALL NOT start for it, so a constituent push spends no
  race-shard runner time. The label SHALL only withhold tests, never admit a merge: the skipped shard job
  makes the required `go-product` check fail with a message naming the label, so armed
  auto-merge cannot land an untested constituent, and the constituent lands only as part of a
  batch whose own run (or merge-queue run, AFP-V0-026) tests the combined tree. The label is read
  from the event payload, so it is off for `push` and `merge_group` events, and removing it takes
  effect on the next push or reopen, not on a re-run of an earlier event. `doc-gates`,
  `docs-plan`, `go-interop` and `artifact-integrity` still run. Limits: the label is applied by
  the batching agent and nothing checks that a batch pull request actually contains the
  constituent; a mislabelled pull request is blocked, never merged untested. Rollback removes
  the job condition and the `go-product` message.
- **AFP-V0-031:** (accepted by decision 0438; V1-0867; narrows the AFP-V0-008 nested-module
  frontier) The Go plugin SHALL decide the `go:nested-module-frontier` per unlisted module:
  it reads each such module's `go.mod` through the same `affected.Source` as every other input
  (the worktree, or the immutable tree under `BuildFS`, where a read error stays fatal), and the
  frontier is raised only when at least one module stays open. A module stays open when its
  manifest is unreadable, larger than 1 MiB, or not parsable by a reader that lexes as the go
  tool does and fails on any verb, token, block, or directive argument it does not admit (for
  example an invalid or quoted `go` or `toolchain` version, a quoted `godebug` or one without
  `=`, a malformed `retract` interval, a raw string, a quote inside an unquoted argument, a block
  of a verb the go tool does not admit as a block, a module version that is not canonical or
  whose major does not match the module path's suffix, or a replacement whose target is a
  directory with a version or a module path without one); when it requires, replaces, or names a tool under an observed
  module path (a tool under its own module path excepted), or replaces a module with one; when a
  directory replacement is absolute or not repository-relative, resolves outside the repository,
  or resolves anywhere other than its own directory or another unlisted module (a relative
  replacement that resolves to the root, or into an observed module, therefore keeps it open);
  when a `go.work` sits in its directory or any ancestor below the root; or when the observed
  side draws it in: an observed `go.mod` or the root `go.work` requires, replaces, or names a
  tool under its module path, or replaces a module with its directory, because the observed
  build then compiles its packages and resolves their imports against the observed modules.
  Every nested module stays open when the observed side cannot be read the same way: an observed
  module path is unresolved, an observed `go.mod` or the root `go.work` is unreadable, over-size,
  or unparsable, an observed `go.mod` declares a module path other than the one the plugin read
  (an escaped quoted path, for example), the root `go.work` use set differs from the observed
  directories, or an observed directory replacement is not repository-relative or does not name
  exactly an observed module directory or a path at or below an unlisted module's directory (a
  differently cased or linked path can name a nested module, so identity is not established).
  A module that is
  none of these neither builds against an observed module nor is built by one,
  so no change to an observed module reaches it and it closes. The plugin keeps one evidence
  record per nested `go.mod` read, with the reason it stayed open; it is internal and does not
  reach the plan wire. The reverse direction is unchanged: a nested module's files are never
  units, so a change inside one is still `UNINDEXED_SOURCE_PATH` or `UNOWNED_DIRTY_PATH`.
  Non-goals: reads by a nested module's tests of observed data (AFP-V0-012 already treats nested
  literals as selecting nothing), symlinked manifests the walk does not follow, and a `GOWORK` or
  `GOFLAGS` (`-modfile`) environment value, which the plugin cannot observe. Rollback restores the unconditional frontier
  in `observeModules` and removes `nested.go`.

- **AFP-V0-032:** (accepted by decision 0438; V1-0865) Only a declaration path that
  does not exist is absent for AFP-V0-009. A `Makefile` or `AGENTS.md` path that exists but is not a
  readable regular file (a directory, a FIFO or other special file, a dangling symlink, or an open,
  stat or read error) MUST add `MANDATORY_DECLARATION_UNREADABLE: <path> exists but is not a
  readable regular file` to `unknown`, yield no checks from that path, and MUST NOT add
  `NO_REPOSITORY_GATE_DECLARED`. The read MUST NOT block: it opens non-blocking and confirms a
  regular file before reading. A symlink to a readable regular file is still read, as before.
  Rollback restores the previous reader, which treated these paths as absent.

- **AFP-V0-033:** (accepted by decision 0438; V1-0868) Every `units` directory of
  `.corvint/unbounded-readers.json` (AFP-V0-025) SHALL have a `reasons` entry that names the
  unbounded read, the call, literal, inherited dependency or path set that leaves the package's
  reads unbounded, and why an AFP-V0-023 declaration cannot bound it, such as a read of `.git`, a
  listing of the repository root or of a whole top-level tree, an open of the filesystem root, a
  nested build or a test that skips when a read fails. `make unbounded-readers-check` MUST fail,
  naming the directories, when a `units` directory, current or stale, has no entry; the report
  lists them as `unreasoned`. A declaration that takes a package out of `units` SHALL be measured
  the AFP-V0-023 way: the per-test `go test -json` outcomes, unconfined and under the wrapper with
  exactly the declared entries, are identical, and a test that skips in either run, or tolerates a
  failed read, keeps the package undeclared unless its skip is shown not to depend on a repository
  read and the test, when enabled, reads nothing outside the entries. When the evidence is missing
  or ambiguous the package stays in `units`. Limits: a reason is a reviewed statement from the
  source and a container measurement, not a proof that no narrower declaration exists; the
  measurement covers the Linux test files only. Rollback restores the optional `reasons` check;
  the reasons themselves stay as documentation.
- **AFP-V0-034:** (accepted by decision 0439; V1-0416; no GitHub issue) The disk source reader (`ReadSource`, `readSourceFile` in
  `internal/liveverify/affected`) admits a unit on the descriptor it reads, not on a separate
  look-up of the name. On darwin and linux it opens the path `O_RDONLY|O_NOFOLLOW|O_NONBLOCK`:
  a symlink is refused at open (`ELOOP`/`EMLINK` become `ErrInvalidUnit`), a FIFO cannot block
  the open, and the regular-file and `MaxSourceBytes` checks run on `fstat` of that descriptor;
  a body that grows past the bound after the stat is refused (`ErrWalkLimit`) while it is read.
  A path whose open fails for any other reason (a socket, a directory the process may not read)
  is `Lstat`-ed only then, so a non-regular path keeps the `ErrInvalidUnit` the `Lstat` path
  gave it and the success path stays at one open and one `fstat`.
  Other platforms keep the `Lstat`-then-`ReadFile` pair (`read_other.go`). The refusals, error
  values and bound are unchanged; the change removes one path resolution per source, which was
  19% of `affected` CPU on a 200,000-file repository (3.7 of 19.7 sampled seconds; see
  `docs/build-log/2026-10-06-v1-0416-refusal-order-and-affected-reads.md` for the before/after
  table). `ReadBounded`'s disk path still takes the pair (follow-up). Falsifier: a symlink,
  directory or FIFO at a unit path that is read rather than refused, or a sparse file over the
  bound that is read. Rollback: delete `read_unix.go` and `read_other.go` and restore the
  `Lstat` body of `ReadSource` in `walk.go`.
- **AFP-V0-035:** (proposed; V1-0943; no GitHub issue) Without `--full`, the worktree,
  `--base` and `--snapshot` forms MUST write `profile="affected-plan/1"`: the AFP-V0-003 document
  with every member except `plan` unchanged and `plan` projected as follows. `graphDigest`,
  `dirty`, `scope` and `unknown` are unchanged; each `selected` entry keeps `unitId` and `witness`
  and replaces `tests` with `testCount`, the length of that list; `excluded` becomes one object
  `{count, digest, groups}`, where `count` is the number of exclusions, `digest` is
  `affected-excluded:sha256:` plus the lowercase hex SHA-256 of the exact `plan.excluded` bytes
  of the `--full` document for the same inputs (`gokernel.CanonicalJSON`, an empty array when
  nothing is excluded), and `groups` states each distinct `{reason, universe, invalidation}` once
  with its `count`, sorted by reason, universe, then invalidation. A bare `--full` (at most once)
  MUST write the `affected-plan/0` document byte-for-byte as before; `--full` with
  `--playwright-config` is `invalid-arguments`, and the Playwright profile is unchanged. In-repo
  readers that check the profile (`internal/companionrelease` core smoke, `.github/cishards`,
  `tools/corvint-pr-tests`) MUST accept both identifiers; a reader that needs test files
  (`tools/retrieval-bench`) MUST pass `--full`. Falsifier: a default document whose digest differs
  from the hash of the `--full` exclusion bytes, whose counts differ from the full lists, or a
  `--full` document that differs from the previous default. Rollback: delete
  `cmd/corvint/affected_compact.go` and the `--full` option, restore `affected-plan/0` as the
  default, and restore the renamed core-freeze goldens.
- **AFP-V0-036:** (proposed (V1-0984); no GitHub issue) A dirty-path set whose only paths in an
  indexed Go unit P are P's own declared `_test.go` files MUST select P's tests with a
  `DIRECT_TEST_CHANGE` witness and MUST NOT traverse from P: P's importers and the units whose
  tests import P are not reached through P, because Go compiles a test file only into P's own
  test binary and no other package can import it. They stay excluded with
  `NO_DEPENDENCY_PATH_TO_DIRTY_UNIT` unless another dirty path reaches them. A non-test dirty
  path in P, a deleted or unindexed Go path in P's directory (AFP-V0-012 rule (a)), or a data
  path that traverses from P (rule (b)) keeps P a traversal root, and P's witness then names its
  smallest dirty source path rather than a test path. The rule is Go-only; every other plugin
  keeps the conservative traversal. Path-literal readers (AFP-V0-021), unbounded readers
  (rule (d)), declared read scopes (AFP-V0-023), language frontiers and `UNKNOWN` scope are
  attributed per dirty path exactly as before. Measured on this repository at `0b5096ca` with
  one new `internal/contextindex` test file: 99 selected (72 `DEPENDENCY_PATH`) before, 43
  selected (0 `DEPENDENCY_PATH`) after; see
  `docs/build-log/2026-10-07-v1-0984-affected-test-only-changes.md`. Falsifier: a test-only Go
  change that selects an importer through a `DEPENDENCY_PATH` witness, or a non-test change in
  P that does not. Rollback: remove `testOnlyGoUnits` and its use in `reach`, and the
  source-first witness preference in `seed`, in `internal/liveverify/affected/select.go`.

- **AFP-V0-037:** (proposed (V1-0991); no GitHub issue) A package whose code or tests run the built
  binary of a command, a `package main` of the root module, depends on that command's whole build
  though no import edge says so. The Go plugin MUST record such a command edge as the unit's
  `execs` member (sorted command unit ids, part of the AFP-V0-005 projection when non-empty) from
  either source: a path token of the unit (AFP-V0-021) that names a command's directory exactly,
  resolved against the module root when anchored or plain (`go build ./cmd/corvint` runs there)
  and against the unit's directory when climbing, never the unit itself; or the project-owned
  declaration `.corvint/test-binary-execs.json`, the closed object
  `{"profile":"corvint-test-binary-execs/0","packages":{DIR:[COMMAND_DIR...]}}`, at most 1 MiB,
  4096 packages and 1 to 256 strictly ascending entries per package, for a package that runs a
  binary it is handed (a `--corvint` flag) and so names no literal. Every `DIR` must hold an observed
  Go package and every entry an observed command other than `DIR`. A declaration that is present but
  unreadable, oversized, not a regular file or invalid in any member keeps no declared edge and
  raises the module-level frontier `go:test-binary-execs-invalid`; literal edges stay. Selection
  MUST then select, with witness `BINARY_EXEC` and `via` ending in the command then the consumer,
  every consumer of a command that the dirty set reaches through the dependency closure
  (traverse) or the enclosing-package rule, before the read-path rules; the consumer is selected
  one edge past the command and not traversed further, like `testUsersOf`. A command reached only
  as a test user, or a change that reaches no command, selects no consumer. Limits: a literal that
  names a command directory as data over-selects; a command at the module root (`.`) is never a
  literal target; an exec consumer's importers are not selected through it; a dirty
  `.corvint/test-binary-execs.json` is an unowned path and widens the plan to `UNKNOWN`; a
  `_test.go`-only change to a command still counts as reaching its build. Intent: `corvint affected`
  missed `conformance/host-lifecycle-v1` on a `cmd/corvint/help.go` change, with no import edge and
  no stated uncertainty (`docs/build-log/2026-10-07-v1-0991-affected-binary-readers.md`).
  Falsifier: a package that runs a command's binary and is excluded on a change the command's build
  reaches, or a `BINARY_EXEC` selection on a change that reaches no command. Rollback: delete
  `execs.go`, `golang/binaryexecs.go` and `.corvint/test-binary-execs.json`, the `Execs` member and
  its digest field, the `execUsersOf` call in `reach`, and the gate tool's frontier entry.

- **AFP-V0-038:** (proposed (V1-0995); no GitHub issue) In the `affected-plan/1` default, the
  advisory check whose `source` is `affected-plan` MUST NOT repeat `provider.go.packages`: when its
  `affected-plan/0` command is exactly `GOTOOLCHAIN=local go test -count=1` followed by every
  `provider.go.packages` entry POSIX single-quoted and joined by one space, the check MUST carry
  `command="GOTOOLCHAIN=local go test -count=1"` and `arguments="provider.go.packages"`, and appending
  those entries the same way MUST reproduce the `affected-plan/0` command exactly. Every other check,
  and any command that does not have that form, MUST be kept whole without `arguments`; `--full` and
  the mandatory checks are unchanged. Falsifier: a default document whose advice names a package
  path, or whose resolved advice differs from the `--full` advice. Rollback: drop
  `compactAffectedAdvice` and copy the `affected-plan/0` advice into the default again.

- **AFP-V0-039:** (proposed (V1-0662); no GitHub issue) A Go test fixture under
  `internal/liveverify` that hides the host's global Git config (`GIT_CONFIG_GLOBAL` or a `HOME=`
  override) and runs a Git subcommand that ends by launching automatic maintenance (`am`,
  `cherry-pick`, `commit`, `fetch`, `merge`, `pull`, `rebase`, `revert`) MUST pass
  `-c maintenance.auto=false -c gc.auto=0` on that command, because CI's global settings no longer
  reach it and a detached `git maintenance` can outlive the test and fail its `TempDir` cleanup.
  Config isolation, assertions and cleanup are unchanged. A source-level guard over
  `internal/liveverify` MUST fail any such file that lacks either string literal. Falsifier: a
  fixture test under `internal/liveverify` whose Git trace records a `git maintenance` child
  launch, or such a fixture that passes the guard without both literals. Rollback: delete
  `internal/liveverify/affected/fixture_maintenance_test.go` and the two config arguments in each
  fixture helper.

- **AFP-V0-040:** (accepted by decision 0487, 2026-10-10; no ticket; no GitHub issue) CI SHALL
  report drift of the AFP-V0-022 cost table without ever failing on it. Each
  full `go-product-shard` whose `go test -json` invocation passed SHALL keep that stream and upload
  it as the artifact `ci-shard-costs-<shard>` with 3-day retention; a docs-only, reused, selected
  or empty shard keeps none, and only `go test`'s own exit status decides the shard. Capture is
  best-effort: when its directory or file cannot be created the shard runs `go test` without `tee`
  and retains nothing, so capture and retention never decide the required shard. The job
  `ci-shard-cost-drift` (`needs: go-product-shard`, run unless cancelled) SHALL run
  `tools/ci-shard-costs check --advisory --shards N` only when every shard succeeded and exactly N
  outcome artifacts are present, and otherwise SHALL abstain with its reason in the job summary
  (invariant 2). The advisory form appends a Markdown table to the job summary and SHALL exit 0
  whatever it finds; it exits 2, with `Abstained: <reason>` in the summary, when the logs are not N
  readable, passing shard streams or the table is invalid. Each stream SHALL be well formed and
  finished: every non-blank line is a `go test -json` event with an Action from the closed test2json
  and build-event set, a Package on every test event and an ImportPath on every build event; no
  test, package or build fails; every package outcome follows that package's single start; at
  least one package reaches a terminal outcome, and no package start lacks one. An empty,
  malformed, structurally invalid or unfinished stream abstains. A finding is material when the time it
  misplaces (|observed - recorded| for drift, the observed time for a missing package; a stale
  entry places no package and misplaces nothing) reaches `--share` percent, default 10, of the
  ideal shard (all observed package time divided by N). A material drift is reported even within
  the AFP-V0-022 factor, because shard balance depends on absolute time, not ratio; non-material
  findings keep the AFP-V0-022 factor and floor. Each material finding gets one `::warning::`
  annotation, and one further warning counts every remaining drift, missing and stale finding, so
  no run exceeds the ten warnings GitHub shows per step. The job and each of its steps are
  continue-on-error, so a checkout, toolchain, download, build or report failure abstains and the
  job never ends red. Every abstention SHALL put `Abstained: <reason>` in the job summary; a final
  always-run step writes it when no earlier step did. The 10% default is chosen from run
  38055182050 (ideal shard 1,281s): hosted per-package noise reached about 60s (5%), while
  `internal/tasks/store` misplaced 1,001s (78%) and the missing `internal/appmap` 132s (10.3%).
  Non-goals: the job is not a required check, never posts or changes the `ci-control-plane` or any
  commit status, never refreshes, rewrites or commits the table, and never changes shard
  membership or placement; refreshing remains the operator's AFP-V0-022 `refresh` step. Failure
  modes: a failed, skipped or cancelled shard, or a missing or extra artifact, abstains with its
  reason; an artifact upload or download failure only removes evidence and is continue-on-error;
  re-run attempts overwrite their own shard's artifact. Falsifier: a pull request whose required
  checks fail, or whose shard membership or placement changes, because of this job or the
  retained stream; or a report over fewer than N shard streams. Acceptance evidence: the
  `TestAFPV0040*` tests, `actionlint`, `make ci-least-privilege-check`, and a local dry run of the
  job's steps against the six shard logs of run 38055182050 (build log
  2026-10-10-ci-shard-drift-detection); hosted behaviour is NOT_OBSERVED until the change's own CI
  runs. Rollback: delete the `ci-shard-cost-drift` job and the retention step, restore the plain
  `go test` invocation in `go-product-shard`, and remove `--advisory` from `tools/ci-shard-costs`.

## Non-goals and authority

No provider modification; execution only through the explicitly admitted AFP-V0-013 driver; no watcher or daemon (invariant 7,
`GPK-V0-010`); no exclusion certificates as authority; no non-Go provider package lists; no
LPCV-V0 requirement is implemented or promoted by this slice. The fast tier (AFP-V0-011) is not the
push or release gate and MUST NOT replace `make gate` in any declaration until a 200-commit shadow
run shows no selected-set miss the plan did not mark `UNKNOWN`; a narrowed run proves nothing about
the packages it omitted (AFP-V0-004). The AFP-V0-040 cost-drift report is advisory only: no
required check, commit status, automatic table refresh or commit, and no effect on shard
membership or placement.

## Failure modes

Git unavailable or the worktree status exceeds its bound: fail closed with
`unsupported-affected-status`. No commits: `unsupported-affected-revision`. Source walk exceeds
`affected.MaxWalkEntries`, a plugin returns a non-canonical unit, or the walk accepts a source file
whose path no unit can name: `unsupported-affected-graph`.
An unreadable or invalid `--playwright-config`, or a Playwright graph that cannot be built:
`unsupported-playwright-affected`; dynamic but readable project/config semantics remain a typed
`UNKNOWN` plan with `FULL_RELEVANT_SUITE`, not a command failure.
Exhausting an admitted-directory sub-bound instead skips only that subtree and reports
`go:included-directory-walk-bounded` at `UNKNOWN` scope; it is not a graph refusal.
A dirty path owned by no plugin, or a changed unit (one that owns a changed path) with no
selectable test in any language (AFP-V0-020): the plan widens to `UNKNOWN` scope rather than
narrowing; the packages that name a dirty path by literal are added, never substituted for the
widening (AFP-V0-021). A potentially deleted or renamed-away Go path widens and selects its
package, or the importers of a package that is gone, rather than claiming no dependency path
(AFP-V0-004). An unreadable subtree:
`unsupported-affected-graph`, never a silently smaller graph. Worktree or HEAD changed during
compilation: `unsupported-affected-drift`. A `--base` that is not a full commit id:
`invalid-arguments`; one that is not a commit here: `unsupported-affected-revision`; a range diff
over its bound: `unsupported-affected-status`. In the fast tier every one of these, a plan the
script cannot read, a module-level Go frontier (including an invalid AFP-V0-023 read-scope
declaration or AFP-V0-037 binary-exec declaration), a dirty root module definition, or an empty
selection over a non-empty diff, or a repository the selector cannot index (AFP-V0-012) runs the
full `./...` command instead of a narrowed one, so the
worst case of `make gate-affected` is the cost of `make go-test`, never a skipped package.
The advisory cost-drift report (AFP-V0-040) abstains, with its reason in the job summary, on a
shard that did not succeed, fewer or more outcome artifacts than shards, an unreadable, failed,
empty, malformed, structurally invalid or unfinished package stream, or an invalid cost table; it
never fails CI on a finding, and a failure of any of its own steps, including the tool's build,
abstains with its reason in the job summary rather than failing the job.

## Acceptance evidence and traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| AFP-V0-001 | `cmd/corvint/affected.go` `compileAffected` | `TestAffectedCleanTreeSelectsNothingAndWritesNothing` compares `git status --porcelain --ignored` before and after |
| AFP-V0-002 | `internal/liveverify/affected/dirty.go` | `TestDecodeStatusFailsClosedOnMalformedInput`; `TestDirtyNonUTF8PathIsDisclosedNotRefused`; `TestAffectedRejectsNonRepositoryAndExtraArguments` |
| AFP-V0-003 | `affectedReceipt`, `providerGoPackages` | `TestAffectedDirtyGoSourceSelectsDependentsAsProviderPackages` |
| AFP-V0-004 | `affected.Select` scope and exclusion-reason rules; `goStructure`, `goSourceRule`, `goDataRule`, `WitnessEnclosingPackage` in `internal/liveverify/affected/structure.go`; `Unit.Embeds` and absent in-module import edges (`resolved`, `underModule`) in the Go plugin | `TestAffectedUnownedDirtyPathIsUnknownScope`, `TestDeletedGoSourceSelectsItsPackageAndImporters_V1_0340`, `TestUnownedDirtyPathSelectsItsPackageAndImporters_V1_0340` (an embedded asset, a nested fixture, a file directly in a package and a deleted package each select their package or importers; an unrelated package stays excluded); `frontierUnknowns`, `participants`, `claimantsOf`, `PathReader`, `Unit.Frontier` (V1-0289) with `TestFrontierBearsOnlyOnThePlansItTakesPartIn_V1_0289` (another plugin's change, a clean plan and an unreached unit name no frontier; an unowned path and a path-reading plugin take part) and `TestBuildConstraintIsTheConstrainedPackagesFrontier_V1_0289`; provider wire unchanged (`go-live-test-provider-v0.md` GLTP-V0-006) |
| AFP-V0-005 | canonical JSON via `gokernel.CanonicalJSON`; `graphDigestDomain`, `digestBody`, `digestUnit`, `projectUnit` in `internal/liveverify/affected/graph.go` | byte-identity assertion in the dirty-source test; `TestGraphDigestIsTheDomainTaggedProjection_V1_0299` |
| AFP-V0-006 | `runAffected` error paths; `affected.ErrWalkUnreadable`; `affected.ErrWalkUnrepresentable` | `TestAffectedRejectsNonRepositoryAndExtraArguments`; `TestAffectedUnreadableSubtreeFailsClosed`; `TestSourceFilesRefusesAnUnrepresentableAcceptedName` |
| AFP-V0-007 | `Graph.rank`, `Graph.proximity` in `internal/liveverify/affected/select.go` | `TestSelectOrdersByDistanceThenSharedDirectoryThenUnitID` (order and two-run byte identity) |
| AFP-V0-008 | `SourceFilesIncluding`, `MaxIncludedDirectoryEntries`, `FrontierIncludedDirectoryWalkBounded`, `observeModules`, `workspaceDirectories`, `enclosingModule`, `groupByDirectory`, `inTestdata`, `unitID`, `readModulePath` in `internal/liveverify/affected` | `TestIncludedDirectoryWalkBoundWidensInsteadOfRefusing_AFPV0008`, `TestWorkspaceModulesAreUnitsUnderTheirOwnModulePath`, `TestWorkspaceDirtySourceSelectsTheOtherModulesTest` over `testdata/workspace` (a listed pair, an unlisted `stray`, an entry outside the root), `TestPackagesUnderBuildOutputDirectoryNamesAreSelected`, `TestNoGoRepositoryProducesNoUnitsOrFrontier`, `TestReadModulePathMatchesGoModEdit`, `TestReadModulePathAbstainsOnBOM`, `TestTestdataIsFixtureDataNotAPackage_AFPV0008`, `TestWorkspaceModuleBelowTestdataIsObserved_AFPV0008`, `TestWorkspaceUseOutsideRootIsAFrontier_AFPV0008`, `TestTestOnlyImportSelectsTheTestUserButNotItsImporters`, `TestSourceParsedEdgesCoverEveryEdgeTheToolchainReports` (a non-test toolchain import must be an ordinary edge) |
| AFP-V0-010 | `parseAffectedBase`, `affectedRangePaths`, `affectedRange` in `cmd/corvint/affected.go`; `RangePaths`, `DecodeNameList` in `internal/liveverify/affected/dirty.go` | `TestAffectedBaseRangeJoinsCommittedPathsAndFailsClosed` (committed edit with a clean worktree selects the dependents; `range.base`/`range.paths`; `main` and an unknown id exit 2 with no document), `TestDecodeNameListNormalizesAndFailsClosed`, `TestAffectedReceiptMembersAreClosedAndByteStable` (the closed member set includes `range`) |
| AFP-V0-011 | `gate-affected`, `gate-affected-test`, `GO_TEST_COMMAND` in `Makefile`; `script/gate-affected.sh`; its selection step `selectPackages` in `tools/gate-affected-select/main.go` (native Go, no Python runtime, decision 0088) | `script/gate-affected_test.sh` via `make gate-affected-test` (a shell test over `testdata/fixture` in a scratch repository with a recording go-test command: clean tree runs nothing; a core edit selects core and leaf; a deleted `core/core.go` is a `frontier` line for core, mid, and leaf; a document no package reads beside a core edit is `data`, does not fall back, and adds no package; a dirty `go.mod` falls back; a committed edit under `BASE` selects; an unresolvable base falls back; an interrupted planner exits nonzero without running go test); `TestSelectPackagesAttributesEveryDirtyPath` (a control character in a dirty path falls back) in `tools/gate-affected-select/main_test.go`; `TestSelectPackagesRejectsSiblingModulePrefix` (a package path that only shares the module's characters as a string prefix, with no `/` boundary, falls back instead of being selected); the 50-commit replay in AFP-V0-011 |
| AFP-V0-012 | `indexRepository`, `scanSource`, `escapesPackage`, `dependents`, `readers`, `enclosing`, `unresolved`, `namesPath` in `tools/gate-affected-select/readers.go`; the per-path loop in `selectPackages` (decision 0131) | `TestSelectPackagesAttributesEveryDirtyPath` in `tools/gate-affected-select/main_test.go` (a deleted source widens to its importers; a Go file read as data selects its reader; a document selects the package that names it; a testdata fixture selects its enclosing package; a `runtime.Caller` package is selected on every dirty path; a nested module's literals select nothing); `TestSelectPackagesFallsBackWhenAttributionFails` (imports that do not parse fall back); `TestSelectPackagesReachesEmbeddingAncestorDependents` (a data path under an embedding ancestor reaches that ancestor's dependents); `TestSelectPackagesResolvesAliasedAndDotRootLocatorImports` (an aliased or dot-imported `runtime.Caller` still marks the package unresolved); `TestIndexRepositoryFailsClosedOnSymlinkedGoFile` (a symlinked `.go` file falls back instead of being silently skipped) |
| AFP-V0-022 | `.github/cishards`, `tools/corvint-pr-tests`, `tools/ci-shard-costs`, CI and qualification workflows | `TestAFPMixedAdmissionPreservesSelectedUnion`, `TestAFPCompleteBalancedPartition`, `TestAFPIsolatedBuildIgnoresModuleRedirection`, `TestShardedPRExecution_AFPV0022`, `TestAFPV0022RefreshFromHostedOutcomes`, `TestAFPV0022RefreshRefusesUnusableInput`, `TestAFPV0022DriftCheck`, `TestAFPV0022OrderRunsSelectedUnitsFirstWithoutChangingTheSet`, `TestAFPV0022OrderFallsBackToTheCurrentOrder`; hosted timing and narrowing qualification NOT_RUN |
| AFP-V0-023 | `ReadScopesPath`, `InReadScope`, `ValidReadScopeEntry`, `Graph.scopedReadersOf`, `WitnessDeclaredReadScope`, `Unit.ReadScoped`, `Unit.ReadScope` in `internal/liveverify/affected`; `readScopes`, `applyReadScopes`, `FrontierReadScopesInvalid` in `internal/liveverify/affected/golang/readscopes.go`; `readScopes`, `inReadScope` in `tools/gate-affected-select/readscopes.go`; `.github/testconfine` (`Load`, `Rules`, `ExecConfined`, `ConfinedEnv`, `cmd`) and the full-run `-exec` in `.github/workflows/ci.yml` | `TestDeclaredReadScopeNarrowsAnUnboundedReader_AFPV0023`, `TestInvalidReadScopeDeclarationDeclaresNothing_AFPV0023`, `TestGraphDigestIsTheDomainTaggedProjection_V1_0299`, `TestSelectPackagesHonorsDeclaredReadScopes`, `TestLoadAcceptsOnlyTheExactGrammar_AFPV0023`, `TestRulesGrantOutsideRootPackageAndEntriesOnly_AFPV0023`, `TestSelectionOnTheLiveDirtyWorktree`, `TestConfinedEnvTurnsOffVCSStamping_AFPV0023`, `TestExecConfinedDeniesUndeclaredRepositoryReads_AFPV0023` (Linux; run on a kernel reporting Landlock ABI 4, including granted and refused reparenting); per-package declarations verified under the wrapper in a Linux container (build log 2026-10-01-declared-test-read-scopes); hosted confined full run NOT_RUN until the protected workflow change is admitted |
| AFP-V0-013 | `tools/corvint-pr-tests` and `.github/workflows/ci.yml` | `TestSelectedFailureAndFallback`, `TestInterruptionLeavesNoLiveDescendant`; trusted pins empty, hosted execution unavailable |
| AFP-V0-015 | `tools/corvint-pr-tests/container.go` and indexed shadow execution | `TestContainerProfileAndArchive`, `TestColdRuntime`, `TestFrozenRowIndex`, `TestDockerCLIInterruption`, `TestContainerCleanupRefusal`; real Linux row/hosted NOT_RUN |
| AFP-V0-024 | `tools/ci-reuse-plan`; `docs-plan` and `go-product-shard` in `.github/workflows/ci.yml` | `TestAFPV0024ReusesOnlyAnExactTreeRecordedByEveryShard`, `TestAFPV0024AnythingElseRunsInFull`; local replay of the push step against the live API returned FULL; hosted reuse `NOT_OBSERVED` |
| AFP-V0-025 | `tools/unbounded-readers`, `.corvint/unbounded-readers.json`, `make unbounded-readers-check`; `Graph.UnboundedReaders`; `ShareOf` in `.github/cishards/order.go`; `doc-gates` and `go-product-shard` in `.github/workflows/ci.yml` | `TestAFPV0025RatchetFailsOffTheRecordedSet`, `TestAFPV0025ConcurrentAdditionsMergeToAPassingRecord`, `TestAFPV0025RatchetWithoutARecordRefuses`, `TestAFPV0025ShareReportsSelectedEstimatedTime`; hosted share report `NOT_OBSERVED` until this change's own CI run |
| AFP-V0-026 | `merge_group` trigger in `.github/workflows/ci.yml`; `merge-group` job in `.github/workflows/ci-control-plane.yml` | `actionlint`; `make ci-least-privilege-check`; hosted merge-queue run `NOT_OBSERVED` until the owner enables the queue |
| AFP-V0-027 | `go-product-shard` and `go-static` job conditions and `go-product` message in `.github/workflows/ci.yml` | `actionlint`; `make ci-least-privilege-check`; hosted constituent run with the label `NOT_OBSERVED` until the label exists and a batch uses it |
| AFP-V0-031 | `nestedModules`, `observedReach`, `unknownDirectory`, `nestedModuleOpen`, `readManifest`, `workspaceAbove`, `replaceDirectoryOpen`, `repositoryDirectory`, `parseManifest`, `manifestLine`, `maxNestedManifestBytes`, `observeModules` in `internal/liveverify/affected/golang` | `TestIndependentNestedModulesCloseTheFrontier_V1_0867`, `TestNestedModuleThatCanReachTheRootKeepsTheFrontier_V1_0867`, `TestUnreadableOrUnparsableNestedManifestKeepsTheFrontier_V1_0867`, `TestNestedFrontierReadsTheSuppliedSource_V1_0867`; `TestWorkspaceModulesAreUnitsUnderTheirOwnModulePath` and `TestWorkspaceDirtySourceSelectsTheOtherModulesTest` over `testdata/workspace/stray`, which now requires a listed module; survey replay in `docs/build-log/2026-10-06-nested-module-frontier.md` (unchanged on this repository, because `tools/local-authority` requires the root) |
| AFP-V0-032 | `readAdviceSource`, `mandatoryAffectedChecks` in `cmd/corvint/affected.go` | `TestAffectedAdviceUnreadableDeclarationSuppressesNoGate` (`cmd/corvint/affected_advice_unix_test.go`) |
| AFP-V0-033 | `unreasoned` in `tools/unbounded-readers`; the `reasons` of `.corvint/unbounded-readers.json` | `TestAFPV0033EveryUnitNamesItsUnboundedRead` (a live or stale unit without a reason fails and is named; both failures are reported together); `TestAFPV0025ConcurrentAdditionsMergeToAPassingRecord` (each addition carries its reason); `make unbounded-readers-check` passes with a reason for every unit; per-package container measurements in build log 2026-10-06-unbounded-reader-reasons |
| AFP-V0-014 | `tools/corvint-pr-tests/shadow.go` | `TestQualificationAndTerminalFailures`, `TestToolIdentityRequiresCurrentGoVersion`; frozen 200-row qualification NOT_RUN |
| AFP-V0-016 | `.github/workflows/ci-control-plane.yml`; the `main` repository ruleset | `actionlint`; `success` posted on PR #26 (run 35444060752) and PR #24 (run 35446378936); ruleset 23699808 active with the decision 0320 settings; the decision 0390 settings (no bypass, `doc-gates` required) and the admin-status consent path NOT_VERIFIED until the owner applies them; `failure` path NOT_RUN on a real PR |
| AFP-V0-017 | `.github/workflows/pr-tests-qualification.yml` | `actionlint`; dispatch NOT_RUN (`main` has fewer than 201 first-parent commits) |
| AFP-V0-019 | `internal/plansnapshot`, `compileSnapshotAffected` | `TestSnapshotImmutableBytesAndCleanup`, `TestSnapshotRejectsIncompleteMismatchedAndStale`, `TestSnapshotStrictWire`, `TestSnapshotRejectsLinksAndIgnoresArchiveAttributes`, `TestAffectedSnapshotMatchesCommittedPlanAcrossDirtySources`, `TestAffectedSnapshotPlaywrightPinsConfigAndSource` |
| AFP-V0-018 | `playwrightAffectedReceipt`, `compilePlaywrightAffected`, and `typescript.SelectPlaywright` | `TestAffectedPlaywrightProfileEmitsProjectDistinctUnits`, `TestAffectedPlaywrightArgumentsFailClosed`, and `internal/liveverify/affected/typescript/playwright_test.go` |
| AFP-V0-009 | `affectedAdvice`, `compileAffectedAdvice`, `mandatoryAffectedChecks`, `advisoryAffectedChecks`, `agentsVerifyCommands`, `agentsCheck`, `nonTerminatingCommand`, `stripShellComment`, `shellQuoteJoin` in `cmd/corvint/affected.go` | `TestAffectedAdviceJoinsMandatoryGateAndAdvisoryPackages`, `TestAffectedAdviceReportsNoDeclaredGate`, `TestAffectedAdviceKeepsMandatoryGateAndNeverAdvisesExclusions`, `TestAffectedReceiptMembersAreClosedAndByteStable` (tightened to assert `advice`'s raw JSON key order), `TestAffectedAdviceBoundsTheDeclarationRead`, `TestShellQuoteJoinEscapesMetacharacters`, `TestAffectedAdviceTruncatedMandatoryDeclarationSuppressesNoGate`, `TestAffectedAdviceCapsMandatoryChecksAtSixteen`, `TestAffectedAdviceSkipsCommentsInVerifyFence`, `TestAffectedAdviceTakesOnlyTheExactVerifyHeading_V1_0342` |
| AFP-V0-021 | `WitnessPathLiteralReader`, `PathTokenBound`, `Graph.readers`, `Graph.tokenBounds`, `namesPath`, `ChangeEvidencePath`, `Graph.resolves`, `resolvesWithin`, `WitnessUnboundedReader`, `Graph.unboundedReadersOf` in `internal/liveverify/affected` (`select.go`, `readers.go`, `graph.go`); `Unit.PathTokens`, `Unit.PathTokensBounded`, `Unit.UnboundedReads`, `Unit.LocatesRoot`; `pathTokens`, `importsEnd`, `ignoredByGo`, `maxPathTokens` in `internal/liveverify/affected/golang/golang.go`; `escapesPackage`, `rootLocatorCall` in `internal/liveverify/affected/golang/unbounded.go` | `TestPathLiteralSelectsItsReaderPackage_AFPV0021` (a named document selects its reader and stays unknown; single and parenthesized imports are no tokens; a file without imports yields tokens; a dependent and an unnamed path select nothing), `TestOwnedDirtyPathSelectsTheUnitsThatNameIt`, `TestReaderWitnessIsTheSmallestNamingDirtyPath`, `TestReaderReachedByDependencyKeepsItsDependencyWitness`, `TestBoundedPathTokensAreUnknownOnlyWhenAMatchIsAttempted`, `TestPathTokenBoundNamesThePackage`, `TestUnlexableSourceIsAFrontierOutsideIgnoredDirectories`, `TestSelectionOnTheLiveDirtyWorktree` (reader witnesses resolve), `TestAffectedDocumentSelectsThePackageThatNamesIt` (receipt shape, provider packages, byte identity), `TestDirectoryShapedLiteralNamesNoPath` (V1-0290: a directory-shaped one-component token names no path; two-component and file-name tokens still select), `TestChangeEvidenceReadersAreNarrowed_V1_0230` (the sidecar keeps only resolving readers; a climbing token names a directory; a same-shaped path is not narrowed), `TestUnboundedReaderIsSelectedOnAnyChange_V1_0230` (rule (d): root locators through plain, aliased and dot imports, a climbing literal and a test-only `--show-toplevel` are selected with their non-test locator's dependents, not the test-only one's; a clean plan selects none) |
| AFP-V0-020 | `UnknownNoSelectableTest` in `affected.Select` (`internal/liveverify/affected/select.go`) | `TestSelectNamesChangedUntestedGoPackageAsUnknownScope`, `TestSelectTraversesUntestedUnitsWithoutSelectingThem` (an untested unit the change only reaches stays bounded), `TestSeamWidensWhenNoTestReachesAChangedUnit_AFPV0020` (every plugin), `TestPlaywrightDiscoveryReconciliation` (an unreached helper keeps the Playwright plan), `TestAffectedUntestedGoPackageIsUnknownScope` |
| AFP-V0-034 | `readSourceFile` (`internal/liveverify/affected/read_unix.go`, `read_other.go`), `ReadSource` (`walk.go`) | `TestReadSourceRefusesNonRegularFilesOnOpenDescriptor` (regular file read; symlink, directory, FIFO, unix socket and mode-0 directory refused as `ErrInvalidUnit` without blocking; a sparse body over `MaxSourceBytes` refused as `ErrWalkLimit`; a missing path reports `fs.ErrNotExist`) |
| AFP-V0-035 | `compactAffectedReceipt`, `summarizeAffectedExclusions` in `cmd/corvint/affected_compact.go`; `--full` in `parseAffectedOptions`; profile checks in `internal/companionrelease/core_smoke.go`, `.github/cishards/order.go`, `tools/corvint-pr-tests/main.go`; `--full` in `tools/retrieval-bench/main.go` | `TestAFPV0035CompactDefaultPlanSummarizesTheFullPlan`; the `--snapshot` default/`--full` parity in `TestAffectedSnapshotMatchesCommittedPlanAcrossDirtySources`; `TestAFPV0035OrderReadsTheCompactDefaultPlan`; core-freeze goldens `affected-*.json` and `affected-full-*.json` |
| AFP-V0-036 | `testOnlyGoUnits`, `reach`, `seed` in `internal/liveverify/affected/select.go` | `TestGoTestOnlyChangeSelectsItsPackageButNotItsImporters_V1_0984` in `internal/liveverify/affected/affected_test.go` (a test-only change selects its package alone plus the unbounded reader; a source change, and a source-and-test change witnessed by the source path, reach the importer and the test user; a test change beside a dependent's source change does not reach the test user; a non-Go plugin's test-only change still traverses) |
| AFP-V0-037 | `Unit.Execs`, `Graph.execUsers`, `Graph.builtCommands`, `Graph.execUsersOf`, `WitnessBinaryExec`, `BinaryExecsPath` in `internal/liveverify/affected` (`unit.go`, `graph.go`, `execs.go`, `select.go`); `applyBinaryExecs`, `literalExecs`, `commandDirectory`, `readBinaryExecs`, `matchBinaryExecs`, `FrontierBinaryExecsInvalid` in `internal/liveverify/affected/golang/binaryexecs.go`; `moduleLevelFrontiers` in `tools/gate-affected-select/main.go`; `.corvint/test-binary-execs.json` | `TestBinaryExecConsumerIsSelectedWithTheCommandsBuild_AFPV0037` (anchored, plain, climbing and module-path literals and a declared runner become `execs`; a change to the command or a package it imports selects every consumer as `BINARY_EXEC` through the command; a literal naming a file under the command directory is no edge; an unrelated change selects no consumer; byte-identical plans), `TestInvalidBinaryExecDeclarationDeclaresNothing_AFPV0037` (nine invalid declarations raise only the frontier, drop declared edges and keep literal ones; a missing one raises nothing) |
| AFP-V0-038 | `compactAffectedAdvice`, `affectedCompactCheck`, `adviceAdvisoryGoTest` in `cmd/corvint/affected_compact.go` and `cmd/corvint/affected.go` | `TestAFPV0038CompactAdviceReferencesProviderPackages` (fixture default vs `--full`, a quoted package path, a non-matching command kept whole); advice resolution in `TestAFPV0035CompactDefaultPlanSummarizesTheFullPlan` and `TestAffectedAdviceJoinsMandatoryGateAndAdvisoryPackages`; core-freeze golden `affected-committed-range.json` |
| AFP-V0-039 | command-local `-c maintenance.auto=false -c gc.auto=0` in the Git helpers of `internal/liveverify/affected/observation_test.go`, `internal/liveverify/affected/golang/golang_test.go`, `internal/liveverify/affected/typescript/mocha_qualification_test.go` and `internal/liveverify/pymutate/pymutate_test.go`; `unguardedFixture` in `internal/liveverify/affected/fixture_maintenance_test.go` | `TestLiveVerifyGitFixturesDisableDetachedMaintenance` (fails on the three unfixed helpers and on the pre-c4f9604d observation helper), `TestUnguardedFixtureDetectsAMissingSafeguard`; `GIT_TRACE2_EVENT` child-launch counts in build log 2026-10-08-liveverify-fixture-maintenance; hosted Linux Git 2.55 cleanup NOT_RUN |
| AFP-V0-040 | `check --advisory` (`report`, `observeStream`, `scan`, `checkEvent`, `findings`, `finding.misplaced`, `escape`) in `tools/ci-shard-costs`; the best-effort capture in the tests step and the shard-outcome retention step of `go-product-shard` and the `ci-shard-cost-drift` job in `.github/workflows/ci.yml` | `TestAFPV0040AdvisoryReportNeverFailsOnFindings`, `TestAFPV0040AdvisoryWarningsStayWithinTheStepLimit`, `TestAFPV0040AdvisoryAcceptsAFullEventStream`, `TestAFPV0040AdvisoryAbstainsOnPartialOrUnusableInput`; `actionlint`; `make ci-least-privilege-check`; local dry run of the job steps against run 38055182050 (build log 2026-10-10-ci-shard-drift-detection); hosted run `NOT_OBSERVED` |

Compatibility and drift: the provider bundle grammar is consumed, not redefined; if
`go-live-test-provider-v0.md` changes its pattern grammar or bound, `providerMaxPackagePatterns`
and this spec change in the same commit. Unresolved decisions: whether a future composer consumes
this wire directly or the provider bundle grows a signed selection field; which trigger (editor
task, pre-commit hook) invokes the command; whether non-Go projections are wanted.

Rollout: the command ships in the experimental Go candidate; no adapter calls it. Rollback: delete
`cmd/corvint/affected.go`, its test, the help entries, this spec, and its README/INDEX rows; for
the fast tier alone, delete `script/gate-affected.sh`, `script/gate-affected_test.sh`, `tools/gate-affected-select`, the
`gate-affected` and `gate-affected-test` targets, and inline `GO_TEST_COMMAND` back into `go-test`,
which leaves `make gate` byte-for-byte as it was. Also delete `RangePaths` and `DecodeNameList` in
`internal/liveverify/affected/dirty.go` when the range form goes.
Promotion or kill: promote only after the LPCV-V0 composer accepts this wire or replaces it; kill
if a shadow run over 200 historical commits shows any selected-set miss against the full suite that
the plan did not mark `UNKNOWN`. The fast tier may become a push gate only after that same shadow
run passes; until then it is an everyday narrowing whose fallback is the full run.

Protected workflow/ruleset status: **VERIFIED** (2026-09-19). The repository workflow and literal
pins alone do not protect their own control plane; decision 0320 establishes the admission on the
free plan with AFP-V0-016 and ruleset 23699808, whose settings and observed runs are recorded in
`tools/corvint-pr-tests/README.md`. Decision 0390 removes the ruleset's bypass actor and adds
`doc-gates`; that ruleset change is not yet observed. Keep pins empty until AFP-V0-017 produces a PASS.
The runtime environment is an allowlist with exact recorded bytes, a fixed absolute Go PATH,
`/usr/bin/cc`, `GOENV=off`, `LANG=C`, `LC_ALL=C`, `TZ=UTC`, and exclusively owned HOME/TMP/cache
under `/tmp/corvint-pr-tests-runtime`. An existing runtime path is refused; owned runtime state is
removed on return; cleanup failure is reported with a nonzero exit. Output/runtime paths are
resolved through their nearest existing ancestors before creation and rechecked as real external
directories, so a symlinked parent cannot create inside the tested tree. Historical rows use that same fixed layout and reset HOME/TMP/GOCACHE immediately before each test invocation.
Stdout is capped at 128 MiB and stderr at 8 MiB; overflow immediately cancels the process group
and invalidates execution. Hosted log preview is capped at 1 MiB/256 KiB. Planner/selector drift
at launch falls back to full; Go/compiler/Git/environment drift fails safely. The frozen identity
records the test Git executable path, digest and version, because tests run Git and its
version-specific output changes their outcomes. The driver's own Git reads and the `go test`
process both keep `GIT_GRAFT_FILE=/dev/null`, so tests inherit the graft refusal in every mode. The `go test`
environment differs only by setting `advice.graftFileDeprecated=false` through `GIT_CONFIG_COUNT`,
since naming a graft file makes current Git print deprecation advice into the combined output tests
read (build log 2026-10-01-ci-shadow-graft-advice). Freeze ignores grafts;
row reuse binds the execution source/environment and raw plan/audit hashes as well as outcomes.

Container control operations have a two-minute deadline within an 80-minute launcher ceiling;
the test operation keeps its 75-minute bound. Docker daemon logging is disabled and inspected,
so PID1 file descriptors cannot bypass the retained-output bound. Both historical and PR
container checkouts use `/work/checkout`; process capture cannot bypass limits via io.Copy.
Run export requires only its actual selection/execution/Go outputs, not historical row files.

## Bounded documentation CI exception (2026-09-29)

The owner requested implementing a qualified README presentation path after PR #348. The separate
[Documentation CI V0](documentation-ci-v0.md) contract defines the only documentation-specific
exception to AFP-V0-013/014's root race invocation. It requires trusted-base source, exact merge
binding, a closed document scope, independently reviewed source-bound consumer qualification and
retained documentation checks. All other mandatory checks and AFP-V0-016 control-plane acceptance
remain. It does not alter this planner's UNKNOWN scope or enable general Go selection; that driver
still needs the full AFP-V0-014 historical campaign. Missing or stale DCI qualification runs FULL.
