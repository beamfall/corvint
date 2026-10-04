# Corvint Tasks external reviews V0

Owner: Russell Lewis
Date: 2026-10-04
Intent status: proposed
Delivery status: experimental

Authoritative inputs: owner request [issue 504](https://github.com/beamfall/corvint/issues/504)
(native ticket V1-0701). The technical contract below adapts the issue's preparation plan
(proposal labels ER504-001..009, an independent Gate A with two MED and one LOW clarification, an
independent frozen source review whose one MED finding F1 was repaired, and a same-reviewer PASS of
that repair). Those reviews were agent reviews, not owner acceptance: the intent stays `proposed`
until the owner accepts it. The existing [agent lease contract](corvint-tasks-agent-leases-v0.md)
and the TCP-00 executable gate rules keep their authority; this profile never amends them.

## Agent digest
- Claim: Review verdicts (G1/G2 PASS/RETURN) and author resubmissions become typed, CAS-ordered queue events that route work without parsing Markdown.
- Status: proposed; experimental delivery of the pure four-file codec/reducer seed only; no installed review capability.
- Exists: closed request/event/reference codecs and a pure transition, current-view and recovery-check helper with focused tests; nothing calls them yet.
- Blocked on: policy/ticket optional-field wiring, the native locked writer and its derived-event slot (shared with issue 501), CLI/history reads, the native workState adapter, the EVIDENCE artifact producer and the issue 394 producer.
- Read next: Requirements; Failure modes and trust; Acceptance evidence and traceability; Rollout and rollback.

## User and current state

A scheduler routing tickets between authors and reviewers needs the current review verdict for each
review gate. Today verdicts live as Markdown headings in status files and a parser reads them. The
issue records three actual misroutes: the prose "No G1 PASS is claimed" read as a pass; an author
section titled "G1 RETURN repair dispositions" read as a newer RETURN, so a resubmitted matrix kept
returning to authors (10 of 24 sessions without progress); and a second RETURN after a resubmit line
was invisible. The simpler baseline is the regression-tested parser in use; it remains text
interpretation and gives neither CAS ordering nor binding to the reviewed content.

At public main `4b10a02144faed4a77b36eba4b0d1cbc315706d3` no Tasks record, policy, writer, CLI verb or
dispatcher input carries an external review. `snapshot/gate.go` executable gate results require an
executed candidate and exit data for PASSED, and `Attempt.Reviews` is reserved for TCP-00 §7.2
executable review dispositions; neither is repurposed. `dispatch/workstate.go` accepts only a
string or a closed `{state,progress}` object from a program and rejects anything else.

This first delivery adds only `internal/tasks/snapshot/external_review.go`,
`internal/tasks/transaction/external_review.go` and their tests: closed wire codecs and a pure
reducer over explicitly supplied observations. No writer, reader, CLI verb, policy field, ticket
field or dispatcher path calls them. Explicitly deferred: everything in ERG-V0-007..010 beyond the
pure seed.

## Requirements

- `ERG-V0-001`: A review record MUST distinguish the reviewed subject from the recording actor.
  The request names the author's subject attempt by attemptId, generation, receipt sequence, receipt
  digest and attempt POST digest, so a reused attempt ID at a later generation cannot stand in for it.
  An optional reviewer lease {attemptId, generation, holder} names the recorder's own live lease and is
  never inferred from the subject. Recorder roles come only from the trusted native actor binding, never
  from the payload. WORKER cannot record a verdict. OWNER or OPERATOR may record without a reviewer
  lease only when policy does not require one, and that event is marked OPERATOR_ATTESTED; otherwise it
  is LEASE_BOUND. Actor authentication and independence are always recorded as NOT_OBSERVED.

- `ERG-V0-002`: The reviewed candidate MUST be a closed tagged union bound to the subject.
  `{kind:"TREE",treeOid}` or `{kind:"EVIDENCE",sha256,bytes}`; inactive members are refused. TREE must
  equal the bound successful SUBMIT receipt's candidate tree. EVIDENCE is admissible only with a prior
  admitted, journal-backed artifact link to that subject generation; without such a producer EVIDENCE
  recording refuses rather than accepting a raw path or caller digest.

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
  with closed gate predicates requiring status CURRENT are required. Legacy string and
  `{state,progress}` workState stays byte-compatible, and arbitrary program-returned gates are not
  native authority.

- `ERG-V0-010`: Promotion and rollback MUST keep the pure seed, full integration and producers distinct.
  The four seed files are pure helpers, not native authority. Promotion needs native two-writer CAS,
  replay, recovery, capacity, legacy-omission and read-no-mutation fixtures; the EVIDENCE artifact
  producer; the issue 394 acceptance-verdict producer as a separately qualified connection; independent
  review; current-main integration; and native completion. Rollback disables new records, resubmits
  and routing consumption while keeping compatible readers and all stored events; it never strips
  fields or deletes history.

## Non-goals

No prose interpreter, side database, daemon, network read, automatic reviewer spawning or worker
messaging. No VERIFIED actor authentication or independence: distinct IDs and holders do not prove
independent review. No change to executable gates, TCP-00 dispositions, retry charging or completion.

## Failure modes and trust

Every refusal happens before a committed effect and leaves counters unchanged. Unknown policy,
subject, candidate-link, evidence or replay observations refuse rather than default to true. A
rehashed event whose bytes are well formed but whose counters, prior RETURN, context or subject do
not match the audited state fails recovery. Reason text and evidence labels are untrusted data. The
event, history and ticket bounds above, plus the existing 128 KiB ticket and store limits, cap cost.

## Acceptance evidence and traceability

This delivery supplies only the pure, focused evidence that the four seed files can establish. Every
integrated witness is NOT_RUN; no row claims installed native behavior.

| Requirement | Parent proposal | Delivered evidence (pure seed) | Required integrated evidence (NOT_RUN) |
|---|---|---|---|
| ERG-V0-001 | ER504-001 | `TestIssue504ReplayAndAuthority` (recorder roles, reviewer lease, operator attestation), `TestIssue504ResubmitAndSecondReturn` (author lease distinct from reviewer) | trusted native binding, live lease lookup, policy-required lease refusal through the CLI |
| ERG-V0-002 | ER504-002 | `TestIssue504BoundsAndUnknown` (EVIDENCE union encodes; an unproved candidate link refuses), `TestIssue504MaterialBindings` (subject receipt binding), `TestIssue504HistoricalPreservation` (candidate change) | inactive-member refusal test; real SUBMIT receipt lookup; EVIDENCE artifact producer |
| ERG-V0-003 | ER504-003 | `TestIssue504CanonicalRoundTrip`, `TestIssue504BoundsAndUnknown` in snapshot; `TestIssue504TypedRouting` (prose cannot change state) | measured worst-case event size; evidence storage |
| ERG-V0-004 | ER504-003 | `TestIssue504CanonicalRoundTrip` (closed per-gate reference codec only), `TestIssue504BoundsAndUnknown` (event revision 4097 refuses) | the 16-gate ticket map, ticket/Core optional-key reader, legacy byte identity, preservation by non-review writers |
| ERG-V0-005 | ER504-004 | `TestIssue504ResubmitAndSecondReturn`, `TestIssue504ReplayAndAuthority`, `TestIssue504BoundsAndUnknown` in transaction (same-CAS single winner over pure state) | two-writer CAS and replay through the native request index |
| ERG-V0-006 | ER504-002, F1 | `TestIssue504HistoricalPreservation`, `TestIssue504ResubmitAndSecondReturn/stale-resubmit-fresh-cycle`, `TestIssue504BoundsAndUnknown` | staleness from real acceptance/submission changes |
| ERG-V0-007 | ER504-005 | `TestIssue504CanonicalRoundTrip`, `TestIssue504TypedRouting` (GateResult decoder rejects the event) | completion predicates unchanged under native records |
| ERG-V0-008 | ER504-005, Gate A MED | `TestIssue504MaterialBindings` (rehashed wrong CAS, priorReturn, context, post and subject) | locked staged recovery and crash/redo fixtures |
| ERG-V0-009 | ER504-001, ER504-005, ER504-006 | none | writer, policy, CLI, history, workState adapter, dispatcher predicates, 1670-byte descriptor measurement |
| ERG-V0-010 | ER504-008, ER504-009 | none | full native fixture set, issue 394 producer, review, integration and native completion |

## Rollout and rollback

This delivery adds unreferenced pure code and this intent; it installs no feature and changes no
existing byte format, so rollback is reverting the four files and this spec's catalog entries. Later
slices wire the optional fields and native writer after the issue 501 derived-event slot exists, then
reads, the workState adapter and producers. Once writes exist, disabling new writes is reversible;
deleting events or pretending an older closed reader is compatible is not rollback.

## Open decisions

Owner acceptance of this intent; the exact shared derived-event slot with issue 501; the EVIDENCE
artifact-binding wire shape; and the issue 394 verdict mapping (accepted to PASS, rejected to RETURN,
blocked to no verdict is a technical proposal only).
