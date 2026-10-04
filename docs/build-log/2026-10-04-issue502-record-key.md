# Typed escalation record key: issue 502

Human-owned intent: GitHub issue 502 (native ticket V1-0699). The contract
`docs/specs/corvint-tasks-escalations-v0.md` stays proposed overall with experimental delivery. The
owner accepted its Gate A decisions as written on 2026-10-04 (decision 0428), which lets the writer
and material work integrate.

## Decision

The rollout order is codecs, then writer and material. ESC-V0-002 stores questions as an optional
tool-owned `escalations` reference on the existing ticket record, so the record codec has to carry
the key before any writer can set it. This slice adds only the key and the readers' handling of
it. No writer sets it.

- **Record codec.** `ticket.Record.Escalations` decodes the member with the reference codec and
  encodes the same canonical object. A record without questions keeps its exact bytes. The record
  also refuses a reference that names a control write after its own revision or a question from a
  future acceptance revision, and it enforces the current-acceptance OPEN bound
  (`EscalationMaxCurrentOpen`, now shared with the transaction reducer).
- **One optional-key list.** `wire.TicketRecordOptionalKeys` names `requiresPool`,
  `requiredRoles` and `escalations`. The Tasks codec and Core's read-only planner both admit
  exactly that list. Before this change Core's reader required the exact base key count and
  refused any record that carried `requiresPool` or `requiredRoles` (V1-0754). Core now validates
  each optional member, and a shared key with no Core validator is refused rather than passed
  through. Core checks the reference's shape and its revision ordering against the record. Event
  chains and the open bound stay with the Tasks readers, which hold the evidence blobs.
- **Tool-owned.** `escalations` joins the ADOPT_FILE protected fields. CREATE's payload and export
  items are closed, so neither can carry it. Re-import keeps the stored reference, just as it
  keeps `createdAt`, because the export cannot express it.
- **ADOPT key union.** ADOPT_FILE compared only the canonical record's keys, so an optional key
  present only in the intent file was silently dropped while the file was reported adopted. The
  comparison and coverage now use the union of both sides' keys, and a file-only key that ADOPT
  cannot compose is refused `ADOPT_UNSUPPORTED_FIELD`. This affects `requiresPool` and
  `requiredRoles` today: ADOPT has no composition for either, although REFINE accepts both. That
  gap is filed as V1-0758.

## Evidence

- Tasks tests: `TestIssue502_RecordEscalationsKey` and `TestIssue502_RecordEscalationsRefusals` in
  `internal/tasks/ticket`, four tests in `internal/tasks/mutation` (`TestIssue502_MutationsPreserveEscalations`,
  `TestIssue502_AdoptRefusesEscalationEdits`, `TestIssue502_AdoptRefusesUncomposedAddedKey` and
  `TestIssue502_CreateCannotCarryEscalations`), and `TestIssue502_ReimportKeepsEscalations` and
  `TestIssue502_ExportCannotCarryEscalations` in `internal/tasks/importer`.
- Core tests: `TestIssue502_ReaderAdmitsSharedOptionalKeys` and
  `TestIssue502_ReaderRefusesMalformedOptionalKeys` in `internal/taskman`. They read
  `internal/tasks/ticket/testdata/issue502-optional-keys-record.json`, which the Tasks test proves
  is that codec's own encoding.
- Negative controls, each run against the tests above and then restored:
  - Raising the OPEN bound to 17 makes the 17-open refusal case pass decoding.
  - Reverting ADOPT to canonical-only keys makes the inject and added-key cases adopt.
  - Dropping the importer preservation fails the re-import test.
  - Restoring Core's exact-count check refuses the shared fixture with "closed object member count".

## Found gaps

- V1-0758: ADOPT_FILE cannot compose `requiresPool` or `requiredRoles`. Before this change it
  silently dropped them; it now refuses them.
- V1-0759 (suspected, not reproduced): re-import may drop an IMPORT record's locally set
  `requiresPool`/`requiredRoles` and bump its acceptance revision.

## Rollback

Revert this change. No writer sets the key, so no store can hold a record that the reverted readers
would refuse. Once the writer slice ships, rolling back past this change would make such records
unreadable. That slice's rollback therefore disables writes and keeps the readers.
