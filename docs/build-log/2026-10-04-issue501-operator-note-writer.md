# Issue 501 operator-note writer, CLI and show view (V1-0698)

## Decision

The owner accepted `docs/specs/corvint-tasks-operator-notes-v0.md` (ON-V0) as drafted, with its
current defaults, on 2026-10-04. The owner gave that acceptance directly, through a structured
question in the orchestrator session. The spec header, its INDEX entry and its README row record
the intent as `accepted (owner decision 2026-10-04)`, and `go test ./internal/specindex/` was rerun
after the change.

This slice wires the pure foundation into the native store. Issues 502 (typed escalations) and 504
(review verdicts as gate results) will reuse the MUTATE derived-event slot. For that reason the slot
landed first as a separate commit, `7d2ba3ed7db02a31facd9fc6117d436b11d1532c`.

## Conservative defaults recorded in the spec

- **Capacity refusals.** A wire LIMIT_EXCEEDED becomes VALIDATION_FAILED/LIMIT_EXCEEDED, matching
  the existing revision-overflow precedent rather than CAPACITY_EXHAUSTED. A BLOCKED refusal
  carries TICKET_STATE.
- **Derived-event slot.** A MUTATE may carry one content-addressed `evidence/<sha256>` POST whose
  digest is not the request digest. It is capped at 65536 bytes, appears at most once, and never
  appears beside the CREATE queue post or the mutation-request post. The worst case was measured
  at 6/6 artifacts and 1669 of 1670 bytes, so the MUTATE limits did not change.
- **Note authority.**
  - OPERATOR can write notes only through an explicit policy row
    (`intent.ExplicitGrantOperations`).
  - OWNER's default row includes the note operations, and an explicit OWNER row narrows it.
  - No other role can be granted them.
- **Expected revision.** `expectedRevision` may be null for note operations; supersedes provides
  the note CAS.
- **Missing events in reads.** `ticket note show` refuses a missing event with MISSING_EVIDENCE.
  `ticket show` refuses it with JOURNAL_FORKED through its journal audit. Without an audit it
  renders the note as UNAVAILABLE with the code. Neither path reports NONE.
- **Retries.** As with the other CLI mutations, issuedAt defaults to now, so a retry has to pass
  `--issued-at` to replay.

## Remainder

- Claim-time delivery (ON-V0-007), plus any agent-lease amendment it needs.
- Anchored history pages.
- Per-event receipt-audit and redo binding (ON-V0-006). An `evidence/` post still downgrades
  semantic coverage to UNKNOWN.
- Durable qualification and promotion.

## Independent review fixes

The independent review of `cd70ba0a..b31a06df` returned CHANGES_REQUIRED. The fixes:

- **Import forgery (HIGH).** `ticket.Decode` admitted `operatorNote`, and IMPORT_APPLY never
  checked it, so an IMPORTER batch could add, rewrite or drop a note reference to an arbitrary
  digest. `importOperatorNote` now requires an imported record to carry exactly the reference of
  the record it replaces, and none for a new ticket. The refusal cases are in
  `TestCTSV0003_ImportApplyRefusals`.
- **Slot narrowing.** The stage contract sees every mutation verb as MUTATE, so REFINE with an
  extra content-addressed evidence POST was stage-valid. The Model now posts a derived event only
  for operations that `mutation.DeclaresDerivedEvent` names (NOTE_SET, NOTE_CLEAR). Any other
  operation is refused UNSUPPORTED. The spec records that #502 and #504 must bind redo and receipt
  audit per operation before they are declared.
- **Coverage warning.** Each note write keeps receipt-audit semantic coverage at UNKNOWN for the
  whole store until ON-V0-006 binding ships. `ticket note set|clear --help` now says so.
- **Evidence.** Additional tests cover the Apply-level refusal mapping and the `ticket show`
  UNAVAILABLE fallback, and the stage-slot cases now assert which refusal fired.
