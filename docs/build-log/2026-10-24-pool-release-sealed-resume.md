# Pool acquire and release resume from the sealed base

Date: 2026-10-24. Ticket: V1-1702. Requirements: LGL-V0-009, LGL-V0-010, LGL-V0-011
(`docs/specs/corvint-tasks-lease-gate-latency-v0.md`), all proposed. Base 13c54f1e8 (stacked on
#1726). Owner decision: 0549 R11/F3 (cross-process sealed-segment resume for the pool verbs;
writer-route admission, CAL-V0-264, stays declined).

## Root cause

`POOL_ACQUIRE` and `POOL_RELEASE` are not served by the writer route, so each new command prepared
the complete audit (complete plus prefix) of the whole history: 35-60 s at 47k receipts in the
ticket's evidence. Two things kept the existing cross-process resume (decision 0549 S6a) from
helping. First, on a segmented store the writer walk refused any tail longer than `MaxWriterTail`
and any checkpoint at or below the sealed sequence. Second, each complete-route write rebased the
checkpoint's `FullSeq` to its own head, so no later command found a resumable base.

## Change

- `journal.SealedResumeBase` (LGL-V0-009) derives a resume base from the bound writer checkpoint
  when its `FullSeq` equals the manifest's `Through`. It stamps the manifest, segments and evidence
  or pinned blobs, which the precheck re-hashes before the gate and the held check re-hashes under
  the lock, along with the manifest digest. The writer walk, resumed from that base, reads the
  whole active tail and checks receipt S from the last bound segment through
  `snapshot.DecodeRetainedReceipt`.
- `store.sealedResumeVerb` (LGL-V0-010) admits only the two pool verbs, once per command, in the
  first round. Acquire is admitted too, so that it keeps `FullSeq` at S for the release that
  follows.
- Every decline keeps the complete route and its decision.

There is no store-format change, wire change or resident process. The checkpoint, segments and
receipts are read and written in their existing formats, so a reverted binary reads the store
unchanged.

## Measurements (LGL-V0-011)

**Setup.**
- Platform: Windows box, WSL2 Ubuntu, linux/amd64, Ryzen 9 5950X (32 logical CPUs), go1.27.3.
- Store: built on ext4 at `/home/russell/bench/lgl-store/47000`, a real disk (V1-2563), with
  47,000 receipts, head 47616 and sealed through 47104.
- Harness: `TestLGLV0011_PoolReleaseLatency`. It clears the in-process audit cache before each
  call, as a fresh CLI process would.
- "Before" is the same binary with `sealedResumeVerb` forced to false, which is the base
  behaviour.
- Both runs are UNQUALIFIED: the host's 1-minute load was above the threshold of 12 because peer
  lanes were running.
- Raw reports: `/mnt/c/cvq/proof/2026-10-24/tasks-pool-release/lgl-release-{before,after}.json`.

| run | verb | n | p50 | p95 | p99 | audits | resumed | load (1/5/15 min, start / end) |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| before | pool-release | 3 | 20.26 s | 22.44 s | 22.44 s | 3 | 0 | 9.01 18.14 20.73 / 16.40 17.84 20.28 |
| before | pool-acquire | 3 | 18.50 s | 22.51 s | 22.51 s | 3 | 0 | same |
| after | pool-release | 20 | 0.68 s | 1.02 s | 1.03 s | 0 | 20 | 12.24 22.35 22.24 / 17.25 21.74 22.02 |
| after | pool-acquire | 20 | 0.68 s | 0.97 s | 1.01 s | 0 | 20 | same |

The 2 s target for a 47k release is met on this host under load, at p99 1.03 s. With only three
samples, the "before" p95 and p99 are the maximum. In the "after" runs, about 0.4 s of each call
is the held phase, and the rest is the tail walk of about 520 to 620 active receipts.

## Limits

- A store whose last write took the complete route has `FullSeq` past S. That includes a widen,
  a reap, or any write after which the checkpoint was invalidated. Its next pool verb still pays
  the complete audit until a writer-route write or a validated seal rebases the checkpoint to S.
- Widen, reap and mutations keep the complete route.
