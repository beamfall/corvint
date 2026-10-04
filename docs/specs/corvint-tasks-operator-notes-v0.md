# Corvint Tasks operator notes V0

Owner: Russell Lewis
Date: 2026-10-04
Intent status: accepted by owner-delegated decision 2026-10-04 (agent decided)
Delivery status: experimental

Authoritative inputs: owner request [issue 501](https://github.com/beamfall/corvint/issues/501)
(native ticket V1-0698); the issue's NOTE501-001..010 preparation intent with its independently
reviewed precision corrections, adopted with limits by owner-delegated decision 2026-10-04 (agent
decided); and the existing [agent lease contract](corvint-tasks-agent-leases-v0.md). That adoption
is an agent decision under the owner's delegation, not a direct owner statement. It limits the
first delivery to the pure four-file foundation plus this intent; writer, CLI, claim delivery and
durable qualification come later. The older TCP-00 task-store recovery contract remains partially
unrecovered; it is not the unrelated `task-context-packet-v0.md`/TCP-V0 spec. This extension
restates its own required ticket/mutation/receipt boundaries without waiving that recovery limitation.

## Agent digest
- Claim: Operators attach one current advisory note to a ticket, retain immutable history, and deliver the exact admitted note with each successful claim.
- Status: accepted by owner-delegated decision 2026-10-04 (agent decided); experimental delivery of the pure four-file foundation only; no installed note capability.
- Exists: closed reference/event codecs, historical resolution, and pure transition/material-binding helpers with focused tests; nothing calls them yet.
- Blocked on: native writer/material/read/claim/CLI integration, original-receipt proofs, durable qualification, and the existing TCP-00 active-staging recovery limit.
- Read next: Requirements; Failure modes and trust; Acceptance evidence and traceability; Rollout and rollback.

## User and current state

An operator needs subsequent workers to receive a single current piece of guidance without
repeatedly pasting older messages. The simpler baseline is manual guidance outside the ticket; it
gives neither immutable supersession history nor an exact claim-time snapshot. Notes are advisory
untrusted content, not new acceptance criteria, actor authentication, retry budget or a live-worker
message channel.

At public main `4b10a02144faed4a77b36eba4b0d1cbc315706d3`, the ticket record and Core codecs do not
accept the new optional reference, and the native MUTATE/material/claim routes do not implement this
profile. The first delivery adds only `internal/tasks/ticket/operator_note.go`,
`internal/tasks/mutation/operator_note.go` and their tests: pure codecs and proposed
reference/event computation that no writer, reader, CLI verb or claim path calls. Existing claim
admission binds TicketRecordSha256 and acceptanceRevision; `store/lease.go` materializes the
original receipt POST attempt. Existing active-stage redo refusal remains. Current source and all
capacity claims require exact-target evidence before promotion.

Explicitly deferred from this delivery: the record-codec wiring of the reference, the native note
writer and material validation, receipt audit and redo, the `ticket note` CLI and show/history
reads, claim-time delivery (including any agent-lease CAL amendment), the Core reader, and durable
native qualification.

## Requirements

- `ON-V0-001`: The ticket MUST carry at most one optional closed operatorNote reference, omitted before the first successful note operation.
  When present it contains exactly revision (canonical Count 1..4096), current (Digest or null), and head (Digest). SET has current=head; CLEAR has current=null and head naming its tombstone. Whole-reference null, aliases, duplicate/unknown keys and invalid types refuse. The mandatory TicketRecordKeys set remains unchanged. Legacy no-note canonical bytes remain identical; all ordinary record mutations preserve the reference. Core's separate reader validates the optional field using its allowed shared wire primitives without importing ticket/mutation or admitting arbitrary unknown fields.

- `ON-V0-002`: Every distinct committed SET or CLEAR MUST create one immutable canonical taskman-operator-note/0 event, linked to the previous head and stored in existing evidence by its digest.
  The closed canonical keys are profile, ticketId, noteRevision, ticketRevision, acceptanceRevision, operation, previous, actor, recordedAt, requestSha256 and request. Counts use canonical Count strings; operation is SET or CLEAR; previous is null exactly at revision 1. Actor is the trusted invocation id/role OWNER or OPERATOR, recordedAt is writer queue time, and request is the original canonical mutation envelope. requestSha256 is its SHA256 including canonical LF. SET prose occurs once in nested payload, is valid UTF-8 of 1..8192 bytes and not whitespace-only; CLEAR has no text member. Entire event including escaping/LF is at most 65536 bytes. Nested operation/target/actor match the event; issuedAt remains request metadata. Distinct same-text SET and CLEAR of NONE/already-cleared create new events. No pruning: revision 4097 or any count/record/store/receipt capacity overflow refuses before committed change. Orphaned pre-receipt evidence is not CURRENT authority.

- `ON-V0-003`: NOTE_SET and NOTE_CLEAR MUST use the existing mutation envelope and original-request replay rules with explicit note and optional content CAS.
  SET payload is exactly {text,supersedes}; CLEAR is exactly {supersedes}. supersedes is required Count or null; CLI omission maps to null. Zero means never-noted NONE, not a cleared reference; positive values compare to the prior note event revision, including CLEAR, under the writer lock. Mismatch returns existing REVISION_CONFLICT without effects. For these verbs only, expectedRevision may be null or an explicit prior ticket-content revision. Other verbs retain their rules. CLI set/clear require target and request ID, expose optional supersedes/content expected revision, and do not synthesize current revision or a new issuedAt into replay bytes. Payload cannot inject actor/time/history/current/head. Preserve actor-binding and canonical replay-before-fresh-role/CAS/state ordering. Identical request returns original event/time/revisions despite successors; changed request bytes conflict. Same-CAS concurrent writes have one winner; unconstrained writes serialize without hidden retries.

- `ON-V0-004`: Fresh note writes MUST respect existing ticket ownership/state and explicitly granted operator policy authority.
  OWNER remains subject to existing policy narrowing. OPERATOR needs an explicit policy.roles.OPERATOR row granting the note verb; omission grants nothing. Vocabulary may allow such explicit rows but does not create a default note grant. WORKER, REVIEWER, IMPORTER and SYSTEM cannot inject/change note state through CREATE, REFINE, ADOPT_FILE or raw import/reconciliation. Existing DRAFT/OPEN/HELD/COMPLETED tickets may receive notes while a live attempt exists; ARCHIVED and SHADOW/import-owned records refuse until existing native prerequisites are satisfied. Missing tickets are not created. Reads identify advisory/untrusted provenance and existing actor-authentication limits; note text cannot grant authority to execute instructions.

- `ON-V0-005`: A fresh note-only transition MUST preserve the complete audited ticket except its note reference and ordinary content-finalizer fields.
  Exactly operatorNote, one content revision increment, previousRecordSha256, updatedAt and updatedBy may change. acceptanceRevision and every other field, including approvals, acceptance criteria, dependencies, required gates, retry debt and attempt state, remain unchanged. Native material validation computes the complete expected canonical post-ticket from the audited canonical pre-ticket through the ordinary finalizer or equivalent whole-record equality; scalar/event/reference equality alone is insufficient. operatorNote is not acceptance-relevant. Existing executable gates bind acceptanceRevision, so a note-only change does not artificially invalidate them; content CAS still becomes stale. New claims use the latest admission digest; existing attempts retain their original digest. KEEP_JOURNAL reconciliation compares canonical history, never treating a divergent physical preimage as note authority.

- `ON-V0-006`: Note mutation, receipt audit and supported redo MUST prove the same complete original transition through the existing bounded MUTATE writer.
  One transaction posts ticket and exactly one matching note event with one request/receipt/head. Permit only the narrow optional note-event slot distinguished from the existing request-envelope or CREATE queue slot, never an arbitrary evidence widening; the six-artifact MUTATE limit and the existing 1670-byte operation-descriptor bound remain binding. Actual maximum encoding must be measured, not inferred from arithmetic. Bind original queue/request ID, canonical request digest, actor and recordedAt to receipt/descriptor; explicit content CAS compares audited pre-ticket revision, supersedes compares audited prior note revision, and previous/head/current/event post revisions match the complete expected post. Capture original canonical pre-ticket/pre-policy before folding the receipt; resolve exact retained POST bytes or full-audit fallback, never current projection/result revision as pre-state. Detect any note-reference addition/change/removal even with no note event: note operations have exactly their event, non-note operations preserve the reference. Validate no-staging receipts too. Stage observers must supply actual bounded artifacts and trusted pre-state or refuse unproved stages. Hash/classification alone is insufficient, and unrelated evidence coverage is not upgraded. Active staging recovery remains unsupported where existing store boundaries refuse it; label supported no-active-stage redo separately. Preserve the 128 KiB ticket, 64 KiB inline, evidence-store and receipt limits and archive reachable blobs/links after CLEAR.

- `ON-V0-007`: Every successful fresh or replayed claim and claim-next MUST deliver the operator note pinned by that original admission, never the latest projection.
  Add an optional reference to the receipt POST Attempt only when the admitted ticket has one; legacy attempts omit it. Copy reference and TicketRecordSha256 from the same audited admitted ticket. Each new generation replaces the snapshot from its own admission, not a prior generation. Resolve the original receipt POST attempt's exact digest into NONE, CLEARED or CURRENT with text/provenance, note revision/head and source ticket digest. Missing/corrupt evidence returns visible MISSING_EVIDENCE/JOURNAL_FORKED rather than NONE. If the lease already committed before response materialization fails, retain attempt/generation/receipt identity and make exact request replay usable; never automatically claim again. A postcommit concurrent replacement changes neither response nor replay; a future claim may receive the newer note. No new claim input, scope, resource authority or retroactive active-worker guidance is added.

- `ON-V0-008`: Current-note and anchored-history reads MUST be bounded, immutable reads with explicit evidence failures.
  Ticket show and ticket note show expose one current note: NONE has revision 0/current null; CLEARED retains tombstone/head/revision/provenance; CURRENT exposes its SET text/provenance/head. Validate only the referenced head for ordinary current read. Event ticket/acceptance revisions remain their original note-write values and need not equal a later ticket after unrelated edits, while original material validation still binds its own transaction post. History defaults to 20 nodes, permits 1..50 and at most 1 MiB output, newest-first including CLEAR. Opaque canonical cursor binds queue/ticket/anchor/next digest/expected revision, never a path; concurrent replacement does not move subsequent pages off the anchor. Verify hash/profile/ticket, exact decreasing revisions and links; missing/cyclic/mismatched evidence refuses rather than fabricating complete history. Stop before page bytes bound with a next cursor; one valid event fits. Reads create no state, ledger or hydrated evidence.

- `ON-V0-009`: The codec/transition foundation and integrated capability MUST have distinct evidence and layering boundaries.
  The four ticket/operator_note.go, ticket/operator_note_test.go, mutation/operator_note.go and mutation/operator_note_test.go files provide proposed immutable reference/blob computation, not installed CLI/native authority. ticket's nested-envelope codec must not import mutation; mutation owns semantic trusted-context comparisons. Core retains its wire-only Tasks dependency. Native qualification starts with a canonically initialized disposable SET, show, identical replay and receipt-audit fixture plus a legacy-byte control. Reached malformed-material and crash witnesses must separate fixture failure from product effects. Passing pure helpers cannot establish integrated note writes, history, claim delivery, capacity or recovery.

- `ON-V0-010`: Promotion and rollback MUST preserve all existing evidence and explicit integration limits.
  Require native CAS/race/material/redo/capacity, archive/restore, immutable reads, original-receipt claim snapshots and note-only gate-preservation evidence, independent exact-source review and current-main integration before calling the capability implemented. Keep active-staging, platform and external qualification limits visible. Rollback disables new writes/UI consumption but retains compatible readers, all references/blobs/receipts/history; never strip fields or downgrade an active store to an incompatible decoder. No direct worker messaging, acceptance override, budget/retry grant, history-as-current guidance or typed request system is included. Same-target spec/catalog/build-log/CEM/dogfood/check/seal and native completion/readback/audit remain required delivery boundaries.

## Failure modes and trust

CAS/policy/state/capacity refusals occur before committed effects. A committed claim can still have
a failed response materialization; preserve its identity for replay. A partial receipt or active
staging is not automatically recoverable merely because an event hash matches. Missing original
canonical ticket/policy/request material produces refusal/UNKNOWN or an explicit full-audit
fallback, never latest-state reconstruction. Rehashed malicious receipts with valid
event/ref/revisions but altered approvals/gates/dependencies/acceptance must fail full post-image
validation; missing/replaced/removed references without matching events must fail both note and
non-note receipt audit. Untrusted note prose remains data, not policy.

The existing global store/archive limits govern total cost. This profile adds no daemon, account,
network service, new mutable store or general recovery algorithm. Event history is bounded at 4096
events and each page at 50 nodes/1 MiB. All writes use the native authority lock and writer; all
reads retain the no-write invariant. Historical policy is the actual original pre-policy, not
retroactively narrowed current policy. Archived and import-owned content cannot bypass native
ownership with a note reference.

## Non-goals and simpler baseline

The simpler baseline is guidance kept outside the ticket. This profile does not add direct or live
worker messaging, acceptance overrides, budget or retry grants, a typed request system, actor
authentication, history used as current guidance, or retroactive guidance to an active attempt.
It adds no daemon, account, network service or new mutable store.

## Acceptance evidence and traceability

The first delivery supplies only pure, focused evidence for the parts of ON-V0-001..004,
ON-V0-008 and ON-V0-009 that the four files can establish without wiring. Every integrated witness
is NOT_RUN; no row below claims delivered native behavior.

| Requirement | Parent intent | Implementation boundary | Delivered evidence | Required integrated evidence (NOT_RUN) |
|---|---|---|---|---|
| ON-V0-001 | NOTE501-001 | ticket/record codecs; Core decode | `TestIssue501_NoteCodec` (closed reference: unknown key, whole-null, zero/4097/overflow revision, uppercase digest, current/head mismatch, wrong-typed members and duplicate keys refuse) | legacy bytes through the record codec, ordinary preservation, Core reader |
| ON-V0-002 | NOTE501-002 | ticket event codec; evidence storage | `TestIssue501_NoteCodec`, `TestIssue501_NoteCASAndBounds` (closed event keys, CLEAR without text, SET text bounds, codec-level maximum escaped event within 65536 bytes, revision 4097 refusal) | evidence storage, SET/CLEAR chain on a native store |
| ON-V0-003 | NOTE501-003 | mutation payload/Apply; native writer | `TestIssue501_NoteTransition`, `TestIssue501_NoteCASAndBounds`, `TestIssue501_NoteMaterialBindings` (supersedes and content CAS, request/queue bindings) | identical replay after successors, conflict, concurrent CAS through the writer and CLI |
| ON-V0-004 | NOTE501-004 | policy/authorization; indirect writers | `TestIssue501_NoteCASAndBounds` (absent caller-supplied grant and mismatched binding refuse; ARCHIVED refuses BLOCKED; shadow/import refuses UNAUTHORIZED) | explicit operator row, narrowed owner, state/role/import refusal through the writer |
| ON-V0-005 | NOTE501-005 | whole canonical post adapter/finalizer | none | normal note-only gates plus rehashed unrelated-field attacks |
| ON-V0-006 | NOTE501-006 | transaction/snapshot/journal/stage/archive | none; the pure material rebinding in `TestIssue501_NoteMaterialBindings` is supporting input only | original pre-policy/pre-ticket, no-stage and supported redo, measured descriptor bounds |
| ON-V0-007 | NOTE501-007 | lease admission; original receipt materializer | none | replacement between commit/materialization, generation refresh, missing-evidence replay |
| ON-V0-008 | NOTE501-008 | show/history; CLI | `TestIssue501_NoteTransition` (after unrelated edits advance the ticket and acceptance revisions, the next note still proposes and validates; a prior note newer than the audited ticket refuses) | current/history reads, anchor concurrency, page bounds, corruption refusal, no reads mutate |
| ON-V0-009 | NOTE501-009 | four-file foundation and dependency layering | `TestIssue501_NoteCodec`, `TestIssue501_NoteTransition`, `TestIssue501_NoteCASAndBounds`, `TestIssue501_NoteMaterialBindings`; `ticket` does not import `mutation` | first native SET/show/replay/audit fixture; no premature promotion |
| ON-V0-010 | NOTE501-010 | qualification and delivery | none | exact-source review, integration, evidence and compatible rollback |

The pure tests do not supply integrated material-preservation, journal, archive, claim or platform
evidence. The 1670-byte MUTATE descriptor encoding and worst-case escaped ticket record sizes remain
unmeasured until the actual writer tests run.

## Unresolved decisions

- Outcome mapping at the writer boundary: the pure helpers return the existing REVISION_CONFLICT,
  UNAUTHORIZED and BLOCKED outcomes, but report note/ticket revision capacity as a wire
  LIMIT_EXCEEDED error. Whether the writer maps that to the existing CAPACITY_EXHAUSTED outcome is
  decided in the writer slice.
- The exact optional note-event slot in the MUTATE descriptor and its measured maximum encoding.
- Whether claim delivery needs an amendment to the agent lease contract, decided in the
  claim-delivery slice.

Kill criterion: stop or narrow the capability if bounded original material cannot be recovered
without an unauthorized general recovery rewrite.

## Rollout and rollback

This delivery adds unreferenced pure code and intent only; it installs no feature and changes no
existing byte format. Rollback reverts the four files, this spec, its INDEX and README entries, its
generated REQUIREMENTS.tsv rows and the build-log entry. Later slices integrate current-main codecs,
writer, material validation, reads, claim delivery and CLI under exact source resources, preserve
recorded 503 claimability changes, and retain each failure/UNKNOWN outcome. Promotion requires the
complete evidence matrix and native closeout. Once writes exist, disabling future writes is
reversible; deleting immutable history or pretending an older closed reader is compatible is not
rollback.
