# Issue 501 operator-note claim delivery and audit binding (V1-0698)

## Decision

The third issue 501 delivery adds ON-V0-007 claim delivery and the ON-V0-006 per-event
receipt-audit and redo binding to `docs/specs/corvint-tasks-operator-notes-v0.md`. These are agent
decisions within the profile the owner accepted on 2026-10-04.

- Audit binding is whole-transition replay. The journal folds each receipt and binds any note event
  by running the ordinary NOTE_SET/NOTE_CLEAR Apply path (`mutation.ReplayOperatorNote`). The
  replay reads the exact retained pre-ticket, pre-policy and prior-event bytes that the audited
  chain names, then requires the posted ticket and event to match byte for byte. Checking hashes or
  reference fields alone would not satisfy the requirement that rehashed, altered approvals fail.
  Every non-note receipt must leave each walked ticket's reference unchanged. Redo uses the same
  PRE_OR_POST audit, so it inherits the binding without a separate path.
- The checkpoint tail never reads its prefix. A note event whose pre-state is before the checkpoint
  returns the existing "checkpoint unusable" fallback to the complete audit. A non-note reference
  change whose prior is before the checkpoint goes undetected by that read only. Mutations, redo
  and receipt audit always use the complete audit. The spec records this limit.
- Claim delivery pins the reference on the attempt at admission, beside TicketRecordSha256. The
  reference is copied in `transaction.admitted`. It is then resolved from the content-addressed
  evidence store when the claim result is built, so exact replay reproduces the original note.
  No agent-lease contract amendment is needed: the optional attempt member is defined by this
  profile, and legacy attempts omit it.
- The seam is shaped for #502. `store.ClaimDelivery` has one member per delivered profile, and the
  claim result has one top-level key per profile. Escalation answers would add an optional attempt
  member, a `ClaimDelivery` member and a result key beside `operatorNote`. This slice implements no
  escalation behaviour.
- Hunks are kept minimal in files that other lanes rewrite. V1-0645 is rewriting
  `journal/records.go`; this slice adds four hooks there. Files shared with the #502 writer get one
  or two lines each: `store/store.go`, `store/lease.go` and `transaction/lease_claim.go`. The
  binding logic lives in the new files `journal/operator_note.go` and `store/claim_delivery.go`.

## Evidence

The focused tests are named in the spec's traceability table. Writing
`TestONV0007_ClaimResultCarriesTheDeliveredNote` exposed a nil dereference in the UNAVAILABLE
rendering, which was fixed before commit. A missing pinned event makes the claim replay refuse with
JOURNAL_FORKED from the complete audit, before any delivery. The store read reports
MISSING_EVIDENCE. The CLI UNAVAILABLE rendering is therefore defensive and is covered by a unit
test.

## Limits

Still outstanding: history pages, dispatch and supervised-run rendering of the delivered note,
crash and active-staging recovery witnesses, durable qualification, and native closeout.
