# Affected Zero Unknown V0

Owner: Russell Lewis
Date: 2026-11-04
Requirement prefix: `AZU-V0`
Intent status: experimental (owner goal, owner chat 2026-11-04; orchestrator design under the 2026-11-02 delegation)
Delivery status: experimental
Authoritative inputs: `docs/specs/affected-plan-v0.md` (parent contract: wire, authority, AFP-V0-511
frontier bearing, AFP-V0-560 terminal rule, AFP-V0-601/602 data readers),
`docs/specs/affected-plan-selection-v0.md` (AFP-V0-2405 driver-input hint, AFP-V0-2440 fixture-only
claim), `docs/specs/test-selection-precision-v0.md` (TSP-V0-005 shell foreign callers, TSP-V0-942
frontier bearers), `internal/liveverify/affected`, `AGENTS.md` invariants 2, 3 and 8. The parent
spec is at its OCM obligation limit, so these requirements take a new prefix.
Lane selfloor (V1-5000, owner lane charter 2026-11-07) defines AFP-V0-3120 to AFP-V0-3128, and
its PR #142 repair AFP-V0-9050 and AFP-V0-9051, here
under the parent's `AFP-V0` prefix for the same reason; see build-log entry
`2026-11-07-selfloor`.

## Agent digest
- Claim: every `corvint affected` plan closes unknowns with sound selections or stated proofs, including bounded readers, never by dropping reached units.
- Status: experimental (owner goal, owner chat 2026-11-04; orchestrator design under the 2026-11-02 delegation); delivery experimental.
- Exists: `frontierUnknowns`, `bearsOnUnowned`, `participants`, `unownedUnknown` and `driverInput` in `internal/liveverify/affected/select.go`; `FrontierReach` in `unit.go`; `.corvint/shell-foreign-callers.json`, `.corvint/runtime-readers.json` and `.corvint/test-binary-consumers.json`.
- Blocked on: aggregate sibling integration, empty unknown lists on every frozen merge, and historical full-suite/mutation receipts; lane verification retains sandbox CLI failures; the registered-driver assertion conflict is repaired without changing frozen receipts.
- Read next: User and current state, then the requirement you are implementing.

## User and current state

The owner directed the goal ("figure out how to get the unknown to 0 safely", owner chat,
2026-11-04), and the orchestrator made the design choices under the owner's delegation of
2026-11-02. Experimental candidates for driver ownership, inbound closure and frozen replay
exist; AZU-V0 is NOT_QUALIFIED.

The merge-forward verifier repair restores the base's registered driver and historical
namespace in default graph identities. Frozen strict/coverage receipts (AFU-V1-024) include
both that namespace and observed driver units. Explicit optional driver sets retain
AZU-V0-8905 absent-driver parity. The self-review repair reconciles the lane-only
`TestAffectedAuditDriverDoesNotChangeDefaultPlan_AZU_V0_002` with the accepted registered
registry: a changed Makefile selects its driver target with BOUNDED scope. Frozen receipt
assertions remain unchanged. Go raw-string directory joins also retain unknowns for
undeclared callers, and partial replay reports remain unaccepted until every listed merge
qualifies. AZU-V0-004/009 aggregate qualification still lacks historical full-suite/mutation
evidence; CLI sandbox failures remain explicit verification limits.
No observed graph vertex is hidden from a digest.

The user is the engineer, agent or CI shard that runs only the tests a change can affect. A plan
with any `unknown` entry has scope `UNKNOWN`, and every consumer that cannot trust it falls back
to a wide or full run. That fallback is the largest cost in this repository's build loop.

Measured on 2026-11-04 with the `8e2c13aa9` tree binary over eight consecutive `origin/main`
merges (`8e2c13aa9`, `39c031980`, `d436dff76`, `6c33df33b`, `ea770fa17`, `24d8c04f9`,
`f13f52b16`, `9fdc2f02e`): every plan was `UNKNOWN`, with 63 to 99 entries. At `8e2c13aa9` they
were `LANGUAGE_FRONTIER` 81, `UNOWNED_DIRTY_PATH` 5, `UNINDEXED_SOURCE_PATH` 2,
`NO_SELECTABLE_TEST` 2 and `UNSELECTED_BINARY_CONSUMER` 2. Single-file probes on the same tree:

| Dirty set | Frontier entries | Why |
|---|---|---|
| one owned Go file (`internal/gateselect/allowlist.go`) | 1 | `shell:foreign-invocation-unmodelled`, born by seven unbounded-reader shell units the change reaches |
| one owned TypeScript file (`extensions/vscode/src/configuration.ts`) | 69 | TypeScript participates, so every TypeScript entry bears (AFP-V0-511 (1)) |
| one owned shell file (`script/heavy.sh`) | 1 | the same shell entry |
| a merge carrying `Makefile`, `.github/workflows/*.yml` or another unowned path | 60 to 81 | every non-participating plugin's path-open entries bear on the unowned path (AFP-V0-511) |

So the frontier count has three sources, all closable: the unowned-path cascade, whole-plugin
participation, and inbound shell callers. The remaining classes are small and local.

## Requirements

- **AZU-V0-001:** **Driver claimant.** A built-in claimant SHALL own every driver input that
  `driverInput` recognises (make files at any depth, `.github/workflows/**`, `.github/actions/**`),
  so such a path is never `UNOWNED_DIRTY_PATH` and never triggers the AFP-V0-511 cascade. Its units
  are make targets and workflow jobs. Edges are the repository paths a recipe or step names
  literally: a script, a `go test`/`go run` package pattern, or another make target. A dirty driver
  file SHALL select the targets or jobs whose recipe or step text a diff hunk touches. A hunk
  outside every recipe or job (a variable, an `include`, `on:`, top-level `env:` or `defaults:`)
  SHALL select every target or job of that file. A recipe or step that invokes a computed name
  carries a `driver:computed-invocation` frontier entry at that target or job; AZU-V0-003 closes it.
  A make computed name is evaluated into the edges of its alternatives. It keeps the entry only
  when its alternatives or the reads of what it runs cannot be bounded (AFP-V0-3120 to
  AFP-V0-3128).
  Selections carry kind `DRIVER_TARGET` and the argv `make <target>`, or `CI_JOB` and the workflow
  and job IDs. A consumer that cannot run a `CI_JOB` reports it as left to CI, never as unknown.
  Readers of a driver file by path literal, declared read scope or unbounded read stay selected as
  before. Supersedes the AFP-V0-2405 hint for claimed paths.
  Go package commands with no package argument SHALL depend on the current package after
  `cd` or `go -C`; an unresolved directory SHALL retain uncertainty or depend on every owner.
  YAML flow mappings outside the supported subset SHALL retain the unparsable-file frontier
  rather than silently discard execution members. Flow steps accepted as scalar text by the
  supported parser SHALL retain their job or action as a computed invocation when execution
  members are not modelled (AZU-V0-9703, AZU-V0-9852). Unparsable driver
  fallback units SHALL be reached for dirty paths not covered by the existing project-owned
  no-unbounded-reader proof (AFP-V0-772), preserving uncertainty even when the hidden execution
  edge cannot participate in traversal. Literal readers remain selected under that proof.
  Driver execution edges SHALL survive Go test-only containment (AFP-V0-036), selecting
  the target/job and its callers when only the invoked package's tests change; ordinary Go
  importers remain contained. Go's `-C=DIR` form SHALL resolve like `-C DIR`. Shell globs in
  Go package operands and unsupported inline interpreter programs SHALL resolve their
  dependencies or retain the invocation site as computed under AZU-V0-003.
- **AZU-V0-002:** **Ownership completeness gate.** `corvint affected --ownership-audit` SHALL list every
  tracked path at a commit that no plugin, the driver claimant or a project declaration claims.
  Declarations are documentation paths, runtime readers, task state, and a new
  `.corvint/non-source-paths.json` for evidence output such as `.agent-evidence/`. A maintained test
  SHALL fail while that list is non-empty at `HEAD`, so a merged tree has no unowned tracked path
  and `UNOWNED_DIRTY_PATH` can arise only for an uncommitted new path. The audit is read-only (AGENTS
  invariant 4).
- **AZU-V0-003:** **Outbound site closure.** A plugin SHALL classify each frontier reason as `OUTBOUND`
  or `INBOUND` through a new optional `FrontierDirection` method on its claimant. `OUTBOUND` means
  the hidden edge starts at the reference in the site's own file: an unresolved, dynamic or bare
  import, an include, an executable config, or a computed invocation. A reason with no declaration
  is `INBOUND`. When an `OUTBOUND` entry bears, for any `Via`, the plan SHALL NOT emit it as unknown.
  Instead it SHALL select every unit whose `Frontier` lists that reason (witness `FRONTIER_SITE`),
  traverse from them as from a changed unit, and apply the bearing rules to the units this reaches,
  to a fixed point. Soundness: every hidden edge of the reason starts at one of those site units. So
  whichever path the edge binds, only a site unit and its dependents can observe the change. This
  needs a complete site list: a plugin SHALL record every site unit in the graph, even when its
  display truncates to `reason:+N`. A plugin that cannot locate a site in a unit keeps that entry
  unknown.
- **AZU-V0-004:** **Inbound caller closure.** An `INBOUND` reason (a hidden caller into the site unit,
  such as `shell:foreign-invocation-unmodelled`, `shell:foreign-callers-changed` or
  `typescript:e2e-runtime-dependency`) SHALL close when the project declares the site's callers.
  Declarations are `.corvint/shell-foreign-callers.json` (TSP-V0-005) and driver edges
  (AZU-V0-001). The plan then selects the declared callers' units and drops the entry. A maintained
  check SHALL scan every tracked file for a literal reference to each declared-callee path and fail
  when a referencing file is neither declared nor an edge the graph already models. A file that
  builds a callee path from a computed name under a declared callee directory SHALL also fail the
  check, so the declaration stays complete under project authority (AGENTS invariant 3). An inbound
  entry whose site has no declaration stays unknown.
- **AZU-V0-005:** **Dot-directory Go packages.** The Go plugin SHALL index tracked Go packages of the
  main module that lie below a directory whose name starts with `.` or `_`, which `./...` skips (for
  example `.github/testconfine` and `.github/testconfine/cmd`). Such a file is then owned source,
  not `UNINDEXED_SOURCE_PATH`. Hidden-directory observation SHALL use the same bounded,
  metadata-excluding source walk for disk and immutable sources; `testdata` remains fixture data.
  Indexing a previously unowned file SHALL retain testless literal/glob readers and their callers.
  Session runner discovery SHALL explicitly list these hidden packages alongside `./...` and
  validate the complete planned Go universe before executing Go units.
  (selbatch merge with AZU-V0-9920/9921, 2026-11-06) A Go file below a hidden or underscore
  directory that `.corvint/non-source-paths.json` declares SHALL stay declared output, not a
  package, in both the planner and the gate selector index. Below a hidden directory, a file
  name no unit can name SHALL refuse only a plugin that claims it; an unclaimed name such as
  retained metadata SHALL NOT refuse every plugin's walk. Outside hidden directories the
  existing refusal is unchanged. (selbatch fix round 1, 2026-11-06) An indexed hidden package's
  files SHALL count as package files for the import-set comparison (AFP-V0-2202), so an import
  change there still selects the `go list` readers of its importers.
- **AZU-V0-006:** **Uncovered, not unknown.** A changed unit with no test (`NO_SELECTABLE_TEST`) SHALL
  first select its declared or modelled callers (AZU-V0-001, AZU-V0-004). When no selectable test
  anywhere can observe it, the plan SHALL record it in a new `uncovered` list, not in `unknown`.
  This is a coverage gap, not a selection gap: no test exists whose outcome the change can alter. A
  unit with any undeclared `INBOUND` entry stays unknown. The optional `plan.uncovered` member
  is omitted when empty; otherwise it is a unit-ID-ordered list of `{reason, detail}` with
  `reason="NO_SELECTABLE_TEST"` and the unit ID as `detail`. Both full and compact plan profiles
  preserve it; the by-path profile preserves it on each path entry, and text emits an
  `uncovered REASON UNIT_ID` line. Declared callers with no owning unit remain unknown. Literal, scoped, glob and
  unbounded reader tests count as observers and remain selected. Before proving a coverage
  gap, testless literal/glob readers of an untested owner SHALL select their caller closures,
  including readers of owned, nonhidden source paths. (selbatch fix round 2, 2026-11-07) A
  testless unit whose declared read scope (AFP-V0-023) holds such a path SHALL select its caller
  closure the same way; a dirty read-scope declaration alone does not. A caller's tests or
  declared consumers count as observers only when the plan runs them. When no test the plan runs
  observes the unit, a caller the plan leaves out keeps `NO_SELECTABLE_TEST` unknown, unless
  additive narrowing alone left it out (ADN-V0-005). Readers and their caller closures SHALL
  retain uncertainty from unresolved declared callers before a coverage gap can be proved.
  This inspection SHALL cover every changed path owned by the unit, independently of the
  representative seed or reader witness. Reader frontier inspection SHALL preserve existing
  own-files bearing rules over the complete dirty set (TSP-V0-001), including short
  concatenated readers (V1-1818).
  Embedding foreign-language
  source bytes does not establish execution coverage (AFP-V0-811); that uncertainty stays unknown.
  A Ruby view with no reachable route-dispatched test retains its existing
  `NO_SELECTABLE_TEST` abstention (RUBY-AFFECTED-V0-761): missing runtime dispatch is not a
  coverage proof. Playwright SHALL inspect both unknown and uncovered units through its
  existing out-of-suite reconciliation (TJAA-V0-106/107/121); a shared-graph coverage gap
  alone cannot exclude a dirty runtime input or bypass its unreached-path declaration.
  The frozen `application-flow-impact/1` projection retains uncovered coverage gaps in its
  legacy `graph.unknown` list and `UNKNOWN` scope; its wire and goldens do not change.
  The existing advice coverage warnings also retain `NO_SELECTABLE_TEST: UNIT_ID` for
  uncovered units: compile-only advice does not claim behavioural test coverage.
- **AZU-V0-007:** **Declared binary consumers run.** An entry of `.corvint/test-binary-consumers.json`
  MAY carry `run`, the argv that runs it from the repository root. Legacy `FILE:[COMMAND_DIR...]`
  entries remain valid; the extended form is `FILE:{"commands":[COMMAND_DIR...],"run":[ARG...]}`.
  `commands` has the existing directory-list grammar; `run` is optional but, when present,
  SHALL contain 1 to 256 UTF-8 arguments, a nonempty executable, no NUL/CR/LF, and at most
  65,536 bytes in total. Unknown members, null/empty argv and invalid entries fail closed through
  the existing declaration frontier. The graph pins `execConsumerRuns` by file alongside
  `execConsumers`; its optional map is included in digest and cache projections. When the change reaches the
  consumed binary's build, an entry with `run` SHALL be selected with kind `DECLARED_CONSUMER` and
  that argv, instead of `UNSELECTED_BINARY_CONSUMER`. Such a selection carries
  `unitId="declared-consumer:"+FILE`, `tests=[FILE]`, `kind="DECLARED_CONSUMER"`, `run=ARGV`,
  and a `DECLARED_CONSUMER` witness extending the command's build witness. The compact plan
  keeps `kind` and `run` and replaces `tests` with `testCount`; by-path additions preserve
  both optional members, and text selections carry the shell-quoted `run` command. Both optional selection members
  are omitted for ordinary units. Several rebuilt commands select a consumer once. Test-only
  or reader-only command reach does not trigger it. An entry without `run` keeps today's unknown.
  Session planning SHALL retain each selected consumer's exact argv using the existing declared
  command adapter and digest, without adding manifest or receipt fields. The local profile runs
  each consumer from the repository root; profiles that cannot execute it retain its command as
  an explicit required check. (selbatch fix round 2, 2026-11-07) Declared commands on one shard
  with identical argv SHALL run as one invocation that records a terminal outcome for each of
  their units, so consumers sharing one runner, such as the browser gate, run it once. CI SHALL also retain the legacy consumer check shape recognized by
  its browser-job selector until that selector supports runnable consumer commands.
  Mixed dependency and command-test changes SHALL extend the binary's
  build witness, including its original dirty path and complete dependency chain, independently
  of the command's final test-selection witness.
- **AZU-V0-008:** **Package-level runner fallback.** A frontier reason that hides only which tests run
  inside a known package or project SHALL select that package's whole test command. Examples are
  `typescript:test-runner-unknown`, `typescript:runtime-flags-unresolved`,
  `dotnet:test-runner-filter-unresolved` and `dotnet:dynamic-test-discovery`. The plugin declares
  the reason `PACKAGE_RUNNABLE`. Only a reason whose package has no runnable command stays unknown,
  as `NO_RUNNER`.
- **AZU-V0-009:** **Zero-unknown acceptance.** The replay `script/affected-zero-unknown-replay.sh`
  SHALL run `corvint affected --base M^1` for each merge in a frozen list. The list starts with the
  eight merges above and adds every merge since. It SHALL report per merge the unknown count by
  reason, the selection size and its wall-clock cost. AZU-V0 is met when every plan in the list has
  `unknown: []` and the safety check finds no miss: on each merge, every test whose outcome differs
  between the parent and the merge in a full-suite run, or that a mutation on a changed line kills,
  is in the selection. A selection whose command filters its unit (`go test -run`) holds only the
  tests that filter runs; a filter the replay cannot evaluate leaves safety `NOT_PRODUCED`.
- `AZU-V0-9700`: Missing wrapper operands. As directed by the owner in the four Greptile P1
  fixes for batch PR #98 (2026-11-05), a recipe or workflow step ending in an operand-taking
  `env` or command-wrapper option SHALL NOT panic during graph construction. It SHALL retain
  a computed invocation at that target or job, conservatively selecting it and its dependents.
- `AZU-V0-9701`: Implicit current Go package. A supported Go package command without package
  operands, including after flags, SHALL import the current working directory's package. An
  unknown directory SHALL fall back to every owner. The repository root MAY use the existing
  every-owner path import when the frozen path-import encoding cannot represent it precisely.
- `AZU-V0-9702`: Complete make alternatives. Alternative variable assignments SHALL preserve
  each complete expanded word, including literal prefixes and suffixes, or retain a computed
  invocation when the alternatives cannot be represented. Joining alternatives into one expansion
  SHALL NOT silently lose a package, script or other dependency.
- `AZU-V0-9703`: Flow-style workflow steps. A flow-style step sequence or mapping SHALL either
  expose its `run` dependencies or conservatively retain its job as a computed invocation. A valid
  flow-style step SHALL NOT be silently skipped merely because the YAML subset represents it as text.
- `AZU-V0-9704`: Driver runner classification. As directed by the owner in the second PR #98
  repair round (2026-11-05), `corvint test` SHALL NOT interpret a driver `NO_SELECTABLE_TEST`
  entry as a language requiring a declared full command. Make targets and workflow jobs remain
  owned by their drivers under AZU-V0-001. This classification SHALL retain selected Go readers,
  existing unknown-frontier fallbacks and missing-adapter refusals for actual source languages.

- `AZU-V0-9850`: (experimental, owner PR #99 repair request 2026-11-05) Make function
  expansions SHALL preserve GNU make's complete resulting words, including attached literal
  prefixes and suffixes, or retain the target as a computed invocation under AZU-V0-001/003.
  Approximate `join`, conditional and word-selection functions SHALL NOT silently drop an edge.
- `AZU-V0-9851`: (experimental, owner PR #99 repair request 2026-11-05) An `env` wrapper's
  `-C DIR`, `-CDIR`, `--chdir DIR` and `--chdir=DIR` SHALL resolve the wrapped command's
  directory or retain its target/job as a computed invocation, including computed directories.
- `AZU-V0-9852`: (experimental, owner PR #99 repair request 2026-11-05) Composite action
  steps outside the supported YAML sequence/mapping subset SHALL expose their dependencies or
  retain the action as a computed invocation, propagating selection to its callers and dependents.
- `AZU-V0-9853`: (experimental, owner PR #99 repair request 2026-11-05) Unresolved local
  `uses` paths, including action directories outside `.github/actions`, SHALL retain their
  caller as a computed invocation. A missing or invalid local lookup SHALL NOT imply no dependency.

- **AZU-V0-9930: Comparison-only working directory.** (experimental, owner lane charter,
  2026-11-05) The Go affected observer and mirrored gate selector SHALL exempt a qualified
  `os.Getwd()` call from rule (d) when its first result is a fresh local identifier in a
  two-identifier short assignment and every use of that binding is a direct equality or
  inequality comparison with a string literal. The proof SHALL resolve local bindings, reject
  shadowed import qualifiers, and preserve the original token offsets. A parse failure,
  reassignment, returned path, alias, closure escape, variable comparison or any other use
  SHALL retain the root locator. Dot imports and unsupported expressions remain conservative.
  This amends AFP-V0-012 rule (d) only for a proven comparison that consumes no file contents;
  path literals, dependent readers and real repository reads retain their existing selections.
- **AZU-V0-9931: Bounded test consumers.** (experimental, owner lane charter, 2026-11-05)
  The selection-precision slice SHALL use the existing AFP-V0-023 declaration grammar for
  bounded fixture/build consumers, retaining every test read and nested build input. Reasons
  SHALL be recorded per audited package in the build log. Existing read-scope checks SHALL
  validate the declarations. History readers, repository-wide scans and consumers without an
  admitted confinement proof SHALL stay unbounded. The four charter replay merges SHALL be
  measured before and after against their first parents, with immutable input identities and
  explicit declaration overlays recorded; no wire, golden, ownership or frontier policy change
  is part of this slice.

- **AFP-V0-3120:** **File-scope make ambiguity.** (experimental, owner lane selfloor charter
  2026-11-07, V1-5000) An ambiguous expansion in a make variable value, assignment, conditional
  or include that a target's own prerequisites, recipe or `$(shell ...)` do not reach SHALL NOT
  make that target a `driver:computed-invocation` site. Only an ambiguous `$(shell ...)` command,
  which make runs while it reads the file, or an ambiguous recipe of a special or computed-target
  rule SHALL mark every target of the file. An ambiguous expansion the target's own text reaches
  SHALL mark that target. Fault injection: one unreferenced `$(or ...)`, `$(if ...)` or
  `$(join ...)` line SHALL NOT turn a literal recipe into a site that claims a new sealed CEM.
  A bare reference line that can expand to text, which make reads as more make file, SHALL keep
  every target of its file a site.
- **AFP-V0-3121:** **Evaluated make alternatives.** (experimental, owner lane selfloor charter
  2026-11-07, V1-5000) The driver claimant SHALL expand each make recipe line, prerequisite list
  and include into every complete alternative value, walking at most 256 expansions per text. A
  variable takes one value per expansion from those its assignments can leave: every branch of a
  conditional, no assignment where a chain has no bare `else`, a target-specific value beside
  every global value (AFP-V0-3126), and `?=` and `+=` applied in file order. An immediate value
  is the one make captures where it reads the assignment (AFP-V0-3125). An unassigned value stays
  one only the environment supplies. An `if` condition, or an `or` or `and` argument, that only the environment
  decides SHALL take both outcomes. `filter`, `filter-out`, `findstring`, `firstword`, `lastword`,
  `word`, `wordlist` and `join` SHALL be evaluated exactly over values with no environment-supplied
  part. A word selected from a wholly environment-supplied value is that value or nothing. Each
  alternative SHALL be scanned as its own shell text, so its paths, make calls and Go package
  patterns are the target's edges. A change that any alternative can run or name SHALL still
  select the target and its callers. This evaluates exactly where AZU-V0-9702 and AZU-V0-9850
  allowed a retained site; AFP-V0-3122 decides whether the evaluated target stays a site.
  Filesystem/path functions (`wildcard`, `realpath`, `abspath`) SHALL NOT pass arguments through
  as results. Literal wildcard names proven to exist through the explicit repository source MAY
  return those names. Patterns, absent names and other unmodeled results SHALL stay unknown;
  an absent wildcard condition SHALL preserve the false branch and its dependencies.
  `strip` SHALL trim and collapse expanded ASCII whitespace to single spaces before testing
  a condition or constructing a package path; an unmodeled result SHALL retain uncertainty.
  Amended by the owner self-review repair charter (2026-11-07): condition alternatives SHALL
  preserve uncertainty about evaluation effects independently of value uncertainty. Only function
  forms with modeled evaluation effects MAY discharge that uncertainty; unsupported or future
  functions, including `file`, SHALL retain the frontier even when their return value is discarded.
  Eager arguments of diagnostic and metadata functions SHALL preserve nested effects. This applies
  to recipe expansions and immediate or other parse-time evaluation; no file read is executed.
  Amended by the owner independent-review repair charter (2026-11-07): filesystem effects
  SHALL survive discarded condition values. The bounded `wildcard` form admits only existing
  repository-relative literal names using ASCII letters, digits, slash, dot, underscore, hyphen
  and plus, and SHALL retain explicit read edges even when its value is discarded or captured
  immediately. Patterns, missing paths, `realpath`, `abspath` and unknown function forms SHALL
  retain effect uncertainty independently of branch enumeration.
  Amended by the owner self-review repair charter (2026-11-07): included target imports
  SHALL NOT certify included assignments or other evaluation effects. Until ordered include
  evaluation is modeled, every include form SHALL retain the including file's target frontiers,
  including target-free files. `notdir` SHALL remove through the last literal slash without
  path normalization, so trailing slashes yield empty words. Only `foreach` bodies whose list
  and variable are resolved and whose at most 256 items are fully visited MAY certify effects;
  exceeding the bound SHALL retain effect uncertainty even inside a discarded condition.
- **AFP-V0-3122:** **Claims only within a certified read set.** (experimental, owner lane selfloor
  charter 2026-11-07, V1-5000) An evaluated target is one whose own text, or its file's variables,
  resolved what the claimant before AFP-V0-3121 retained as ambiguous: a reassigned variable, or
  a `strip`, `if`, `or`, `and`, `filter`, `filter-out`, `findstring`, `firstword`, `lastword`, `word`,
  `wordlist` or `join`. Such a target SHALL drop its `driver:computed-invocation` entry only when
  every command its alternatives run is certified, because then the graph already bounds what it
  reads. A command is certified when it is one of:
  - a command that reads no file, or one that reads only literal operands in a known directory, in
    a mode AFP-V0-3128 admits;
  - a go package command other than `go run`, `go generate` or `go tool`, with no `-exec`,
    `-toolexec` or `-vettool`, whose only unknown words are output or selection values;
  - a make invocation of literal goals that overrides no variable, whose targets are units of
    their own;
  - a shell test script by the shell plugin's naming rule, run with literal arguments, which that
    plugin selects for itself.

  Every command make runs while it reads the file counts as run by every one of its targets
  (AFP-V0-3127).

  An evaluated target that runs any other repository program, an interpreter, `git` or an
  unlisted command SHALL keep claiming every dirty path, since its read set can include every
  tracked file. A certified target SHALL be selected for a dirty path only through its edges, with
  a dependency, path-literal or other reader witness. A dirty path that no alternative can run,
  name or read SHALL NOT select it or its dependents by `FRONTIER_SITE`. A new
  `.corvint/changes/<sha>.cem.json` or `docs/build-log/` entry is such a path unless an alternative
  names it. A make target that was never evaluated keeps the AZU-V0-001 literal edges unchanged.
- **AFP-V0-3123:** **Unbounded sites keep claiming.** (experimental, owner lane selfloor charter
  2026-11-07, V1-5000) A target or job whose alternatives cannot be bounded SHALL keep its
  `driver:computed-invocation` entry. As AZU-V0-001 and AZU-V0-003 require, it SHALL be selected
  with `FRONTIER_SITE` on every dirty path, together with its dependents. Such sites include:
  - a command word from the shell environment, or from a make variable only the environment can
    supply;
  - a `$(shell ...)` result used as a command;
  - `eval`, or an unsupported interpreter option;
  - a word function over a mix of literal and environment-supplied values;
  - alternatives past the walk bound;
  - an evaluated target that runs an uncertified command (AFP-V0-3122), including one with an
    unknown operand or in a mode that reads more than its operands (AFP-V0-3128);
  - every target of a file whose make reading runs an uncertified command (AFP-V0-3127);
  - every other existing computed-invocation case, including every workflow and action site.

  A missed alternative is worse than over-selection.
- **AFP-V0-3124:** **Selector parity.** (experimental, owner lane selfloor charter 2026-11-07,
  V1-5000) `internal/gateselect` attributes Go packages only and holds no driver unit or
  frontier-site claim. AFP-V0-3120 to AFP-V0-3123 SHALL change only a plan's driver selections:
  never its Go selections, scope or unknown entries, nor the selector's verdict on it. A parity
  test SHALL hold this over a fixture with bounded, unbounded and injected-fault sites, planned
  with and without the driver claimant.
- **AFP-V0-3125:** **Values where make reads them.** (experimental, owner lane selfloor review
  round 1 2026-11-07, V1-5000) Make expands some text as it reads the file: the value of an
  supported immediate assignment (`:=`, `::=`, `define NAME :=`), the text `+=` appends to a simple
  variable, a `!=` command, a conditional line, a bare statement, an include and a prerequisite
  list. The claimant SHALL expand each such text against only the values the assignments before
  it can leave. An immediate variable SHALL keep each captured expansion as its value, never the
  text re-expanded against later assignments. Example: after `PKG = ci`, `RUN := go test
  ./tools/$(PKG)` and `PKG = local`, a recipe running `$(RUN)` SHALL keep the `tools/ci` edge, and
  `tools/local` SHALL NOT reach it. An immediate value whose expansion the claimant cannot
  represent SHALL leave its variable unbounded, which retains every site that reads it. The
  values are those of the default invocation. As before, command-line and environment overrides
  of a variable the file sets stay outside the model. Amended by the 2026-11-07 orchestrator
  review decision under the 2026-11-02 delegation: assignment flavors SHALL be an exact
  allowlist. Only `=`, `:=`, `::=`, `?=`, `+=` and `!=` are modeled, including `define` and
  target-specific forms. An unsupported flavor, including immediate-recursive `:::=`, SHALL
  retain every target as a conservative site, never be treated as a simple assignment.
- **AFP-V0-3126:** **Target-specific values survive global assignments.** (experimental, owner
  lane selfloor review round 1 2026-11-07, V1-5000) A target-specific assignment SHALL stay an
  alternative whatever global assignment of the same variable follows it. Its `=`, `:=`, `?=` and
  `+=` SHALL apply over the global values make leaves at the end of the file, and over every
  earlier target-specific value. Example: with `check: PKG = ci` and then `PKG = local`, the
  recipe `go test ./tools/$(PKG)` SHALL keep the `tools/ci` edge.
- **AFP-V0-3127:** **Make-reading commands run for every target.** (experimental, owner lane
  selfloor review round 1 2026-11-07, V1-5000) Make runs some commands while it reads the file,
  before it builds any target:
  - every `$(shell ...)` and `!=` command that an AFP-V0-3125 text reaches, including one whose
    value no recipe uses;
  - the program a SHELL or `.SHELLFLAGS` assignment hands each recipe line, unless it is a plain
    shell with a `-c` flag. Repository shell wrappers SHALL NOT be certified by basename;
    only bare shell names and exact `/bin/<shell>` or `/usr/bin/<shell>` paths qualify here.

  The claimant SHALL scan each such command. One that is not certified (AFP-V0-3122) SHALL leave
  every evaluated target of the file uncertified, and one it cannot represent SHALL make every
  target a site. A `$(shell ...)` in a recursive value runs only where a recipe expands it, so it
  SHALL count only for that recipe's target. Example: `AUDIT := $(shell $(if
  $(CI),script/enumerate.sh,script/enumerate.sh))` SHALL keep every target claiming a new CEM even
  when no recipe uses `AUDIT`. Amended in selfloor fix round 2 (2026-11-07, orchestrator decision
  under the 2026-11-02 delegation): the repository's root `Makefile` SHALL run no uncertified
  command while make reads it. Its `WINDOWS_VET_GOCACHE` is therefore recursive, so the `stat`
  probe runs only where `windows-vet` expands it, and the same cache directory results
  (`AFP-V0-054`, `ASD-V0-061`).
- **AFP-V0-3128:** **Operand-bounded commands.** (experimental, owner lane selfloor review round 1
  2026-11-07, V1-5000) A command certified for reading only its operands SHALL also have:
  - every operand and option literal, with no value the environment or the shell supplies;
  - a known working directory, so a `cd` to an unknown directory leaves later relative operands
    uncertified;
  - no input redirection from an unknown file;
  - no operands that `xargs` reads from input.

  Literal file operands SHALL be represented even without a slash or extension (for example,
  `cat LICENSE`), including input redirections and test file operators. Their path tokens SHALL
  select unowned file readers as well as path imports reaching owners. Deferred `trap` bodies
  SHALL remain uncertified because their execution environment and reads are not bounded here.

  A test command with a file operator or `-nt`, `-ot` or `-ef` on an unknown operand SHALL NOT be
  certified. Certification is per command. A checksum command in `-c`/`--check` mode reads the
  files its manifest lists, so it SHALL NOT be certified. Neither SHALL `--files0-from`, a
  recursive `grep`, `cp` or `diff`, a jq library path or module directive, or `awk` and `sed`,
  whose programs can read any file or run one. A command that reads no file at all (`echo`,
  `printf`, `basename`, `dirname`, `sleep`, `seq`, `expr`, `pwd`, `uname`, `id`, `mktemp`) MAY
  take unknown operands. Amended by the 2026-11-07 orchestrator review decision under the
  2026-11-02 delegation: command options SHALL be certified by exact allowlisted spelling and
  arity only. Abbreviated or unknown long options and unknown short options SHALL NOT be
  certified. A go command SHALL have only recognized flags and exactly bounded literal values;
  a value from an unbounded variable SHALL NOT be certified, including an output or selection
  flag's value, whether separate, attached with `=`, or quoted. An unquoted unknown value can
  split into executable flags such as `-toolexec`. Unknown constructs SHALL retain conservative
  frontier claims and run-all behavior; no wire shape, error or output contract changes.
  Attached option values that can name input files, including extensionless `--file=LICENSE`,
  SHALL have explicit dependency edges or remain uncertified. Make options SHALL use an exact
  allowlist of modeled forms; evaluation, loading, include search paths and unsupported forms
  SHALL retain claims. Explicit command-local `GOFLAGS` SHALL remain part of certification,
  including through `env` and shell wrappers. Nonempty or unresolved values SHALL remain
  uncertified until their effects are modeled.
  Amended by the owner self-review repair charter (2026-11-07): file operands SHALL be literal
  throughout the word, including after `--`; glob characters at any position SHALL retain claims
  until complete read sets are represented. The sole admitted glob form is a literal relative
  directory followed by `/*` and a literal suffix, with an explicit directory read edge covering
  every match, including newly added files. The directory SHALL NOT be the root or an option;
  directory and suffix characters are ASCII letters, digits, slash, dot, underscore, hyphen and
  plus only, with no additional glob or expansion syntax. Persistent shell environment mutations
  and make exports SHALL retain claims unless their effects belong to an explicit modeled allowlist.
  These effects SHALL retain the frontier independently of make-expansion activity, including
  literal recipes and command-local nonempty or unresolved GOFLAGS. The owner independent-review
  repair charter (2026-11-07) explicitly permits this test-selection soundness repair.
  Shell export/declaration mutations have no admitted forms. Make exports admit only the project
  controls `CORVINT_REQUIRE_NODE`, `HELP_BASE`, `LANE`, and `DOGFOOD_FINISH_ENTRY`, which the
  certified built-in commands do not interpret, and only when their value expansion has no
  unmodeled effects or shell execution. Other names, including `GOFLAGS`, computed names,
  export-all and unsupported forms SHALL remain uncertified.
  Command-local and standalone assignments, including through `env`, SHALL use a closed effects
  allowlist: `GOCACHE` (cache location) and empty `GOFLAGS`. Every other name or form, including
  shell startup-hook settings and future unknown names, SHALL retain the frontier independently
  of make expansion. Assignment effects SHALL survive shell wrapping and later assignments.
- **AFP-V0-9050:** **Exact make function spelling.** (experimental, PR #142 Greptile P1 repair,
  2026-11-07, V1-5000) An AFP-V0-3121 make function whose value can reach a condition or a
  filter SHALL return the exact text GNU make returns, or SHALL retain. In particular:
  - `dir` keeps each word's literal prefix through its last slash, or `./`, never a cleaned path;
    `notdir` keeps an empty word for a trailing slash; `suffix` emits nothing for a word without
    one; `subst` with an empty search string appends its replacement once, at the end;
  - blanks after a function name are not part of its first argument, and a substitution
    reference expands its search and replacement before matching;
  - `if`, `or` and `and` strip only ASCII whitespace from an unexpanded argument, and word and
    wordlist indexes are decimal digits only.

  It SHALL retain a value that holds a byte outside ASCII, whose word splitting depends on make's
  locale; a word only partly known to `dir`, `notdir`, `basename`, `suffix` or `patsubst`; a
  pattern in which a backslash may quote a percent sign; and a `wildcard` name that is not a
  clean, distinct spelling of a regular file or directory reached without a symlink.
- **AFP-V0-9051:** **Unrecordable operands never certify.** (experimental, PR #142 Greptile P1
  repair, 2026-11-07, V1-5000) A command SHALL be certified only when every file it may read
  has a recorded repository edge. An absolute, `~`-prefixed, repository-escaping or empty file
  operand, or any file operand in an unknown directory, SHALL leave the command uncertified, on
  every certification path:
  - bounded-command operands, including after `--`, and file-naming option values (`grep -f`,
    `sort -o`, `-T`);
  - input redirections and test-command file operands;
  - a shell test script and each of its arguments other than a bare option, which SHALL be
    recorded as file operands;
  - make goals and `-C` directories, go package patterns and `-C` directories, and every go
    test argument after `-args` that names a directory.

  `/dev/null` reads nothing. A `tr` set, `jq`'s program and `grep`'s first operand when no `-e`
  or `-f` supplies the pattern are not file operands.
  A dash-prefixed operand other than `-` SHALL NOT be recorded, so it leaves the command
  uncertified, and an operand SHALL be recorded only when the scanner holds the source and every
  component of its path is a plain file or directory there, not a symbolic link (Greptile P1
  repair 2).

### Lane qualification details

- **AZU-V0-8900:** **Conservative verification ownership.** Remaining tracked data MAY be assigned
  verification owners through the existing `corvint-test-selection/0` contract. An owner SHALL add
  its tests without removing any existing reader, executor, import or embed selection. Owner paths
  SHALL remain unowned for frontier bearing. The exact parser-owned control declarations SHALL count as declared ownership in the audit,
  even when frozen selection rules retain uncertainty for a reader of the declaration. No such
  audit claim may remove a selection or a plan unknown. The maintained ownership baseline SHALL be empty;
  a new tracked path outside all claims SHALL fail the ownership gate. Declarations include
  `.github/forward-rows/tiers.jq` and `script/ci-least-privilege-exceptions.tsv`.
- **AZU-V0-8901:** **Constructed foreign callers.** The shell caller completeness guard SHALL reject
  an undeclared source file constructing a declared callee directory through a quoted directory
  concatenation, a path join or a formatted directory path. Existing shell value expansion SHALL
  remain in force. These checks widen possible callers; they SHALL NOT infer that another caller
  cannot exist. Existing declaration shapes, bounds and invalid-declaration diagnostics stay fixed. A declaration
  newly rejected only by the stronger directory-construction check SHALL retain caller edges
  admitted by the legacy literal/shell-expansion guard, while retaining both invalid-declaration
  and foreign-invocation unknowns. New uncertainty SHALL never remove those prior selections.
  Root-directory callees SHALL also check computed basenames: without a directory literal to
  exclude a callee, joins, quoted concatenations and formatted paths remain possible callers.
  A literal prefix SHALL NOT exclude an unresolved intermediate join operand that may traverse
  out of that prefix before later operands name the callee directory.
  A fixed final suffix MAY exclude that join only when it cannot name any supported shell script;
  partial shell suffixes and components removed by path cleaning are not exclusion proofs.
  A dynamic final operand with a literal suffix, bare (`name+".sh"`), parenthesized as a whole or
  with a grouped suffix (`name+(".sh")`), MAY carry `..` itself and SHALL count as traversal. Known limit: a bare identifier final operand
  (`filepath.Join("elsewhere", name)`) is still read as a file directly under its literal
  prefix, because counting it as traversal flagged 75 source files against all 34 `script/`
  callees on 2026-11-06 and made completeness unusable; V1-4779 tracks a bounded sound rule.
  For root and nested scripts alike, quoted computed basenames include backtick substitution,
  `$()`, `${}`, `$VAR` and percent formats, including paths after a command prefix.
- **AZU-V0-8902:** **Located runtime caller closure.** A located TypeScript end-to-end runtime site
  covered by an existing runtime edge SHALL select its declared test unit and traverse its
  dependents on every nonempty code-bearing change, including changes within TypeScript. A site's
  file drops its backquoted token and then its line, so a line-less site is located too
  (AFP-V0-9009). Only after selection
  SHALL that site's inbound unknown close. Unlocated, truncated, stale or undeclared sites SHALL
  remain unknown. A declared-data-only change SHALL not introduce runtime code selections. Computed shell
  outbound targets retain uncertainty until modelled; the literal repository-root query of a
  wrapper MAY resolve its known root-relative shell target. Changed caller declarations use
  AZU-V0-8904; without historical caller completeness they retain their existing uncertainty.
- **AZU-V0-8903:** **Replay evidence admission.** The replay SHALL use a supplied implementation binary
  over exact historical merge trees in private clones outside the repository. It SHALL retain the
  binary digest, frozen-list digest, immutable merge/parent/tree IDs, per-merge plans, unknown
  entries and counts, selection/test counts and wall time. An optional baseline binary SHALL
  produce named removed unknowns, added selections and dropped selections. The initial eight full
  SHAs are committed; subsequent first-parent merges through a pinned local ref are appended.
  Safety receipts SHALL bind each merge and parent, complete suite inventories and per-test
  outcomes at both revisions, plus mutation observations bound to unique mutant IDs and changed lines on either the parent
  or merge side, including deleted lines. Missing or invalid receipts, including a parent test absent from the merge outcomes,
  SHALL report `NOT_PRODUCED`, never safety success. Every changed or new outcome and every mutant
  killer SHALL belong to a selected unit; a deliberately removed killer or differing-outcome unit
  SHALL fail the same checker. External runner completeness statements are required evidence,
  not authenticated by this comparator; it cannot establish mutation adequacy or independent
  full-suite execution itself. AZU acceptance SHALL remain false with any unknown or absent safety
  receipt. No synthetic fixture result may fill a historical qualification cell.

These details preserve the rollback rule above. The lane's repository ownership and constructed
caller tests are verification evidence; overall AZU-V0-009 promotion still requires the aggregate
lanes and external full-suite/mutation observations at every historical revision.

- **AZU-V0-8904:** **Revision-bound caller changes.** A changed shell caller declaration SHALL close
  its change frontier only with bounded base/head content observed at the measured revisions and
  a current declaration admitted by the shell plugin. Selection SHALL seed the union of the old
  and new callee/caller/mention paths and traverse their dependents, test users and executable
  users. A missing historical read, invalid declaration or caller absent from the current graph
  SHALL retain the existing changed-declaration unknown. Both snapshots SHALL pass the canonical
  shell declaration validation: nonempty, sorted, unique, disjoint caller/mention lists, clean
  paths and supported script suffixes, and bounded, nonempty, control-free UTF-8 reasons.
  A valid current declaration SHALL NOT discharge an invalid historical declaration.
  A bare selector without revision content
  SHALL keep its existing uncertainty. This uses the existing measured-content seam and adds no
  receipt members or diagnostics.

- **AZU-V0-8905:** **Absent driver identity parity.** The optional driver claimant SHALL not change
  the canonical graph identity of a tree with no driver units, frontiers or unobserved/runnable
  directories. Its claimant MAY still recognize a probed new driver path. Existing graph digests
  and committed application-flow goldens SHALL remain unchanged for such trees.
  The default provider registry SHALL retain its already-frozen driver namespace even without
  driver files; optional-provider parity SHALL not remove that historical namespace.

- **AZU-V0-9900: Ownership ratchet selection (V1-4729, owner lane charter 2026-11-05).**
  An addition, deletion or rename of a tracked path, or an edit to
  `.corvint/documentation-paths.json`, `.corvint/runtime-readers.json` or
  `.corvint/non-source-paths.json`, SHALL select `TestOwnershipAuditRatchet_AZU_V0_002`
  when its test file exists in the observed graph. `affected` SHALL name that test and the
  triggering path; `test --dry-run` SHALL carry the same reason, including during FULL fallback.
  FULL removes narrowing patterns while retaining explanatory reasons. Index membership used
  for selection SHALL be retained and rechecked before emitting a receipt; drift is refused
  through the existing worktree-drift diagnostic. A gate-only selection MAY run
  just that named test; any independently reached package coverage SHALL remain whole.
  Staged membership changes count even if the indexed addition is missing from the worktree;
  an untracked file alone does not. Content-only changes to
  other existing files SHALL add no selection or alter frozen dry-run output.
- **AZU-V0-9901: Committed ownership guidance and fixture closure.** A committed
  `UNOWNED_DIRTY_PATH` SHALL retain its reason and name `runtime-readers.json` for test/runtime
  inputs, `documentation-paths.json` for documentation, and `non-source-paths.json` for output.
  The compatible diagnostic channel is `affected --format text`: every unowned-path line
  provides this guidance, including committed content-only edits without the ratchet. JSON
  receipts and frozen content-only dry-run bytes retain their existing contracts.
  A newly tracked fixture below a Go-hosted `testdata/` ancestor whose package is selected
  SHALL NOT also bear unrelated language-frontier unknowns when no other plugin lists content
  there or treats the path as a resolution input and the existing conservative `FrontierReach`
  classifier proves closure per plugin and reason. Dynamic and unclassified frontiers remain
  open, including plugins without `FrontierReach`. Independent participating, reached and
  resolution-input frontiers SHALL keep their reasons. The missing declaration remains unknown;
  selecting the ratchet is no proof of ownership and cannot admit arbitrary undeclared readers.
  When every unowned path is such a tracked fixture membership change, terminal readers of
  other changed paths SHALL retain terminal status: each unowned-path frontier is judged by
  that same per-plugin/reason classifier instead of promoting the whole plugin. A non-fixture
  unowned path, content-only plan, or unselected Go fixture host retains prior participation.
- **AZU-V0-9902: One audit classifier.** The CLI and maintained ratchet SHALL use the same
  `AuditOwnership` code path and return the same raw unowned list for the same tracked tree.
  The CLI is the complete inventory required by AZU-V0-002; the ratchet additionally checks
  additions and stale entries against its pre-existing shrinking baseline. A passing ratchet
  does not assert an empty inventory. The baseline SHALL NOT claim paths, change selection,
  suppress unknowns or change the CLI's inventory or exit contract.
- `AZU-V0-9920`: (accepted under the owner's standing auto-accept (2026-10-11); fullplan charter 2026-11-05; amended under the owner's standing auto-accept (2026-11-05), fullplan Verifier repair 1) The Go and Python plugins SHALL index helper sources and tests below `.github/` through the bounded, cache-stamped source reader, on disk and immutable inputs. Existing module boundaries, skipped subdirectories and source failures remain effective. A helper edit SHALL select its tested unit and modelled or declared consumers; the dynamic load of `.github/macos/darwin_select.py` by `select_test.py` is declared through `.corvint/runtime-edges.json`. The maintained test-command declaration SHALL name the existing CI Python suite, `python3 -B .github/macos/select_test.py`; the replay SHALL use that declaration and check the local invocation and required CI check (STR-V0-004). The Go runner SHALL confirm these indexed helpers through its real inventory probe as well as ordinary packages, retaining refusal for extra or missing units (STR-V0-069). Its independently enumerated helper directories SHALL retain the graph's skipped-directory, testdata and nested-module boundaries. Test read declarations SHALL bound helper tests when confinement qualifies their actual reads (AFP-V0-023); discovery alone SHALL NOT add an unreasoned unbounded-reader exemption. This is the `.github/` slice of AZU-V0-005; for Go, AZU-V0-005's general hidden-directory walk supplies this discovery (selbatch merge, 2026-11-06), while the Python plugin keeps its bounded `.github` reader.
- `AZU-V0-9921`: (accepted under the owner's standing auto-accept (2026-10-11); fullplan charter 2026-11-05) A path covered by `.corvint/non-source-paths.json` that no indexed unit owns SHALL be classified as `DECLARED_NON_SOURCE` even when a language claims its suffix. This applies to retained `.agent-evidence/` scripts and source-shaped dumps under AZU-V0-002. The declaration itself adds no test units; existing embed, literal, declared, enclosing-directory and unbounded readers remain selected, and frontier diagnostics remain effective. Indexed source, driver inputs, task projections and undeclared source SHALL retain their existing treatment. In particular, an undeclared unindexed source still forces FULL under AFP-V0-028; its allowlist is unchanged.
- `AZU-V0-9922`: (accepted under the owner's standing auto-accept (2026-10-11); fullplan charter 2026-11-05) Reduced replay fixtures for merges `39c031980`, `8e2c13aa9` and `cdeed7ee6` SHALL select the named helper tests, classify retained evidence without adding units or unindexed uncertainty, and retain FULL for new unknown source. Acceptance retains before/after unit counts with the replay input scope, complete touched-package tests, and declaration-reader safety checks. No wire shape, output golden or workflow change is required.
- `AZU-V0-9923`: (accepted under the owner's standing auto-accept (2026-10-11); selbatch Greptile fix round 1, 2026-11-07, PR #125) The declared Go full suite (`declared:go`, STR-V0-006), which a FULL run schedules when it has no Go universe, SHALL NOT pass over Go packages that `./...` skips. The session runner SHALL confirm the suite with the same hidden-package helper as the root Go adapter (AZU-V0-005, AZU-V0-9920): with a planned universe, discovery lists `./...` plus the `./.github/<dir>/...` patterns, `./.github` and the indexed dot and underscore packages, validates the complete universe, and the suite's argv adds the same patterns. With no plan (STR-V0-003: the graph failed, for example on an invalid declaration) there is no universe, so the Go plugin's own bounded hidden-source walk, with its testdata and declared non-source treatment, SHALL find no Go source below a `.` or `_` directory. Otherwise, or when that walk fails or is bounded, the suite SHALL NOT launch and the shard SHALL be INCOMPLETE, naming the first such file; a hidden file inside a nested module also counts, conservatively. A no-plan repository without hidden Go source still runs `go test ./...` and may pass. This amends STR-V0-003 and STR-V0-004 for the declared Go suite only; other declared commands are unchanged.
- **AZU-V0-9950: Named non-source declaration faults.** (experimental under the standing lane
  auto-accept, 2026-11-06; V1-4919) When `.corvint/non-source-paths.json` has an entry out of
  strictly ascending byte order, a duplicate path, or an undeclarable path, the refusal SHALL name
  the entry's index and path and say which rule it breaks: for order, the earlier entry it fails
  to follow and the fix (move it, or reorder all entries with `jq '.entries |= sort_by(.path)'`);
  for a duplicate, the entry it repeats; for an undeclarable path, the declarable forms. The
  accepted grammar and its fail-closed result are unchanged. Evidence:
  `TestNonSourcePathsNamesOrderingFaults_AZU_V0_9950`. Rollback: restore the single `path %q`
  error.

## Non-goals

- No requirement here may drop a reached, reading, executing, importing or embedding unit. Every
  rule turns an unknown into a selection or a stated proof.
- AZU-V0-003 can widen a selection (all site units of a bearing reason). Only the bounded
  reader precision slice AZU-V0-9930/9931 narrows rule (d) here; it does not change frontier
  classification or claim that all unknowns are closed.
- No embeddings, network or learned ranking, and no change to the authority order.
- AFP-V0-3120 to AFP-V0-3123 evaluate make expansions only. Shell variables, loops and globs in a
  recipe or step, interpreter options such as `node --test` or `python3 -B`, and binaries that a
  workflow builds or downloads at run time keep their computed sites. As before, make
  command-line and environment overrides of an assigned variable stay outside the model.
- AFP-V0-3122 certifies commands; it does not model what a repository script or Go program reads.
  Those read sets belong to the reader rules, such as unbounded and broad-scope readers. Until a
  reader rule bounds them, an evaluated target that runs them keeps its claim. A literal target
  that was never evaluated is unchanged, including one that runs a script listing every tracked
  file.

## Failure modes

- A plugin reports an `OUTBOUND` reason whose hidden edge can start outside its site. Then
  AZU-V0-003 is unsound. Mitigation: each `OUTBOUND` declaration needs a fixture test showing that
  the edge's start is the site file, and review rejects an `OUTBOUND` label without one.
- A plugin under-reports site units. Mitigation: a test checks that `frontierSites` counts equal
  the graph's per-unit frontier occurrences for every plugin.
- A declaration drifts from the code (a caller is added and not declared, a new tracked path has no
  owner). Mitigation: the AZU-V0-002 and AZU-V0-004 checks run in the doc gates and CI and fail
  closed.
- A driver hunk is misattributed to the wrong target. Mitigation: an unparsable make file or
  workflow selects every target or job of that file.
- The AFP-V0-3121 walk misses or mis-evaluates an alternative, which would drop an edge.
  Mitigations:
  - only the listed functions are evaluated, each exactly and over definite values;
  - any function over a value it cannot represent still retains the site;
  - a condition the walk cannot represent takes both branches;
  - the walk is bounded, and text past the bound retains the site;
  - the fixtures pin selection for every evaluated form.
- AFP-V0-3122 certifies a command that reads more than its operands, or a script the shell plugin
  does not index as a test. Mitigations:
  - the certified list is closed and small, and recursive options are refused;
  - `go run`, interpreters, `git` and every repository program other than a shell test are never
    certified;
  - `TestFrontierSiteShellTestParity_AFP_V0_3122` holds every certified script to the shell
    plugin's test units;
  - AFP-V0-3128 certifies per command and mode, and refuses every unknown operand;
  - AFP-V0-3127 counts every command make runs while it reads the file, including one whose
    value no recipe uses, as run by every target. Review round 1 found the first lane head
    discarding that uncertainty.
- The claimant expands a value against assignments make has not read yet, or loses a
  target-specific value, which would drop the edge make actually runs. Review round 1 found both
  in the first lane head. Mitigations:
  - AFP-V0-3125 expands each read-time text against strictly earlier events;
  - AFP-V0-3126 keeps every target-specific value;
  - regressions pin the review's exact orderings.

## Acceptance evidence

V1-4729 witnesses AZU-V0-9900 with `TestOwnershipMembershipSelection_AZU_V0_9900` and
`TestOwnershipContentOnlyBytes_AZU_V0_9900`, AZU-V0-9901 with
`TestOwnershipReplay0c8ae2da9_AZU_V0_9901`, and AZU-V0-9902 with
`TestOwnershipCLIAndRatchetAgree_AZU_V0_9902` in `cmd/corvint/affected_ownership_selection_test.go`.
The lane build log retains the historical replay counts, frozen-byte comparison and limits.
Independent review repair resolves the former guidance gap through the existing text diagnostic
channel. `TestOwnershipTextGuidance_AZU_V0_9901` covers committed content-only unowned edits
with and without the ratchet; `TestOwnershipGuidanceRequiresRatchet_AZU_V0_9901` continues to
check the JSON compatibility boundary. `TestOwnershipIndexDriftAfterSources_AZU_V0_9900`
stages an initially untracked path after source observation and requires the existing drift refusal.
`TestOwnershipInvalidUTF8Membership_AZU_V0_9900` covers raw Git filename additions,
deletions and replacements sharing a normalized dirty path, while content-only edits retain
their selection boundary. `TestOwnershipInvalidUTF8IndexDrift_AZU_V0_9900` requires drift
refusal when distinct raw index names share that normalized path. These Unix witnesses live
in `cmd/corvint/affected_ownership_invalid_utf8_unix_test.go`.
`TestOwnershipFullDryRun_AZU_V0_9900` covers public dry-run FULL fallback after deleting a
package's last source while retaining its directory. `TestOwnershipFullReasons_AZU_V0_9900`
in `internal/sessiontest/ownership_full_test.go` covers both gate-only and whole-package reasons
and verifies the caller's narrowing input is unchanged.
`TestOwnershipPartitionedReasons_AZU_V0_9900` covers the committed affected-package slices
across 16 shards under universe, balanced and measured placement: each slice, including the
catch-all, retains the obligation's reason and reason code during narrowed and FULL runs.
`TestOwnershipPartitionedContentOnlyBytes_AZU_V0_9900` checks serialized manifest bytes and
manifest digests for ordinary content-only selections in those same placements, including
FULL fallback: partitioned units retain their prior omission of ordinary package reasons.
`TestOwnershipContentOnlyBytes_AZU_V0_9900` compares owned and unowned plan bytes and compact
dry-run digests. `TestOwnershipMembershipSelection_AZU_V0_9900/stage-worktree-delete` covers an
indexed addition subsequently removed only from disk. Fixture closure uses the existing
per-plugin/reason classifier; `TestOwnershipFixtureComputedImport_AZU_V0_9901` retains a real
TypeScript computed `require` frontier, and
`TestOwnershipFixtureFrontierClassification_AZU_V0_9901` retains dynamic, bounded and
unclassified frontiers while excluding only classified unrelated reasons.
`TestOwnershipFixtureTerminalReaders_AZU_V0_9901` covers terminal readers alongside
resolution inputs, foreign fixture content, owned-source changes, unbounded readers and
another unowned path. `TestOwnershipReplayTerminalReader_AZU_V0_9901` reproduces the
historical source-edit-plus-fixtures shape with a real terminal TypeScript reader: its
source-bound import reason is excluded while its computed data-load reason remains unknown.
The terminal-reader fixture witness also checks that its exclusion certificate names the
selected Go host and each excluded entry's `FrontierReach` classification while the fixture
remains explicitly unowned. Ordinary terminal exclusion explanations retain their bytes.
Rollback removes these selection inputs and restores the missed ownership-gate risk; no JSON
shape, error/exit contract, committed golden or baseline changes are authorized.

The pk-affected independent-review repair checks AZU-V0-001 with
`TestDriverTestOnlyPackageChange_AZU_V0_001` (target, direct job and transitive callers,
with ordinary Go importers still excluded), `TestDriverGoGlobComputed_AZU_V0_001`,
`TestDriverGoEqualsDirectory_AZU_V0_001`, and
`TestDriverInlineInterpreterComputed_AZU_V0_001` in `driver_review_test.go`.
The glob and inline cases check the stored frontier as well as caller selection;
the directory case checks that an unrelated package does not select those drivers.
Acceptance requires these regressions and the complete affected package. This repair
does not qualify AZU-V0-009. Reverting it restores the four missing-driver risks,
without a wire migration or changes to Go importer containment.

The PR #97/#98 merge integration declares the three exact retained linecov experiment JSON
outputs and `tools/linecov/command-tree.json` under AZU-V0-002. Existing literal, enclosing-package,
declared and unbounded readers remain selected; the ownership ratchet baseline is unchanged.
`TestWitnessesResolveFromTestlessReader_AFPV0495` checks the live-witness oracle's existing
AFP-V0-495 rule: a dependency chain may start at a testless literal reader of an unowned path,
but a tested reader, owned path, unnamed path or invalid edge cannot justify that origin.

The second PR #98 repair declares the native coverage README as documentation under AZU-V0-002,
maintains that explicit tracked path in `script/gen-documentation-paths.sh`, and checks the
complete affected package including the ownership ratchet and sibling audits. The generator
fixture checks its untracked, staged and removed states without broadening other internal paths.
AZU-V0-9704 is checked by `TestDriverUnknownIsNotLanguage_AZU_V0_9704` and the unchanged selection
baseline replay, including PR 6 compared with the `8e2c13aa9` binary. No corpus, threshold or wire
shape changes are part of this repair. Reverting its runner classification restores the spurious
driver-language incompleteness while leaving affected selections unchanged.

Lane selfloor witnesses its nine rows with fixtures:
- `TestFrontierSiteFaultInjection_AFP_V0_3120` failed on the base for the injected `$(or ...)`,
  `$(if ...)` and `$(join ...)` cases, and its control passed.
- `TestFrontierSiteBoundedAlternativesKeepTheirEdges_AFP_V0_3121` includes a recipe that names
  `.corvint/changes`, which stays selected.
- `TestFrontierSiteBoundedSetExcludesPath_AFP_V0_3122` and
  `TestFrontierSiteUnboundedKeepsClaim_AFP_V0_3123` cover the dropped and kept claims, including
  evaluated targets that run a repository script, `go run`, `git ls-files`, an interpreter or a
  recursive `grep`. `TestFrontierSiteShellTestParity_AFP_V0_3122` holds the shell-test rule.
- `TestDriverFrontierBoundParity_AFP_V0_3124` in `internal/gateselect` holds the selector parity.
- Each review round 1 finding has its own regression:
  - `TestFrontierSiteImmediateValueKeepsCapturedEdge_AFP_V0_3125`;
  - `TestFrontierSiteTargetSpecificSurvivesGlobal_AFP_V0_3126`;
  - `TestFrontierSiteParseTimeCommandKeepsClaim_AFP_V0_3127`, and for the root `Makefile`
    `TestFrontierSiteRootMakefileRunsNoReadTimeCommand_AFP_V0_3127`;
  - `TestFrontierSiteUnboundedOperandKeepsClaim_AFP_V0_3128`, with the control
    `TestFrontierSiteBoundedOperandDropsClaim_AFP_V0_3128`.

  Every positive case failed on the first lane head's sources, and every control passed.

The next independent review found immediate-recursive assignment, unbounded Go flag-value
injection and checksum-option abbreviation gaps. The orchestrator's 2026-11-07 allowlist decision
is exercised by `TestFrontierSiteUnsupportedAssignmentKeepsClaim_AFP_V0_3125`,
`TestFrontierSiteGoFlagAllowlist_AFP_V0_3128` and
`TestFrontierSiteCommandOptionAllowlist_AFP_V0_3128`. These retain the target and its dependents
for an unrelated sealed CEM, including unrecognized constructs. The previous Go-output control
keeps its assertions with a literal cache value; its original unbounded value is now a negative
case under the amended contract. Run results and digests live in
`docs/build-log/2026-11-07-selfloor-review-allowlists.md`. Earlier replay numbers below describe
the earlier implementation; this repair makes no new precision or replay claim. Rollback must
retain conservative frontier claims for these forms; restoring permissive certification restores
the three reviewed missing-edge risks.

`TestDriverAlternativeMakeValues_AZU_V0_9702` and `TestDriverMakeFunctionsComputed_AZU_V0_9850`
now assert the exact edge and an unrelated path's exclusion where the value is definite, and the
retained site where it is not. The 20-merge selmeasure replay at `538e823f4` is in
`docs/build-log/2026-11-07-selfloor.md`. With the review round 1 repairs:
- every merge selects exactly the base's units: 6,876 in all, the same test files, and the same
  unknown entries and scope;
- `FRONTIER_SITE` units fell only from 1,573 to 1,550, because those units are now reached by
  an edge first;
- the first lane head's 716 dropped units all came back. The root Makefile's immediate
  `WINDOWS_VET_GOCACHE` runs `$(shell stat -f -c %T ...)` while make reads the file, on a
  directory the environment supplies, so AFP-V0-3127 and AFP-V0-3128 leave every evaluated target
  of that file uncertified;
- no unit was dropped, so no miss is possible against the base.

Fix round 2 made that assignment recursive (amended AFP-V0-3127). The regression builds the root
Makefile's driver graph: 115 of its 188 targets are computed sites, against all 188 with the
assignment immediate, and `windows-vet` stays one. The replay applies the one-line change to both
the merge base and the PR head of each of the 20 merges:
- 686 root make targets drop their `FRONTIER_SITE` claim;
- units fall from 6,876 to 6,190, and `FRONTIER_SITE` units from 1,573 to 864 (CEM witness:
  1,487 to 820);
- test files, dirty paths, unknown entries and scope are unchanged on every merge, and no unit is
  added;
- every dropped unit is a root make target the round 1 model certifies, so this is the same model
  applied to the changed file, not a new rule.

Rollback reverts the lane's code commit. That restores collapsing every unrepresentable or
alternative make value into a computed site, with no wire or receipt change. Reverting the fix
round 2 Makefile line restores the immediate `WINDOWS_VET_GOCACHE`, which only widens selection
again; drop its regression with it.

The PR #99 repairs (AZU-V0-9850 through AZU-V0-9853) extend the same driver soundness boundary.
Acceptance requires the table regressions below and the complete affected package. Unsupported
make functions, env directory changes and unresolved local actions deliberately widen selection
at their invocation sites; they do not claim exact evaluation or arbitrary action discovery.
Rollback reverts these scoped repairs and restores the documented missing-edge risk. No wire
format or wider zero-unknown qualification changes are claimed.

The four PR #98 driver repairs (AZU-V0-9700 through AZU-V0-9703) are scoped corrections of
AZU-V0-001. Their acceptance evidence is the table regressions below, the complete affected
package, and the focused CLI affected and frozen-profile tests. This does not qualify the wider
AZU-V0-009 zero-unknown goal or a new wire format.

Each requirement's lane retains: unit tests for the rule; the AZU-V0-009 replay before and after,
showing which unknown entries it removed and the selection growth; and for AZU-V0-003 and AZU-V0-004
the soundness fixtures above. AZU-V0 as a whole is accepted when the replay shows `unknown: []` on
every listed merge with zero safety misses.

The fullplan slice (AZU-V0-9920 through AZU-V0-9922) is bounded to `.github/` helper discovery and declared non-source precedence over suffix-only claims. `TestFullplanReplay_AZU_V0_9920_9921` covers disk and immutable plans, the Python runtime edge, Go helper dependencies, zero-unit evidence and the unchanged FULL fallback. Whole touched-package results and path-set replay measurements are retained in `docs/build-log/2026-11-05-fullplan-selection-precision.md`. Existing consumers of evidence still run; zero units describes an isolated fixture without such readers, not permission to discard real readers. Revert the discovery changes, declared Python edge and non-source precedence to restore conservative FULL behavior. Wider AZU-V0 acceptance, native Darwin execution and hosted workflow behavior remain unqualified. AZU-V0-9923 closes the no-plan FULL fallback: its declared Go suite shares the root adapter's hidden-package helper and, with no universe, refuses to pass over hidden Go source; revert it to restore `go test ./...` alone, which reports PASS over skipped hidden tests.

## Rollback

Each requirement only turns an unknown into a selection, behind its own code path. Reverting its
commit restores the earlier `UNKNOWN` plan, which consumers already treat as a wide or full run.
Declarations added for AZU-V0-002, AZU-V0-004 and AZU-V0-007 are inert without their readers.

## Traceability

| Requirement | Tickets | Evidence |
|---|---|---|
| AZU-V0-001 | V1-4684 | `Driver` claimant in `internal/liveverify/affected/driver*.go`; `TestDriverImplicitGoPackage_AZU_V0_001`, `TestDriverImplicitGoPackageDirectory_AZU_V0_001`, `TestDriverFlowMappingKeepsUncertainty_AZU_V0_001`; `driver_test.go` (per-hunk, remainder, unparsable, removed-block, deleted-file and `FRONTIER_SITE` cases); replay `--base 39c031980` at `8e2c13aa9`: unknown 92 to 89, `UNOWNED_DIRTY_PATH` 5 to 2, selected 266 to 488, no selection dropped (`docs/build-log/2026-11-04-azu-driver-claimant.md`) |
| AZU-V0-002 | V1-4685; V1-4659, V1-3480 | `affected --ownership-audit`, `.corvint/non-source-paths.json`, `TestOwnershipAuditRatchet_AZU_V0_002` over an empty baseline, `TestRepositoryOwnershipComplete_AZU_V0_8900` (including a new unclaimed path), `TestAuditControlKeepsSelectionUncertainty_AZU_V0_8900`; aggregate safety qualification remains external |
| AZU-V0-9704 | PR #98 repair round 2 | `TestDriverUnknownIsNotLanguage_AZU_V0_9704` in `internal/sessiontest`; unchanged `make doc-gate/selection-baseline-check` corpus |
| AZU-V0-003 | V1-4686 | not yet |
| AZU-V0-004 | V1-4687; V1-4392, V1-4535 | `TestRepositoryInboundCallers_AZU_V0_8901`, `TestConstructedForeignCallers_AZU_V0_8901`, `TestRootConstructedForeignCallers_AZU_V0_8901`, `TestRawStringJoinRetainsUnknown_AZU_V0_8901`, `TestConstructedDirectoryRetainsUnknown_AZU_V0_8901` (raw formatted paths, split separators, trailing-slash joins and backticks without format markers), `TestConstructedBasenameSpellings_AZU_V0_8901` (shared root/nested substitution, quote, format, concatenation and join matrix; known callers remain selected), `TestNormalizedJoinRetainsUnknown_AZU_V0_8901` (normalized literal components and unresolved suffixes retain UNKNOWN), `TestTraversalJoinRetainsUnknown_AZU_V0_8901` (an unresolved intermediate ancestor operand cancels an unrelated prefix; integrated selection retains UNKNOWN and known callers), `TestTraversalJoinSuffixBounds_AZU_V0_8901` (only a fixed non-shell suffix proves exclusion; shell suffixes, partial suffixes and cleaned components retain uncertainty, grouped or not, and a suffix on one branch of a conditional or boolean expression proves nothing), `TestGroupedSuffixJoinRetainsUnknown_AZU_V0_8901` (an undeclared `name + (".sh")` caller retains UNKNOWN), `TestNormalizedJoinBounds_AZU_V0_8901`, `TestNormalizedJoinSharedDirectoryRetainsUnknown_AZU_V0_8901` (two declared scripts share a directory; embedded Python with an escaped basename retains UNKNOWN and known caller selection), `TestDeclaredInboundRuntime_AZU_V0_8902`, `TestInboundRevisionRetainsOldCallers_AZU_V0_8904`, `TestInboundRevisionRejectsInvalidSnapshots_AZU_V0_8904` (invalid base with valid HEAD and invalid HEAD retain changed-declaration uncertainty) |
| AZU-V0-005 | V1-4688 | `TestAZUV0005HiddenPackages`; `TestHiddenPackageImportSetSelectsObserver_AFPV02202`; `TestAZUV0005KeepsTestlessReadersCallers`; `TestAZUV0005HiddenPackageSelection`; `TestAZUV0005RunnerExecutesHiddenPackages`; `TestHiddenUnrepresentableNames_AZU_V0_005`; `TestHiddenHelperScopes_AZU_V0_9920` (declared output); [whole-package evidence](../build-log/2026-11-04-azu-small-classes.md); [eight-merge replay](../build-log/2026-11-05-azu-small-wave-1105b.md) |
| AZU-V0-006 | V1-4689 | `TestAZUV0006CoverageGap`; `TestAZUV0006TestlessScopedReaderCallers`; `TestAZUV0006CoverageCountsOnlyRunCallers`; `TestAZUV0006ReaderCallerUncertainty`; `TestAZUV0006MultipleChangedPathsReaderUncertainty`; `TestAZUV0006MultipleChangedFilesDeclaredReader`; `TestAZUV0006ReaderOwnFilesFrontierInBatch`; `TestShortFragmentsKeepConcatenatedReaders_V1_1818`; `TestAZUV0006DeclaredReaderUnownedCaller`; `TestAZUV0006OwnedTestlessReaderCallers`; `TestAZUV0006OwnedSourceReaderCaller`; `TestAffectedUntestedGoPackageIsUnknownScope`; `TestUseCaseHostileChangeConsequence`; `TestAZUV0006And007CompactPlan`; [coverage and inbound uncertainty evidence](../build-log/2026-11-04-azu-small-classes.md); [self-review repair](../build-log/2026-11-05-azu-small-self-review-reader-callers.md) |
| AZU-V0-007 | V1-4690 | `TestAZUV0007RunnableConsumer`; `TestAZUV0007InvalidRun`; `TestAZUV0006And007CompactPlan`; `TestAZUV0007CLIExecutesDeclaredConsumers`; `TestAZUV0007CIRunnableBrowserConsumerRequired`; `TestAZUV0007IdenticalDeclaredCommandsShareInvocation`; `TestAZUV0007SharedConsumerRunnerRunsOnce`; [whole-package evidence](../build-log/2026-11-04-azu-small-classes.md); [eight-merge consumer replay](../build-log/2026-11-05-azu-small-wave-1105b.md) |
| AZU-V0-008 | V1-4691 | not yet |
| AZU-V0-009 | V1-4692 | replay tool, `TestSafetyRejectsUnsafeDrop_AZU_V0_8903`, `TestReplayIncompleteRemainsUnaccepted_AZU_V0_8903` and `TestChangedLineGitPaths_AZU_V0_8903` (Git-quoted Unicode/tab paths, space delimiters and deleted-file parent coordinates), `TestSourceDiffConfiguration_AZU_V0_8903` (canonical prefixes under local diff settings, source coordinates despite textconv), `TestInterHunkContextRejectsUnchangedMutants_AZU_V0_8903` (actual +/- records and canonical zero inter-hunk context; unchanged-line mutants remain NOT_PRODUCED on both sides), `TestSafetyHonorsSelectionTestFilter_AZU_V0_009` (an ownership-only `-run` selection does not catch other tests of its package; subtest, `-skip`, `-list`, repeated or invalid filters remain NOT_PRODUCED); historical full-suite/mutation qualification remains NOT_PRODUCED |
| AZU-V0-9700 | PR #98 review | `TestDriverMissingWrapperOperand_AZU_V0_9700` |
| AZU-V0-9701 | PR #98 review | `TestDriverImplicitGoPackage_AZU_V0_9701` |
| AZU-V0-9702 | PR #98 review | `TestDriverAlternativeMakeValues_AZU_V0_9702` |
| AZU-V0-9703 | PR #98 review | `TestDriverFlowSteps_AZU_V0_9703` |
| AZU-V0-9850 | PR #99 finding B | `TestDriverMakeFunctionsComputed_AZU_V0_9850`, `TestDriverMakeSortWords_AZU_V0_9850` |
| AZU-V0-9851 | PR #99 finding C | `TestDriverEnvChdirComputed_AZU_V0_9851` |
| AZU-V0-9852 | PR #99 finding D | `TestDriverCompositeFlowSteps_AZU_V0_9852` |
| AZU-V0-9853 | PR #99 finding E | `TestDriverUnresolvedLocalUses_AZU_V0_9853` |
| AFP-V0-3120 | V1-5000 | `TestFrontierSiteFaultInjection_AFP_V0_3120`, `TestFrontierSiteFileStatementKeepsClaim_AFP_V0_3120` |
| AFP-V0-3121 | V1-5000 | `TestFrontierSiteIncludeEffects_AFP_V0_3121`, `TestFrontierSiteNotdirCondition_AFP_V0_3121`, `TestFrontierSiteForeachLimitEffects_AFP_V0_3121`, `TestFrontierSiteFilesystemConditionEffects_AFP_V0_3121`, `TestFrontierSiteConditionEffectsKeepClaim_AFP_V0_3121`, `TestFrontierSiteStripCondition_AFP_V0_3121`, `TestFrontierSiteFilesystemCondition_AFP_V0_3121`, `TestFrontierSiteBoundedAlternativesKeepTheirEdges_AFP_V0_3121`, `TestDriverAlternativeMakeValues_AZU_V0_9702`, `TestDriverMakeFunctionsComputed_AZU_V0_9850` |
| AFP-V0-3122 | V1-5000 | `TestFrontierSiteBoundedSetExcludesPath_AFP_V0_3122`, `TestFrontierSiteShellTestParity_AFP_V0_3122`; 20-merge replay in `docs/build-log/2026-11-07-selfloor.md` |
| AFP-V0-3123 | V1-5000 | `TestFrontierSiteUnboundedKeepsClaim_AFP_V0_3123` |
| AFP-V0-3124 | V1-5000 | `TestDriverFrontierBoundParity_AFP_V0_3124` |
| AFP-V0-3125 | V1-5000 | `TestFrontierSiteImmediateValueKeepsCapturedEdge_AFP_V0_3125`, `TestFrontierSiteUnsupportedAssignmentKeepsClaim_AFP_V0_3125` |
| AFP-V0-3126 | V1-5000 | `TestFrontierSiteTargetSpecificSurvivesGlobal_AFP_V0_3126` |
| AFP-V0-3127 | V1-5000 | `TestFrontierSiteRepositoryShellWrapper_AFP_V0_3127`, `TestFrontierSiteParseTimeCommandKeepsClaim_AFP_V0_3127`, `TestFrontierSiteRootMakefileRunsNoReadTimeCommand_AFP_V0_3127` |
| AFP-V0-3128 | V1-5000 | `TestFrontierSiteCommandEnvironmentEffects_AFP_V0_3128`, `TestFrontierSiteGlobOperandsKeepClaim_AFP_V0_3128`, `TestFrontierSiteDirectoryGlobEdges_AFP_V0_3128`, `TestFrontierSiteLiteralExportEffects_AFP_V0_3128`, `TestFrontierSiteExportEffectsKeepClaim_AFP_V0_3128`, `TestFrontierSiteAttachedFileOptionKeepsClaim_AFP_V0_3128`, `TestFrontierSiteMakeOptionAllowlist_AFP_V0_3128`, `TestFrontierSiteGoEnvironmentKeepsClaim_AFP_V0_3128`, `TestFrontierSiteExtensionlessOperand_AFP_V0_3128`, `TestFrontierSiteDynamicTrap_AFP_V0_3128`, `TestFrontierSiteUnboundedOperandKeepsClaim_AFP_V0_3128`, `TestFrontierSiteBoundedOperandDropsClaim_AFP_V0_3128`, `TestFrontierSiteGoFlagAllowlist_AFP_V0_3128`, `TestFrontierSiteCommandOptionAllowlist_AFP_V0_3128`, `TestDriverMakeFunctionsComputed_AZU_V0_9850` |
| AFP-V0-9050 | V1-5000 | `TestFrontierSiteMakeDirSpelling_AFP_V0_9050`, `TestFrontierSiteMakeFunctionSpelling_AFP_V0_9050` |
| AFP-V0-9051 | V1-5000 | `TestFrontierSiteUnrecordableOperand_AFP_V0_9051`, `TestFrontierSiteRecordedOperandDropsClaim_AFP_V0_9051` |
| AZU-V0-9920 | fullplan charter | `TestFullplanReplay_AZU_V0_9920_9921` (PLANNED), `TestHiddenHelperUniverse_AZU_V0_9920`, `TestHiddenHelperModuleBoundary_AZU_V0_9920`, `TestHiddenHelperScopes_AZU_V0_9920`; Go/Python `.github/` discovery and `.corvint/runtime-edges.json` |
| AZU-V0-9921 | fullplan charter | `TestFullplanReplay_AZU_V0_9920_9921` (PLANNED); `non_source_paths.go`; declaration preserves reader selection |
| AZU-V0-9922 | fullplan charter | `TestFullplanReplay_AZU_V0_9920_9921` (PLANNED); `docs/build-log/2026-11-05-fullplan-selection-precision.md` |
| AZU-V0-9923 | PR #125 Greptile P1 (selbatch) | `TestNoPlanFullConfirmsHiddenGo_AZU_V0_9923` (real `go`, graph failed on an invalid declaration), `TestDeclaredGoSuiteSharesHiddenUniverse_AZU_V0_9923`, `TestAZUV09923CLINoPlanHiddenGoIsIncomplete`; `docs/build-log/2026-11-06-selbatch.md` |

| AZU-V0-9930 | Owner unbounded lane charter, 2026-11-05 | `TestComparisonOnlyLocators_AZU_V0_9930` (both scanners), `TestLocatorComparisonSelection_AZU_V0_9930`, `TestLocatorComparisonScanner_AZU_V0_9930` |
| AZU-V0-9931 | Owner unbounded lane charter, 2026-11-05 | Existing `tools/test-read-scopes-check`; four-merge replay in `docs/build-log/2026-11-05-unbounded-reader-precision.md` |
| AZU-V0-9950 | V1-4919; standing lane auto-accept, 2026-11-06 | `TestNonSourcePathsNamesOrderingFaults_AZU_V0_9950` (out-of-order, duplicate and undeclarable entries) |

The comparison proof's failure mode is an escaped path mistaken for a pure guard. The binding
and scanner regressions retain real reads, aliases, closures, shadowing and parse failures.
Acceptance requires whole-package tests of both scanner packages and the existing read-scope
check. Rollback removes the six added scope entries and the comparison proof/call sites, restoring
the conservative rule (d) floor. Independent review and host-wide qualification remain separate.
