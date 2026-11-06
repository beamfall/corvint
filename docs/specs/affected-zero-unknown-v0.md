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

## Agent digest
- Claim: every `corvint affected` plan reaches `unknown: []` through sound selections or stated proofs, never by dropping a reached unit.
- Status: experimental (owner goal, owner chat 2026-11-04; orchestrator design under the 2026-11-02 delegation); delivery experimental.
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

- **AZU-V0-001: Driver claimant.** A built-in claimant SHALL own every driver input that
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
  Driver execution edges SHALL survive Go test-only containment (AFP-V0-036), selecting
  the target/job and its callers when only the invoked package's tests change; ordinary Go
  importers remain contained. Go's `-C=DIR` form SHALL resolve like `-C DIR`. Shell globs in
  Go package operands and unsupported inline interpreter programs SHALL resolve their
  dependencies or retain the invocation site as computed under AZU-V0-003.
- **AZU-V0-002: Ownership completeness gate.** `corvint affected --ownership-audit` SHALL list every
  tracked path at a commit that no plugin, the driver claimant or a project declaration claims.
  Declarations are documentation paths, runtime readers, task state, and a new
  `.corvint/non-source-paths.json` for evidence output such as `.agent-evidence/`. A maintained test
  SHALL fail while that list is non-empty at `HEAD`, so a merged tree has no unowned tracked path
  and `UNOWNED_DIRTY_PATH` can arise only for an uncommitted new path. The audit is read-only (AGENTS
  invariant 4).
- **AZU-V0-003: Outbound site closure.** A plugin SHALL classify each frontier reason as `OUTBOUND`
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
- **AZU-V0-004: Inbound caller closure.** An `INBOUND` reason (a hidden caller into the site unit,
  such as `shell:foreign-invocation-unmodelled`, `shell:foreign-callers-changed` or
  `typescript:e2e-runtime-dependency`) SHALL close when the project declares the site's callers.
  Declarations are `.corvint/shell-foreign-callers.json` (TSP-V0-005) and driver edges
  (AZU-V0-001). The plan then selects the declared callers' units and drops the entry. A maintained
  check SHALL scan every tracked file for a literal reference to each declared-callee path and fail
  when a referencing file is neither declared nor an edge the graph already models. A file that
  builds a callee path from a computed name under a declared callee directory SHALL also fail the
  check, so the declaration stays complete under project authority (AGENTS invariant 3). An inbound
  entry whose site has no declaration stays unknown.
- **AZU-V0-005: Dot-directory Go packages.** The Go plugin SHALL index tracked Go packages of the
  main module that lie below a directory whose name starts with `.` or `_`, which `./...` skips (for
  example `.github/testconfine` and `.github/testconfine/cmd`). Such a file is then owned source,
  not `UNINDEXED_SOURCE_PATH`.
- **AZU-V0-006: Uncovered, not unknown.** A changed unit with no test (`NO_SELECTABLE_TEST`) SHALL
  first select its declared or modelled callers (AZU-V0-001, AZU-V0-004). When no selectable test
  anywhere can observe it, the plan SHALL record it in a new `uncovered` list, not in `unknown`.
  This is a coverage gap, not a selection gap: no test exists whose outcome the change can alter. A
  unit with any undeclared `INBOUND` entry stays unknown.
- **AZU-V0-007: Declared binary consumers run.** An entry of `.corvint/test-binary-consumers.json`
  MAY carry `run`, the argv that runs it from the repository root. When the change reaches the
  consumed binary's build, an entry with `run` SHALL be selected with kind `DECLARED_CONSUMER` and
  that argv, instead of `UNSELECTED_BINARY_CONSUMER`. An entry without `run` keeps today's unknown.
- **AZU-V0-008: Package-level runner fallback.** A frontier reason that hides only which tests run
  inside a known package or project SHALL select that package's whole test command. Examples are
  `typescript:test-runner-unknown`, `typescript:runtime-flags-unresolved`,
  `dotnet:test-runner-filter-unresolved` and `dotnet:dynamic-test-discovery`. The plugin declares
  the reason `PACKAGE_RUNNABLE`. Only a reason whose package has no runnable command stays unknown,
  as `NO_RUNNER`.
- **AZU-V0-009: Zero-unknown acceptance.** The replay `script/affected-zero-unknown-replay.sh`
  SHALL run `corvint affected --base M^1` for each merge in a frozen list. The list starts with the
  eight merges above and adds every merge since. It SHALL report per merge the unknown count by
  reason, the selection size and its wall-clock cost. AZU-V0 is met when every plan in the list has
  `unknown: []` and the safety check finds no miss: on each merge, every test whose outcome differs
  between the parent and the merge in a full-suite run, or that a mutation on a changed line kills,
  is in the selection.
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

## Rollback

Each requirement only turns an unknown into a selection, behind its own code path. Reverting its
commit restores the earlier `UNKNOWN` plan, which consumers already treat as a wide or full run.
Declarations added for AZU-V0-002, AZU-V0-004 and AZU-V0-007 are inert without their readers.

## Traceability

| Requirement | Tickets | Evidence |
|---|---|---|
| AZU-V0-001 | V1-4684 | `Driver` claimant in `internal/liveverify/affected/driver*.go`; `driver_test.go` (per-hunk, remainder, unparsable, removed-block, deleted-file and `FRONTIER_SITE` cases); replay `--base 39c031980` at `8e2c13aa9`: unknown 92 to 89, `UNOWNED_DIRTY_PATH` 5 to 2, selected 266 to 488, no selection dropped (`docs/build-log/2026-11-04-azu-driver-claimant.md`) |
| AZU-V0-002 | V1-4685; V1-4659, V1-3480 | partial: `affected --ownership-audit` (`cmd/corvint/affected_ownership_audit.go`, `OwnershipAudit` in `ownership_audit.go`), `.corvint/non-source-paths.json` (`non_source_paths.go`); `TestOwnershipAuditListsUnclaimedPaths_AZU_V0_002`, `TestNonSourcePathsRefusesInvalidDeclarations_AZU_V0_002`, `TestAffectedOwnershipAuditListsUnownedReadOnly_AZU_V0_002`; the maintained test `TestOwnershipAuditRatchet_AZU_V0_002` is a ratchet over a 1694-path baseline, not yet the empty list the requirement names |
| AZU-V0-9704 | PR #98 repair round 2 | `TestDriverUnknownIsNotLanguage_AZU_V0_9704` in `internal/sessiontest`; unchanged `make doc-gate/selection-baseline-check` corpus |
| AZU-V0-003 | V1-4686 | not yet |
| AZU-V0-004 | V1-4687; V1-4392, V1-4535 | not yet |
| AZU-V0-005 | V1-4688 | not yet |
| AZU-V0-006 | V1-4689 | not yet |
| AZU-V0-007 | V1-4690 | not yet |
| AZU-V0-008 | V1-4691 | not yet |
| AZU-V0-009 | V1-4692 | not yet |
| AZU-V0-9700 | PR #98 review | `TestDriverMissingWrapperOperand_AZU_V0_9700` |
| AZU-V0-9701 | PR #98 review | `TestDriverImplicitGoPackage_AZU_V0_9701` |
| AZU-V0-9702 | PR #98 review | `TestDriverAlternativeMakeValues_AZU_V0_9702` |
| AZU-V0-9703 | PR #98 review | `TestDriverFlowSteps_AZU_V0_9703` |
| AZU-V0-9850 | PR #99 finding B | `TestDriverMakeFunctionsComputed_AZU_V0_9850`, `TestDriverMakeSortWords_AZU_V0_9850` |
| AZU-V0-9851 | PR #99 finding C | `TestDriverEnvChdirComputed_AZU_V0_9851` |
| AZU-V0-9852 | PR #99 finding D | `TestDriverCompositeFlowSteps_AZU_V0_9852` |
| AZU-V0-9853 | PR #99 finding E | `TestDriverUnresolvedLocalUses_AZU_V0_9853` |
