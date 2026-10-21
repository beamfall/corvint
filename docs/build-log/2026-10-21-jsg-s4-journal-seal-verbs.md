# JSG S4: journal seal and unseal, compaction and archive on sealed stores

Date: 2026-10-21. Tickets: V1-0887, V1-1512. Spec: `docs/specs/corvint-tasks-journal-segments-v0.md`
(`JSG-V0-012`, `JSG-V0-013`, amended `JSG-V0-008`). Authority: decision 0549 (accepted by the
orchestrator under decision 0548's delegation).

## Change

- `store`: `SealJournal` and `UnsealJournal` (`journalMigrate`) take the writer lock and the writer
  guards, refuse a store with a retired prefix as `UNSUPPORTED`, run the complete settled audit,
  invalidate the writer checkpoint, then seal or unseal one segment at a time until nothing
  changes. They report the head sequence, S before and after, and the segment count.
- `store`: unseal of the last segment is a staged transaction with the S3 marker, prior and next
  heads swapped: it relinks the sealed receipts and requests, publishes the head with S lowered by
  512 (or a `/0` head without segment fields), then removes the segment and its manifest.
  `settleSegmentSeal` settles a stopped unseal exactly as a stopped seal.
- `store`: `dropEmptySegments` removes an empty `segments/` once the head binds no segment, after a
  settle or a migration, so unseal restores the byte-identical unsegmented tree.
- `store`: liveness compaction runs on a segmented store. It reads sealed receipts through the
  logical view and elides only active receipts (`seq > S`); sealed segments are never rewritten.
- `store`: `receipt retire` refuses a segmented store naming `journal unseal`; `receipt unretire`
  refuses one as `UNSUPPORTED`.
- `archive`: export of a segmented store reads the bound manifest and segments by digest, serves
  the sealed receipts and requests at their logical paths, and renders the head as the `/0` head
  without segment fields. The archive is the archive of the same store unsegmented, so archive
  verify and import are unchanged. A `segments/` directory without a segmented head refuses
  `JOURNAL_FORKED`.
- `cli`: read-registry verbs `journal seal` and `journal unseal`, with help; no arguments.

## Forks decided within the design

1. The archive of a sealed store is the logical unsegmented archive, byte for byte, so the
   archive format, verify and import need no segment profile. Sealed bytes are held in memory
   during export, bounded by the archive's existing file and byte limits.
2. Unseal reuses the seal marker with the heads swapped instead of a second marker profile: one
   settle path, one set of stop tests.
3. Compaction keeps sealed receipts whole and elides only active ones, matching the spec's
   non-goal. Compacting sealed history is `journal unseal`, `receipt compact`, `journal seal`.
4. The migration verbs take the writer guards (`JOURNAL_SEAL`, `JOURNAL_UNSEAL`) like every other
   writer, so a held or pending store refuses them the same way.
5. A store with a retired prefix stays unsegmented: both verbs refuse it as `UNSUPPORTED`, and
   sealing from the anchor is deferred. The active set of a retired store is therefore unbounded
   for T1; retire and seal are alternatives until the anchor work lands.
6. An empty `segments/` is removed when the head binds no segment, so a rolled-back first seal or
   a full unseal leaves no stray directory; the archive treats an empty one as unsegmented.
7. `receipt unretire` refuses a segmented store, because its restored prefix would sit below S.
8. `journal seal` uses the writer's 1,024 trigger rather than sealing every whole range, so at
   least 512 receipts stay active and the operator and writer forms agree.
9. Both verbs remove the writer checkpoint, so the next write takes the complete route.

## Evidence

- Tests: the spec's Traceability rows for `JSG-V0-012` and `JSG-V0-013`, including
  `TestJSGV0013_JournalSealAndUnseal` (no-op unseal, seal of 1,700 receipts into two segments, a
  second seal that changes nothing, sealed archive byte-equal to the unsegmented archive, unseal
  to the byte-identical tree), `TestJSGV0013_UnsealStopsSettle` (four stop stages),
  `TestJSGV0013_MigrationRefusesAFault`, `TestJSGV0012_CompactionOnASealedStore`,
  `TestJSGV0012_ArchiveExportRefusesADamagedSegment`, the retire, unretire and retired-store refusals, and the cli envelope test. The superseded
  `TestJSGV0008_SegmentedStoreRefusesCompactAndRetire` is removed. Store compaction, retire and
  JSG runs, archive, and the cli help and read-verb tests pass.
- Mutation checks, run and reverted:
  - swapping the prior and next heads in the unseal marker: `TestJSGV0013_UnsealStopsSettle`
    (settle removes the wrong files and the audit refuses `JOURNAL_FORKED`);
  - letting compaction elide receipts at or below S: `TestJSGV0012_CompactionOnASealedStore`
    (a sealed receipt is relinked and the store refuses `JOURNAL_FORKED`);
  - never removing an empty `segments/`: `TestJSGV0013_JournalSealAndUnseal`;
  - exporting the segmented head without the `/0` rendering: `TestJSGV0013_JournalSealAndUnseal`
    (the sealed archive differs from the unsegmented one);
  - dropping the retired-store refusal of the verbs: `TestJSGV0013_RetiredStoreRefusesJournalMigration`;
  - dropping the unretire refusal: `TestJSGV0013_SegmentedStoreRefusesRetireAndUnretire`, by its
    message only: without the refusal unretire still refuses, because a segmented store never has a
    retired prefix. The refusal stays as the named operator route.
  - Skipping the export's segment digest check survives
    `TestJSGV0012_ArchiveExportRefusesADamagedSegment`: export's audit refuses the same damage first.
    The check stays for a segment changed between that audit and the read.

## Rollback

Run `corvint-tasks journal unseal` on every segmented store, then revert the S4 commit. The verbs,
compaction on segmented stores and the segmented archive export go with it; S3's refusals return.
An archive exported from a sealed store is an unsegmented archive and stays importable by either
binary.
