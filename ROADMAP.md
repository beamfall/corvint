# Roadmap

The owner's 2026-09-12 integrated release request is tracked in
[Corvint integrated product roadmap](docs/plans/integrated-product-roadmap-2026-09-12.md):
working Corvint, automatic docs through MCP, agent/VS Code automatic test tracking including E2E,
the local dashboard, and task manager with a roadmap. Its delivery slices reference the owning
`AT-` and `TCP-` work below; they do not replace those contracts or declare them complete.

The accepted [0.6 local-workflow scope](docs/decisions/0332-verified-local-workflow-scope-2026-09-22.md) and [0.6 portfolio reading map](docs/PORTFOLIO-0.6.md) govern the current qualification target. It remains **NOT_QUALIFIED**. The broader integrated outcome and AT/E/U selections below remain historical intent, not a live execution queue. When an initialized `.taskman` store is present, its native queue and receipt own current execution status; otherwise work in a workspace that has the store. Do not initialize a replacement or infer status from this Markdown.

Dates are targets; evidence gates, not dates, promote a version. Every version must deliver one
useful standalone workflow before the next begins. An experiment that misses its kill criterion is
removed or returned to an adapter; it does not become permanent architecture.

## Current build queue: indispensable context and cumulative development savings

Selected 2026-09-04 at `01aa66ad071756f7308bb04b0ec379b051a231e3` in response to the owner's
request for a carefully selected roadmap and extensive Astra/Corvint self-use. Tickets below were open at selection; consult the native store for current status when initialized. Roadmap selection authorizes planning; it does not accept new wire semantics, promote
experimental capabilities, or change existing release gates. Re-pin before implementation.

This was the selected queue for the exceptional-Corvint program
and efficiency proposals.
`AT-*` tickets preserve selection history; `E*` and `U*` identify design material, not separate queues.
This selection replaces their earlier proposed scheduling, particularly U02's early priority and
the coupling of checkpoint recovery to compatibility reduction. V4/V5/V6 obligations below remain
active; the 64-spec map preserves the wider portfolio.

### Owner priority 2026-09-05: verified work at lower complete cost

Owner refinement, 2026-09-06: token efficiency is vital. Corvint should make complete verified
work materially quicker, cheaper and more efficient than working without Corvint, across its
supported workflows. Reduce avoidable reads, repeated context, tool calls and duplicated
verification while preserving correctness, model compatibility and normal caching behavior.
Keep model settings, prompt handling and cache behavior fixed during this process refinement.
The owner's "always" ambition is a product target, not an established universal performance
claim: charge every regression and failed attempt, measure the complete workflow against the
strongest adequate baseline, and retain the exact eligibility and promotion gates below.

Wave 1 is landed at `300e181`; its measured failures and limits remain in
`docs/reviews/nextgen-wave1-2026-09-05.md`. The owner prioritizes reliable evidence
selection and honest uncertainty (AT-02/05), complete cost measurement alongside
it (AT-01/08), and downstream reviewer usefulness (AT-17). The isolated H1 latency
prototype is paused. These are existing queue owners, not a second roadmap.

The target for a fresh, frozen evaluation is zero treatment-only critical misses
and zero false complete-evidence verdicts on designated missing-evidence cases.
Measure useful coverage with abstention accuracy; refusing every task cannot pass.
Governing requirements and unresolved obligations must survive context reduction
and compaction. Structural receipts do not prove completeness or correctness.

PCCO's accepted promotion gates remain binding: at least 30 eligible tasks across
three repositories, sealed correctness and compaction survival, at least 25% lower
median and 20% lower p75 complete cost per solved task against the strongest
baseline, and no p95 task-latency regression above 10%. Separately measure total
COMPLETE campaign cost divided by verified successes, charging every failed
attempt and retry. A proposed announcement of at least 25% lower aggregate cost
must pass that additional measurement; a median reduction does not establish it.
Include cached/uncached input, output/reasoning, compaction, tools, verification,
indexing and setup; separate cold-start and amortized setup and preserve raw usage
with independently versioned offline pricing. Allowance claims require observed
allowance use. These are targets, not delivered savings.

Competitive evaluation follows qualification under AT-08/16: identical tasks and
model/configuration, frozen selection and exclusions, repeated trials, uncertainty
intervals and a preregistered expansion rule. Compare vanilla, Corvint and Halv only
when each is actually available; otherwise name the strongest tested baseline and
mark Halv `NOT_RUN`. Retain every pair, failure, exclusion and regression. No
sealed partition is opened to tune implementation, and no publication is authorized.

### Owner request 2026-09-04: an agent task manager (outside this queue)

The owner asked for a best-in-class tool that runs a ticket queue across agent runtimes, picks
runners and reviewers, summons catalog experts at a chosen model tier and effort for adversarial
review, checks generated documentation against delivered behaviour, and is fully configurable,
after a dated landscape survey. Intent, proposed requirements `ATM-V0-001..027`, acceptance
evidence, and the survey requirement are recorded in
[docs/plans/AGENT-TASK-MANAGER-2026-09-04.md](docs/plans/AGENT-TASK-MANAGER-2026-09-04.md). It is
proposed as a standalone Go tool consuming Corvint as its evidence engine, so it is not an `AT-`
ticket; Gate A on the plan is in progress (five rounds, all findings applied) and nothing is built.
The owner added (2026-09-04): "once the task manager is working, you should move the rest of the
astra work to it and start using and refinding it" — so the open `AT-` tickets below are the tool's
first real queue once its first slice lands, and its self-use is the refinement loop.
P6 tracker-read adapters fold into ATM; Corvint retains only content-hash pinning for imported intent.

### Selection: build the dependable daily workflow first

| Choice | Decision and reason | Delivery |
|---|---|---|
| Source admission, sufficient context and exact expansion | First: supported evidence omissions undermine every optimization. | AT-02/05 |
| Snapshot-scoped batching | First: isolate fewer loads and round trips without changing retrieved meaning. | AT-03 |
| Explicit checkpoint and recovery | First: repeated reconstruction is a daily cost; it does not need a reducer. | AT-06 |
| Exact delta expansion | First, after recovery: never suppress evidence merely because it was once delivered. | AT-07 |
| Selective immutable serving | Independent format experiment now; production selection follows its own accepted gates. | AT-04 |
| CEM outcomes and independent interoperability | Continue immediately; no compatibility or context win substitutes for these gates. | AT-17 |
| Replay, meaningful reduction, retained behavior and truthful documentation | Next differentiation loop; prepare the corpus alongside earlier builds. | AT-09/10/11/12/15 |
| Advisory verification guidance | Start independently of compatibility where existing contracts suffice. Execution/exclusion remains separately qualified. | AT-13/14 |
| Stable cache layout, richer progress coaching, compact observations and shared-worker evidence | Conditional: first demonstrate eligibility, measurable waste and sufficient telemetry. | Conditional work below |
| Neural pruning/generation, verdict caching, new hosted surfaces | Defer behind existing contracts and a measured advantage over simpler tools. | Existing owning gates; no new speculative implementation ticket |

The first useful milestone is **start, investigate, resume and explain a Corvint change with less
repeated work**. The next adds behavioral reproduction, retained witnesses and maintained knowledge.
The stretch target remains 50% lower total observed tokens per solved repeated-change task; it is
not a forecast. Preserve accepted correctness, critical-miss, cost and release conditions. No
paper review can certify a perfect selection; these choices have explicit disproof and removal paths.

### Astra execution and Corvint self-use

Use Astra for the coordinator and bounded builders/reviewers (`gpt-6-astra`, high effort for
architecture, invariants and review; medium for routine implementation). Use deterministic tools
for mechanical checks. Initially run at most two builders with disjoint owned paths and one fresh
read-only reviewer; the coordinator owns integration. Shared format, harness and scorer changes
are serial. Read-only case preparation may use a free slot. Do not create a new scheduler or a
permanent service. Record the actual model/effort; if Astra is unavailable, disclose the gap rather
than silently claim equivalent execution.

Every substantive ticket follows this loop:

1. **Ground:** use the last admitted Corvint profile for narrow context/impact, then exact source
   expansion. Start the private measurement receipt before context acquisition. Pin the base,
   binary, specs, task, access and actual host capabilities. If integration is unavailable, try
   the supported public CLI; if that cannot serve the task, record the reason and use targeted
   source inspection. Fallback permits engineering, not a Corvint-use claim.
2. **Build:** isolate the change, claim exact files, record an acceptance-to-test map, and use focused
   regressions during repair. New wire/invariant work needs Gate A before code; update the owning
   accepted or explicitly experimental spec and its index entry in the same slice.
3. **Review:** a separate Astra reviewer inspects the actual diff, governing evidence and tests.
   Lock independent findings before discussing builder conclusions. Builders never adjudicate their
   own correctness labels or author expected behavior from candidate output. Same-model review is
   useful engineering review, not an independent organization or a new execution authority root.
4. **Verify and bind:** run the appropriate final `make gate` once per unchanged target; check for
   an existing coordinator/gate first. Use no PTY. Audit acceptance criteria (Gate B), inspect CEM
   and OCM reports, and finish the documented dogfood sequence when committing is authorized.
   Keep every `NOT_PRODUCED` reason; a refusal or an uncovered requirement is not completion.
5. **Adopt on the next task:** test a candidate in shadow first. After its relevant conformance
   gate, use the opt-in experimental feature on the next eligible development task, with the last
   admitted route as fallback. Production defaults still wait for promotion. Record `USED`,
   `AVAILABLE_NOT_ELIGIBLE(reason)`, `UNAVAILABLE(reason)` or `FALLBACK(reason)` for each feature.
   A feature's own demonstration alone does not prove subsequent use.
6. **Learn locally:** close the full-task receipt after review/repair, record actual outcomes in
   `docs/BUILD-LOG.md`, and file concrete friction in the six existing agent-memory backlogs.
   Reuse this evidence to prioritize work, never to rewrite intent or tune a held-out partition.

Count coordinator, builder, reviewer, repair and failed/cancelled worker costs. Use scoped,
pseudonymous worker/event identifiers; never forward raw session IDs or transcripts through AHI.
Deduplicate usage events and distinguish provider totals from incremental events. Sum token/cost
units across workers; report elapsed task wall time separately from summed worker time. Missing
cache, reasoning, token or source-open dimensions are `NOT_OBSERVED`, not zero or bytes-to-token
estimates. Accounting stays outside recursive Corvint analysis; do not spend model calls reviewing
every accounting event. Human review effort belongs in the same full-workflow report.

| Once qualified for experimental use | Required next eligible use | Evidence retained |
|---|---|---|
| AT-02/05 supported context | Ground the next builder and independent reviewer in its exact packet. | Misses, expansions, omitted frontier, source opens and complete cost. |
| AT-03 batch | Use for the next investigation requiring three-to-five independent reads. | Same-snapshot result parity, total bytes, round trips, p95 and memory. |
| AT-06 recovery / AT-07 delta | Resume the next interrupted task; then supply deltas during subsequent review. | Visible survival, drift, rehydration, stale-action controls and notes baseline. |
| AT-04 candidate reader | Shadow the next context task; switch only after the selected format gate. | Old/new receipts, touched bytes, resource bounds and rollback. |
| AT-10/11 replay and reduction | Apply to the next eligible Corvint CLI/schema change and genuine unexplained difference. | Frozen expectation, exact replay, meaningful reduction and rerun after repair. |
| AT-12/15 retained knowledge | Retrieve the example/page for the next related task without naming its file. | Discovery success, current versus historical state, invalidation and source rederivation. |
| AT-13/14 guidance | Order checks on the next repair while mandatory checks still run. | Time to actionable finding, observation identities, full-gate shadow misses. |

Lack of an eligible task is visible; do not manufacture a bug or burn a holdout to fill an adoption
cell. Corvint self-use is development/friction evidence. External demand, independent producers and
consumers, and held-out outcome claims keep their separate gates (`DOGFOOD-006`).

### Astra builds its working capabilities as it goes

Within this roadmap, Astra's working capabilities are the Corvint tools and development workflow it
implements and operates. Build them during real Corvint work, then use the tested result immediately
on the next eligible slice. Do not postpone self-use until a complete platform exists.

| Build | Then use it to build | Concrete improvement to Astra's work |
|---|---|---|
| AT-01 cost measurement | AT-02 admission and AT-03 batching | Identify actual repeated reads, worker overhead and missing telemetry before choosing another optimization. |
| AT-03 batching | AT-05 context and AT-06 recovery | Collect several required evidence views in one bounded investigation. |
| AT-05 sufficient context | AT-06/07 recovery and deltas | Begin implementation and independent review from governing code/test/spec evidence with visible gaps. |
| AT-06 checkpoint recovery | AT-07 delta expansion | Resume a real interruption with the tested checkpoint; validate lost/stale evidence before continuing. |
| AT-07 exact deltas | AT-10 replay and AT-13 guidance | Avoid unnecessary retransmission across continuing investigation/review while keeping recovery available. |
| AT-10 replay | AT-11 reducer and eligible subsequent CLI changes | Compare old/new behavior using independently frozen expectations before accepting a change. |
| AT-11/12 reduction and retained witnesses | AT-15 documentation and later relevant fixes | Keep an actual reproducer, retrieve it later, rerun it, and explain what it establishes. |
| AT-15 capability documentation | The next related implementation/review | Orient from maintained knowledge, with exact source expansion and independent rederivation. |

These are consumption obligations, not additional hard dependency edges for unrelated tickets.
If a successor starts before the prerequisite is qualified, use the admitted predecessor and record
the gap; require candidate adoption on the next eligible slice after qualification.

When a task reveals repeated manual glue or a costly missing capability, Astra records the concrete
trace, checks for an existing primitive and chooses the smallest enabling change. If it fits the
current owning contract, build it in a separately reviewable slice with a focused regression and
use it to continue the blocked work. If it changes accepted intent or an unresolved wire boundary,
prepare the bounded proposal and keep progressing through the supported fallback. Assign an AT
subticket only when the scope/dependency requires it; do not build a speculative general framework.

Limit each detour to one named enabling slice and a declared time/work budget, then resume the
original ticket or report why it remains blocked. No recursive chain of tools to build tools.
The changed tool cannot supply its own acceptance oracle: retain predecessor output, independent
fixtures and the normal gates. Record whether subsequent use actually reduced complete cost.

### Audit follow-up execution, 2026-09-06

The owner requested the expert audit
recommendations be put in place. The [bounded integration spec](docs/specs/expert-audit-followup-v0.md)
records the immediate repairs; the [development protocols](benchmarks/expert-audit-followup/README.md)
give the new hypotheses inputs, executable baselines and stopping conditions. They do not create
another queue or qualify a feature by adding its ticket.

| Existing owner | Required follow-up and status |
|---|---|
| AT-02/05 | Separate admission/cap/rank/budget losses before changing ranking. Fixed rarity cutoff two repairs the padding counterexample but loses four trace-positive results and fails the registered utility guard after all five paired subsets complete. Candidate REJECTED; production ranking remains unchanged. |
| AT-06/07 | Obligations now survive checkpoint output with caller-reported-unverified attribution; partial-work, authority-drift and empty/forged-input regressions pass. Real fresh-agent/notes comparison remains open. Query help now gives same-task trace-preserving fallback. |
| AT-12/15 | Added-consumer and parser-failure source-rederivation regressions pass. A bounded 100-transition development prototype matches full recomputation and rejects 60 injected stale results; full snapshot/task cost remains unmeasured, so runtime reuse is not qualified. |
| AT-05/17 | The replacement 20-change development cohort rejects the frozen challenge: 5 of 19 applicable obligations found, 14 missed, 0 of 20 unrelated controls matched; 1 UNKNOWN stays unscored. The original author-citation-selected cohort is invalid; the replacement lacks a fully rebound preregistration. Runtime integration is rejected pending a different independently evaluated mechanism. |
| AT-17 | Ordinary proposed-policy tests cover 18 atomic inputs, all 8,191 nonempty reason sets and wrong/missing/duplicate-cell controls. Stop at these tests; no new adapter or general utility qualification. |
| AT-01/08/16/17 | Question-first activation and reviewer-requested evidence share one repeated-change experiment with unrestricted tools/notes/CI controls and complete all-worker costs. Three real-repository candidates are identified, but independent acceptance oracles, authorized related follow-ups and matched environments remain incomplete; workflow trials remain NOT_RUN. The exposed V4 partition is quarantined for this task. |
| AT-17 / decision 0009 | The mutually exclusive execution-authority choice remains owner-pending; the general audit follow-up does not select a new root or allow caller observations to close Frontier. |

The quoted-credential and mutation-mode bugs are repaired under EAF-V0-001/002; their pending
bug entries are removed with those fixes. No AT ticket is checked off by these bounded repairs.
Final code-gate, independent review and CEM/OCM results belong in BUILD-LOG.

### Build tickets

All tickets are in repository **corvint**. Each owns only its named slice; no checkbox means delivery.
Sizes are planning estimates, not promised dates. Dependencies below are hard build prerequisites;
lane priority is the selected order above. Conditional/external evidence does not block unrelated
local work. All behavioral tickets inherit the build/review/dogfood loop above.

- [x] **AT-00 — Pin the build contracts and usable self-use path.** E00 foundation. Delivered 2026-09-04: pin record.
  **Size:** S. **Depends:** none. **Owns:** existing spec/index routing, dogfood protocol and support inventory.
  **Accept:** map each first-wave behavior to existing requirement IDs or a bounded proposed amendment;
  pin base/tool versions; distinguish available CLI from unavailable integration; preserve current failures.
  **Verify:** resolve requirement/source links; inspect an actual narrow query and applicable impact/refusal;
  run change-start `make dogfood-change BASE=<base>` and retain its complete report. No invented closure.

- [ ] **AT-01 — Measure complete Astra development cost.** E00a; `DOGFOOD-008/009`, `BRAIN-DOG-013`, `PCCO-V0-012` where applicable.
  **Size:** M. **Depends:** AT-00. **Owns:** `benchmarks/dogfood_measure.py` and the smallest host-side accounting adapter.
  **Accept:** one all-worker receipt includes retries, compaction, source opens, review and failed work;
  missing telemetry is explicit; freeze context/recovery baselines and scorer edge cases independently of compatibility.
  P7 cost-receipt scope accepts only an explicit host log passed on the command line; automatic
  host-session-directory reads are not authorised.
  **Verify:** double-count, missing, cancelled-worker, zero-solved and pricing-version fixtures; one real
  builder/reviewer task. Partial telemetry may support engineering but cannot pass a complete-cost claim.
  **Status 2026-09-05:** receipt adapter and context/recovery definition/scorer freeze delivered (`benchmarks/workflow-baseline-v0.md` and `.json`). Owner-selected task list, solved thresholds, sample/power rules and complete real host-log baseline remain open; task-list/run obligations continue under AT-08. The current bounded repair prevents invalid or ambiguous usage from reducing observed totals; it does not complete the cost claim.

- [ ] **AT-02 — Repair supported-evidence admission and expose omission stages.** E01; Track A.
  **Size:** M. **Depends:** AT-00. **Owns:** `internal/contextindex` admission/parsing/query diagnostics.
  **Accept:** distinguish absent/excluded/unparsed/candidate/ranked/budget omissions; repair demonstrated
  documentation/test gaps without promoting ordinary text to governing authority.
  **Verify:** frozen development counterexamples for each stage, positive/negative authority controls,
  exact response budget and applicable current query gates. Preserve blind-v4's existing first-run conditions.
  **Status 2026-09-04:** slice 1 delivered (`GPK-V0-040` denomination past `--limit`, accepted `GPK-V0-052` (decision 0052, 2026-09-04) withheld-test-symbol disclosure inside the byte budget, `DR-0022` gate ceiling); `DR-0025` pins the `--limit 1` parity case; decision 0051 routes the two remaining silent stages to `DR-0023`/`DR-0024`; slice 2 landed `DR-0024` (null-budget wording `omitted by result limit`, proposed `GPK-V0-053`, parity 133/0 FAIL, ACCEPTED by a fresh `wire-contract-engineer` review) and deferred `DR-0023` on measurement (19 parity cases would move; decision 0051 amended); feature/learning denominators remain (`fixes.md`); full gate pending.

- [ ] **AT-03 — Batch unchanged investigations under one snapshot.** U01a; E02/E03 seam.
  **Size:** M. **Depends:** AT-00. **Owns:** bounded batch verb in `cmd/corvint` over the `internal/contextindex` snapshot-reader seam (`LoadSnapshot`); `internal/gokernel` is the harness probe, not this seam. Needs its own experimental spec before code.
  **Accept:** bounded independent context/impact/expansion requests share one captured snapshot; per-operation
  errors and receipts remain explicit; no speculative reads or persistent query writes.
  **Verify:** independent-command parity at the same snapshot, operation/byte/work bounds, partial failure,
  dirty-state refusal, and measured three-to-five-read latency/RSS against separate calls. Adopt on the next eligible task.
  **Status 2026-09-04:** delivered as `corvint batch` under proposed `SBQ-V0-001..006` after Gate A and a fresh Astra review; measured 0.23 s/152 MB against 1.71 s/246 MB for four separate calls on this repository; adopted for the AT-05 grounding; full gate pending.

  **Development adoption 2026-09-06:** the opt-in `benchmarks/selfuse-batch` consumer exercises
  ordinary batching with full evidence and explicit fallback; independent review removed its
  unqualified possession shortcut. A clean same-snapshot comparison retained four exact packets
  (one Corvint process versus four); the failed-view control stayed failed. This is mechanism evidence,
  not a complete-task/cost win or promotion. The recurring loop continues through the existing AT owners.

- [ ] **AT-04 — Select the immutable reader using accepted format evidence.** E03; Track F.
  **Size:** M experiment. **Depends:** AT-00. **Owns:** isolated reader benchmarks at `internal/contextindex/snapshot.go`.
  **Accept:** compare current gob, immutable SQLite and bounded section reader without changing ranking;
  identify winner or inconclusive result against `DNIP-IDX-001..009` and applicable GPK/IDX-SNAP gates.
  **Verify:** equal receipts, touched bytes, cold/warm/tail latency, RSS, corruption, crash/rollback and
  declared scale cells. Small corpora cannot satisfy million-file gates. Production-format work is specified after selection.
  **Status 2026-09-04:** experiment run in `benchmarks/snapshot-reader/` (own module); inconclusive after a measurement repair, the pack's O(paths) manifest is the next lever; no selection.
  **Status 2026-09-12:** benchmark pack v2 (O(sections) manifest) reads 14,388 bytes, not 26.1 MB, for a cold one-path lookup at 100k; latency is load-contaminated and the other cells were not run; no selection (BUILD-LOG 2026-09-12).
  **Review 2026-09-06:** current storage audit
  distinguishes that old benchmark pack from current experimental `.aip`, which still verifies
  full-query bodies and materializes metadata. Continue decision 0069 through current-format SQLite
  comparison, bounded authenticated reads, reader lifetime and lookup coverage. Current diagnostic
  means and passing fixture parity do not close latency, RSS, integrity or scale promotion gates.

- [ ] **AT-05 — Serve sufficient context with bounded widening and inline budget gaps.** E02; U03 minimum.
  **Size:** M. **Depends:** AT-01, AT-02. **Owns:** task-context typed relations, selection and exact expansion.
  **Accept:** reserve known governing/critical evidence, deduplicate exact handles, expose unexamined scope
  and budget shortage in existing responses; widen deterministically without false absence or answerability claims.
  **Verify:** fixed slots and equal-byte lexical controls; complementary code/test/doc cases; unmet-critical-budget
  and vocabulary-mismatch cases; full-task cost/correctness under `BRAIN-DOG-012/013/015/016`.
  **Expert review 2026-09-05:** the relation allowlist repair is a bounded prerequisite.
  `critical_missing` covers reserved selectors only; raising the final limit cannot recover
  rows discarded at relation caps. Localize admission, cap, ranking and byte-budget losses
  against the strongest same-byte baseline before adding a widening heuristic.
  **Status 2026-09-04:** built under proposed `TCP-V0-008..012` after three Gate A rounds (`taskcontext.go`, 8 falsifier tests); on this repository `AGENTS.md` is now the governing row and the `AHI-003` spec is admitted as spec-mentioned; fresh Astra review returned five repairs (slot dedupe before cap, per-spec ambiguity, independent coverage oracle, falsifier (d) fixture, subject activation), all applied with tests and ACCEPTED on round 2; a pre-existing `coverage.candidates` accounting defect is in `bugs.md`; full gate pending.

- [ ] **AT-06 — Checkpoint and recover an interrupted task.** E06a; U04; Track D.
  **Size:** M. **Depends:** AT-01, AT-05. **Owns:** explicit checkpoint/expansion slice under AHI/Genesis/PCCO.
  **Accept:** preserve visible intent, handles, unknowns, failed approaches and observation provenance;
  rehydrate critical selectors or return supported fallback. No hidden-reasoning or remembered-evidence claims.
  P2 session-capsule scope is owned here and by AT-07, not by a new ticket.
  **Owner-goal follow-up 2026-09-06:** the authorized FPK-V0-021/023 amendment now returns
  the exact obligation array with caller-reported-unverified attribution. Partial-work, authority-drift
  and empty/forged-input regressions pass. This closes transport loss, not the fresh-agent recovery
  and complete-cost outcome obligations below; the ticket remains experimental and open.
  **Verify:** fresh-agent resume, unavailable host state, merge/revert/deletion/authority drift and budget
  shortage; compare against competent structured notes. Deferred SESSION persistence remains unimplemented.
  **Status 2026-09-04:** Accepted revision 11 (0052); checkpoint implementation and same-tree HEAD/marker repairs pass focused tests. Fresh Codex repair review PASS; full gate, Claude review and real interrupted-task comparison remain pending; not promoted.

- [ ] **AT-07 — Expand deltas without starving the agent of evidence.** U01b.
  **Size:** M. **Depends:** AT-03, AT-06. **Owns:** caller-supplied possession and exact expansion contract.
  **Accept:** validate current visibility, snapshot and access; return new evidence/invalidations;
  resend required material when retention is unobserved. Query stays stateless and read-only.
  P2 session-capsule scope is owned here and by AT-06, not by a new ticket.
  **Verify:** stale/forged possession, compaction, access change, deletion and exact rehydration; compare
  against complete packets. Reject suppression if retries or source rereads erase full-task savings.
  **Status 2026-09-04:** Accepted revision 10 (0052); implemented and fresh Codex review PASS. Focused/repeated/race tests pass; fixture suppresses 1663/2231 evidence bytes (74%). Full gate, Claude review and full-task savings evaluation remain pending.

- [ ] **AT-08 — Evaluate the first complete daily workflow.** E09a; Tracks A/D/F.
  **Size:** M evaluation. **Depends:** AT-01, AT-02, AT-03, AT-05, AT-06, AT-07.
  **Accept:** freeze unseen start/investigate/resume/change-review tasks; compare same-Astra current Corvint,
  unrestricted native tools plus structured notes, and candidate; include failures and all-worker costs.
  **Verify:** component ablations and integrated outcomes, independent correctness labels, per-class cost,
  completion, critical misses and p50/p95. Apply owning BRAIN-DOG/AHI/PCCO gates only with their exact eligibility;
  report screening-only where power or telemetry is insufficient. AT-17 remains a separate release obligation.
  **Freeze prerequisites 2026-09-05:** independently label solved, critical-miss and false-complete
  outcomes; select useful-coverage floors, repeats, intervals, stopping and expansion rules
  before fresh task selection. Repeats are nested within tasks, not extra independent tasks.
  Amend WB-005 under this owner before pricing projections: raw host-qualified usage, a
  complete worker/request roster, terminal reconciliation, independently versioned pricing,
  cached-input/reasoning subset bounds, setup and queue/task timing boundaries are required.
  Report per-solved-task distributions and all-COMPLETE-campaign-cost/verified-successes
  separately; include failures/retries and mark zero-success ratios undefined. Any required
  unobserved request or cost makes the campaign incomplete; dropping partial observations
  cannot create a qualifying COMPLETE subset.
  The independently reviewed measurement amendment
  is proposed/inactive; its sidecar does not alter frozen WB/WS receipts or authorize execution.
  **Status 2026-09-04:** Astra hunt found no fresh holdout source for start/resume and every existing corpus development-only or reserved; screening protocol and exclusion registry drafted (`benchmarks/workflow-screening-v0.md/.json`, `WS-001..008`); review running; evaluation `NOT_RUN` until AT-05/06/07 land.

- [ ] **AT-09 — Freeze the compatibility experiment independently.** E00b/P0.
  **Size:** M. **Depends:** AT-00. **Owns:** applied-intelligence experiment contract, supplied-input development corpus and baseline.
  **Accept:** freeze comparison semantics, resource profile, independent labels, 20 development cases and
  strong differential baseline; preregister the 40-case screen and full-cost/undefined-ratio rules from the program.
  **Verify:** source/build/fixture acquisition and unsupported denominators; withheld labels cannot reach builders.
  Corpus preparation may run alongside the first milestone; it does not block AT-02 through AT-08.
  **Status 2026-09-04:** freeze document written (`docs/specs/compat-trial-v0.md`, `CTR-V0-001..010`); about ten of twenty candidate cases look eligible; labels, baseline and rubric await the owner (`questions.md`).

- [ ] **AT-10 — Qualify one old/new replay profile.** E04; Track B.
  **Size:** M. **Depends:** AT-09. **Owns:** proposed `tools/compat-trial` descriptor, runner and host envelope.
  **Accept:** independently pinned expectations, repeated observations, explicit instability/setup/timeouts;
  bounded IO/resources and denied unrequested effects. External execution stays unavailable without containment.
  **Verify:** contamination and changed-executable cases; interruption leaves no descendants, including
  process-group/session escape; replay eligible Corvint changes. Existing parity tooling is prior evidence, not automatic qualification.
  **Status 2026-09-04:** Accepted revision 12 (0052), distinct descriptor-invalid reason approved in 0053; experimental runner implemented. Focused checks pass; fresh reviews and full gate pending. Full CLI parity timed out; external containment and scored replay remain unqualified.

- [ ] **AT-11 — Reduce and export the same behavioral difference.** E05; Track B.
  **Size:** M. **Depends:** AT-10. **Owns:** deterministic bounded reducers and ordinary replay fixtures.
  **Accept:** preserve domain and named disagreement; retain original/provenance and rerun final specimen.
  **Verify:** unrelated-parser-failure and unstable-reduction controls; uninvolved reproduction of >=8/10
  eligible development findings. Disable an unhelpful reducer while retaining useful replay/export.

- [ ] **AT-12 — Retrieve and invalidate retained behavior across revisions.** E06b; Track D.
  **Size:** M. **Depends:** AT-05, AT-06, AT-11. **Owns:** bounded reviewed-witness lifecycle under Genesis/BRAIN-DOG.
  **Accept:** R0 retain, R1 discover without naming the artifact and rerun, R2 invalidate on helper/config/
  fixture/toolchain/policy/absence-universe change; preserve tombstones, conflicts and failed attempts.
  **Verify:** independently checked deltas, revert/revocation, clean/incremental equality and ordinary
  saved-fixture/notes ablations. Never inherit a passing execution across changed executables.

- [ ] **AT-13 — Recommend grounded checks without exclusions.** E07a; U07 foundation; Track C.
  **Size:** M. **Depends:** AT-00, AT-02. **Owns:** existing Go affected/provider planning seams.
  **Accept:** advise checks from supported current dependencies with explicit unknowns; all repository-mandatory
  gates remain mandatory. Static advice does not wait for compatibility replay or claim execution evidence.
  **Verify:** required dependency/discovery/widening fixtures, snapshot identity, unsupported surfaces and
  full-gate comparison. Defer cost ranking until observations support calibration.
  **Status 2026-09-04:** delivered as the `advice` member of `affected` under accepted `AFP-V0-009` (decision 0052, 2026-09-04) after a fresh Astra review and repair; its advisory command ran as this wave's focused regression (16 packages ok); full gate pending.

- [ ] **AT-14 — Qualify current verification observations and shadow exclusions.** E07b; Track C.
  **Size:** M. **Depends:** AT-13. **Owns:** `internal/liveverify/provider`, `cmd/corvint-go-test-provider` and owning LPCV/GLTP slice.
  **Accept:** separately qualify its process profile and observation authority, bind current identities,
  run full verification in shadow, and retain unsupported observations. Reuse AT-10 containment evidence only where exactly applicable.
  **Verify:** independent failure/timeout/resource/identity fixtures and no surviving descendants; owning
  discovery/exclusion outcome gates. One missed release-blocking failure disables that automatic exclusion profile.
  **Status 2026-09-04:** Accepted revision 12 (0052); implemented experimental qualification slice, focused tests and Codex repair review PASS. Real 30-minute deadline is BLOCKED; external/Linux/full qualification, full gate and Claude review remain pending.

- [ ] **AT-15 — Maintain one truthful capability page from retained evidence.** E08; Track E.
  **Size:** M. **Depends:** AT-12. **Owns:** bounded existing doccompiler/docviews integration.
  **Accept:** source-bound ordinary Markdown with explicit epistemic state, behavior examples and unknowns;
  update affected claims, preserving conflicts/tombstones and accepted intent.
  **Verify:** source/policy drift, clean/incremental equality, affected-claim recall and unrelated churn;
  next-task orientation followed by independent source rederivation. Full HDC publication/security gates remain open.
  **Status 2026-09-05:** experimental SDD-V0 draft/consume integration derives ordinary Markdown from owner-source excerpts plus tracked non-test Go declarations/imports, then consumes actual bytes after fresh source rederivation. Full claim maintenance, conflicts/tombstones, independent outcome validation and HDC gates remain open.

- [ ] **AT-16 — Evaluate the complete repeated-change loop.** E09b; all six tracks.
  **Size:** M evaluation. **Depends:** AT-08, AT-12, AT-14, AT-15.
  **Accept:** compare three-revision Corvint workflow against the strongest frozen conventional stack with
  retained fixtures/notes/caches, and named eligible competitors. Report only the actual qualified index scale.
  **Verify:** sealed labels, complete positive/control/failure costs, paired uncertainty, no additional
  critical misses, reproducibility and voluntary repeat use where available. Inaccessible competitors/participants
  remain `NOT_RUN`. Passing a compatibility screen cannot promote migration, knowledge, index or interoperability claims.
  The 30-task/three-repository floor does not establish statistical power; honor the
  preregistered expansion and strongest-baseline choice without optional stopping.

- [ ] **AT-17 — Continue CEM-first release and independent outcome obligations.** Existing V4/V5 work, tracked rather than duplicated.
  **Size:** ongoing bounded slices. **Depends:** AT-00. **Owns:** existing CEM trial, release and interoperability owners.
  **Accept:** retain the 30/30 outcome requirements, release/held-out conditions and independent producer/consumer
  matrix. Reuse existing implementation tickets/evidence; repair the known scorer wording under its owning contract before new claims.
  **Verify:** actual frozen trial and conformance artifacts; preserve September 4 pilot limitations and
  `NOT_RUN` external cells. Absence of independent participants blocks that claim, not unrelated local builds.
  **Expert review 2026-09-05:** retain exact CRT point/interval rules; a positive lower
  confidence bound does not establish a population minimum of 20%. Report attrition and
  all-assigned failure sensitivity. Citation-selection success alone does not establish
  that independent reviewers catch consequential defects more effectively.
  **Status 2026-09-04:** scorer wording repaired under amended `CRT-V0-008` (five readings on the absolute-delta scale); no new outcome claim.

### Conditional work: specify only after the trigger

These are named obligations or experiments, not secretly completed work or abandoned accepted outcomes.
Create a bounded `AT-*` follow-on only once its inputs support concrete acceptance criteria.

| Work | Trigger / dependency | Required next decision |
|---|---|---|
| Production immutable format, extraction reuse and tree placements | AT-04 selection and accepted DNIP/GPK gates | Freeze chosen format/rollback and split implementation/migration/scale tickets; retain existing reader if no candidate qualifies. |
| U02 cache-aware layout | AT-05 eligible complete packets, AT-01 complete cache telemetry and applicable PCCO prerequisites | Compare ordinary stable ordering first; no token-to-cache inference or transformed source evidence. |
| U03 richer search-progress/cost guidance | AT-05/08 traces demonstrate repeated unproductive calls and calibratable next operations | Freeze progress semantics and test no extra misses/false absence; avoid an extra mandatory dialogue round. |
| U05 compact tool-result views | AT-14 supported structured output plus AT-01 evidence of material repeated log cost | Select one schema, keep raw expansion and faithful failure states; compare an ordinary concise formatter. |
| U06 shared-worker evidence | AT-06/07 proven recovery/access behavior and measured duplicate retrieval | Freeze scoped sharing and sealed reviewer judgments; count coordinator cost and correlated misses. |
| U07 estimated-cost ranking | AT-13/14 useful checks and independently calibrated costs | Keep mandatory gates; compare static order and time to actionable finding. |
| PCCO learned minimization and SEG generation | Their accepted observation, corpus, verifier and eligibility gates | Specify eligible offline experiments; serving stays local/deterministic/model-free. |
| Cross-revision passing-result cache | Complete admitted dependency/environment profile and forced-rerun comparator | Any false reuse blocks the profile; ordinary retained examples remain useful without the cache. |
| Wider integrations and external adoption | Actual independent use and supported host/profile demand | Continue accepted interoperability obligations; do not infer adoption from Astra workers or self-authored consumers. |
| P11 pinned external unit graph (`affected-units/0`), with SCIP as the first producer | Measured recall gain from symbol-precise rows on a held-out partition | Choose a first `go.mod` dependency or a stdlib protobuf decoder; this row authorises neither. |

### Sequence, completion and current evidence

Start AT-00, then AT-01 measurement and AT-02 admission. Fill a free disjoint lane with AT-03;
AT-04 benchmarking, AT-09 case preparation and AT-17 existing obligations proceed as ownership permits.
Next integrate AT-05/06/07 and evaluate AT-08. Use those capabilities to build AT-10/11/12/15;
AT-13/14 run when provider ownership is available. Finish AT-16 only when its real inputs exist.
Assign dates after measured ticket throughput; the earlier ninety-day windows remain research budgets.

A completed ticket needs requirement-to-evidence traceability, positive/negative checks, actual gate
results, independent findings/disposition, subsequent self-use or explicit ineligibility, known gaps,
and rollback. Structural implementation, experimental outcome, production admission and external
validation retain distinct statuses. If a hypothesis fails, record the result and simplify; do not
weaken its metric or keep adding features to disguise the failure.

Selection reviews by Product Strategy / Discovery Lead and Performance / Scalability Engineer used
Astra at high effort. They demoted cache optimization, split early checkpoint recovery from later
behavioral continuity, and made all-worker measurement and next-task self-use explicit. This roadmap
is documentation-only; no implementation, outcome trial or new promotion was performed.

A separate Test Architecture / Conformance Engineer using Astra at high effort found no HIGH/MED
actionable findings in the ticket selection, dependency graph or self-use boundaries. Its Corvint
query reported `STALE_INDEX` and excluded two oversized benchmark sources; direct current-file
inspection supported the review. The verdict is bounded planning review, not implementation evidence.

Documentation checks passed: 18 unique open tickets, 31 dependency edges with no cycles or missing
IDs, acceptance/verification on each ticket, valid local links, balanced fences, and continued
coverage of all 64 indexed specs in the companion map. No production gate or held-out suite was run
for this documentation-only revision.

The roadmap preflight produced a CLI query with mixed-worktree freshness; ordinary targeted reads
supplied the remaining context. `make dogfood-change` returned incomplete with every reason retained:
`prechange-impact: unsupported-impact-worktree`; `cem-prepare: git-diff-failed`;
`cem-cite: citation-plan-not-provided`; `cem-status: not-ready`; `ocm-aggregate: missing-intent-scope`;
`local-outcome: outcome-input-not-provided`. Base equals target while these plans remain uncommitted.
The Codex Corvint integration remains unavailable; that does not imply the public CLI is unavailable.

## V4: mechanical activation and reference evidence producer alpha (target 2026-09-12)

**Useful outcome:** `corvint init` or `corvint adopt` returns a complete mechanical repository
inventory, explicit orphan/unsupported frontier, and a small revision-pinned evidence packet without
a build, server, database, or model.

- Ship one idempotent bootstrap compiler behind exactly two doors: `init` for current-state startup
  and `adopt` for the identical fast pass plus bounded, resumable Git history.
- Mechanically classify every tracked path and expose code, configuration, schemas, instructions,
  docs, specs/RFCs/PRDs/TDDs, tests/E2E, CI, manifests/locks, ownership, runbooks/incidents, assets,
  exclusions, unsupported formats, dirty state, shallow/missing history, and access gaps. First useful
  p95 must be under five minutes; exceeding ten minutes removes history from the fast path.
- Package the current deterministic engine without a rewrite.
- Establish the project-profile seam; remove residual Beamfall compatibility behavior behind its
  adapter before beta.
- Add generic Markdown specification and decision evidence.
- Stabilize `corvint query`, `feature`, `impact`, `eval`, and `record` JSON contracts.
- Publish a 25-case, five-repository benchmark and a reproducible 60-second demo.
- Store disposable evidence fragments by immutable blob hash and make warm queries byte-identical
  to cold queries in under one second.
- Freeze the receipt schema and conformance fixtures as the product boundary; retrieval engines are
  replaceable producers of conforming receipts.
- Preserve blind-v1 and blind-v2 as observed development/challenge evidence and blind-v3 as the
  untouched V4 first-run failure. Repair only general integrity, performance, and correctness gaps;
  never tune selectors or relabel an observed partition held out.

V4 does not promise exhaustive feature flows, semantic test claims, incident guidance, or a
behavioral twin. Blind-v3 failed the V4 generalization gate, so this remains a reference-producer
alpha rather than a universal context product. Corvint is licensed under AGPL-3.0-or-later, with an
Apache-2.0 interoperability layer; release remains gated by artifact integrity and product evidence.

## V5: Change Evidence Map (30-day target 2026-09-21)

**Useful outcome:** within 15 minutes, an existing repository can verify a vendor-neutral evidence
association map for agent edits and a reviewer can see which evidence the producer attached to each
textual hunk, or where evidence is explicitly absent.

After the 0.4.0 tag, deliver these V5 emitters in order:

1. **P9 — “What shipped” narrator:** HDC's first emitter. Regenerate the 0.4.0 release notes from
   merged CEM and OCM, then diff them against the hand-written file; every discrepancy is a narrator
   defect or a drift finding.
2. **P4 — PR check adapter:** keep the CI wrapper under `examples/**` or `protocol/**`, preserving the
   Apache-2.0 boundary. Accept after 20 Corvint PRs carry the advisory check with bounded report bytes,
   zero network calls from the digest-pinned binary, and an independent consumer reproduces report
   bytes toward AT-17. Published digest-pinned 0.4.0 binaries are a hard dependency.

- The staging reference slice now includes `docs/CHANGE-EVIDENCE-MAP.md`, the `cem/0.1` schema,
  deterministic conformance vectors, strict local verifier, and `begin`/`cite`/`mark`/`verify`
  commands. The common evidence-backed path remains `begin`/`cite`/`verify`.
  Add the CI example and independent implementations before calling it interoperable.
- Bind every textual diff hunk to immutable receipt evidence, an explicit unknown, or a mechanically
  proven exception; re-verify maps at merge and report uncited hunks plus stable, relocated, ambiguous, stale, or deleted
  evidence without fuzzy re-anchoring.
- Implement only the Minimum Witness Compiler obligation types required for CEM: identity/freshness,
  authority, structural consequence, verification, and bounded negative scope. Retrieval may come
  from Corvint or any external producer and never determines the proposition verdict.
- Make clean installation, one CI command, first map creation/import, and the hunk report fit within
  15 minutes at median across trial repositories.
- Run the pre-registered 30 Beamfall lanes versus 30 controls experiment. Continue only if
  reviewer-found missed evidence falls at least 20%, at least 70% of material hunks are citable, and
  incorrect verifier hard failures stay below 5%.
- Require at least one independent non-Corvint producer and two independent consumers to pass the same
  `cem/0.1` fixtures; Corvint is a reference producer/verifier, not the mandatory retriever.
- Compute Merkle receipt diffs from changed blobs: added, removed, moved, dangling, contradicted,
  and stale-verification evidence.
- Stabilize the transport-neutral request/receipt SDK so later integrations do not create parallel
  evidence models.

Do not build an IDE, dashboard, hosted service, new agent, graph/vector database, universal repository
map, automatic editor, specification language, Jira/E2E adapter, or complete claim ledger for this
milestone. Kill or radically simplify CEM if it misses the evidence-reduction or citable-hunk gate.

## V6: Minimum Witness Compiler and claim-ledger prototype (target 2026-11-21)

**Useful outcome:** a bounded proposition compiles to typed proof obligations and returns the minimum
mechanically verified witness, a contradiction, or an explicit unknown frontier.

- Extract test claims from names, docstrings, table cases, assertions, and accepted specifications;
  answer capability questions with a proving claim at revision R or `UNKNOWN` plus nearest claims.
- Pilot anchored extraction for `test proves claim` and `decision governs symbol` edges on Beamfall
  plus two public repositories. Require independently labelled accepted-edge precision >=0.85 and
  critical-recall-at-five gain >=0.10; otherwise remove the model extractor.
- Route models only from a mechanically exhausted, verifier-eligible semantic frontier. Select the
  least-cost adequate class with frozen calibration for the exact schema; allow at most one bounded
  escalation after a named rejection. Require mechanical yield >=0.70, model calls <=20% of gap
  IDs (kill above 35%), >=50% fewer model-input tokens than model-everything, zero authority
  promotion, and zero calls where no deterministic admission verifier exists.
- Compare the claim-ledger unknown frontier with coverage percentage as a predictor of missed
  behavior. Continue only if generic claim precision is >=0.70 and the frontier performs better.
- Keep retrieval state (`READY`, `NEEDS_WIDENING`, `OUT_OF_SCOPE`) separate from proposition verdict
  (`PROVED`, `REFUTED`, `CONFLICTED`, `UNKNOWN`) in every protocol and evaluation.
- Add bounded negative-scope certificates that enumerate revision, roots, adapters, search semantics,
  exclusions, budgets, and unsupported surfaces; never translate them into global absence.
- Promote ranker-disagreement abstention only if its Brier score beats a single-ranker score-gap
  baseline by at least 0.05. Ranker confidence can route retrieval, never prove a proposition.
- Run deterministic spec-code-test drift checks in CI and pull requests using Merkle receipt diffs.
- Cover at least 20 repositories and three language families in the public benchmark.
- Document stable adapter and receipt extension contracts.

## Default language contract

"Supported by default" means an officially maintained adapter with frozen multi-repository cases
for symbols, dependencies, tests, impact, and abstention. It does not mean generic token search.

- V4 benchmarked adapters: Go, Python, JavaScript, and TypeScript. TSX and Markdown extraction remain
  alpha until their own multi-repository cases pass.
- V5-V6 adapters: HTML/template references; CSS, CSS Modules, Sass/SCSS, and Less; Java/Kotlin,
  C#/.NET, Rust, Ruby, and PHP.
- Next by measured demand: Swift, then C/C++.
- Other ecosystems use the public adapter contract until benchmark demand justifies core support.

TSX maps components, props, hooks, rendered children, routes, tests, and style/module references.
HTML maps templates to backend renderers/controllers and linked assets; it is not modeled as a
conventional import graph. Ruby and PHP adapters include framework conventions such as Rails and
Laravel rather than syntax alone. CSS-family adapters map imports, selectors, custom properties,
layers, design tokens, component/template usage, and visual-test impact without treating every
textual class-name match as proven runtime use.

Default evidence also spans the files that connect languages: SQL and ORM schemas/migrations;
GraphQL, OpenAPI, JSON Schema, and Protobuf contracts; dependency manifests and lockfiles;
Docker, Compose, Kubernetes, Terraform, CI, TOML/YAML/JSON configuration; shell, Make, and task
runners; routes, jobs, middleware, events, feature flags, and environment-variable references;
fixtures, snapshots, visual tests, localization, fonts, images, SVG, and design tokens. These become
typed adapters only when Corvint can prove cross-file relationships and uncertainty, not merely parse
the extension.

## Later, demand-gated

- **V7 — workflow distribution:** third-party adapter ecosystem, LSP/MCP/harness clients, and
  issue/specification imports over the same receipt contract. Ship maintained native integrations
  for Codex (plugin where supported; standalone skills, hooks, and MCP in the IDE), Claude Code (plugin, skills, hooks, MCP), Gemini CLI
  (extension, hooks, MCP), OpenCode (plugin events plus MCP), Pi, and the actual DeepSeek Harness
  Cordis plugin system, plus a paragraph-anchored Markdown/MkDocs renderer that
  can open a reviewable documentation pull request. Useful outcome: Corvint becomes the first context
  call without replacing the caller's editor, tracker, model, or specification/documentation tool.
  Each integration must pass install/discovery, revision identity, query/expansion, bounded context
  injection, observed-evidence/session-outcome capture, drift/frontier lifecycle, permission,
  failure-degradation, upgrade, and uninstall conformance tests. Generic MCP access is the portable
  baseline, not the bar for claiming full support. Harness-specific APIs remain in versioned
  adapters—especially OpenCode's beta plugin API—and never alter Corvint Core semantics.
  Publish support per `(host, surface, host version, adapter version, OS)` as `FULL`, `FALLBACK`, or
  `UNSUPPORTED`; a missing lifecycle capability, failed case, or untested release is `FALLBACK`.
  Keep adapters to native manifests/hooks around the shared Corvint CLI/MCP service with no
  adapter-owned knowledge store. OpenCode cannot reach `FULL` until a pinned version passes a safe
  frontier/continuation test.
  A first Codex/Claude Code/Gemini CLI/OpenCode native package slice and shared lifecycle command now
  exist as `FALLBACK` developer previews. V7 remains open: exact expansion, MCP, accepted Frontier
  authority, and black-box host matrices are not delivered.
  P1 MCP verb parity remains behind the Conditional-work outside-demand trigger: a consumer Corvint did
  not author must request it.
- **V8 — operational consequences:** explainable side effects and test selection, E2E journey and
  result evidence, incident capsules, and a security frontier that joins analyzer findings with
  entry points, trust boundaries, authorization/sanitizer paths, sensitive effects, proving tests,
  incidents, and explicit unexamined paths. Useful outcome: change, incident, and security queries
  return proven surfaces, checks, owners, runbooks, and explicit unknowns. Promote the security
  frontier only on unseen repository-scale vulnerabilities against an unguided-agent plus
  deterministic-analyzer baseline; Corvint complements CodeQL/Semgrep-class analyzers rather than
  replacing them.
  The same evidence substrate must support change-breakage analysis, missing-test discovery,
  selective test execution, onboarding, ticket-to-team routing, code review, inferred spec
  backfill, migration/deprecation planning, release narration, and incident orientation. PR update
  and exact-merge hooks maintain only affected knowledge; Corvint does not autonomously merge code or
  promote inferred specifications.
- **V9 — outcome-labelled context commons:** opt-in, privacy-screened receipt features and explicit
  pass/fail/reverted outcomes, promoted only when external-repository calibration beats a constant
  abstention model. No source content, raw private paths, prompts, secrets, or mutable ticket text is
  shared; public evidence may use hash-only selectors, while private identifiers are keyed and
  pseudonymous. This outcome-labelled dataset is the potential data moat, not a prerequisite for
  the local product.
- **V10 — mature receipt/CEM standardization:** evolve the already interoperable `cem/0.1` and
  receipt contracts with compatibility, reproducibility, neutral governance, and migration suites.
  Require at least two independent producers and three independent consumers with real use before
  claiming a standard; V10 is maturity, not the first external integration.

CEM V5 starts without pretending V4's broad retriever passed generalization: that sequence inversion
is deliberate. V4 work is limited to making the reference producer honest, deterministic, fast, and
replaceable. A proposed Core node, edge, model, service, database, UI, or integration must name the
user task, simpler baseline, frozen evaluation, resource budget, and kill criterion; otherwise it
stays outside Core.
