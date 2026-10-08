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
  request ID is deterministic (`reap`, attempt, generation, `lease-expired`, observed expiry). A
  reap the store reports as done (fresh receipt or replay) emits `lease-expired` and marks the worker `KILLING`
  with reason `LEASE_EXPIRED`, so the next supervision pass stops the whole tree with SIGTERM, then
  SIGKILL after `killGraceSeconds`. A completed reap that changed nothing (the worker ended the
  attempt first) leaves the worker running silently. A refused reap emits `alert` and leaves the
  worker running for the next tick's retry. Every heal pass also stops a running worker that holds
  no live attempt but holds a `FAILED` `LEASE_EXPIRED` attempt (`stopReapedWorkers`). Before
  supervision signals on a tick, `keepReclaimedWorkers` cancels an unsignalled `LEASE_EXPIRED`
  stop (zero `KillDeadline`) for a worker now observed holding a live attempt.
- `internal/tasks/dispatch/ledger.go`: `lease-expired` joins `EventKinds`. The ledger shape is
  unchanged.
- `internal/tasks/dispatch/roster.go`, `internal/tasks/cli/dispatch.go`: `Queue.ReapExpired`
  reports whether this request moved the attempt; an observed `Attempt` carries its native cause.
- `internal/tasks/transaction/lease.go`, `internal/tasks/cli/lease.go`: `reap` accepts an optional
  `--lease-expires-at` with `--attempt` and `--generation`, joins the preimage only when present,
  and refuses `FENCED` when the current lease expiry differs. A supplied empty value, or the flag
  on another verb, is refused `MALFORMED` at the CLI.
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
  SIGKILL. The stop reuses the wall-cap path.
- Independent review (Codex, of b9210faf) found three races, each fixed in a follow-up commit:
  a completed no-op reap (worker released first) still stopped the worker, so the stop now needs
  the store's evidence of an actual reap; the grace was checked against an observed expiry the
  transaction did not fence, so a renewal followed by a late dispatcher could still reap, now
  refused by the `--lease-expires-at` fence; and `KILLING` lived only in memory until the tick-exit
  save, so a crash after the reap left a running worker with a terminal attempt that heal ignores.
  The store's reaped attempt (`FAILED`, cause `LEASE_EXPIRED`, holder kept) is now the durable
  record: each heal pass stops a running holder of one that has no live attempt. A pending-reap
  ledger member was tried first and dropped, because any ledger member moves
  `taskman-dispatch-state` (CAL-V0-131) and so would force a drain to install this fix. Saving
  `KILLING` before the reap was rejected because it would stop a worker whose reap is then refused
  or finds the attempt already ended.
- A second independent review (Codex, of 3989698e..4fc9cdca) confirmed those two fixes and the
  digest and ledger compatibility, and found three more:
  - an empty `--lease-expires-at` read as absent and ran an unfenced reap or sweep: now
    `MALFORMED`;
  - a recovery stop decided on an observation could kill a worker that claimed a new generation
    just after it: the next tick cancels an unsignalled stop when the worker holds a live attempt.
    A signalled stop is never cancelled, so no stopped worker is revived. The claim that lands
    between a tick's observation and its supervision pass stays a residual window, as there is no
    coordination with claim admission;
  - a crash after the reap followed by another holder's claim before restart reuses the attempt ID
    and erases the reaped holder and cause, so the stop is not recovered. It is a documented
    residual limit: the worker runs until the idle or wall cap stops it, which is no worse than
    before this change. Closing it needs a ledger member (a CAL-V0-131 version bump) or a claim
    wire change, both out of scope.
- The issue says "KILL after idleSeconds", but the wall cap actually waits
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
- `TestCALV0191_WorkerThatWinsTheRaceKeepsRunning`: a release, or a renewal, between observation
  and reap leaves the worker running with no event, and the next tick sends no further reap.
- `TestCALV0191_StopAfterReapIsRecoveredAfterRestart`: a ledger captured after the store reap but
  before the outcome is recorded reopens with the worker `RUNNING`; the first tick stops it from
  the store's reaped attempt with no second reap.
- `TestCALV0191_ReapedAttemptSparesAWorkerWithALiveOne`: a reaped attempt does not stop a worker
  that holds a live one; a reap by another writer stops it once it holds none.
- `TestCALV0191_ReclaimAfterTheStopDecisionKeepsTheWorker`: a recovery stop followed by a new live
  attempt is cancelled before any signal and the process survives; a stop that has already
  signalled runs to completion despite a live attempt.
- `TestCALV0191_ReapIsFencedOnTheObservedLeaseExpiry` (`internal/tasks/store`): a stale expiry is
  `FENCED` and changes nothing; the current expiry reaps and replays; a released attempt is a
  receiptless no-change; an expiry without attempt, on another verb, or malformed is `MALFORMED`.
- `TestCALV0191_NativeReapExpiredReportsOnlyAnActualReap` (`internal/tasks/cli`): the native
  adapter reports a fresh reap and its replay as reaped, a fence as an error, and a released
  attempt as not reaped.
- `TestCALV0191_EmptyLeaseExpiryIsMalformed` (`internal/tasks/cli`): an empty expiry on a fenced
  reap or a sweep, and an expiry on `release`, refuse `MALFORMED` and leave the attempt live.
- `TestCALV0191_ExpiredLeaseGraceConfig`: an absent value is not serialized and defaults to 600;
  0, 1 and 86400 are accepted; -1 and 86401 are refused; an unknown role key is refused.
- The focused `internal/tasks/dispatch` package, the dispatch-related `internal/tasks/cli` tests,
  `go vet` and the doc gates were run; results are in the change report. Live dispatcher
  qualification is NOT_RUN.

## Rollback

Revert the commit. A configuration that sets `expiredLeaseGraceSeconds` is then refused as an
unknown member. Events already written stay readable, and store records keep their shape (a reap
without the new flag keeps its digest). A worker recorded `KILLING` with reason `LEASE_EXPIRED` is
still stopped by the old supervisor, which replays the recorded reason. The ledger keeps
`taskman-dispatch-state/3`, so either direction needs no drain.
