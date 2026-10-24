# Heartbeat and renew skip staging-directory syncs; the wider fsync allowance stops (V1-2203)

Date: 2026-10-24. Ticket: V1-2203. Spec: [LGL-V0-005 to 008](../specs/corvint-tasks-lease-gate-latency-v0.md).
Owner decision: "V1-2203: heartbeat fsync floor" in `2026-10-24-owner-queue-decisions.md`, accepted
2026-10-24. Lane: `claude/tasks-heartbeat-durability-1024`.

## Decision taken

The owner allowed heartbeat and renew receipts to commit without their own fsync. They would become
durable with the next synced write or a group sync of at most 1 s, and recovery would read a lost
tail heartbeat as an older heartbeat. The lane prompt said to stop and report rather than weaken
integrity if the chain design made that unsafe. It does, so the lane implemented only the subset of
the allowance that cannot lose or tear a published artifact.

**Why the full allowance stops (LGL-V0-008).** A heartbeat publishes four artifacts: the receipt
(its link-in is the commit point), the request entry, the attempt and `head.json`.

- If the receipt's own sync is skipped, a crash can keep the head's advance and lose or tear the
  receipt. The audit then reports `JOURNAL_FORKED` or `MALFORMED`, the fork-detection signal.
- If the head's advance is lost while the receipt survives, reads refuse `REDO_PENDING` until a
  mutating command redoes it.
- `TestLGLV0008_LostLivenessTailIsRefusedNotAdopted` pins all three shapes.

Reading any of them as "an older heartbeat" would need three things the current store lacks:

- a durable watermark that tells a liveness tail apart from an authority tail. The closed
  `head.json` member set cannot carry one, so this is a store-format cutover that old binaries
  refuse.
- recovery that deletes committed receipts and reissues their sequence numbers.
- a 1 s group sync, which needs a periodic process (product invariant 7 forbids a daemon).

The owner's premise also does not hold for renew. `planRenew` sets the expiry to the renew's own
recorded time plus its lease minutes, so a renew can shorten a lease. If such a renew is lost, the
earlier and longer expiry stays in force, and the lease looks newer, not older. That design needs
its own owner decision on the store format.

**What ships (LGL-V0-005 to 007).** A heartbeat or renew session skips only the sync calls on
`staging/`:

- after each prepare;
- after each slot removal or orphan unlink;
- after each replace's rename out of staging.

It still crosses the same boundaries and runs the same checks. Every file sync and every
destination directory sync still runs, so each published artifact is as durable at its commit point
as before. Staging entries are never published state. The only new post-crash shape is a slot name
the crash restored, holding a second link to the receipt, request entry, attempt or head. That is the orphan a killed
writer already leaves (CAL-V0-019): the audit sets it aside once the writer lock is free
(CTS-V0-026), and the next writer removes it. `TestLGLV0007_LivenessStagingResidueIsAnOrphan`
injects exactly that residue after a heartbeat and after a renew, on both lease routes, and checks:

- the audit verifies;
- the lease reads as acknowledged;
- the next heartbeat clears the residue and commits;
- a second holder is not granted the lease.

Claim, submit, gate, complete, release and every other write keep every sync
(`TestLGLV0006_OnlyLivenessWritesSkipStagingSync`). So does another write's pending redo, even when
a heartbeat is the request that finds it (`TestLGLV0005_LivenessRequestRedoesPendingWriteWithFullSyncs`,
added for the independent review's P2 finding).

**Decision 0549 F2.** F2 kept "the current 4 to 5 `F_FULLFSYNC` calls". The owner decision amends
F2 for heartbeat and renew. This change uses that amendment only for the staging directory, and
records it in [decision 0549](../decisions/0549-tasks-lease-write-performance-design-2026-10-20.md).

**On-disk format.** No stored state, wire format or receipt changes. A store written by this binary
reads unchanged under the previous one, and the previous binary clears any staging residue the same
way.

## Measurements (linux/WSL2, ext4, under load)

**Host and run setup.**

- Harness: `TestCALV0118_WriterHoldProfile`, 13,500 receipts, 880 tickets, median of 5 reps.
- Offload host: AMD Ryzen 9 5950X, 32 logical CPUs, run through `heavy.sh`.
- `TMPDIR` was `$HOME/bench/tmp` on ext4. The host's `/tmp` is tmpfs, where fsync costs about
  nothing.
- Both runs used origin/main `ec41e4e9c`. "After" adds only this change, not batch #1716.

**Load.** The host was not quiet: other lanes kept it loaded.

- Before: load 32.4 / 31.0 / 22.8 at start and 28.9 / 30.2 / 22.8 at end.
- After: load 81.1 / 60.0 / 37.5 at start and 62.0 / 57.7 / 37.5 at end.

**Results.** Wall, hold and `lease.fsync` are medians in ms.

| Verb | Wall before | Wall after | Hold before | Hold after | fsync before | fsync after |
| --- | --- | --- | --- | --- | --- | --- |
| heartbeat | 152.4 | 112.2 | 130.0 | 93.1 | 68.4 | 41.2 |
| renew | 178.0 | 135.5 | 161.1 | 108.7 | 90.7 | 52.4 |
| claim | 829.3 | 766.6 | 629.4 | 549.3 | 115.4 | 128.3 |
| release | 620.5 | 589.7 | 595.5 | 574.9 | 125.7 | 108.4 |

- Heartbeat samples, before: 133.1, 103.0, 152.4, 160.2 and 192.6 ms.
- Heartbeat samples, after: 100.4, 106.7, 182.2, 149.9 and 112.2 ms.
- Sync calls per heartbeat or renew fell from 16 to 8 (from 18 to 10 when the write creates a
  request shard). Counted by `LeaseTiming.Syncs`.
- The claim fsync is unchanged within the noise, as expected.

**Target: the V1-0887 heartbeat median of at most 50 ms on a quiet host is NOT MET.** No quiet-host
run was possible.

The following projection is an inference, not a measurement. The 2026-10-23 quiet WSL2 baseline
had a 72 ms heartbeat median, with `lease.fsync` near 32 ms over 16 calls. Halving the call count
would put fsync near 16 to 19 ms and the wall near 56 to 59 ms. That is still above 50 ms.

The rest of the floor is the eight retained syncs, the journal write CPU (about 27 ms here) and
the snapshot read (about 22 ms). The no-fsync allowance that could remove the retained syncs is
the store-format question LGL-V0-008 leaves for the owner. The skip removes a fixed number of calls per
write, so the receipt count does not change it. This lane measured no 40k point.
