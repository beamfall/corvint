# Store lease writer batch: writer-lock contention, refusal codes and store/CLI flakes

Date: 2026-10-20. Lane: store-lease-writer. Base: origin/main 90ab9f33e.

## Decisions

- **V1-1108 / V1-0961.** The program writer mutex was one process-wide `sync.Mutex`. It is now
  keyed by the store root (`StateDir`), and `leaseAudits` keeps one lease audit per root, bounded
  at 8 roots. Independent stores in one process no longer serialize or evict each other, and
  per-store exclusion is unchanged. After review, writer entries count their holder and waiters
  and are removed at zero, so the map holds only stores with a transition in flight and never
  two mutexes for one store. The focused store run
  (`Program|Supervisor|Pool|Lease`) took 232.6 s before and 148.1 s after, on the same host at
  comparable load. This is a single paired measurement, not a benchmark.
- **V1-0908.** `wire.Annotatef` restates a cause with its own location and message while keeping
  its code and `WithoutRetry` mark. The 19 `wire.Errorf(wire.CodeOf(err), ...)` call sites now use
  it, so an annotated error from a call that already ran stays not retryable (CAL-V0-078).
- **V1-0829.** Program, supervisor and control transition refusals now carry `ATTEMPT_LIVE`,
  `QUIESCENCE_UNPROVED`, `SUPERVISOR_LOST`, `FENCED` or the writer's first refusal code instead of
  `MALFORMED` (CAL-V0-037). After review, a recovery boot read that failed for a retryable
  operational cause (descriptor exhaustion) keeps that code instead of `QUIESCENCE_UNPROVED`. This also helps diagnose V1-0771, which did not reproduce.
- **V1-1703.** A preparation audit that refuses is reused instead of walked again (PSR-V0-153).
  The pinned walk count went from 3 to 2.
- **V1-2138.** A claim's Git observation and scope derivation no longer run under the writer lock
  (PSR-V0-168, "Claims (V1-2138)" amendment).
  - `newClaimProbe` prewarms unlocked: the base commit, its tree when a deriver is set, and the
    pool source.
  - Under the lock, the observer serves only memoized results. On a miss it declines the writer
    route with `SNAPSHOT_MOVED` ("claim Git observation deferred") and records the audited state.
  - `settle` then observes unlocked, and the route retries once. A second decline falls back to
    the complete route, which observes Git afresh and reuses the derivation.
  - Review finding (Codex): memoizing `HEAD` resolution and worktree status by argument list let
    the retried route reuse a base observed before another workflow integrated a commit and
    advanced the journal. Only exact-object queries (`rev-parse`/`ls-tree` of a full OID) are
    now memoized by arguments. Every other Git output is bound to the head.json and intent tree
    digests read before it ran, and is served under the lock only while the audited state carries
    them. Trade-off: any concurrent write during the unlocked derivation now sends the claim to
    the complete route (two declines) instead of committing on the writer route. The lock is
    still never held across Git or the deriver, and the deriver still runs once.
    `TestV12138_ClaimRebindsMovedBaseAfterJournalAdvance` fails without the binding: the claim
    committed fast on the stale base.
  - Rollback: remove the `withWriterFacts` install for CLAIM/CLAIM_NEXT in `leaseOnce`. Claims then
    observe Git under the lock as before.
  - Evidence: `TestV12138_ClaimDerivationOutsideWriterLock` blocks the deriver and requires a
    concurrent writer to commit in the meantime. With the install removed, that writer blocked and
    the test failed after 36 s. With the fix it passes, and the deriver runs exactly once.
  - This does not duplicate claude/feat-tasks-perf (V1-0887, V1-1512): that branch does not touch
    the claim observer.

## Flake measurements

- **V1-1532** (archive read-wait). The base did not reproduce: 20/20 passed at load 33, with quiet
  exports of 8 to 23 ms. The test still depended on timing, because the concurrent writer could
  finish before enough exports overlapped it. It now keeps committing until 5 exports overlap,
  with a cap of 400 commits. It passed 20/20 at load 24. No timeout was raised.
- **V1-1955** (dispatch status store-touch). It reproduced 1/10 at load about 6, at
  `dispatch_test.go:90`. Cause: the worker launched by `dispatch --once` claims through the store
  asynchronously. A claim landing between the snapshot and the status read was reported as a
  status write. The fix waits for the worker pid to exit, then compares the store around a second
  status read. It passed 20/20. The "status never writes" assertion is kept.
- **V1-0723** was not attempted. Reproducing it needs the full store package with a long TMPDIR,
  which exceeds the 10-minute foreground limit.
