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
- **Subject currency.** A subject is current while no later receipt posts a BUILT author-stage
  attempt of the ticket. A newer submission makes the verdict STALE, and it stays STALE after that
  attempt leaves BUILT.
- **Unchanged slot rules.** The `ticket note` slot and its limits are unchanged.

## Review repair (Codex CHANGES_REQUIRED at 3fe36ab4)

- **P1-1, currency.** Currency was derived from current attempt phases, so a released or checking
  newer attempt revived an obsolete PASS. It now comes from durable submission history: the writer
  streams the receipts after the subject, chain-checked one at a time, and readers use the fold.
- **P1-2, consumers.** The binding fold moved to `store.FoldExternalReviews`. `receipt audit`, the
  dispatcher observation and redo of a pending ticket-posting receipt all run it, so a rehashed
  untrue event refuses as JOURNAL_FORKED before redo publishes anything.
- **P1-3, counters.** The fold replays each head event through `ValidateExternalReviewRecovery`
  against the preceding reference and event, which checks expected counters and the prior RETURN.
- **P2-4, retries.** A retry whose request id and inputs match a retained event resubmits that
  event's request bytes, so it replays after a head, policy or lease change.
- **Cost (V1-0645).** REVIEW_* writes keep one extra full audit and add an O(receipts after the
  subject) scan. The writer hunks in `store/mutate.go` and `store/redo.go` stay at three lines each.
  The O(receipts) claim for the fold was wrong; the second repair below corrects it.

## Second review repair (Codex CHANGES_REQUIRED at c0d36bd2)

- **P1-1, blob-backed submissions.** `ExternalBuiltPosts` skipped attempt posts that carried
  `blobSha256` instead of an inline record, so a blob-backed newer submission did not stale a PASS.
  It now reads both encodings through the evidence blob store and re-hashes them. An absent,
  unretained, mismatched or non-record attempt post is JOURNAL_FORKED and is never skipped.
- **P1-2, author stages.** Recovery checked supersession only against the subject's own stage. It
  now checks every author stage of the retained definition, and it requires the subject itself to be
  in one of those stages.
- **P1-3, subject and candidate binding.** Recovery did not bind the subject's attempt generation or
  candidate tree. The fold now retains each BUILT post's generation and tree. It requires the
  event's subject and candidate to match them, and it checks the event's definition and policy
  digests against the policy retained in effect at that receipt. The binding it replays is built
  from those retained facts.
- **P2-4, retries.** `retainedReviewRequest` used to fall through to fresh composition on a mismatch.
  It now refuses as REQUEST_ID_CONFLICT before any policy lookup, so the refusal holds after a policy
  change or with the gate undeclared.
- **P2-5, cost.** The fold rescanned all earlier submissions for every event, which is Θ(N²). It now
  keeps a ticket→stage→latest-BUILT-sequence index:
  - building it costs O(receipts × posts per receipt);
  - each supersession check costs O(author stages);
  - the writer's own check stays a linear scan of the receipts after the subject.

  `TestERGV0006_BlobBackedSubmissionSupersedes` bounds the lookup time across 16 and 4096 folded
  submissions. That bound is only a ratio check, not a benchmark.
- **V1-0645 interaction.** The fold now resolves blob-backed attempt posts and decodes retained
  policy posts, so it reads more per receipt. Its asymptotic cost is unchanged and it remains a full
  O(receipts) pass for each consumer. V1-0645's writer-cost work should treat the fold as one more
  full pass to cache or amortise.

## Third review repair (Codex CHANGES_REQUIRED at d500620a)

- **P1-1, reviewer lease.** The replay observations copied the definition's recorder roles but
  not `requireReviewerLease`, which defaulted to false. Under a lease-required policy, a
  lease-bound RECORD rewritten to an operator attestation and rehashed therefore replayed. Replay
  now carries the retained requirement. The forge test adds a lease-bound control and a
  dropped-lease forgery, each in pending and settled form. The forgery was redone before the fix.
- **P1-2, malformed attempts.** `ExternalBuiltPosts` checked the raw phase before decoding, so a
  hash-consistent object with missing or mistyped attempt fields was skipped. Every attempt post is
  now fully decoded before filtering, and a failure is JOURNAL_FORKED. Three valid-JSON malformed
  cases were added, and they fail against d500620a.
- **P2-3, changed-target retries.** A retry on another gate or ticket missed the gate-chain lookup
  and failed on fresh composition (GATE_UNKNOWN or MALFORMED). A request id is now resolved
  through `journal.RequestIndex` before composition:
  - the afterimage path is probed first, so a fresh id costs no audit;
  - a completed request off the selected chain, or any retained request for another ticket, is
    REQUEST_ID_CONFLICT;
  - an uncompleted one on the same ticket falls through to the writer.

  The chain walk covers every event a gate can hold (4096). Tests cover the undeclared-gate,
  other-ticket and other-operation retries.

Focused tests (all passing):
- `TestERGV0009_NativeVerdictsThroughTheCLI`;
- `TestERGV0009_ForgedReviewEventsRefuseAtRecovery`;
- `TestERGV0009_ReviewRetriesReplay`;
- `TestERGV0006_BlobBackedSubmissionSupersedes`, `TestERGV0006_RecoveryChecksEveryAuthorStage` and
  `TestERGV0006_BuiltPostsNeverSkipUnreadable`. Each of these, and the new forged and retry cases,
  failed when its fix was temporarily reverted, as did the third repair's lease, malformed-attempt
  and changed-target cases;
- `TestERGV0009_PolicyExternalReviewsGrantNothingByDefault`;
- `TestERGV0009_TicketReviewReferencesCodec`;
- `TestERGV0009_CoreReaderAdmitsOnlyTheClosedReviewReferences`;
- `TestONV0006_DerivedEventSlotClosedToDeclaringOperations`, updated so that the slot is declared
  by the note and review operations only.

NOT_RUN:
- the two-writer fixture;
- the 1670-byte descriptor measurement for review operations;
- REVIEWER actors;
- the full `gates[G]` field set;
- the issue 394 producer;
- promotion.
