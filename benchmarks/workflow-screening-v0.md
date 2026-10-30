# Workflow screening protocol and contamination-exclusion registry (pre-AT-08)

AT-08 ("Evaluate the first complete daily workflow", `ROADMAP.md`) depends on AT-01, AT-02, AT-03,
AT-05, AT-06, and AT-07. AT-05/06/07 are not yet built, so AT-08's own unseen freeze cannot exist
yet: there is no candidate arm surface to freeze it against. The only thing defensible to freeze
now is (1) a screening protocol for what a pre-freeze comparison may run and report, and (2) a
contamination-exclusion registry (`benchmarks/workflow-screening-v0.json`) naming every existing
task corpus and stating whether it may ever become an AT-08 unseen sample. This document does not
freeze AT-08's task list, does not run anything, and does not shorten AT-08's dependency chain.

Pinned commit: `01aa66ad071756f7308bb04b0ec379b051a231e3`. This document extends
`benchmarks/workflow-baseline-v0.md` (task-class definitions WB-classes, arm definitions, metrics,
and scorer rules `WB-001..010`) rather than restating it; read that document first.

## 1. Task-ingredient coverage by class

- **investigate** and **change-review** have existing corpus ingredients: CW `trace2code`
  retrieval and blind `query`/`impact` cases supply investigate-shaped retrieval/consequence
  questions; CW `code2test`/`edit2ripple`/`comment2context`/co-change tasks and CEM source-hunk
  patches supply change-review-shaped change/evidence tasks.
- **start** and **resume** have **no** complete existing task source in any corpus listed in
  Section 2. Start has scattered question/repository seeds but no onboarding/ownership/first-action
  task with a labelled answer; resume has no interruption/working-set/checkpoint episode source at
  all. Both need wholly new task construction and labels before AT-08 can freeze them.
- Ingredient mapping is an inference about reusable *methodology*, never an existing-corpus
  qualification: every corpus in Section 2 is excluded from direct reuse as an AT-08 sample.

## 2. Contamination-exclusion registry

`benchmarks/workflow-screening-v0.json` lists ten existing corpora (cw-trial pilot, heldout-v1,
unseen-corvint-v1/v2, unseen-beamfall, unseen-beamfall-apple, cem-trial pilot, blind-v2/v3/v4), each
with path, task count, repositories, the partition label the file itself carries, an
`status_for_at08` of `development`, `reserved-release-evidence`, `excluded-tuned`, or `quarantined-exposure`, a one-sentence
cited reason, and whether its task-generation method (not its specific cases) may inform new AT-08
task construction (`reusable_as: sampling-template`) or may not be drawn on at all
(`reusable_as: none`, blind-v4 only). None is `available` for direct AT-08 sampling.

On 2026-09-06 the audit-follow-up reviewer crossed the V4 boundary while preparing trial inputs
and disclosed material from the reserved Zod, Flask and Cobra files. Those proposed tasks are
discarded. The original corpus and manifest remain immutable; the registry quarantines V4 for this
task, and no timing, correctness, development or sealed-release claim may reuse the exposed input.
An independent replacement or owner-approved exposure adjudication is required before a future
sealed-release claim. A null historical observation field does not undo this exposure.

## 3. Screening rules

Each rule is one sentence, `MUST` wording, preregistered before any screening run.

- **WS-001**: A held-out task whose result is used to guide an implementation or regression-repair
  change is development forever; renaming, rephrasing, or moving its partition label does not
  reverse this, and a held-out case's first run is otherwise preserved (`WB-007`;
  `benchmarks/README.md:17-19` first-observation rule). Separately, every corpus in Section 2 is
  excluded from ever serving as a fresh AT-08 unseen sample regardless of that rule, because each
  one is already exposed through prior development, tuning, or challenge use (`WS-007`).
- **WS-002**: A screening run MAY report per-class completion, critical misses, all-worker cost via
  `benchmarks/dogfood_workers.py` receipts (its `METRIC_FIELDS` and `_totals` fields), and warm
  first-packet latency separately from expansion latency (`BRAIN-DOG-015`).
- **WS-003**: A screening run MUST NOT claim promotion, a complete-cost savings figure
  (`WB-004`/`WB-005`), or satisfaction of any BRAIN-DOG, AHI, or PCCO gate; each of those gates has
  its own eligibility (BRAIN-DOG's workflow-specific minima and ten-workflow suite denominator;
  AHI's host/version/event-specific qualification; PCCO's own ≥30-task, three-repository,
  three-baseline, ≥3-compaction-cycle design) that the four AT-08 classes do not satisfy merely by
  existing.
- **WS-004** (arm pins, summarised from `benchmarks/workflow-baseline-v0.md:53-88`): Arm
  A0 admits only `query`, `context`, and `impact` at pinned commit
  `01aa66ad071756f7308bb04b0ec379b051a231e3`; `batch` is not part of arm A until its spec is
  admitted. Arm B is unrestricted native tools (read, grep/find, run tests, edit) plus the
  nine-field structured notes template: `intent`, `handles`, `decisions`, `progress`, `outcome`,
  `nextAction`, `unknowns`, `failedApproaches`, `commandsRun`. Arm C is whatever AT-05/06/07 deliver
  by run time, candidate-only, not frozen here.
- **WS-005**: Any AT-08 unseen freeze is gated on the sealed-label rule in
  `docs/specs/technical-brain-dogfood.md:90-108`: after the interface and evaluated Corvint build are
  frozen, an independent labeller creates a held-out manifest with two hash-bound projections —
  `query.json`, revealed identically to both arms before either runs, and `oracle.json`, sealed
  until outputs lock. Before either arm runs, the labeller publishes a `commitment.json` carrying
  the query digest and `SHA-256(32-byte random salt || canonical oracle bytes)`, while the producer
  cannot access the oracle or salt; only `oracle.json` and the salt are revealed, after outputs
  lock.
- **WS-006**: The minimum ablation cells for arm C are full C (batch on, widening on, checkpoint on)
  plus each of batch, widening, and checkpoint off individually, holding the other settings fixed;
  every one of these cells is `NOT_RUN` until AT-06 and AT-07 land, and this document activates none
  of them.
- **WS-007**: Every corpus in Section 2 is excluded from serving as an AT-08 unseen sample; only its
  cited task-generation methodology, never its specific cases, may inform new task construction,
  except blind-v4, whose `reusable_as` is `none` because it is sealed release evidence under
  `docs/decisions/0037-release-owner-calls-2026-09-03.md` items 3-4.
- **WS-008**: A screening report MUST publish the all-attempt and paired tables side by side
  (`WB-006`) and, for a class or arm with zero solved tasks, MUST leave every per-solved ratio
  undefined while still reporting completion and the other defined rates (`WB-003`); these
  baseline rules apply unchanged to any comparison run under this screening document.

## 4. Owner decisions still open

- **Solved thresholds** for `start`/`investigate`/`resume`/`change-review` remain `OWNER DECISION`
  (`benchmarks/workflow-baseline-v0.md` Section 1); this document adds no threshold.
- **n per class**: how many tasks per class a screening or eventual powered run requires is open.
- **Repeats per task**: whether and how many repeated attempts per task are drawn is open.
- **Paired statistical test**: which paired test (e.g. a paired bootstrap or sign test analogous to
  `CTR-V0-006`'s clustered analysis) applies to a screening or powered comparison is open.
- **Independent labeller identity**: who plays the sealed-label role in `WS-005` for AT-08's own
  freeze — a specific person, role, or process distinct from every builder — is open.
- **Same-model-author exclusion**: whether and how AT-08 excludes a worker/reviewer pairing where
  the same model authored and later judged a response is open; neither `heldout-v1` (labelled only
  "agent without Corvint access") nor `blind-v4` (independent sessions, no model IDs recorded) settles
  this by precedent.

## 5. Not claimed

This document does not freeze AT-08's unseen task list, does not run any comparison, does not
establish a solved threshold, sample size, or statistical test for any of the four classes, does
not reduce or reinterpret AT-08's stated dependency on AT-01/02/03/05/06/07, and does not certify
that `investigate` or `change-review` are ready to sample merely because an ingredient mapping
exists in Section 1 — that mapping identifies reusable methodology, not an approved task source.
No corpus in `benchmarks/workflow-screening-v0.json` is available for direct reuse, and no run
under this protocol may be reported as anything but screening-only, per `WB-003` and `WB-010`.

## 6. Portal diagnostic preparation (2026-09-08; proposed, NOT_RUN)

This addendum prepares the existing AT-08 work; it accepts no sample size, outcome threshold,
measurement amendment or campaign spend. The introduction's “not yet built” refers to the historical
pin, not current executable availability. AT-05/06/07 now have experimental implementations at
`2f843df329afb52ebef9516721ead77bec2cf491`; their outcome gates remain open. A0 stays at
`01aa66ad071756f7308bb04b0ec379b051a231e3`; a newer installed binary never substitutes for it.

Proposed development screen: eight newly authored synthetic scenarios, two per existing class.
Before execution, freeze each scenario's input bytes/digest, exact candidate build and source tree,
expected evidence/unknowns, permitted tools and stop condition. The following expectations guide
fixture construction only; no scenario or outcome has run and none can become unseen evidence.

| Class | Positive development expectation | Negative development expectation |
|---|---|---|
| start | Exact governing requirement, revision handle and safe first action are present. | Missing/conflicting authority stays unresolved; no invented first-action approval. |
| investigate | A known static dependency and its original test/spec handle are recovered. | Unsupported/dynamic scope is named; no exhaustive consequence verdict. |
| resume | Explicit checkpoint retains obligations and exact evidence for a fresh agent. | Changed/deleted authority invalidates its handle; unknown possession causes resend. |
| change-review | Exact diff, owning requirement and actual verification result remain distinguishable. | Wrong-revision or absent evidence stays missing; no false complete or passed verdict. |

Keep model, effort, tools, access, prompts, budgets and normal cache handling equal across A0,
native tools plus the existing nine-field notes, and candidate. Charge installation/index/setup,
coordinator, builder, reviewer, repair, retries and every failed/cancelled attempt; elapsed wall time
is separate from summed worker time. Missing observations stay `NOT_OBSERVED`. Owner-selected
outcome definitions, independent labels, repetitions, uncertainty/stopping rules and authorized
complete host inputs remain prerequisites to any AT-08 outcome campaign. The development screen
never replaces WS-005's fresh sealed-label construction or WS-006's required ablations.

## 7. U4 external timing protocol (proposal only, NOT_RUN)

Owner: existing LAC-V0 U4. Proposed comparison is one human operator using the opt-in console versus
the same operator using both owning CLIs plus `corvint-dashboard-snapshot`, with equal repository,
ticket data, source access and original evidence. No console telemetry, host-session discovery,
background capture or agent-browser timing stands in for human operator time. Query/impact axes
stay unqualified; the console displays only source-supplied values and explicit unknowns.

Proposed design for owner selection: eight matched pairs of independently authored equivalent
journeys (two pairs each: ticket-to-requirement, revision-to-evidence, missing evidence, wrong
revision). Each pair uses distinct instances to limit memorization; four pairs run console then
CLI, four CLI then console, counterbalanced within each category. One operator performs sixteen
trials, with no discretionary repeats. Freeze order, task prompts, exact tool/build/source digests,
expected source identities and success criteria before the first timed task. Retain every failed,
abandoned or technically interrupted trial; any replacement rule requires a prior amendment.

Proposed instrument: an explicit local CSV/JSON input authored by the observer, containing pair,
category, arm, order, candidate and data digests, operator pseudonym, start/stop monotonic times,
terminal disposition, cited identities, independent correctness/false-complete labels and reason.
Time starts when the prompt is revealed and ends at the submitted answer or a proposed ten-minute
cap. Record setup-to-first-use time separately and include it in the all-assigned total. A missing
terminal timestamp invalidates that pair for paired latency, but retains its attempt and elapsed
observations in the all-attempt table. The CLI arm may use ordinary shell tools and notes.

Proposed descriptive success rule: all sixteen trials return correct source identity and status,
zero false-complete or false-pass answers, and at least 20% lower median paired completion time
for the console. Report raw pairs, failures, all-attempt time, setup, median paired ratio and range;
with eight pairs do not claim a qualified p95 or population benefit. Report a single-operator
engineering observation only. A failure triggers journey-specific diagnosis, not interface expansion.
Task count, effect rule, timeout, repeat policy, observer/operator identities and this protocol all
remain unaccepted owner choices; U4 remains open until those choices and actual measurements exist.
