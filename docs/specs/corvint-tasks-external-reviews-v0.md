# Corvint Tasks external reviews V0

Owner: Russell Lewis
Date: 2026-10-04
Intent status: accepted (owner decision 2026-10-04)
Delivery status: experimental

Authoritative inputs: owner request [issue 504](https://github.com/beamfall/corvint/issues/504)
(native ticket V1-0701). The technical contract below adapts the issue's preparation plan
(proposal labels ER504-001..009, an independent Gate A with two MED and one LOW clarification, an
independent frozen source review whose one MED finding F1 was repaired, and a same-reviewer PASS of
that repair). Those reviews were agent reviews. On 2026-10-04 the owner accepted this intent as
drafted, including its defaults for the derived-event slot shared with issue 501, the EVIDENCE
artifact-binding wire shape and the issue 394 verdict mapping (see Open decisions). The existing
[agent lease contract](corvint-tasks-agent-leases-v0.md) and the TCP-00 executable gate rules keep
their authority; this profile never amends them.

## Agent digest
- Claim: Review verdicts (G1/G2 PASS/RETURN) and author resubmissions become typed, CAS-ordered queue events that route work without parsing Markdown.
- Status: accepted (owner decision 2026-10-04); experimental: pure seed, read adapter/history, dispatcher gate predicates and the native writer (OWNER/OPERATOR/REVIEWER recorders, WORKER resubmissions, TREE and EVIDENCE candidates, evidence references, the issue 394 acceptance mapping).
- Exists: closed codecs, a pure reducer, gate adapter and history reader; policy `externalReviews`, the ticket `externalReviews` field (Tasks and Core readers), the locked REVIEW_RECORD/REVIEW_RESUBMIT writer on the derived-event slot with a receipt-audit binding, `gate record|resubmit|history|state`, REVIEWER/WORKER actor admission, journal-backed EVIDENCE candidates and evidence references with the issue 394 acceptance mapping (`--from-acceptance`), the full `gates[G]` field set, native dispatcher gate observation, two-writer, crash/redo and descriptor-measurement fixtures, and the read-only `nextAction: complete-manual` offer when every required gate is a CURRENT PASS (ERG-V0-011).
- Blocked on: promotion (ERG-V0-010): independent review, current-main integration, live qualification of the issue 394 connection with a real `corvint tests accept` report, and native completion.
- Read next: Requirements; Failure modes and trust; Acceptance evidence and traceability; Rollout and rollback.

## User and current state

A scheduler routing tickets between authors and reviewers needs the current review verdict for each
review gate. Today verdicts live as Markdown headings in status files and a parser reads them. The
issue records three actual misroutes: the prose "No G1 PASS is claimed" read as a pass; an author
section titled "G1 RETURN repair dispositions" read as a newer RETURN, so a resubmitted matrix kept
returning to authors (10 of 24 sessions without progress); and a second RETURN after a resubmit line
was invisible. The simpler baseline is the regression-tested parser in use; it remains text
interpretation and gives neither CAS ordering nor binding to the reviewed content.

At public main `4b10a02144faed4a77b36eba4b0d1cbc315706d3` (before this profile) no Tasks record, policy, writer, CLI verb or
dispatcher input carries an external review. `snapshot/gate.go` executable gate results require an
executed candidate and exit data for PASSED, and `Attempt.Reviews` is reserved for TCP-00 §7.2
executable review dispositions; neither is repurposed. `dispatch/workstate.go` accepts only a
string or a closed `{state,progress}` object from a program and rejects anything else.

The first delivery added `internal/tasks/snapshot/external_review.go`,
`internal/tasks/transaction/external_review.go` and their tests: closed wire codecs and a pure
reducer over explicitly supplied observations. The second delivery (issue 504 slice, 2026-10-04)
adds `internal/tasks/transaction/external_review_read.go`, the pure ERG-V0-009 read side:
`ExternalReviewGates` maps a ticket's gate references, retained event bytes and the explicit current
binding of each gate to one `ExternalReviewView` per gate (at most 16; a missing head, a missing
binding or a binding for another gate gives UNKNOWN), and `ExternalReviewHistory` reads one gate's
events newest first, anchored at the head reference. It also adds typed gate routing to the
dispatcher: `dispatch.Ticket.Gates` carries the native `{status,verdict,generation,revision,
resubmitted,head}` per gate, and a role's `match.gates` is a closed list of at most 16
`{gate,states}` predicates whose states are 1..4 of PASS, RETURN, RESUBMITTED and NONE. PASS and
RETURN need a CURRENT verdict; RESUBMITTED needs a CURRENT resubmission awaiting review; STALE and
UNKNOWN match no predicate. NONE (no record for that gate) was proposed by the implementer and
accepted by the owner on 2026-10-04 as an ERG-V0-009 amendment. It fails closed: until the native
observation sets `Ticket.GatesObserved`, every gate reads UNKNOWN, so no predicate (NONE included)
matches and a configuration using `match.gates` routes nothing. A gate head joins the CAL-V0-057
progress fingerprint, and a ticket without gates fingerprints exactly as before. The workState
program decoder is unchanged, so a program cannot supply gates.

The third delivery (native ticket V1-0701, 2026-10-04) installs the first native slice. Policy gains
optional `externalReviews` (1..16 closed entries: gateId distinct from every executable gate,
purpose ROUTING_ONLY, non-empty recorderRoles from OWNER/OPERATOR/REVIEWER, reviewStages, non-empty
authorStages, requireReviewerLease); `definitionSha256` is the digest of the entry's canonical
bytes, and a gate absent from policy refuses every record (GATE_UNKNOWN). Tickets gain an optional
`externalReviews` map of at most 16 `{generation,revision,head}` references, omitted until first use
so a ticket without reviews keeps its legacy bytes; Core's reader admits exactly that closed shape.
`REVIEW_RECORD` and `REVIEW_RESUBMIT` are MUTATE operations on the issue 501 derived-event slot: the
locked writer re-derives every reducer observation from the audited inventory (acceptance revision,
definition and policy digests, the current gate head, the subject's BUILT transition receipt linked
into the audited chain and posting the attempt record, subject currency, and live leases), posts the
ticket and the one event, advances only the ticket revision, and refuses with REVISION_CONFLICT,
UNAUTHORIZED, STALE_TICKET, STALE_TREE, LIMIT_EXCEEDED or MISSING_EVIDENCE before any effect. To
satisfy the slot-reuse rule, `store.FoldExternalReviews` folds receipts through
`ExternalReviewReceiptAudit`, which refuses (JOURNAL_FORKED) a gate reference that is dropped or
changes outside one single-gate MUTATION receipt, and replays each head event's material transition
through the reducer (`ValidateExternalReviewRecovery`) against the preceding reference and event:
the expected counters, generation, predecessor, prior RETURN, actor, trust source, time, sequence
and request must reproduce the posted event under the retained definition's recorder roles and
reviewer-lease requirement. The definition and policy digests must be those of the
policy retained in effect at that receipt, and the subject must be a retained BUILT submission
receipt of the same ticket in one of that definition's author stages, with the event's attempt
generation and candidate tree, that no later submission in any author stage superseded; the
binding replayed is built from those retained facts, not from the event. Every consumer runs that fold:
`receipt audit`, the dispatcher observation (a refused fold leaves gates unobserved), and redo of a
pending receipt that posts a ticket record, which refuses before publishing any post. The replay does
not re-judge historical lease liveness. `gate
record`, `gate resubmit` and `gate history` (newest first, at most 50, default 20, `--cursor` from
the truncation warning) are the CLI; a `record|resubmit` retry whose request id names an event on
the gate's chain and whose inputs match it resubmits the retained request bytes, so with the same
`--issued-at` it replays after a head, policy or lease change, while a changed input refuses as
REQUEST_ID_CONFLICT before any fresh composition or policy lookup. A request id the queue-wide
request index retains for another ticket, gate or operation refuses the same way. The dispatcher's native observation fills `Ticket.Gates` and sets
`GatesObserved` for each ticket whose gates it reads; a gate set it cannot read stays unobserved
(UNKNOWN). Subject currency comes from durable submission history: a subject is current while no
later receipt posts a BUILT author-stage attempt of the ticket, inline or blob-backed (the writer streams and chain-checks
the receipts after the subject; readers use the fold, indexed by ticket and stage, and an attempt
post they cannot read and fully decode is JOURNAL_FORKED rather than skipped), so a newer submission stales a verdict even
after that attempt leaves BUILT.

This slice admits only OWNER and OPERATOR actors, because the existing writer guard and mutation
model accept only those roles: an OWNER/OPERATOR record without a reviewer lease is
OPERATOR_ATTESTED (only when the definition does not require a lease), with a holder-matched
review-stage lease it is LEASE_BOUND, and a resubmission needs the actor's holder-matched live
author-stage lease. The fifth delivery (below) admits REVIEWER and WORKER actors and EVIDENCE
candidates and evidence references.

The fourth delivery (native ticket V1-0792, issue 587 part 3, 2026-10-05) answers the issue's
report that accepted tickets stayed OPEN until someone noticed and ran `complete-manual` by hand.
Under owner decision D7 (option a) it adds only a read-only offer (ERG-V0-011): `ticket show`,
`ticket blockers` and `plan preview` report `nextAction: complete-manual` with the review heads as
`suggestedEvidence` when every required gate is a CURRENT PASS and nothing else blocks the ticket.
The offer is derived per read in `transaction.ExternalReviewCompletionOffer` and
`ticket.View.OfferCompletion`; it writes nothing, adds no result code, and completion stays the
operator's `complete-manual` disposition, so ERG-V0-007 is unchanged.

The fifth delivery (native ticket V1-0701, issue 504, 2026-10-05) completes the native writer.
Actors: a REVIEWER records only with its own holder-matched live review-stage lease (LEASE_BOUND) and
never resubmits; a WORKER never records and resubmits only with its own live author-stage lease;
neither role enters any other mutation; OWNER and OPERATOR keep the third delivery's rules. Subject
currency is judged from submission history only, so a re-claim or a review-stage submission does
not stale a verdict. Artifact link: an EVIDENCE candidate or an evidence reference `{label,sha256,
bytes}` is admitted only when it is an evidence entry of an executable GateResult that names the
subject attempt, generation and candidate tree, that GateResult being listed by an attempt record
posted in the same receipt that posts the artifact fresh under `evidence/<sha256>`, and whose process
exited (outcome EXIT) in a clean worktree at that candidate tree; its exit code and state are not
judged, so a failing run at the candidate links its report while output from a worktree the gate
dirtied or a tree it moved does not. The writer scans links in receipts after the subject; the receipt fold
checks the same link at replay. When an EVIDENCE candidate artifact parses as an issue 394 report
(`corvint-new-e2e-assessment/0|1`), accepted admits only PASS, rejected only RETURN, blocked no
verdict (MISSING_EVIDENCE), and any other verdict refuses as MALFORMED; the writer and the fold both
enforce it. `gate record --candidate-evidence SHA:BYTES`, `--from-acceptance REPORT_PATH` (which reads
the report file, takes its digest as the candidate and its verdict as the request's) and repeatable `--evidence LABEL=SHA:BYTES` are the CLI. `gate state
TICKET` exposes the full `gates[G]` field set (gate, verdict, status, generation, revision,
resubmitted, head, evidenceSha256, subject, candidate, trust). Measured: a maximal review record
descriptor uses 5 of 6 artifacts and 1342 of 1670 bytes; a blob-backed ticket post adds one EVIDENCE
artifact and stays within the generic maximal derived-event descriptor of 1669 bytes.

## Requirements

- `ERG-V0-001`: A review record MUST distinguish the reviewed subject from the recording actor.
  The request names the author's subject attempt by attemptId, generation, receipt sequence, receipt
  digest and attempt POST digest, so a reused attempt ID at a later generation cannot stand in for it.
  An optional reviewer lease {attemptId, generation, holder} names the recorder's own live lease and is
  never inferred from the subject. Recorder roles come only from the trusted native actor binding, never
  from the payload. WORKER cannot record a verdict. OWNER or OPERATOR may record without a reviewer
  lease only when policy does not require one, and that event is marked OPERATOR_ATTESTED; otherwise it
  is LEASE_BOUND. A REVIEWER records only LEASE_BOUND with its own holder-matched review-stage lease
  and cannot resubmit; a WORKER resubmits only with its own author-stage lease. Actor authentication
  and independence are always recorded as NOT_OBSERVED.

- `ERG-V0-002`: The reviewed candidate MUST be a closed tagged union bound to the subject.
  `{kind:"TREE",treeOid}` or `{kind:"EVIDENCE",sha256,bytes}`; inactive members are refused. TREE must
  equal the bound successful SUBMIT receipt's candidate tree. EVIDENCE is admissible only with a prior
  admitted, journal-backed artifact link to that subject generation; without such a producer EVIDENCE
  recording refuses rather than accepting a raw path or caller digest. The link is an evidence entry
  of an executable GateResult for the subject attempt, generation and candidate tree whose process
  exited in a clean worktree at that tree, posted fresh in the receipt whose attempt record lists that
  GateResult. Each evidence reference needs the same link.
  An EVIDENCE artifact that is an issue 394 acceptance report admits only its mapped verdict
  (accepted PASS, rejected RETURN, blocked none); a contradiction refuses.

- `ERG-V0-003`: Each committed record or resubmission MUST be one immutable canonical `taskman-external-review-event/0` blob of at most 65536 bytes.
  Closed keys: profile, ticketId, acceptanceRevision, gateId, definitionSha256, policySha256,
  reviewGeneration, eventRevision, action, subject, candidate, actor, reviewerLease, trust, verdict,
  reasons, evidence, previous, recordedAt, receiptSeq, requestSha256, request. The original closed
  `taskman-external-review-request/0` request is retained and its JSON+LF digest verified; every
  duplicated event field must equal the retained request. RECORD carries PASS or RETURN; RESUBMIT
  carries null. Reasons are at most 16 `{code,text}` with text 1..512 bytes and at most 4096 bytes in
  total; RETURN and RESUBMIT need at least one. Evidence is at most 16 `{label,sha256,bytes}`
  references. previous is null exactly at eventRevision 1. Unknown or duplicate keys, omitted nullable
  fields, non-canonical bytes and a trust value other than NOT_OBSERVED refuse. Reason prose is data:
  no heading or phrase changes state.

- `ERG-V0-004`: A ticket MAY carry an optional bounded `externalReviews` map of at most 16 gate labels to `{generation,revision,head}`.
  The field is omitted until first use and is never added to the mandatory ticket keys. Per gate at most
  4096 events; overflow refuses. Current state is derived from the head event, not from an inline
  history. Non-review mutations preserve the map; CREATE, REFINE, import and adopt cannot inject or
  erase it.

- `ERG-V0-005`: Records and resubmissions MUST be ordered by exact generation and revision CAS.
  The first RECORD expects generation 0 and revision 0 and produces 1/1. Every later request names
  both current counters; a mismatch refuses, so of two requests with the same expectations one wins.
  A same-generation replacement verdict increments only the revision. RESUBMIT names the exact current
  RETURN head as priorReturn and a live author lease for the ticket in a configured author stage, and
  opens generation+1 with a null verdict and resubmitted=true. A later RECORD in that generation is
  current again, so a second RETURN stays actionable. G1 and G2 counters are independent. Identical
  request replay returns the original event and time, and is checked after actor binding and before
  fresh policy and CAS; changed bytes under the same request ID conflict. A refusal produces no event.

- `ERG-V0-006`: Current routing state MUST become STALE when the acceptance revision, external gate definition, subject or candidate changes, and only an explicit transition may leave STALE.
  The historical policy digest and unrelated content or note edits do not make a review stale. A
  stale PASS, or a stale RESUBMIT (repair F1), may be followed by a newly authorized RECORD against the
  exact current bindings and CAS; it opens generation+1 and links previous to the old head. A stale
  RETURN requires an explicit author RESUBMIT. Missing, malformed or unbound heads give UNKNOWN, which
  never carries an actionable verdict and refuses new records.

- `ERG-V0-007`: External review events MUST remain routing evidence only.
  An external PASS cannot satisfy executable required gates, synthesize exit or worktree proof,
  satisfy TCP-00 §7.2 independence, complete a ticket, authorize a merge or clear holds or debt. The
  executable GateResult decoder rejects the event profile. Mandatory external approval for completion
  would need a separately accepted policy.

- `ERG-V0-008`: Recovery MUST validate material bindings, not only digests.
  A retained event and post-reference are accepted only when replaying the same transition from
  audited observations reproduces them byte for byte: the retained request's expected counters equal
  the audited pre-reference, priorReturn equals the preceding RETURN head, queue/ticket/request ID and
  request digest equal the descriptor and receipt context, subject and candidate equal the bound
  subject receipt POST, and the produced generation, revision and head equal the post-reference.
  Unknown observations refuse.

- `ERG-V0-009`: The native writer, policy, reads and workState adapter MUST be installed before the capability is called implemented.
  Policy gains optional `externalReviews` definitions (at most 16; gateId, recorderRoles, reviewStages,
  authorStages, requireReviewerLease, purpose ROUTING_ONLY) with no default grant. The locked writer
  produces every observation of ERG-V0-005..008 from one audited inventory, posts the ticket and one
  event through the existing MUTATE transaction within its six-artifact and 1670-byte descriptor
  bounds using a closed derived-event slot coordinated with issue 501, and supports idempotent redo.
  `gate record` and `gate resubmit` CLI verbs, bounded anchored history (page at most 50, default 20,
  1 MiB) and a native workState adapter exposing
  `gates[G]={verdict,generation,revision,resubmitted,status,candidate,subject,trust,evidenceSha256}`
  with closed gate predicates are required; `gate state TICKET` reads that field set. A PASS, RETURN or RESUBMITTED predicate requires status
  CURRENT; a NONE predicate (owner decision 2026-10-04) matches only when the ticket's gates were
  natively observed and that gate has no record, so an unobserved, STALE or UNKNOWN gate matches no
  predicate. Legacy string and
  `{state,progress}` workState stays byte-compatible, and arbitrary program-returned gates are not
  native authority.

- `ERG-V0-010`: Promotion and rollback MUST keep the pure seed, full integration and producers distinct.
  The four seed files are pure helpers, not native authority. Promotion needs native two-writer CAS,
  replay, recovery, capacity, legacy-omission and read-no-mutation fixtures; the EVIDENCE artifact
  producer; the issue 394 acceptance-verdict producer as a separately qualified connection; independent
  review; current-main integration; and native completion. Rollback disables new records, resubmits
  and routing consumption while keeping compatible readers and all stored events; it never strips
  fields or deletes history.

- `ERG-V0-011`: `ticket show`, `ticket blockers` and `plan preview` MUST offer `nextAction: complete-manual` read-only exactly when every required external review gate is a CURRENT PASS and nothing else blocks the ticket.
  The required set is every gate the policy's `externalReviews` declares plus every gate the ticket
  references; a policy declaring no gate never offers. Each gate must read CURRENT with verdict PASS
  through the ERG-V0-009 adapter, so it is bound to the current acceptance revision, definition,
  subject and candidate (ERG-V0-006); a missing, STALE, UNKNOWN or RETURN gate, or a resubmission
  awaiting review, gives no offer. The offer replaces only `admit` for an OPEN ticket whose view has
  no blocker and no unknown: a hold, escalation, dependency, approval, effects, cutover or live-attempt
  blocker keeps its own action. One predicate serves all three reads: the planner's claim-blocker
  derivation over the read's plan input (`transaction.ClaimBlockerObservations`: queue pause, missing
  execution cutover, budget, pool eligibility, retry exhaustion, the reservation-aware view and every
  unknown) must also be empty, so a `plan preview` entry that is BLOCKED never offers, and `ticket
  show` withholds the offer under the same facts it reports through `claimabilityReason`. A DEFERRED
  entry (a resource collision with other work or the attempt limit) is not blocked by a fact about
  the ticket and may offer. It needs audited journal evidence: an absent journal, a refused receipt
  fold or an unreadable gate set gives no offer and never fails the read. With an offer, the
  `ticket show|blockers` item gains `suggestedEvidence` (the review head digests, byte-sorted, usable
  as `complete-manual` evidence), and the `plan preview` entry gains `nextAction` and
  `suggestedEvidence`. Without an offer every item is byte-identical to the output before this
  requirement. The read stays pure (CAL-V0-034): no probe, no write, no new result code and no writer
  change. It never completes a ticket or satisfies an executable gate, whose results stay
  `gateResults: NOT_OBSERVED` in the view and are not part of the predicate; ERG-V0-007 is unchanged.
  `queue status`, `roadmap` and the `plan preview --selected-only` projection are unchanged.

## Non-goals

No prose interpreter, side database, daemon, network read, automatic reviewer spawning or worker
messaging. No VERIFIED actor authentication or independence: distinct IDs and holders do not prove
independent review. No change to executable gates, TCP-00 dispositions, retry charging or completion.
No automatic completion and no opt-in completion policy (owner decision D7, 2026-10-05): the
ERG-V0-011 offer is a read-only suggestion, and `complete-manual` stays an operator action.

## Failure modes and trust

Every refusal happens before a committed effect and leaves counters unchanged. Unknown policy,
subject, candidate-link, evidence or replay observations refuse rather than default to true. A
rehashed event whose bytes are well formed but whose counters, prior RETURN, context or subject do
not match the audited state fails recovery. Reason text and evidence labels are untrusted data. The
event, history and ticket bounds above, plus the existing 128 KiB ticket and store limits, cap cost.
The ERG-V0-011 offer fails closed: a missing journal, any planner claim blocker or unknown (for
example a queue pause), a refused receipt fold, an unreadable gate set or any gate that is not a
CURRENT PASS gives no offer. `ticket show|blockers` then keeps its existing `nextAction` and adds no
`suggestedEvidence`; a `plan preview` entry carries neither member. It folds
the receipts at most once per read and only when a ticket carries a review reference under a policy
declaring review gates. A wrong offer can only suggest an operator disposition; it cannot complete.

## Acceptance evidence and traceability

The first two deliveries supply pure and dispatcher-level focused evidence; the third adds focused
native evidence for the OWNER/OPERATOR slice and the fifth for REVIEWER/WORKER actors, EVIDENCE
links, two writers and the descriptor. The last column stays NOT_RUN.

| Requirement | Parent proposal | Delivered evidence (pure seed, read slice and first native slice) | Required integrated evidence (NOT_RUN) |
|---|---|---|---|
| ERG-V0-001 | ER504-001 | `TestIssue504ReplayAndAuthority` (recorder roles, reviewer lease, operator attestation), `TestIssue504ResubmitAndSecondReturn` (author lease distinct from reviewer); `TestERGV0009_NativeVerdictsThroughTheCLI` (OPERATOR-attested record and holder-matched author lease through the native writer); `TestERGV0001_ReviewerAndWorkerActors` (policy-required lease refuses an OPERATOR attestation; REVIEWER records LEASE_BOUND only on its own lease and never resubmits; WORKER never records and resubmits on its own author lease; neither enters another mutation; the reviewer's review-stage submission does not stale the verdict) | operator use in a live queue |
| ERG-V0-002 | ER504-002 | `TestIssue504BoundsAndUnknown` (EVIDENCE union encodes; an unproved candidate link refuses), `TestIssue504MaterialBindings` (subject receipt binding), `TestIssue504HistoricalPreservation` (candidate change); `TestERGV0009_NativeVerdictsThroughTheCLI` (real SUBMIT receipt lookup); `TestERGV0002_EvidenceCandidatesNeedAnArtifactLink` (an unlinked candidate or evidence reference refuses; a linked issue 394 report records RETURN and, after resubmission, PASS from `--from-acceptance`; a blocked report and a contradicted verdict refuse; retries replay; a report printed by a gate that dirtied its worktree or moved its tree while running does not link), `TestERGV0002_ForgedArtifactEventsRefuseAtRecovery` (rehashed events with an unlinked candidate, an unlinked evidence reference or a verdict the report contradicts refuse redo as JOURNAL_FORKED with the projection unchanged, and once settled fail the receipt audit and leave gates unobserved) | inactive-member refusal test; live `corvint tests accept` report |
| ERG-V0-003 | ER504-003 | `TestIssue504CanonicalRoundTrip`, `TestIssue504BoundsAndUnknown` in snapshot; `TestIssue504TypedRouting` (prose cannot change state) | measured worst-case event size; evidence storage |
| ERG-V0-004 | ER504-003 | `TestIssue504CanonicalRoundTrip` (closed per-gate reference codec only), `TestIssue504BoundsAndUnknown` (event revision 4097 refuses); `TestERGV0009_TicketReviewReferencesCodec` (16-gate map, legacy byte identity), `TestERGV0009_CoreReaderAdmitsOnlyTheClosedReviewReferences` | preservation by every non-review writer |
| ERG-V0-005 | ER504-004 | `TestIssue504ResubmitAndSecondReturn`, `TestIssue504ReplayAndAuthority`, `TestIssue504BoundsAndUnknown` in transaction (same-CAS single winner over pure state); `TestERGV0009_NativeVerdictsThroughTheCLI` (stale counters refuse with REVISION_CONFLICT); `TestERGV0005_TwoWritersOneWinner` (two concurrent writers with equal expectations: exactly one commits, the loser refuses REVISION_CONFLICT, the head and history hold one event and the winner's retry replays) | none beyond promotion |
| ERG-V0-006 | ER504-002, F1 | `TestIssue504HistoricalPreservation`, `TestIssue504ResubmitAndSecondReturn/stale-resubmit-fresh-cycle`, `TestIssue504BoundsAndUnknown`; `TestERGV0009_NativeVerdictsThroughTheCLI` (a newer submission stales a PASS and refuses a verdict on the old subject, also after the newer attempt is released), `TestERGV0006_BlobBackedSubmissionSupersedes` (a blob-backed newer submission stales the PASS and refuses a verdict on the old subject; the indexed fold answers supersession within a fixed bound across a 256-fold longer history), `TestERGV0006_RecoveryChecksEveryAuthorStage` (redo refuses a rehashed verdict superseded by a newer submission in another author stage or a blob-backed one, and admits one whose newer submission is outside the author stages), `TestERGV0006_BuiltPostsNeverSkipUnreadable` (an absent, unretained, mismatched, non-record or malformed attempt post, whatever phase it claims, is JOURNAL_FORKED, never skipped) | staleness from a real acceptance change |
| ERG-V0-007 | ER504-005 | `TestIssue504CanonicalRoundTrip`, `TestIssue504TypedRouting` (GateResult decoder rejects the event) | completion predicates unchanged under native records |
| ERG-V0-008 | ER504-005, Gate A MED | `TestIssue504MaterialBindings` (rehashed wrong CAS, priorReturn, context, post and subject) | locked staged recovery and crash/redo fixtures |
| ERG-V0-009 | ER504-001, ER504-005, ER504-006 | `TestIssue504GateAdapter` (16-gate bound, UNKNOWN for a missing head or binding or a head for another ticket or gate, STALE), `TestIssue504AnchoredHistory` (default and maximum page, cursor paging, off-chain cursor, broken link and digest refusals), `TestIssue504DispatchMisroutes` (the three issue misroutes route by typed verdict through adapter and roster), `TestERGV0009_GatePredicates` (closed predicate config, NONE only when observed, unobserved gates are UNKNOWN, STALE/UNKNOWN never match, fingerprint compatibility, programs cannot supply gates), `TestERGV0009_PolicyExternalReviewsGrantNothingByDefault`, `TestERGV0009_NativeVerdictsThroughTheCLI` (native writer, CLI verbs, history paging, observation filling ticket gates, receipt-audit binding refusing a forged receipt), `TestERGV0009_ForgedReviewEventsRefuseAtRecovery` (rehashed events with a forged actor, counters, prior RETURN, subject generation or candidate tree, or a lease-bound record stripped to an operator attestation under a lease-required policy, refuse redo as JOURNAL_FORKED with the projection unchanged, and once settled leave gates unobserved), `TestERGV0009_ReviewRetriesReplay` (retries replay after head, policy and lease changes; a changed input is a request-id conflict, also after a policy change or with the gate undeclared, and so is a retry on another gate or ticket or reusing another operation's request id), `TestONV0006_DerivedEventSlotClosedToDeclaringOperations` ; `TestERGV0001_ReviewerAndWorkerActors` (`gate state` and the observation expose the full `gates[G]` field set), `TestERGV0009_ReviewDescriptorMeasured` (maximal review record descriptor: 5 of 6 artifacts, 1342 of 1670 bytes, within the 1669-byte generic maximum) | operator use in a live queue |
| ERG-V0-010 | ER504-008, ER504-009 | the native fixture set above (two-writer CAS, replay, crash/redo refusal and control redo, capacity, legacy omission, read-no-mutation) and the issue 394 mapping | independent review, current-main integration, live qualification of the issue 394 connection, native completion |
| ERG-V0-011 | issue 587 part 3, D7(a) | `TestERGV0011_CompletionOfferNeedsEveryRequiredGateCurrentPass` (missing, STALE, UNKNOWN, RETURN, resubmitted, unbound and undeclared gates, and a policy without gates, give no offer; heads are sorted), `TestERGV0011_OfferReplacesOnlyAnUnblockedAdmit` (only an OPEN unblocked admit with nothing NOT_OBSERVED is replaced; every other view renders byte-identically), `TestERGV0011_CompletionOfferThroughTheCLI` (native RETURN, resubmission, live-attempt PASS, a queue pause with a BLOCKED/PAUSED plan entry, undeclared gate and STALE PASS give no offer; unpausing restores it; a CURRENT PASS offers through `ticket show`, `ticket blockers` and `plan preview` without advancing the journal or completing; an unreviewed ticket's show item and plan entry stay byte-identical) | a two-gate native fixture; operator use in a live queue |

## Rollout and rollback

The seed and read slice add pure code and an optional dispatcher predicate; they change no existing
byte format, and a configuration without `match.gates` behaves and fingerprints as before. Rollback
is reverting those files and this spec's catalog entries; a dispatch configuration that uses
`match.gates` must drop the predicate first, since the closed decoder would then refuse it. The native
slice adds optional policy and ticket fields and two operations. Disabling new records is removing
the policy `externalReviews` entries (every record then refuses GATE_UNKNOWN) and dropping
`match.gates` from dispatch configurations; stored references and events stay readable. Reverting
the code is safe only before any ticket carries `externalReviews`, because an older closed reader
refuses the field; once writes exist, deleting events or pretending an older closed reader is
compatible is not rollback.
The ERG-V0-011 offer stores nothing: rollback is reverting its code and requirement row, after which
`nextAction` reads `admit` again and the additive `suggestedEvidence` and plan-entry `nextAction`
members disappear. A consumer of the offer must treat both members as optional.

## Open decisions

Resolved by owner decision 2026-10-04: the intent is accepted as drafted, with its defaults for the
derived-event slot shared with issue 501 (the writer posts the ticket and one event through the
existing MUTATE transaction on that closed slot, within the six-artifact and 1670-byte bounds), the
EVIDENCE artifact-binding wire shape (`{kind:"EVIDENCE",sha256,bytes}` admissible only with a prior
admitted journal-backed artifact link) and the issue 394 verdict mapping (accepted to PASS, rejected
to RETURN, blocked to no verdict). The issue 501 slot itself is not yet committed; the writer follows
its committed shape. A second owner decision on 2026-10-04 amended ERG-V0-009 to admit the
fail-closed NONE predicate. Owner decision D7 (2026-10-05, ticket V1-0792) chose option (a), the
read-only `complete-manual` offer of ERG-V0-011, and declined an opt-in completion policy, so
ERG-V0-007 stands.

Decided (owner, 2026-10-05, ticket V1-0792), keeping the delivered behavior. A policy-required
executable gate without an observed PASS does not withhold the ERG-V0-011 offer: these reads do not
observe executable gate results (`gateResults: NOT_OBSERVED`), withholding on NOT_OBSERVED would make
the offer unreachable in a queue with required executable gates, and `complete-manual` still needs
its own evidence. `queue status` and `roadmap` do not carry the offer; `ticket show` and `plan
preview` do. The required set stays as delivered in ERG-V0-011: every review gate the policy declares
plus every review gate the ticket references.

Decided 2026-10-05 (ticket V1-0701): the owner accepted issue #504 with qualification `NOT_RUN`
and delegated these choices to the orchestrator, which kept the fail-closed defaults: a REVIEWER
cannot resubmit; subject currency follows submission history only, so a re-claim does not stale a
verdict; an artifact link needs a GateResult of the subject attempt, generation and tree that exited
clean at that tree, but not a PASSED one; the issue 394 mapping applies whenever an EVIDENCE artifact parses as such a report,
with no policy switch; the blob read bound for EVIDENCE artifacts is the 16 MiB gate-output bound;
and the writer admits links only from receipts after the subject while the fold admits any linked
receipt, so the writer is the stricter of the two.
