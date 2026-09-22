# Decision 0064 — Add-on portfolio disposition and the release positioning falsifier

Date: 2026-09-05. Status: accepted. Authority: repository owner, delegated call; the owner instructed on 2026-09-05 that open calls be resolved through the expert panel, and this record applies the panel memo `docs/reviews/panels/panel-7-roadmap.md` after independent review of its cited evidence.

## Context

`docs/reviews/b3-addons.md` proposes eleven add-ons toward "the only tool an LLM coding agent
needs". The active queue is already selected, the V4 generalisation gate is failed, and a first
release tag is being prepared under `docs/reviews/r8-release.md` section 3.

## Decision

1. No add-on enters the active queue before the 0.4.0 tag. The session capsule (P2) is scope of
   AT-06 and AT-07 and the cost receipt (P7) is scope of AT-01, explicit host-log input only;
   automatic reading of host session directories stays a non-goal.
2. Post-tag under V5, in order: the "what shipped" narrator (P9) as the first HDC emitter, accepted
   by regenerating the 0.4.0 notes from merged CEM and OCM and diffing against the hand-written file;
   then the PR check adapter (P4), whose CI wrapper lives under `examples/**` or `protocol/**` so the
   Apache-2.0 boundary does not widen silently.
3. MCP verb parity (P1) belongs to V7 behind the existing demand trigger, which forbids inferring
   demand from self-authored consumers. The SCIP consumer (P11) becomes a Conditional-work row shaped
   as a pinned external unit graph and does not authorise a first `go.mod` dependency. Tracker read
   adapters (P6) fold into ATM. Memory verify (P3, probe-gated), advisory dependency rows (P5, the
   CEM half rejected for now), the authority pre-check (P8, blocked on decision 0009 and the harness
   cap) and the worktree observer (P10) stay in the ideas backlog behind named triggers.
4. The release carries one falsifier for its positioning claim: on the next untouched partition,
   preregistered with at least eight abstention or epistemic-state cases, any case reporting
   `critical_missing: []` while missing gold critical evidence falsifies the "names what it still
   cannot prove" sentence. Both current abstention and epistemic-state accuracies (0.0 on blind-v2
   and blind-v3, the latter at n=1) are disclosed in the README status row beside the critical-miss
   count, per the truth non-divergence rule.
5. `docs/agent-memory/ideas.md` consolidates to the thirteen entries listed in the panel memo.

## Consequences

The release surface does not grow; the front-page claim acquires a preregistered way to be proven
wrong.

## Rollback

Re-open any deferred item by recording its trigger as met; this record changes no wire contract,
spec requirement, or gate.
