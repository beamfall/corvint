# Corvint Tasks evidence attachments V0

Owner: Russell Lewis
Date: 2026-10-05
Intent status: proposed
Delivery status: experimental

Authoritative inputs: the agent-filed native ticket V1-0808 ("attach evidence to an OPEN ticket
without changing it"), implemented on orchestrator instruction on 2026-10-05; owner acceptance of
the drafted defaults below is pending. The existing
[operator notes profile](corvint-tasks-operator-notes-v0.md) (ON-V0-005, a revision-only write that
preserves the audited record) and the ticket/mutation/receipt boundaries of the
[agent lease contract](corvint-tasks-agent-leases-v0.md) are reused unchanged.

## Agent digest
- Claim: Writers attach sha256 evidence digests and a reason to an OPEN native ticket in one receipt-backed write that leaves acceptance, status and gates unchanged.
- Status: proposed; experimental; the native `ticket attach-evidence` writer (operation `ATTACH_EVIDENCE`), the optional record/Core `attachedEvidence` member and the `ticket show` listing exist with focused tests.
- Exists: closed payload and record codecs, Core reader validation, Apply/finalize through the existing mutation path, OWNER default and explicit OPERATOR grant, ADOPT_FILE and IMPORT_APPLY refusal, CLI help, replay and receipt-audit witnesses.
- Blocked on: owner acceptance of the resolved defaults; an untested native archive round trip of tickets that carry attachments; durable qualification.
- Read next: Requirements; Resolved decisions; Acceptance evidence and traceability; Rollout and rollback.

## User and current state

An agent or operator often holds evidence for an OPEN ticket before it can complete it: a focused
test log, a review transcript or a reproduction. Before this profile the only recorded evidence was
the `evidence` digest list of COMPLETE_MANUAL, which closes the ticket, or a REFINE of the body,
which is unstructured prose. Neither attaches content-addressed evidence to a ticket that stays
OPEN. The simpler baseline is pasting digests into the body; that changes no acceptance state
either, but it gives no bounded structure, no actor or time binding and no duplicate check.

This profile adds one ordinary mutation envelope. It reuses the request-id replay (TM-V0-006), the
expectedRevision CAS, Apply, finalize, the journal append and the fold path; no new receipt kind,
derived event, lock or recovery path exists.

## Requirements

- `TEA-V0-001`: `corvint-tasks ticket attach-evidence --target T --expected-revision N --payload P`
  MUST apply operation `ATTACH_EVIDENCE` with the closed payload `{evidence, reason}` (1..16
  sorted unique lowercase sha256 digests and nonblank prose of 1..512 bytes) to an OPEN NATIVE
  ticket as one receipt-backed revision. The writer MUST append exactly one entry
  `{acceptanceRevision, actor, evidence, reason, recordedAt}` to the optional record member
  `attachedEvidence`, taking actor and time from the trusted binding and clock and the acceptance
  revision from the current record. The post record MUST equal the pre record in every member
  except `attachedEvidence` and the finalizer fields `revision` (+1), `previousRecordSha256`,
  `updatedAt` and `updatedBy`; `acceptanceRevision`, status, required gates, criteria, completion
  and every gate result bound to the acceptance revision MUST stay unchanged. The writer MUST
  refuse a non-OPEN or IMPORT ticket BLOCKED/TICKET_STATE, a digest already attached at the
  current acceptance revision VALIDATION_FAILED/DUPLICATE_ID, a 33rd entry
  VALIDATION_FAILED/LIMIT_EXCEEDED, a malformed payload MALFORMED, a stale expectedRevision
  REVISION_CONFLICT, and any role other than OWNER (default) or OPERATOR (only through an explicit
  `policy.roles.OPERATOR` row naming `ATTACH_EVIDENCE`) UNAUTHORIZED. An identical retry MUST
  replay without a new receipt. `ticket show` MUST list the entries; receipt audit MUST stay
  CONSISTENT; ADOPT_FILE and IMPORT_APPLY MUST refuse any `attachedEvidence` difference; the Tasks
  and Core readers MUST refuse a malformed or record-inconsistent member and keep every other
  unknown member closed. Attached evidence MUST NOT satisfy a gate, a criterion or completion.

## Failure modes and trust

- Attached digests are claims by the writing principal, not verified content. Nothing resolves,
  fetches or checks the digested bytes, and no gate, criterion binding, completion or dispatch
  decision reads the member. The reason is untrusted prose.
- A crash between stage and commit is the existing ordinary-mutation case: the transaction either
  commits one receipt or leaves nothing; redo and receipt audit replay the receipt structurally
  like every other inline mutation. No derived event is declared.
- Concurrency: two writers racing on the same ticket serialize through the store lock and the
  expectedRevision CAS; the loser sees REVISION_CONFLICT and may re-read and retry.
- A hand edit of the intent file that adds, rewrites or drops an entry is refused by ADOPT_FILE as a
  protected field before any composition, and the ticket stays diverged.
- Revision bump: the write consumes one ticket revision. A live attempt's admission binds the
  acceptance revision and the claimed record's own digest, not the current record digest, so an
  attachment during a live attempt neither blocks nor invalidates it (as for operator notes).

## Non-goals and simpler baseline

- No evidence storage, upload, fetch or verification; digests only.
- No removal or edit verb; a mistaken attachment stays in the audited history and a later entry
  can explain it.
- No attachments on DRAFT, HELD, COMPLETED, ARCHIVED or IMPORT tickets.
- No change to completion, gates, criterion binding, release or dispatch semantics.
- No foreign-import carrier: the closed key set of the foreign-ticket importer (`ticket import`)
  still refuses an export item that carries the member. Native archive export captures ticket
  files verbatim, so an attachment-bearing record should survive an archive round trip, but that
  round trip is untested here.

## Acceptance evidence and traceability

| Requirement | Ticket acceptance | Implementation boundary | Delivered evidence | Required integrated evidence (NOT_RUN) |
|---|---|---|---|---|
| TEA-V0-001 | V1-0808 criteria 1-4 | `internal/tasks/mutation` (payload, Apply, adopt); `internal/tasks/ticket` (record codec, view); `internal/tasks/intent` (policy grant); `internal/tasks/transaction` (import guard); `internal/taskman` (Core reader); `internal/tasks/cli` (verb, help) | `TestTEAV0001_AttachEvidenceLeavesTheRecordUnchanged` (whole-record comparison; acceptanceRevision, status and gates unchanged; live attempt does not block; same digest admitted again after the acceptance revision moves); `TestTEAV0001_AttachEvidenceRefusals` (DRAFT/HELD/COMPLETED/ARCHIVED and IMPORT BLOCKED/TICKET_STATE; malformed payloads; 17 digests; 33rd entry LIMIT_EXCEEDED; DUPLICATE_ID; every non-OWNER default role UNAUTHORIZED; explicit OPERATOR row admits; a WORKER row naming the verb does not decode; stale CAS REVISION_CONFLICT); `TestTEAV0001_RecordCodecRefusals`; `TestTEAV0001_AdoptFileRefusesAttachedEvidence`; `TestTEAV0001_ImportApplyNeverCarriesAttachedEvidence`; `TestTEAV0001_ReaderAttachedEvidence` and `TestIssue502_ReaderAdmitsSharedOptionalKeys` (Core reader over the shared fixture); `TestTEAV0001_TicketAttachEvidenceThroughTheCLI` (receipt, unchanged acceptance revision, `ticket show` listing, record unchanged member by member, replay, DUPLICATE_ID, receipt audit CONSISTENT/AGREES, help) | native archive round trip of an attachment-bearing store; concurrent two-process CAS; interrupted-commit redo of an attachment receipt; durable qualification |

## Resolved decisions (drafted defaults, owner acceptance pending)

1. Verb and operation: `ticket attach-evidence`, operation `ATTACH_EVIDENCE`, payload
   `{evidence, reason}`. It reuses the generic mutation verb surface (`--target`,
   `--expected-revision`, `--payload`, `--request-id`, `--issued-at`) instead of adding flags.
2. `--expected-revision` is required. The write is a content change to the canonical record, so the
   ordinary CAS applies; a null expectedRevision would let a stale writer append blindly.
3. Revision +1, acceptanceRevision unchanged. The record bytes change, so the previousRecordSha256
   chain and the CAS need a new revision (ON-V0-005 precedent). The acceptance revision does not
   move because no acceptance-relevant member changes; gate results stay bound.
4. OPEN only, fail-closed. HELD, DRAFT, COMPLETED and ARCHIVED refuse. Whether HELD should admit
   attachments is an open owner question.
5. NATIVE only. An IMPORT ticket's content is owned by its source queue and is overwritten by
   IMPORT_APPLY, which refuses any attachment difference.
6. Duplicates: a repeated digest inside one payload is MALFORMED (set reader); a digest already
   attached at the current acceptance revision is DUPLICATE_ID. The same digest may be attached
   again after the acceptance revision moves, because it then supports different acceptance.
7. Idempotency: the existing request-id replay. An identical retry returns the original result
   with `replayed: true` and writes nothing; a reused request id with a different payload conflicts.
8. Role: OWNER by default. OPERATOR only through an explicit `policy.roles.OPERATOR` row (the same
   explicit-grant class as the note and escalation verbs), so existing policies do not silently
   widen. WORKER, REVIEWER, IMPORTER and SYSTEM cannot be granted it.
9. Bounds: at most 32 entries per record, 16 digests per entry and 512 bytes of reason, which keeps
   the worst case well inside the 128 KiB record bound.
10. "Record unchanged" means unchanged except the attachment member and the four finalizer fields
    every write rewrites; a byte-identical record is impossible for a receipt-backed write that
    must chain previousRecordSha256.

## Unresolved decisions

- Should HELD tickets admit attachments?
- Should OPERATOR receive the verb by default instead of by explicit row?
- Are the 32/16/512 bounds right?
- Should this live in its own profile (TEA-V0) or fold into an existing Tasks profile?
- Should the foreign-ticket importer carry the member?

## Rollout and rollback

Rollout is additive: a record without attachments keeps its exact legacy bytes, and existing
policies do not grant OPERATOR the verb. Rollback is reverting the change before any store holds a
record with `attachedEvidence`. After such a write, an older binary refuses that record at the
closed reader (fail-closed, no silent loss); recovery is to run the newer binary or restore the
store from a backup taken before the first attachment.
