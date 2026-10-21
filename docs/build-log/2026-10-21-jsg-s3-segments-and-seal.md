# JSG S3: segments, the writer seal and the retired listing reuse

Date: 2026-10-21. Tickets: V1-0887, V1-1512. Spec: `docs/specs/corvint-tasks-journal-segments-v0.md`
(`JSG-V0-005` to `JSG-V0-008`, and part of `JSG-V0-014`). Authority: decision 0549 (accepted by the
orchestrator under decision 0548's delegation).

## Change

- `snapshot`: `taskman-segment/0` (512 receipts plus the request afterimages they posted, sorted),
  `taskman-segment-manifest/0`, `taskman-seal-marker/0`, and `taskman-journal-head/1`, which binds
  `segmentsThrough` S and the manifest digest. All four join the CLI's live formats and the format
  gate; a `/0`-only reader refuses a `/1` head as `UNSUPPORTED_VERSION`.
- `journal`: a segmented head's walk reads receipts 1..S and their requests from the bound segments
  by their logical paths (`segmentView`), with the same validators as active files. `Logical` gives
  single-file readers (doctor, store scans) the same view. `segments/` must hold exactly the bound
  manifest and segments, and no sealed receipt may stay linked (`JSG-V0-007`).
- `journal`: the `CAL-V0-224` liveness probe and quiescent listing reuse are removed. Both captures
  list `receipts/` exactly, bounded by the active set on a sealed store.
- `store`: `sealAfterCommit` seals the next 512 receipts once a committed write on a segmented store
  leaves 1,024 receipts after S. `sealSegmentsLocked` (the base of S4's `journal seal`) also
  converts an unsegmented store. `unsealSegments` restores a byte-identical unsegmented tree.
- `CAL-V0-224` and `CAL-V0-116` text amended for the retirement; no new CAL IDs.

## Forks decided within the design

1. Trigger at 1,024 receipts past S, sealing 512, so the head receipt is never sealed and at least
   512 receipts stay active.
2. Writers seal only segmented stores. Converting a `/0` store is the operator's `journal seal`
   (S4) or INIT after the format gate. A write therefore never makes a store unreadable to a prior
   binary (`R13`); without this, any write on a store of 1,024 receipts converted it.
3. Order: marker (synced) → segment and manifest → head publish (the commit point) → unlink the
   sealed receipts → remove the marker. A reader that sees the marker reports `REDO_PENDING`.
4. Settle by the next writer: under the prior head, remove the new files; under the next head,
   verify the segment and every still-linked file, then unlink; under any other head,
   `JOURNAL_FORKED` and nothing is removed.
5. A failed seal is settled by the same writer at once and never declines the commit it followed.
6. Import does not seal; a store with a retired prefix is not sealed until S4 seals from the anchor.
7. Request afterimages are write-once: a sealed request whose bytes differ from the digest its
   receipt posted, or a removed afterimage, makes the seal decline as `UNSUPPORTED`.
8. `ResumeBase` and the prefix walk decline on a segmented head until S6 re-hashes segments
   (`JSG-V0-011`); compact and retire refuse segmented stores as `UNSUPPORTED` until S4.
9. The S2 writer base still rewrites at 512 receipts; binding segment digests in the base moves to
   S5 (`JSG-V0-004` traceability says so).
10. The store's retained-elsewhere check reads the index through the logical view on a segmented
    head; the inventory adds sealed logical entries and skips the head re-read without `segments/`.
11. Unseal invalidates the writer checkpoint, so the next write takes the complete route.
12. A head's S must be whole 512-receipt ranges, greater than 0 and less than its last sequence.
13. The T1 harness seals its history with `sealSegmentsLocked` before measuring.

## Evidence

- Tests: listed in the spec's Traceability rows for `JSG-V0-005` to `JSG-V0-008`, including
  `TestJSGV0005_SealUnsealRollback` (seal, byte-identical unseal, reseal), the format gate rows in
  `TestCALV0131_EveryLiveFormatRefusesANewerVersion`, and the `CAL-V0-224` tests amended for exact
  listings. Store regression chunks, snapshot, journal, transaction and focused cli runs pass.
- Mutation checks, run and reverted (each killed):
  - skipping the chain check in `collectSegment`, or the request digest check:
    `TestJSGV0008_SealDeclinesUnprovenInput`;
  - skipping the settle's linked-file compare: `TestJSGV0008_SealSettleRefusesForeignState`;
  - not writing the marker: `TestJSGV0008_SealStopsSettle`;
  - scans skipping sealed entries: `TestJSGV0005_SealUnsealRollback`;
  - dropping the `segments/` count check: `TestJSGV0007_SegmentLayoutRefusals` (unbound segment);
  - dropping the writer's convert gate: `TestJSGV0008_WriterSealsOnlySegmentedStores`.
  - Dropping the "sealed receipt still linked" check survives: the walk's receipt contiguity check
    refuses the same tree. It stays as defence in depth.
- T1 spot measurement (`TestCALV0118_WriterHoldProfile`, sealed history, 5 reps, 12 cores). A
  receipts count at or below the ticket count is unbuildable, so the smallest cells are 3,500 and
  10,000. Medians in ms, heartbeat / renew, with the 1-minute load at the start:

  | tickets \ receipts | smallest (3,000; 3,500; 10,000) | 13,500 | 40,000 |
  | --- | --- | --- | --- |
  | 880 | 47.2 / 55.1 (7.6) | 46.9 / 53.9 (6.5) | 58.9 / 61.8 (5.6) |
  | 3,000 | 55.8 / 63.5 (5.8) | 48.9 / 58.0 (4.9) | 62.6 / 64.6 (12.7) |
  | 9,500 | 57.1 / 66.8 (6.1) | 48.9 / 64.7 (11.6) | 52.1 / 63.7 (6.5) |

  Flatness holds: the largest cell is 1.33x (heartbeat) and 1.24x (renew) the smallest. The 50 ms
  median does not yet hold. At 9,500 by 40,000 a heartbeat spends about 16 ms observing, 13 ms in
  fsync and 22 ms in commit; S2 measured 114.2 ms there under load 102. Not qualifying: one host,
  load near the core count in two cells.

## Rollback

Run `unsealSegments` (S4's `journal unseal`) on every segmented store, which restores the
unsegmented tree byte for byte, then revert the S3 commit. The probe, the listing reuse and their
tests return with the revert. A prior binary refuses a `/1` head as `UNSUPPORTED_VERSION` rather
than misreading it.
