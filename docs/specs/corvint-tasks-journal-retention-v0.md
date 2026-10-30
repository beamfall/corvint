# Corvint Tasks journal retention V0

Owner: Russell Lewis
Date: 2026-10-17
Intent status: JRT-V0-001..014 accepted under the owner's standing auto-accept (2026-10-11) (V1-1618; GitHub #1055); JRT-V0-015..016 accepted under the owner's standing auto-accept (2026-10-11) (V1-1701; re-homed from the agent leases spec, owner-approved 2026-10-17)
Delivery status: experimental
Authoritative inputs: owner request [issue 1055](https://github.com/beamfall/corvint/issues/1055)
("get 1055 done", 2026-10-17); the receipt chain, the complete audit and the read rule of
[the agent leases contract](corvint-tasks-agent-leases-v0.md) (CAL-V0-060, CAL-V0-061); its writer
checkpoint and fast route (CAL-V0-115..117); its liveness compaction (CAL-V0-300..304); the archive
export and verify procedure (TM-V0-022, `internal/tasks/archive`). This contract cites those
requirements and does not restate them. The design choice and its reasons are recorded in
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-17-V1-1618-journal-retention.md`.

## Agent digest
- Claim: `receipt retire` bounds the live journal by sealing the head receipt over a verified archive of the prefix; `receipt unretire` restores it.
- Status: JRT-V0-001..014 accepted under the owner's standing auto-accept (2026-10-11) (V1-1618; GitHub #1055); JRT-V0-015..016 accepted under the owner's standing auto-accept (2026-10-11) (V1-1701; re-homed from the agent leases spec, owner-approved 2026-10-17) / experimental source implementation with focused store and CLI tests.
- Exists: `receipt retire --archive FILE [--dry-run]`, `receipt unretire --archive FILE`, the `RETIRED_PREFIX` seal, sealed complete audit and binding folds from the anchor, crash recovery through `RETIRE_INCOMPLETE`, doctor journal size, growth and JOURNAL_GROWTH.
- Blocked on: nothing for the slice; retire is one-shot per seal, and export, compaction and import refuse on a sealed store (Non-goals).
- Read next: Requirements; Failure modes; Compatibility and rollback; Traceability.

## Intent and boundary

Issue #1055 reports that the receipt journal only grows. Every complete audit, cold start and
every lease verb that misses the checkpoint fast route walks the whole chain from receipt 1, so
their cost and the store's bytes grow with history that no current decision reads. Liveness
compaction (CAL-V0-300..304) shrinks superseded heartbeat and renew bodies but keeps every receipt
file, chain digest and request afterimage, so it does not bound either.

This contract defines one retention story: an operator-invoked, one-way retire of the settled
prefix of the journal into a verified archive the operator keeps, with an exact rollback from that
archive. The live store keeps the head receipt as an anchor, a seal binding the anchor, the
retired prefix's last digest and the archive, and the anchor's afterimages. Every later audit is
rooted at the anchor. No write path, scheduler, doctor or hook invokes it.

## Requirements

- `JRT-V0-001`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1618; GitHub #1055) Retention is one operation, `corvint-tasks receipt retire`, run only by an operator. No other command, write path, lease verb, scheduled refresh or doctor run retires, archives or drops a receipt. Retire is destructive to the live store and one-way except through `receipt unretire` with the retained archive (JRT-V0-011).
- `JRT-V0-002`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1618; GitHub #1055) `receipt retire --archive FILE [--dry-run]` takes the writer lock, finishes any interrupted run (JRT-V0-009), and requires one complete settled audit (CAL-V0-061) whose head S is at least 2, with no pending receipt and no staging slot. It then checks eligibility (JRT-V0-007) and runs the material binding folds to S; any failure refuses with nothing written. `--dry-run` runs every check except the archive and reports the anchor, the retired count and the live receipt bytes before and after; it needs no `--archive` and writes nothing. Otherwise it exports, verifies (JRT-V0-006), writes the marker, publishes the seal, removes receipts 1..S-1 and both derived checkpoints (CAL-V0-115), removes the marker, and runs the closing complete audit from S; a closing audit that fails restores the prefix from the archive just verified. The item reports `anchorSeq`, `retired`, `receiptBytesBefore`, `receiptBytesAfter`, `sealBytes`, `archiveSha256` and `archiveBytes`.
- `JRT-V0-003`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1618; GitHub #1055) `--archive FILE` is required unless `--dry-run`. It must not exist (retire never replaces a file) and must resolve outside the state directory, the git common directory and the primary worktree, each compared both as named and with symbolic links resolved (the archive's parent directory is resolved too); otherwise retire refuses VALIDATION_FAILED `MALFORMED` before anything is written. The seal `<state dir>/RETIRED_PREFIX` has profile `taskman-retired-prefix/1`, at most 512 MiB, and holds the anchor's writer checkpoint at S, the SHA-256 of receipt S-1 (which receipt S's `prev` binds), the archive's SHA-256 and size, and the canonical afterimage bytes of every anchor path with a digest outside `evidence/`, `pinned/` and `requests/`, followed by a SHA-256 trailer over all of it. It is written to a temporary file, synced, renamed and the directory synced. Any decoding, trailer, seed-digest or missing-seed failure is JOURNAL_FORKED; a store with a seal is audited only through it.
- `JRT-V0-004`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1618; GitHub #1055) The complete audit of a sealed store starts at S. It reads receipt S, requires its digest to equal the anchor's and its `prev` to equal the seal's retired digest, seeds the projection from the seal, and walks S+1 onward under CAL-V0-061 unchanged. A receipt file named 1..S-1 in the live store, a later receipt that posts a request ID the seal records as retired, or a later receipt whose chain does not link to the anchor refuses JOURNAL_FORKED. The store inventory treats the absent prefix as retired by design, not missing. Request files that retired receipts posted stay in `requests/`; the audit matches their names to the anchor's request set, refusing JOURNAL_FORKED when they differ, and does not read or hash them (JRT-V0-013).
- `JRT-V0-005`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1618; GitHub #1055) The audit root of a sealed store is the anchor: the `receipt audit` item reports `anchorSeq` (S, or 0 for an unsealed store) and `retiredThrough` (S-1, or 0), and a sealed store has no checkpoint route, so every writer audit and lease verb (CAL-V0-116) takes the complete sealed walk from S. CAL-V0-116 detection is unchanged: a tampered receipt at or after S, or a tampered seal, refuses the next lease verb JOURNAL_FORKED.
- `JRT-V0-006`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1618; GitHub #1055) Nothing is dropped before the archive verifies. Retire exports the store at S by the `archive export` procedure (TM-V0-022) to a new file, re-reads it through `archive verify`, requires its manifest to name this queue at S and every exported receipt to be byte-identical to the receipt the audit read, and binds its SHA-256 and size into the seal before the marker. The archive file is synced and closed and its parent directory synced before the seal is written.
- `JRT-V0-007`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1618; GitHub #1055) The anchor is eligible only when nothing a later receipt binds by sequence is live at S: no barrier (PAUSED), every attempt in a terminal phase with no held reservation and no pool member or capacity slot occupied (ATTEMPT_LIVE), and no non-terminal ticket carrying external review references (TICKET_STATE) or escalation references (ESCALATION_PENDING). Pending redo or staging refuses under JRT-V0-002 (REDO_PENDING). Other non-terminal tickets and the policy are carried by value in the seal's seeds.
- `JRT-V0-008`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1618; GitHub #1055) The material binding folds (external review, escalation, obligation and gate-reuse audits) of a sealed store start at S, seeded from the seal's afterimages exactly as folding 1..S leaves every input a later receipt can name. The first folded receipt must be S+1 and link to the anchor; otherwise the fold refuses JOURNAL_FORKED. A later receipt that names a review submission, artifact link or claim admission made before S refuses as the unsealed fold refuses an unknown one. A fold asked to end before S refuses UNSUPPORTED.
- `JRT-V0-009`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1618; GitHub #1055) Retire and unretire are crash-safe: the store is the old one or the new one, never a mix. Each writes `<state dir>/RETIRE_INCOMPLETE` before its first destructive step and removes it after its last. While it exists every journal read and every writer except retire and unretire refuses RESTORE_INCOMPLETE. The next retire or unretire finishes it under the lock before doing anything else: a seal present rolls forward (removes receipts 1..S-1 and the derived checkpoints), a seal absent rolls back (removes the temporary seal); the item reports `recovered` as ROLLED_FORWARD or ROLLED_BACK and does no other work.
- `JRT-V0-010`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1618; GitHub #1055) On a sealed store, `archive export`, `receipt compact` and a second `receipt retire` refuse UNSUPPORTED and write nothing; replay of a retired request ID by any writer (lease verb or mutation) refuses UNSUPPORTED and writes nothing. An export cannot stand for a store whose receipt 1 is gone; the retained archive is the export of the prefix.
- `JRT-V0-011`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1618; GitHub #1055) `receipt unretire --archive FILE` is the rollback. Under the writer lock it requires the archive's SHA-256 and size to equal the seal's, `archive verify` to pass and its manifest to match the anchor, writes the marker, links each retired receipt back (a different file already at a retired name refuses JOURNAL_FORKED), requires the restored count and receipt S-1's digest to match the seal, removes the seal and the derived checkpoints and the marker, and requires a complete audit from receipt 1. Receipts written after S are kept. An archive that differs refuses JOURNAL_FORKED and leaves the sealed store unchanged. Each restored receipt is written under the canonical name regenerated from its sequence, never the archive member's own name bytes (V1-3179).
- `JRT-V0-012`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1618; GitHub #1055) `doctor` adds `journal` to its `taskman-doctor/0` item (an additive key beside TQD-V0-001's): `receipts` and `bytes` of the live `receipts/` directory (names and sizes only, at most 4,194,304 entries; NOT_OBSERVED with a scan warning when unreadable), `retiredThrough` (S-1, or 0), `receiptsPerDay` and `bytesPerDay` over the TQD-V0-004 window scan (NOT_OBSERVED below two receipts or a zero span), `thresholdReceipts` 16384 and `thresholdBytes` 268435456. Past either threshold it emits one JOURNAL_GROWTH finding, who `journal`, whose remedy names `receipt retire --dry-run` and the archive. The window scan ends without truncation at the retired prefix. The doctor stays a pure read.
- `JRT-V0-013`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1618; GitHub #1055) After retire the live journal holds the anchor and later receipts only, so the complete audit and cold start read no retired receipt and no retired request body, and their cost no longer grows with retired history. The before/after audit time and receipt bytes are measured and recorded in the build log; unmeasured scales are NOT_RUN.
- `JRT-V0-014`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1618; GitHub #1055) The `receipt compact` help states that it reclaims superseded liveness bodies only, that every receipt, chain digest and request afterimage stays in the live journal, and points to `receipt retire` (JRT-V0) to bound the journal.

### V1-1701 complete journal walk decode reuse and bounded read-ahead

Re-homed (owner-approved, 2026-10-17): these requirements were first numbered CAL-V0-380..381 in the
agent leases spec; they moved here with the same text so that spec stays within the 256-obligation
bound.

A complete audit at 47,000 receipts took about 21 s. The CPU profile blamed `openat`, but the wall
time fell once decode work was removed, so the cost was mostly CPU: each receipt's ticket and request
afterimages were parsed two or three times, all on the walking goroutine.

- `JRT-V0-015`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1701) The complete
  walk (CAL-V0-061) MUST parse each inline ticket or request afterimage once. The record validation that already
  parses a post hands its decoded ticket or request to the semantic step and to the operator-note
  binding. It keeps the existing size bounds and refusal text, so no input that was refused before is
  accepted, and no accepted input is refused.
- `JRT-V0-016`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1701) The walk MAY
  decode and validate receipts ahead of the step, in parallel, within one abort window (the 512-receipt
  head-moved check) and within a 512 KiB budget of held receipt bytes. Receipt bytes are read in
  sequence on the walking goroutine, each read bounded by the budget left before it buffers, so a
  window never holds more than 512 KiB of receipt bytes. The decoded receipts and inline afterimages
  a window retains are in addition to the budget, in proportion to those bytes. A receipt that does
  not fit ends the window; one larger than the whole budget is read by step under its own bound.
  A read-ahead records nothing until step reaches that receipt: its close error, physical-read
  witness and read hook are settled then, in sequence, and a read-ahead that fails for any reason is
  discarded with its close error and read again in sequence. Every ordering, chain, coverage and
  authority check still runs in step, in sequence order. Elided receipts and receipts with any
  blob-backed post (checked before any of its posts is validated, so JRT-V0-015 still holds) are
  left to step's sequential route, as are receipts whose bytes fail to decode. A walk with read-ahead
  MUST return the same result or the same refusal as a walk without it.

Non-goals: concurrent file reads, a cache across commands, and changes to the capture and listing
cost (two captures `lstat` every store file). Failure modes: a read error inside a window is
discarded and the receipt is read again in sequence, so the refusal names the same receipt and code;
a receipt read ahead but never reached (the walk refused earlier) contributes no close error or
witness, so it cannot replace the refusal; held receipt bytes stay within the budget and decoded
state is proportional to them (TMV0002 AS10 streaming bound still passes). A window that ends on a
receipt that does not fit opens that receipt once more.

Acceptance evidence: `TestV11701_PrefetchedWalkMatchesSequentialWalk` (`internal/tasks/journal`)
audits a 601-receipt history with and without read-ahead. It asserts equal verdicts, selected records,
request digests and canonical latest, one read per receipt in each mode, and the number of receipts
decoded and post sets validated ahead. `TestV11701_PrefetchedWalkRefusesLikeSequentialWalk` asserts
the same refusal for an undecodable receipt and a forked `prev` inside a window.
`TestV11701_ReadAheadCloseFailureSettlesInSequence` injects a close failure on a later receipt of the
same window and asserts the earlier forked `prev` stays the refusal, and that a reached receipt's
close failure is the sequential refusal. `TestV11701_ReadAheadHoldsAtMostTheBudget` narrows the
budget and asserts no window holds more than it, one read per receipt, and that only the receipt
larger than the whole budget is left to step. `TestV11701_MixedInlineAndBlobPostsValidateOnce`
asserts equal `validateRecord` call counts with and without read-ahead for receipts mixing an inline
and a blob-backed post. The opt-in
`TestV11701_WalkCost` (`internal/tasks/store`) harness measured a 47,000-receipt synthetic store at
a median of 21.2 s before and 5.8 s after (five runs each); see
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-20-v1-1701-walk-decode-prefetch.md`.

Rollback: revert the commit. Setting `walkPrefetch` to false in a test restores the sequential walk
with decode reuse.

## Non-goals

- No automatic, scheduled or threshold-triggered retire; JOURNAL_GROWTH is advisory.
- No repeated retire on a sealed store, no export, compaction or import of a sealed store, and no CLI `archive restore` (TCP-02's). Unretire first.
- No change to the receipt chain, the CAL-V0-116 fast route on unsealed stores, the elided receipt format or any retained receipt.
- No retention of `evidence/`, `pinned/` or `requests/` blobs: they stay in the live store.
- No new ticket identity: the anchor keeps every ticket's journal identity (option (b), a fresh genesis, was rejected; see the build log).

## Failure modes

- A seal forged consistently together with its anchor and every seed is detectable only against the retained archive (unretire checks the archive digest and every restored receipt).
- Losing the archive makes the retire irreversible; the seal still audits the retained store.
- A handoff policy, a review submission, an artifact link or a claim admission made before S is not seeded; a later receipt that names one refuses.
- Readers of an old receipt by sequence (`receipt show`, `receipt replay`) see it as absent.
- `ReviewFold` cannot advance across the seal incrementally and refolds from S on each tick; the cost is bounded by the post-anchor journal.
- The seal is not charged to store capacity.
- An interrupted run blocks every read and write until an operator reruns retire or unretire.
- The pending-redo refusal path (REDO_PENDING) has no direct test.

## Acceptance evidence

Focused store tests in `internal/tasks/store/retire_test.go` build eligible stores through real lease
verbs, retire and unretire them, tamper with the archive, the seal, the anchor and later receipts,
interrupt every durable step of retire and unretire through a test seam, count the sealed audit's
reads, run escalation and gate-reuse transitions and retired-request replays across the seal, and
measure the audit and bytes. CLI tests in `internal/tasks/cli/receipt_retire_test.go` cover the help,
the argv refusals, a retire/unretire round trip, the audit's `anchorSeq` and `retiredThrough`, the
doctor journal fields and JOURNAL_GROWTH, and external review and obligation ledger transitions
after the anchor. The CAL-V0-131 tables classify `taskman-retired-prefix/1` and refuse a newer one.

## Compatibility and rollback

A store without `RETIRED_PREFIX` is unchanged and audits from receipt 1. A binary without this
contract refuses a sealed store (receipt 1 is absent). Rollback of a store: `receipt unretire
--archive FILE` with the retained archive (JRT-V0-011). Rollback of the feature: unretire every
sealed store, then remove the two verbs and the doctor `journal` key.

## Traceability

| Requirement | Evidence |
| --- | --- |
| JRT-V0-001 | `TestJRTV0012_RetireCLIAndDoctorJournal` |
| JRT-V0-002 | `TestJRTV0002_RetireShrinksJournalAndRollsBack`, `TestJRTV0012_RetireCLIAndDoctorJournal` |
| JRT-V0-003 | `TestJRTV0007_RetireRefusesIneligibleAnchor`, `TestJRTV0006_TamperRefuses`, `TestJRTV0006_ArchiveTargetResolvesLinks`, `TestCALV0131_EveryLiveFormatRefusesANewerVersion`, `TestCALV0131_LiveFormatsCoverEveryDecodedProfile` |
| JRT-V0-004 | `TestJRTV0002_RetireShrinksJournalAndRollsBack`, `TestJRTV0006_TamperRefuses` |
| JRT-V0-005 | `TestJRTV0002_RetireShrinksJournalAndRollsBack`, `TestJRTV0006_TamperRefuses`, `TestJRTV0012_RetireCLIAndDoctorJournal` |
| JRT-V0-006 | `TestJRTV0002_RetireShrinksJournalAndRollsBack`, `TestJRTV0006_TamperRefuses`, `TestJRTV0007_RetireRefusesIneligibleAnchor`, `TestJRTV0006_ArchiveTargetResolvesLinks`, `TestJRTV0006_ArchiveDirectorySyncPrecedesSeal` |
| JRT-V0-007 | `TestJRTV0007_RetireRefusesIneligibleAnchor`, `TestJRTV0008_EscalationAndGateReuseAcrossRetirement`, `TestJRTV0008_ReviewAcrossRetirement` |
| JRT-V0-008 | `TestJRTV0006_TamperRefuses`, `TestJRTV0002_RetireShrinksJournalAndRollsBack`, `TestJRTV0008_EscalationAndGateReuseAcrossRetirement`, `TestJRTV0008_ReviewAcrossRetirement`, `TestJRTV0008_ObligationAcrossRetirement` |
| JRT-V0-009 | `TestJRTV0009_InterruptedRetireIsOldOrNew`, `TestJRTV0011_InterruptedUnretireIsOldOrNew` |
| JRT-V0-010 | `TestJRTV0002_RetireShrinksJournalAndRollsBack`, `TestJRTV0008_EscalationAndGateReuseAcrossRetirement` |
| JRT-V0-011 | `TestJRTV0002_RetireShrinksJournalAndRollsBack`, `TestJRTV0006_TamperRefuses`, `TestJRTV0011_InterruptedUnretireIsOldOrNew`, `TestJRTV0011_UnretireAfterCompaction`, `TestV13179RetiredReceiptReturnsCanonicalName` |
| JRT-V0-012 | `TestJRTV0012_RetireCLIAndDoctorJournal` |
| JRT-V0-013 | `TestJRTV0013_RetireBoundsAuditCost` |
| JRT-V0-014 | `TestJRTV0012_RetireCLIAndDoctorJournal` |
| JRT-V0-015 | `TestV11701_PrefetchedWalkMatchesSequentialWalk`, `TestV11701_MixedInlineAndBlobPostsValidateOnce`, `TestTMV0002_AS10_ReceiptBytesAreStreamed` (`internal/tasks/journal`); `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-20-v1-1701-walk-decode-prefetch.md` |
| JRT-V0-016 | `TestV11701_PrefetchedWalkMatchesSequentialWalk`, `TestV11701_PrefetchedWalkRefusesLikeSequentialWalk`, `TestV11701_ReadAheadCloseFailureSettlesInSequence`, `TestV11701_ReadAheadHoldsAtMostTheBudget`, `TestTMV0002_AS10_ReceiptBytesAreStreamed` (`internal/tasks/journal`); `TestV11701_WalkCost` (`internal/tasks/store`, opt-in) |
