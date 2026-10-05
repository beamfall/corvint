## 2026-10-05 V1-0792: a current external review PASS offers complete-manual

Human-owned intent: owner request [issue 587](https://github.com/beamfall/corvint/issues/587) part 3,
ticket V1-0792, owner decision D7 option (a). When every required external review is a current
PASS, the reads should say that `complete-manual` is the next action and name the evidence, rather
than leaving an operator to reconstruct it from gate views. D7 rules out completing automatically
and building an opt-in completion policy.

Requirements: `ERG-V0-011` in `docs/specs/corvint-tasks-external-reviews-v0.md`, inside
`## Requirements`.

### Change

- Transaction: `ExternalReviewCompletionOffer(policy, views)` returns the sorted review heads only
  when the policy declares at least one gate, every declared gate has a view, and every view
  (declared or ticket-referenced) is `CURRENT` with a `PASS` verdict, is not resubmitted and has a
  head. Otherwise it returns nil.
- Ticket view: `OfferCompletion` replaces `nextAction: admit` with `complete-manual`, and sets
  `suggestedEvidence`, only on an OPEN ticket that has no blockers and no unknowns. The member is
  rendered only when it is set.
- Shared predicate: show, blockers and preview all go through `completionOffers.offer`, which also
  requires the planner's claim-blocker derivation (`transaction.ClaimBlockerObservations`) over the
  read's plan input to be empty. A queue pause, missing execution cutover, budget unknown, pool
  ineligibility or retry exhaustion therefore withholds the offer, and a BLOCKED plan entry never
  carries it. This was added after Codex review r1 found that a paused queue still offered.
- CLI: `ticket show` and `ticket blockers` apply the offer after the existing derivation. For
  `plan preview`, an entry gets additive `nextAction` and `suggestedEvidence` members only when it
  is offered. The receipt fold runs lazily, at most once per read, and only for tickets that carry
  review references in a journal-backed store. A fold or view failure means no offer and never
  fails the read.

### Decisions and limits

- No new result codes, writer changes or stored state. The read stays pure (CAL-V0-034), and
  ERG-V0-007 is unchanged.
- `nextAction` has no closed vocabulary in the specs or in Core, so no amendment was needed. Core's
  closed plan decoder covers plan history, not preview entries.
- Byte identity: tickets without review references, and queues whose policy declares no gates,
  render exactly as before. The CLI test compares the encoded bytes.
- Executable gate results are not part of the offer predicate. They stay NOT_OBSERVED in these
  reads. Whether they should withhold the offer is recorded as an open question in the spec's Open
  decisions, together with whether `queue status` and `roadmap` should carry the offer and how the
  required gate set is chosen. Withholding on NOT_OBSERVED would make the offer unreachable in a
  queue with required executable gates.
- A DEFERRED plan entry may still offer. A resource collision with other work or the attempt limit
  is not a fact about the ticket, and it does not block a manual completion.
- `queue status` and `roadmap` are unchanged and still report `admit` for an offered ticket. The
  `plan preview --selected-only` projection is also unchanged; it lists selected IDs and counts and
  has no per-ticket `nextAction`.

### Evidence

- Focused tests: `TestERGV0011_*` in `internal/tasks/{transaction,ticket,cli}`, plus the
  `internal/tasks/{ticket,transaction,cli}`, `internal/lrfrepo` and `internal/specindex` packages.
  `internal/tasks/store` was run with `ExternalReview|ERGV0|Show|Plan`. Also gofmt, `go vet` of
  `internal/tasks/...`, `go build ./...`, the `GOOS=windows` cross-build, the CI doc gates and the
  use-case receipt checks.
- NOT_RUN: `make gate`, the repository-wide suite, the Windows test-vet, a two-gate native fixture,
  operator use in a live queue, and the dogfood bind/seal (owned by the coordinator after review).
