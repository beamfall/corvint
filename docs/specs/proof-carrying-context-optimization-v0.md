# Proof-Carrying Context Optimization V0

Owner: Russell Lewis
Frozen: 2026-08-23
Intent status: accepted (decision 0047, 2026-09-04)
Delivery status: not-started
Authoritative inputs: `docs/PRODUCT.md`, `docs/TECHNICAL-BRAIN.md`,
`docs/specs/agent-harness-integration-v0.md`,
`docs/specs/session-context-dividend-v0.md`, `docs/DOGFOOD.md`

## Agent digest
- Claim: Typed context may be minimized and survive compaction only through frozen outcome-tested proof-carrying trials.
- Status: accepted (decision 0047, 2026-09-04)/not-started
- Exists: contract for profile-guided minimization and proof-carrying compaction survival.
- Blocked on: accepted observation authority, frozen replay harness, qualified task classes, and outcome trials.
- Read next: Verified current state and prerequisites; Requirements; Promotion gate.

## User and measurable job

An agent should receive the smallest evidence set that preserves successful task completion, know
exactly what was omitted and why, and recover the still-live set after every compaction. Corvint should
learn this from explicit local outcome trials without turning historical correlation into repository
truth or a global mutable memory.

V0 combines two primitives:

1. **profile-guided context minimization**: offline counterfactual ablation finds an empirically
   minimal sufficient witness set for one frozen task class and environment; and
2. **proof-carrying compaction survival**: every visible compaction cycle emits a verifiable manifest
   of evidence retained, omitted, stale, unresolved, and rehydrated.

“Minimal sufficient” is always relative to the registered replay corpus, harness, model, host,
revision domain, and cost function. It is not a proof that the context is globally minimal, that an
omitted fact is irrelevant, or that context caused a stochastic model outcome.

## Why this is a new Corvint boundary

Current context compressors optimize size or summarize history. Corvint can instead optimize a typed,
revision-pinned evidence IR and preserve independently checkable omission and survival receipts. The
unit is not a transcript chunk: it is a witness handle with authority, freshness, obligation, and
criticality. This allows delta-debugging-style experiments without weakening the runtime proof
boundary.

The simpler baseline is the complete budgeted Corvint packet plus native host compaction. V0 earns a
place only if it improves complete cache-aware cost per solved task on held-out work while preserving
correctness and critical evidence.

## Verified current state and prerequisites

- Corvint emits bounded revision-pinned query/impact receipts with explicit critical omissions.
- Harness preview adapters expose compact-start recovery from exact tracked dirty paths and explicit
  untracked gaps. They do not observe model-internal context or prove task-state recovery.
- Codex currently exposes compaction items, thread token-usage updates, file/tool completion items,
  and turn completion through app-server. Corvint has not implemented or qualified an attested
  `HARNESS_OBSERVED` bridge.
- Session Context Dividend V0 is deferred and its required equal-tool outcome corpus is `NOT_RUN`.
  PCCO MUST NOT bypass that gate, infer success from exit status, or begin runtime learning first.

Implementation admission requires an accepted observation authority, a frozen replay harness, and at
least 20 eligible tasks across Corvint, Beamfall, and one unrelated repository. Until then this spec is
research direction only.

## Requirements

- `PCCO-V0-001`: An optimization trial MUST begin from one canonical Corvint packet whose exact bytes,
  receipt identity, repository tree, critical selectors, evidence handles, exclusions, host/model
  tuple, task-class key, tool policy, and byte budget are frozen. A packet with a critical omission,
  stale critical evidence, or an unresolved authority conflict is ineligible.
- `PCCO-V0-002`: V0 transformations are only whole-handle omission, exact duplicate elimination, and
  deterministic ordering. They MUST NOT summarize, rewrite, merge, synthesize, truncate, or promote
  evidence; change source spans; alter authority; or introduce model-generated context.
- `PCCO-V0-003`: Counterfactual search MUST run outside live agent sessions using bounded hierarchical
  delta debugging over typed groups, then individual handles. Every candidate is content-addressed
  and replayed from a clean isolated checkout. Search stops on its declared run/time/cost bound and
  reports `INCONCLUSIVE`; it never guesses the remaining minimum.
- `PCCO-V0-004`: A candidate is successful only with a sealed correctness oracle, zero critical
  evidence misses, all required `HARNESS_OBSERVED` verification passes, and complete observed token,
  cache, compaction, retry, tool, latency, and source-open measurements. Producer-reported success,
  test text, or process exit alone is insufficient.
- `PCCO-V0-005`: Each advisory profile MUST bind task-class key, repository identity domain, accepted
  instruction/spec digests, Corvint build/profile, host and adapter versions, model and reasoning
  configuration, tool policy, context-window policy, budget, replay-corpus identity, cost-function
  version, and promotion evidence. No key component may be wildcarded in V0.
- `PCCO-V0-006`: Promotion MUST use a preregistered train/validation/held-out split. Search sees only
  train tasks; candidate selection sees validation; the final claim is made once on held-out tasks.
  Repeated attempts, cherry-picking seeds, or editing labels after observation invalidates the run.
- `PCCO-V0-007`: Runtime serving MUST remain local, deterministic, bounded, read-only, and model-free.
  It intersects the eligible profile with the fresh canonical packet, never fabricates a missing
  handle, and falls back to the complete packet on key mismatch, expiry, conflict, drift, unsupported
  host state, or budget uncertainty.
- `PCCO-V0-008`: Every eligible host-visible compaction cycle MUST emit a survival manifest binding
  the pre-cycle packet, post-cycle visible packet, cycle ordinal, repository snapshot, and disjoint
  sorted sets `retained`, `omitted`, `stale`, `unresolved`, and `rehydrated`. Every pre-cycle critical
  selector MUST be retained or rehydrated; otherwise the cycle is `CRITICAL_SURVIVAL_FAILURE`.
- `PCCO-V0-009`: A survival manifest attests only Corvint adapter-visible evidence. Hidden reasoning,
  host-managed prompt cache, model attention, and transcript retention remain `NOT_OBSERVED`. Corvint
  MUST NOT claim that a supplied handle was read, remembered, or causally used by the model.
- `PCCO-V0-010`: An omission entry MUST carry the original handle, typed reason, profile identity,
  trial evidence identity, and recovery operation. Absence from the optimized packet is never a
  verified-absence claim about the repository.
- `PCCO-V0-011`: Profiles expire on any bound instruction/spec digest change, critical-selector
  change, evidence deletion or ambiguous relocation, tool/model/host policy change, failed replay,
  treatment-only miss, or owner revocation. Ordinary source drift may retain only independently
  reverified stable handles; it cannot refresh the learned omission claim.
- `PCCO-V0-012`: The primary metric is complete cache-aware cost per solved task, including every
  attempt, replayed history, cached and uncached input class, output/reasoning class, compaction,
  retry, tool, and verification cost until the sealed outcome. Raw observed units are immutable;
  provider pricing is a separately versioned offline projection. Token reduction alone cannot pass.
- `PCCO-V0-013`: Training artifacts remain repository-local, owner-readable, secret-screened,
  bounded, and deletable. No raw prompt, transcript, source body, private path, model output, tool
  body, or credential enters a profile. V0 has no daemon, database, upload, federation, or shared
  team memory.
- `PCCO-V0-014`: Profiles are advisory performance artifacts, never project authority, accepted
  intent, behavioral claims, test evidence, CEM/OCM witnesses, or Frontier-closing relations.

## Promotion gate

The held-out gate contains at least 30 eligible tasks: ten Corvint, ten Beamfall, and ten from an
unrelated repository, with at least three task classes per repository. Compare against all of:

1. the complete budgeted Corvint packet;
2. deterministic lexical top-k at the same byte budget; and
3. native host compaction without a Corvint profile.

Promotion requires non-inferior sealed correctness, zero treatment-only critical misses, zero
critical survival failures across at least three compaction cycles per eligible task, at least 25%
lower median complete cache-aware cost per solved task than the strongest baseline, at least 20%
lower p75, no p95 task-latency regression above 10%, and at least 80% of eligible tasks completing
without a broad repository rescan. Report every ineligible and failed task outside the denominator.

Kill profile-guided minimization if the 30-task gate saves less than 10%, increases retries or broad
searches, or produces one treatment-only critical miss. Retain survival manifests only if they catch
at least one real compaction loss or reduce recovery time in the trial; otherwise keep exact dirty-
path rehydration and remove the extra protocol.

## Research boundary

- ACE motivates incremental, outcome-evaluated playbook evolution while warning about context
  collapse: `https://arxiv.org/abs/2510.04618`.
- SWE-Pruner motivates task-aware pruning and reports that retrieval/summarization baselines can
  underperform direct structural pruning: `https://arxiv.org/abs/2601.16746`.
- Governed Shared Memory identifies stale propagation, contradiction persistence, provenance
  collapse, and scope leakage as memory risks: `https://arxiv.org/abs/2606.24535`.
- Code Compression Bench treats cache-aware cost per solved task, rather than headline token shrink,
  as the comparison boundary: `https://github.com/daseinlabs/code-compression-bench`.
- CompactBench motivates repeated compaction-cycle drift testing:
  `https://github.com/compactbench/compactbench`.

These are research leads, not Corvint performance evidence. PCCO's gate remains independently observed
local dogfood and held-out evaluation.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| `PCCO-V0-001..014` | not started | observation authority, frozen corpus, hostile manifest vectors, three-baseline held-out trial |
