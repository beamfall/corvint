# Decision 0003: agent-essentialness resequencing and evidence-gated retrieval quality

- Status: accepted direction
- Owner: Russell Lewis
- Date: 2026-08-27
- Inputs: five-expert strategy panel (product strategy, competitive, LLM-agent ergonomics,
  eval conformance, performance), 2026-08-25..27 Beamfall dogfood runtime evidence, decision
  `0001`, `../specs/live-proof-carrying-verification-v0.md`,
  `../specs/agent-harness-integration-v0.md`, `../specs/context-evolution-program-v0.md`,
  `../specs/lexical-relevance-floor-v0.md`

## Context

Live dogfood inside the Beamfall fleet (58 runtime invocations, 23 real ticket auto-routes) shows
the routing, exact-revision identity, typed fallback, and latency floors working, while falsifying
the two claims that carry the product category: ranked-context relevance (a tooling ticket's top
citations were two generic docs plus an unrelated test-fixture README; zero subject files) and
abstention (an out-of-scope task confidently top-ranked an unrelated feature on the token "error";
abstention never fired). Competitive review finds proof-carrying partial reads, verified citations
with contractual abstention, and audit-ready context provenance to be unoccupied ground mapped to
measured buyer pain — and finds that proofs over mediocre citations lose to unverified-but-relevant
substitutes. The panel's convergent conclusion: essentialness is decided in the agent's daily loop
first, and the loop is not yet won.

## Decision

1. **Retrieval quality and abstention are the gate for every outward bet.** No public agent
   surface, cloud phase, or marketing claim ships before the harvested-corpus retrieval gates pass:
   critical-recall@5 at least +0.10 over the static class-packet baseline (macro-averaged, CI-bound);
   dual-sided abstention (recall 1.0 on out-of-scope probes AND false-positive rate <= 0.02 on
   in-scope queries, one gate, so over-abstention cannot game it); keyword-bait top-5 hit rate
   <= 0.05 absent structural agreement. Eval corpora are harvested, not hand-labeled: a landed
   change's diff is the retrospective relevance judgment; labels are path/span digests; task text
   never leaves the lane. Every retrieval gate lands with a demonstrated-red fixture (the two
   observed dogfood failures are the first two) and a digest-enforced `EVAL_CONTAMINATED` fence
   between ranker training manifests and eval tickets. Stop-loss: if the subject-file citation rate
   does not beat the static packet within ~6-8 weeks of the harness existing, outward phases halt
   in favor of retrieval work.

2. **The dirty-worktree overlay is promoted from deferral to product-critical.** Agents query
   hardest mid-edit; a clean-commit-only index is stale exactly then, and the amortization story of
   5-30 calls per task dies at call 1 (the dogfood's routes are all clean-HEAD orientation calls —
   consistent with this failure mode, not evidence against it). This also becomes a hard dependency
   of the operator's live-verification target: a language-agnostic Wallaby-class experience that
   agents consume as well as humans — live affected-test results, values, and coverage on
   uncommitted edits, delivered as structured branchable receipts
   (`live-proof-carrying-verification-v0.md` is the normative home; overlay identity is its
   prerequisite). First step is measurement, not implementation: instrument dogfood routes for two
   weeks to record dirty-at-query-time rate before freezing overlay scope.

3. **Deferred until external pull exists:** the full cloud data + sync/control planes. The
   proof-carrying remote-read breakthrough is demonstrated first with a statically hosted immutable
   index (object store or CI artifact) plus local Merkle verification — the same canonical bytes,
   roughly a tenth of the surface. The vector challenger stays parked: the observed quality gap is
   lexical/structural relevance and abstention, which vectors do not address and competitors already
   commoditize. Identity gates right-size to 100K randomized schedules until the wedge gates pass.

4. **Performance targets are amended to gate what agents experience.** In addition to the kernel
   targets of decision `0001`'s program: end-to-end routed call <= 1s p95 over real routes (dogfood
   shows the wrapper at 3-10x the kernel budget); snapshot-drift rebuild never blocks a query
   (serve stale-with-lag-receipt, rebuild async); fresh-process open splits into warm-page-cache
   (<= 100ms) and true-cold (<= 500ms) so cache state is never hidden; first useful query on a
   pinned 10 GiB polyglot corpus <= 60s from build start with honest partial-coverage receipts;
   query RSS gains a 1M-file clause (<= 256 MiB). Continuous perf gating uses deterministic
   counters (bytes read, block touches, allocations, tokens emitted) as the primary ratchet;
   wall-time gates run only on quiet dedicated runners with bootstrap CIs and a typed
   `insufficient_evidence` verdict — laptop wall-time gating is rejected (demonstrated red on noise
   in dogfood, 2026-08-27).

5. **The receipt/citation contract is published as an open specification** (Apache-licensed
   protocol portions per decision `0002`) once it survives dogfood, with the engine as reference
   verifier — pursued as the neutral cross-platform standard before platform vendors make context
   provenance proprietary. Kill criterion: no external emitters/verifiers after two quarters of
   active outreach demotes it to an internal format.

## Consequences

Near-term effort concentrates on: the eval harness and its corpora; relevance/abstention work
gated by it; overlay measurement then design; the three-tool progressive agent surface with
machine-branchable verdicts (`CONFIDENT | LOW_EVIDENCE | OUT_OF_SCOPE | STALE` + `suggested_next`)
under `agent-harness-integration-v0.md`; and the review-pinned CI reviewer as the first sticky
integration. Existing contracts retain their delivery truth; conflicts with prior sequencing are
resolved by this record rather than silent scope change. Beamfall-side execution flows through the
Beamfall roadmap (eval-corpus harvesting, abstention gates, dirty-rate instrumentation tickets);
Corvint-side execution flows through the specs named above.
