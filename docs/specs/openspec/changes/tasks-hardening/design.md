# Design: `tasks-hardening`

## Decisions

- Build in `beamfall/corvint-tasks`, on top of the imported tree (PR #4). If PR #4 has not merged
  when the lane starts, the lane bases on its head and lands after it; nothing is built on Tasks
  `main` at ae47138, which lacks the lease code.
- One liveness rule for reads and writes. Reads use the existing `expired()` predicate
  (`transaction/lease.go:774`) at the read's `observedAt`, the same clock `queue status` already uses
  for `holderStatus: LEASE_EXPIRED`. The write path is unchanged: claim still reaps first
  (CAL-V0-011). The read now predicts what the claim does.
- `nextAction` for an expired holder's ticket is `claim`, because a claim on that ticket reaps the
  lease and admits the caller in one step (the existing `expiredRemedy` text says the same). The reap
  command is still shown for an operator who only wants to clear the lease.
- Lease extension by progress (TKF-V1-003, 004) is "later of current and now plus L", with L from the
  command or `DefaultLeaseMinutes` (60, `transaction/lease.go:46`). The lease record stores no
  granted length (`snapshot.Lease` holds holder, granted sequence and expiry), so the default is
  used; storing the granted length is not needed for this change.
- Heartbeat extends rather than being removed: it is strictly more forgiving, keeps every existing
  caller working, and removes the rule "renew AND heartbeat every 10 minutes" (decided, R42 (panel
  pick) C1).
- `finish` composes existing transactions. It adds no new state, no new phase and no new completion
  kind. A renew before each gate covers a gate command that outlasts the lease, because `gate run`
  records its result only after the command finishes (`transaction/lease_gate.go:249-275` checks
  fencing at write time).
- Counter and output changes (TKF-V1-002, 006, 008 to 012) land before `tasks-conformance` freezes
  schema v1, so schema v1 starts with the corrected names.
- Thresholds are policy keys with defaults (`completionHealth.maxManualShare` 0.5,
  `progressStaleDays` 7), so a project can tune them without a release.

## Failure modes

- A holder renews between a read and a claim: the read showed the lease as expired at its
  `observedAt`; the claim reads the store again and finds it live. The claim refuses as today.
- A command arrives after expiry: still fenced (CAL-V0-010). TKF-V1-003 prevents expiry during
  progress, not after it.
- `finish` is interrupted between steps: each step is its own receipt; rerunning with the same
  request ID skips the accepted steps by request-ID replay.
- `finish` finds two live attempts for one ticket: impossible by the one-entry-per-live-attempt
  invariant (`entryOracle`, `transaction/lease.go:956-958`); if seen, refuse with `MALFORMED`.
- A script reads `intentChecksPassed` or relies on `roadmap` listing closed tickets: it breaks.
  The supported-command list and schema v1 record the change; `roadmap --all` restores the old set.
- The manual-share threshold flags a project that completes manually on purpose: the project sets
  `completionHealth.maxManualShare` to 1.

## Acceptance evidence

- Unit tests for each scenario in `specs/tasks-hardening/spec.md`, traced by requirement ID.
- A read-only replay against a copy of the Corvint store with three expired leases: `plan preview
  --summary` shows no collision from them, `queue status` shows `attemptsExpired: 3`, `ticket
  blockers` shows `LEASE_EXPIRED`; the copy's file hashes are unchanged.
- One ticket completed with `finish` in a scratch store, and one `finish` stopped by a failing gate,
  both with receipts and a clean `receipt audit`.
- `receipt audit` on the Corvint store copy reporting `MOSTLY_MANUAL` with the share.
- The conformance pack of `tasks-conformance` includes `finish` and `next` on the supported-command
  list.

## Rollback

Revert the corvint-tasks commits. No store format changes: lease extension writes the existing
`expiresAt` field, and the new policy keys are optional with defaults. Reverted reads return to the
old counters. Retire TKF-V1-001 to TKF-V1-012.

## Decided questions

1. Decided, R42 (panel pick) C1: heartbeat extends the lease (TKF-V1-004).
2. Decided, R42 (panel pick) C2: `finish` ships in 1.0 (TKF-V1-005).
3. Decided, R42 (panel pick) C3: defaults `maxManualShare` 0.5 and `progressStaleDays` 7.
4. Decided, R42 (panel pick) C4: the `roadmap` default shows open and held tickets by priority;
   `--all` restores the old output.
