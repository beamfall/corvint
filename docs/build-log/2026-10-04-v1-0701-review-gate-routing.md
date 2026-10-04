## 2026-10-04 V1-0701: typed review-gate routing and ERG-V0 acceptance

Human-owned intent: the owner asked to implement issue 504 (native ticket V1-0701) and, on
2026-10-04, accepted the ERG-V0 intent as drafted. The acceptance includes the drafted defaults for
three decisions:

- the derived-event slot shared with issue 501;
- the EVIDENCE artifact-binding wire shape;
- the issue 394 verdict mapping: accepted to PASS, rejected to RETURN, blocked to no verdict.

The spec header, INDEX and README now record `accepted (owner decision 2026-10-04)`.

### Change

This slice delivers the parts that do not depend on the issue 501 slot.

- **`transaction/external_review_read.go` (pure ERG-V0-009 read side).**
  - `ExternalReviewGates` maps at most 16 gate references to the reducer's current view.
  - A missing head event, a missing binding, or a binding for another gate gives UNKNOWN.
  - `ExternalReviewHistory` reads one gate's events newest first, anchored at the head reference.
    The page size is 1..50 (default 20) with a 1 MiB byte budget, and a cursor continues the walk.
  - Every link is verified: it must be canonical, carry its own digest, belong to the same ticket
    and gate, and follow the exact revision and generation. A resubmission must link the RETURN it
    names. A cursor that is not on the chain refuses.
- **Dispatcher: typed gate routing.**
  - `Ticket.Gates` carries the native per-gate state.
  - A role's `match.gates` takes closed `{gate,states}` predicates over PASS, RETURN, RESUBMITTED
    and NONE.
  - STALE and UNKNOWN never match, so neither ever routes work.
  - The gate head joins the progress fingerprint. A ticket without gates fingerprints exactly as
    before.
  - The workState program decoder is unchanged, so a program cannot inject gates.

### Decision

The dispatcher keeps its own small `GateView` and does not import the transaction package. The
native observation will fill the view from `ExternalReviewGates`.

The predicate names `NONE` explicitly as "no record". This lets a reviewer role select tickets that
have never been reviewed, without reading prose.

### Evidence

The three issue misroutes are replayed through real reducer events, the adapter and `Roster`
(`TestIssue504DispatchMisroutes`):

| Event | Reason text | Routes to |
|---|---|---|
| RETURN | "No G1 PASS is claimed" | author |
| Resubmission | "G1 RETURN repair dispositions" | reviewer |
| Second RETURN after a resubmission | none | author |

A second RETURN also changes the fingerprint.

Other focused tests:

- `TestIssue504GateAdapter`
- `TestIssue504AnchoredHistory`
- `TestERGV0009_GatePredicates`

### Remainder (NOT_RUN)

The following are not delivered in this slice:

- The native locked writer. It needs a second MUTATE EVIDENCE POST of the event under issue 501's
  derived-event slot, plus the ticket's `externalReviews` reference, in one transaction.
- The policy and ticket optional fields, and their Core/adopt guards.
- The `gate record`, `gate resubmit` and history CLI verbs.
- The native observation filling `Ticket.Gates`.
- The EVIDENCE and issue 394 producers.
- The native fixtures for promotion.

Rollback: revert this slice. A configuration using `match.gates` must drop the predicate first.
