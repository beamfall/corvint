# JSG S5: seals validate and advance complete coverage; the segmented writer bound

Date: 2026-10-21. Tickets: V1-0887, V1-1512 (issue 967 proposals 2 remainder and 3). Spec:
`docs/specs/corvint-tasks-journal-segments-v0.md` (`JSG-V0-009`, `JSG-V0-010` amended; `JSG-V0-004`
segment digests). Authority: decision 0549, accepted by the orchestrator under decision 0548's
delegation; the amendments are accepted under the owner's standing auto-accept (2026-10-11).

## Problem

After S3 a seal moved receipts into segments but complete coverage (`FullSeq`) still advanced only
through the complete route or the scheduled refresh, and the writer bound was still
`WriterFullBound` (4,096). On a segmented store the refresh walk was the remaining
receipt-proportional term on the writer route, and the 4,096 cliff (issue 967 proposal 2) still sent
a writer through the complete audit when no refresh succeeded.

## Change

- `journal`: the sealed base is `taskman-writer-base/1`. It appends the checkpoint header (no
  entries), the ten aggregates at its sequence and the SHA-256 of every sealed segment. The decoder
  checks them against its own table (header at the base's sequence, one file per receipt and
  request, intent aggregate equal to the live records, segments only over covered receipts), and
  `DecodeWriterCheckpoint` checks the base against the delta that binds it (same queue, worktree and
  init, aggregates that never shrink, and equality at the base's own sequence). A `/0` base is
  refused as another version; the next write takes the complete route once.
- `journal`: `SealValidates` and `SealCheckpoint`. A writer seal whose checkpoint has a base at
  `FullSeq`, with `FullSeq` in first−1..last−1 and the base's segment digests leading the manifest,
  runs the complete per-receipt validators over `FullSeq`+1..last from the exact bytes it is about to
  seal (`segmentSource`), resumed from the base's header and aggregates. It requires the chain to
  reach the range's last digest, derives a base-free checkpoint at last, and `rebase`s the
  retained checkpoint on it: every entry, request and note at or before last must equal the
  validated one, and the result is encoded and decoded as a self-check.
- `store`: `sealSegment` validates before anything lands; a refusal declines the seal.
  `sealAfterCommit` removes the writer checkpoint after any failed seal, so the next writer's
  complete audit classifies the fault. Once the seal is complete, `publishSealCheckpoint` replaces
  the invalidation token it read the checkpoint under and retains the rebased checkpoint, so the
  sealing writer's own advance (derived from the pre-seal checkpoint) cannot overwrite the higher
  `FullSeq`.
- `journal`/`store`: on a segmented store the writer bound is `WriterSegmentedBound` (1,024) and
  `advanceWriterCheckpoint` never schedules the refresh; the base is kept until the next seal
  rebuilds it. Unsegmented stores keep `WriterFullBound`, `WriterBaseTail` and the refresh.

## Forks decided within the design

1. `WriterFullBound` stays for unsegmented stores rather than being removed outright: they have no
   seal to advance coverage, and the format gate still keeps INIT unsegmented. `JSG-V0-010` is
   amended to say so.
2. A seal that cannot validate (no base at `FullSeq`, or `FullSeq` already past the range) still
   seals and leaves `FullSeq` alone. The 1,024 bound then routes one writer through the complete
   audit, whose checkpoint the following seal validates from. This keeps sealing independent of
   checkpoint health.
3. Validation reads the segment's own staged bytes, not the files in `receipts/`, so what is
   validated is exactly what is sealed.
4. The forged-aggregate case in `TestJSGV0003_CompleteRouteVerifiesRetainedCheckpoint` now forges
   past the base: at the base's own sequence the codec refuses the forgery itself.

## Evidence

- Tests (focused, through the host semaphore): `TestJSGV0009_SealValidatesAndAdvancesFullSeq`,
  `TestJSGV0009_SealDeclinesACarriedValueThatDiffers`, `TestJSGV0009_SealWithoutCheckpointSealsPlainly`,
  `TestJSGV0010_SegmentedWriterBound`, every `TestJSGV00*` store test, the `TestONV0014` note-resume cases, the base-meta rows of
  `TestCALV0115_WriterCheckpointCodecIsClosed`, the journal, transaction and archive packages, and
  the store writer/checkpoint/resume subset (`Writer|Checkpoint|Resume|PSRV005|PSRV011|PSRV015|
  CALV011|CALV0189|CALV0190|CALV0265|ONV001`).
- `TestONV0014_UnboundNotePreStateDeclinesTheWriter/writer-note-digest` now forges the note digest
  where it ends in the `/1` layout (before the trailing header), not the base's last byte, which is
  now trailer data the decoder refuses on its own.
- Mutation checks, each run and reverted:
  - `SealValidates` always false: `TestJSGV0009_SealValidatesAndAdvancesFullSeq` fails (the seal
    retained no advanced `FullSeq`).
  - `sealAfterCommit` without `invalidateWriterCheckpoint`:
    `TestJSGV0009_SealDeclinesACarriedValueThatDiffers` fails (a declined seal left the checkpoint).
  - the segmented walk bound set to `WriterFullBound`: `TestJSGV0010_SegmentedWriterBound` fails
    (head − `FullSeq` = 1,024 still took the fast route).
  - Before the test fix, `bindsDelta` made `TestJSGV0003_CompleteRouteVerifiesRetainedCheckpoint`
    fail at the base's own sequence, which is why that case now forges past the base.

## NOT_RUN

- The internal Linux qualification of the targets (no Linux host).
- `make gate` (owner scope rule).

## Rollback

Revert the S5 commit. The `/1` base is derived state: a reverted binary refuses it as another
version and the next write takes the complete route, which retains a `/0` base. No store migration
is needed.
