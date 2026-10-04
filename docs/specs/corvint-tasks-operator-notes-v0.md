# Corvint Tasks operator notes V0

Owner: Russell Lewis
Date: 2026-10-04
Intent status: accepted (owner decision 2026-10-04, relayed by the orchestrator session)
Delivery status: experimental

Authoritative inputs: owner request [issue 501](https://github.com/beamfall/corvint/issues/501)
(native ticket V1-0698); the issue's NOTE501-001..010 preparation intent with its independently
reviewed precision corrections, adopted with limits by owner-delegated decision 2026-10-04 (agent
decided); the owner's acceptance of this profile with its drafted defaults on 2026-10-04, relayed
by the orchestrator session and not directly observed by the implementing agent; and the existing
[agent lease contract](corvint-tasks-agent-leases-v0.md). The second delivery adds the native writer,
the record and Core codecs, the `ticket note` CLI and the `ticket show` view; claim delivery,
anchored history, per-event receipt-audit binding and durable qualification come later. The older TCP-00 task-store recovery contract remains partially
unrecovered; it is not the unrelated `task-context-packet-v0.md`/TCP-V0 spec. This extension
restates its own required ticket/mutation/receipt boundaries without waiving that recovery limitation.

## Agent digest
- Claim: Operators attach one current advisory note to a ticket, retain immutable history, and deliver the exact admitted note with each successful claim.
- Status: accepted (owner decision 2026-10-04, relayed by the orchestrator session); experimental; native NOTE_SET/NOTE_CLEAR writer, `ticket note set|clear|show` and the `ticket show` operatorNote view exist; claims do not yet deliver notes.
- Exists: closed reference/event codecs, the optional record/Core `operatorNote` reference, one measured MUTATE derived-event slot, explicit OPERATOR policy grants, ADOPT_FILE refusal, CLI and current-note reads with focused and native-store tests.
- Blocked on: claim-time delivery (ON-V0-007), anchored history pages, per-event receipt-audit and redo binding (ON-V0-006), durable qualification, and the existing TCP-00 active-staging recovery limit.
- Read next: Requirements; Failure modes and trust; Acceptance evidence and traceability; Rollout and rollback.

## User and current state

An operator needs subsequent workers to receive a single current piece of guidance without
repeatedly pasting older messages. The simpler baseline is manual guidance outside the ticket; it
gives neither immutable supersession history nor an exact claim-time snapshot. Notes are advisory
untrusted content, not new acceptance criteria, actor authentication, retry budget or a live-worker
message channel.

At public main `cd70ba0ab538a612a13d146934cf95ef356c333d` the pure codecs and transition helpers
existed but nothing called them. The second delivery wires them:

- The ticket record and the Core reader accept the optional closed `operatorNote` reference; a
  never-noted ticket keeps its exact legacy bytes, and every other unknown member still refuses.
- NOTE_SET and NOTE_CLEAR are ordinary mutation envelopes. `expectedRevision` may be null for them
  (as for CREATE); the payload is `{supersedes,text}` or `{supersedes}`.
- The pure Apply path proposes the event through `ProposeOperatorNote`, sets the reference on a
  clone of the audited ticket and finalizes it with the ordinary content finalizer, so the ticket
  revision advances by one and the acceptance revision is unchanged.
- The MUTATE stage carries the event as one content-addressed `evidence/<sha256>` POST beside the
  ticket post (the derived-event slot; measured below). The writer reads the prior event at the
  audited reference head; a missing prior event refuses with MISSING_EVIDENCE.
- `corvint-tasks ticket note set|clear|show` and the `ticket show` `operatorNote` view render the
  current note as advisory, untrusted operator prose.

Existing claim admission binds TicketRecordSha256 and acceptanceRevision; claims do not yet deliver
the note. An `evidence/` post on a receipt still downgrades receipt-audit semantic coverage to
UNKNOWN, as gate and REOPEN evidence already do; no per-event semantic binding exists yet. Existing
active-stage redo refusal remains.

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

The first delivery supplied pure evidence; the second adds native-store and CLI witnesses. Cells
under "Required integrated evidence" remain NOT_RUN.

| Requirement | Parent intent | Implementation boundary | Delivered evidence | Required integrated evidence (NOT_RUN) |
|---|---|---|---|---|
| ON-V0-001 | NOTE501-001 | ticket/record codecs; Core decode | `TestIssue501_NoteCodec`; `TestONV0001_RecordCodecKeepsLegacyBytes` (legacy bytes unchanged, SET/CLEARED references round-trip); `TestONV0001_CoreReaderAdmitsOnlyTheClosedNoteReference` (Core admits valid references and refuses malformed ones and other unknown members) | archive export/import of noted tickets |
| ON-V0-002 | NOTE501-002 | ticket event codec; evidence storage | `TestIssue501_NoteCodec`, `TestIssue501_NoteCASAndBounds`; `TestONV0006_NativeNoteSetClearReplayAndAudit` (SET then CLEAR chain stored in evidence and resolved) | history pages over a long chain |
| ON-V0-003 | NOTE501-003 | mutation payload/Apply; native writer; CLI | `TestIssue501_NoteTransition`, `TestIssue501_NoteMaterialBindings`; `TestONV0006_NativeNoteSetClearReplayAndAudit`, `TestONV0008_TicketNoteSetShowClearThroughTheCLI` (identical replay writes no receipt, stale supersedes is REVISION_CONFLICT) | concurrent CAS between two processes |
| ON-V0-004 | NOTE501-004 | policy/authorization; indirect writers | `TestIssue501_NoteCASAndBounds`; `TestONV0004_NotePolicyNeedsAnExplicitOperatorRow` (OPERATOR needs an explicit row naming the verb, OWNER default allows, a narrowed OWNER row refuses, a WORKER row naming NOTE_SET does not decode) | import-owned ticket refusal through the writer |
| ON-V0-005 | NOTE501-005 | whole canonical post adapter/finalizer; ADOPT_FILE | `TestONV0006_NativeNoteSetClearReplayAndAudit` (acceptance revision unchanged, REFINE preserves the reference); `TestONV0005_AdoptFileRefusesNoteReferenceChanges` | rehashed unrelated-field attacks in receipt audit |
| ON-V0-006 | NOTE501-006 | transaction/snapshot/journal/stage/archive | `TestONV0006_DerivedEventSlotMeasuredAndNarrow` (worst case 6/6 artifacts and 1669/1670 bytes; second event, queue, request, oversized, wrong-address and non-MUTATE widenings refuse); `TestONV0006_NativeNoteSetClearReplayAndAudit` (store digest unchanged after replay and conflict) | per-event receipt-audit binding and supported redo |
| ON-V0-007 | NOTE501-007 | lease admission; original receipt materializer | none | replacement between commit/materialization, generation refresh, missing-evidence replay |
| ON-V0-008 | NOTE501-008 | show/history; CLI | `TestIssue501_NoteTransition`; `TestONV0008_TicketNoteSetShowClearThroughTheCLI` (NONE, CURRENT and CLEARED views; a missing event is MISSING_EVIDENCE in `ticket note show` and JOURNAL_FORKED in `ticket show`, never NONE) | anchored history reads, page bounds, read-only proof over noted stores |
| ON-V0-009 | NOTE501-009 | foundation and dependency layering | the first-delivery pure tests; `ticket` does not import `mutation` | no premature promotion |
| ON-V0-010 | NOTE501-010 | qualification and delivery | none | exact-source review, integration, evidence and compatible rollback |

## Resolved decisions (conservative defaults, owner-accepted with the profile)

- Capacity refusal mapping: a note or ticket revision overflow reported as wire LIMIT_EXCEEDED maps
  to VALIDATION_FAILED with code LIMIT_EXCEEDED, following the existing revision-overflow
  precedent, not CAPACITY_EXHAUSTED. A BLOCKED note refusal carries TICKET_STATE; UNAUTHORIZED and
  REVISION_CONFLICT carry no code.
- Event slot: one content-addressed `evidence/<sha256>` POST whose digest differs from the request
  digest, at most 65536 bytes, at most one per MUTATE, never with the CREATE queue post or the
  mutation-request post. The measured worst case is 6/6 artifacts and 1669/1670 bytes, so the
  existing MUTATE bounds are unchanged.
- Policy: OPERATOR may write notes only through an explicit `policy.roles.OPERATOR` row naming
  NOTE_SET or NOTE_CLEAR; its default row omits them. OWNER's default row includes them and an
  explicit OWNER row narrows it. No other role may be granted them.
- `ticket show` renders a missing or mismatched event explicitly (UNAVAILABLE with its code when no
  journal audit runs; the audit otherwise refuses the read), never as NONE.
- Retries: as for other CLI mutations, issuedAt defaults to now, so a retry passes `--issued-at`
  to replay.

## Unresolved decisions

- Whether claim delivery needs an amendment to the agent lease contract, decided in the
  claim-delivery slice.
- The relayed owner acceptance was not directly observed by the implementing agent; confirm before
  promotion.

Kill criterion: stop or narrow the capability if bounded original material cannot be recovered
without an unauthorized general recovery rewrite.

## Rollout and rollback

The feature remains experimental. Until a ticket is noted, no existing byte format changes. Rollback
before any note is written reverts the writer, CLI, codec and stage-slot changes, this spec's
INDEX and README entries, its generated REQUIREMENTS.tsv rows and the build-log entries. Once notes
exist, an older reader refuses noted tickets (closed key set), so rollback first disables further
writes and keeps the immutable events; deleting history or claiming an older closed reader is
compatible is not rollback. Promotion requires claim delivery, receipt-audit binding, the complete
evidence matrix and native closeout.
