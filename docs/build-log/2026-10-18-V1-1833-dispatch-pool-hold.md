# V1-1833: dispatcher-held pool members (issue 1147)

Ticket V1-1833, GitHub issue 1147. Adds ATR-V0-120..124 to
[corvint-tasks-attempt-runner-v0](../specs/corvint-tasks-attempt-runner-v0.md). Accepted under the
owner's standing auto-accept of lane requirements (2026-10-11); this is not a new owner decision.

## Decisions

- **Claim before launch, not acquire mid-run.** A ticket that requires a pool only matches a role
  with `match.pool`, and that role's sessions claim with `--pool`, which already allocates the
  member. A dispatcher `pool acquire` on the session's attempt would therefore find it holding a
  member already. So a `holdPool` role has the dispatcher run that native `claim --pool` itself,
  with the worker as holder, before spawn. The store's admission, room gate and waitlist order
  (grace, then priority, then arrival; PSR-V0-130..136) decide every claim; the dispatcher never
  names a member and never queues of its own.
- **A per-worker hold record, not a ledger field.** The ledger profile is closed-decoded, so the
  hold lives in `workers/<id>/pool-hold.json` (`taskman-dispatch-pool-hold/0`). That same file is
  what the session receives as `CORVINT_DISPATCH_POOL_FILE`, and `dispatch status` reads it.
- **Return before hand-off.** On worker end the exact allocation is returned with `pool release`
  (CAL-V0-201) before the attempt is handed off, so a killed session leaves no allocation whether
  hand-off is on or off. An attempt that ended first was quarantined natively (CAL-V0-202) and is
  only recorded.
- **Orphan sweep.** A crash between claim and ledger save leaves a live attempt held by no worker.
  When this program's hold record names that attempt, heal returns and releases it. A crash before
  the record is written is bounded by the attempt's lease.

## Evidence

`TestATRV0122_KilledSessionLeavesNoOrphanedAllocation`, `TestATRV0122_EndedAttemptIsNotReleasedAgain`,
`TestATRV0122_FailedLaunchReturnsTheClaim`, `TestATRV0124_RefusedClaimKeepsNativeOrder`,
`TestATRV0124_OrphanedClaimIsReturned`, `TestATRV0120_HoldPoolIsOptIn` (`internal/tasks/dispatch`),
`TestATRV0124_PoolClaimKeepsWaiterOrderCLI` (real store) and `TestATRV0123_DispatchStatusShowsPoolHold`
(`internal/tasks/cli`). Live dispatcher qualification and Linux are NOT_RUN.
