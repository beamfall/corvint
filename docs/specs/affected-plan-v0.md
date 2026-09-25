# Affected Plan V0

Owner: Russell Lewis
Date: 2026-09-01
Requirement prefix: `AFP-V0`
Intent status: accepted for AFP-V0-008 (decision 0057) and AFP-V0-009 (decision 0052); AFP-V0-013/014/015 accepted (decision 0289); AFP-V0-016/017 accepted (decision 0320), AFP-V0-016 amended (decision 0390); AFP-V0-021 accepted (decision 0376); other AFP-V0 requirements proposed
Delivery status: experimental
Authoritative inputs: `docs/specs/go-live-test-provider-v0.md` (provider plan wire and non-goals),
`docs/specs/live-proof-carrying-verification-v0.md` (future composer, not-started),
`internal/liveverify/affected` (selector), `AGENTS.md` invariants 2, 4, and 8.

## Agent digest
- Claim: `corvint affected` emits a read-only, non-authoritative affected-test selection plan with provider-ready Go package paths.
- Status: accepted for AFP-V0-008 (decision 0057) and AFP-V0-009 (decision 0052); AFP-V0-013/014/015 accepted (decision 0289); AFP-V0-016/017 accepted (decision 0320), AFP-V0-016 amended (decision 0390); AFP-V0-021 accepted (decision 0376); other AFP-V0 requirements proposed/experimental
- Exists: `internal/liveverify/affected`, `corvint affected`, `cmd/corvint/affected_test.go`, the `advice` member (AFP-V0-009: repository-declared mandatory checks, one advisory Go command, the unknown frontier), the `--base FULL_COMMIT_ID` range form and `range` member (AFP-V0-010), and the `make gate-affected` fast tier over the receipt (AFP-V0-011: `script/gate-affected.sh`, fail-closed to the full `go-test` run; not the push gate), whose union is attributed per dirty path from a static repository index of imports and path literals (AFP-V0-012), whose literal-reader rule also adds, in the plan itself, selections for every dirty path a package names, without narrowing an unowned path's `UNKNOWN` scope (AFP-V0-021); `tools/corvint-pr-tests` and `.github/workflows/ci.yml` remain full until separately pinned AFP-V0-014 qualification.
- Blocked on: the LPCV-V0 composer accepting or replacing this wire; the 200-row qualification, which needs 201 first-parent commits on `main` (AFP-V0-017).
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
  one source walk, at most two bounded advice-declaration reads (AFP-V0-009), with `--base` one
  bounded base identity read and one bounded `git diff --name-only` (AFP-V0-010), no test
  execution, no index, trace, cache, or ledger write, and no `observeUnsupported` call on failure.
- **AFP-V0-002:** The dirty set MUST come from `affected.DirtyPaths` (porcelain v1, NUL-delimited,
  untracked included, ignored excluded, 8 MiB / 10 s bounds) and MUST fail closed on overflow,
  malformed output, or an unavailable Git. The one directory record Git emits under
  `--untracked-files=all`, a nested repository or linked worktree as `?? DIR/`, is admitted as
  the dirty path `DIR`, which no plugin owns, so the plan widens to `UNKNOWN` over it rather than
  the capture failing. The dirty set and HEAD MUST be re-read after the graph
  is built; any difference MUST fail closed as `unsupported-affected-drift`, because the status and
  the graph are two observations of one mutable worktree.
- **AFP-V0-003:** Stdout MUST be one canonical JSON line with exactly the members `advice`
  (AFP-V0-009), `mutates=false`,
  `ok=true`, `plan` (the selector's canonical `affected.Plan`), `profile="affected-plan/0"`,
  `provider.go.{packages,state}`, `range.{base,paths}` (AFP-V0-010), `revision` (HEAD commit id),
  and `tool="affected"`.
  `provider.go.state` MUST be one of `RUNNABLE`, `EMPTY_SELECTION`, `MODULE_PATH_UNRESOLVED`, or
  `PACKAGE_BOUND_EXCEEDED`; `packages` MUST be non-empty only when the state is `RUNNABLE`, and is
  then the sorted, de-duplicated import paths of selections in the `go:` namespace, at most 4,091
  entries (the `go-live-plan/0` argv bound). The module-path state exists because unit identities
  are directories rather than import paths when the Go plugin reports `go:module-path-unresolved`.
- **AFP-V0-004:** The plan is not authority. `plan.scope` MUST be `UNKNOWN` whenever `plan.unknown`
  is non-empty; the provider's plan wire MUST keep `NO_AFFECTED_SELECTION_PROOF`; no consumer may
  treat an exclusion as proof that the excluded test is safe to omit. When a current-tree-unindexed
  dirty Go path shares an observed package directory, that package's exclusion reason MUST be
  `UNINDEXED_DIRTY_GO_PATH_MAY_BE_DELETED_OR_RENAMED`, not
  `NO_DEPENDENCY_PATH_TO_DIRTY_UNIT`. The dirty-path input does not preserve Git status or overlay
  provenance, so the reason names the deletion/rename possibility without asserting it occurred.
- **AFP-V0-005:** For a fixed tree, HEAD, and dirty set the document MUST be byte-identical across
  runs; with `--base`, the base commit is part of that fixed input.
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
  read from text; an entry outside the root is skipped). Each observed module's packages are
  units under that module's own import path, so an edge between two workspace modules resolves
  as an edge inside one does. A `go.mod` below the root that no `use` names is the
  `go:nested-module-frontier`: its packages are absent from the graph, never attributed to the
  module above them. A listed module whose path cannot be read is `go:module-path-unresolved`.
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
  toolchain is never executed.
- **AFP-V0-009:** (accepted 2026-09-04 by decision 0052) The receipt MUST carry an `advice` member with exactly the
  members `status="PLAN_ONLY"`, `checks`, `unknown`, and `note`, plus `test_selection` only when
  `--provider` is given (ETS-V0-002, `docs/specs/external-test-selection-v0.md`). Each `checks` entry has exactly
  `command`, `kind` (`mandatory` or `advisory`), `reason` (one sentence), and `source` (a repository
  path or `affected-plan`). A `mandatory` entry MUST come only from a repository-owned declaration
  read from the working tree at the root: a `gate:` target in `Makefile` yields `make gate`, and a
  fenced `sh`/`bash`/`console` block under a heading containing "Verify" in `AGENTS.md` yields each
  of its non-empty command lines with a leading `$ ` stripped. Both reads are bounded at 256 KiB per
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
  `go:nested-module-frontier` do not fall back: they are present on every clean tree here and name
  packages the selector never claims. The fast tier is not the push gate: `make gate` is unchanged
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
  (d) whenever `plan.dirty` is non-empty (`unresolved`), every package whose reads no literal bounds.
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
  smallest dirty path naming it. The CEM sidecar narrowing of rule (c) is not applied. Rollback
  removes the reader selections and the bound entries; the paths stay unknown as before.

## Non-goals and authority

No provider modification; execution only through the explicitly admitted AFP-V0-013 driver; no watcher or daemon (invariant 7,
`GPK-V0-010`); no exclusion certificates as authority; no non-Go provider package lists; no
LPCV-V0 requirement is implemented or promoted by this slice. The fast tier (AFP-V0-011) is not the
push or release gate and MUST NOT replace `make gate` in any declaration until a 200-commit shadow
run shows no selected-set miss the plan did not mark `UNKNOWN`; a narrowed run proves nothing about
the packages it omitted (AFP-V0-004).

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
widening (AFP-V0-021). A potentially deleted or renamed-away Go path widens and names that possibility on its package
exclusion rather than claiming no dependency path. An unreadable subtree:
`unsupported-affected-graph`, never a silently smaller graph. Worktree or HEAD changed during
compilation: `unsupported-affected-drift`. A `--base` that is not a full commit id:
`invalid-arguments`; one that is not a commit here: `unsupported-affected-revision`; a range diff
over its bound: `unsupported-affected-status`. In the fast tier every one of these, a plan the
script cannot read, a module-level Go frontier, a dirty root module definition, or an empty
selection over a non-empty diff, or a repository the selector cannot index (AFP-V0-012) runs the
full `./...` command instead of a narrowed one, so the
worst case of `make gate-affected` is the cost of `make go-test`, never a skipped package.

## Acceptance evidence and traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| AFP-V0-001 | `cmd/corvint/affected.go` `compileAffected` | `TestAffectedCleanTreeSelectsNothingAndWritesNothing` compares `git status --porcelain --ignored` before and after |
| AFP-V0-002 | `internal/liveverify/affected/dirty.go` | `TestDecodeStatusFailsClosedOnMalformedInput`; `TestAffectedRejectsNonRepositoryAndExtraArguments` |
| AFP-V0-003 | `affectedReceipt`, `providerGoPackages` | `TestAffectedDirtyGoSourceSelectsDependentsAsProviderPackages` |
| AFP-V0-004 | `affected.Select` scope and exclusion-reason rules | `TestAffectedUnownedDirtyPathIsUnknownScope`, `TestDeletedGoSourceNamesDeletionInOwnUnitExclusion`; provider wire unchanged (`go-live-test-provider-v0.md` GLTP-V0-006) |
| AFP-V0-005 | canonical JSON via `gokernel.CanonicalJSON` | byte-identity assertion in the dirty-source test |
| AFP-V0-006 | `runAffected` error paths; `affected.ErrWalkUnreadable`; `affected.ErrWalkUnrepresentable` | `TestAffectedRejectsNonRepositoryAndExtraArguments`; `TestAffectedUnreadableSubtreeFailsClosed`; `TestSourceFilesRefusesAnUnrepresentableAcceptedName` |
| AFP-V0-007 | `Graph.rank`, `Graph.proximity` in `internal/liveverify/affected/select.go` | `TestSelectOrdersByDistanceThenSharedDirectoryThenUnitID` (order and two-run byte identity) |
| AFP-V0-008 | `SourceFilesIncluding`, `MaxIncludedDirectoryEntries`, `FrontierIncludedDirectoryWalkBounded`, `observeModules`, `workspaceDirectories`, `enclosingModule`, `groupByDirectory`, `inTestdata`, `unitID`, `readModulePath` in `internal/liveverify/affected` | `TestIncludedDirectoryWalkBoundWidensInsteadOfRefusing_AFPV0008`, `TestWorkspaceModulesAreUnitsUnderTheirOwnModulePath`, `TestWorkspaceDirtySourceSelectsTheOtherModulesTest` over `testdata/workspace` (a listed pair, an unlisted `stray`, an entry outside the root), `TestPackagesUnderBuildOutputDirectoryNamesAreSelected`, `TestNoGoRepositoryProducesNoUnitsOrFrontier`, `TestReadModulePathMatchesGoModEdit`, `TestReadModulePathAbstainsOnBOM`, `TestTestdataIsFixtureDataNotAPackage_AFPV0008`, `TestWorkspaceModuleBelowTestdataIsObserved_AFPV0008` |
| AFP-V0-010 | `parseAffectedBase`, `affectedRangePaths`, `affectedRange` in `cmd/corvint/affected.go`; `RangePaths`, `DecodeNameList` in `internal/liveverify/affected/dirty.go` | `TestAffectedBaseRangeJoinsCommittedPathsAndFailsClosed` (committed edit with a clean worktree selects the dependents; `range.base`/`range.paths`; `main` and an unknown id exit 2 with no document), `TestDecodeNameListNormalizesAndFailsClosed`, `TestAffectedReceiptMembersAreClosedAndByteStable` (the closed member set includes `range`) |
| AFP-V0-011 | `gate-affected`, `gate-affected-test`, `GO_TEST_COMMAND` in `Makefile`; `script/gate-affected.sh`; its selection step `selectPackages` in `tools/gate-affected-select/main.go` (native Go, no Python runtime, decision 0088) | `script/gate-affected_test.sh` via `make gate-affected-test` (a shell test over `testdata/fixture` in a scratch repository with a recording go-test command: clean tree runs nothing; a core edit selects core and leaf; a deleted `core/core.go` is a `frontier` line for core, mid, and leaf; a document no package reads beside a core edit is `data`, does not fall back, and adds no package; a dirty `go.mod` falls back; a committed edit under `BASE` selects; an unresolvable base falls back; an interrupted planner exits nonzero without running go test); `TestSelectPackagesAttributesEveryDirtyPath` (a control character in a dirty path falls back) in `tools/gate-affected-select/main_test.go`; `TestSelectPackagesRejectsSiblingModulePrefix` (a package path that only shares the module's characters as a string prefix, with no `/` boundary, falls back instead of being selected); the 50-commit replay in AFP-V0-011 |
| AFP-V0-012 | `indexRepository`, `scanSource`, `escapesPackage`, `dependents`, `readers`, `enclosing`, `unresolved`, `namesPath` in `tools/gate-affected-select/readers.go`; the per-path loop in `selectPackages` (decision 0131) | `TestSelectPackagesAttributesEveryDirtyPath` in `tools/gate-affected-select/main_test.go` (a deleted source widens to its importers; a Go file read as data selects its reader; a document selects the package that names it; a testdata fixture selects its enclosing package; a `runtime.Caller` package is selected on every dirty path; a nested module's literals select nothing); `TestSelectPackagesFallsBackWhenAttributionFails` (imports that do not parse fall back); `TestSelectPackagesReachesEmbeddingAncestorDependents` (a data path under an embedding ancestor reaches that ancestor's dependents); `TestSelectPackagesResolvesAliasedAndDotRootLocatorImports` (an aliased or dot-imported `runtime.Caller` still marks the package unresolved); `TestIndexRepositoryFailsClosedOnSymlinkedGoFile` (a symlinked `.go` file falls back instead of being silently skipped) |
| AFP-V0-013 | `tools/corvint-pr-tests` and `.github/workflows/ci.yml` | `TestSelectedFailureAndFallback`, `TestInterruptionLeavesNoLiveDescendant`; trusted pins empty, hosted execution unavailable |
| AFP-V0-015 | `tools/corvint-pr-tests/container.go` and indexed shadow execution | `TestContainerProfileAndArchive`, `TestColdRuntime`, `TestFrozenRowIndex`, `TestDockerCLIInterruption`, `TestContainerCleanupRefusal`; real Linux row/hosted NOT_RUN |
| AFP-V0-014 | `tools/corvint-pr-tests/shadow.go` | `TestQualificationAndTerminalFailures`, `TestToolIdentityRequiresCurrentGoVersion`; frozen 200-row qualification NOT_RUN |
| AFP-V0-016 | `.github/workflows/ci-control-plane.yml`; the `main` repository ruleset | `actionlint`; `success` posted on PR #26 (run 35444060752) and PR #24 (run 35446378936); ruleset 23699808 active with the decision 0320 settings; the decision 0390 settings (no bypass, `doc-gates` required) and the admin-status consent path NOT_VERIFIED until the owner applies them; `failure` path NOT_RUN on a real PR |
| AFP-V0-017 | `.github/workflows/pr-tests-qualification.yml` | `actionlint`; dispatch NOT_RUN (`main` has fewer than 201 first-parent commits) |
| AFP-V0-019 | `internal/plansnapshot`, `compileSnapshotAffected` | `TestSnapshotImmutableBytesAndCleanup`, `TestSnapshotRejectsIncompleteMismatchedAndStale`, `TestSnapshotStrictWire`, `TestSnapshotRejectsLinksAndIgnoresArchiveAttributes`, `TestAffectedSnapshotMatchesCommittedPlanAcrossDirtySources`, `TestAffectedSnapshotPlaywrightPinsConfigAndSource` |
| AFP-V0-018 | `playwrightAffectedReceipt`, `compilePlaywrightAffected`, and `typescript.SelectPlaywright` | `TestAffectedPlaywrightProfileEmitsProjectDistinctUnits`, `TestAffectedPlaywrightArgumentsFailClosed`, and `internal/liveverify/affected/typescript/playwright_test.go` |
| AFP-V0-009 | `affectedAdvice`, `compileAffectedAdvice`, `mandatoryAffectedChecks`, `advisoryAffectedChecks`, `shellQuoteJoin` in `cmd/corvint/affected.go` | `TestAffectedAdviceJoinsMandatoryGateAndAdvisoryPackages`, `TestAffectedAdviceReportsNoDeclaredGate`, `TestAffectedAdviceKeepsMandatoryGateAndNeverAdvisesExclusions`, `TestAffectedReceiptMembersAreClosedAndByteStable` (tightened to assert `advice`'s raw JSON key order), `TestAffectedAdviceBoundsTheDeclarationRead`, `TestShellQuoteJoinEscapesMetacharacters`, `TestAffectedAdviceTruncatedMandatoryDeclarationSuppressesNoGate`, `TestAffectedAdviceCapsMandatoryChecksAtSixteen`, `TestAffectedAdviceSkipsCommentsInVerifyFence` |
| AFP-V0-021 | `WitnessPathLiteralReader`, `PathTokenBound`, `Graph.readers`, `Graph.tokenBounds`, `namesPath` in `internal/liveverify/affected` (`select.go`, `readers.go`); `Unit.PathTokens`, `Unit.PathTokensBounded`; `pathTokens`, `importsEnd`, `ignoredByGo`, `maxPathTokens` in `internal/liveverify/affected/golang/golang.go` | `TestPathLiteralSelectsItsReaderPackage_AFPV0021` (a named document selects its reader and stays unknown; single and parenthesized imports are no tokens; a file without imports yields tokens; a dependent and an unnamed path select nothing), `TestOwnedDirtyPathSelectsTheUnitsThatNameIt`, `TestReaderWitnessIsTheSmallestNamingDirtyPath`, `TestReaderReachedByDependencyKeepsItsDependencyWitness`, `TestBoundedPathTokensAreUnknownOnlyWhenAMatchIsAttempted`, `TestPathTokenBoundNamesThePackage`, `TestUnlexableSourceIsAFrontierOutsideIgnoredDirectories`, `TestSelectionOnTheLiveDirtyWorktree` (reader witnesses resolve), `TestAffectedDocumentSelectsThePackageThatNamesIt` (receipt shape, provider packages, byte identity) |
| AFP-V0-020 | `UnknownNoSelectableTest` in `affected.Select` (`internal/liveverify/affected/select.go`) | `TestSelectNamesChangedUntestedGoPackageAsUnknownScope`, `TestSelectTraversesUntestedUnitsWithoutSelectingThem` (an untested unit the change only reaches stays bounded), `TestSeamWidensWhenNoTestReachesAChangedUnit_AFPV0020` (every plugin), `TestPlaywrightDiscoveryReconciliation` (an unreached helper keeps the Playwright plan), `TestAffectedUntestedGoPackageIsUnknownScope` |

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
at launch falls back to full; Go/compiler/environment drift fails safely. Freeze ignores grafts;
row reuse binds the execution source/environment and raw plan/audit hashes as well as outcomes.

Container control operations have a two-minute deadline within an 80-minute launcher ceiling;
the test operation keeps its 75-minute bound. Docker daemon logging is disabled and inspected,
so PID1 file descriptors cannot bypass the retained-output bound. Both historical and PR
container checkouts use `/work/checkout`; process capture cannot bypass limits via io.Copy.
Run export requires only its actual selection/execution/Go outputs, not historical row files.
