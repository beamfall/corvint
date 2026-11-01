# Context Evolution Program V0

Owner: Russell Lewis
Frozen: 2026-08-23
Intent status: proposed
Delivery status: experimental
Document kind: hypothesis catalog plus one accepting slice. The five hypotheses are prose intent
only, with no numbered clauses, so OCM must abstain on them rather than invent coverage; decision
0179 (2026-09-13) holds them prose-only permanently, and a hypothesis gains `PREFIX-NNN` clauses
only via a separate accepting slice's own requirements, never by relabelling its prose. The one
numbered set here, `CEP-V0-001`..`010`, belongs to the external evaluation slice accepted by
decision 0372 (2026-09-23), and governs only the `tools/retrieval-bench` and `tools/cw-trial`
evaluation harnesses.
Authoritative inputs: `docs/PRODUCT.md`, `docs/TECHNICAL-BRAIN.md`,
`docs/specs/agent-harness-integration-v0.md`,
`docs/specs/proof-carrying-context-optimization-v0.md`, `docs/DOGFOOD.md`

## Agent digest
- Claim: Five independent context-evolution hypotheses require frozen held-out evidence before promotion.
- Status: proposed/experimental
- Exists: `internal/gokernel` local evidence for authority closure and snapshot capsules; external evaluation slice `CEP-V0-001`..`010` in `tools/retrieval-bench` and `tools/cw-trial`.
- Blocked on: frozen held-out outcome trials.
- Read next: `proof-carrying-context-optimization-v0.md` and `agent-harness-integration-v0.md`.

Strategic frame: this program is an initial research slice of the Corvint Evidence Operating System
described in the internal research catalogue (not part of the public tree). It does not claim that retrieval,
memory, or compaction alone is a category breakthrough.

## Goal

Corvint will test five independent advances that can make agent context more correct, portable,
survivable, efficient, and self-invalidating. Each advance has a simpler baseline, a frozen
observable gate, and a kill criterion. Shared implementation does not merge their claims: one may
pass while another fails.

No item is a breakthrough because it is novel-sounding. It becomes one only after its named held-out
gate passes with reproducible artifacts.

## 1. Authority Closure Compiler

**Hypothesis.** Before semantic relevance, Corvint can mechanically close a task over the repository's
governing instructions and pinned references. Product vocabulary cannot hijack an operational task,
and an agent receives the rules for how to work before suggestions about what code resembles the
prompt.

**Simpler baseline.** Rank every indexed feature, document, and symbol only by task vocabulary.

**Current evidence.** The first Beamfall prompt incorrectly led with product features named
`work-queues` and `queue-next-up`. The repaired general project-operations intent returns binding
`AGENTS.md` lines plus `make orient`, context-packet, roadmap, and workflow-gate references as the
critical result. The focused query/harness suite passes; this is one-repository dogfood, not broad
generalization.

**Gate.** Freeze 100 operational prompts across ten repositories, with independent labels for every
governing instruction and required tool. Require 100% critical authority recall, zero non-authority
result above a governing instruction, at most 10% irrelevant authority references, zero secret or
untracked-body inclusion, and p95 under 500 ms warm.

**Kill or narrow.** If authority recall is below 98% or more than 5% of product tasks are incorrectly
routed as operations, restrict this compiler to explicit caller intent rather than adding heuristics.

## 2. Portable Snapshot Context Capsule

**Hypothesis.** One content-addressed, transport-neutral context capsule can be consumed by different
agent harnesses without adapter-owned knowledge or freshness drift. Repository envelope, authority,
evidence, exclusions, and uncertainty all refer to one Corvint snapshot.

**Simpler baseline.** Each host plugin performs its own repository search and emits an unverified
prompt fragment.

**Current evidence.** The shared harness core and four preview adapters normalize 21 common native
event paths to one checked-in protocol golden. Dogfood caught and repaired a double-build freshness
split. Repository and nested context now reuse one Corvint snapshot, and clean oversized Git blobs are
excluded without being called dirty. Every host tuple remains `FALLBACK`; black-box host promotion is
`NOT_RUN`.

**Gate.** For Codex, Claude Code, Gemini CLI, and OpenCode, run 50 common logical interactions each
across clean, tracked-dirty, untracked, commit-race, oversized, malformed, timeout, and interrupted
fixtures. Require byte-identical normalized requests and core receipts for common events, exact
single-snapshot identity, zero adapter-owned semantic fields, zero descendant leakage, complete
install/uninstall, and zero false `FULL` claims.

**Kill or narrow.** Remove any native adapter whose translation requires a second graph, host-only
authority semantics, or lossy reinterpretation; retain the CLI/MCP capsule as the portable boundary.

## 3. Proof-Carrying Compaction Survival

**Hypothesis.** Context compaction can become observable evidence continuity rather than invisible
summary loss. A survival manifest can prove which Corvint handles were retained, omitted, stale,
unresolved, or rehydrated without reading the transcript or claiming model attention.

**Simpler baseline.** Trust native compaction and re-search the repository after an obvious miss.

**Current evidence.** Codex exposes a compact-start lifecycle, and Corvint now rehydrates exact tracked
dirty-path impact while digesting untracked gaps. The Claude Code plugin additionally registers
`PreCompact` and `PostCompact` (decision 0340, AHI-026 to AHI-030): before compaction it pins the
compact packet's revision and tracked dirty paths into the compactor's instructions, and after
compaction it verifies the pin the summary preserved against the object store and names every
non-rehydratable path to the user. That is a per-cycle pin verdict, not a survival manifest: the
verdict never reaches model context, the hook payloads were read from the installed 2.1.267 host
only, and no live compaction cycle or multi-cycle trial has run.

**Gate.** Run at least three forced compaction cycles in each of 30 held-out tasks across Corvint,
Beamfall, and an unrelated repository. Require zero critical-selector loss, 100% classification of
pre-cycle handles into disjoint survival states, byte-verifiable manifests, no prompt/transcript
persistence, no more than one automatic widening per cycle, and at least 30% lower recovery latency
than native compaction plus manual re-search.

**Kill or narrow.** If no real loss is detected and recovery time does not improve, retain exact
dirty-path rehydration and remove the survival protocol.

## 4. Counterfactual Minimum Witness Compiler

**Hypothesis.** Offline hierarchical delta debugging over typed evidence handles can learn a smaller
task-class packet that preserves success, while runtime serving stays deterministic and model-free.

**Simpler baselines.** Complete Corvint packet, lexical top-k at equal bytes, and native host
compaction.

**Current evidence.** The PCCO contract freezes the experiment, but attested outcome observation and
the replay corpus are absent. No savings claim exists.

**Gate.** Use a preregistered train/validation/held-out split of at least 30 eligible tasks across
three repositories. Require non-inferior sealed correctness, zero treatment-only critical misses,
at least 25% lower median and 20% lower p75 complete cache-aware cost per solved task than the
strongest baseline, no p95 latency regression above 10%, and at least 80% completion without broad
repository rescan.

**Kill or narrow.** Kill learned minimization below 10% savings, on any treatment-only critical miss,
or if retries/broad searches rise. Keep deterministic receipt compilation.

## 5. Live Context Frontier

**Hypothesis.** Context can have liveness, not just relevance. Every supplied handle can be tied to
an open obligation, active change, governing authority, or explicit recovery dependency; when that
reason closes or drifts, Corvint invalidates the handle instead of propagating stale memory.

**Simpler baseline.** Append retrieved facts and summaries to session or shared memory until a size
limit forces undifferentiated compaction.

**Proposed primitive.** A local frontier maps each handle to `LIVE|STALE|CONFLICTED|CLOSED|UNKNOWN`,
its exact keeping reason, scope, revision, dependencies, expiry/invalidation trigger, and reacquisition
operation. It composes existing Corvint receipt, CEM/OCM, drift, and Unknown Frontier identities; it
does not create a truth graph or allow learned profiles to close obligations.

**Gate.** Freeze 30 five-session task sequences. Between sessions, mutate governing instructions,
move evidence, introduce a contradiction, close an obligation, and leave one dependency unchanged.
Require 100% invalidation of changed authority and stale critical handles before serving, 100%
visibility of injected contradictions, zero closed handle resurrected without a new receipt, exact
reacquisition for unchanged/uniquely relocated evidence, at least 30% lower cumulative context bytes
than append-only memory, non-inferior correctness, and p95 refresh under one second warm.

**Kill or narrow.** If liveness cannot beat revision-only invalidation without a false stale/closed
classification, retain revision-pinned receipts and expose the frontier only as an audit report.

## Program sequencing

1. Stabilize and externally test Authority Closure and Portable Snapshot Capsules.
2. Add attested harness observation without source bodies or transcript capture.
3. Trial Compaction Survival before learned minimization.
4. Run Counterfactual Minimum Witness only on eligible observed tasks.
5. Compose successful survival/omission evidence into the Live Context Frontier.

Promotion is per item. Passing item 1 or 2 does not authorize learning; passing item 4 does not make
a learned profile project authority; passing item 5 does not establish a shared memory service.

## Research leads, not product evidence

- OpenAI reports large benchmark gains from retained reasoning plus compaction in its harness:
  `https://developers.openai.com/blog/codex-as-a-platform`.
- ACE studies incremental outcome-evaluated context evolution: `https://arxiv.org/abs/2510.04618`.
- SWE-Pruner studies task-aware structural pruning: `https://arxiv.org/abs/2601.16746`.
- Governed Shared Memory defines stale propagation, contradiction persistence, leakage, and
  provenance collapse risks: `https://arxiv.org/abs/2606.24535`.
- Code Compression Bench and CompactBench motivate cache-aware solved-task cost and multi-cycle
  compaction evaluation: `https://github.com/daseinlabs/code-compression-bench` and
  `https://github.com/compactbench/compactbench`.

## External evaluation slice

Decision 0372 accepts this slice as the separate accepting slice decision 0179 requires. It governs
two local harnesses that score Corvint packets against outside evidence: the ContextBench
(`https://arxiv.org/abs/2602.05892`, `EuniAI/ContextBench`) row adapter in `tools/retrieval-bench`,
and an already-fixed abstention control in the confidently-wrong trial `tools/cw-trial`
(`https://arxiv.org/abs/2603.25764`, `docs/specs/confidently-wrong-trial-v0.md`). It adds no clause
to the five hypotheses above and no `corvint` command.

## Requirements

- `CEP-V0-001`: ContextBench rows. `tools/retrieval-bench --samples` MUST accept a ContextBench row
  exported one JSON object per line with the dataset's own column names, recognised by an
  `instance_id` member, and map it to exactly one sample: `instance_id` is the ID, `repo` and
  `base_commit` pin the snapshot, `problem_statement` is the verbatim query, the task type is
  `contextbench`, and the distinct files of the JSON-encoded `gold_context` spans are the gold.
  Gold paths MUST be normalised exactly as ContextBench's `_normalize_rel_path` does, including its
  leading `.`/`/` strip. A row lacking `instance_id`, `repo`, `base_commit` or `problem_statement`, with
  a `gold_context` that is not a JSON span list, or with a span whose start line is below 1, whose end
  precedes its start, whose end exceeds 10,000,000, or whose line numbers are not integers MUST be
  refused with its line number; line numbers stay integers end to end. The tool MUST NOT read Parquet or the
  network.
- `CEP-V0-002`: ContextBench metrics. For every error-free arm answer on a ContextBench sample, the
  report MUST add `cb_file_coverage`, `cb_file_precision`, `cb_line_coverage` and `cb_line_precision`
  beside the bench's own metrics, with ContextBench's definitions: coverage is shared over gold,
  precision is shared over predicted, an empty gold has coverage 1 and an empty prediction precision 1.
  The prediction is the ranking cut at the limit. Line intervals merge when they overlap or touch. A
  ranked file predicts every line it has in the snapshot copy, because a packet names files, not
  spans; a ranked path absent from the copy predicts no line. Symbol and byte-span granularities are
  not measured and MUST NOT be reported.
- `CEP-V0-960`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-2761)
  Abstention scoring. ContextBench's vacuous precision of 1 for an empty prediction makes
  `cb_file_precision` mix abstention with ranking quality, so for every error-free arm answer on a
  ContextBench sample the report MUST also add `cb_file_abstained` (1 when the cut prediction is
  empty, else 0; its mean is the abstention rate), `cb_file_precision_abst0` (the file precision
  with an empty prediction scored 0), and, only when the prediction is non-empty,
  `cb_file_precision_answered`. For every other arm on a sample where both it and the `bm25:ident`
  baseline answered without error and it predicted `L > 0` files, the report MUST add
  `cb_file_baseline_precision_length_matched`: the baseline's file precision over its ranking cut at
  `L`, so a list-length difference is not read as a ranking difference. Each is averaged per arm in
  every summary group, including each `fold:` split, over the samples that carry it. A
  preregistered ContextBench precision criterion written after 2026-10-31 MUST cite
  `cb_file_precision_answered` with the abstention rate, or the length-matched comparison, never
  `cb_file_precision` alone. Re-running the scorer on the retained V1-2716 rows needs those rows,
  which are held off this host, so that reproduction is recorded `NOT_RUN` until it is run where
  they are.
- `CEP-V0-003`: Pinned offline runs. Every external run MUST be local and offline: rows and
  repository snapshots come only from explicit local `--samples` and `--snapshot`/`--corpus` inputs,
  and the report carries the sha256 of the samples file as `samples_sha256`. A run whose subset or
  snapshots are not locally available MUST be recorded as `NOT_RUN` with the reason, never
  approximated from another subset.
- `CEP-V0-004`: Already-fixed control. A `tools/cw-trial` task MAY carry `control:
  "already-fixed"`: its issue is already resolved at the pinned revision, so nothing is left to
  change. Such a task MUST carry no gold, and any other `control` value MUST be refused. Every valid
  claim on it is judged against an empty gold per kind, so each claim is `FALSE`, never `UNJUDGED`.
- `CEP-V0-005`: Control scoring. On an already-fixed control each arm record MUST carry
  `control_failed`, the agent's own failure: 1 exactly when the agent made a `certain` claim, so
  every arm is compared on the same measure. Whether a produced corvint packet abstained is recorded
  apart as `packet_abstained`, only for a produced packet, and never sets `control_failed`; a packet
  abstains on `NO_CANDIDATES` or `OUT_OF_SCOPE`, no result row, or an `unsupported-conjunction`
  answerability verdict, and a packet that does not parse did not abstain. Control tasks MUST stay
  out of every other arm aggregate (task and error counts, success, abstention, retrieval rates,
  F1, tool calls, wall time, tokens) and are reported only in the arm's `already_fixed` block (tasks,
  errors, control failures, packets observed, packets abstained), present only when the arm ran a
  control, so a report without controls keeps its bytes.
- `CEP-V0-006`: Evidence, not promotion. Results of this slice are measured evidence that a later
  promotion decision may cite, never a promotion: no result changes a spec's intent or delivery
  status, a ranking default, or a product claim. Each recorded run MUST name in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/BUILD-LOG.md`
  the exact subset, its samples-file sha256, the metric definitions and every `NOT_RUN` label, and
  report files and bench data MUST NOT be committed; only small synthetic fixtures are.
- `CEP-V0-007`: (accepted 2026-10-25, standing auto-accept; V1-2716) Whole-packet ranking. For the
  `corvint` arm, `retrieval-bench` MUST rank the first evidence path of each confident `results[]` row
  in packet order and then the ids of the `context.widening.results[]` rows (`GPK-V0-090`) in their
  order, deduplicated, truncated at the row limit, so file recall and precision are measured over the
  whole bounded packet. Abstention stays the confident tier's `state`: a `widening` member never turns
  `OUT_OF_SCOPE` into an answer. A packet without the member ranks exactly as before, so recorded
  reports keep their bytes. Reports MUST state which tier a recall or precision figure describes:
  a widened packet's arm record carries `ranking_tier` `confident+widening` and `confident_ranked`,
  the length of the confident prefix, so the confident-tier figure can be recomputed from the
  record; a packet without the member carries neither.
  Failure modes: ranking widening ids first would hide the confident tier's precision; counting a
  widened abstention as an answer would reward the lower-confidence tier. Rollback: drop the
  `Widening` field of `packet` and the second loop in `parsePacket`.
- `CEP-V0-008`: (accepted 2026-10-28, standing auto-accept; V1-2945) Equal-budget text windows. A
  ranked path list says nothing about how much text an agent must read, so packet bytes and a
  baseline's whole files are not comparable context. Under `--budget-bytes N` (1..16777216, off by
  default, refused with `--summarize`) every arm, Corvint packets and lexical baselines alike, is
  consumed through the same window, the first `--window-bytes W` (default 4096) of each ranked
  regular file in the pinned snapshot copy, read through `os.Root`, taken whole in ranking order
  until the next window would exceed `N`; a ranked path that is not a regular file there supplies
  nothing and costs nothing, and an errored or abstained arm (including an `OUT_OF_SCOPE` packet that
  still ranks widening paths) delivers nothing. Each arm records
  `budget_bytes_used` and `budget_files`, and on positive samples `budget_recall` and `budget_hit`;
  `budget_recall` is a paired metric. The report and the registration carry a `budget` block naming
  the unit, both sizes, the window rule and `tokens` = `NOT_MEASURED`, since the bench has no shared
  tokenizer. Without the flag no budget key is written, so recorded reports keep their bytes.
  `--summarize` refuses stored reports whose `budget` blocks differ (one with and one without a
  block included), so `budget_recall` is paired only at one budget, and carries the shared block into
  the merged report and its registration.
  Failure modes: comparing packet bytes against whole files rewards whichever arm is measured more
  cheaply; a window that escaped the snapshot would score text no arm could read. Rollback: remove
  `budget.go`'s `budgetMetrics` and the two flags.
- `CEP-V0-009`: (accepted 2026-10-28, standing auto-accept; V1-2945) Outcome separation. Every
  summary group carries `outcomes[arm]` with `n`, `answered`, `abstained`, `errors`, `coverage`,
  `abstention_rate`, `error_rate` (null over an empty group) and `answered_precision@k`, the mean
  `precision@k` over the positive samples the arm answered without error (null when there are
  none), so an abstention or an error never moves it. The historical means, which score an error as
  zero and an abstention by its empty ranking, are kept unchanged beside it. Failure mode: a fusion
  or abstaining arm whose empty answers raise a blended precision. Rollback: drop the `outcomes` key.
- `CEP-V0-010`: (accepted 2026-10-28, standing auto-accept; V1-2945) Experiment kinds. Every report,
  and every `--summarize` rescore, carries `experiment` with `kind` = `retrieval`, the only kind this
  tool produces, and `oracle_context`, `tool_adoption` and `downstream_grader` each `NOT_RUN`.
  Oracle-context utility and voluntary tool adoption are separate experiments with their own arms; a
  SWE-bench Pro outcome is graded only by the official pinned grader, never by a reimplementation in
  this tree, and no official-grader run is part of this slice (recorded `NOT_RUN`, V1-2945).
  Rollback: drop the `experiment` key.

## Non-goals (external evaluation slice)

- No new `corvint` verb, flag or ranking change from this slice; the harnesses are tools only. The
  only packet member the bench reads beyond `results[]` is the `widening` member that `GPK-V0-090`
  defines (`CEP-V0-007`); the bench never ranks it ahead of the confident tier.
- No ContextBench agent harness, repository cloning, Parquet reader, tree-sitter symbol extraction,
  or byte-span scoring.
- No held-out claim: the committed fixtures are synthetic scoring fixtures.
- No "already fixed" detection in Corvint itself; the control measures its absence.

## Failure modes (external evaluation slice)

- A ContextBench export whose columns drift is refused per row (`CEP-V0-001`), never partly scored.
- A snapshot missing a gold file still counts that file's gold lines; a ranked path missing from the
  snapshot predicts no line, so line precision is not inflated.
- Whole-file line prediction makes `cb_line_precision` a lower bound on what a span-granular packet
  could reach; it is reported as such, not compared to span-predicting agents as equal.
- A failed corvint producer on a control leaves `packet_abstained` absent: an unobserved packet is
  not an abstention.

## Acceptance evidence and rollback (external evaluation slice)

| Requirement | Implementation | Evidence |
|---|---|---|
| CEP-V0-001 | `decodeSample`, `contextBenchSample`, `contextBenchPath` in `tools/retrieval-bench/contextbench.go`; `readSamples`, `queryText` | `TestContextBenchRowsMapToSamplesAndBadRowsAreRefused` |
| CEP-V0-002 | `contextBenchMetrics`, `mergeLines`, `fileLineCount`; `judge` | `TestContextBenchFixtureScoresFileAndLineCoverage` |
| CEP-V0-960 | `contextBenchMetrics`, `contextBenchLengthMatched` | `TestContextBenchAbstentionAndLengthMatchedPrecision`, `TestContextBenchFixtureScoresFileAndLineCoverage` |
| CEP-V0-003 | `readSamples` digest, `resolveSnapshot` | `TestContextBenchFixtureScoresFileAndLineCoverage` (pinned `samples_sha256`); full ContextBench run retained as an aggregate receipt (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-25-v0415-contextbench-swebench-pro.md`, V1-0415) |
| CEP-V0-004 | `validateControl`, `scoringGold` in `tools/cw-trial/control.go` | `TestAlreadyFixedControlRefusesGoldAndUnknownControls`; `TestAlreadyFixedFixtureRecordsAConfidentAnswerAsFailure` |
| CEP-V0-005 | `controlMetrics`, `packetAbstained`, `controlSummary`; `scoreArm`, `summarizeArm` | `TestAlreadyFixedControlScoresAnyConfidentAnswerAsFailure`; `TestAlreadyFixedFixtureRecordsAConfidentAnswerAsFailure`; `TestAlreadyFixedControlStaysOutOfRealTaskAggregates` |
| CEP-V0-006 | BUILD-LOG V1-0097 entry, decision 0372 | fixtures only under `testdata/`; no report committed |
| CEP-V0-008 | `validateBudget`, `budgetBlock`, `budgetMetrics` in `tools/retrieval-bench/budget.go`; `judge`, `registerBeforeRun`, `pairedMetrics`, `resummarize` | `TestCEPV0008EqualBudgetWindowsScoreEveryArmAlike`, `TestCEPV0008AbstainedOrErroredArmDeliversNoWindow`, `TestCEPV0008SummarizeRefusesMixedBudgetsAndKeepsTheBudget` |
| CEP-V0-009 | `outcomes`, `nullableRatio` in `tools/retrieval-bench/budget.go`; `summarize` | `TestCEPV0009OutcomesSeparateCoverageAbstentionErrorsAndAnsweredPrecision` |
| CEP-V0-010 | `experiment` in `tools/retrieval-bench/budget.go`; `bench`, resummarize | `TestCEPV0010ReportNamesItsExperimentKindAndBudgetFlagsAreChecked`; official SWE-bench Pro grader `NOT_RUN` |
| CEP-V0-007 | `packet.Widening`, `parsePacket` in `tools/retrieval-bench/main.go` | `TestParsePacketRanksEachResultOncePerPath` (widened and abstained-widened packets); preregistered rerun recorded in `docs/build-log/2026-10-25-v2716-v2717-query-recall-opencode-adoption.md` |

Rollback: revert the V1-0097 change. `contextbench.go`, `control.go`, their tests and fixtures are
new files; the `readSamples`, `queryText`, `judge`, `validateTask`, `scoreArm` and `summarizeArm` hooks
are single calls, and existing reports keep their bytes because the new fields appear only for
ContextBench rows and already-fixed controls.

## Traceability

| Breakthrough | Delivery | Present evidence | Next proof |
|---|---:|---|---|
| Authority Closure Compiler | experimental | Corvint/Beamfall dogfood + focused regressions | 100-prompt/10-repository authority corpus |
| Portable Snapshot Context Capsule | experimental | shared core, protocol golden, single-snapshot regressions | four-host black-box matrix |
| Proof-Carrying Compaction Survival | not-started | exact dirty-path compact recovery; Claude Code `PreCompact`/`PostCompact` pin verdict (`TestAHI027ClaudePreCompactEmitsPinFromCompactionBlock`, `TestAHI028ClaudePostCompactReportsNonRehydratablePaths`), live cycle `NOT_RUN` | 30-task three-cycle survival trial |
| Counterfactual Minimum Witness Compiler | not-started | frozen PCCO contract only | observed held-out cost-per-solved-task trial |
| Live Context Frontier | not-started | composition hypothesis only | 30 five-session mutation sequences |
