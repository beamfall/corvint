# Decision 0428: accept the typed-escalation Gate A decisions

Date: 2026-10-04. Status: accepted (owner answer 2026-10-04: "accept the Gate A decisions as
written").
Tickets: V1-0699 (GitHub issue 502). Contract: `docs/specs/corvint-tasks-escalations-v0.md`.

## Context

The typed-escalation contract's rollout puts the native writer and material slice after the pure
foundation, and its Unresolved decisions section required explicit owner acceptance of the Gate A
decisions before that slice integrates. The foundation and its hardening landed as unreferenced
pure code (PR 549). The agent listed the decisions and asked the owner to accept or change them.

## Decision

The owner accepted, as written:

1. Admitted-receipt invocation context for the shorthand, with ActorAuthentication NOT_OBSERVED.
2. One current event per question, rather than one question per ticket.
3. Sole-open-at-commit shorthand next to the exact compare-and-set route.
4. A blocked reason defaults to the explicitly typed question.
5. Infrastructure exhaustion is a distinct, visible automation hold.
6. Typed control operations are excluded from `workRevision`.
7. The optional-reference capacities, including the 16-open bound counting only OPEN questions of
   the current acceptance revision (the agent decision of the hardening slice), with a specialized
   StageLease shape. The contract's kill criterion stays binding: if the typed-event stage cannot fit
   the existing StageLease bounds, the contract is revised rather than the bounds widened.

## Consequences

The writer and material slice may integrate. Acceptance covers these design choices only: each
ESC-V0 requirement stays proposed until its acceptance evidence is retained, and V1-0699 stays open
until the integrated feature is qualified and natively completed.
