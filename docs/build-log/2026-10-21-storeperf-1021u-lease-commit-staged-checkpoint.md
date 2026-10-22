# Lease commit stages the derived checkpoints before the writer lock (JSG-V0-015, V1-0850)

Lane storeperf-1021u, base `9ae946a5c2b03f69a327c9230c30d9f35d1f19dd`. JSG-V0-015 is accepted under
the owner's standing auto-accept (2026-10-11).

## Problem

V1-0850 reported that the complete-route lease commit (`commitLease`) retains the read checkpoint
under the writer lock. It derives `proof.Checkpoint()`, encodes it and writes it to a fixed `.tmp`
before the rename. It then encodes and writes the fresh writer-checkpoint base the same way. Both
files grow with the store: on this repository's store, at 2,371 tickets and 11,022 receipts, they
are 822 KB and 1.29 MB. Every complete-route lease write paid that cost while other writers waited.

## Decisions

- **Stage before the lock.** `commitLease` encodes both files before `AcquireLock` and writes each
  one to a private, uniquely named temporary (`<dest>.staged-*`, mode 0600) beside its destination.
  Under the lock, after the head-digest check, it only renames them into place and writes the
  bounded writer delta. This stays within decision 0549. It adds no cache and no new evidence
  source. The files keep their formats, and readers keep their verification (CAL-V0-115,
  JSG-V0-003/004).
- **Bind to the token.** A staged writer base is published only when two things hold:
  - the commit retains under the invalidation token the base was encoded with;
  - the lock holder still reads that token.

  If the commit's own complete audit replaced the token (`verifyWriterCheckpoint`), the commit falls
  back to encoding under the lock, as before. If another writer replaced the token, nothing is
  retained. This keeps the write-base-then-delta order (JSG-V0-004) and the invalidation contract.
- **Stopped commits.** A stopped commit leaves a temporary. The next stager removes any older than
  ten minutes, and readers never open them. Removing a live one early only skips retention of
  derived state.
- **Out of scope** (residuals):
  - the writer route (`leaseWriter`, `mutateWriter`), `Mutate`'s complete route and the
    writer-checkpoint refresh still retain under the lock;
  - `verifyWriterCheckpoint` still compares an old checkpoint in O(store) under the lock.

## Ticket dispositions

- **V1-0850: DELIVERED** (JSG-V0-015).
- **V1-1653: ALREADY-DELIVERED, not reproduced.** Commit 557f6e44e (PR #1452, PSR-V0-169) schedules a
  non-publishing refresh only when `refreshDue` holds. The first refresh is due at 512, then every
  64 (`writerRefreshRetry`). It does not run a complete audit on every write. Test:
  `TestPSRV0169_NonPublishingRefreshDoesNotRecurPerWrite`.
- **V1-0646: ALREADY-DELIVERED.** Two commits cut the intent-tree passes per read from 5 to 2, the
  two TM-V0-008 probes, which is the minimum:
  - 900cf18f7 lets LoadTree reuse the probed tree (`reuseProbedTree`);
  - 9565d5fe7 (CAL-V0-140, V1-0893) gives the journal audit the outer probed tree.

  Tests: `TestCALV0140_AuditSharesTheOuterIntentTree` and `TestTMV0008_ProbedTreeReadParity`.
- **V1-0910: SKIPPED, needs an owner decision.** Decision 0549 F1 declined option (b), the
  stat-keyed cache: it adopts file stamps as evidence, which CAL-V0-116 declines. JSG-V0-001 removes
  intent reads from liveness writes instead. Building the cache would reverse an accepted decision.

## Evidence

**Harness.** The measurement was a scratch harness, not committed, built on the store test helpers
(`historyStoreAt`, `holdLeasePolicy`).

- **Setup.** Each rep is a complete-route lease claim, taken after the writer and read checkpoints
  are removed and the retained audits are forgotten. Each claim uses its own worktree and holder and
  is released after measurement.
- **Samples.** Each figure is the median of 4 claims, at 880 and 2,400 tickets (receipts = tickets +
  64).
- **Before.** The same harness ran with `commitLease` reverted to retain under the lock.
- **Host load.** The 1-minute load was 16 to 18 on a 12-core host. That is above the core count, so
  under the B1 rule the durations are recorded but do not qualify. The byte counts do not depend on
  load.

| Tickets | Bytes written under the writer lock (before → after) | Lock hold, ms (before → after) | Claim wall, ms (before → after) |
| --- | --- | --- | --- |
| 880 | 248,589 → 672 | 84.7 → 51.8 | 1,147 → 878 |
| 2,400 | 666,542 → 673 | 55.1 → 45.4 | 1,729 → 1,559 |

What the numbers show:

- **Locked bytes.** Before the change they grew linearly with the store. After it, only the writer
  delta, about 0.7 KB, is written under the lock, at every size.
- **This repository's store.** At 2,371 tickets and 11,022 receipts, the derived files are 822 KB
  (read checkpoint) and 1.29 MB (writer base). About 2.1 MB of encoding and writing leaves the
  locked section of every complete-route lease commit.
- **Wall time.** The work is moved, not removed, so wall time should be about unchanged. The wall
  drop above is within host noise.
- **Files read.** Unchanged. No read path changed; the commit opens two staged temporaries and
  renames them. Read counts were not instrumented: NOT_OBSERVED.
- **Lock hold.** It includes every lock acquisition in the claim, so the sweep and receipt commit
  dominate it at this load.

Tests: `TestJSGV0015_LeaseCommitStagesCheckpointsOutsideWriterLock` and
`TestJSGV0015_StagedWriterCheckpointNeedsItsToken`. The first fails when `commitLease` is reverted
to `retainCheckpoint` and `retainWriterCheckpoint` under the lock (observed: 6,125 + 7,490 bytes locked at 40 tickets, 51,797 + 60,411 at 400).

## NOT_RUN

Run locally:

- **`./internal/tasks/store`.** Full package, run in 8 name shards. The unsharded run exceeded the
  30-minute hang detector at host load 13 to 50, with no failure reported.
- **`./internal/tasks/cli`.** Full package, 678 s.
- **Other packages.** `./internal/tasks`, `mutation`, `ticket`, `transaction`, `scale` and `service`.
- **Pre-push traps.** `TestCALV0131_LiveFormatsCoverEveryDecodedProfile` and
  `TestImportDirection`.
- **Checks and gates.** go vet, gofmt, the doc gates, the extra CI gates and `make windows-vet`.

NOT_RUN:

- **Tier 2.** The tier-2 units of `corvint affected` (75 packages, including `./cmd/corvint`) are
  left to batch CI.
- **Benchmarks B1 to B3.** Not run. The durations above do not qualify.
- **Files-read counts.** Not instrumented (NOT_OBSERVED).

## Rollback

Revert the commit. The checkpoint files keep their formats. A leftover `.staged-*` temporary is
inert and can be deleted by hand.
