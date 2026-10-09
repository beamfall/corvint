# corvint-tasks scale performance (V1-1053) and reads under a writer pool (V1-1060)

Date: 2026-10-09

V1-1053 asked for corvint-tasks to be as fast as possible at large ticket and receipt counts, without
weakening receipt verification, authority checks, canonical-form checks or read non-mutation
(AGENTS.md invariant 4), and with byte-identical output. V1-1060 (beamfall/corvint#707, P1) reported
plain reads taking 1–2 min at about 22.5k receipts while a writer pool was active. This entry
records both. No requirement ID is added or changed, and no persisted format changes.

## Changes

| commit (subject) | effect |
|---|---|
| `d9635b0a` `tools/tasks-scale: synthetic corvint-tasks scale store generator and command matrix` | `go run ./tools/tasks-scale gen` builds a store through the real CLI; `matrix` reports wall, user/sys CPU, mallocs, allocated bytes, peak descriptors and output bytes per command, with optional pprof. |
| `dd774af7` `tasks/authority: size mount-name decoding to the NUL prefix` | `cString` allocated a full `[1024]` array per mount entry; fails on base (`cstring_test.go`). |
| `92b31a1a` `tasks: read files into their stat-sized buffer` | `intent.readAll` and the journal's native read start from the stat size, which removes `io.ReadAll` regrowth (35,888 bytes allocated per 3,000-byte file on base; `read_all_test.go` fails on base). |
| `4c2cbcf0` `tasks: fold receipt bindings from the audited receipts` | `receipt audit` and external-review material bindings reuse the receipts the journal audit already decoded and verified, through `Reader.ReceiptFold`. They no longer re-read and re-parse every receipt. A per-ticket `TicketPosts` memo replaces a quadratic per-step scan. The fold only runs on the complete walk, and its verdict comes from the same verified records. |
| `393344c7` `tasks: keep plain reads on the checkpoint path under an active writer (V1-1060)` | See below. |
| `20ac1f16` `tools/tasks-scale: contend mode for reads under a writer pool` | `contend` runs W `churn` writer processes and R concurrent readers, and reports per-read wall/CPU/audit mode/codes. |

## V1-1060: what drops a read into full replay when the head moves

Line numbers below are for base `02e84575`.

- Plain reads run inside `withStore` (`internal/tasks/cli/cli.go:480`), which calls
  `snapshot.Reader.Read` (`internal/tasks/snapshot/probe.go:227`). That function probes, runs the
  body (`:276`), re-probes, and keeps the result only when `Same` holds (`:287`).
- The body calls `auditState` (`internal/tasks/cli/lease.go:469`). It builds a checkpoint-resumed
  `journal.Reader` (`:479`) and refuses any proof whose head differs from the outer snapshot
  (`:484`).
- In `journal.Reader.audit` (`internal/tasks/journal/audit.go:262-275`), the checkpoint loop retried
  only `SNAPSHOT_MOVED`, and broke out on any other error into the complete walk loop. The complete
  walk loop makes up to four attempts, and each one reads every receipt.
- A writer between receipt link-in and head rename makes `walkTail` return the checkpoint refusals
  "journal is not plainly settled" (`internal/tasks/journal/records.go:707`, staging present) or
  "receipt beyond head" (`:747`). Both are `UNSUPPORTED`, not `SNAPSHOT_MOVED`, so an ordinary
  in-flight write sent the read straight to the complete walk.
- Under churn, each complete walk then saw the head move, and repeated up to four times. The outer
  reader then saw `Same` fail and ran the whole body again.
- The result was 5–25 s reads at about 4k receipts (below), and minutes at 22.5k. These are N
  independent readers each doing the same full work. Base also sometimes ended in `REDO_PENDING`.

**Fix (`internal/tasks/journal/audit.go`, `records.go`, `cli/lease.go`)**

1. The two in-flight refusals are wrapped as `inFlight` (`records.go:718`). The checkpoint loop
   retries them within the same four-attempt bound as a moved head, and only then falls back to the
   complete audit.
2. `Reader.ExpectHeadSha256` (`audit.go:130`) is set by `auditState` to the outer snapshot's head. An
   attempt whose first capture shows another head returns `SNAPSHOT_MOVED` at once (`audit.go:346`),
   without walking or falling back. The outer reader re-probes, as it would have after discarding
   that verdict at `lease.go:484`.

Nothing a read writes changes: reads still write nothing. Stale, forged or undecodable checkpoints
still fall back to the complete audit, and that verdict is the one reported. All existing CAL-V0-059
and CAL-V0-061 tests pass unchanged.

**Interpretation for owner confirmation (not a spec edit).** CAL-V0-061 says a head that moves
between captures "is retried at most four times first". This change treats a writer in flight as
the head moving, so the retry bound covers it. The early `SNAPSHOT_MOVED` for a head that is no
longer the caller's skips a fallback whose verdict `auditState` would always discard. The only
non-equivalent case is a head that returns to byte-identical content, which requires a restore. If
the owner reads CAL-V0-061 as requiring the complete audit even then, the second point is a contract
fork and should be reverted.

**Does the CTS-V0-006 2 s re-read budget bound this path? No.**

- The patience deadline is fixed once (`probe.go:246`), and `wait()` is checked only between bodies
  (`:291`). Nothing interrupts a body that is running.
- Once patience is spent, up to `Retries` (3) further unpaused bodies run (`:295`).
- So a read can take about 4 × the body's duration plus 2 s. The budget bounds waiting, not work.
  With the fix, each body is bounded by the checkpoint-resumed tail, about 0.2–0.6 s at these
  sizes.

**Tests.** `internal/tasks/journal/head_passed_test.go`:

- `TestV11060_InFlightTailIsRetriedBeforeCompleteAudit` gives the first read of the next receipt a
  pending link. It expects mode `CHECKPOINT`, zero complete walks, and the same seq, digest and head
  as the complete audit.
  - With the retry disabled, it fails: `<nil> mode FULL, 1 complete walks, 1 reads of 000000000006.json`.
- `TestV11060_HeadPastCallerSnapshotStopsTheAudit` covers three cases:
  - Checkpoint, bound: 0 walks.
  - Complete, bound: 1 walk.
  - Complete, unbound: 2 walks, ending `FULL` at the new head. This case keeps the old behavior
    without the binding.
  - Each bound case also checks that a rebound audit succeeds.
- The second test cannot compile on base, because the field does not exist there.

**Contention measurements (macOS arm64, host shared with other lanes, GOMAXPROCS=3)**

| store / load (40 s) | build | reads | p50 ms | p95 ms | max ms | failures | audit modes |
|---|---|---|---|---|---|---|---|
| st-1k (2.4k receipts), 3 churn writers + 3 `queue status` readers | base | 21 | 461 | 17,200 | 25,800 | 1 `REDO_PENDING` | mixed, long reads FULL |
| same | new | 199 | 402 | 1,680 | 3,070 | 0 | all `CHECKPOINT_PLUS_TAIL` |
| st-10k (4.2k receipts), generator writing + 3 readers | base | 33 | 571 | 17,200 | 30,000 | 2 `REDO_PENDING` | mixed |
| same | new | 149 | 594 | 1,490 | 2,260 | 0 | all `CHECKPOINT_PLUS_TAIL` |

- On st-1k, 170 of 199 new reads finished under 1 s. CPU per read was p50 211 ms and p95 1.27 s.
  Writers completed 52 creates (new) versus 43 (base).
- With one reader and three writers, new reads took 185–551 ms. The slower ones ran two bodies.
- A profile of a retried read shows about 42% in the outer probe's `intent.TreeDigest`, which opens
  every ticket file, and about 20% in the journal audit.
- So under three readers plus three writers, the warm <1 s target is met at p50 and missed at p95.
  The remainder is outer-probe work repeated per body, not replay.

**Not done (forks).**

- **Sharing work across concurrent readers.** One audit result shared across N reader processes
  would need a read to write shared state, which invariant 4 forbids, or a shared lock. That is a
  contract fork.
- **Real 22.5k-receipt store with an active pool.** This is `NOT_RUN`. The synthetic stores reached
  4.4k receipts.

## V1-1053 measurements

Medians, macOS arm64, `GOMAXPROCS=3`, on a host shared with other lanes. Wall time is noisy at
±30% between runs. Allocated bytes and mallocs are stable. Output bytes are identical base↔new for
every command, except `queue status` `observedAt`.

**st-1k: 1,000 tickets, about 2.4k receipts, checkpoint present, median of 5**

Base is `265dcd07`. The new column is after the cString and readAll fixes, before the fold.

| command | wall ms | alloc MiB | mallocs | peak fds |
|---|---|---|---|---|
| queue status | 158 → 126 | 114.3 → 51.0 | 575k → 572k | 23 → 23 |
| roadmap | 137 → 112 | 102.6 → 39.6 | 437k → 435k | 10 → 9 |
| ticket list | 172 → 161 | 118.3 → 55.1 | 603k → 601k | 22 → 22 |
| ticket search --facets | 112 → 108 | 104.6 → 41.7 | 447k → 444k | 9 → 10 |
| ticket search --count | 109 → 95 | 101.4 → 38.5 | 427k → 425k | 9 → 9 |
| ticket show | 151 → 127 | 114.2 → 50.9 | 572k → 569k | 22 → 22 |
| receipt audit | 1,492 → 1,106 | 922 → 558 | 9.11M → 6.61M | 534 → 534 |

**st-1k without a checkpoint (moved aside, then restored), 2,387 receipts**

| command | wall ms | alloc MiB |
|---|---|---|
| queue status | 1,645 → 1,429 | 596 → 513 |
| ticket list | 1,438 → 1,389 | 601 → 517 |
| ticket show | 1,311 → 1,487 (noise) | 596 → 513 |
| roadmap | 168 → 161 | 118 → 45 |
| search --facets | 199 → 136 | 120 → 47 |
| search --count | 154 → 171 (noise) | 117 → 44 |

Journal-auditing reads peak at 534 descriptors. Roadmap and search peak at 9.

**st-10k store: generation stopped at 1,936 tickets / 4,374 receipts, checkpoint present, median of 3**

New is the full branch, including the fold and V1-1060.

| command | wall ms | user / sys ms | alloc MiB | peak fds |
|---|---|---|---|---|
| queue status | 278 → 413 (noise) | 165 / 152 → 160 / 202 | 222.6 → 100.5 | 23 → 22 |
| roadmap | 367 → 443 (noise) | 165 / 183 → 137 / 171 | 197.6 → 76.1 | — |
| ticket list | 554 → 602 | 224 / 227 → 178 / 212 | 227.7 → 105.6 | — |
| search --facets | 341 → 318 | 169 / 183 → 132 / 151 | 199.6 → 78.2 | — |
| search --count | 263 → 325 (noise) | 157 / 150 → 124 / 174 | 196.4 → 75.0 | — |
| ticket show | 411 → 350 | 201 / 222 → 149 / 209 | 222.3 → 100.2 | — |
| receipt audit | 3,842 → 2,876 | 2,766 / 1,121 → 1,924 / 836 | 1,832 → 1,111 (18.1M → 13.1M mallocs) | 534 → 534 |

**V1-0841-style descriptor check.**

- Peak descriptors in `receipt audit` are 534 at both 2,193 and 4,374 receipts. They are bounded
  by the 256 request shard directories pinned per attempt, not by receipt count.
- Plain reads peak at 22–23.
- No growth with receipt count was observed above 3,800 receipts. V1-0841 itself (the Darwin change
  guard) was out of scope and was not touched.

**st-1k writes, median of 3**

| command | wall ms | alloc MiB | peak fds |
|---|---|---|---|
| ticket create | 1,111 → 732 | 340.8 → 227.8 | 637 → 656 |
| claim | 1,151 → 883 (bimodal in both builds; reruns new 702/777, base 1,006) | 354.5 → 232–322 | 549 → 549 |
| complete-manual | 849 → 752 | 326.6 → 225.0 | 651 → 668 |

## Top profile costs after the change

- **Full `receipt audit` (4.4k receipts).**
  - About 50% is `safeopen.openat`: the per-file no-follow open beneath the pinned parent
    (CAL-V0-070), reached from `nativeRead.Read` in `walk`/`step`.
  - `wire.Parse` is about 12%. Capture and scan are about 9%.
  - The openat floor is one syscall per receipt, request and attempt file. Lowering it needs fewer
    files, meaning a packed or segment format, which is a persisted-format contract fork.
- **Plain reads with a checkpoint.**
  - Time is dominated by the outer probe's `intent.TreeDigest`, which opens and hashes every ticket
    file, and by the authority mount scan.
  - The probe runs twice per body (`probe.go`), plus a third time in `auditState`'s journal capture.
- **Writes.**
  - `F_FULLFSYNC` dominates.
  - So does the writer-side complete `AuditForWriteObserved`, which writers keep by contract.

## Forks reported, not implemented

1. **Second TM-V0-008 probe.** Removing the re-probe after the body, which would halve plain-read
   probe work, changes the snapshot-consistency contract.
2. **CAL-V0-061 interpretation.** Owner confirmation is needed for the early `SNAPSHOT_MOVED` when the
   head no longer equals the caller's snapshot (see above).
3. **Cross-reader sharing.** Sharing one audit between concurrent reader processes needs read-side
   state writes or a lock (invariant 4).
4. **Packed receipt format.** This is needed to lower the per-file openat floor of the complete audit.
5. **100k tickets.** `MaxTicketsPerQueue` is 10,000, so a 100k store cannot be built. Copied stores
   also refuse with "relocation unsupported", so the harness must generate each store in place.

## Limits and NOT_RUN

- `NOT_RUN`:
  - the 100k scale (blocked by the limit above)
  - the 10k scale, where generation was stopped at 1,936 tickets after about 30 min
  - a real 22.5k-receipt store under an active pool
  - Linux
  - repository-wide `go test ./...` and `make gate`, per the lane rules
- Wall times come from a host shared with several other test lanes. Base and new were measured
  interleaved, but individual wall cells can invert (marked "noise"). Allocation and descriptor
  figures are the reliable comparison.
- Under 3 readers plus 3 writers, p95 is still above 1 s. The residual is the outer probe's
  ticket-tree digest repeated per body. It is ticketable as an optimization, not a correctness gap.

## Remaining hot spots to ticket

- **Outer probe.** The probe re-reads and hashes every ticket file once or twice per body.
- **Complete audit.** It costs one openat per state file, and the floor needs a format fork.
- **Claim latency.** It is bimodal (2.4M vs 3.8M mallocs) in both builds, and the cause has not
  been diagnosed.
- **Writes.** Writes run the full writer-side audit on every mutation.
