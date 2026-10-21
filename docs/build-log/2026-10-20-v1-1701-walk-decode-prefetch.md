## 2026-10-20 V1-1701: complete journal walk decode reuse and bounded read-ahead (JRT-V0-015..016)

Lane pool-checkpoint, base origin/main 90ab9f33 (build 697), darwin/arm64 (12 cores), go1.27.2.

Re-homed (owner-approved, 2026-10-17): these requirements were first numbered CAL-V0-380..381 in the
agent leases spec; they moved to JRT-V0-015..016 in `docs/specs/corvint-tasks-journal-retention-v0.md`
with the same text so the leases spec stays within the 256-obligation bound.

**Finding: the openat profile was misattributed.** The ticket's profile put about two thirds of the
walk in `syscall.Syscall6` (`openat`). The macOS CPU profiler charges that frame for more than it
costs. The ticket itself measured an isolated `openat` at about 18 us per file. Removing
only decode work, with the reads unchanged, cut the walk median from 22.8 s to 14.3 s, yet the profile
still blamed `Syscall6` at 66%. The walk was CPU-bound. Every receipt's inline ticket and request
afterimages were parsed two or three times: once in `validateRecord`, once in step's
`ticket.Decode`/`snapshot.DecodeRequest`, and once for the operator-note binding. All of it ran on
the walking goroutine.

**Change.**
- JRT-V0-015: `validateRecord` returns the ticket or request it decoded. The step and the note
  binding use that value. `snapshot.RequestFromValue` and `ticket.FromValue` keep the size bounds and
  refusal text.
- JRT-V0-016: within one 512-receipt abort window and a 512 KiB byte budget, the walking goroutine
  reads receipts sequentially through the same `nativeRead`. Worker goroutines (at most
  min(GOMAXPROCS, 8)) decode each receipt and validate its inline posts. `nativeRead` is not
  goroutine-safe, so reads stay sequential. step still runs every check in order.
- Elided receipts go through `ReceiptRestorer.Next`, which is stateful. Blob-backed posts and failed
  reads or decodes stay on the sequential route.
- The first window had no byte budget. It failed `TestTMV0002_AS10_ReceiptBytesAreStreamed` with
  5,052,744 bytes of heap growth against the 2 MiB bound, so the budget was added.

**Measurements** (`TestV11701_WalkCost`, one complete `Audit` per run, 5 runs, median). The synthetic
`historyStoreAt` fixture has 47,000 receipts and about 94k files, at
`/private/tmp/cem10-build/pool-checkpoint-scratch/w47k`. Command:

    CORVINT_TASKS_WALK_COST=<report>.json CORVINT_TASKS_WALK_COST_STORE=<scratch>/w47k \
      go test -count=1 -timeout 30m -p 4 -run '^TestV11701_WalkCost$' ./internal/tasks/store/

| run | median | runs (ms) | load before / after |
|---|---|---|---|
| before (base, same-load rerun) | 21,206 ms | 21206 20833 22423 21440 20644 | 20.92 / 10.71 |
| before (first run) | 22,758 ms | 20751 21782 22758 38767 51082 | 9.00 / 70.63 |
| decode reuse only | 14,341 ms | 23828 15400 14230 14306 14341 | 13.89 / 9.86 |
| decode reuse + read-ahead, final (512 KiB budget) | 5,844 ms | 17488 5538 6805 5834 5844 | 40.85 / 26.01 |
| final, 3 runs with CPU profile | 6,600 ms | | 8.93 / 10.21 |

The same-load before rerun reverted the change with `git diff > patch && git apply -R patch`. The
first run of each series includes a cold page cache. The final profile (`after-3.prof`) and the
reports are kept in the scratch directory above. Its top frames are now runtime scheduling
(`pthread_cond_wait`, `usleep`), `Syscall6` at 14% flat, and wire string parsing. What remains is
sequential I/O: two captures listing and `lstat`ing about 94k files, 47,000 receipt reads, and
settle's 47,000 request reads.

**Outcome.**
- Criterion 1, a few seconds at 47k entries: met at a 5.8 s median under load 26 to 41. The profile
  is retained.
- Criterion 2, identical results: met by `TestV11701_PrefetchedWalkMatchesSequentialWalk` and
  `TestV11701_PrefetchedWalkRefusesLikeSequentialWalk`.
- Criterion 3, closing GitHub #1063 with this measurement: NOT_RUN in the lane. It needs the
  orchestrator.

**Not done:** concurrent reads, which need a goroutine-safe `nativeRead`; capture cost. Both are left
for a later slice and overlap the claude/feat-tasks-perf segment work.
