# 2026-10-08: V1-1016 reap a running worker's expired lease

## Intent

Owner request [issue 674](https://github.com/beamfall/corvint/issues/674), native ticket V1-1016.
Under heavy host load, dispatched workers kept running after their attempt leases expired
(`holderStatus` `LEASE_EXPIRED`, `phase` `RUNNING`). `heal.reap` skips every holder that is a
running worker, so the attempts kept their pool members ALLOCATED for up to 75 minutes and no
ticket completed for six hours. CAL-V0-104 (issue 622) handles only a worker that exited. The owner
asked that the dispatcher reap such an attempt after a configured grace, stop the worker the way the
wall-time cap does, and record a `lease-expired` event.

## Change

- `internal/tasks/dispatch/config.go`: optional role `expiredLeaseGraceSeconds` (0..86400, absent
  is 600), validated by the closed decoder; `Role.ExpiredLeaseGrace` returns the default for an
  absent value or a nil role.
- `internal/tasks/dispatch/loop.go`: the `heal.reap` pass no longer skips running holders outright.
  For a live attempt held by a running worker of this dispatcher, `reapRunningExpired` reaps it once
  the lease has been expired for longer than the grace of the role the worker launched under. The
  request ID is deterministic (`reap`, attempt, generation, `lease-expired`). A successful reap
  emits `lease-expired` and marks the worker `KILLING` with reason `LEASE_EXPIRED`, so the next
  supervision pass stops the whole tree with SIGTERM, then SIGKILL after `killGraceSeconds`. A
  refused reap emits `alert` and leaves the worker running for the next tick's retry.
- `internal/tasks/dispatch/ledger.go`: `lease-expired` joins `EventKinds`.
- Spec `docs/specs/corvint-tasks-agent-leases-v0.md`: new proposed `CAL-V0-191` (V1-1016 section),
  status, inputs, slice table and traceability rows; `docs/specs/INDEX.json` and `README.md` carry
  the same status. `docs/TASKS-SUPERVISION.md` gains one paragraph.

## Decisions

- Gated on `heal.reap`, with no separate switch: reaping is what `heal.reap` already authorizes, and
  turning it off keeps the old behavior. A large grace is the per-role opt-out.
- The default grace is 600 seconds. CAL-V0-010 refuses `renew` once a lease has expired, so a worker
  past expiry can never become valid again. The grace only absorbs clock and observation lag and
  gives a worker time to release by itself. It does not decide whether the attempt survives.
- Reap first, then stop. A reap frees the member even if the stop is slow or a process survives
  SIGKILL. The stop reuses the wall-cap path, so a restart finishes it from the recorded `KILLING`
  state. The issue says "KILL after idleSeconds", but the wall cap actually waits
  `killGraceSeconds`. The requirement follows the actual wall-cap behavior, because the issue asks
  for "the same as the wall-time limit".
- The grace is read from the configuration the worker launched under, like `idleSeconds` and
  `wallSeconds` (CAL-V0-127).
- `lease-expired` is a new event kind, as the issue asked. The `reaped` event is not also emitted.
- Non-goal: the optional "lease expired, stop and hand off" notice. No adapter injects messages
  into a running session, and the reap already fences the attempt.

## Evidence

- `TestCALV0191_RunningWorkerPastGraceIsReapedAndStopped`: a real `sleep` worker on a fake clock is
  untouched at exactly the grace. One second later it is reaped once with the deterministic request
  ID, `lease-expired` carries the attempt, grace and expiry, and the next tick kills the process
  (`killing`, `killed`, `finished`).
- `TestCALV0191_WithinGraceOrRenewedIsUntouched`: within the default grace, a renewed (future)
  lease, and `heal.reap` off all make no store write and leave the worker running.
- `TestCALV0191_RefusedReapKeepsTheWorker`: a refused reap alerts, keeps the worker and retries
  with the same request ID.
- `TestCALV0191_ExpiredLeaseGraceConfig`: an absent value is not serialized and defaults to 600;
  0, 1 and 86400 are accepted; -1 and 86401 are refused; an unknown role key is refused.
- The focused `internal/tasks/dispatch` package, the dispatch-related `internal/tasks/cli` tests,
  `go vet` and the doc gates were run; results are in the change report. Live dispatcher
  qualification is NOT_RUN.

## Rollback

Revert the commit. A configuration that sets `expiredLeaseGraceSeconds` is then refused as an
unknown member. Events already written stay readable. A worker recorded `KILLING` with reason
`LEASE_EXPIRED` is still stopped by the old supervisor, which replays the recorded reason.
