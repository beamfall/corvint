# V1-1597: host sleep awareness

Date: 2026-10-17. Base: `072a306f1dbd89c2f29b20a3434a0f25864ebbf7` (origin/main).
Ticket: V1-1597, from GitHub issue 1040. Spec: `docs/specs/corvint-tasks-attempt-runner-v0.md`,
ATR-V0-063..065, accepted under the owner's standing auto-accept of lane requirements
(2026-10-11).

## Decisions

- Detection. Each tick compares the wall-clock and monotonic time since the previous tick.
  `time.Now` carries a monotonic reading that does not advance while macOS or Linux is suspended.
  Tests inject `Dispatcher.Mono`. A fake wall clock alone never detects a gap, so the existing
  fake-clock tests keep their behaviour.
- Exclusion, not mutation. Recorded gaps are subtracted where a timer is compared (`slept`). The
  alternative was to shift `Started`, `LastActive` or the lease expiry, which would rewrite
  persisted ledger facts and the reap request identity.
- Leases. The issue proposed a one-time lease extension. The store lease is not touched. The
  dispatcher's reaper excludes the gap from its CAL-V0-191 grace instead. That covers the reap of
  a live worker without a store write.
- State. The gaps live in a sidecar record, `host-sleep.json` (profile
  `taskman-dispatch-host-sleep/0`), so `dispatch status` can show the last gap. The event is an
  `alert` with `hostSlept` detail, keeping the closed event vocabulary.
- Residuals. Keep-awake (`service install --keep-awake`, off by default, documented as not
  preventing lid-close sleep) and a doctor finding for suspends are not delivered. Issue 1040's
  sixth acceptance criterion stays open, so the issue is referenced, not closed.

## Evidence

- `TestATRV0063_SuspendIsRecordedAndExcludedFromTimers`: with a three-hour suspend, the gap is
  recorded once as one alert and one record entry. Neither the wall cap (7200 s) nor the idle cap
  (3600 s) fires, and a lease that expired inside the gap is not reaped. The reap follows 121 s
  after the wake under a 120 s grace. Mutation check: with `slept` returning zero, the test fails
  on the WALL stop.
- `TestATRV0063_NoGapLeavesTimersUnchanged`: in-step clocks record nothing, and the wall cap fires
  as before.
- `TestATRV0063_RecordVersions`: a newer record is kept, and a malformed one is replaced.
- `TestATRV0065_DispatchStatusShowsLastHostSleep` and the CAL-V0-131 format row.
- The lease keeper's idle check has no separate test. Linux: NOT_RUN. A real suspend: NOT_RUN.
