# Affected Zero Unknown V0

Owner: Russell Lewis
Date: 2026-11-04
Requirement prefix: `AZU-V0`
Intent status: experimental; the owner directed the goal ("figure out how to get the unknown to 0
safely", owner chat, 2026-11-04), and the orchestrator made the design choices under the owner's
delegation of 2026-11-02
Delivery status: experimental
Authoritative inputs: `docs/specs/affected-plan-v0.md` (parent contract: wire, authority, AFP-V0-511
frontier bearing, AFP-V0-560 terminal rule, AFP-V0-601/602 data readers),
`docs/specs/affected-plan-selection-v0.md` (AFP-V0-2405 driver-input hint, AFP-V0-2440 fixture-only
claim), `docs/specs/test-selection-precision-v0.md` (TSP-V0-005 shell foreign callers, TSP-V0-942
frontier bearers), `internal/liveverify/affected`, `AGENTS.md` invariants 2, 3 and 8. The parent
spec is at its OCM obligation limit, so these requirements take a new prefix.

## Agent digest
- Claim: every `corvint affected` plan reaches `unknown: []` through sound selections or stated proofs, never by dropping a reached unit.
- Status: experimental / experimental. Local class fixes have evidence below; overall acceptance is pending.
- Exists: `frontierUnknowns`, `bearsOnUnowned`, `participants`, `unownedUnknown` and `driverInput` in `internal/liveverify/affected/select.go`; `FrontierReach` in `unit.go`; `.corvint/shell-foreign-callers.json`, `.corvint/runtime-readers.json` and `.corvint/test-binary-consumers.json`.
- Blocked on: nothing; each requirement lands on its own.
- Read next: User and current state, then the requirement you are implementing.

## User and current state

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

- `AZU-V0-001`: Driver claimant. A built-in claimant SHALL own every driver input that
  `driverInput` recognises (make files at any depth, `.github/workflows/**`, `.github/actions/**`),
  so such a path is never `UNOWNED_DIRTY_PATH` and never triggers the AFP-V0-511 cascade. Its units
  are make targets and workflow jobs. Edges are the repository paths a recipe or step names
  literally: a script, a `go test`/`go run` package pattern, or another make target. A dirty driver
  file SHALL select the targets or jobs whose recipe or step text a diff hunk touches. A hunk
  outside every recipe or job (a variable, an `include`, `on:`, top-level `env:` or `defaults:`)
  SHALL select every target or job of that file. A recipe or step that invokes a computed name
  carries a `driver:computed-invocation` frontier entry at that target or job; AZU-V0-003 closes it.
  Selections carry kind `DRIVER_TARGET` and the argv `make <target>`, or `CI_JOB` and the workflow
  and job IDs. A consumer that cannot run a `CI_JOB` reports it as left to CI, never as unknown.
  Readers of a driver file by path literal, declared read scope or unbounded read stay selected as
  before. Supersedes the AFP-V0-2405 hint for claimed paths.
- `AZU-V0-002`: Ownership completeness gate. `corvint affected --ownership-audit` SHALL list every
  tracked path at a commit that no plugin, the driver claimant or a project declaration claims.
  Declarations are documentation paths, runtime readers, task state, and a new
  `.corvint/non-source-paths.json` for evidence output such as `.agent-evidence/`. A maintained test
  SHALL fail while that list is non-empty at `HEAD`, so a merged tree has no unowned tracked path
  and `UNOWNED_DIRTY_PATH` can arise only for an uncommitted new path. The audit is read-only (AGENTS
  invariant 4).
- `AZU-V0-003`: Outbound site closure. A plugin SHALL classify each frontier reason as `OUTBOUND`
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
- `AZU-V0-004`: Inbound caller closure. An `INBOUND` reason (a hidden caller into the site unit,
  such as `shell:foreign-invocation-unmodelled`, `shell:foreign-callers-changed` or
  `typescript:e2e-runtime-dependency`) SHALL close when the project declares the site's callers.
  Declarations are `.corvint/shell-foreign-callers.json` (TSP-V0-005) and driver edges
  (AZU-V0-001). The plan then selects the declared callers' units and drops the entry. A maintained
  check SHALL scan every tracked file for a literal reference to each declared-callee path and fail
  when a referencing file is neither declared nor an edge the graph already models. A file that
  builds a callee path from a computed name under a declared callee directory SHALL also fail the
  check, so the declaration stays complete under project authority (AGENTS invariant 3). An inbound
  entry whose site has no declaration stays unknown.
- `AZU-V0-005`: Dot-directory Go packages. The Go plugin SHALL index tracked Go packages of the
  main module that lie below a directory whose name starts with `.` or `_`, which `./...` skips (for
  example `.github/testconfine` and `.github/testconfine/cmd`). Such a file is then owned source,
  not `UNINDEXED_SOURCE_PATH`. Hidden-directory observation SHALL use the same bounded,
  metadata-excluding source walk for disk and immutable sources; `testdata` remains fixture data.
  Indexing a previously unowned file SHALL retain testless literal/glob readers and their callers.
- `AZU-V0-006`: Uncovered, not unknown. A changed unit with no test (`NO_SELECTABLE_TEST`) SHALL
  first select its declared or modelled callers (AZU-V0-001, AZU-V0-004). When no selectable test
  anywhere can observe it, the plan SHALL record it in a new `uncovered` list, not in `unknown`.
  This is a coverage gap, not a selection gap: no test exists whose outcome the change can alter. A
  unit with any undeclared `INBOUND` entry stays unknown. The optional `plan.uncovered` member
  is omitted when empty; otherwise it is a unit-ID-ordered list of `{reason, detail}` with
  `reason="NO_SELECTABLE_TEST"` and the unit ID as `detail`. Both full and compact plan profiles
  preserve it; the by-path profile preserves it on each path entry, and text emits an
  `uncovered REASON UNIT_ID` line. Declared callers with no owning unit remain unknown. Literal, scoped, glob and
  unbounded reader tests count as observers and remain selected. Embedding foreign-language
  source bytes does not establish execution coverage (AFP-V0-811); that uncertainty stays unknown.
  The frozen `application-flow-impact/1` projection retains uncovered coverage gaps in its
  legacy `graph.unknown` list and `UNKNOWN` scope; its wire and goldens do not change.
- `AZU-V0-007`: Declared binary consumers run. An entry of `.corvint/test-binary-consumers.json`
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
- `AZU-V0-008`: Package-level runner fallback. A frontier reason that hides only which tests run
  inside a known package or project SHALL select that package's whole test command. Examples are
  `typescript:test-runner-unknown`, `typescript:runtime-flags-unresolved`,
  `dotnet:test-runner-filter-unresolved` and `dotnet:dynamic-test-discovery`. The plugin declares
  the reason `PACKAGE_RUNNABLE`. Only a reason whose package has no runnable command stays unknown,
  as `NO_RUNNER`.
- `AZU-V0-009`: Zero-unknown acceptance. The replay `script/affected-zero-unknown-replay.sh`
  SHALL run `corvint affected --base M^1` for each merge in a frozen list. The list starts with the
  eight merges above and adds every merge since. It SHALL report per merge the unknown count by
  reason, the selection size and its wall-clock cost. AZU-V0 is met when every plan in the list has
  `unknown: []` and the safety check finds no miss: on each merge, every test whose outcome differs
  between the parent and the merge in a full-suite run, or that a mutation on a changed line kills,
  is in the selection.

## Non-goals

- No requirement here may drop a reached, reading, executing, importing or embedding unit. Every
  rule turns an unknown into a selection or a stated proof.
- This is not a cost optimisation. AZU-V0-003 can widen a selection (all site units of a bearing
  reason). Narrowing that width is later work under its own requirements, once zero unknowns is
  measured.
- No embeddings, network or learned ranking, and no change to the authority order.

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

## Acceptance evidence

Each requirement's lane retains: unit tests for the rule; the AZU-V0-009 replay before and after,
showing which unknown entries it removed and the selection growth; and for AZU-V0-003 and AZU-V0-004
the soundness fixtures above. AZU-V0 as a whole is accepted when the replay shows `unknown: []` on
every listed merge with zero safety misses.

## Rollback

Each requirement only turns an unknown into a selection, behind its own code path. Reverting its
commit restores the earlier `UNKNOWN` plan, which consumers already treat as a wide or full run.
Declarations added for AZU-V0-002, AZU-V0-004 and AZU-V0-007 are inert without their readers.

## Traceability

| Requirement | Tickets | Evidence |
|---|---|---|
| AZU-V0-001 | V1-4684 | not yet |
| AZU-V0-002 | V1-4685; V1-4659, V1-3480 | not yet |
| AZU-V0-003 | V1-4686 | not yet |
| AZU-V0-004 | V1-4687; V1-4392, V1-4535 | not yet |
| AZU-V0-005 | V1-4688 | `TestAZUV0005HiddenPackages`; `TestAZUV0005KeepsTestlessReadersCallers`; [merge replay and whole-package evidence](../build-log/2026-11-04-azu-small-classes.md) |
| AZU-V0-006 | V1-4689 | `TestAZUV0006CoverageGap`; `TestAZUV0006And007CompactPlan`; [coverage and inbound uncertainty evidence](../build-log/2026-11-04-azu-small-classes.md) |
| AZU-V0-007 | V1-4690 | `TestAZUV0007RunnableConsumer`; `TestAZUV0007InvalidRun`; `TestAZUV0006And007CompactPlan`; [consumer replay and whole-package evidence](../build-log/2026-11-04-azu-small-classes.md) |
| AZU-V0-008 | V1-4691 | not yet |
| AZU-V0-009 | V1-4692 | not yet |
