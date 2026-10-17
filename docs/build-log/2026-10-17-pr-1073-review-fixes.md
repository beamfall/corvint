# PR 1073 review fixes: dispatch batch (V1-1558, V1-1595, V1-1597, V1-1600)

Date: 2026-10-17. Branch `claude/dispatch-batch-1582`, fixing head `63953dc9e`.
Source: one independent Codex review round of PR 1073 (one P1, four P2 findings). Spec:
`docs/specs/corvint-tasks-attempt-runner-v0.md`. Each fix has a regression test that fails
without it.

## Findings and dispositions

- P1, ATR-V0-064. The wall cap, the idle cap, the expired-lease grace and the lease keeper subtracted
  recorded suspend gaps from `now.Sub(t)`. When both instants carry a monotonic reading, Go already
  measures the elapsed time on the monotonic clock, which does not advance during a host suspend on
  Darwin or Linux, so the gap was excluded twice. The fix is `Dispatcher.awake`. It uses the
  monotonic difference when both readings exist, and otherwise the wall difference less the
  recorded gaps. The spec now says so.
  `TestATRV0064_MonotonicTimestampsAreNotDiscountedTwice` uses `time.Now()` timestamps for the wall
  cap, the idle cap and the keeper idle check.
- P2, ATR-V0-060. A short-session hold's `holdUntil` was not a wake deadline of the idle
  StoreWitness gate. A held ticket therefore waited for the 15-minute full pass. `idleSettle` now
  adds every hold end. `TestATRV0060_IdleGateWakesAtHoldEnd` drives the production StoreWitness
  path.
- P2, ATR-V0-059. A detached claim's PENDING, LOST and refused envelopes carried no drain notice.
  A kept result that was replayed after the drain began did not carry it either.
  `leaseDetachClaim` gives every detached-claim envelope the notice exactly once. Envelopes that
  need no notice pass through byte for byte. `withDrainNotice` no longer adds a second copy.
  `TestATRV0059_DetachedEnvelopesCarryDrainNotice` covers both cases.
- P2, ATR-V0-058, ATR-V0-061 and ATR-V0-063. The drain-signal, session-churn and host-sleep readers
  accepted the following:
  - a missing or empty `profile`;
  - another profile at the same version;
  - trailing data after the object.

  They now require their own profile and EOF. `TestATRV0058_RecordReadersAreClosed` covers all
  three readers.
- P2, ATR-V0-051. The lock-contention retry test passed without the retry loop, because a single
  try waits for the lock. A test-only per-try fault (`SetRunOutcomeTryFault`) now fails tries
  deterministically. `TestATRV0051_FailedOutcomeWriteIsRetried` proves two things. First, a failed
  try is retried under the same request ID and recorded (exit 124). Second, tries that keep failing
  stop inside one budget (exit 127, `LOCK_TIMEOUT`). With the retry disabled, both subtests fail.

## Limits

These were run on Darwin only. Linux and a real host suspend are `NOT_RUN`.
