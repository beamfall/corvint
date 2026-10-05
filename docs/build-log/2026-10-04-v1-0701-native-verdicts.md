# Issue 504 native review verdicts, first slice (V1-0701)

## Decision

The owner accepted ERG-V0 as drafted on 2026-10-04 (no open decision). This slice installs the first
native path, so that review verdicts are recorded through a CLI and reach the dispatcher as typed
gate state, without Markdown parsing. Its contract is the updated current-state text in
`docs/specs/corvint-tasks-external-reviews-v0.md`.

## What landed

- **Optional fields.** The policy gains `externalReviews` (closed entries, no default grant). The
  ticket gains `externalReviews` (at most 16 references, omitted until first use). Core's ticket
  reader admits the same closed shape.
- **Writer.** `REVIEW_RECORD` and `REVIEW_RESUBMIT` reuse the issue 501 MUTATE derived-event slot.
  - The locked writer derives every reducer observation from the audited inventory.
  - It verifies the subject's BUILT transition receipt against the audited chain and the attempt
    record that receipt posts.
  - It maps reducer refusals to the existing outcomes before any effect.
  - The store hooks are three small hunks in `store/mutate.go` plus a new
    `store/external_review.go`, kept minimal because V1-0645 and V1-0698 also touch the writer.
- **Slot-reuse binding.** `receipt audit` folds every receipt through the pure
  `ExternalReviewReceiptAudit`. A gate reference that changes outside one single-gate MUTATION, or a
  head event that does not record its receipt's transition, refuses as JOURNAL_FORKED. The audit
  output keys are unchanged, because Core's capture reader closes them.
- **CLI.** `gate record`, `gate resubmit` and `gate history`.
- **Dispatcher.** The dispatcher's native observation fills `Ticket.Gates` and sets
  `GatesObserved`. An unreadable gate set stays UNKNOWN.

## Conservative choices

- **Actor roles.** Only OWNER and OPERATOR actors are admitted. The existing mutation model, the
  plan digest and the store guard refuse every other role, and widening them is a separate change.
- **Evidence references.** Non-empty evidence references refuse as an unknown observation until the
  EVIDENCE producer exists.
- **Subject currency.** A subject is current while no later BUILT author-stage attempt of the
  ticket exists. A newer submission makes the verdict STALE.
- **Unchanged slot rules.** The `ticket note` slot and its limits are unchanged.

## Evidence

Focused tests (all passing):
- `TestERGV0009_NativeVerdictsThroughTheCLI`;
- `TestERGV0009_PolicyExternalReviewsGrantNothingByDefault`;
- `TestERGV0009_TicketReviewReferencesCodec`;
- `TestERGV0009_CoreReaderAdmitsOnlyTheClosedReviewReferences`;
- `TestONV0006_DerivedEventSlotClosedToDeclaringOperations`, updated so that the slot is declared
  by the note and review operations only.

NOT_RUN:
- crash/redo and two-writer fixtures;
- the 1670-byte descriptor measurement for review operations;
- REVIEWER actors;
- the full `gates[G]` field set;
- the issue 394 producer;
- promotion.
