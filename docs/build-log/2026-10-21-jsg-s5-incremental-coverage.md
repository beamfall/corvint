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

## Measurements before and after (V1-0887, V1-1512)

Darwin, 12 CPUs, Go 1.27.3, one host shared with other lanes. Load is the 1-minute average; no
cell qualifies under decision 0549's load bound (load at or below the core count), so these are
evidence of shape, not qualification.

T1, `TestCALV0118_WriterHoldProfile` (B1), sealed history, 5 reps, median wall ms, heartbeat / renew:

| tickets x receipts | 0549 baseline (unsegmented) | S3 (load 5-13) | S5 (load 20-35) |
| --- | --- | --- | --- |
| 880 x 3,500 | - | 47.2 / 55.1 | 67.8 / 69.1 |
| 880 x 13,500 | 145 / - | 46.9 / 53.9 | 65.3 / 79.1 |
| 880 x 40,000 | - | 58.9 / 61.8 | 95.6 / 89.4 |
| 3,000 x 13,500 | 470 / - | 48.9 / 58.0 | not rerun |
| 9,500 x 13,500 | 1,042 / - | 48.9 / 64.7 | not rerun |

S5 does not add to the liveness write's per-call work (the seal runs once per 512 receipts, after a
durable commit), and the S5 row tracks load: at 880 x 40,000 a heartbeat spent 35 ms observing,
19 ms in fsync and 15 ms writing the journal, against 16, 13 and 22 ms in S3 at load 6.5. Flatness
from 3,500 to 40,000 receipts is 1.41x under the rising load of this run (1.33x in S3). The 50 ms
median holds in no cell yet: the floor is the commit (about 30 ms of journal write and fsync) plus
the observe step (about 16 ms at low load). Residual, with that cause.

T2, `TestCALV0263_LeaseGateLoad` (B2), 40,000 receipts, 15 writers, 10 s think time, 60 s window.
The harness now seals the store before measuring, as B1 does since S3 (a store that long is
segmented in use). Load 127 before and 57 after the window (setup 276 s).

| verb | 2026-10-20, unsegmented (load 25-28): p50 / p99 ms | S5, sealed (load 57-127): p50 / p99 ms | gate hold p99, S5 |
| --- | --- | --- | --- |
| heartbeat | 113 / 1,138 | 139 / 304 | 197 |
| renew | 105 / 1,131 | 148 / 406 | 286 |
| claim | 1,601 / 2,700 | 1,699 / 2,780 | 181 |
| release | 152 / 309 | 134 / 298 | 163 |
| widen | 24,564 / 39,468 (0 failed) | 12,941 / 28,911 (6 of 7 SNAPSHOT_MOVED) | 313 |
| reap | 52,398 (0 failed) | 18,663 / 26,074 (3 of 4 SNAPSHOT_MOVED) | - |

Liveness p99 falls from about 1.13 s to 0.30 to 0.41 s under a much heavier host load, because a
widen or reap no longer holds the gate for about a second behind the complete audit. It is not yet
at T2's 250 ms. Widen and reap on a sealed store under sustained appends fail `SNAPSHOT_MOVED`:
their prefix fallback (`walkPrefix`) and the same-process resume base (`ResumeBase`) both refuse a
segmented journal and leave that to slice S6 (`JSG-V0-011`, not started in this lane). An
unsegmented store keeps the 2026-10-20 behaviour (no failures), and a write never converts a store,
so the regression is confined to stores an operator sealed with `journal seal`. Until S6 lands,
`journal seal` should not be run on a store with long-running widen or reap traffic. T3 (widen and
reap at 40,000 within 2 s) also needs S6.


- The internal Linux qualification of the targets (no Linux host).
- `make gate` (owner scope rule).
- Slice S6 (`JSG-V0-011`) and the B3 harness: not started; T2's widen/reap criterion and T3 wait on them.
- B1 at 3,000 and 9,500 tickets after S5: not rerun (S5 does not touch the per-call path).

## Rollback

Revert the S5 commit. The `/1` base is derived state: a reverted binary refuses it as another
version and the next write takes the complete route, which retains a `/0` base. No store migration
is needed.
