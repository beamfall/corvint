# Applied Intelligence Breakthroughs V0

Owner: Russell Lewis
Frozen: 2026-08-23
Intent status: accepted (decision 0047, 2026-09-04)
Delivery status: not-started
Document kind: hypothesis catalog (prose intent only; no numbered requirement clauses, so OCM
must abstain rather than invent coverage). Decision 0179 (2026-09-13) holds this catalog
prose-only permanently; a hypothesis gains `PREFIX-NNN` clauses only via a separate accepting
slice's own requirements, never by relabelling this prose.
Authoritative inputs: `docs/PRODUCT.md`, `docs/TECHNICAL-BRAIN.md`, `ROADMAP.md`,
`docs/specs/context-evolution-program-v0.md`, `docs/DOGFOOD.md`

## Agent digest
- Claim: Eleven application hypotheses must beat frozen external baselines before any can become a Corvint product claim.
- Status: accepted (decision 0047, 2026-09-04)/not-started
- Exists: nothing — eleven hypotheses with baselines, gates, and kill criteria only.
- Blocked on: frozen external outcome trials.
- Read next: `context-evolution-program-v0.md` and `../BREAKTHROUGH-CORVINT.md`.

The broader research portfolio is an internal research catalogue (not part of the public tree). This spec
selects eleven application hypotheses from that open catalogue; it is not the complete opportunity
map.

## Product thesis

Corvint should not compete by producing more unverified prose or code. Its unfair advantage is the
local proof substrate: immutable evidence handles, project authority, change maps, explicit unknown
frontiers, harness observations, drift, and independently replayable receipts. The eleven candidates
below turn that substrate into applications for coding, debugging, research, testing, review,
documentation, maintenance, and operations.

They are possible breakthroughs, not roadmap commitments. Each must beat its simpler baseline on a
frozen external trial before becoming Core. Generative output is always a candidate; mechanically
verified relations and human-owned policy retain authority.

## 1. Causal Change and Debugging Frontier

**Job.** Before editing—or while debugging—answer: what can this change break, what observed failure
can reach this code, and what remains unexamined?

**Corvint move.** Compose changed hunks, contracts, call/import/data boundaries, configuration, tests,
E2E journeys, owners, incidents, and runtime observations into `AFFECTED|NOT_AFFECTED_IN_SCOPE|UNKNOWN`
items. Every causal edge has a pinned witness; every absence is bounded by the inspected universe.

**Baseline.** Repository search plus an LLM explanation.

**Gate.** On 200 sealed historical changes and 50 historical defects across ten repositories,
require at least 95% critical affected-surface recall, 80% precision, zero global-absence claims,
all seeded contradictions visible, and at least 30% lower median time to the first correct debugging
hypothesis than the baseline. Kill whole-program causality if critical recall is below 90%; retain
the bounded impact frontier.

## 2. Missing-Test Obligation Compiler

**Job.** Find behavior that a change or specification requires but no accepted test claim witnesses,
then write the smallest non-tautological candidate test.

**Corvint move.** Compile intent and change obligations against qualified test claims. Generation is
allowed only for an exact unknown obligation; the candidate must fail before the fix or kill a
registered mutation, pass after the fix, and remain linked to the requirement and changed behavior.

**Baseline.** Coverage-guided or prompt-only test generation.

**Gate.** On 100 seeded and historical bugs, require at least 70% useful-test yield, 85% generated-
test precision, at least 20% more real defects found than coverage-only generation, zero tests that
pass both buggy and fixed revisions while claiming closure, and median review under five minutes.
Kill automatic writing below 50% useful yield; retain missing-obligation reports.

## 3. Minimum Verification Plan

**Job.** Run only the tests and checks a change needs without silently missing a failure.

**Corvint move.** Select tests from structural consequence, requirement/test claims, historical failure,
configuration, E2E journeys, and changed contracts. The plan carries a witness per selected check and
an explicit unknown frontier; low confidence widens to package or full gates.

**Baseline.** Full CI and path-name heuristics.

**Gate.** Shadow 1,000 real or replayed changes across ten repositories. Require zero missed required
failures relative to full CI, at least 50% lower median test wall time and compute, no more than 5%
false full-suite widening, deterministic selection, and calibrated abstention. One missed release-
blocking failure disables autonomous selection.

## 4. Living Behavioral Spec and Human Documentation Compiler

**Job.** Write a reviewable specification from what the code and tests demonstrably do, keep MkDocs
human documentation current, and explain what shipped.

**Corvint move.** Compile `SUPPORTED|CONFLICTED|UNKNOWN` behavioral clauses from accepted intent, code,
tests, public contracts, examples, and change evidence. Generate paragraph-anchored Markdown with
evidence footers, uncertainty blocks, migration notes, and release deltas. Publish only to a branch
or draft PR after explicit repository policy permits the outward action.

**Baseline.** LLM code summarization or template documentation generation.

**Gate.** Across 100 modules and 50 shipped changes, independent maintainers require at least 90%
clause correctness, 100% evidence resolvability, zero fabricated behavior, at least 80% acceptance
with minor edits, all injected code/spec contradictions shown, and at least 40% lower documentation
latency. Any fabricated public contract blocks automatic PR creation.

## 5. Autonomous E2E Journey Compiler

**Job.** Discover critical user journeys, generate executable E2E tests, find regressions, and repair
test mechanics without rewriting product intent.

**Corvint move.** Join routes/screens, API contracts, state transitions, accessibility identifiers,
accepted journeys, fixtures, and prior failures into a typed journey obligation. A browser/device
agent explores only the unknown steps; Corvint records state/action/oracle evidence and distinguishes
product regressions from selector/environment drift.

**Baseline.** Prompt-generated Playwright/Appium scripts and checklist-based browser agents.

**Gate.** On WebTestBench plus 50 repository-owned web/mobile journeys, require at least 85% valid-
test rate, 80% seeded-defect recall, under 10% false defect reports, under 2% flake across ten runs,
100% oracle provenance, and at least 50% lower authoring time. Self-repair may change mechanics only;
one silent oracle weakening disables it.

## 6. Authority-Aware Ticket and Reviewer Router

**Job.** Route a ticket or change to the team and reviewers who actually own its consequences, with
reasons and uncertainty.

**Corvint move.** Compose declared ownership, architecture boundaries, code history, runbooks,
contracts, active changes, and blast radius. Policy/ownership evidence outranks commit frequency;
conflicting or missing ownership produces a multi-owner or abstaining result.

**Baseline.** CODEOWNERS alone, keyword routing, or author-frequency recommendation.

**Gate.** On 2,000 historical tickets/PRs across ten repositories, require 80% top-1 and 95% top-3
team recall, 90% critical-domain reviewer recall, calibrated abstention better than always-routing,
zero unauthorized identity disclosure, and at least 30% lower reassignment latency. Below 90% top-3
critical recall, remain advisory.

## 7. Proof-Carrying PR Steward and Safe Merge Loop

**Job.** Review a PR against intent, keep it current while its base moves, repair only evidenced
drift, and merge automatically when repository policy says every required frontier is closed.

**Corvint move.** Bind each hunk to CEM/OCM evidence; detect stale base, duplicate resolution, missing
tests, contract drift, reviewer findings, and required checks; rebase/reverify through an isolated
leaf; and produce one merge authorization receipt. Corvint never invents approval and never bypasses
protected branches or human/policy holds.

**Baseline.** Green CI plus generic AI review and auto-merge.

**Gate.** Shadow 500 historical PRs, then run 100 policy-eligible low-risk PRs. Require at least 30%
fewer reviewer-found critical misses, 25% lower ready-to-merge latency, zero duplicate/already-fixed
merges, zero merge with stale evidence or required-check failure, and zero policy bypass. One unsafe
merge permanently returns the feature to recommendation-only mode.

## 8. Removal and Migration Proof Planner

**Job.** Plan and execute a deprecation, API migration, schema evolution, dependency replacement, or
feature removal without leaving consumers behind.

**Corvint move.** Compile the target's producers, consumers, contracts, version boundaries, data and
configuration dependencies, owners, tests, docs, rollout stages, compatibility windows, and rollback
frontier. Generate transformation candidates only where a mechanical verifier can admit them.

**Baseline.** Search for names, write a checklist, and run the full suite.

**Gate.** Replay 50 completed migrations/removals and prospectively shadow ten. Require 100% recall
of independently labelled critical consumers, at least 85% precision, zero premature removal,
complete rollback evidence, and at least 30% lower planning/review time. Any missed external/wire
consumer blocks autonomous edits.

## 9. Evidence-Bound Research, Onboarding, and Incident Capsule

**Job.** Get a new engineer productive, answer an engineering research question, or orient an
incident responder without forcing a broad repository archaeology pass.

**Corvint move.** Compile a time-bounded capsule of governing terminology, architecture, owners,
runbooks, recent relevant changes, known incidents, live unknowns, and primary external sources.
Separate repository truth, runtime observation, external research, and inference; preserve source
dates and revision identities; widen on contradiction or staleness.

**Baseline.** README/search, hand-written onboarding, or an unconstrained research agent.

**Gate.** Run 30 onboarding tasks, 30 source-backed research questions, and 20 incident drills.
Require non-inferior answer/action correctness, 100% citation resolvability, zero authority mixing,
at least 40% lower time to first correct action, at least 50% fewer broad searches, and all injected
stale runbooks or contradictory sources surfaced. One hidden critical contradiction blocks default
injection.

## 10. Self-Discovering Agent Harness Fabric

**Job.** Make Corvint automatically discoverable and safely updateable across coding agents, including
a native Pi extension and DeepSeek Harness Cordis plugin, without forking semantics per vendor.

**Corvint move.** Ship signed/versioned compatibility manifests and thin native packages over the
Portable Snapshot Context Capsule. Discovery reports exact host/surface/version/OS support; updates
stage conformance before activation, retain rollback, require policy approval where the host does,
and never silently promote `FALLBACK` to `FULL`.

**Baseline.** Manual CLI/MCP setup and independently maintained vendor plugins.

**Gate.** For Codex, Claude Code, Gemini CLI, OpenCode, Pi, and DeepSeek Harness, test clean discovery,
install, first query, update, downgrade, disable, uninstall, malformed/revoked manifest, offline mode,
permission denial, and host-version skew on every claimed tuple. Require 100% lifecycle pass, no
semantic divergence from the shared golden, no unauthorized network/write, rollback after every
failed update, and zero false compatibility claims. Remove auto-update for any host that cannot stage
and roll back atomically; retain manual installation.

## 11. Live Proof-Carrying Verification Plane

**Job.** Give engineers and coding agents immediate affected-test, coverage, runtime-value, and
debugging feedback for the exact unsaved or committed source state while proving which obligations
were checked, what was skipped, and when a result became stale.

**Corvint move.** Join a provider-neutral execution protocol to immutable repository identity,
editor-overlay identity, obligation-selected checks, runtime witnesses, bounded exclusion reasons,
and fail-closed widening. IDEs and agents consume the same replayable result graph; providers retain
language-specific execution authority and Corvint never treats coverage or one observed path as
universal behavior.

**Baseline.** A Wallaby-class continuous test runner plus separate affected-test selection, AI code
review, and full CI.

**Gate.** Shadow 1,000 changes across independent Python, Go, and JavaScript providers. Require zero
stale-result display after any source-identity transition, zero release-blocking failure missed
against repository-mandatory full CI, at least 50% lower median verification compute, sub-second
affected-result feedback on the registered local corpus, exact receipt replay, and no source or
runtime-value disclosure outside the local policy boundary. Any stale display or required-failure
miss disables selective mode and widens to the project gate.

## Research leads and competitive warning

- WebTestBench reports major remaining gaps in completeness, defect detection, and long-horizon E2E
  reliability: `https://arxiv.org/abs/2603.25226`.
- Formal intermediate specifications have shown promise where direct test generation missed a real
  constraint defect, but the evidence is still small: `https://arxiv.org/abs/2607.18555`.
- A broader formal-spec extraction study reports oversimplification and fabrication as core failure
  modes: `https://arxiv.org/abs/2504.01294`.
- An empirical study of 8,106 agent-involved fix PRs identifies test failures and already-resolved
  issues among the leading non-merge reasons: `https://arxiv.org/abs/2602.00164`.
- Submission-time PR acceptance prediction can support triage but should remain advisory:
  `https://arxiv.org/abs/2607.12057`.
- Wallaby.js documents affected-test execution, inline coverage/errors, runtime values, execution
  paths, time-travel debugging, Smart Start, and AI/MCP integration; these are incumbent capabilities,
  not Corvint novelty: `https://wallabyjs.com/`, `https://wallabyjs.com/docs/features/ai/`, and
  `https://wallabyjs.com/docs/features/smart-start.html`.

The opportunity is not to duplicate these generators and predictors. Corvint should make their inputs,
unknowns, transformations, verification, and lifecycle independently inspectable.

## Traceability

| Candidate | Delivery | First required artifact |
|---|---:|---|
| Causal Change and Debugging Frontier | not-started | sealed historical change/defect corpus |
| Missing-Test Obligation Compiler | not-started | bug/mutation obligation corpus |
| Minimum Verification Plan | not-started | 1,000-change full-CI shadow log |
| Living Behavioral Spec and Human Documentation Compiler | not-started | evidence-anchored MkDocs draft corpus |
| Autonomous E2E Journey Compiler | not-started | WebTestBench plus repository journey adapter |
| Authority-Aware Ticket and Reviewer Router | not-started | ownership-labelled ticket/PR corpus |
| Proof-Carrying PR Steward and Safe Merge Loop | not-started | 500-PR shadow review |
| Removal and Migration Proof Planner | not-started | 50 completed migration replays |
| Evidence-Bound Research, Onboarding, and Incident Capsule | not-started | onboarding/research/incident trial pack |
| Self-Discovering Agent Harness Fabric | not-started | Pi and DeepSeek package discovery contracts |
| Live Proof-Carrying Verification Plane | not-started | provider protocol plus 1,000-change cross-language shadow corpus |
