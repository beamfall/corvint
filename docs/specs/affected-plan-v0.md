# Affected Plan V0

Owner: Russell Lewis
Date: 2026-09-01
Requirement prefix: `AFP-V0`
Intent status: accepted for AFP-V0-008 (decision 0057) and AFP-V0-009 (decision 0052); AFP-V0-013/014/015 accepted (decision 0289); AFP-V0-016/017 accepted (decision 0320), AFP-V0-016 amended (decision 0390); AFP-V0-021 accepted (decision 0376); AFP-V0-009 and AFP-V0-021 amended (decision 0424); AFP-V0-009 amended by the owner (decision 0531; V1-0682); AFP-V0-023 owner-directed, proposed (2026-10-01, V1-0246); AFP-V0-031/032/033 accepted (decision 0438); AFP-V0-034 accepted (decision 0439); AFP-V0-035 proposed (V1-0943; no GitHub issue); AFP-V0-036 proposed (V1-0984; no GitHub issue); AFP-V0-037 proposed (V1-0991; no GitHub issue); AFP-V0-038 proposed (V1-0995; no GitHub issue); AFP-V0-039 proposed (V1-0662; no GitHub issue); AFP-V0-040 accepted (decision 0487); AFP-V0-041 accepted (decision 0488), amended (decision 0500); AFP-V0-042 accepted (decision 0496); AFP-V0-028/029/030 accepted (decision 0431), amendments accepted (decision 0509); AFP-V0-013/014/016/017/022/026 amended (decision 0431, accepted decision 0509); other AFP-V0 requirements proposed; AFP-V0-043 accepted, decision 0505; AFP-V0-044/045 accepted (decision 0519; V1-1141; GitHub #783); AFP-V0-047 accepted under the owner's standing auto-accept (2026-10-11; V1-0811); AFP-V0-048 accepted, and AFP-V0-041 (6) and AFP-V0-047 amended to it, under the owner's standing auto-accept (2026-10-11; V1-1361); AFP-V0-049 accepted under the owner's standing auto-accept (2026-10-11; V1-1371); AFP-V0-050 accepted under the owner's standing auto-accept (2026-10-11; V1-1298); AFP-V0-023 and AFP-V0-050 amended by the owner (decision 0528; V1-1399, V1-1400), and (3) reworded by the owner (decision 0529; V1-1425); AFP-V0-051 accepted under the owner's standing auto-accept (2026-10-11; V1-1351); AFP-V0-053 accepted by the owner (2026-10-16; ticket ci-shadow-historical-compat); AFP-V0-029/030 amended by the owner for the `INFRA` row class (decision 0537; V1-1514); AFP-V0-029 shadow bar amended by the owner to count the whole epoch (owner chat, 2026-10-20; V1-0812); AFP-V0-052 accepted under the owner's standing auto-accept (2026-10-11; V1-1514); AFP-V0-054 accepted under the owner's standing auto-accept (2026-10-11; V1-1719); AFP-V0-066 accepted under the owner's standing auto-accept (2026-10-11; V1-1796; GitHub #1130); AFP-V0-072 accepted under the owner's standing auto-accept (2026-10-11; V1-1806); AFP-V0-079/080/081/082 accepted under the owner's standing auto-accept (2026-10-11; V1-1778/V1-1642); AFP-V0-060 accepted by the owner (owner chat, 2026-10-18; V1-1096); AFP-V0-088 accepted under the owner's standing auto-accept (2026-10-11; V1-2049); AFP-V0-016/024/027/028 amended by them; AFP-V0-150/151/152/153/154/155 accepted under the owner's standing auto-accept (2026-10-11; V1-1087, V1-0875, V1-1820, V1-2070, V1-0996, V1-1005); AFP-V0-029/030 amended and AFP-V0-097/098/099 decided by the orchestrator under owner delegation (owner chat, 2026-10-20; V1-0812); AFP-V0-316 accepted under the owner's standing auto-accept (2026-10-11; V1-2169); AFP-V0-160/161 accepted, and AFP-V0-023 amended for rule (c) by AFP-V0-161, under the owner's standing auto-accept (2026-10-11; V1-0923, V1-1766/V1-2069); AFP-V0-356 accepted under the owner's standing auto-accept (2026-10-11; V1-2053); AFP-V0-156 accepted under the owner's standing auto-accept (2026-10-11; V1-2162); AFP-V0-162 accepted under the owner's standing auto-accept (2026-10-11; V1-2256, V1-2252); AFP-V0-181 to AFP-V0-186 accepted under the owner's standing auto-accept (2026-10-11; V1-2050); AFP-V0-187 to AFP-V0-191 accepted, and AFP-V0-008, AFP-V0-031 and AFP-V0-182 amended by them, under the owner's standing auto-accept (2026-10-11; V1-2297, V1-2296, V1-0874, V1-0922); AFP-V0-163/164 accepted under the owner's standing auto-accept (2026-10-11; V1-2113, V1-1752); AFP-V0-192 accepted, and rule (d) of AFP-V0-012 amended by it, under the owner's standing auto-accept (2026-10-11; V1-2075)
Delivery status: experimental
Authoritative inputs: `docs/specs/go-live-test-provider-v0.md` (provider plan wire and non-goals),
`docs/specs/live-proof-carrying-verification-v0.md` (future composer, not-started),
`internal/liveverify/affected` (selector), `AGENTS.md` invariants 2, 4, and 8.

## Agent digest
- Claim: `corvint affected` emits a read-only, non-authoritative affected-test selection plan with provider-ready Go package paths.
- Status: accepted for AFP-V0-008 (decision 0057) and AFP-V0-009 (decision 0052); AFP-V0-013/014/015 accepted (decision 0289); AFP-V0-016/017 accepted (decision 0320), AFP-V0-016 amended (decision 0390); AFP-V0-021 accepted (decision 0376); AFP-V0-009 and AFP-V0-021 amended (decision 0424); AFP-V0-009 amended by the owner (decision 0531; V1-0682); AFP-V0-023 owner-directed, proposed (2026-10-01, V1-0246); AFP-V0-031/032/033 accepted (decision 0438); AFP-V0-034 accepted (decision 0439); AFP-V0-035 proposed (V1-0943; no GitHub issue); AFP-V0-036 proposed (V1-0984; no GitHub issue); AFP-V0-037 proposed (V1-0991; no GitHub issue); AFP-V0-038 proposed (V1-0995; no GitHub issue); AFP-V0-039 proposed (V1-0662; no GitHub issue); AFP-V0-040 accepted (decision 0487); AFP-V0-041 accepted (decision 0488), amended (decision 0500); AFP-V0-042 accepted (decision 0496); AFP-V0-028/029/030 accepted (decision 0431), amendments accepted (decision 0509); AFP-V0-013/014/016/017/022/026 amended (decision 0431, accepted decision 0509); other AFP-V0 requirements proposed; AFP-V0-043 accepted, decision 0505; AFP-V0-044/045 accepted (decision 0519; V1-1141; GitHub #783); AFP-V0-047 accepted under the owner's standing auto-accept (2026-10-11; V1-0811); AFP-V0-048 accepted, and AFP-V0-041 (6) and AFP-V0-047 amended to it, under the owner's standing auto-accept (2026-10-11; V1-1361); AFP-V0-049 accepted under the owner's standing auto-accept (2026-10-11; V1-1371); AFP-V0-050 accepted under the owner's standing auto-accept (2026-10-11; V1-1298); AFP-V0-023 and AFP-V0-050 amended by the owner (decision 0528; V1-1399, V1-1400), and (3) reworded by the owner (decision 0529; V1-1425); AFP-V0-051 accepted under the owner's standing auto-accept (2026-10-11; V1-1351); AFP-V0-053 accepted by the owner (2026-10-16; ticket ci-shadow-historical-compat); AFP-V0-029/030 amended by the owner for the `INFRA` row class (decision 0537; V1-1514); AFP-V0-029 shadow bar amended by the owner to count the whole epoch (owner chat, 2026-10-20; V1-0812); AFP-V0-052 accepted under the owner's standing auto-accept (2026-10-11; V1-1514); AFP-V0-054 accepted under the owner's standing auto-accept (2026-10-11; V1-1719); AFP-V0-066 accepted under the owner's standing auto-accept (2026-10-11; V1-1796; GitHub #1130); AFP-V0-072 accepted under the owner's standing auto-accept (2026-10-11; V1-1806); AFP-V0-079/080/081/082 accepted under the owner's standing auto-accept (2026-10-11; V1-1778/V1-1642); AFP-V0-060 accepted by the owner (owner chat, 2026-10-18; V1-1096); AFP-V0-088 accepted under the owner's standing auto-accept (2026-10-11; V1-2049); AFP-V0-016/024/027/028 amended by them; AFP-V0-150/151/152/153/154/155 accepted under the owner's standing auto-accept (2026-10-11; V1-1087, V1-0875, V1-1820, V1-2070, V1-0996, V1-1005); AFP-V0-029/030 amended and AFP-V0-097/098/099 decided by the orchestrator under owner delegation (owner chat, 2026-10-20; V1-0812); AFP-V0-316 accepted under the owner's standing auto-accept (2026-10-11; V1-2169); AFP-V0-160/161 accepted, and AFP-V0-023 amended for rule (c) by AFP-V0-161, under the owner's standing auto-accept (2026-10-11; V1-0923, V1-1766/V1-2069); AFP-V0-356 accepted under the owner's standing auto-accept (2026-10-11; V1-2053); AFP-V0-156 accepted under the owner's standing auto-accept (2026-10-11; V1-2162); AFP-V0-162 accepted under the owner's standing auto-accept (2026-10-11; V1-2256, V1-2252); AFP-V0-181 to AFP-V0-186 accepted under the owner's standing auto-accept (2026-10-11; V1-2050); AFP-V0-187 to AFP-V0-191 accepted, and AFP-V0-008, AFP-V0-031 and AFP-V0-182 amended by them, under the owner's standing auto-accept (2026-10-11; V1-2297, V1-2296, V1-0874, V1-0922); AFP-V0-163/164 accepted under the owner's standing auto-accept (2026-10-11; V1-2113, V1-1752); AFP-V0-192 accepted, and rule (d) of AFP-V0-012 amended by it, under the owner's standing auto-accept (2026-10-11; V1-2075)/experimental
- Exists: `internal/liveverify/affected`, `corvint affected`, `cmd/corvint/affected_test.go`, the `advice` member (AFP-V0-009: repository-declared mandatory checks, conditional when the AGENTS.md Verify section scopes them, one advisory Go command, the unknown frontier), the `--base FULL_COMMIT_ID` range form and `range` member (AFP-V0-010), and the `make gate-affected` fast tier over the receipt (AFP-V0-011: `script/gate-affected.sh`, fail-closed to the full `go-test` run; not the push gate), whose union is attributed per dirty path from a static repository index of imports and path literals (AFP-V0-012), whose literal-reader rule also adds, in the plan itself, selections for every dirty path a package names, without narrowing an unowned path's `UNKNOWN` scope (AFP-V0-021); `tools/corvint-pr-tests` and `.github/workflows/ci.yml` remain full until separately pinned AFP-V0-014 qualification; AFP-V0-022 adds complete advisory CI partitions and a digest-bound experimental sharded PR profile, and AFP-V0-041 (accepted, decision 0488) lets the full run split an owner-allowed package into test slices across those shards, and AFP-V0-048 makes the sharded PR driver run each slice on the shard the same helper assigns; AFP-V0-023 lets a project-owned `.corvint/test-read-scopes.json` take a root-locating package off the rule (d) floor, enforced in full CI by the Landlock wrapper `.github/testconfine`.
- Blocked on: the LPCV-V0 composer accepting or replacing this wire; for queue-backed narrowing, AFP-V0-028/029 shadow admission, the owner setting the repository variable `CORVINT_PR_NARROWING` to `on`, a `CORVINT_PR_TOOL_SOURCE` pin carrying `narrow.sh` and the driver's plan mode, a scheduled `ci-forward-bar` verdict saying MET (AFP-V0-097 to AFP-V0-099; `docs/build-log/2026-10-20-v1-0812-narrowing-decisions.md`), reviewed tool pins and hosted observation of the V1-0760 queue-run reuse (AFP-V0-024; hosted enablement `NOT_OBSERVED`); the optional historical profile still requires genuine 200-row qualification and matching reviewed pins (AFP-V0-014/017), and its 201-commit prerequisite is met at `adf8358220769b8d6724ad27d27625602b8a7c62`, but no campaign PASS is implied.
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
  plugin takes part once a dirty path is owned by none. No plugin takes part when no path is dirty
  (amended for V1-2126 under the owner's standing auto-accept, 2026-10-11: an empty dirty set
  changes nothing a frontier could hide an edge from, so a clean worktree's plan is `BOUNDED`). A reason that bears only on one unit is that unit's sorted `frontier` member, covered
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
  read by the same strict manifest reader as AFP-V0-031, so a `go.work` that reader refuses is the
  `go:workspace-unparsed` frontier over the root module alone, AFP-V0-190; an entry outside the root is skipped and raises the
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
  `--provider` is given (ETS-V0-002, `docs/specs/external-test-selection-v0.md`), and `app_navigation` only when `--appmap-impact` is given (ANL-V0-031; this amendment accepted, decision 0519 (V1-1167)). Each `checks` entry has exactly
  `command`, `kind` (`mandatory`, `conditional` (amended, decision 0531) or `advisory`), `reason` (one sentence), and `source` (a repository
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
  `affected-plan`, and is otherwise replaced by one `unknown` line naming the state, except that an
  `EMPTY_SELECTION` over a graph with no `go:` unit adds no line (amended for V1-2076: a repository
  without Go units has no Go advice gap), and an `EMPTY_SELECTION` over an empty `plan.dirty` adds
  no line either (amended for V1-2126 under the owner's standing auto-accept, 2026-10-11: there is
  nothing to run, and the mandatory checks still stand). `checks` MUST
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
  (amended by the owner 2026-10-15, decision 0531; amends the decision 0052 and 0424 text; V1-0682)
  A project instruction can scope a declared command. A scoping sentence is a prose sentence
  outside ```` ``` ```` and `~~~` fences under the exact `Verify` heading of `AGENTS.md` that
  contains, as whole words in any case, "unless", "only when" or "only if" (every non-letter counts
  as a word break). A paragraph's lines are joined and split into sentences at a `.`, `!` or `?`
  followed by a space or the paragraph's end; a blank line, heading, fence or list item (`-`, `*`,
  `+`, or digits then `.` or `)`, then a space) ends a paragraph, and a fence closes only on its own
  character repeated at least as often. A `mandatory` entry, from the
  `Makefile` or the Verify fence, whose exact command appears in backticks in a scoping sentence
  MUST instead be `conditional`, with reason `the repository <source> declares this command, but
  its AGENTS.md Verify section scopes it, so run it only when this condition holds: <sentence>`,
  the first such sentence verbatim. It keeps its source-order place and counts toward the 16-entry
  cap. A `conditional` entry is a declared gate, so it also suppresses
  `NO_REPOSITORY_GATE_DECLARED`; it is not a `mandatory` check for ETS-V0-009 or `corvint test`.
  A command no scoping sentence names stays `mandatory`, so an unscoped project, a sentence
  outside the Verify section or a sentence without a condition word changes nothing. The condition
  is named, never evaluated: Corvint does not decide whether it holds. Non-goals: prose that
  scopes commands without naming them (for example "the exhaustive commands below"), and
  conditions stated in other files. Rollback removes the `conditional` kind and
  `scopeConditionalChecks`/`agentsVerifyScopes`/`markdownListItem`/`markdownFence`, restoring every declared command to `mandatory`.
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
  Since STR-V0-012 rollout step 2 (2026-10-12) the `gate-affected` target runs `corvint test`
  through `script/gate-affected-corvint.sh` instead, under STR-V0-012; this requirement governs
  `script/gate-affected.sh` as the retained rollback, which `make gate-affected-test` still runs.
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

- **AFP-V0-013:** (accepted (decision 0431); amended by decision 0431; originally decision 0289) `tools/corvint-pr-tests` MUST execute only
  typed root-module Go package argv from a separately pinned trusted planner and the existing
  AFP-V0-012 selector outside the PR checkout. It MUST verify the exact captured merge commit,
  clean tree, event base as first parent and event head as second parent, with Git replacements
  disabled and grafts refused; validate receipt profile/base/revision and package grammar;
  recheck source before execution; and preserve planner/selector digests, audit, identities,
  fallback reason, exact argv and observed exit outside source. Advice strings MUST NOT execute.
  Unsupported tool/profile, planning failure or topology drift MUST run the full root Go suite.
  Queue-backstopped PR narrowing SHALL follow AFP-V0-028, without requiring the historical
  qualification artifact; missing/malformed/stale qualification still forces FULL for the
  historical qualification profile. PR execution SHALL use the full branch's argv, environment,
  AFP-V0-023 `test-confine` wrapper and cache policy, not the driver's closed replay environment. Cancellation MUST exit nonzero and kill owned process
  groups without starting fallback. Main/release, static/security/vet/format/build/interop and
  JavaScript/E2E checks MUST remain full. PR workflow permissions MUST be read-only, with no
  persisted credentials, secrets or pull_request_target execution. Initial empty trusted pins
  MUST stay visibly full-suite; local fixture PASS MUST NOT claim hosted or narrowed CI PASS.
- **AFP-V0-014:** (accepted (decision 0431); amended by decision 0431; originally decision 0289) The optional
  historical qualification profile MUST require a separately trusted,
  SHA256-pinned qualification artifact matching exact reviewed tool source/binaries, Go 1.27.2 (decision 0522; formerly 1.27.1 under decision 0293),
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
  tool source and precede final release gate freeze. This artifact and campaign SHALL remain
  optional evidence, not a precondition for AFP-V0-028 narrowing; nothing is deleted. Clearing
  the trusted tool pin MUST restore full PR execution.

### Shared isolated container profile

- **AFP-V0-015:** (accepted by decision 0289) The trusted PR driver SHALL support one 1-based indexed row from the unchanged frozen
200-pair corpus. Each container row starts in a fresh owned checkout, with HOME/TMP/GOCACHE
empty immediately before the fixed race invocation; run mode SHALL use the same cold-cache
boundary after planning. Qualification SHALL accept retained read-only rows separately from
its writable output. Missing rows, altered pairs, profile mismatch or incomplete outcomes fail.

The retained `corvint-pr-container/1` profile and its receipt reader remain exact Go 1.27.0.
Current native execution requires Go 1.27.2 under decision 0522; current-source execution in the
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

- **AFP-V0-016:** (accepted (decision 0431); amended by decision 0431; originally decision 0320; ruleset and consent amended by decision 0390)
  `.github/workflows/ci-control-plane.yml` SHALL run
  on `workflow_run` after every completed `CI` run, cancelled ones included (AFP-V0-079), never check out
  or execute PR code, and post the commit status
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
  later push gets a fresh workflow `failure` and needs fresh consent. The `main` ruleset SHALL
  also have an active `merge_queue` rule, merge method `MERGE`, maximum group size 1 and the
  same three required checks reporting on queue commits (AFP-V0-026). Its status-check timeout
  MUST exceed one full `CI` run. The trusted tool-source pin is a `.github/` change and requires
  this admin consent on the exact head.
- **AFP-V0-017:** (accepted (decision 0431); amended by decision 0431; originally decision 0320) The optional
  historical evidence workflow `.github/workflows/pr-tests-qualification.yml`
  SHALL be `workflow_dispatch` only, with `contents: read` and no persisted credentials. It
  SHALL build the three trusted binaries from the dispatched `tool_source`, freeze the corpus
  ending at `corpus_end`, run each requested row (`1` or all 200) on its own fresh runner
  provisioned as `ci.yml`, and qualify only when every row job succeeded, with the frozen
  driver. Concurrent rows on separate runners satisfy AFP-V0-014's campaign only because every
  row identity must equal the frozen identity. It MUST NOT commit, pin, or publish anything
  but workflow artifacts. It SHALL stay available and SHALL NOT be a precondition for
  AFP-V0-028 queue-backstopped PR narrowing.
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
  on a change, or is not reached, raises nothing. Amendment (accepted, decision 0514;
  V1-0211): the separate Playwright profile explicitly excludes an unreached changed helper
  from selection under TJAA-V0-013. With matched discovery and otherwise resolved selection it
  retains `BOUNDED`, `fallback=NONE`, and nothing selected for that helper: a full E2E suite cannot
  exercise it. The shared plan still reports `NO_SELECTABLE_TEST`; other Playwright unknowns and
  discovery fallback remain unchanged. Rollback restores the silent skip in the shared plan;
  reverting only this amendment restores the undecided Playwright contract, not runtime behavior.
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
  (amended for V1-1818) The Go lexicon writes an outer component of one or two characters of a
  multi-component token as whole when the literal itself delimits it with an ASCII byte: `./`
  before a first one and `/` after a last one, so the `s/a` of `"sed 's/a b'"` is the token
  `./s/a/` and does not name `docs/specs/affected-plan-v0.md`. A component at the literal's start
  or end or beside a printf verb may join another value, and one beside a non-ASCII byte may
  continue the same name, so it stays a fragment: `"docs/a" + "lpha.go"` still names
  `docs/alpha.go`. A path whose short component is split from the rest of its name by an ASCII
  byte the lexicon drops (`my a/`) needs the whole component, as a lone token already does.
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
  the smallest dirty path and `via` that unit alone, and is not traversed, its witness naming the
  cause (AFP-V0-192); a clean plan selects none. The `UNOWNED_DIRTY_PATH` entry still stays: rule (d) is a lexical heuristic that cannot
  bound every read built at run time, and no plugin but Go records path tokens, so `plan.scope`
  stays `UNKNOWN` for an unowned path. Rollback removes the `UNBOUNDED_READER` selections and the
  two unit members.

### Complete-universe PR partition profile

- **AFP-V0-022:** (accepted (decision 0431); amended by decision 0431; owner-directed experimental profile,
  2026-10-01; V1-0616/0617) Full CI SHALL
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
  package outcomes; exact event topology and mandatory full checks remain required, while the
  200-row campaign is optional evidence for AFP-V0-028. Sharded container execution is unsupported.
  Promotion of the queue-backstopped profile from experimental requires the first 20 narrowed
  PRs to have complete forward rows and no trip, plus median and p90 PR wall time, runner-minutes
  per merged PR and ready-to-merge time measured over at least 20 merged PRs before and after,
  retained in a build-log entry. Unavailable measurements stay `NOT_OBSERVED`; local fixtures
  and timing simulations cannot establish hosted speedup. Rollback clears the selection pins and
  reverts partition placement to the previous complete round-robin workflow.
  (V1-0714) The cost bytes SHALL be regenerated by `tools/ci-shard-costs refresh` from the terminal
  `go test -json` package outcomes of one complete hosted run, recording that run and its revision;
  a failed or repeated package outcome, logs that lack a package the current table lists (unless
  the operator states the removal), or a table the partition would reject, SHALL refuse the
  refresh and leave the table unchanged. The table records the toolchain refresh runs under as
  its source `goVersion`; the partition admits `go1.27.1` and `go1.27.2` (decision 0522), so
  refresh under any other toolchain refuses. `tools/ci-shard-costs check` SHALL report each package whose observed time differs from
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

- **AFP-V0-041:** (accepted by decision 0488, 2026-10-10; V1-1096; no GitHub issue) Full
  CI MAY run an owner-allowed package as test slices in several shards, so that no single package
  sets a floor under the slowest shard. The AFP-V0-022 complete-universe guarantee is unchanged:
  every unsplit package, and every top-level test of a split package, runs in exactly one shard.
  (1) A package SHALL split only when the project-owned allow-list
  `.github/cishards/test-split-allow.json` (`corvint-ci-test-split-allow/0`) names it with a
  non-empty reason, which records that its tests neither depend on order nor share `TestMain` or
  package state across tests, and the slice file `.github/cishards/test-slices.json`
  (`corvint-ci-test-slices/0`) gives it from 2 to min(16, shard count) slices. Every other
  package, including one whose tests depend on order or on process state, SHALL stay whole.
  (2) Slices 0..K-2 SHALL each name a non-empty, strictly ascending set of plain top-level names
  (`^(Test|Example|Fuzz)[A-Za-z0-9_]*$`), disjoint from every other slice, and run as
  `go test -run '^(A|B|...)$'`. Slice K-1, the catch-all, SHALL run as `go test -skip` over the
  anchored union of every named slice. A pattern is one element that names top-level tests only,
  so subtests run with their parent; the anchors keep `TestA` from matching `TestAB`. A test added
  or renamed after generation therefore runs once, in the catch-all, and a removed name matches
  nothing.
  (3) Unusable input SHALL keep packages whole and never drop a test. A file-level failure keeps
  every package whole: an allow-list or slice file that is not exactly its closed profile (unknown
  members, trailing data, over 1 MiB), or a source other than a 40-hex revision, `go1.27.1` or
  `go1.27.2` (decision 0522; generate records its own toolchain) and a run of this repository. A package-level failure keeps that package whole: a package outside the
  runtime universe or the allow-list, a cost outside 1..3,000,000 ms, an invalid, repeated or
  unsorted name, or a pattern over 64 KiB (half of Linux `MAX_ARG_STRLEN`) or that does not
  compile. Invalid package costs keep the AFP-V0-022 lexical round-robin and split nothing.
  (4) Placement SHALL be deterministic and advisory. Whole packages carry their AFP-V0-022 cost and
  slices their recorded cost. Units go longest first, ties by package then slice index, each into
  the least-loaded shard (lowest index on ties). A slice never goes into a shard that already
  holds a sibling. With no split package the plan SHALL equal the AFP-V0-022 partition. Costs move
  units between shards and never change membership.
  (5) `tools/ci-test-slices generate` SHALL write the slice file from one complete, passing hosted
  run. `generate` and `replay` SHALL take exactly one log per shard of the declared `--shards N`
  (default 8, the go-product-shard matrix; amended from 6 by decision 0500, V1-1094), each with at least one
  terminal package outcome, and SHALL refuse any other count ("K of N shard logs present") or an
  empty log before any work, so a partial run never sets the split target or the predicted sums.
  In each shard log, every package that appears SHALL run one lifecycle: its start record first,
  then its tests, every
  test or subtest that runs ending once, then one terminal pass or skip and no further record.
  No test, subtest or package fails. After the log prefix (the job and step columns of
  `gh run view --log`, a byte-order mark, the runner timestamp and an annotation such as
  `##[error]`), a line whose content starts with `{` SHALL decode completely
  as a `go test -json` record that names its `Action`, and a record that follows other text on its
  line SHALL refuse; only lines whose content is not JSON (runner, shell and build output) are
  skipped. A record without `Package` SHALL be build output that names its `ImportPath`; a
  `build-fail` record, or any other record without `Package`, SHALL refuse, even when the stream
  ends right after it. The recorded revision SHALL be a full commit id of the repository, and the
  file-level source SHALL be checked before any work, whether or not anything splits.
  Generation checks out exactly that commit's tracked tree into a temporary directory through a
  temporary index, reads the allow-list there, and enumerates there, so untracked, modified or
  staged files cannot contribute and the repository index, worktrees and refs are not written. It
  enumerates each allowed package slower than the target (default: the ideal shard share) with
  `go test -race -count=1 -list .`, outside workspace and `GOFLAGS` redirection, and excludes
  benchmarks. A failed or unexpected
  enumeration keeps the package whole. Named slices are contiguous runs of the sorted plain names
  that minimise the largest slice. Each name is weighted by its observed top-level time, the
  median for a name the run lacks, or by count when the run has no test records. Per-test costs
  (V1-1100) MAY replace those weights. Any of the following SHALL refuse the write: a failed,
  repeated, restarted or unterminated outcome; a failed build; a damaged, truncated or interleaved
  record; a revision that is not a full commit id of the repository; a source or file the
  partition would not admit. So SHALL SIGINT
  or SIGTERM, which cancels checkout and enumeration and removes the temporary checkout. On unix
  each checkout and enumeration subprocess SHALL run in its own process group; cancellation
  interrupts the group, kills every member still alive ten seconds later, and returns only once
  the group is empty, so no descendant outlives the command. CI never
  enumerates, and never consults the tested test binary for placement.
  AFP-V0-049 combines a split package's per-slice outcomes in `generate` and `replay` and
  supersedes the limit below that they read only whole runs.
  (6) The slice code, allow-list and slice file SHALL be part of the protected helper. That helper
  is built independently of tested module directives, and its profile digest (domain
  `corvint-ci-partition/2`) covers them, and so does the AFP-V0-014 frozen identity. The
  package-level PR driver SHALL run a sharded execution only with the helper's own placement of
  whole packages and slices, as AFP-V0-048 states (amended by V1-1361, which replaced the
  refusal while the helper splits any package), so that a driver shard and a sliced fallback
  shard cannot mix. A pinned older driver fails on the digest mismatch.
  (7) The full path SHALL run each slice as its own `go test` invocation, with the same `-exec`
  confinement and `-json -p 1 -count=1 -race -timeout 50m`. Whole packages keep one invocation.
  The shard SHALL fail when any invocation fails, before the AFP-V0-024 tested-tree record.
  Every invocation SHALL run inside the one AFP-V0-040 capture, so the retained stream holds the
  whole packages and the slices of that shard.
  (8) `tools/ci-shard-costs` `refresh`, `check` and `check --advisory` SHALL take the CI shard
  count as a declared `--shards N`, the go-product-shard matrix size, and SHALL refuse unless
  exactly N logs, one per shard, are given. They SHALL derive the split set from the committed
  allow-list and slice file (`--allow`, `--slices`) with the same `SplitPackages` call as the
  partition, over the packages the logs name and with N, never the number of logs supplied, as
  the shard count. A log SHALL hold at most one terminal outcome per package. A split package
  SHALL end exactly once per slice, so each slice in a different log, and its cost is the sum of
  those outcomes. Any other count, a single outcome included, any failed outcome, a repeat of a
  package the files do not split, a slice that started without a terminal outcome, or an allowed
  package that the slice file splits into more slices than N SHALL refuse the refresh and make the
  drift check abstain, so one slice is never counted as the whole package. One slice's outcome is
  indistinguishable from a whole run, so a run made before the split cannot refresh or check a
  split package's cost. An
  unreadable or unusable allow-list or slice file splits nothing, so a sliced run then refuses
  visibly rather than being half counted.
  Non-goals: runtime enumeration in CI; splitting subtests; selection or skip authority; sharded
  container execution; and a hosted speedup claim from replay alone.
  Limits:
  - A split loses cross-slice parallel interleavings, including race detection between tests in
    different slices.
  - It repeats the package's build, link and `TestMain` in each slice; that overhead is
    NOT_OBSERVED.
  - Costs and slices need different runs. `tools/ci-shard-costs` measures a split package only
    from a sliced run and refuses a run made before the split. `tools/ci-test-slices generate`
    reads only whole runs and refuses a sliced one ("two terminal outcomes"), so regenerating the
    slice file needs a complete run in which the package ran whole, which full CI does not make
    while the package is split (for example, a run of a branch whose slice file is emptied).
  - A stale slice file only moves time between shards.
  - `--shards` is declared, not read from the workflow. A count that differs from the matrix
    refuses a sliced run, or measures only the logs given, but never counts one slice as a whole
    package. `ci-test-slices` has no cost table to compare, so a partial log set declared as its
    own shard count (five logs of a six-shard run as `--shards 5`) is not detected there.
  - On Windows, cancellation kills only the immediate subprocess, not its descendants (no job
    object). On unix, a descendant that leaves its process group is not reached, and a SIGKILL of
    the tool itself stops nothing and leaves the temporary checkout behind.
  - The AFP-V0-014 selective PR driver runs a split package's slices on their assigned shards
    (AFP-V0-048). The pins are empty today.
  Replay of hosted run 38055182050 is recorded in build log 2026-10-10-ci-test-level-shards.
  Falsifier: a passing sliced run in which a top-level test of a split package has zero or two
  terminal outcomes, or in which a split package also runs whole. Rollback: set `packages` to `{}`
  in `.github/cishards/test-slices.json`, after which the plan equals AFP-V0-022, or revert the
  workflow slice loop and the helper files.

### Declared confined test read scopes

- **AFP-V0-023:** (owner-directed, proposed, 2026-10-01; V1-0246, V1-0081; declaration-edit
  clause amended by decision 0528, V1-1399) A project MAY commit
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
  is an ancestor of an entry, or is the declaration itself (an edit of the declaration is
  narrowed by AFP-V0-050, for the planner and for the selector). Rules (a) and (b) and every dependency
  edge are unchanged, and rule (c) is unchanged except that the planner bounds a declared package's
  path-literal reads by the same scope (AFP-V0-161); a package that depends on a declared root locator but is not itself declared
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
- **AFP-V0-024:** (owner-directed, proposed, 2026-10-04; V1-0717; merge-queue amendment under the owner's standing auto-accept (2026-10-11); V1-0760) A push to `main` MAY reuse a
  passed pull-request or merge-queue result instead of repeating the root Go race invocation, only when all of
  the following hold: the pushed commit is a two-parent merge; a completed, successful run of
  `.github/workflows/ci.yml` in this repository exists that is either a `pull_request` run for
  the merged head (the pushed commit's second parent) or a `merge_group` run whose head is the
  pushed commit itself (V1-0760: with merge method `MERGE` the queue commit is the commit that
  becomes `main`); and that run's AFP-V0-170 `ci-tested-tree-record` job printed a record of the
  current shard count whose tree is exactly the pushed commit's tree id (amended under the owner's
  standing auto-accept (2026-10-11); V1-2271: the per-shard `ci-tested-tree-TREE-full-SHARD-of-SHARDS`
  artifacts it replaces were trusted by name alone, and the shard jobs that uploaded them run
  repository code, so any tested tree could forge them). That job SHALL print the record only
  after every shard passed its complete package set in the full branch, so a documentation-only
  (DCI-V0), selection-narrowed or reused (AFP-V0-046) run can never be reused; after a reused
  queue run the push can reuse only the same pull-request run, whose record names the same tree. Tree
  identity binds every tracked file, including the workflow file blob and the pinned Go
  version; no separate comparison is trusted. REUSE therefore means that the pull-request
  merge-ref commit, or the queue commit, with the identical tree passed in full. A fork pull-request run is admitted like any
  other: it runs in this repository, and a record for the merged head can only come from a
  workflow that head, and so the merged tree, contains. The decision SHALL be made by `tools/ci-reuse-plan --event push`, built from
  the pushed `main` commit, over retained public run and job listings (the `pull_request`
  runs of the merged head and the `merge_group` runs of the pushed commit) and the record job's
  log, read with `actions: read` and `contents: read` (an ARTIFACT-V0-008 exception row names the
  `docs-plan` job); no artifact is read. The decision's `head` stays the merged head for either admitted run; `runId`
  names the reused run. Only the root race invocation is omitted: static, build, interop, documentation
  and artifact checks still run, except that AFP-V0-080 skips `go-static`, `go-interop` and
  `artifact-integrity` when the queue run of the pushed commit itself passed them. The decision, with the reused run id and tree id, SHALL be
  retained as the `ci-reuse` artifact. A missing record, an unreachable API, a failed build, any
  other event or any mismatch runs FULL. Limits: the record name is written by the reused run's
  own workflow steps, which the tree identity binds to the merged, owner-protected workflow;
  the commit id and history of the checkout, the event environment, the installed host
  packages and the hosted runner image are not part of the identity and may differ between the
  two runs; a run still in progress is not admitted, and the required `ci-control-plane`
  status, posted only on a completed run (AFP-V0-026), is what normally completes the queue run
  before `main` moves; the
  hosted reuse path and its effect on completed `main` results are `NOT_OBSERVED` until a
  merge lands with matching records, and the reuse of a `merge_group` run is `NOT_OBSERVED`
  until the first push after a queue run that wrote them. Rollback removes the `reuse` step, which restores FULL
  for every push; restoring the `pull_request`-only record condition and dropping the
  `merge_group` listing withdraws only the V1-0760 queue-run reuse.
- **AFP-V0-025:** (owner-directed, proposed, 2026-10-04; V1-0719, V1-0752) The repository SHALL
  commit `.corvint/unbounded-readers.json`, the closed object
  `{"profile":"corvint-unbounded-reader-set/0","units":[DIR...],"reasons":{DIR:REASON}}`, at
  most 64 KiB. `units` names, strictly ascending, the test directories of the admitted
  unbounded test units: units with tests that rule (d) selects on any dirty path because
  neither a literal nor a declared read scope (AFP-V0-023) bounds their reads. A unit whose
  identity is its single test file (a shell test, `NSA-V0-011`) is named by that file's path
  instead of its directory (V1-1301). `reasons`
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
- **AFP-V0-026:** (accepted (decision 0431); amended by decision 0431; owner-directed, 2026-10-04; V1-0752; identical-tree reuse amendment under the owner's standing auto-accept (2026-10-11); V1-1360) `.github/workflows/ci.yml`
  SHALL also run on `merge_group` `checks_requested`, so a merge-queue entry reports the
  required `go-product` and `doc-gates` checks on the exact commit that becomes `main`. Such a
  run MUST be FULL except for the identical-tree reuse of AFP-V0-046: the documentation
  classifier (DCI-V0), the AFP-V0-013 driver and the AFP-V0-022 order plan stay bound to their
  own events and are off for it, and the AFP-V0-024 reuse applies to it only in the form
  AFP-V0-046 admits. `.github/workflows/ci-control-plane.yml` SHALL post `ci-control-plane` `success` on
  the head of a completed `merge_group` run of `CI` whose branch begins
  `gh-readonly-queue/main/pr-`, and post nothing for any other branch; it still never checks
  out or executes repository code. That status carries no consent of its own: a pull request
  enters the queue only after the AFP-V0-016 decision on its own head, and a queue commit
  holds only such pull requests on top of `main`. Limits: enabling the queue is the owner's
  ruleset change and hosted queue behavior is `NOT_OBSERVED` until then; V1-0760, which lets
  the `main` push reuse the queue run of the identical commit (now part of AFP-V0-024), is a prerequisite for narrowing
  (AFP-V0-028); a narrowed PR MUST NOT write an AFP-V0-024 tested-tree record; the pull-request run
  still tests a possibly stale merge, so only the queue run closes the AFP-V0-025 residual
  skews; the status is posted only after the whole `CI` run completes, so the queue's
  status-check timeout must exceed that run; the queue-entry precondition, the branch form
  and `workflow_run` delivery for `merge_group` runs are GitHub behavior assumed here, and a
  wrong assumption about the latter two posts nothing; the status stays on a queue commit
  after its entry leaves the queue, so a pull request whose head is that commit shows it
  until its own `CI` run completes and AFP-V0-016 replaces it, which can admit only
  `.github/` content already consented to on a queued head. Rollback: disable the queue in the ruleset
  only after restoring FULL PR runs (R1 below), then remove the trigger and the `merge-group` job; removing them while the queue is
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
  `docs-plan` and `go-product` still run; AFP-V0-081 also withholds `go-interop`,
  `artifact-integrity`, `ci-shard-cost-drift` and intake containment, which the batch reruns. Limits: the label is applied by
  the batching agent and nothing checks that a batch pull request actually contains the
  constituent; a mislabelled pull request is blocked, never merged untested. Rollback removes
  the job condition and the `go-product` message.
- **AFP-V0-028:** (accepted, decision 0509; intent decision 0431) Queue-backstopped narrowing SHALL apply only to
  `pull_request` root Go race packages selected by the trusted AFP-V0-012 selector, intersected
  with the existing AFP-V0-022 complete-universe shard partition. Empty intersections SHALL run
  no Go test and retain their audit. Only the package set may differ: argv, environment,
  AFP-V0-023 `test-confine` wrapper and cache policy SHALL match the full branch, not the driver's
  historical closed replay environment. Missing or untrusted tools, planning or selector failure,
  a `FALLBACK` verdict, topology or partition drift, a dirty root module definition and any
  module-level Go frontier SHALL still force FULL. Three additional conditions SHALL force FULL:
  the PR base is not `main`; a changed path is under `.github/`; or the API cannot read the
  `main` branch rules or they lack an active `merge_queue` rule. Disabling the queue while
  narrowing is armed SHALL therefore force FULL. `merge_group`, `main` push and release stay
  FULL; static, vet, format, cross-build, interop, documentation and artifact checks stay full
  on every event (a `main` push takes them from the queue run of its own commit, AFP-V0-080, and a
  `ci:batched` constituent from its batch, AFP-V0-081; neither narrows a package set). AFP-V0-026 keeps DCI, the driver, order plan and AFP-V0-024 reuse off on
  `merge_group`.
  Scope `UNKNOWN` may narrow only when every `plan.unknown` entry belongs to this closed,
  reviewed allowlist of classes already covered by AFP-V0-012 widening: `LANGUAGE_FRONTIER`
  with a non-`go:` detail (fixture paths still attributed by rules (b) to (d));
  `go:build-constraint-variants` and `go:nested-module-frontier` (the race job covers only
  linux/amd64 root packages and cross-build/nested interop remain full); `UNOWNED_DIRTY_PATH`
  attributed by rules (b), (c) and (d); and `NO_SELECTABLE_TEST` traversed to dependents
  (AFP-V0-020). Every other reason or detail, including future additions, SHALL force FULL
  until reviewed amendment of the allowlist.
  Planner, selector and driver SHALL remain built outside the PR checkout from a reviewed
  `main` commit pinned in `ci.yml`; changes to the pin require AFP-V0-016 exact-head admin
  consent. Go version, Git and tool-digest drift SHALL force FULL. The SHA256 historical
  qualification artifact and 200-row campaign SHALL NOT be prerequisites; AFP-V0-014/017
  and the historical workflow remain optional evidence. V1-0760 MUST be merged and the
  AFP-V0-029 shadow bar met before narrowing is enabled. A narrowed PR run MUST NOT write an
  AFP-V0-024 tested-tree record. Selection remains non-authoritative (AFP-V0-004); exclusions
  are deferred to the full queue run, which MUST pass before the commit reaches `main`.
  Both `go-product` and step summaries SHALL say "selected packages passed: N of M; the full
  suite runs in the merge queue", never "suite passed". Hosted behavior, net cost and batch
  speedup remain `NOT_OBSERVED`; batch speedup is not claimed.
- **AFP-V0-029:** (accepted, decision 0509; intent decision 0431; amended decision 0537; amended by the owner (owner chat, 2026-10-20; V1-0812)) Every PR run computing a trusted selection SHALL
  retain S with tool source, plan and selection digests and shard partition digest. Every
  `merge_group` shard SHALL retain bounded terminal `go test -json` package outcomes. A
  default-branch `workflow_run` job, as in AFP-V0-016, SHALL build one row per queue run,
  with only `actions: read` and `contents: read`, never executing repository code. S is the
  selection of the latest successful PR run for the head entering the queue; F is every queue
  package with fail, timeout, build failure or missing outcome. A miss is F minus S for a PR
  whose PR-time run narrowed; F intersect S is `SELECTED_FAILED_IN_QUEUE` (base skew, flakiness
  or environment difference), not a miss. Each failing package SHALL be rerun once on the
  same queue commit; a pass is `FLAKY`, but the queue verdict remains failure. Missing or
  unreadable inputs SHALL produce `INCOMPLETE`, neither clean nor a miss, including a missing
  selection artifact at queue time. (Amended by the owner, decision 0537; V1-1514.) A hosted
  pre-step failure, a shard job the runner failed at `Set up job` before any step of its own, SHALL
  instead produce `INFRA` when it explains every gap of the row, as AFP-V0-052 states; an `INFRA`
  row is neither complete nor `INCOMPLETE`.
  The row SHALL be `corvint-pr-forward-row/0`, at most 64 KiB, named
  `ci-forward-<CLEAN|FLAKY|MISS|INCOMPLETE|INFRA|NARROWED>-pr<N>-<queue sha>`, with default artifact
  retention; `NARROWED` is the AFP-V0-099 class of a pull-request run that narrowed.
  The class is the worst in the row: `MISS` if any missed package fails its rerun, `FLAKY`
  if every missed package passes it. Rows SHALL NOT feed ranking, selection or learning;
  their admission effect can only widen to FULL. Shadow SHALL first enable the queue,
  full-event policy and rows with PR tests still FULL, computing and retaining S before the
  full shard. Shadow PR runs SHALL produce rows from their same-commit complete outcomes.
  Shadow is enabled only after a real PR has merged through the queue with `go-product`,
  `doc-gates` and `ci-control-plane` reported on the queue commit, `main` equal to that commit
  and `dogfood-check` reading its seal; until then it is `NOT_OBSERVED`.
  Narrowing requires, among all complete shadow rows of the current epoch (one per pull request
  and commit, not only the latest 20), at least 20 complete rows, at least 5 of them with failing
  packages, and no row anywhere in the epoch with a miss other than `FLAKY`; `INFRA` rows stay
  outside these counts and `INCOMPLETE` rows are not complete. The bar is met only on a listing
  that reached the start of the epoch (it read every artifact or names a row of an earlier
  epoch); otherwise it is not established, never met. (Amended by the owner (owner chat,
  2026-10-20; V1-0812): the bar counts the whole epoch; the AFP-V0-030 kill switch keeps its
  latest-20 window.) (Amended by the orchestrator under owner delegation (owner chat,
  2026-10-20; V1-0812).) `NARROWED` rows (AFP-V0-099) are outside every count of this bar and are
  dropped before the per-(pull request, commit) deduplication, so a narrowed run never replaces a
  counted row; a PR run reads the bar only through the trusted verdict of AFP-V0-097, never by
  listing the epoch itself. Hosted queue branch form, `workflow_run` delivery,
  branch-rules API and `main` reaching the tested queue commit remain assumed, `NOT_OBSERVED`.
- **AFP-V0-030:** (accepted, decision 0509; intent decision 0431; amended decision 0537; amended by the orchestrator under owner delegation (owner chat, 2026-10-20; V1-0812)) PR runs SHALL return to FULL with reason
  `forward-qualification-tripped` when the latest 20 rows of the current epoch, counting
  `INCOMPLETE` rows in the window, contain at least 2 `MISS` rows or at least 3 `INCOMPLETE`
  rows. (Amended by the owner, decision 0537; V1-1514.) An `INFRA` row (AFP-V0-052) takes no place
  in the window and never counts as `INCOMPLETE` or toward the 20 complete rows of AFP-V0-029.
  (Amended by the orchestrator under owner delegation (owner chat, 2026-10-20; V1-0812).) A
  `NARROWED` row (AFP-V0-099) likewise takes no place in the window. The window is read twice:
  in the trusted AFP-V0-097 verdict, and live by the PR job over the newest 5 pages of artifact
  names, so rows newer than the verdict can still trip it.
  The PR job SHALL list artifact names as AFP-V0-024 does, without downloading them;
  listing failure SHALL force FULL. Narrowing is off unless the admin-settable repository
  variable `CORVINT_PR_NARROWING` is exactly `on`; unset, `off` or any other value SHALL force
  FULL immediately, and the variable can only disable. A reviewer
  classifying a single `MISS` as `CHANGE_CAUSED` SHALL set that variable; this classification
  is human judgment, not information carried by an artifact name. Re-arming SHALL require a
  reviewed `.github/` change advancing the `CORVINT_PR_FORWARD_EPOCH` pin, removing earlier
  rows from the window, with a build-log entry naming the review. The queue fails and ejects
  a missed PR before it reaches `main`; the switch protects PR-time green and queue throughput.
  Rollback of AFP-V0-028..030 SHALL follow decision 0431: R1 sets
  `CORVINT_PR_NARROWING=off` or clears `CORVINT_PR_TOOL_SOURCE`, restoring full PR race runs
  while the queue remains and forward rows continue in shadow; R2, only after R1, removes
  the `main` ruleset's `merge_queue` rule, restoring `pull_request` plus the three required
  checks (the inert `merge_group` trigger may stay); R3 reverts the spec amendments and
  requirements AFP-V0-028..030, restoring decisions 0289/0320's required AFP-V0-014 artifact.
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
  (the plugin reads a quoted path with Go string-literal rules and leaves one it cannot read
  unresolved, V1-0873), the root `go.work` use set differs from the observed
  directories, or an observed directory replacement is not repository-relative or does not name
  exactly an observed module directory or a path at or below an unlisted module's directory (a
  differently cased or linked path can name a nested module, so identity is not established).
  A module that is
  none of these neither builds against an observed module nor is built by one,
  so no change to an observed module reaches it and it closes. The plugin keeps one evidence
  record per nested `go.mod` read, with the reason it stayed open; it is internal and does not
  reach the plan wire. The reverse direction: a nested module's files are never units, so a
  change inside one is the `LANGUAGE_FRONTIER` detail of AFP-V0-191, which names the module
  directory, not `UNINDEXED_SOURCE_PATH`.
  Non-goals: reads by a nested module's tests of observed data (AFP-V0-012 already treats nested
  literals as selecting nothing), symlinked manifests the walk does not follow, and a `GOWORK` or
  `GOFLAGS` (`-modfile`) environment value, which the plugin cannot observe. Rollback restores the unconditional frontier
  in `observeModules` and removes `nested.go`.
- **AFP-V0-190:** (accepted under the owner's standing auto-accept (2026-10-11; V1-0874)) The Go
  plugin's workspace use set (AFP-V0-008) MUST come from the strict manifest reader
  (`readManifest`, the AFP-V0-031 reader), so the unit universe and the nested-module check read one
  `go.work` the same way: a quoted directory is unquoted with Go string-literal rules and two
  directories on one `use` line are both read. A `go.work` that reader refuses (a raw-string or
  otherwise unreadable `use` entry, an unknown directive, over-size or unreadable) MUST raise the
  frontier `go:workspace-unparsed` and observe only the root module, which the AFP-V0-031 rules
  then keep open against every nested module; `CommittedReadScopes` treats the same refusal as an
  unknown base (AFP-V0-050). Before this, a loose line reader guessed a use set
  from such a file and the plan stayed bounded. Evidence:
  `TestWorkspaceUseSetIsTheStrictReaders_AFPV0190` (fails without the change). Rollback: restore the
  line reader in `workspaceDirectories`.
- **AFP-V0-191:** (accepted under the owner's standing auto-accept (2026-10-11; V1-0922)) A language
  plugin MAY return `Result.Unobserved`: canonical repository-relative directories, strictly
  ascending, that it owns but builds no unit for; `Build` MUST refuse any other value as
  `ErrInvalidLanguage`, and the graph digest body carries them as the `unobserved` member
  (`namespace:directory`), omitted when empty so a graph without them keeps its digest. A dirty
  path such a plugin claims that lies under one of them MUST be the unknown `LANGUAGE_FRONTIER`
  with detail `<namespace>:unobserved-directory:<directory>:<path>` (the longest directory wins) in
  place of `UNINDEXED_SOURCE_PATH`; the reason enum is unchanged and the detail keeps the plan
  `UNKNOWN` and the gate tier full (AFP-V0-028 admits only its own `go:` details). The Go plugin
  reports the directory of every nested `go.mod` it does not observe, leaving out the root and any
  directory that is or contains an observed module, because an unindexed file there may belong to
  an observed module and stays `UNINDEXED_SOURCE_PATH`. Evidence:
  `TestNestedModuleChangeIsANestedModuleUnknown_AFPV0191`,
  `TestUnobservedLeavesOutDirectoriesHoldingObservedModules_AFPV0191`,
  `TestUnobservedDirectoriesAreValidatedAndDigested_AFPV0191`, and the amended expectation of
  `TestIndependentNestedModulesCloseTheFrontier_V1_0867`. Non-goal: indexing a nested module's
  packages, which would need its own root-runnable test command (V1-0922 design note, PR #1420).
  Rollback: drop `Result.Unobserved`, the digest member and `Graph.unownedUnknown`.
- **AFP-V0-192:** (accepted under the owner's standing auto-accept (2026-10-11; V1-2075)) Every
  `UNBOUNDED_READER` witness MUST carry a `cause` object, `{"unit": <unit id>, "reads": <that unit's
  unboundedReads>}`, naming the concrete read path that makes the unit read any dirty path: the
  selected unit itself when it records its own `unboundedReads`, otherwise the `locatesRoot` unit
  that starts the dependency or test-user chain by which rule (d) reached it. No other witness kind
  carries `cause`, and the member is omitted there, so every other selection's wire is unchanged;
  `via` stays the unit alone, so ranking (AFP-V0-034) is unchanged. A change that reaches only
  non-source paths, such as `AGENTS.md` alone, still selects every rule (d) unit (the reads are
  lexically unbounded, so dropping them would be unsound), but each selection now states why; a
  repository with no rule (d) unit selects none. Evidence:
  `TestInstructionFileChangeNamesEachUnboundedReadersCause_V1_2075` (fails without the change) and
  the amended expectations of `TestGoTestOnlyChangeSelectsItsPackageButNotItsImporters_V1_0984`.
  Non-goal: proving that a rule (d) unit cannot reach the dirty path, which needs a bound the
  repository declares (AFP-V0-023). Rollback: drop `Witness.Cause` and `Graph.causes`.

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
  runs. Under AFP-V0-041 the capture holds every invocation of the shard, and a split package's
  slice outcomes combine into one cost as AFP-V0-041 (8) states. Rollback: delete the
  `ci-shard-cost-drift` job and the retention step, restore the plain
  `go test` invocation in `go-product-shard`, and remove `--advisory` from `tools/ci-shard-costs`.

- **AFP-V0-042:** (accepted, decision 0496; V1-1117; no GitHub issue; owner consent for the
  `.github` change under decision 0390) The `go-static` job of `.github/workflows/ci.yml` SHALL run
  `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go vet ./...` on the root module and on
  `interop/cem01-go`, so a `_test.go` file that does not type-check for Windows fails the required
  `go-product` check before merge; `go build` never compiles test files and the native `go vet`
  never sees a file excluded by its build constraint. The step SHALL be skipped only on a pull
  request whose checked-out merge commit has the event base as its first parent and changes no
  `.go`, `go.mod` or `go.sum` path against it; a push, merge-group entry, missing base parent or
  failed diff runs the vet (invariant 2). The checkout keeps `fetch-depth: 2` for that diff.
  Non-goals: `GOOS=linux GOARCH=arm64` stays with the Makefile `cross-vet` and nightly release
  gates; no new job, no change to shard membership, docs-plan modes or the `ci-control-plane`.
  Failure modes: a Windows-only vet failure fails `go-static` and therefore `go-product`; a
  non-Go input that only Windows vet would reject (such as an embedded file) on a skipped pull
  request is caught by the merge-group and main runs. Falsifier: a pull request adding a
  `//go:build windows` test that references an undefined name passes `go-static`. Acceptance
  evidence: `actionlint`, `make ci-least-privilege-check`, and the local fixture run of the step's
  script (build log 2026-10-10-windows-test-vet): a docs-only merge skips, a merge adding such a
  test exits 1 while the native vet and Windows build pass; hosted behaviour is NOT_OBSERVED until
  the change's own CI runs. Rollback: delete the step and the `fetch-depth: 2` line.

- `AFP-V0-043`: (accepted, decision 0505) Cost-table
  `refresh` and plain `check` MUST validate each shard log against the completed-stream
  standard of AFP-V0-040 before combining costs or writing the table: every non-blank
  line is a valid event, every started package ends, and each log has a terminal package
  outcome. Malformed lines, truncated final events and unfinished packages MUST refuse
  with exit 2 and leave the table unchanged. Valid raw streams MUST retain the prior
  non-strict observer's encoded table bytes. Non-goal: changing the legacy `observe`
  helper's other callers. Rollback: revert the strict reader selection and its tests.

- `AFP-V0-044`: (accepted, decision 0519 (V1-1141); GitHub #783) When `--base`
  contributes at least one committed range path that is not in the worktree dirty set and that
  dirty set is non-empty, the worktree-planned receipt (`affected-plan/0`, `affected-plan/1` and the
  Playwright receipt) MUST carry a top-level `warning` string naming the HEAD commit the receipt's
  `revision` names and the worktree dirty-path count, and stating that the dirty content is
  mutable and pinned by no commit. It is the `affected` counterpart of `DIRTY-CACHE-014`: the
  requested paths are the committed range, and the plan unions them with uncommitted worktree
  content (AFP-V0-010). A clean worktree, the worktree-only form (whose request is the dirty set
  itself), a range entirely within the dirty set, and `--snapshot` (which never reads the
  worktree) MUST omit the member, so those receipts stay byte-identical. The warning carries no
  path or file content. Rollback: remove the member.
- `AFP-V0-045`: (accepted, decision 0519 (V1-1141); GitHub #783) `affected` MUST
  accept the experimental boolean `--require-clean` on the worktree, `--base`, provider and
  Playwright forms. When the worktree status (AFP-V0-002, untracked files included) has any entry,
  it MUST refuse with exit 2, code `unsupported-affected-mixed-worktree`, a message naming the
  HEAD commit and dirty-path count, and no stdout document, before the range diff, the graph walk
  or any provider runs. `affected` has no structured abstention document, so the strict form is
  this typed refusal, consistent with its other fail-closed codes. A clean worktree MUST produce
  the byte-identical receipt the command produces without the flag. The flag rejects a value or
  repetition, `--snapshot`, and `affected discovery`, each as `invalid-arguments`. It adds no
  repository, index or ledger write. Rollback: remove the flag and its refusal.
- **AFP-V0-094:** (accepted under the owner's standing auto-accept (2026-10-11; V1-2044); GitHub #1301)
  `affected` MUST accept the experimental repeatable `--path PATH` (as `--path PATH` or
  `--path=PATH`, at most 4096 distinct paths; the separated form refuses a value that begins with
  `-` as `invalid-arguments`, so `--path --full` never swallows the next option, and a
  dash-prefixed name is written `--path=VALUE`) and plan exactly that changed-path set at the
  captured HEAD instead of the worktree status set or a `--base` range. Each value MUST be a
  canonical repository-relative path (`ValidRelativePath`), else `invalid-arguments`; a repeated
  path collapses, and `plan.dirty` is the sorted unique set. A path absent at HEAD is planned as
  a deleted or unindexed path under AFP-V0-004, never refused. The worktree status is still read
  (AFP-V0-001): any entry, untracked included, MUST refuse with exit 2, code
  `unsupported-affected-mixed-worktree`, a message naming the HEAD commit and dirty count, and
  no stdout document, before the graph walk, because a dirty graph is not HEAD's. An ignored file
  HEAD does not track is not a dirty path, so after the graph build the ignored untracked set
  (`git ls-files --others --ignored --exclude-standard --directory`, with the status bounds) is
  read, and a listed path or any graph unit source or test that is, or lies under, an ignored
  entry MUST refuse with the same code, naming that path and the HEAD commit; so MUST an ignored
  entry that overlaps a Go unit's `testdata/` directory, its package directory when the unit
  embeds files or declares a read scope (which admits that subtree even with no entries), or one
  of its declared read scopes, naming that area (amended for V1-2086). The receipt is
  the `affected-plan/1` (or, with `--full`, `affected-plan/0`) document of the default form with
  the same advice, provider projection, drift checks and `range` `{base: "", paths: []}`, plus
  one optional top-level `path_list` member echoing the sorted set (CCF-V1-006: an added optional
  member); every other form omits it and stays byte-identical. For one committed change its
  `plan` MUST be byte-identical to the `--base` plan of that change over the same clean tree.
  `--path` MUST NOT combine with `--snapshot`, `--base`, `--provider`, `--playwright-config`,
  `--playwright-discovery` or `--appmap-impact` (`invalid-arguments`). Non-goals: no
  Playwright, provider or snapshot path-list profile, no overlay of uncommitted bytes, no change
  to `impact` output. Rollback: remove the flag, its refusal and the member.
- **AFP-V0-095:** (accepted under the owner's standing auto-accept (2026-10-11; V1-2044); GitHub #1301)
  A `--path` plan MUST use the same transitive reverse-dependency traversal as every other form
  (`affected.Select`), so a test reached through an intermediate importer is selected with its
  full witness chain even when the intermediate unit declares no test, and MUST keep the
  existing scope vocabulary: `plan.scope` is `BOUNDED` only when no listed path raised a
  widening reason, and an unowned, unindexed or frontier-bearing listed path leaves `UNKNOWN`
  with its typed `plan.unknown` entry. No `--path` receipt states a selection as complete that
  its graph did not close. Rollback: with AFP-V0-094.
- **AFP-V0-096:** (accepted under the owner's standing auto-accept (2026-10-11; V1-2044); GitHub #1301)
  The `impact` help MUST state that its reverse-import results stop at direct importers (one
  hop; Ruby's advisory autoload rows reach two), that a test reached through an intermediate
  importer is absent, that a `READY` impact receipt is evidence and not a closed test
  selection, and that `affected --path` plans one; the `affected` help and
  `docs/CAPABILITIES.md` MUST document `--path`. The `impact` receipt bytes are unchanged: they
  are pinned by `conformance/cli-parity-v0/manifest.json` under CCF-V1-006, so a
  machine-readable closure-depth member there remains a separate breaking-change decision.
  Rollback: remove the sentences.
- **AFP-V0-097:** (decided by the orchestrator under owner delegation (owner chat, 2026-10-20;
  V1-0812)) Whole-epoch bar evidence. The AFP-V0-029 bar and AFP-V0-030 window SHALL be
  computed over the whole epoch only by the trusted `.github/workflows/ci-forward-bar.yml`
  workflow, which runs on `schedule` or `workflow_dispatch` from `refs/heads/main` alone, with
  only `actions: read` and `contents: read`, executes only `.github/forward-rows` of that main
  commit and no repository code, and retains `afp-shadow-bar-epoch-<N>` holding one
  `afp-shadow-bar-verdict/0` document: status `MET`, `NOT MET` or `NOT ESTABLISHED`, the epoch
  counts and window, the kill switch, the epoch start, the run id and attempt, the repository and
  the main SHA. A pull request MAY narrow only when the newest such artifact by `created_at`
  (a) came from a `schedule` or `workflow_dispatch` run of that workflow path on `main` of this
  repository, not a fork, (b) says `MET` with a clear kill switch, (c) was created at most 7 days
  ago and has not expired, and (d) names the current forward epoch; and the newest 5 pages of
  forward-row names still show no `MISS` of the epoch and a clear kill switch. Those pages SHALL
  either end the listing or reach back to the verdict artifact's `created_at` and hold the latest
  20 rows of the epoch or its start; a read that stops at the page budget without that coverage
  cannot exclude a `MISS` beyond it. Anything else, including an unreadable listing, run or
  download, an unparseable verdict or one naming another run, or uncovered pages, SHALL run full
  CI with the reason recorded in the job summary. The acceptance script runs
  from the pull request's base commit, not its head. The `CORVINT_PR_FORWARD_EPOCH` pin in
  `ci.yml`, `FORWARD_EPOCH` in `ci-control-plane.yml` and `ci-forward-bar.yml`, and the epoch in
  the verdict artifact name SHALL be equal. Non-goals: no push trigger (a whole-epoch listing
  costs up to 400 artifact pages); no new marker artifacts; no authority beyond widening.
  Failure modes: a stale verdict up to 7 days old is bounded by the live 5-page kill-switch read;
  a verdict whose listing did not reach the epoch's start is `NOT ESTABLISHED`; an epoch advance
  leaves no verdict for the new epoch, so every PR runs full until the schedule computes one.
  Rollback: unset `CORVINT_PR_NARROWING`, or revert the change.
- **AFP-V0-098:** (decided by the orchestrator under owner delegation (owner chat, 2026-10-20;
  V1-0812)) Selector identity. A pull-request Go shard SHALL narrow only when the driver's
  selection (`corvint-pr-tests --mode plan`, which executes nothing) is a superset of the
  `corvint test --dry-run` shadow selection of AFP-V0-047 for the same base and tested merge
  commit, so a miss computed against the shadow S of the epoch's rows can never fail open. The
  gate SHALL also force FULL, recording each reason in the `corvint-pr-narrowing/0` record, when
  the base branch is not `main`, the change touches `.github/` or its change set is unreadable,
  `main` has no `merge_queue` rule or its rules are unreadable, the driver refused, fell back or
  selected the whole universe, its plan names another base, head or target, the shadow exited
  other than 0 or 3, wrote no plan, has a scope other than `NARROWED`, `FULL` or `NOTHING`, or is
  incomplete for anything but a declared adapter limit. It runs only on same-repository
  `pull_request` events with `CORVINT_PR_NARROWING` exactly `on`, a MET AFP-V0-097 verdict, and
  both `CORVINT_PR_TOOL_SOURCE` and `CORVINT_TEST_SHADOW_SOURCE` set, from the trusted tool
  checkout. A narrowed shard runs its own packages and slices intersected with the driver
  selection, writes no AFP-V0-024 tested-tree record and no cost capture, and says "selected
  packages passed; the full suite runs in the merge queue". The gate is the only route to a
  narrowed shard; the driver never executes tests in CI. A skipped, refused or failed verdict or
  gate, a record that could not be retained, an `executed` list beside the record that was not
  written or is unreadable or not equal line for line to the retained record's `executed`, or an
  executed set that differs from the record runs the whole shard. Only a verified empty
  `executed` (an AFP-V0-028 empty intersection) runs no Go test.
  Non-goals: no change to the `merge_group` run, which stays full. Rollback: as AFP-V0-097.
- **AFP-V0-099:** (decided by the orchestrator under owner delegation (owner chat, 2026-10-20;
  V1-0812)) Rows of narrowed runs. Each narrowing gate SHALL retain its record as
  `ci-narrowing-<shard index>` (90 days). A pull-request row whose run retained any `NARROWED`
  record, or a record that cannot be read, SHALL be class `NARROWED`, excluded from the
  AFP-V0-029 bar and the AFP-V0-030 window. A `merge_group` run SHALL always run full: it never
  reuses a narrowed run, because a narrowed shard writes no tested-tree record. `collect.sh`
  SHALL download the selection run's narrowing records, and `row.jq` SHALL classify a queue
  failure of a package a record lists as skipped as a `MISS`, even when its rerun passes, since a
  narrowed-out failure is exactly what the PR did not test. A record of another run, a malformed
  record or unreadable records SHALL make the queue row `INCOMPLETE`. Failure modes: a record
  that says `NARROWED` while the shard ran whole is still treated as narrowed (conservative);
  rows expire with default artifact retention, so an old `MISS` can drop out of a later verdict,
  which AFP-V0-030 re-arming by epoch bounds. Rollback: as AFP-V0-097.
- **AFP-V0-046:** (accepted under the owner's standing auto-accept (2026-10-11); V1-1360) A
  `merge_group` run of `.github/workflows/ci.yml` MAY reuse the passed pull-request run of the
  same tree instead of repeating the root Go race invocation, only when all of the following
  hold: the queue commit is a two-parent merge; its first parent is an ancestor of the
  checked-out `refs/remotes/origin/main`, so the entry is built directly on `main`; a completed,
  successful `pull_request` run of `.github/workflows/ci.yml` in this repository exists whose
  head is the queue commit's second parent (the PR head); and that run's AFP-V0-170
  `ci-tested-tree-record` job printed a record of the current shard count whose tree is exactly
  the queue commit's tree id (amended under the owner's standing auto-accept (2026-10-11);
  V1-2271, as for AFP-V0-024). The first entry on an unmoved `main` has
  the parents of the merge ref the PR run tested, so the trees are equal; a later entry whose
  tree includes a preceding entry differs and stays FULL. The decision SHALL be made by
  `tools/ci-reuse-plan --event merge_group`, which admits no `merge_group` run, built from the
  queue commit's first parent (so a PR's own edit to the tool cannot decide its queue run), over
  the same public listings and permissions as AFP-V0-024, and the `docs-plan` job SHALL retain
  its decision as the `ci-reuse` artifact; `head` names the PR head and `runId` the reused run.
  Only the root race invocation is omitted: `doc-gates`, `go-static`, `go-interop`,
  `artifact-integrity` and the other non-race checks still run, and `go-product` aggregates a
  REUSE shard as for AFP-V0-024. A REUSE queue run SHALL write no tested-tree record, gocache
  entry or shard cost outcome. Every failure path is FULL: a first parent not on `main`, a
  single-parent commit, an unreadable run or job listing or record job log, a failed, cancelled, unfinished,
  other-workflow or other-repository run, a missing, partial or other-tree record, a different
  shard count, a tool that does not build or exits nonzero, and a decision the workflow's
  admission check does not accept. Limits: AFP-V0-029 shadow rows (V1-0811) MUST take a REUSE
  queue run's outcomes from the reused pull-request run, because the queue run has none; the
  `ci-control-plane` status still posts on the completed `merge_group` run (AFP-V0-026); the
  change that introduces this requirement runs its own queue run with the previous tool and so
  in full; hosted observation is `NOT_OBSERVED` until the first REUSE queue run.
  AFP-V0-028 keeps every `merge_group` run unnarrowed; this exception skips the race invocation only
  for a tree a full pull-request run already passed, and a narrowed PR run writes no record, so
  its queue run still runs in full. Rollback: restore the `push`-only condition on the `reuse` step and the
  `pull_request`-only record step.
- **AFP-V0-170:** (accepted under the owner's standing auto-accept (2026-10-11); V1-2271) The
  only evidence an AFP-V0-024 or AFP-V0-046 REUSE decision accepts that a run tested a tree in full
  SHALL be one line in the log of that run's `ci-tested-tree-record` job:
  `corvint-ci-tested-tree/1 tree=TREE target=COMMIT head=HEAD shards=N`, at the start of a log line
  after at most the hosted timestamp prefix. The job SHALL run only in a `pull_request` or
  `merge_group` run after every `go-product-shard` succeeded, when `narrowing-verdict` is not
  `MET` (no shard narrowed) and `docs-plan` decided neither `DOCS` nor `REUSE`; it SHALL run no
  repository code (one `actions/checkout` with `persist-credentials: false`, then shell built-ins,
  `git` and `printf` only), and print the line only when the checked-out commit is the run's
  two-parent merge and, for a pull request, its second parent is the pull-request head. Trust
  argument: an artifact, or a line any other job prints, can be produced by repository code of
  the tested tree (a test, a build step, a fork's workflow edit), so neither is evidence; this
  job's own log can only be written by the steps of this job, whose definition is the trusted
  `ci.yml` of the very tree the reusing commit must equal, and changing it needs the AFP-V0-016
  admin review. `tools/ci-reuse-plan` SHALL read only the jobs listing of an admitted run and the
  log of its record job (`--jobs`, `--logs`) and decide REUSE only when that run has exactly one
  job of that name, it belongs to the run and its head, it completed with `success`, its log holds
  exactly one record line, and that line names the deciding tree, head and shard count (and, for a
  reused `merge_group` run, that run's own commit as target); the decision names it by
  `recordJobId`. Failure modes, each FULL: no record job (a forged artifact set, a narrowed,
  documentation-only or reused run), two record jobs, a failed, skipped or unfinished job, a job
  of another run or head, a missing, repeated, prefixed or suffixed line, another tree, head or
  shard count, an older record profile, and an unreadable listing or log. Non-goals: proving that
  GitHub runs the job as defined, and protecting against a repository administrator. Transition:
  until this tool is on `main`, a queue entry builds the earlier tool, whose flags the reuse step
  no longer passes, and runs FULL. Rollback: revert the record job, the reuse step and the tool
  together; the earlier artifact records are no longer written, so a partial rollback runs FULL.
- **AFP-V0-171:** (accepted under the owner's standing auto-accept (2026-10-11); V1-1847) When
  the `control-plane` job of `.github/workflows/ci-control-plane.yml` would post `failure` on a
  pull-request head, it SHALL first read the head's commit statuses (newest first) and post
  nothing when the newest `ci-control-plane` status there is `success` posted by a creator of
  type `User` (the admin review AFP-V0-016 requires), so a CI rerun on an already reviewed head
  keeps the review. A new push has no status on its head and still gets `failure`; a newer
  failure, a bot-posted success, another context and a pending status still get `failure`; an
  unreadable status listing stops the step, posting nothing, so the merge stays blocked. Limits:
  any user with write access can already post that status, so the rule widens nothing; a head
  with more than 100 newer statuses is not proved reviewed and gets `failure`. Rollback: remove
  the status read; reruns then overwrite the review again.
- **AFP-V0-047:** (accepted under the owner's standing auto-accept (2026-10-11); V1-0811) The
  AFP-V0-029 shadow SHALL run without the deciding driver. `go-product-shard` SHALL carry
  `CORVINT_TEST_SHADOW_SOURCE`, a full reviewed `main` commit id pinned apart from
  `CORVINT_PR_TOOL_SOURCE` (whose deciding driver, before AFP-V0-048, refused AFP-V0-041 test slices); on a
  `pull_request` run at a mode other than DOCS or REUSE, a step SHALL check out that commit outside
  the tested checkout without credentials, verify `HEAD` equals the pin, build only `./cmd/corvint`
  to `$RUNNER_TEMP/corvint-shadow-bin/corvint`, and the shadow `corvint test --dry-run` (STR-V0-012
  step 3) SHALL use that binary; the Go tests step keeps executing the full shard, so no PR run
  narrows. Each such shard SHALL retain, in its `corvint-test-shadow-<shard>` artifact beside
  `dry-run.json` and `manifest.json`, a `selection.json` document `corvint-pr-selection/0` with
  keys `profile`, `toolSource`, `toolDigest`, `pr`, `head`, `base`, `target`, `tree`, `runId`,
  `runAttempt`, `shard`, `shards`, `exit`, `scope`, `reason`, `placement`, `shardIndex`,
  `shardCount`, `incomplete`, `selected` (the shard's run set), `manifestDigest`, `manifestUnits`,
  `planDigest`, `partitionDigest`, `sourceDigest` and `notProduced`, which names every digest the
  dry-run does not print (the selection digest) instead of inventing it. Every `pull_request` and
  `merge_group` full-branch shard SHALL retain `ci-outcomes-<event>-<shard>`, a
  `corvint-ci-outcomes/0` document with keys `profile`, `event`, `runId`, `runAttempt`, `target`,
  `head`, `tree`, `shard`, `shards`, `status`, `outcomes` (package-level `pass`, `fail`, `skip`, or
  `missing` for an expected package or slice without a terminal event) and `rerun`, also after a
  failed shard. A shard whose tests failed SHALL rerun each failed package or AFP-V0-041 slice
  once under the same confinement, at most 8 invocations, recording `rerun.state` `RAN` with the
  rerun outcomes, `NOT_NEEDED`, `INTERRUPTED`, or `SKIPPED_BOUND` with no rerun above the bound;
  the shard still fails; a failed outcomes setup leaves that shard without outcomes (so its rows
  are INCOMPLETE) and never changes the shard's status. The `forward-rows` job of `.github/workflows/ci-control-plane.yml`, on a
  completed (`success` or `failure`) `merge_group` or `pull_request` run of CI, SHALL hold only
  `actions: read` and `contents: read`, run only the default-branch `.github/forward-rows` scripts
  through a sparse checkout of its own commit with `gh` and `jq`, and download only the
  `ci-outcomes-*`, `corvint-test-shadow-*` and `ci-reuse` artifacts. A queue row takes S from the
  latest successful `pull_request` run of the queue commit's second parent and F from the queue
  run, or, for a REUSE queue run (AFP-V0-046), from the reused run its `ci-reuse` decision names,
  recording `outcomesSource` `reuse` (else `merge_group`, or `pull_request` for a PR row); a PR
  row takes both from its own run and names its tested merge commit. A run on a branch that is not
  a queue branch of `main`, without a pull request number, or a PR run that retained neither
  outcomes nor a shadow plan produces no row. Because a change to `.github/` runs its own
  workflow and could upload any outcome, a run whose change set (the pull request's base to head,
  or the queue commit's first parent to the commit, read through the compare API) touches a
  `.github/` path, or lists 300 files or more, also produces no row; an unreadable change set
  makes the row INCOMPLETE.
  The row SHALL be INCOMPLETE, never CLEAN, when any selection or outcomes shard is missing,
  unreadable, misshapen (including a tool source, head or target that is not a full commit id, or a
  tool, manifest, plan or partition digest that is not `sha256:` hex) or duplicated, when a shard count differs from the job's
  `FORWARD_SHARDS`, when a document names another run, head or target than the row (or, for a
  REUSE row, another tree than the decision), when the
  dry-run exited other than 0 or 3, when shards disagree on the manifest digest, when the selection
  records a gap other than a non-Go language without an adapter (retained in `selection.limits`),
  when a failed shard names no failed package, or when the collector could not read an input.
  The row is `corvint-pr-forward-row/0` with the closed keys `class`, `counts`, `epoch`, `failed`,
  `flaky`, `head`, `incomplete`, `infrastructure` (AFP-V0-052), `kind`, `missed`, `outcomesRunId`, `outcomesSource`, `pr`,
  `profile`, `reruns`, `runId`, `selectedFailedInQueue`, `selection`, `selectionRunId`, `sha`,
  `shards` and `truncated`, at most 64 KiB (package lists are cut and `truncated` set), uploaded
  with default retention as `ci-forward-<CLASS>-pr<N>-<sha>-e<EPOCH>-f<0|1>`, where `EPOCH` is
  `CORVINT_PR_FORWARD_EPOCH` and `f1` marks a row with failing packages. The job's step summary
  SHALL count, from the names of non-expired artifacts that runs on `main` of this repository
  uploaded (a pull-request or queue run can upload any name) and without downloading, two
  separate counts of the current epoch's rows (one per pull request and commit): every listed row
  against the AFP-V0-029 bar of 20 complete and 5 failing rows with no non-`FLAKY` miss, and the
  latest 20 rows against the AFP-V0-030 thresholds. It SHALL say NOT MET while the epoch is short
  and NOT ESTABLISHED, never MET, when the listing neither read every artifact nor reached a row of
  an earlier epoch (owner chat, 2026-10-20; V1-0812). Nothing in
  this requirement narrows a run or changes a required check. Limits: hosted row delivery is
  `NOT_OBSERVED` until the first queue run on `main` after the change; the rerun bound leaves a
  ninth failure without a rerun, so its misses are MISS; artifact listing reads at most 20 pages;
  shadow artifacts keep 14-day retention, so a queue run later than that is INCOMPLETE; fork pull
  requests produce no row; a pull request's own test code runs in its shards and could rewrite its
  outcomes files, so a row is as trustworthy as the reviewed change, like any test result.
  Rollback: clear `CORVINT_TEST_SHADOW_SOURCE` (the shadow and its
  selection stop) and remove the `forward-rows` job; no consumer reads the rows before V1-0812.

- **AFP-V0-048:** (accepted under the owner's standing auto-accept (2026-10-11); V1-1361) A
  sharded AFP-V0-022 run of the package-level PR driver `tools/corvint-pr-tests` SHALL execute
  exactly what the protected full path places on its shard. It SHALL take the complete placement
  of the enumerated runtime universe from the same `cishards.Plan` call that `ci-shards --slices`
  makes, never a reimplemented partition. It SHALL run the shard's whole packages in one `go test`
  invocation and then each AFP-V0-041 slice placed on the shard in its own invocation (the driver's
  test arguments, the slice's `-run` or catch-all `-skip` pattern, then the package), all within
  one deadline, and the shard SHALL fail when any invocation fails. A FULL run executes the
  shard's whole placement. A narrowed run executes the selected whole packages placed on the
  shard and, for each selected split package, every slice placed on the shard, so over all shards
  a selected split package runs each slice exactly once and never whole; a selected package
  outside the runtime universe, or repeated, refuses. Before any execution the driver SHALL
  refuse, with exit 2, a reason naming `AFP-V0-048` and no execution record, a placement that does
  not have the declared shard count, places a package other than exactly once whole or as
  slices, runs a split package whole as well, places a slice other than exactly once, gives one
  package two slice counts, or holds a slice whose index, flag (`-run` for a named slice, `-skip`
  for the catch-all) or pattern (an anchored alternation of plain `Test`, `Example` or `Fuzz`
  names within `MaxPatternBytes`) is malformed, or whose named slices of one package overlap or
  whose catch-all does not skip exactly their union. A FULL fallback executes the same placement,
  so it refuses too. The AFP-V0-022 profile digest, shard source and source drift checks still
  precede execution. A slice file whose names are stale needs no refusal: as in the full path, a
  removed name matches nothing and a new or renamed test runs in the catch-all slice; an unusable
  or unadmitted slice file leaves the affected package whole, as in the full path.
  `selection.json` SHALL record the shard's slices (`ShardSlices`) and `execution.json` each slice
  invocation's arguments and exit (`Slices`); with no split package both documents and the
  whole-package arguments are unchanged. Non-goals: selection authority from slices, slicing in
  qualification rows or the container profile (both stay unsharded), and a change to the
  protected helper or workflow. Limits: like the driver's whole-package invocation, each slice
  invocation runs without the AFP-V0-023 `test-confine` wrapper that the full path applies (the
  AFP-V0-023 residual for the AFP-V0-013 driver; AFP-V0-028 requires the full branch's wrapper
  before narrowing); hosted execution is `NOT_OBSERVED` until V1-0812 sets
  `CORVINT_PR_TOOL_SOURCE`, and the `.github/workflows/ci.yml` comment beside that pin still
  describes the earlier refusal until V1-0812 edits it. Falsifier: a driver shard set whose union
  of whole packages and slices differs from the full path's for the same universe and shard
  count, or a top-level test of a selected split package with zero or two terminal outcomes over
  the shards. Rollback: revert the change, which restores the AFP-V0-041 (6) refusal.

- **AFP-V0-049:** (accepted under the owner's standing auto-accept (2026-10-11); V1-1371)
  `tools/ci-test-slices generate` and `replay` SHALL combine the per-slice terminal outcomes of a
  split package as AFP-V0-041 (8) does for `tools/ci-shard-costs refresh`, so a complete, passing
  sliced run can regenerate the slice file and be replayed. The split set SHALL be the one the
  tested tree ran with: `generate` reads the allow-list and slice file of the `--revision`
  checkout and `replay` those of `--root`, through the same `SplitPackages` call over the
  packages the logs name and the declared `--shards N`; `replay --slices` chooses only the
  placement it predicts.
  (1) A package the files split into K slices SHALL have exactly K terminal outcomes across the
  logs. Its time is the sum of those outcomes and its top-level test times the union of theirs.
  (2) Each outcome SHALL be attributed to one slice by its top-level tests: every top-level test
  of the outcome SHALL belong to the slice that admits the first of them in byte order (the
  catch-all admits every name that no named slice names), and no slice SHALL be attributed twice.
  An outcome without top-level tests, such as a named slice whose tests the run no longer has,
  counts toward K without a name.
  (3) Every other AFP-V0-041 (5) refusal SHALL be kept, with exit 2 and no write: a repeated
  outcome of a package the files do not split, including every package when the allow-list or
  slice file is absent or unusable ("package P has two terminal outcomes"); a top-level test
  with two terminal outcomes across the run; a test outside the slice its log ran; a slice
  attributed twice; any count other than K, so a run made before the split or a run missing a
  slice refuses ("N terminal outcomes for its K test slices"); and an allowed package that the
  slice file splits into more slices than N.
  (4) `replay` SHALL state that its sliced plan places every package of the run exactly once,
  whole or as each of its slices ("units U placed exactly once"), and SHALL refuse a plan that
  drops, repeats or adds a unit.
  Non-goals: attributing subtests; changing which packages split; cost-table changes.
  Limits: an outcome without top-level tests is attributed by count only, so which named slice it
  was is not checked; the per-log lifecycle rule of AFP-V0-041 (5) is unchanged, so two slices of
  one package in one shard log still refuse; the exactly-once line checks the helper's plan, not
  the hosted run. Falsifier: an accepted run in which a top-level test has two terminal outcomes,
  a slice runs twice, or a package the files do not split ends twice. Rollback: revert `combine`
  and `placedOnce` in `tools/ci-test-slices/main.go`; `generate` and `replay` then refuse a sliced
  run as before, and the committed slice file stays valid.
- **AFP-V0-050:** (accepted under the owner's standing auto-accept (2026-10-11); V1-1298; (3)
  and (4) amended by the owner, decision 0528, V1-1399, V1-1400; (3) reworded by the owner, decision 0529, V1-1425) When
  `.corvint/test-read-scopes.json` is among the changed paths, `corvint affected` SHALL read
  the declaration the change is measured from, at the `--base` commit when one is given and at
  HEAD otherwise, from verified immutable Git objects and never the worktree; a commit that holds no declaration is an empty
  declaration. Against that base, the declaration path SHALL bear, with witness
  `DECLARED_READ_SCOPE`, only on (1) a Go package whose own key was added, whose entries changed,
  or whose key was removed, and (2) a declared package whose subtree or declared entries hold the
  declaration path, as AFP-V0-023 states for any other dirty path. A package whose declaration
  is unchanged is no longer selected by the edit alone, so an edit that only adds keys selects
  the packages it declares, which a new package's own source edit already selects.
  (3) Fail closed: a base commit or base declaration that cannot be read, is oversized or is
  invalid, or that declares a directory holding no Go package at the base, is unknown. A declared
  directory holds a Go package at the base only when the nearest module that the base's root
  `go.work` lists (the root module alone when there is no `go.work`) owns it, not a nested,
  unlisted or testdata `go.mod` (an unlisted `go.mod` that is a regular file or a symlink
  counts; a directory named `go.mod` is no manifest); the listed owner's own `go.mod` is not
  checked, as the observer keeps a listed module whose manifest is missing and raises
  `FrontierModulePath`; the
  observer's walk reaches it (no hidden or `affected.SkippedDirectories` component, unless the
  Go plugin admits it); it lies outside its owner's testdata; and it directly holds a regular
  `.go` file. For an unknown base the AFP-V0-023 rule applies unchanged (every declared package bears on
  the edit); an invalid declaration in the observed tree still declares nothing and raises
  `go:test-read-scopes-invalid`, so every package the base declared counts as removed and is
  selected. Rules (a) to (d), every other dirty path, and the readers that name the declaration
  by literal (rule (c)) are unchanged.
  (4) The AFP-V0-012 selector (`internal/gateselect`, run by `tools/gate-affected-select` in the
  fast tier and by `corvint test` for its Go selection) SHALL narrow the same way, printing
  `declared <pkg> <- .corvint/test-read-scopes.json` only for the packages (1) and (2) name. It
  reads the base declaration at the plan's `range.base`, else its `revision`, which must be a
  full commit id, from the base commit's Git objects with the same existence and ownership checks;
  any unknown, including a `--snapshot` plan, keeps the AFP-V0-023 rule. A package whose key was
  removed is selected only while it is still a package in the observed tree; one that is not is
  out of the selector's reach, as for any other dirty path.
  Non-goals: the `--snapshot` form keeps the AFP-V0-023 rule; `.corvint/unbounded-readers.json` is a ratchet
  record (AFP-V0-025), not a selection input, and is unaffected.
  Limits: the base is the commit the range is diffed from (`git diff BASE HEAD`), so a base that
  is not an ancestor compares two unrelated declarations, which is still sound because any
  difference selects; the rule-(d) floor and rule-(c) readers of the edited files remain and
  dominate the residual selection. The selector reads the base with the hermetic `git ls-tree`
  and `git cat-file` plumbing (no system or global configuration, no replace objects, no lazy
  fetch) and, unlike the planner, does not re-hash the objects it reads; `internal/gateselect`
  stays standard-library only because `tools/gate-ledger` builds it in isolation.
  Falsifier: an accepted plan, or a selector run, over a changed declaration that omits a Go
  package whose parsed entries differ between the base and the observed tree, or a narrowed plan
  or selection when the base declaration could not be read. Rollback: remove the
  `WithReadScopeBase` call in `compileAffectedGraph`, and the `baseReadScopes` call in
  `internal/gateselect` `selectPackages`; a graph or a selector without a base selects every
  declared package on a declaration edit, as before.
- **AFP-V0-051:** (accepted under the owner's standing auto-accept (2026-10-11); V1-1351) A local
  check, `make test-read-scopes-check` (`tools/test-read-scopes-check`), MUST exit 1 and name the
  package, the test file and the repository path when a `_test.go` file of a package declared in
  `.corvint/test-read-scopes.json` (AFP-V0-023) names, as a string literal, a tracked repository path
  outside that package's read scope, so an undeclared read fails before the confined CI shard denies
  it with an error that looks like a content failure (V1-1340). (1) Literals are the AFP-V0-021
  lexicon of each test file, read by the same scanner the planner uses (`golang.FilePathTokens`),
  never a second parser. (2) A literal names a path when, joined to the package directory, or, if it
  has no `.` or `..` component and some test file of the package locates the repository root (a
  parent-only literal or the Git root lookup, the AFP-V0-021 narrowing), taken from the root, it is a
  Git-tracked path of two or more components, compared case-exactly. A `/`-anchored literal names
  nothing here, and neither does the shared change-evidence map `.corvint/change.cem.json`, which is
  tracked only between a dogfood bind and the seal that moves it (`DCW-V0-027`) and which tests name
  for the fixture repositories they write. (3) A named path is in scope when it is the package directory or below it, is or lies
  under a declared entry, or is a directory above the package or above an entry, which a test climbs
  through or joins further components onto. (4) A literal that is fixture content, an expected
  message, or the read of a test the confined shard never runs is recorded with its reason in the
  closed record `.corvint/test-read-scope-literals.json` (profile
  `corvint-test-read-scope-literals/0`, `{"profile", "literals": {PACKAGE: {PATH: REASON}}}`, every
  reason non-blank); a recorded path is not reported, and a recorded path no test file names any more
  is reported on stderr without failing, as the AFP-V0-025 ratchet treats a stale entry. (5) An
  invalid AFP-V0-023 declaration or an invalid record exits 2. (6) The check is a step of `make gate`
  and of the `doc-gates` job of `.github/workflows/ci.yml`, which reports beside the confined
  `go-product-shard` jobs rather than gating them. (7) (V1-2117) Each run of two or more adjacent
  string-literal arguments of a `filepath.Join` or `path.Join` call in a test file is joined with `/`
  and treated as one more literal, resolved against the package directory only (never from the root,
  since fixture repositories join the same segments onto temporary roots), so
  `filepath.Join(filepath.Dir(file), "..", "..", "protocol", "cem-1.0", "stable", "neutral-criterion")`
  names `protocol/cem-1.0/stable/neutral-criterion`. (8) (V1-2037) When a test file builds a
  `go list` command with `exec.Command` or `exec.CommandContext`, its package patterns (literal
  arguments after `list` that are `.` or start with `./` or `../`, resolved against the package
  directory or, when no directory is there, the root; `.` when none is given) are listed with
  `go list -deps`, and every repository directory of a non-standard package it loads MUST be the
  package directory, below it, or under a declared directory entry (a directory above an entry is not
  enough, since `go list` reads the directory's own files); otherwise the check exits 1 naming the
  package, the test file and the directory, and a pattern that names no repository directory, or a
  `go list` that fails, exits 2 with go's stderr.
  Non-goals: it does not prove a read: a literal built at run time, a path read through a variable
  joined onto a non-literal root, an environment value or a file a helper package opens is invisible to
  it, and the Landlock wrapper (`.github/testconfine`) stays the enforcement; it never edits the
  declaration and does not change planner selection.
  Falsifier: the current tree with the `protocol/cem-1.0/stable/tamper-matrix/` entry of
  `internal/cem/verify` removed passes the check. Rollback: remove the `test-read-scopes-check`
  target from `make gate` and from the `doc-gates` step list, and delete the tool and the record.
- **AFP-V0-052:** (accepted under the owner's standing auto-accept (2026-10-11); V1-1514; the
  AFP-V0-029/030 amendment it implements is owner-accepted, decision 0537) A forward row SHALL tell
  a hosted pre-step failure apart from runner incompleteness. (1) The `forward-rows` collector
  SHALL read the job list of the outcomes run (`filter=latest`) and name each `go-product-shard (N)`
  job whose conclusion is `failure` and whose first step, `Set up job`, failed; an unreadable job
  list names none and says so in the step summary. (2) The row SHALL carry `infrastructure`, one
  reason `infrastructure: go-product-shard (N) of run R failed at Set up job` per named shard, also
  listed in `incomplete`. (3) The class SHALL be `INFRA`, not `INCOMPLETE`, only when the named
  shards are below the shard count, the outcomes documents cover exactly the other shards, the
  selection documents cover every shard or, when the selection run is the outcomes run (a
  pull-request row), exactly the other shards, every other incomplete reason is absent, and no
  missed package failed its rerun; otherwise the row stays `INCOMPLETE`, so a real gap beside a
  hosted failure is never hidden. A hosted failure therefore never makes a row `CLEAN`, `FLAKY` or
  `MISS`. (4) The row name SHALL use `INFRA`, and the step-summary bar count SHALL accept it, give an
  `INFRA` row no place in the latest-20 window, never count it as complete or `INCOMPLETE`, and
  report how many `INFRA` rows lie between the window's rows.
  Non-goals: it does not rerun the lost shard, change a required check, or classify any other job or
  step; a later step's failure, a cancelled job and a runner lost mid-job stay `INCOMPLETE`.
  Limits: the selection run of a queue row is the latest successful pull-request run, which has no
  failed job, so only the outcomes run is read; a queue run whose every shard failed at set-up
  retained no outcomes artifact and stays `INCOMPLETE` on the collector's reason; hosted `INFRA`
  rows are `NOT_OBSERVED` until a hosted set-up failure recurs after the change.
  Falsifier: the queue-row fixture of run 38686348902 (shard 5 lost at `Set up job`, its real job
  list) classifies `INCOMPLETE`, or a row with a missing shard that no set-up failure names classifies
  `INFRA`.
  Rollback: revert the `infra` reading in `collect.sh`, `classify.sh`, `row.jq` and `bar.jq` and
  decision 0537; earlier `INFRA` artifacts then match no bar name and are not counted.

- **AFP-V0-053:** (accepted by the owner (2026-10-16; ticket ci-shadow-historical-compat))
  Historical replay Git compatibility. (1) The AFP-V0-014 frozen identity SHALL include the Git
  executable that tests resolve under the `go test` environment: its resolved path, SHA-256 and
  `git --version` output. On Darwin, when that executable is the `/usr/bin/git` xcrun shim, the
  identity SHALL also bind the resolved path and SHA-256 of the Git that `xcrun --find git` names
  under the closed environment (V1-1521). Execution MUST refuse when Git cannot be identified, and Git drift
  before execution MUST fail safely like Go/compiler drift. (2) Git configuration inputs are bound
  through the frozen environment, not a separate field: `GIT_CONFIG_NOSYSTEM=1`,
  `GIT_CONFIG_GLOBAL=/dev/null`, a private `HOME`, `GIT_NO_REPLACE_OBJECTS=1`,
  `GIT_GRAFT_FILE=/dev/null` and the `GIT_CONFIG_COUNT` entries. The `go test` environment SHALL
  equal the driver's closed environment except for one entry, `advice.graftFileDeprecated=false`,
  so tests keep the graft refusal while Git prints no graft advice into the output they parse.
  (3) A corpus, row or qualification frozen under another identity, including one frozen before Git
  joined it, MUST NOT be resumed, reused or relabelled under the current identity; it needs a fresh
  freeze and campaign. (4) A failure observed under a superseded profile stays retained as observed.
  It is classified as a compatibility failure, not a change-caused omission, only by a controlled
  replay at both pair commits that varies the profile input alone; that classification never
  waives an omitted failing package under AFP-V0-014, never names packages as a safety proof, and
  says nothing about the selector's sufficiency for any other pair.
  Non-goals: Git's exec-path helpers and templates are not hashed (the version string stands for
  them), and the developer directory that xcrun consults is identified only through the Git it
  resolves; the hosted profile is Linux. It qualifies no selective CI.
  Falsifier: with the resume identity check removed from `shadow`, a corpus frozen without a Git
  identity is resumed. Rollback: remove this requirement; the implementation notes below still
  describe the shipped driver.

- **AFP-V0-054:** (accepted under the owner's standing auto-accept (2026-10-11); V1-1719; no
  GitHub issue) Local Windows vet before push. (1) The Makefile target `windows-vet` SHALL run
  exactly the `go vet` commands of the AFP-V0-042 step, in the same order and module directories
  (the root module, then `interop/cem01-go`), with the same file-selecting environment as the
  step's effective environment (the `go-static` job env with the step's own `env` merged over it):
  `CGO_ENABLED=0 GOOS=windows GOARCH=amd64`, `GOTOOLCHAIN=local`, `GOWORK=off` and an empty
  `GOFLAGS`, so no build tag is added and `_test.go` files are type-checked. It SHALL also set
  `GOENV=off`, so a value saved by `go env -w` (which an empty `GOFLAGS` falls back to) cannot
  change the selected files, and SHALL depend on the `go-version` target, so it runs only on the
  `go1.27.2` toolchain CI's `setup-go` pins. (2) The
  `AGENTS.md` Verify section SHALL declare `make windows-vet` in a shell fence and scope it, in a
  sentence naming the command, to a change that touches a `.go`, `go.mod` or `go.sum` file, so
  `corvint affected` lists it as a `conditional` check (AFP-V0-009, decision 0531) and an agent
  runs it before the push that CI would otherwise be first to judge. (3) A test that calls a helper
  defined only in a `//go:build unix` file (the PR #1089 shape, V1-1092) SHALL pass the native
  `go vet` and fail the target's root-module command naming the undefined helper.
  Design: the whole-module run, not a touched-package subset, is the smallest exact match. It is the
  CI command itself; a subset would miss a break in a package whose files did not change (a
  dependent's test, or a helper moved under a constraint) and would need a package resolver CI does
  not use. It costs about 5 s warm and 45 s cold (build log 2026-10-10-windows-test-vet; 19 s
  observed on this change). The declaration is project-owned authority (invariant 3): no
  `corvint affected` code changes, so other repositories see no new advice.
  Non-goals: no `.github/` change, no skip logic mirroring the step's pull-request condition (the
  scoping sentence carries it), no pre-push hook, no `GOOS=linux GOARCH=arm64` run (it stays in
  `cross-vet`), and no new `GATE_STEPS` entry (`cross-vet` already covers Windows in `make gate`).
  Failure modes: a target that drifts from the step (another GOOS, a tag in `GOFLAGS`, a dropped
  module, a dropped `GOENV=off` or `go-version` prerequisite) fails
  `TestAFPV0054WindowsVetTargetMatchesTheCIStep`; a tag added in the step's own `env` fails it too
  (`TestAFPV0054StepEnvOverridesJobEnv`); a saved `go env -w GOFLAGS` is ignored
  (`TestAFPV0054WindowsVetIgnoresSavedGoEnv`); an agent that skips the check is
  still stopped by the unchanged AFP-V0-042 step, one CI round later. Falsifier: the fixture of (3)
  passes `make windows-vet`'s root-module command. Acceptance evidence: the five `TestAFPV0054*`
  tests, mutation checks and a `corvint affected` run on this tree listing the conditional check
  (build log 2026-10-18-v1-1719-local-windows-vet). Rollback: delete the target, the `AGENTS.md`
  paragraph and fence, and the tests.

- **AFP-V0-060:** (accepted by the owner in owner chat, 2026-10-18, which also gave AFP-V0-016
  consent for the `.github/` edits of V1-1096 and V1-0812 only and accepted the V1-1096 amendment
  of AFP-V0-022 that AFP-V0-041 states; V1-1096; no GitHub issue) Production test-name slices of
  the oversized allowed packages. (1) The committed `.github/cishards/test-slices.json` SHALL split
  both allowed packages, `cmd/corvint` and `internal/tasks/store`, and SHALL be regenerated by
  `tools/ci-test-slices generate --target` of at most 350s from one complete, passing hosted run
  of the full go-product-shard matrix, with `package-costs.json` refreshed by
  `tools/ci-shard-costs refresh` from the same run, so both files record one revision and run.
  (2) Over the committed cost table's package universe and the 8-shard matrix, every planned slice
  of a split package SHALL carry at most 400,000 ms, the plan SHALL place every unsplit package
  and every slice exactly once, and sibling slices SHALL land in different shards. (3) Every test
  name the slice file records SHALL be selected by exactly one slice of its package; a name added
  or renamed after generation SHALL be selected by the catch-all alone, and a removed name by
  none. The AFP-V0-041 refusals and whole-package fallbacks, the AFP-V0-022 complete-universe
  partition, the full-suite merge-queue run and the required checks `go-product`, `doc-gates` and
  `ci-control-plane` are unchanged.
  Non-goals: no new allow-list entry (`internal/tasks/cli` and `internal/tasks/authority` stay
  whole); no PR narrowing or selective execution (V1-0812 stays behind the AFP-V0-029 bar); no
  claim of hosted speedup from local replay alone.
  Failure modes: a refresh from a failed, partial or repeated run is refused by the tools, as
  AFP-V0-022 and AFP-V0-041 (5) state; a slice file that no longer decodes or that a package
  outgrows keeps the package whole (AFP-V0-041 (3)); with fewer shards than a package's slices the
  package runs whole.
  Acceptance evidence: `TestAFPV0060CommittedSlicesBoundOversizedPackages` in
  `.github/cishards/slices_test.go`; the per-shard Go-test step times and the largest
  `cmd/corvint` slice of this change's hosted run; ten paired sliced and unsliced runs with an
  identical pass/fail set (`NOT_OBSERVED` until collected). Falsifier: with the bound lowered to
  300,000 ms the test fails on the largest `internal/tasks/store` slice.
  Rollback: revert the two JSON files to their previous revision; placement returns to the
  previous slices and costs with no other change.
- **AFP-V0-066:** (accepted under the owner's standing auto-accept (2026-10-11); V1-1796; GitHub
  #1130 item 4) Advisory RSpec check. This amends AFP-V0-009's single advisory entry. (1) When
  `plan.selected` holds a unit whose id begins `ruby:rspec:`, `advice.checks` MUST gain one
  `advisory` entry with source `affected-plan`, placed after the Go advisory entry when there is
  one, whose command is `bundle exec rspec` followed by the sorted, deduplicated `tests` files of
  those units, each prefixed `./` so a dash-leading file name is never read as an RSpec option and
  POSIX single-quoted as AFP-V0-009 quotes packages, and whose reason counts the RSpec units. (2) The entry exists only when the working-tree root holds a regular-file `Gemfile`,
  observed by `lstat`; otherwise `unknown` gains
  `NO_ADVISORY_RSPEC_COMMAND: the plan selected RSpec units but the repository root has no Gemfile`.
  Amended for V1-1828 under the owner's standing auto-accept (2026-10-11), the entry also needs
  `rspec`, `rspec-core` or `rspec-rails` declared by the `NGI-V0-052` check: a `gem` line of the root
  `Gemfile`, or a two-space `DEPENDENCIES` or four-space specs entry of the root `Gemfile.lock`, each
  read only as a regular file of at most 256 KiB, so an absent, oversized or unreadable manifest
  declares nothing. Otherwise, as for a Minitest repository with a `spec/` directory, `unknown` gains
  `NO_ADVISORY_RSPEC_COMMAND: the plan selected RSpec units but the root Gemfile and Gemfile.lock declare no rspec gem`.
  (3) More than 1024 distinct spec files yield no entry and
  `NO_ADVISORY_RSPEC_COMMAND: <n> selected spec files exceed the 1024-file bound`. (4) A plan that
  selects no RSpec unit gains nothing, so every receipt without one keeps its exact bytes; the Go
  advisory entry, its `NO_ADVISORY_GO_COMMAND` line and the AFP-V0-038 compact form are unchanged,
  and the RSpec command is never compacted.
  Non-goals: no Minitest or `rails test` command, no Ruby `provider` projection (`provider` keeps
  only `go`, and this is advice, not a provider package list), no execution, no Gemfile read beyond
  the bounded gem-declaration check, and no change to `corvint test`.
  Failure modes: a plan whose Ruby frontier widens scope (for example `ruby:rails-autoload`) still
  lists only the selected files; the unknown frontier says the list may be incomplete, and mandatory
  checks stay required (AFP-V0-004). A root Gemfile and Gemfile.lock that declare no `rspec`,
  `rspec-core` or `rspec-rails` gem get no advisory command and the `NO_ADVISORY_RSPEC_COMMAND`
  line instead, even when the selected units were classified as RSpec by their own source; a
  manifest that is a symlink, FIFO or other non-regular file, is swapped during the read, or exceeds
  256 KiB declares nothing.
  Acceptance evidence: `TestAFPV0066AdviceListsSelectedRSpecFiles`,
  `TestAFPV0066NoRSpecCommandWithoutGemfile`, `TestAFPV0066RSpecAdviceOrderBoundAndScope`,
  `TestAFPV0066DashLeadingSpecFileIsNeverAnOption`,
  `TestAFPV0066RSpecAdviceNeedsDeclaredRSpecGem`, `TestAFPV0066SymlinkedGemManifestDeclaresNothing`, and a
  `corvint affected` run on the GitHub #1130 reproduction (build log
  2026-10-18-v1-1796-ruby-verification-command). Rollback: delete `advisoryRSpecCheck`, its
  constants and the tests.

- **AFP-V0-072:** (accepted under the owner's standing auto-accept (2026-10-11); V1-1806; no
  GitHub issue) File-name-only Go readers must place the path. AFP-V0-021 selects a unit whose
  path token names a dirty path, and V1-0290 lets an unanchored lone token without `..` name only
  the path's file name. (1) For a Go unit and any dirty path except the CEM sidecar (whose own
  narrowing is unchanged), when every token that names the path does so only through such a lone
  file-name run, the unit SHALL be a `PATH_LITERAL_READER` of that path only when the path lies in
  the unit's own directory subtree (or the unit is at the repository root), or the path lies under a
  base the unit's literals can reach and every directory component of the path below that base is
  a whole component of one of the unit's tokens (so literals such as
  `filepath.Join(root, "docs", "specs", "README.md")` could assemble it) or a component the Go
  lexicon cannot keep in one token (a space, a non-ASCII letter or any other character outside
  `[A-Za-z0-9._~@+-]`), which no token can witness. The bases are the repository root and, when
  any token has a `..` component (`filepath.Join("..", "model.go")`), every ancestor of the unit's
  directory. (2) A token that places a directory SHALL keep the component-run relation: a run of
  two or more components (with its fragment outer components), an anchored token, or a climbing
  token. (3) A non-Go unit SHALL keep the component-run relation, because its tests' working
  directory is unknown. (4) Only the reader relation changes: the unit frontier (V1-0289) of a
  unit that (1) keeps out of the plan is still named, so no unknown entry is added or removed and
  the plan's scope is unchanged; rule (d) unbounded readers, declared read scopes (AFP-V0-023),
  dependency and binary edges are unchanged.
  Design: a Go test runs in its package directory. A bounded unit (not a rule (d) reader) reaches
  another repository directory only through a literal that names it, which (1) and (2) keep, or a
  root supplied at run time, which no literal index bounds (AFP-V0-021). A lone `README.md` joined
  with `t.TempDir()` is the dominant false reader: on the PR #1134 range (`70a21da5d..5c57314aa`)
  it selected 7 of 52 path-literal readers through `docs/specs/README.md` alone.
  Non-goals: anchored lone tokens (`root + "/corvint"`), shell and other non-Go readers, fragment
  matches of multi-component runs (`"ecs/aff"`; short outer components the literal delimits are
  V1-1818's AFP-V0-021 amendment), readers of `.corvint/changes/` sealed CEMs, the
  binary data-literal edges of V1-1006, and any symbol-level narrowing of a changed constant (a
  schema-ID bump keeps the package's reverse-dependency closure).
  Failure modes: a Go test that reaches a repository directory through a run-time root naming
  the directory itself (an environment variable or flag set to `docs/specs`) and then a lone
  file name is no longer selected; this is the AFP-V0-021 run-time-root gap, and the plan's scope
  stays as before. A directory name assembled from fragments (`"spe" + "cs"`) is not a whole
  component and does not place the path. A narrowed unit whose frontier vanished would turn an
  `UNKNOWN` plan `BOUNDED`; the narrowed set feeds the frontier, as reached units do. Falsifier: a file-name-only fixture reader under
  `t.TempDir()` is still selected for `docs/specs/README.md`, or a reader that joins `"docs"`,
  `"specs"` and `"README.md"` is dropped. Acceptance evidence:
  `TestFileNameOnlyReadersMustPlaceThePath_AFPV0072`,
  `TestFileNameOnlyReadersKeepWhatTheyCanReach_AFPV0072` (a `..` reader, a spaced directory, and
  the narrowed unit's frontier), `TestNonGoFileNameReaderIsNotNarrowed_AFPV0072`, a mutation that
  restores the old relation failing the first and one mutation per clause failing the second, and the before/after plans on the
  PR #1134 range (build log 2026-10-18-V1-1806-affected-overselection-1806). Rollback: make
  `resolves` return true for every path but the CEM sidecar and delete the two tests.
- **AFP-V0-079:** (accepted under the owner's standing auto-accept (2026-10-11; V1-1778/V1-1642); no GitHub issue) The
  `control-plane` and `merge-group` jobs of `.github/workflows/ci-control-plane.yml` SHALL start
  for every completed `CI` run of their event whatever its conclusion, `cancelled` included, and
  each SHALL hold the job concurrency group `ci-control-plane-<event>-<head sha>` (event
  `pull_request` or `merge_group`) with `cancel-in-progress`, so a newer completion for the same
  event and head replaces a queued or running job. Both jobs compute their status from the
  current pull requests at the head or the queue branch name, never from the run, so the
  replacing job posts what the replaced one would, for every open pull request at that head, and
  a superseded pull-request head posts nothing because no open pull request has it at its head.
  The `forward-rows` job keeps one run per completed `success` or `failure` run and no group
  (AFP-V0-047). A `ci:batched` constituent's pull-request jobs are kept: `ci-control-plane` is a
  required context, and the `workflow_run` payload carries no labels, so no job-level condition
  can read the label.
  Design: in the window of build log 2026-10-18-ci-waste-1778-1642, 49 of 293 control-plane
  workflow runs started a status job for a cancelled `CI` run (median queue wait 296 s each, 5.0
  runner-minutes). Skipping them was rejected after review: GitHub can cancel a run after its
  `go-product` passed while optional jobs still ran, and a skipped status job then leaves a head
  with no `ci-control-plane` status and no later run to post one. The payload carries no job
  results, so no job-level condition can tell that case apart, and a check inside the job would
  still start the runner; the rule therefore fails toward reporting.
  Failure modes: a job cancelled between two pull-request posts leaves the rest to the replacing
  job. A status posted for a cancelled run grants nothing by itself: `go-product` is a separate
  required context.
  Acceptance evidence: `TestAFPV0079ControlPlaneReportsEveryCompletedRun` (the job conditions,
  evaluated for every event and conclusion) and `TestAFPV0079CancelledRunStillPostsTheStatus`
  (each status step, with a stub `gh`, posts one status on a cancelled run's head that had none);
  hosted behavior `NOT_OBSERVED` until this change runs on `main` (the workflow runs from the
  default branch).
  Rollback: remove both `concurrency` blocks.
- **AFP-V0-080:** (accepted under the owner's standing auto-accept (2026-10-11; V1-1778/V1-1642); no GitHub issue) A `push`
  run of `.github/workflows/ci.yml` on `refs/heads/main` SHALL skip `go-static`, `go-interop` and
  `artifact-integrity` only when its `static-reuse` job decides `REUSE`, which it SHALL do only
  for a run of `.github/workflows/ci.yml` in this repository, listed by
  `head_sha=<pushed commit>&event=merge_group&status=success`, whose `head_sha` is the pushed
  commit, event `merge_group`, conclusion `success`, `head_branch` beginning
  `gh-readonly-queue/main/` and numeric id, among at most five such runs, and whose latest jobs
  (`filter=latest`) include exactly one job of each of the three names, each concluded
  `success` on the pushed commit. With merge method `MERGE` the queue commit is the pushed
  commit (AFP-V0-024, V1-0760), so the tree, the job definitions, the `Makefile` and the pinned Go
  version are the ones that passed. The step SHALL read only the public run and job listings,
  with the token and then anonymously, under the workflow's existing `contents: read`, check out
  nothing and download nothing, be `continue-on-error`, and write its decision and the reused
  run id to the job summary. Every other outcome (any other event or ref, no such run, a failed,
  cancelled or other-branch run, a missing, duplicated, skipped or failed job, an unreadable or
  malformed listing, a step failure or timeout) leaves the decision empty and the three jobs run
  in full. `go-product` SHALL accept a skipped `go-static` only on a `push` whose decision is
  `REUSE`; otherwise `go-static` must succeed, and the shard condition is unchanged. Pull-request
  and merge-queue runs never run the decision, so the exact tree that becomes `main` is still
  tested in full by its queue run (AFP-V0-026, AFP-V0-046 unchanged).
  Design: every one of the 47 `main` push runs in the build-log window had a passed queue run on
  the same commit; the three jobs cost 610 runner-minutes there (13.0 per push) and repeated
  that run.
  Failure modes: a runner image, host package or module-proxy change between the queue run and
  the push goes unobserved on the push, as AFP-V0-024 already limits the race invocation; a
  queue run whose non-required `go-interop` or `artifact-integrity` failed concludes `failure`,
  so its push runs them in full; a listing outage costs only the skipped saving.
  Acceptance evidence: `TestAFPV0080WorkflowWiresPushStaticReuse`,
  `TestAFPV0080StaticReuseDecidesOnlyForAPassedQueueRunOfThisCommit` (the extracted step with a
  stub listing: REUSE for a passed queue run and after a refused token; empty for another commit,
  event, conclusion, workflow, repository, branch or id type, an unreadable jobs page, a failed,
  skipped, missing, duplicated or other-commit job; mutations of the commit, branch and count
  clauses each fail it); live replays against hosted listings in the build log. Hosted push reuse
  `NOT_OBSERVED` until the first push after this change lands. Rollback: delete the
  `static-reuse` job, its `needs` and `mode` conditions, and restore
  `test "$STATIC_RESULT" = success` in `go-product`.
- **AFP-V0-081:** (accepted under the owner's standing auto-accept (2026-10-11; V1-1778/V1-1642); no GitHub issue) For a pull
  request labelled `ci:batched` (AFP-V0-027), the `go-interop`, `artifact-integrity` and
  `ci-shard-cost-drift` jobs of `.github/workflows/ci.yml` and the `containment` job of
  `.github/workflows/intake-containment.yml` SHALL NOT start, besides the shards and `go-static`.
  `doc-gates`, `docs-plan` and `go-product` (failing with the label message) SHALL still run, so
  the constituent still cannot merge alone. Each withheld job runs on the batch pull request,
  which carries the constituent's commits, and the three `ci.yml` jobs again on its queue entry.
  Design: 21 constituent runs in the build-log window spent 334 runner-minutes on these jobs
  (median 12.5 minutes per constituent push).
  Failure modes: a constituent's own interop, artifact or intake failure surfaces only on its
  batch pull request; a mislabelled pull request stays blocked, never merged untested.
  Acceptance evidence: `TestAFPV0081BatchedConstituentSkipsOnlyRerunJobs`; hosted behavior
  `NOT_OBSERVED` until the next constituent push. Rollback: remove the label clause from the four
  job conditions.
- **AFP-V0-082:** (accepted under the owner's standing auto-accept (2026-10-11; V1-1778/V1-1642); V1-0728; no GitHub issue)
  The workflow concurrency group of `.github/workflows/ci.yml` SHALL be the pull-request number
  for a pull-request run, the pushed commit for a `push` run and the ref otherwise, with
  `cancel-in-progress` kept, so a pull request still cancels its superseded run, each queue entry
  stays its own group, and a later `main` push never cancels an earlier one. A reader of
  required-check state for a `main` commit (the primary-main bridge precondition "required
  checks green") then finds two completed check runs per required name on that commit, from its
  queue run and its push run, and neither is cancelled by a later merge.
  Design: 12 of 47 `main` push runs in the build-log window ended cancelled.
  Failure modes: a rapid run of merges keeps several push runs in flight; after AFP-V0-024 and
  AFP-V0-080 each costs about 6 runner-minutes. Acceptance evidence:
  `TestAFPV0082PushConcurrencyGroupsByCommit`; hosted behavior `NOT_OBSERVED` until two merges
  land within one push run. Rollback: restore `github.event.pull_request.number || github.ref`.
- **AFP-V0-088:** (accepted under the owner's standing auto-accept (2026-10-11; V1-2049); no GitHub issue)
  A dirty path of a Corvint Tasks store projection is task state, not source. (1) The recognised
  projection layout is closed: `.taskman/policy.json`, `.taskman/queue.json`, and one
  `.taskman/tickets/<name>.json` or `.taskman/releases/<name>.json` record whose name has no
  further `/`; any other path, including any other path under `.taskman/` or a `.taskman/`
  directory below the root, keeps the AFP-V0-004 `UNOWNED_DIRTY_PATH` widening. (2) A projection
  path that some unit names, by a path literal that resolves to it (AFP-V0-021, AFP-V0-072) or by
  a declared read scope that holds it (AFP-V0-023), is read by a test: the reader rules select
  that unit as before and the path keeps its `UNOWNED_DIRTY_PATH` entry, so the plan stays
  `UNKNOWN`. An unbounded reader (rule (d)) is selected on every dirty path and does not count as
  naming one. (3) Every other projection path raises no unknown entry and is listed instead in the
  plan member `nonSource`, one group per class
  `{"class":"TASK_STORE_PROJECTION","reason":...,"paths":[...]}` with the stated reason once per
  class and the paths in dirty-path order; the member is omitted when no path is task state, and
  the default `affected-plan/1` compact receipt carries it unchanged. (4) Such a path makes no
  plugin take part in the plan (`participants`), so it brings no language frontier: a dirty
  projection alone is `BOUNDED` with nothing selected, and beside a source change it adds
  nothing to the selection or the unknown entries.
  Design: in Beamfall core-history 820 of 866 unknown entries were projection paths, so every
  plan touching the store was `UNKNOWN` whatever the source change. Non-goals: impact or
  worktree freshness (`mixed-worktree`) is not reclassified here (V1-2049 `NOT_PRODUCED`); no
  read of the projection's content, no other task store layout; the
  `tools/gate-affected-select` fast-tier mirror follows in AFP-V0-153. Failure modes: a test that reads a projection
  file through a computed path with no literal and no declared scope is not seen, the same blind
  spot AFP-V0-021 already has for any data file; a store-relative literal such as `/tickets/`
  names every ticket record and keeps it unknown, which over-widens rather than hides a reader.
  Acceptance evidence: `TestTaskStoreProjectionIsTaskStateNotUnknown_AFPV0088`,
  `TestTaskStoreProjectionBesideASourceChangeAddsNothing_AFPV0088`,
  `TestTaskStoreProjectionReaderIsSelectedAndKeepsItUnknown_AFPV0088`,
  `TestTaskStoreProjectionLayoutIsClosed_AFPV0088`, `TestTaskStoreProjectionWire_AFPV0088` and
  `TestAffectedTaskStoreProjectionIsTaskState_AFPV0088`; core-history and Corvint measurements
  in build log 2026-10-20-V1-2049-affected-taskman-projection. Rollback: delete
  `internal/liveverify/affected/taskstate.go` and its test, the `NonSource` plan field and its
  `participants` exclusion, and the compact receipt member.

### Plan observation bounds

- **AFP-V0-150:** (accepted under the owner's standing auto-accept (2026-10-11; V1-1087))
  The default plan path (`compileAffectedGraph`, reached by `affected`, `affected --base`,
  `affected --path`, `corvint test` and the `review --base` fallback) MUST read the worktree
  status and HEAD again after its last source observation (the graph build, the prior read
  scopes, the advice, the provider projection, the e2e-safe or external-evidence selection and
  the app navigations), not between the graph build and those reads. Any change of the dirty
  set or of HEAD, a same-tree commit included, MUST refuse with exit 2, code
  `unsupported-affected-drift` and no stdout document, the ordering the Playwright discovery
  producer already has (TJAA-V0-019). Failure mode: a commit that lands after the final HEAD
  read is outside the observation and is the caller's race, as for every other read command.
  Rollback: move the two reads back above the prior read-scope step.
- **AFP-V0-151:** (accepted under the owner's standing auto-accept (2026-10-11; V1-0875))
  The plan path MUST hand its caller's context, with that context's deadline and cancellation,
  to `affected.Build`, whose walk and language adapters observe it (AFU-V1-059). A cancelled or
  expired standalone `affected` MUST refuse with exit 2, code `unsupported-affected-graph` and
  a message naming the context error, and no stdout document; the `review --base` fallback
  (RGV-V0-013), which runs the same path under the review's own deadline, MUST return the same
  typed refusal and no review receipt. Neither MAY present a partial plan as complete. Non-goal:
  no new command-line timeout flag; the bound is the caller's context (process signals for the
  CLI, the review deadline for guidance). Rollback: none needed beyond AFU-V1-059; the test seam
  `affectedGraphContext` is the identity in production.
- **AFP-V0-152:** (accepted under the owner's standing auto-accept (2026-10-11; V1-1820))
  A sealed CEM, a dirty path `.corvint/changes/<name>.cem.json` with a non-empty `<name>` and
  no further `/`, MUST narrow its Go literal readers exactly as the CEM sidecar does
  (AFP-V0-012 (c), V1-0230): in the plan (`resolves`, `internal/liveverify/affected`) and in the
  gate selector and its `-bounds` attribution (`resolvingReaders`, `internal/gateselect`), a Go
  package or unit reads it only when one of its tokens, joined to its own directory, or to the
  repository root when the package holds an anchored or parent-only token, forms the path or one
  of its ancestors. A fixture-root token such as `".corvint/changes/"` joined to `t.TempDir()`
  therefore no longer selects its holder. Every other rule is unchanged: non-Go readers keep the
  component-run relation, rule (d) unbounded readers and declared read scopes still select, the
  narrowed units' frontier reasons are kept, and any other path under `.corvint/changes/`
  (nested, or not ending `.cem.json`) keeps rule (c) unnarrowed. The readers that remain are
  the ones this rule names: resolving or root-anchored Go readers, non-Go readers, and rule (d)
  and declared-scope units. Measured on this repository (base 5cf75daf, one added sealed CEM):
  literal-reader witnesses 15 to 13, selected units 53 to 53, because the two narrowed units
  (`internal/liveverify/affected`, `.../affected/golang`) are also rule (d) unbounded readers
  and the other eleven Go readers hold root-anchored tokens; the residual cost is the rule (d)
  floor (V1-0081), not rule (c). Failure mode: a test that reaches the real corpus through a
  root supplied at run time with no anchoring literal is not selected, the residual AFP-V0-072
  already accepts. Rollback: restore the exact sidecar comparison in both packages.
- **AFP-V0-153:** (accepted under the owner's standing auto-accept (2026-10-11; V1-2070))
  The fast-tier selector (`internal/gateselect`, run by `tools/gate-affected-select`) MUST read
  the plan's `scope` and `nonSource` members, and when its own attribution (rules (a) to (d) and
  the declared read scopes) selects no Go package over a non-empty dirty set it MUST print
  `NOTHING` instead of the full-suite `FALLBACK` only when all of these hold: the plan `scope` is
  `BOUNDED`; every dirty path is listed in a `nonSource` group of class `TASK_STORE_PROJECTION`
  (AFP-V0-088 (3)); and every dirty path is in the AFP-V0-088 (1) layout by the gate's own check
  (`taskprojection.IsPath`). Any other empty selection over a dirty set, including an `UNKNOWN`
  plan, an absent `nonSource` member, an unlisted path or a path outside the layout, keeps the
  fallback. A package whose literal or declared scope names a projection record is still selected
  by the ordinary rules, so this never drops a reader. This amends the AFP-V0-088 non-goal that
  left the fast-tier mirror unchanged. Failure mode: a planner that lists a path wrongly as task
  state is caught by the layout check only when the path is outside the closed layout; inside it
  the gate trusts the plan's classification exactly as the plan does. Rollback: delete
  `taskStateOnly`; the empty-selection branch falls back again.
- **AFP-V0-154:** (accepted under the owner's standing auto-accept (2026-10-11; V1-0996))
  The fast-tier selector's rule (a) (`attributeStructure` in `internal/gateselect/select.go`)
  MUST follow the AFP-V0-036 rule: a dirty Go path ending in `_test.go` whose directory is an
  indexed package of the root module selects that package only and MUST NOT widen to its
  importers, whether the file is changed, added or deleted, because Go compiles a test file only
  into its own package's test binary. Every other Go path, a test file whose directory is not
  an indexed package, and a test file in a package whose non-test files carry any `//go:embed`
  directive (it may embed the test file as production data its importers' tests read) keep the
  widening to importers; rules (b) to (d), the declared read scopes
  and the plan's own `provider.go.packages` are unchanged, and the GL-V0-009 `-bounds` listing
  follows the same rule. Before this, a one-test-file change in a package selected every
  importer of that package in the fast tier although `corvint affected` did not. Falsifier: a
  `_test.go`-only change that selects an importer through rule (a), or a non-test source change
  that does not. Rollback: delete the `_test.go` early return in `attributeStructure`.
- **AFP-V0-155:** (accepted under the owner's standing auto-accept (2026-10-11; V1-1005))
  In the `affected-plan/1` default, `advice.unknown` MUST NOT repeat `plan.unknown`: every
  `advice.unknown` line of the `affected-plan/0` advice that is exactly a `plan.unknown` entry's
  `reason`, `": "` and `detail` MUST be replaced by the one line `PLAN_UNKNOWN: N plan.unknown
  entries; see plan.unknown`, where N is the number of `plan.unknown` entries, and every other line
  (the advice-specific gaps such as `NO_ADVISORY_GO_COMMAND`, `NO_REPOSITORY_GATE_DECLARED` and
  the application-navigation lines) MUST be kept; the list stays sorted and duplicate-free. When no
  line repeats a `plan.unknown` entry the list is unchanged. The default keeps `plan.unknown`
  whole, so replacing the count line by those entries reproduces the `--full` advice exactly;
  `--full` and the review embedding of the full advice are unchanged. Compatibility: `advice.unknown`
  is free text outside the CCF-V1-007 register (no reader branches on its lines), so the change is
  compatible; the `affected committed range` core-freeze golden is amended for its element count.
  Measured on this repository's clean worktree at 318a0d93e: the default document fell from 5,279 to
  4,172 bytes (23 repeated lines replaced by one), with an identical `plan` member. Falsifier: a
  default document whose `advice.unknown` repeats a `plan.unknown` entry, drops an advice-specific
  line, or does not resolve to the `--full` advice. Rollback: return `advice.Unknown` unchanged
  from `compactAffectedAdvice`.
- **AFP-V0-316:** (accepted under the owner's standing auto-accept (2026-10-11; V1-2169))
  The fast-tier selector's repository index (`indexRepository` in
  `internal/gateselect/readers.go`, used by `corvint test`, `tools/gate-affected-select` and the
  `-bounds` and `-unresolved` listings) MAY read, parse and lex the Go sources it walks with a
  bounded pool of at most eight workers (never more than `GOMAXPROCS`), but MUST record their
  imports, path literals, root-locating calls and `//go:embed` directives in walk order, so the
  index is the one the serial pass builds, and MUST return the error the serial pass meets first:
  a source that fails its bounded read or import parse precedes any walk error (entry cap,
  non-regular Go source) found after it. The walk, the entry cap, the nested-module and hidden
  directory skips, and the bounded-read identity check are unchanged. Measured on this repository
  at c9ca8beff (load average 31 to 64): indexing fell from a 1.77 s to a 0.65 s median of five
  ten-iteration runs, and `corvint test --dry-run` over a two-package change from a 5.05 s to a
  3.90 s median of five, with byte-identical stdout, stderr and manifest. Falsifier: an index or
  first error that differs from the serial pass (`GOMAXPROCS=1` takes it). Rollback: restore the
  in-walk `scanFile` call.
- **AFP-V0-296:** (accepted under the owner's standing auto-accept (2026-10-11; V1-2194))
  `corvint-pr-tests` admits exactly the toolchains decision 0547 allows, `go1.27.2` and `go1.27.3`.
  One definition in the driver, `admittedGoVersions`, holds that set. The trusted tool identity
  observed under any other `go env GOVERSION` is refused with the existing error `unsupported Go
  version`. The `.github/cishards` source-version set and the hosted `setup-go` pin are unchanged.
  Falsifier: an identity accepted under a version outside the set, or refused under `go1.27.3`.
  Rollback: set `admittedGoVersions` back to `go1.27.2` alone.

### Declared documentation paths and scope-bounded literal readers

- **AFP-V0-163:** (accepted under the owner's standing auto-accept (2026-10-11; V1-2113); amended
  before merge to job-log evidence after an independent review found that artifact names do not
  authenticate their producer (2026-10-21))
  A merge-queue entry MUST NOT rerun the `artifact-integrity` archive gate on a tree that a
  successful pull-request run already verified. In a `pull_request` run, an
  `artifact-integrity-record` job MUST run with `needs: artifact-integrity`, only when that job
  concluded `success`; it MUST check out the tested merge commit without credentials, run no
  repository code (no `go`, `make` or repository script; shell builtins, `git` and `printf`
  only), persist no credentials, and print one line to its own log:
  `corvint-ci-integrity-tree/0 tree=<tree> target=<merge commit> head=<pull-request head>`. In a
  `merge_group` run, the job MUST decide REUSE only when the queue commit is a two-parent merge
  whose tree is `<tree>`, and a successful `pull_request` run of `.github/workflows/ci.yml` in
  this repository whose head is that commit's second parent has exactly one latest
  `artifact-integrity` job and exactly one latest `artifact-integrity-record` job, both of that
  run and concluded `success` on that head, and that record job's log, read through the job-log
  API, holds exactly one record line whose tree and head equal the queue commit's. The decision
  MUST NOT read or trust any artifact. On REUSE the job MUST skip `setup-go` and the archive gate,
  still conclude `success` (so AFP-V0-080 push reuse keeps its three passed jobs), write the
  decision (profile `corvint-ci-integrity-reuse/0`: mode, tree, head, target, run id, record job
  id) to the step summary and retain it as the diagnostic `ci-integrity-reuse` artifact, which
  nothing reads back. Any other listing, refusal, unreadable response or log, missing or
  duplicated job, missing, duplicated or mismatched record line, or a commit that is not a
  two-parent merge MUST leave the decision empty and run the gate in full. Trust argument: a job
  log is written only by the runner executing that job, so code in any other job of the run (a
  test, a build step, or a fork pull request's code) cannot add a line to the record job's log,
  whereas any job can upload an artifact under any name. The record job runs no repository code,
  so only its definition in `ci.yml` decides what it prints. A pull-request run takes `ci.yml`
  from the tested merge commit, whose tree must equal the queue commit's, so any `ci.yml` that
  could print a false line is itself in the tree being merged, and `.github/` edits are gated by
  the `ci-control-plane` admin review. The tree holds `ci.yml`, the `Makefile`, `go.mod` and every
  gate input, and the gate runs with `GOPROXY=off`, so an equal tree is an equal verdict; the
  `needs`/`if` lines of `artifact-integrity` are unchanged (AFP-V0-080/081). Measured before:
  queue run 39458994189 spent 11.35 min in `artifact-integrity` while `go-product` reused its
  pull-request shards and finished at about 4.9 min, so the job was the queue's critical path;
  the predicted queue wall time after is about 5 min (`go-static`, about 4.8 min, becomes the
  critical path). Falsifier: a queue run that skips the gate for a tree no passed record job
  printed, or that runs the gate when one did and the listings and log were readable. Rollback:
  delete the `artifact-integrity-record` job, the `permissions` block, reuse and retain steps and
  the step conditions of `artifact-integrity`, and the `job:artifact-integrity` row of
  `script/ci-least-privilege-exceptions.tsv`.
- **AFP-V0-164:** (accepted under the owner's standing auto-accept (2026-10-11; V1-1752))
  `internal/tasks/cli` and `internal/tasks/authority` MUST be listed in
  `.github/cishards/test-split-allow.json`, each with its order-independence review, and split in
  `.github/cishards/test-slices.json` so that no slice is predicted above the AFP-V0-060 bound of
  400 s; every test of both packages MUST still be selected exactly once across the plan
  (AFP-V0-048 coverage parity). Measured from run 39613481398 (revision f15f4111d): the packages
  took 467.6 s and 444.5 s, and the slowest of 8 shards took 1561.4 s against an ideal 1132.9 s;
  `tools/ci-test-slices replay` over that run's costs predicts a maximum shard of 1628.8 s with
  the previous slices and 1133.9 s with the new ones, and 1556.7 s to 1414.9 s under the committed
  `package-costs.json`. Falsifier: a plan that drops or duplicates a test of either package, or a
  slice of either predicted above 400 s. Rollback: revert both entries in the two files.
- **AFP-V0-160:** (accepted under the owner's standing auto-accept (2026-10-11; V1-0923); no
  GitHub issue) A repository MAY declare which of its paths are documentation rather than build or
  test inputs, so a change of documentation alone is not an unowned path. (1) The declaration is
  `.corvint/documentation-paths.json`, the closed object
  `{"profile":"corvint-documentation-paths/0","paths":[ENTRY...]}`, a regular file of at most
  64 KiB with at most 256 entries. An `ENTRY` is a canonical repository-relative file, or a
  directory when it ends in one `/`, under the AFP-V0-023 entry grammar (never `.git` or below
  it), never `.corvint` or a path below it, and the entries are strictly ascending. An absent
  declaration declares nothing; one that is unreadable, oversized, not a regular file or not exactly
  this grammar refuses the graph build, naming the declaration, rather than classifying anything.
  The declared entries are part of the graph's canonical form and digest, omitted when there are
  none, so graphs of repositories without the declaration keep their digest. (2) A dirty path that
  equals an entry or lies under a directory entry, that raised `UNOWNED_DIRTY_PATH` and that is not
  in the AFP-V0-088 (1) layout, raises no unknown entry and is listed instead in a second `nonSource`
  group `{"class":"DECLARED_DOCUMENTATION","reason":...,"paths":[...]}`, after any
  `TASK_STORE_PROJECTION` group, with the stated reason once and the paths in dirty-path order.
  (3) The reader rules are unchanged: every unit whose path literal (AFP-V0-021, AFP-V0-072),
  declared read scope (AFP-V0-023) or unbounded read (rule (d)) reaches the path is still selected,
  and those selections are the plan's documentation checks. A path a plugin claims as source
  (`UNINDEXED_SOURCE_PATH`) is never reclassified. (4) A declared documentation path makes no plugin
  take part in the plan (`participants`), so documentation alone brings no language frontier, and
  beside a source change it adds nothing to the selection or the unknown entries. (5) A Tasks store
  projection path stays under AFP-V0-088 even inside a declared directory. (6) This repository
  declares `docs/build-log/`, one new file per entry (decision 0423), and nothing else. Design: the
  class is a repository declaration, modelled on the AFP-V0-088 task-store class, so the product
  hard-codes no documentation layout; the declaration cannot name `.corvint/`, so it can never
  classify itself or another Corvint declaration. Non-goals: no content read of the documentation,
  no default layout, no change to the `tools/gate-affected-select` fast-tier mirror or to
  `internal/gateselect`, whose AFP-V0-153 task-state case still counts only
  `TASK_STORE_PROJECTION` groups (a documentation-only diff keeps its own attribution there).
  Failure modes: a declared path that a test reads through a computed path with no literal and no
  declared scope is not seen, the AFP-V0-021 blind spot for any data file; a too-wide declaration
  over a real build input is a project authority error the declaration owner corrects, and its
  readers by literal or scope are still selected. Acceptance evidence:
  `TestDeclaredDocumentationIsNonSourceNotUnknown_AFPV0160`,
  `TestDeclaredDocumentationKeepsEveryReader_AFPV0160`,
  `TestDeclaredDocumentationLeavesTaskStateAndSourceAlone_AFPV0160`,
  `TestDocumentationDeclarationIsValidated_AFPV0160`,
  `TestDocumentationDeclarationIsInTheDigest_AFPV0160` and
  `TestRepositoryDeclaresBuildLogDocumentation_AFPV0160`; the repository measurement in build log
  2026-10-21-design-1021-documentation-and-scoped-readers. Rollback: delete
  `internal/liveverify/affected/documentation.go`, its test and `.corvint/documentation-paths.json`,
  and the `documentation` graph field, digest member and `Select` call.
- **AFP-V0-161:** (accepted under the owner's standing auto-accept (2026-10-11; V1-1766, V1-2069);
  no GitHub issue) A unit with a declared read scope (AFP-V0-023) is not a path-literal reader
  (rule (c), AFP-V0-021) of a dirty path outside that scope. (1) For such a unit, a dirty path that
  is not in its directory subtree, not an entry, not under a subtree entry, not an ancestor of an
  entry and not the declaration itself does not resolve for any token, so the planner neither
  selects the unit with a `PATH_LITERAL_READER` witness nor counts it as naming a Tasks store
  projection path (AFP-V0-088 (2)). (2) Every path the scope holds keeps the AFP-V0-021 and
  AFP-V0-072 relation unchanged, so every reader the declaration allows is still selected with the
  same witness. (3) The narrowing adds no unknown entry and removes none; a unit whose unit
  frontier was named because a token named the path still has it named (AFP-V0-072 (4)). Design:
  the declaration states that the package's tests read nothing under the root outside that scope,
  and full CI enforces it with the Landlock wrapper (AFP-V0-023), so a literal outside it is a
  fixture name, not a read the tests can make without failing full CI; project-owned authority
  outranks syntax. Literal matching itself is not narrowed: the remaining over-selection comes from
  whole-segment anchored (`/x`), climbing (`../x`) and file-name literals that a root supplied at
  run time can join, which no literal index can rule out. Non-goals: the `tools/gate-affected-select`
  fast-tier mirror and `internal/gateselect` keep rule (c) unbounded, a superset of the planner;
  no new declaration is added for any package by this requirement. Failure modes: the AFP-V0-023
  residuals carry over unchanged, now for rule (c) as for rule (d): a test that depends only on
  metadata or absence under the root, or tolerates a denied read, is not confined, and the
  AFP-V0-013 driver runs are not confined; a too-narrow declaration fails that package's tests in
  full CI rather than silently under-selecting. Acceptance evidence:
  `TestDeclaredReadScopeBoundsPathLiteralReaders_AFPV0161`,
  `TestDeclaredReadScopeKeepsEveryInScopeLiteralReader_AFPV0161` and
  `TestDeclaredReadScopeDropsOutOfScopeFixtureNamers_AFPV0161`; this repository's namer counts in
  build log 2026-10-21-design-1021-documentation-and-scoped-readers. Rollback: delete the
  read-scope guard at the top of `resolves` in `internal/liveverify/affected/readers.go` and its
  test, and restore the AFP-V0-023 rule (c) sentence.

- **AFP-V0-356:** (accepted under the owner's standing auto-accept (2026-10-11; V1-2053))
  A MIXED-worktree warning (`--base`) and the MIXED refusals of `--require-clean` and `--path`
  (`unsupported-affected-mixed-worktree`) MUST name the dirty paths, sorted, at most 8 quoted, with
  `(+N more)` for the rest, and MUST state that Corvint runs Git without global configuration
  (decision 0158), so a path ignored only by a global excludes file (`core.excludesFile` or
  `~/.config/git/ignore`) is dirty here, with the fix: ignore it in a tracked `.gitignore` or in
  `.git/info/exclude`. Global excludes stay unhonoured; the dirty set, exit codes and codes are
  unchanged. Non-goals: the `impact` MIXED warning (its budgeted envelope at
  `MinPacketBytes` cannot carry the detail) and the MCP `RepositoryBinding`, which stays a count and
  digest; `overview` already names the cause (RGV-V0-050). Falsifier: a MIXED refusal or warning
  caused only by a globally excluded path that does not name the path and the fix, or that lists
  more than 8 paths. Rollback: drop `affectedDirtyDetail` from the three messages.
- **AFP-V0-156:** (accepted under the owner's standing auto-accept (2026-10-11; V1-2162))
  A root check that only needs to prove a directory is the top of a Git worktree MUST NOT name the
  repository-root locator option of AFP-V0-012 rule (d). It MUST run `git rev-parse
  --is-inside-work-tree --show-prefix` in the directory and accept exactly `true`, newline, empty
  prefix, newline; any other `true` output is "not the top", and a failure or `false` (a plain
  directory, a Git directory) is "not a worktree". The refusal reasons and codes stay as they were.
  This covers `openRoot` in `internal/appmap/gitread.go`, `bindRepository` in
  `internal/appflows/repositories.go` and `resolveCheckout` in `internal/extevidence/repository.go`,
  none of which read through the located top. A package that reads, runs Git or lists files through
  a located top stays a rule (d) reader: this requirement removes no such reader and changes no
  detector. Measured 2026-10-21 at base c9ca8beff (load averages 55 to 120 on a 12-core host):
  `tools/gate-affected-select -unresolved` lists 104 unresolved packages at the base and, without
  the new `tools/dogfood-guard-hunks` package that inherits from `internal/jstestprovider`, 104
  after the change. The 63 packages that inherited from `internal/appflows` now inherit from
  `internal/jstestprovider` (also reached are `internal/worksource` and
  `internal/postmergeconnector`), which use the located top, so the floor does not drop until those
  are bounded or declared under AFP-V0-023. A one-package change to `internal/appmap/gitread.go`
  gives 130 bound and 105 unresolved `-bounds` rows (130 and 104 at the base). Falsifier: one of
  the three sources names the locator option, or binds a subdirectory, a Git directory or a plain
  directory. Rollback: restore the `--show-toplevel` comparison in the three functions.

- **AFP-V0-162:** (accepted under the owner's standing auto-accept (2026-10-11; V1-2256, V1-2252))
  The `affected-plan/1` default (and the review embedding of it, RGV-V0-015) MUST carry an optional
  top-level member `tiers` whenever `plan.selected` is non-empty, and MUST omit it otherwise:
  `{counts, summary, units}`. `units` maps every `plan.selected[].unitId` to exactly one tier:
  0 when the witness kind is `DIRECT_SOURCE_CHANGE`, `DIRECT_TEST_CHANGE` or `ENCLOSING_PACKAGE`
  (the unit owns or encloses a dirty path); otherwise 1 when the witness is `DEPENDENCY_PATH` or
  `ADVISORY_CONSTANT_REFERENCE` with a chain of at most two units (a direct dependent of a changed
  unit), or the unit id's path part (after its namespace) has the same parent directory as a tier-0
  unit's whose witness `dirtyPath` has a parent of its directory that is not a top-level directory
  (`internal/a/b/x.go` makes `internal/a/*` siblings; `internal/b/x.go` makes none); otherwise 2, path-literal and read-scope readers included, because almost every change
  touches a spec or build-log path that many packages name. `counts` is `[tier0, tier1, tier2]` and `summary` is the text line
  `tier0=N tier1=N tier2=N`. Tiers never add, remove or reorder a selection, and every other member,
  `provider.go.packages` included, is unchanged; `--full` (`affected-plan/0`) is byte-for-byte
  unchanged. Compatibility: an optional member under CCF-V1-006; the `affected committed range`
  core-freeze golden is regenerated for it. Local recipe (V1-2252): a lane runs the Go packages of
  tier 0 and 1 locally, capped at 15 packages (nearest first in `plan.selected` order), skipping
  `./cmd/corvint` unless the change touches it directly, and records every tier-2 package and every
  capped or skipped one as `NOT_RUN` with this command's `summary`, leaving them to CI; that subset is
  advisory and never a proof that tier-2 tests pass (AFP-V0-004). Evidence:
  `TestAFPV0162AffectedTiersRankTheFullSelection` (tier 0 changed package, tier 1 direct importer and
  same-parent sibling dependent, tier 2 transitive dependent). Falsifier: a selected unit missing from
  `units`, a tier that disagrees with the rule, counts that do not sum to `plan.selected`, or a
  changed `--full` document. Rollback: drop `Tiers` from `affectedCompactReceipt` and regenerate the
  golden.

### Python adapter (experimental)

`internal/liveverify/affected/python`, registered in `languages.All()`, observes Python modules from
source text. Qualification: experimental. No live external-corpus qualification has run, the adapter
is not advertised as delivered, and the known under-selections under Failure modes must close or be
accepted before promotion.

- **AFP-V0-181:** (accepted under the owner's standing auto-accept (2026-10-11; V1-2050)) The Python
  plugin MUST own exactly the paths ending in `.py` and MUST give each such file one unit with id
  `python:` plus its repository-relative dotted name (a trailing `/__init__` dropped), so no two files
  claim one identity. A file is a test unit when its name starts with `test_` or ends in `_test.py`,
  or a directory component is `tests` or `test` (the discovery conventions pytest and unittest
  share); otherwise it is a source unit. Evidence: `TestUnitsGivesEachFileOneIdentity`,
  `TestIsTestFileMatchesDiscoveryConventions`.
- **AFP-V0-182:** (accepted under the owner's standing auto-accept (2026-10-11; V1-2050)) Import
  edges MUST come from source text alone, with no interpreter or virtual environment: each
  `import a, b as c` or `from x import y` statement, joined across continuation lines (AFP-V0-187). Each name resolves to the longest
  observed module name that prefixes it, where a file under `src/`, `lib/` or `python/` is also
  importable relative to that root. A name inside a string or comment may add an edge; that only
  widens selection. Evidence: `TestParseImportLineReadsEveryStatementForm`,
  `TestResolveRefPrefersTheLongestObservedModule`, `TestModuleAliasesExposeSourceRootLayouts`.
- **AFP-V0-183:** (accepted under the owner's standing auto-accept (2026-10-11; V1-2050)) Each name a
  `from X import name` line imports (not `*`, after any `as` alias, parentheses and trailing comment
  removed) MUST also be tried as the submodule `X.name`, so a change to `pkg/sub.py` selects the test
  that writes `from pkg import sub`, in a regular or namespace package and through a relative
  import. Before this, such a change selected nothing and reported `NO_SELECTABLE_TEST`. Evidence:
  `TestAFPV0183FromImportNamesReachSubmodules` (fails without the change). Rollback: drop the name
  loop in `parseImportLine`.
- **AFP-V0-184:** (accepted under the owner's standing auto-accept (2026-10-11; V1-2050)) A relative
  import MUST be anchored at the importing file's directory, one level up per extra leading dot. One
  that resolves to no observed module, or climbs to the repository root or above, MUST raise the
  frontier `python:relative-import-unresolved` and add no edge. Evidence:
  `TestResolveRefAnchorsRelativeImportsAtTheImportingPackage` (resolution only; the frontier raise is
  a gap).
- **AFP-V0-185:** (accepted under the owner's standing auto-accept (2026-10-11; V1-2050)) What source
  text cannot settle MUST be a frontier reason, never a silent omission: `importlib` or `__import__`
  on a line raises `python:dynamic-import`; an indented import raises `python:conditional-import`
  and keeps its edge; an unreadable file raises `python:unreadable-source`. Evidence:
  `TestScanImportsFlagsWhatSourceTextCannotSettle` (dynamic and conditional; unreadable source is a
  gap).
- **AFP-V0-186:** (accepted under the owner's standing auto-accept (2026-10-11; V1-2050)) On the
  repository's own Python corpus, a one-file edit MUST narrow the full test set and still reach the
  test the corpus specification pairs with that module. Evidence:
  `TestPythonSelectionNarrowsARealCorpus`, which skips below 20 Python tests; the 18-module corpus it
  was written for (`src/context_corvint_index.py`) is not tracked on `origin/main`, so it skipped on 2026-10-21
  and this requirement is unobserved until a fixture corpus replaces it (gap).

- **AFP-V0-187:** (accepted under the owner's standing auto-accept (2026-10-11; V1-2297)) An
  `import` or `from` statement continued across lines, inside open parentheses or brackets or after
  a trailing backslash, MUST be read as one statement: its lines are joined, after `#` comments are
  removed, until the brackets balance, and every name it lists is read as AFP-V0-182 and AFP-V0-183
  read a one-line statement. The continuation lines are consumed, so an indented one never raises
  `python:conditional-import`, and a statement still open at end of file keeps the names it shows.
  Before this, `from pkg import (` followed by one name per line read none of the names, so a change
  to `pkg/late.py` selected nothing. Evidence: `TestAFPV0187MultiLineFromImportReadsEveryName`,
  `TestAFPV0187ContinuationLinesAreNotStatements` (both fail without the change). Rollback: read each
  physical line on its own again in `scanImports` and drop `joinContinuation`.
- **AFP-V0-188:** (accepted under the owner's standing auto-accept (2026-10-11; V1-2296)) A
  column-zero `pytest_plugins =` or `pytest_plugins +=` assignment (not a comparison), joined across
  lines as AFP-V0-187, MUST add an absolute import of each module its string literals name. A value
  that is not only string literals MUST raise the frontier `python:pytest-plugins-unresolved`.
  Evidence: `TestAFPV0188PytestPluginsAreImports` (list, multi-line, string and augmented forms; the
  non-literal raise; a comparison not read). Rollback: drop the `pytest_plugins` branch of
  `scanImports` and `pluginModules`.
- **AFP-V0-189:** (accepted under the owner's standing auto-accept (2026-10-11; V1-2296)) Each test
  file MUST have an import edge to every `conftest.py` in its own directory and each ancestor up to
  the repository root, because pytest loads those without an import. A `conftest.py` change therefore
  selects every test at or below its directory and none beside or above it; with AFP-V0-188 a plugin
  module a conftest names reaches the same tests. Evidence: `TestAFPV0189ConftestSelectsEveryTestBelowIt`
  (fails without the change). Rollback: remove `linkConftests`.

Python non-goals: no interpreter, import hook, `sys.path` or `.pth` evaluation, packaging metadata
(`pyproject.toml`, `setup.cfg`) or virtual-environment discovery; no pytest collection.

Python failure modes and gaps (each widens unknowns or under-selects; none is promoted):
A plugin registered outside a `.py` file (`-p` on the command line, `pytest.ini`, `pyproject.toml`
or a `pytest11` entry point) is not read: a change to that file is already an unowned unknown, but
a change to the plugin module it names selects only the tests that import it. A test that reads a non-Python data file records no path token, so that file stays an
unowned unknown. `sys.path` changes and source roots other than `src/`, `lib/` and `python/` are not
modelled. Untested requirements: the `python:relative-import-unresolved` raise (AFP-V0-184),
`python:unreadable-source` (AFP-V0-185) and the corpus narrowing of AFP-V0-186, whose test skips.

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
A dirty Tasks store projection path that no unit names is task state in `nonSource`, not an
unknown entry (AFP-V0-088). A dirty path owned by no plugin, or a changed unit (one that owns a changed path) with no
selectable test in any language (AFP-V0-020): the plan widens to `UNKNOWN` scope rather than
narrowing; the packages that name a dirty path by literal are added, never substituted for the
widening (AFP-V0-021). A potentially deleted or renamed-away Go path widens and selects its
package, or the importers of a package that is gone, rather than claiming no dependency path
(AFP-V0-004). An unreadable subtree:
`unsupported-affected-graph`, never a silently smaller graph. Worktree or HEAD changed during
compilation: `unsupported-affected-drift`. A `--base` that is not a full commit id:
`invalid-arguments`; one that is not a commit here: `unsupported-affected-revision`; a range diff
over its bound: `unsupported-affected-status`. `--require-clean` over a worktree with any status
entry: `unsupported-affected-mixed-worktree` (AFP-V0-045). In the fast tier every one of these, a plan the
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
In full CI, an unusable test-split allow-list, slice file or cost table keeps every package whole,
and a split package's catch-all slice runs every top-level test that no named slice names
(AFP-V0-041). A split package whose slice outcomes are failed, unterminated or not one per slice,
logs that are not one per declared shard, or a slice file that splits a package into more slices
than the declared shards, refuses the cost refresh and makes the drift report abstain
(AFP-V0-041 (8)). Shard logs that are not one per declared shard, or a shard log without any
terminal package outcome, refuse slice generation and replay (AFP-V0-041 (5)). The sharded PR
driver refuses, before execution, a placement that does not put every package or slice of the
runtime universe on exactly one shard (AFP-V0-048). Slice generation and replay combine a split
package's per-slice outcomes; any other repeated outcome, a test outside the slice its log ran,
or a slice that ends twice still refuses (AFP-V0-049).

Queue-backstopped PR narrowing failure modes (decision 0431): absent queue rule or unreadable
rules API, an allowlist miss or a new frontier force FULL; missing selection at queue time
produces `INCOMPLETE`; a queue timeout shorter than `CI` fails entries. Disabling the queue
forces FULL through AFP-V0-028. AFP-V0-026 retains the lingering control-plane status limit.
Late failures eject PRs; rerun passes classify flakes without changing the queue failure.
Artifact listing failure forces FULL; threshold trips and a reviewer-confirmed `CHANGE_CAUSED`
miss disable narrowing as AFP-V0-030 requires. Hosted auto-merge command compatibility is
`NOT_VERIFIED`, and net runner cost and ready-to-merge effects are `NOT_OBSERVED`.

`--path` failure modes (AFP-V0-094/095): a non-canonical value or an unsupported combination is
`invalid-arguments`, and so is a separated `--path` value that begins with `-`; a dirty worktree,
or an ignored untracked file the list names or the graph reads, is
`unsupported-affected-mixed-worktree`, never a plan that mixes bytes HEAD does not pin into HEAD's
graph; a listed path no language claims widens to `UNKNOWN`
rather than selecting nothing at `BOUNDED`. A consumer still cannot read closure depth from an
`impact` receipt; only the help says so (AFP-V0-096).

## Acceptance evidence and traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| AFP-V0-001 | `cmd/corvint/affected.go` `compileAffected` | `TestAffectedCleanTreeSelectsNothingAndWritesNothing` compares `git status --porcelain --ignored` before and after |
| AFP-V0-002 | `internal/liveverify/affected/dirty.go` | `TestDecodeStatusFailsClosedOnMalformedInput`; `TestDirtyNonUTF8PathIsDisclosedNotRefused`; `TestAffectedRejectsNonRepositoryAndExtraArguments` |
| AFP-V0-003 | `affectedReceipt`, `providerGoPackages` | `TestAffectedDirtyGoSourceSelectsDependentsAsProviderPackages` |
| AFP-V0-004 | `affected.Select` scope and exclusion-reason rules; `goStructure`, `goSourceRule`, `goDataRule`, `WitnessEnclosingPackage` in `internal/liveverify/affected/structure.go`; `Unit.Embeds` and absent in-module import edges (`resolved`, `underModule`) in the Go plugin | `TestAffectedUnownedDirtyPathIsUnknownScope`, `TestDeletedGoSourceSelectsItsPackageAndImporters_V1_0340`, `TestUnownedDirtyPathSelectsItsPackageAndImporters_V1_0340` (an embedded asset, a nested fixture, a file directly in a package and a deleted package each select their package or importers; an unrelated package stays excluded); `frontierUnknowns`, `participants`, `claimantsOf`, `PathReader`, `Unit.Frontier` (V1-0289) with `TestFrontierBearsOnlyOnThePlansItTakesPartIn_V1_0289` (another plugin's change, a clean plan and an unreached unit name no frontier; an unowned path and a path-reading plugin take part), `TestAffectedEmptyDiffIsABoundedEmptyPlan_V1_2126` (a clean worktree, with and without `--base HEAD`, is `BOUNDED` with no unknown) and `TestBuildConstraintIsTheConstrainedPackagesFrontier_V1_0289`; provider wire unchanged (`go-live-test-provider-v0.md` GLTP-V0-006) |
| AFP-V0-005 | canonical JSON via `gokernel.CanonicalJSON`; `graphDigestDomain`, `digestBody`, `digestUnit`, `projectUnit` in `internal/liveverify/affected/graph.go` | byte-identity assertion in the dirty-source test; `TestGraphDigestIsTheDomainTaggedProjection_V1_0299` |
| AFP-V0-006 | `runAffected` error paths; `affected.ErrWalkUnreadable`; `affected.ErrWalkUnrepresentable` | `TestAffectedRejectsNonRepositoryAndExtraArguments`; `TestAffectedUnreadableSubtreeFailsClosed`; `TestSourceFilesRefusesAnUnrepresentableAcceptedName` |
| AFP-V0-007 | `Graph.rank`, `Graph.proximity` in `internal/liveverify/affected/select.go` | `TestSelectOrdersByDistanceThenSharedDirectoryThenUnitID` (order and two-run byte identity) |
| AFP-V0-008 | `SourceFilesIncluding`, `MaxIncludedDirectoryEntries`, `FrontierIncludedDirectoryWalkBounded`, `observeModules`, `workspaceDirectories`, `enclosingModule`, `groupByDirectory`, `inTestdata`, `unitID`, `readModulePath` in `internal/liveverify/affected` | `TestIncludedDirectoryWalkBoundWidensInsteadOfRefusing_AFPV0008`, `TestWorkspaceModulesAreUnitsUnderTheirOwnModulePath`, `TestWorkspaceDirtySourceSelectsTheOtherModulesTest` over `testdata/workspace` (a listed pair, an unlisted `stray`, an entry outside the root), `TestPackagesUnderBuildOutputDirectoryNamesAreSelected`, `TestNoGoRepositoryProducesNoUnitsOrFrontier`, `TestReadModulePathMatchesGoModEdit`, `TestReadModulePathAbstainsOnBOM`, `TestTestdataIsFixtureDataNotAPackage_AFPV0008`, `TestWorkspaceModuleBelowTestdataIsObserved_AFPV0008`, `TestWorkspaceUseOutsideRootIsAFrontier_AFPV0008`, `TestTestOnlyImportSelectsTheTestUserButNotItsImporters`, `TestSourceParsedEdgesCoverEveryEdgeTheToolchainReports` (a non-test toolchain import must be an ordinary edge) |
| AFP-V0-010 | `parseAffectedBase`, `affectedRangePaths`, `affectedRange` in `cmd/corvint/affected.go`; `RangePaths`, `DecodeNameList` in `internal/liveverify/affected/dirty.go` | `TestAffectedBaseRangeJoinsCommittedPathsAndFailsClosed` (committed edit with a clean worktree selects the dependents; `range.base`/`range.paths`; `main` and an unknown id exit 2 with no document), `TestDecodeNameListNormalizesAndFailsClosed`, `TestAffectedReceiptMembersAreClosedAndByteStable` (the closed member set includes `range`) |
| AFP-V0-011 | `gate-affected`, `gate-affected-test`, `GO_TEST_COMMAND` in `Makefile`; `script/gate-affected.sh`; its selection step `selectPackages` in `tools/gate-affected-select/main.go` (native Go, no Python runtime, decision 0088) | `script/gate-affected_test.sh` via `make gate-affected-test` (a shell test over `testdata/fixture` in a scratch repository with a recording go-test command: clean tree runs nothing; a core edit selects core and leaf; a deleted `core/core.go` is a `frontier` line for core, mid, and leaf; a document no package reads beside a core edit is `data`, does not fall back, and adds no package; a dirty `go.mod` falls back; a committed edit under `BASE` selects; an unresolvable base falls back; an interrupted planner exits nonzero without running go test); `TestSelectPackagesAttributesEveryDirtyPath` (a control character in a dirty path falls back) in `tools/gate-affected-select/main_test.go`; `TestSelectPackagesRejectsSiblingModulePrefix` (a package path that only shares the module's characters as a string prefix, with no `/` boundary, falls back instead of being selected); the 50-commit replay in AFP-V0-011 |
| AFP-V0-012 | `indexRepository`, `scanSource`, `escapesPackage`, `dependents`, `readers`, `enclosing`, `unresolved`, `namesPath` in `tools/gate-affected-select/readers.go`; the per-path loop in `selectPackages` (decision 0131) | `TestSelectPackagesAttributesEveryDirtyPath` in `tools/gate-affected-select/main_test.go` (a deleted source widens to its importers; a Go file read as data selects its reader; a document selects the package that names it; a testdata fixture selects its enclosing package; a `runtime.Caller` package is selected on every dirty path; a nested module's literals select nothing); `TestSelectPackagesFallsBackWhenAttributionFails` (imports that do not parse fall back); `TestSelectPackagesReachesEmbeddingAncestorDependents` (a data path under an embedding ancestor reaches that ancestor's dependents); `TestSelectPackagesResolvesAliasedAndDotRootLocatorImports` (an aliased or dot-imported `runtime.Caller` still marks the package unresolved); `TestIndexRepositoryFailsClosedOnSymlinkedGoFile` (a symlinked `.go` file falls back instead of being silently skipped) |
| AFP-V0-022 | `.github/cishards`, `tools/corvint-pr-tests`, `tools/ci-shard-costs`, CI and qualification workflows | `TestAFPMixedAdmissionPreservesSelectedUnion`, `TestAFPCompleteBalancedPartition`, `TestAFPIsolatedBuildIgnoresModuleRedirection`, `TestShardedPRExecution_AFPV0022`, `TestAFPV0022RefreshFromHostedOutcomes`, `TestAFPV0022RefreshRefusesUnusableInput`, `TestAFPV0022DriftCheck`, `TestAFPV0022OrderRunsSelectedUnitsFirstWithoutChangingTheSet`, `TestAFPV0022OrderFallsBackToTheCurrentOrder`; hosted timing and narrowing qualification NOT_RUN |
| AFP-V0-023 | `ReadScopesPath`, `InReadScope`, `ValidReadScopeEntry`, `Graph.scopedReadersOf`, `WitnessDeclaredReadScope`, `Unit.ReadScoped`, `Unit.ReadScope` in `internal/liveverify/affected`; `readScopes`, `applyReadScopes`, `FrontierReadScopesInvalid` in `internal/liveverify/affected/golang/readscopes.go`; `readScopes`, `inReadScope` in `internal/gateselect/readscopes.go`; `.github/testconfine` (`Load`, `Rules`, `ExecConfined`, `ConfinedEnv`, `cmd`) and the full-run `-exec` in `.github/workflows/ci.yml` | `TestDeclaredReadScopeNarrowsAnUnboundedReader_AFPV0023`, `TestInvalidReadScopeDeclarationDeclaresNothing_AFPV0023`, `TestGraphDigestIsTheDomainTaggedProjection_V1_0299`, `TestSelectPackagesHonorsDeclaredReadScopes`, `TestLoadAcceptsOnlyTheExactGrammar_AFPV0023`, `TestRulesGrantOutsideRootPackageAndEntriesOnly_AFPV0023`, `TestSelectionOnTheLiveDirtyWorktree`, `TestConfinedEnvTurnsOffVCSStamping_AFPV0023`, `TestExecConfinedDeniesUndeclaredRepositoryReads_AFPV0023` (Linux; run on a kernel reporting Landlock ABI 4, including granted and refused reparenting); per-package declarations verified under the wrapper in a Linux container (build log 2026-10-01-declared-test-read-scopes); hosted confined full run NOT_RUN until the protected workflow change is admitted |
| AFP-V0-013 | `tools/corvint-pr-tests` and `.github/workflows/ci.yml` | `TestSelectedFailureAndFallback`, `TestInterruptionLeavesNoLiveDescendant`; trusted pins empty, hosted execution unavailable |
| AFP-V0-015 | `tools/corvint-pr-tests/container.go` and indexed shadow execution; `admittedGoVersions` in `tools/corvint-pr-tests/main.go`, the one Go version set both `toolIdentity` and `containerRunResult` admit (V1-1520, V1-2194) | `TestContainerProfileAndArchive`, `TestColdRuntime`, `TestFrozenRowIndex`, `TestDockerCLIInterruption`, `TestContainerCleanupRefusal`; `TestContainerRunResult` admits a `go1.27.2` container result and refuses a coherent `go1.27.0` one (mutation: the old `go1.27.0` literal fails the admitted case); real Linux row/hosted NOT_RUN |
| AFP-V0-024 | `tools/ci-reuse-plan` (`admits`, `Decide`, `recorded`, `readRuns`); `docs-plan` and `ci-tested-tree-record` in `.github/workflows/ci.yml` | `TestAFPV0024ReusesOnlyAnExactTreeRecordedByEveryShard`, `TestAFPV0024AnythingElseRunsInFull`, `TestAFPV0024PushReusesTheQueueRunOfThePushedCommit`, `TestAFPV0024QueueRunRefusalsRunInFull`, `TestAFPV0024UnreadableListingContributesNoRun`, `TestAFPV0024WorkflowShardCountsAgree`, `TestAFPV0046WorkflowWiresQueueReuse`; local replay of the push step against the live API returned FULL; the V1-0760 replay is in `docs/build-log/2026-10-13-ci-identical-tree-reuse.md`; hosted reuse `NOT_OBSERVED` |
| AFP-V0-025 | `tools/unbounded-readers`, `.corvint/unbounded-readers.json`, `make unbounded-readers-check`; `Graph.UnboundedReaders`; `ShareOf` in `.github/cishards/order.go`; `doc-gates` and `go-product-shard` in `.github/workflows/ci.yml` | `TestAFPV0025RatchetFailsOffTheRecordedSet`, `TestAFPV0025ConcurrentAdditionsMergeToAPassingRecord`, `TestAFPV0025RatchetWithoutARecordRefuses`, `TestAFPV0025ShareReportsSelectedEstimatedTime`; hosted share report `NOT_OBSERVED` until this change's own CI run |
| AFP-V0-026 | `merge_group` trigger in `.github/workflows/ci.yml`; `merge-group` job in `.github/workflows/ci-control-plane.yml` | `actionlint`; `make ci-least-privilege-check`; hosted merge-queue run `NOT_OBSERVED` until the owner enables the queue; the identical-tree exception is traced under AFP-V0-046 |
| AFP-V0-027 | `go-product-shard` and `go-static` job conditions and `go-product` message in `.github/workflows/ci.yml` | `actionlint`; `make ci-least-privilege-check`; hosted constituent run with the label `NOT_OBSERVED` until the label exists and a batch uses it |
| AFP-V0-031 | `nestedModules`, `observedReach`, `unknownDirectory`, `nestedModuleOpen`, `readManifest`, `workspaceAbove`, `replaceDirectoryOpen`, `repositoryDirectory`, `parseManifest`, `manifestLine`, `maxNestedManifestBytes`, `observeModules` in `internal/liveverify/affected/golang` | `TestIndependentNestedModulesCloseTheFrontier_V1_0867`, `TestNestedModuleThatCanReachTheRootKeepsTheFrontier_V1_0867`, `TestUnreadableOrUnparsableNestedManifestKeepsTheFrontier_V1_0867`, `TestNestedFrontierReadsTheSuppliedSource_V1_0867`; `TestWorkspaceModulesAreUnitsUnderTheirOwnModulePath` and `TestWorkspaceDirtySourceSelectsTheOtherModulesTest` over `testdata/workspace/stray`, which now requires a listed module; survey replay in `docs/build-log/2026-10-06-nested-module-frontier.md` (unchanged on this repository, because `tools/local-authority` requires the root) |
| AFP-V0-032 | `readAdviceSource`, `mandatoryAffectedChecks` in `cmd/corvint/affected.go` | `TestAffectedAdviceUnreadableDeclarationSuppressesNoGate` (`cmd/corvint/affected_advice_unix_test.go`) |
| AFP-V0-033 | `unreasoned` in `tools/unbounded-readers`; the `reasons` of `.corvint/unbounded-readers.json` | `TestAFPV0033EveryUnitNamesItsUnboundedRead` (a live or stale unit without a reason fails and is named; both failures are reported together); `TestAFPV0025ConcurrentAdditionsMergeToAPassingRecord` (each addition carries its reason); `make unbounded-readers-check` passes with a reason for every unit; per-package container measurements in build log 2026-10-06-unbounded-reader-reasons |
| AFP-V0-028 | Not implemented; planned queue rules and PR narrowing integration | `TestAFPV0028QueueBackstoppedNarrowing` (PLANNED): three new FULL conditions, UNKNOWN allowlist, trusted pins without historical artifact, partition/argv/environment/confinement/cache parity and summaries; hosted queue checks, main identity/seal and V1-0760 prerequisite `NOT_OBSERVED` |
| AFP-V0-029 | Retained selections, queue and PR outcomes and the default-branch `forward-rows` job of AFP-V0-047: `.github/forward-rows/row.jq` (classifier), `classify.sh`, `collect.sh` | `TestAFPV0029ForwardRowsAndShadowBar`: S/F classification on fixtures derived from queue run 38485555207 and PR run 38482995697, `SELECTED_FAILED_IN_QUEUE`, FLAKY, MISS (rerun failed, `SKIPPED_BOUND`, interrupted, missing), INCOMPLETE inputs, REUSE outcomes and the 64 KiB bound; the whole-epoch 20/5 bar, its coverage rule (`NOT ESTABLISHED` when the listing did not reach the epoch's start) and the separate latest-20 kill-switch window in `TestAFPV0047BarCountsArtifactNames` (owner chat, 2026-10-20; V1-0812); narrowing admission and hosted shadow/queue rows and delivery `NOT_OBSERVED` |
| AFP-V0-030 | The latest-20 window and kill switch in `.github/forward-rows/bar.jq`; the trusted verdict (`verdict.sh`) and the live 5-page read in `accept.sh`; the `vars.CORVINT_PR_NARROWING == 'on'` conditions in `.github/workflows/ci.yml` | `TestAFPV0047BarCountsArtifactNames` (window, thresholds); `TestAFPV0097VerdictAcceptance` (a tripped kill switch, a live MISS); `TestAFPV0097WorkflowBindings` (variable and fork guard); hosted disable/re-arm `NOT_OBSERVED` |
| AFP-V0-014 | `tools/corvint-pr-tests/shadow.go` | `TestQualificationAndTerminalFailures`, `TestToolIdentityRequiresCurrentGoVersion`; frozen 200-row qualification NOT_RUN |
| AFP-V0-016 | `.github/workflows/ci-control-plane.yml`; the `main` repository ruleset | `actionlint`; `success` posted on PR #26 (run 35444060752) and PR #24 (run 35446378936); ruleset 23699808 active with the decision 0320 settings; the decision 0390 settings (no bypass, `doc-gates` required) and the admin-status consent path NOT_VERIFIED until the owner applies them; `failure` path NOT_RUN on a real PR |
| AFP-V0-017 | `.github/workflows/pr-tests-qualification.yml` | `actionlint`; dispatch NOT_RUN (`main` has fewer than 201 first-parent commits) |
| AFP-V0-019 | `internal/plansnapshot`, `compileSnapshotAffected` | `TestSnapshotImmutableBytesAndCleanup`, `TestSnapshotRejectsIncompleteMismatchedAndStale`, `TestSnapshotStrictWire`, `TestSnapshotRejectsLinksAndIgnoresArchiveAttributes`, `TestAffectedSnapshotMatchesCommittedPlanAcrossDirtySources`, `TestAffectedSnapshotPlaywrightPinsConfigAndSource` |
| AFP-V0-018 | `playwrightAffectedReceipt`, `compilePlaywrightAffected`, and `typescript.SelectPlaywright` | `TestAffectedPlaywrightProfileEmitsProjectDistinctUnits`, `TestAffectedPlaywrightArgumentsFailClosed`, and `internal/liveverify/affected/typescript/playwright_test.go` |
| AFP-V0-009 | `affectedAdvice`, `compileAffectedAdvice`, `mandatoryAffectedChecks`, `advisoryAffectedChecks`, `agentsVerifyCommands`, `agentsVerifyScopes`, `scopeConditionalChecks`, `markdownListItem`, `markdownFence`, `agentsCheck`, `nonTerminatingCommand`, `stripShellComment`, `shellQuoteJoin` in `cmd/corvint/affected.go` | `TestAffectedAdviceJoinsMandatoryGateAndAdvisoryPackages`, `TestAffectedAdviceReportsNoDeclaredGate`, `TestAffectedAdviceKeepsMandatoryGateAndNeverAdvisesExclusions`, `TestAffectedReceiptMembersAreClosedAndByteStable` (tightened to assert `advice`'s raw JSON key order), `TestAffectedAdviceBoundsTheDeclarationRead`, `TestShellQuoteJoinEscapesMetacharacters`, `TestAffectedAdviceTruncatedMandatoryDeclarationSuppressesNoGate`, `TestAffectedAdviceCapsMandatoryChecksAtSixteen`, `TestAffectedAdviceSkipsCommentsInVerifyFence`, `TestAffectedAdviceTakesOnlyTheExactVerifyHeading_V1_0342`, `TestAffectedAdviceLabelsScopedGateConditional_V1_0682`, `TestAffectedEmptyDiffIsABoundedEmptyPlan_V1_2126` (no Go advice gap over an empty dirty set) |
| AFP-V0-021 | `WitnessPathLiteralReader`, `PathTokenBound`, `Graph.readers`, `Graph.tokenBounds`, `namesPath`, `ChangeEvidencePath`, `Graph.resolves`, `resolvesWithin`, `WitnessUnboundedReader`, `Graph.unboundedReadersOf` in `internal/liveverify/affected` (`select.go`, `readers.go`, `graph.go`); `Unit.PathTokens`, `Unit.PathTokensBounded`, `Unit.UnboundedReads`, `Unit.LocatesRoot`; `pathTokens`, `wholeShortEnds`, `importsEnd`, `ignoredByGo`, `maxPathTokens` in `internal/liveverify/affected/golang/golang.go`; `escapesPackage`, `rootLocatorCall` in `internal/liveverify/affected/golang/unbounded.go` | `TestPathLiteralSelectsItsReaderPackage_AFPV0021` (a named document selects its reader and stays unknown; single and parenthesized imports are no tokens; a file without imports yields tokens; a dependent and an unnamed path select nothing), `TestOwnedDirtyPathSelectsTheUnitsThatNameIt`, `TestReaderWitnessIsTheSmallestNamingDirtyPath`, `TestReaderReachedByDependencyKeepsItsDependencyWitness`, `TestBoundedPathTokensAreUnknownOnlyWhenAMatchIsAttempted`, `TestPathTokenBoundNamesThePackage`, `TestUnlexableSourceIsAFrontierOutsideIgnoredDirectories`, `TestSelectionOnTheLiveDirtyWorktree` (reader witnesses resolve), `TestAffectedDocumentSelectsThePackageThatNamesIt` (receipt shape, provider packages, byte identity), `TestDirectoryShapedLiteralNamesNoPath` (V1-0290: a directory-shaped one-component token names no path; two-component and file-name tokens still select), `TestChangeEvidenceReadersAreNarrowed_V1_0230` (the sidecar keeps only resolving readers; a climbing token names a directory; a same-shaped path is not narrowed), `TestUnboundedReaderIsSelectedOnAnyChange_V1_0230` (rule (d): root locators through plain, aliased and dot imports, a climbing literal and a test-only `--show-toplevel` are selected with their non-test locator's dependents, not the test-only one's; a clean plan selects none), `TestShortFragmentsDoNotNamePaths_V1_1818`, `TestShortFragmentsKeepConcatenatedReaders_V1_1818` (V1-1818: a delimited short component is whole; `"docs/a" + "lpha.go"` keeps its reader and build-constraint frontier) |
| AFP-V0-020 | `UnknownNoSelectableTest` in `affected.Select` (`internal/liveverify/affected/select.go`) | `TestSelectNamesChangedUntestedGoPackageAsUnknownScope`, `TestSelectTraversesUntestedUnitsWithoutSelectingThem` (an untested unit the change only reaches stays bounded), `TestSeamWidensWhenNoTestReachesAChangedUnit_AFPV0020` (every plugin), `TestPlaywrightDiscoveryReconciliation` (an unreached helper keeps the Playwright plan), `TestAffectedUntestedGoPackageIsUnknownScope` |
| AFP-V0-034 | `readSourceFile` (`internal/liveverify/affected/read_unix.go`, `read_other.go`), `ReadSource` (`walk.go`) | `TestReadSourceRefusesNonRegularFilesOnOpenDescriptor` (regular file read; symlink, directory, FIFO, unix socket and mode-0 directory refused as `ErrInvalidUnit` without blocking; a sparse body over `MaxSourceBytes` refused as `ErrWalkLimit`; a missing path reports `fs.ErrNotExist`) |
| AFP-V0-035 | `compactAffectedReceipt`, `summarizeAffectedExclusions` in `cmd/corvint/affected_compact.go`; `--full` in `parseAffectedOptions`; profile checks in `internal/companionrelease/core_smoke.go`, `.github/cishards/order.go`, `tools/corvint-pr-tests/main.go`; `--full` in `tools/retrieval-bench/main.go` | `TestAFPV0035CompactDefaultPlanSummarizesTheFullPlan`; the `--snapshot` default/`--full` parity in `TestAffectedSnapshotMatchesCommittedPlanAcrossDirtySources`; `TestAFPV0035OrderReadsTheCompactDefaultPlan`; core-freeze goldens `affected-*.json` and `affected-full-*.json` |
| AFP-V0-036 | `testOnlyGoUnits`, `reach`, `seed` in `internal/liveverify/affected/select.go` | `TestGoTestOnlyChangeSelectsItsPackageButNotItsImporters_V1_0984` in `internal/liveverify/affected/affected_test.go` (a test-only change selects its package alone plus the unbounded reader; a source change, and a source-and-test change witnessed by the source path, reach the importer and the test user; a test change beside a dependent's source change does not reach the test user; a non-Go plugin's test-only change still traverses) |
| AFP-V0-037 | `Unit.Execs`, `Graph.execUsers`, `Graph.builtCommands`, `Graph.execUsersOf`, `WitnessBinaryExec`, `BinaryExecsPath` in `internal/liveverify/affected` (`unit.go`, `graph.go`, `execs.go`, `select.go`); `applyBinaryExecs`, `literalExecs`, `commandDirectory`, `readBinaryExecs`, `matchBinaryExecs`, `FrontierBinaryExecsInvalid` in `internal/liveverify/affected/golang/binaryexecs.go`; `moduleLevelFrontiers` in `tools/gate-affected-select/main.go`; `.corvint/test-binary-execs.json` | `TestBinaryExecConsumerIsSelectedWithTheCommandsBuild_AFPV0037` (anchored, plain, climbing and module-path literals and a declared runner become `execs`; a change to the command or a package it imports selects every consumer as `BINARY_EXEC` through the command; a literal naming a file under the command directory is no edge; an unrelated change selects no consumer; byte-identical plans), `TestInvalidBinaryExecDeclarationDeclaresNothing_AFPV0037` (nine invalid declarations raise only the frontier, drop declared edges and keep literal ones; a missing one raises nothing) |
| AFP-V0-038 | `compactAffectedAdvice`, `affectedCompactCheck`, `adviceAdvisoryGoTest` in `cmd/corvint/affected_compact.go` and `cmd/corvint/affected.go` | `TestAFPV0038CompactAdviceReferencesProviderPackages` (fixture default vs `--full`, a quoted package path, a non-matching command kept whole); advice resolution in `TestAFPV0035CompactDefaultPlanSummarizesTheFullPlan` and `TestAffectedAdviceJoinsMandatoryGateAndAdvisoryPackages`; core-freeze golden `affected-committed-range.json` |
| AFP-V0-039 | command-local `-c maintenance.auto=false -c gc.auto=0` in the Git helpers of `internal/liveverify/affected/observation_test.go`, `internal/liveverify/affected/golang/golang_test.go`, `internal/liveverify/affected/typescript/mocha_qualification_test.go` and `internal/liveverify/pymutate/pymutate_test.go`; `unguardedFixture` in `internal/liveverify/affected/fixture_maintenance_test.go` | `TestLiveVerifyGitFixturesDisableDetachedMaintenance` (fails on the three unfixed helpers and on the pre-c4f9604d observation helper), `TestUnguardedFixtureDetectsAMissingSafeguard`; `GIT_TRACE2_EVENT` child-launch counts in build log 2026-10-08-liveverify-fixture-maintenance; hosted Linux Git 2.55 cleanup NOT_RUN |
| AFP-V0-040 | `check --advisory` (`report`, `observeStream`, `scan`, `checkEvent`, `findings`, `finding.misplaced`, `escape`) in `tools/ci-shard-costs`; the best-effort capture in the tests step and the shard-outcome retention step of `go-product-shard` and the `ci-shard-cost-drift` job in `.github/workflows/ci.yml` | `TestAFPV0040AdvisoryReportNeverFailsOnFindings`, `TestAFPV0040AdvisoryWarningsStayWithinTheStepLimit`, `TestAFPV0040AdvisoryAcceptsAFullEventStream`, `TestAFPV0040AdvisoryAbstainsOnPartialOrUnusableInput`; `actionlint`; `make ci-least-privilege-check`; local dry run of the job steps against run 38055182050 (build log 2026-10-10-ci-shard-drift-detection); hosted run `NOT_OBSERVED` |
| AFP-V0-041 | `Slice`, `Shard`, `Allowed`, `SplitPackages`, `Splits`, `Plan`, `PlanFrom`, `profileNames` and `--slices` in `.github/cishards`; `.github/cishards/test-split-allow.json`, `.github/cishards/test-slices.json`; `shardLayout` and `shardUnits` in `tools/corvint-pr-tests` (AFP-V0-048); `tools/ci-test-slices`, with `inGroup` in its build-tagged `procgroup_unix.go` and `procgroup_other.go`; `outcomes`, `combine`, `--allow` and `--slices` in `tools/ci-shard-costs`; the full-run slice loop through `run_tests` in `.github/workflows/ci.yml` | `TestAFPV0041PlanEqualsPartitionWithoutSplits`, `TestAFPV0041SlicedPlanIsCompleteAndDeterministic`, `TestAFPV0041FallbackKeepsPackagesWhole`, `TestAFPV0041SlicesRunEveryTestExactlyOnce` (real `go test` over added, removed and renamed names, subtests, a parallel test, a fuzz seed and an example), `TestAFPV0041CommittedSlicesAreAdmitted`, `TestAFPIsolatedBuildIgnoresModuleRedirection` (`--slices`), `TestAFPV0048DriverPlacesTheFullBranchUnits`, `TestAFPV0041GenerateSplitsOnlySlowListablePackages`, `TestAFPV0041GenerateRefusesUnboundInputs`, `TestAFPV0041GenerateEnumeratesOnlyTheRevision`, `TestAFPV0041ObserveRefusesIncompleteOrFailedLogs`, `TestAFPV0041InterruptRemovesTheCheckout`, `TestAFPV0041CancelStopsTheEnumerationGroup` (a fake `go` whose sleeping child ignores SIGINT), `TestAFPV0041ListParsing`, `TestAFPV0041SliceWithoutTimesSplitsByCount`, `TestAFPV0041MinMaxCutsIsOptimal`, `TestAFPV0041ReplayPredictsShardSums`, `TestAFPV0041RefreshCombinesTestSlices`, `TestAFPV0041AdvisoryCombinesTestSlices`; replay of hosted run 38055182050, a local run of both store slices, and a local run of the workflow's slice loop and capture whose streams `refresh` and `check --advisory` combine, in build log 2026-10-10-ci-test-level-shards; hosted sliced run, paired timing and per-slice overhead NOT_RUN; owner acceptance in decision 0488 |
| AFP-V0-042 | the `Windows vet including test files (AFP-V0-042)` step and `fetch-depth: 2` of `go-static` in `.github/workflows/ci.yml` | `actionlint`; `make ci-least-privilege-check`; local fixture run (build log 2026-10-10-windows-test-vet); hosted run `NOT_OBSERVED` until this change's CI runs |
| AFP-V0-043 | `run` strict per-log `observeStream` in `tools/ci-shard-costs/main.go` | `TestAFPV0043RefreshRefusesMalformedLine`, `TestAFPV0043RefreshRefusesTruncatedFinalEvent`, `TestAFPV0043RefreshRefusesUnfinishedPackage`, `TestAFPV0043RefreshRefusesEmptyShard`, `TestAFPV0043RefreshValidStreamByteParity` |
| AFP-V0-044 | `affectedMixedWarning` in `cmd/corvint/affected_freshness.go`; `Warning` in `affectedReceipt`, `playwrightAffectedReceipt`, `affectedCompactReceipt` | `TestAffectedMixedRangeWarning` (warning on /1 and /0 with a dirty path outside the range; none for a clean worktree, the worktree-only form or a range inside the dirty set; no mutable content), `TestAffectedPlaywrightFreshness` (Playwright receipt warning; none clean or worktree-only) |
| AFP-V0-045 | `RequireClean` parsing in `parseAffectedOptions`; `affectedMixedWorktreeRefusal` in `compileAffected` and `compilePlaywrightAffected` | `TestAffectedRequireClean` (clean byte parity; untracked entry refuses on the worktree, `--full --base` and provider forms before the provider runs, with no write; value, repetition, `--snapshot` and `discovery` refused), `TestAffectedPlaywrightFreshness` (Playwright clean byte parity and dirty refusal with no write) |
| AFP-V0-094 | `addAffectedPath` and the `--path` combination check in `parseAffectedOptions`; `affectedPathListMixedRefusal`, `affectedPathListIgnoredCheck` over `affected.IgnoredPaths`, and the `PathList` branch in `compileAffectedGraph`; `path_list` in `affectedReceipt` and `affectedCompactReceipt` | `TestAFPV0094PathListSelectsTwoHopImportersAtHead` (HEAD revision, empty range, no warning, `plan` byte-identical to the `--base` plan in compact and `--full`, repeated path collapses, no `path_list` without the flag), `TestAFPV0094PathListRefusals` (absolute, parent, empty and missing values; `--base`, `--snapshot`, `--provider`, `--playwright-config`; dirty worktree refuses with the HEAD commit and no write), `TestAFPV0094PathListRefusesIgnoredFilesTheGraphReads` (an ignored `core/generated.go` refuses whether listed or only read), `TestAFPV0094PathListRefusesIgnoredFilesUnitsRead` (an ignored file under a unit's `testdata/`, embedding or read-scoped package directory, including an empty `core: []` scope, or declared read scope refuses; an unread ignored `build/out.bin` plans), `TestAFPV0094PathListRejectsOptionLookingSeparatedValue` (`--path --path=..`, `--path --full`, `--path -x` refuse; `--path=-dash.txt` plans) |
| AFP-V0-095 | `affected.Select` reached unchanged through `compileAffectedGraph` | `TestAFPV0094PathListSelectsTwoHopImportersAtHead` (leaf selected through an untested mid with witness core, mid, leaf), `TestAFPV0095PathListKeepsUnknownScope` (an unowned listed path keeps `UNKNOWN` with `UNOWNED_DIRTY_PATH`) |
| AFP-V0-096 | `impactHelp` and `affectedHelp` in `cmd/corvint/help.go`; `docs/CAPABILITIES.md` `impact` and `affected` rows | `TestAFPV0096HelpStatesImpactClosureDepth` |
| AFP-V0-097 | `.github/workflows/ci-forward-bar.yml`; `.github/forward-rows/verdict.sh`, `accept.sh`, `names.sh`; the `--arg format json` output of `bar.jq`; the `narrowing-verdict` job in `.github/workflows/ci.yml` | `TestAFPV0097VerdictWrittenOnMain` (refused off main; MET, epoch start, narrowed count; NOT MET with a MISS); `TestAFPV0097VerdictAcceptance` (MET; each of (a) to (d) failing, no artifact, newest by `created_at`, unparseable, another run, failed download, live MISS, a page budget that ends with no row, a MISS beyond the scanned pages, scanned pages under 20 rows give FULL; scanned pages that reach the verdict with 20 rows give MET); `TestAFPV0097WorkflowBindings` (equal epochs, triggers, read-only permissions, guards; the tests step runs no driver and no qualification pin remains); hosted run `NOT_OBSERVED` |
| AFP-V0-098 | `--mode plan` (`planOnly`) in `tools/corvint-pr-tests/main.go`; `.github/forward-rows/narrow.sh`, `narrow.jq`; the narrowing gate and the tests-step filter of `go-product-shard` in `.github/workflows/ci.yml` | `TestAFPV0098PlanModeExecutesNothing`; `TestAFPV0098SupersetGuard` (superset narrows the shard to its intersection; an omitted shadow package and every other FULL reason run the whole shard); `TestAFPV0098SupersetGuardFilter` (the ci.yml filter narrows only a retained NARROWED record equal to its sidecar; an empty mode, FULL, unretained, merge-queue, missing, unreadable, empty or disagreeing sidecar or record runs the whole shard; a verified empty list runs none); `TestAFPV0098SupersetGuardSidecar` (narrow.sh writes the sidecar equal to the record; an unwritable sidecar is FULL with no record); hosted run `NOT_OBSERVED` |
| AFP-V0-099 | `NARROWED` in `row.jq`, `classify.sh` (`ROW_NARROWING_DIR`), `collect.sh` and `bar.jq`; the `ci-narrowing-<index>` retention step in `.github/workflows/ci.yml` | `TestAFPV0099NarrowedRows` (narrowed-out queue failure MISS despite a passing rerun; FULL record clean; another run, malformed or unreadable record INCOMPLETE; PR row NARROWED); `TestAFPV0099CollectorFetchesNarrowing`; `TestAFPV0099BarExcludesNarrowedRows`; hosted run `NOT_OBSERVED` |
| AFP-V0-046 | `admits` and `Decide` in `tools/ci-reuse-plan/main.go` (`--event merge_group`); the `reuse` step, its first-parent tool build and the `ci-tested-tree-record` job in `.github/workflows/ci.yml` | `TestAFPV0046QueueDecisionAdmitsOnlyThePullRequestRun`, `TestAFPV0046WorkflowWiresQueueReuse`, `TestAFPV0024ReusesOnlyAnExactTreeRecordedByEveryShard`, `TestAFPV0024AnythingElseRunsInFull`; `actionlint`; local replay against the live API in `docs/build-log/2026-10-13-ci-identical-tree-reuse.md`; hosted REUSE queue run `NOT_OBSERVED` |
| AFP-V0-170 | `recorded`, `recordLine`, `RecordJob` and the `--jobs`/`--logs` reads in `tools/ci-reuse-plan/main.go`; the `ci-tested-tree-record` job and the `reuse` step of `docs-plan` in `.github/workflows/ci.yml`; the `job:docs-plan` row of `script/ci-least-privilege-exceptions.tsv` | `TestAFPV0170OnlyTheRecordJobLogVouchesForTheTree`, `TestAFPV0170RecordJobRunsNoRepositoryCode`, `TestAFPV0170ReuseStepRefusesForgedArtifacts` (fails against the artifact-trusting step); `make ci-least-privilege-check`; hosted record job and REUSE `NOT_OBSERVED` until merged |
| AFP-V0-171 | the status read before a `failure` post in the `control-plane` job of `.github/workflows/ci-control-plane.yml` | `TestAFPV0171ControlPlaneKeepsAUserPostedSuccess` (fails against the earlier job); hosted rerun after an admin post `NOT_OBSERVED` |
| AFP-V0-047 | `CORVINT_TEST_SHADOW_SOURCE`, the shadow checkout and build, the selection document and the outcomes, rerun and upload steps in `.github/workflows/ci.yml`; the `forward-rows` job in `.github/workflows/ci-control-plane.yml`; `.github/forward-rows/` (`selection.jq`, `outcomes.sh`, `outcomes.jq`, `collect.sh`, `classify.sh`, `row.jq`, `bar.sh`, `bar.jq`) | `TestAFPV0047ShardOutcomes` (real flaky PR run 38476173623 shard 7 stream and its rerun; bound of 8), `TestAFPV0047SelectionDocument`, `TestAFPV0047CollectorNamesRows`, `TestAFPV0047BarCountsArtifactNames`, `TestAFPV0047WorkflowBindings`, `TestAFPV0029ForwardRowsAndShadowBar`, `TestSTRV0012CIConstantsMatchFullBranch`; `actionlint`; `make ci-least-privilege-check`; read-only replay against hosted runs in `docs/build-log/2026-10-13-ci-forward-rows.md`; hosted rows `NOT_OBSERVED` |
| AFP-V0-048 | `planShards`, `shardLayout`, `shardUnits`, `sliceRun` and the slice loop of `execute` in `tools/corvint-pr-tests/main.go` | `TestAFPV0048DriverPlacesTheFullBranchUnits` (2 to 8 shards: each shard equals `PlanFrom`, every package or slice exactly once, narrowed selections with a split package), `TestAFPV0048ZeroSlicesKeepTheIntersect`, `TestAFPV0048MismatchedSliceFileMatchesTheFullBranch`, `TestAFPV0048CommittedSlicesArePlacedOnce`, `TestAFPV0048DriverRefusesAmbiguousPlacement`, `TestAFPV0048ShardedRunExecutesSlices` (real `go test`: FULL, narrowed and stale-name runs, each test once over the shards; a lost slice refuses with no execution record), `TestShardedPRExecution_AFPV0022`; a mutation check that drops the slice placement, in `docs/build-log/2026-10-14-v1-1361-driver-runs-slices.md`; hosted sharded driver run `NOT_OBSERVED` until V1-0812 sets the pin |
| AFP-V0-049 | `observation.ends`, `combine` and `placedOnce` in `tools/ci-test-slices/main.go`; `generate` and `replay` read the tested tree's allow-list and slice file; `.github/cishards/test-slices.json` regenerated from hosted run 38485555207 (go1.27.2) | `TestAFPV0049ReplayCombinesSlicedRuns`, `TestAFPV0049GenerateRegeneratesFromASlicedRun`, `TestAFPV0049RefusesEveryOtherRepeat`, `TestAFPV0049ReplayRefusesAPlanThatMisplacesAUnit` in `tools/ci-test-slices/combine_test.go`; mutation check and before/after replay in build log 2026-10-14-v1-1371-slices-split-outcomes |
| AFP-V0-050 | `Graph.WithReadScopeBase`, `Graph.declarationChanged`, `inDeclaredScope` and `Graph.scopedReadersOf` in `internal/liveverify/affected`; `CommittedReadScopes` in `internal/liveverify/affected/golang/readscopes.go`; `affectedPriorReadScopes` in `cmd/corvint/affected.go`; (4) `baseReadScopes`, `baseOwner`, `walkedDirectory`, `declaredBears` and `inDeclaredScope` in `internal/gateselect/readscopes.go`, `Receipt.Range`, `Receipt.Revision`, `Receipt.Snapshot` and the declared loop of `selectPackages` in `internal/gateselect/select.go` | `TestReadScopeEditSelectsOnlyChangedDeclarations_AFPV0050` (added, changed and removed keys, a package declaring the declaration's directory, unchanged keys left out, the unknown-base fallback), `TestCommittedReadScopesFailClosed_AFPV0050` (absent, malformed, wrong-profile, packageless-directory and missing-commit bases), `TestCommittedReadScopesFollowModuleOwnership_AFPV0050` (a base declaring a directory a nested or unlisted module owns, a testdata fixture module, or a hidden or skipped directory, is unknown; a go.work-listed module, an admitted `build` directory and a package beside a directory named `go.mod` are read; each agrees with the observed tree, V1-1400), `TestAffectedAdditiveReadScopeEditSelectsOnlyTheNewPackage_AFPV0050` (CLI `--base` over an additive commit and a changed key); mutation checks and the 199 to 67 unit re-measurement in build log 2026-10-15-affected-batch-1309-1310-1298; (4) `TestSelectPackagesNarrowsDeclarationEditsAgainstTheBase_AFPV0050` (added, changed and removed keys, a package declaring `.corvint/`, `range.base` over `revision`, and no, missing, abbreviated or `--snapshot` bases selecting every declared package), `TestBaseReadScopesFollowCommittedReadScopes_AFPV0050` (17 ownership, walk, testdata, regular-file and grammar cases, each agreeing with `CommittedReadScopes`), `TestSkippedDirectoriesMirrorTheObserverWalk`, `TestSelectorNarrowsDeclarationEditAgainstThePlanBase_AFPV0050` (`tools/gate-affected-select`), `TestCorvintTestNarrowsReadScopeDeclarationEdit_AFPV0050` (`corvint test --dry-run`); BASE-versus-fixed counts and mutations in build log 2026-10-15-v1-1399-gateselect-narrowing |
| AFP-V0-052 | The job-list read in `.github/forward-rows/collect.sh`, `infra` and `$explained` in `row.jq`, `ROW_INFRA` in `classify.sh`, the `INFRA` window in `bar.jq` | `TestAFPV0029ForwardRowsAndShadowBar` (INFRA on the run 38686348902 shape for a queue and a pull-request row; INCOMPLETE with no hosted failure named, a second missing shard, the named shard's outcomes present, a queue selection gap, a collector reason, a shard beyond the count and a non-FLAKY miss; INFRA keeps a FLAKY count), `TestAFPV0047CollectorNamesRows` (real job list of run 38686348902 names shard 5; unreadable job list and a shard whose tests failed stay INCOMPLETE), `TestAFPV0047BarCountsArtifactNames` (three INFRA rows ahead of 20 complete rows: MET, clear); BASE-versus-fixed counts and mutations in build log 2026-10-16-v1-1514-shadow-infra-failure-reason; hosted INFRA row `NOT_OBSERVED` |
| AFP-V0-051 | `FilePathTokens` in `internal/liveverify/affected/golang/golang.go`; `check`, `named`, `covered`, `locatesRoot`, `tracked`, `readLiterals`, `joinRuns` (7), `goListTargets`, `dependencyDirectories` and `dependencyCovered` (8) in `tools/test-read-scopes-check`; `.corvint/test-read-scope-literals.json`; the `test-read-scopes-check` Makefile target in `GATE_STEPS` and the `doc-gates` step list of `.github/workflows/ci.yml` | `TestUndeclaredRepositoryLiteralFailsNamingPackageAndPath_V1_1351` (the V1-1340 shape: a root-locating package naming the tamper matrix with the entry, its parent or a file below it declared, removed, or only a sibling declared; a recorded and a stale literal; blank-reason, empty-package, unknown-member and invalid-declaration refusals; a non-root-locating package, the package's ancestor, a case-folded literal and the change-evidence map never reported; a literal naming both an in-package and a root path reports the undeclared root path); the real tree with the `internal/cem/verify` tamper-matrix entry removed exits 1 naming the package and path, and the current tree exits 0; mutations in build log 2026-10-15-gate-tooling; (7) `TestJoinedSegmentsOutsideScopeFail_V1_2117` (the d114cc62e `next_action_test.go` shape, undeclared and declared; a run joined onto a fixture root is not resolved from the root); (8) `TestGoListDependencyOutsideScopeFails_V1_2037` (the #1288 transitive omission, a file entry under the directory only, and the declared directory); the real tree with `.github/cishards/` removed from `cmd/corvint-test` (V1-1736), `internal/cem/sim/` from `internal/gitnotes` (V1-2037) and every entry from `cmd/corvint-cem-candidate` (V1-2117) exits 1 naming all three; mutation: `dependencyCovered` replaced by `covered` passes the file-entry case |
| AFP-V0-053 | `toolIdentity` Git field and `gitIdentity` (Darwin xcrun shim resolution, `TestGitIdentityResolvesDarwinShim`, V1-1521), `testEnv`, the `execute` Git guards in `tools/corvint-pr-tests/main.go`; the resume identity check in `shadow` and `validateRow` in `tools/corvint-pr-tests/shadow.go` | `TestSelectedFailureAndFallback` (Git drift refuses execution; `frozenWithoutGit`: a corpus frozen without a Git identity or under another Git is refused before any checkout, and its row is not reused), `TestExecuteRequiresGitIdentity`, `TestTestEnvironmentSilencesGraftAdvice`, `TestExecuteGivesTestsSilencedGraftEnvironment`, `TestContainerRunResult`; controlled base/target replay in build log 2026-10-16-ci-shadow-historical-compat; hosted re-qualification NOT_RUN |
| AFP-V0-054 | `windows-vet` and `WINDOWS_VET_ENV` in `Makefile`; the `make windows-vet` fence and scoping sentence of the `AGENTS.md` Verify section | `TestAFPV0054WindowsVetTargetMatchesTheCIStep` (parity with the go-static step and its effective env, `GOENV=off`, the `go-version` prerequisite), `TestAFPV0054StepEnvOverridesJobEnv` (step-level env mutation), `TestAFPV0054WindowsVetIgnoresSavedGoEnv` (saved `go env -w` GOFLAGS), `TestAFPV0054AffectedListsWindowsVet` (conditional advice check from the real `AGENTS.md`), `TestAFPV0054WindowsVetFlagsUnixOnlyTestHelper` (unix-only helper fixture: native vet passes, the recipe fails); mutations and the `corvint affected` run in build log 2026-10-18-v1-1719-local-windows-vet |
| AFP-V0-066 | `advisoryRSpecCheck`, `adviceAdvisoryRSpec`, `adviceMaxRSpecFiles`, `adviceNoRSpecGemfile`, in `cmd/corvint/affected.go`; `adviceDeclaresRSpecGem`, `adviceNoRSpecGem` in `cmd/corvint/affected_rspec_gems.go` | `TestAFPV0066AdviceListsSelectedRSpecFiles`, `TestAFPV0066NoRSpecCommandWithoutGemfile`, `TestAFPV0066RSpecAdviceNeedsDeclaredRSpecGem`, `TestAFPV0066SymlinkedGemManifestDeclaresNothing`, `TestAFPV0066RSpecAdviceOrderBoundAndScope`, `TestAFPV0066DashLeadingSpecFileIsNeverAnOption`; the reproduction run in build log 2026-10-18-v1-1796-ruby-verification-command |
| AFP-V0-072 | `resolves` and `namesFileOnlyWithin` in `internal/liveverify/affected/readers.go` | `TestFileNameOnlyReadersMustPlaceThePath_AFPV0072` (temp-dir fixture dropped for `docs/specs/README.md`, kept in its subtree, at the root and when its tokens name every directory; anchored and multi-component readers kept), `TestNonGoFileNameReaderIsNotNarrowed_AFPV0072`; PR #1134 range 163 to 156 units in build log 2026-10-18-V1-1806-affected-overselection-1806 |
| AFP-V0-060 | `.github/cishards/test-slices.json` and `.github/cishards/package-costs.json` refreshed from run 39120414701 (revision 2ab0e744) | `TestAFPV0060CommittedSlicesBoundOversizedPackages`; `TestSTRV0007UniversePlacementEqualsCIPlan` and `TestSTRV0008BalancedPlacesEveryUnitOnce` over the committed files; `tools/ci-test-slices replay` in build log 2026-10-18-v1-1096-ci-slice-refresh; hosted run of this change; paired runs NOT_OBSERVED |
| AFP-V0-079 | the event-only conditions and `concurrency` groups of the `control-plane` and `merge-group` jobs in `.github/workflows/ci-control-plane.yml` | `TestAFPV0079ControlPlaneReportsEveryCompletedRun`; `TestAFPV0079CancelledRunStillPostsTheStatus`; `actionlint`; build log 2026-10-18-ci-waste-1778-1642; hosted `NOT_OBSERVED` |
| AFP-V0-080 | the `static-reuse` job, the `needs`/`if` of `go-static`, `go-interop` and `artifact-integrity`, and the `STATIC_REUSE` check of `go-product` in `.github/workflows/ci.yml` | `TestAFPV0080WorkflowWiresPushStaticReuse`, `TestAFPV0080StaticReuseDecidesOnlyForAPassedQueueRunOfThisCommit`; live replays in build log 2026-10-18-ci-waste-1778-1642; hosted `NOT_OBSERVED` |
| AFP-V0-081 | the `ci:batched` clause of `go-interop`, `artifact-integrity` and `ci-shard-cost-drift` in `.github/workflows/ci.yml` and of `containment` in `.github/workflows/intake-containment.yml` | `TestAFPV0081BatchedConstituentSkipsOnlyRerunJobs`; hosted `NOT_OBSERVED` |
| AFP-V0-082 | the workflow `concurrency` group of `.github/workflows/ci.yml` | `TestAFPV0082PushConcurrencyGroupsByCommit`; hosted `NOT_OBSERVED` |
| AFP-V0-088 | `IsTaskStoreProjectionPath`, `taskState`, `named` in `internal/liveverify/affected/taskstate.go`; `Plan.NonSource` and the `participants` exclusion in `select.go`; the compact `nonSource` member in `cmd/corvint/affected_compact.go` | the five `_AFPV0088` tests in `internal/liveverify/affected/taskstate_test.go`; `TestAffectedTaskStoreProjectionIsTaskState_AFPV0088`; build log 2026-10-20-V1-2049-affected-taskman-projection |
| AFP-V0-150 | the final `affected.DirtyPaths` and `affectedHeadRevision` reads after `affectedAppNavigations` in `compileAffectedGraph` (`cmd/corvint/affected.go`); seam `affectedPlanSourcesObserved` | `TestAFPV0150PlanHeadDriftAfterSourcesRefuses` (an empty commit injected after the last source observation refuses `unsupported-affected-drift`; failing before the reorder; control plan passes) |
| AFP-V0-151 | `affected.Build(affectedGraphContext(ctx), ...)` in `compileAffectedGraph`; the `unsupported-affected-graph` refusal; AFU-V1-059 walk cancellation | `TestAFPV0151StandaloneAffectedCancellationRefuses` and `TestAFPV0151ReviewFallbackCancellationRefuses` (over the 4,100-file over-cap fixture: cancellation after ten walk visits, at most two further context checks, typed refusal, no plan or review receipt; the review fallback hands its deadline to the build) |
| AFP-V0-152 | `resolvedEvidence` in `internal/liveverify/affected/readers.go` (`resolves`) and in `internal/gateselect/select.go` (`attribute`, `packageBounds`) | `TestSealedChangeEvidenceReadersAreNarrowed_AFPV0152` (plan: fixture-root holder dropped, relative, deep parent and root-anchored readers still selected; nested and non-CEM paths unnarrowed) and `TestSelectPackagesNarrowsSealedChangeEvidenceReaders` (gate verdict and `-bounds` agree); both fail before the change |
| AFP-V0-153 | `taskStateOnly` and the `Plan.Scope`/`Plan.NonSource` receipt members in `internal/gateselect/select.go` | `TestSelectPackagesTaskStateOnlyDiffSelectsNothing` (classified task state is `NOTHING`; an unclassified path, an absent `nonSource`, an `UNKNOWN` plan and an out-of-layout path fall back; a package naming `.taskman/queue.json` is still selected) |
| AFP-V0-154 | the `_test.go` early return in `attributeStructure`, `internal/gateselect/select.go` | `TestSelectPackagesAttributesEveryDirtyPath` (a changed or deleted test file selects only its package; a deleted source still widens to its importers; a test file in an embedding package still selects its importer); both test-file cases fail before the change |
| AFP-V0-155 | `compactAdviceUnknown` and `affectedPlanUnknownReference` in `cmd/corvint/affected_compact.go` | `TestAFPV0155CompactAdviceDoesNotRepeatPlanUnknown`; `resolveCompactAdvice` resolution against `--full` in `TestAFPV0038CompactAdviceReferencesProviderPackages` and `TestAFPV0035CompactDefaultPlanSummarizesTheFullPlan`; the amended `affected-committed-range.json` core-freeze golden |
| AFP-V0-163 | the `reuse` and `Retain integrity reuse decision` steps, `permissions`, `fetch-depth: 2` and the `steps.reuse.outputs.mode` conditions of `artifact-integrity`, and the `artifact-integrity-record` job, in `.github/workflows/ci.yml`; the `job:artifact-integrity` row of `script/ci-least-privilege-exceptions.tsv` | `TestAFPV0163WorkflowWiresIntegrityTreeReuse`, `TestAFPV0163RecordJobRunsNoRepositoryCode`, `TestAFPV0163IntegrityReuseDecidesOnlyForAPassedPullRequestRunOfThisTree` (all fail before the change); queue timings in build log 2026-10-21-ci-waste-1021g-integrity-reuse-and-tasks-slices; hosted after-measurement `NOT_OBSERVED` |
| AFP-V0-164 | the `internal/tasks/cli` and `internal/tasks/authority` entries of `.github/cishards/test-split-allow.json` and their slices in `.github/cishards/test-slices.json` (regenerated from run 39613481398) | `TestAFPV0060CommittedSlicesBoundOversizedPackages` (now requires both packages split, every test selected once, each slice within 400 s); `tools/ci-test-slices replay` before/after in build log 2026-10-21-ci-waste-1021g-integrity-reuse-and-tasks-slices; shuffled package runs; hosted observed max shard `NOT_OBSERVED` |
| AFP-V0-316 | `scanFiles`, `scanFile` and `record` in `internal/gateselect/readers.go` | `TestAFPV0316ParallelIndexMatchesSerial` (the pooled index deep-equals the `GOMAXPROCS=1` serial index over a 99-file fixture, five times) and `TestAFPV0316ParallelIndexReportsTheFirstErrorInWalkOrder` (two unparsable sources: always the walk-first one; recording in completion order fails it); `BenchmarkIndexRepository` |
| AFP-V0-296 | `admittedGoVersions` in `tools/corvint-pr-tests/main.go` | `TestToolIdentityRequiresCurrentGoVersion` (go1.27.2 and go1.27.3 admitted, go1.27.1, go1.27.4 and go1.28.2 refused), run green under go1.27.2 and go1.27.3; fails on go1.27.3 without the fix |
| AFP-V0-160 | `readDocumentationPaths`, `declaredDocumentation` and `documentationPaths` in `internal/liveverify/affected/documentation.go`; the `documentation` graph field and digest member in `graph.go`; the `Select` call in `select.go`; `.corvint/documentation-paths.json` | `TestDeclaredDocumentationIsNonSourceNotUnknown_AFPV0160`, `TestDeclaredDocumentationKeepsEveryReader_AFPV0160`, `TestDeclaredDocumentationLeavesTaskStateAndSourceAlone_AFPV0160`, `TestDocumentationDeclarationIsValidated_AFPV0160`, `TestDocumentationDeclarationIsInTheDigest_AFPV0160`, `TestRepositoryDeclaresBuildLogDocumentation_AFPV0160` |
| AFP-V0-161 | the read-scope guard in `resolves`, `internal/liveverify/affected/readers.go` | `TestDeclaredReadScopeBoundsPathLiteralReaders_AFPV0161`, `TestDeclaredReadScopeKeepsEveryInScopeLiteralReader_AFPV0161`, `TestDeclaredReadScopeDropsOutOfScopeFixtureNamers_AFPV0161` |
| AFP-V0-356 | `affectedDirtyDetail`, `affectedMixedWarning`, `affectedMixedWorktreeRefusal` and `affectedPathListMixedRefusal` in `cmd/corvint/affected_freshness.go` | `TestAFPV0356MixedNamesGloballyExcludedPath` (a path excluded only through `GIT_CONFIG_GLOBAL`'s `core.excludesFile` is named, with the cause and fix, by the `--require-clean` and `--path` refusals and the `--base` warning), `TestAFPV0356DirtyDetailIsBounded` |
| AFP-V0-156 | the `--is-inside-work-tree --show-prefix` checks in `openRoot` (`internal/appmap/gitread.go`), `bindRepository` (`internal/appflows/repositories.go`) and `resolveCheckout` (`internal/extevidence/repository.go`) | `TestAFPV0156AppmapCarriesNoRootLocator` and the `git dir` case of `TestAMAPV0017RootUnavailable`; `TestAFPV0156BindRepositoryTopLevel`; `TestAFPV0156CheckoutTopLevelWithoutLocator` and the `git directory` case of `TestCheckoutBinding` (each locator assertion failing before the change) |
| AFP-V0-181 | `Language.Owns`, `primaryModuleName`, `isTestFile` in `internal/liveverify/affected/python/python.go` | `TestUnitsGivesEachFileOneIdentity`; `TestIsTestFileMatchesDiscoveryConventions` |
| AFP-V0-182 | `scanImports`, `parseImportLine`, `resolveRef`, `moduleAliases`, `SourceRoots` | `TestParseImportLineReadsEveryStatementForm`; `TestResolveRefPrefersTheLongestObservedModule`; `TestModuleAliasesExposeSourceRootLayouts` |
| AFP-V0-183 | the imported-name loop in `parseImportLine` | `TestAFPV0183FromImportNamesReachSubmodules` |
| AFP-V0-184 | `resolveRef`, `resolve`, `FrontierRelativeImport` | `TestResolveRefAnchorsRelativeImportsAtTheImportingPackage`; frontier raise untested (gap) |
| AFP-V0-185 | `scanImports`, `FrontierDynamicImport`, `FrontierConditionalImport`, `FrontierUnreadableSource` | `TestScanImportsFlagsWhatSourceTextCannotSettle`; unreadable source untested (gap) |
| AFP-V0-186 | `languages.All()` registration; `python.New` | `TestPythonSelectionNarrowsARealCorpus` (skips: corpus not tracked; gap) |
| AFP-V0-187 | `scanImports`, `joinContinuation` in `internal/liveverify/affected/python/python.go` | `TestAFPV0187MultiLineFromImportReadsEveryName`; `TestAFPV0187ContinuationLinesAreNotStatements` |
| AFP-V0-188 | `scanImports`, `pluginsAssignment`, `pluginModules`, `FrontierPytestPlugins` | `TestAFPV0188PytestPluginsAreImports` |
| AFP-V0-189 | `linkConftests` | `TestAFPV0189ConftestSelectsEveryTestBelowIt` |
| AFP-V0-190 | `workspaceDirectories`, `appendUse`, `FrontierWorkspaceUnparsed` in `internal/liveverify/affected/golang/golang.go`; `CommittedReadScopes` in `golang/readscopes.go` | `TestWorkspaceUseSetIsTheStrictReaders_AFPV0190`; `TestBaseReadScopesFollowCommittedReadScopes_AFPV0050` (a go.work that is an empty tree) |
| AFP-V0-191 | `Result.Unobserved` in `internal/liveverify/affected/unit.go`; `digestBody.Unobserved`, `Graph.admit`, `Graph.unobservedEntries` in `graph.go`; `Graph.unownedUnknown`, `underDirectory`, `FrontierUnobservedDirectory` in `select.go`; `unobservedModules` in `golang/golang.go` | `TestNestedModuleChangeIsANestedModuleUnknown_AFPV0191`; `TestUnobservedLeavesOutDirectoriesHoldingObservedModules_AFPV0191`; `TestUnobservedDirectoriesAreValidatedAndDigested_AFPV0191`; `TestIndependentNestedModulesCloseTheFrontier_V1_0867` |
| AFP-V0-192 | `Witness.Cause`, `UnboundedCause` in `internal/liveverify/affected/select.go`; `Graph.causes`, `Graph.unboundedReaders` in `graph.go`; `Graph.unboundedReadersOf` in `readers.go` | `TestInstructionFileChangeNamesEachUnboundedReadersCause_V1_2075`; `TestGoTestOnlyChangeSelectsItsPackageButNotItsImporters_V1_0984` |

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
`doc-gates`; that ruleset change is not yet observed. For queue-backstopped PR narrowing,
AFP-V0-028/029 replace the historical PASS prerequisite; hosted enablement is `NOT_OBSERVED`.
The following closed runtime describes historical replay, not queue-backstopped PR execution.
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
remain. (accepted (decision 0431); amended by decision 0431) It does not alter this planner's
UNKNOWN scope or enable general Go selection; AFP-V0-028/029 govern queue-backstopped general
Go narrowing without the historical campaign. Missing or stale DCI qualification runs FULL.
