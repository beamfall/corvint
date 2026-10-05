# Tasks user service: launch fence, drain and legacy stop file

Owner request: [issue 500](https://github.com/beamfall/corvint/issues/500), native ticket V1-0697.
Governing spec: [corvint-tasks user service](../specs/corvint-tasks-user-service-v0.md), SERVICE500-001/003.

## Decisions

- **One short fence, F (`fence.lock`).** F serializes every control write with every launch admission. The writes are stop, drain, resume, install bind, rollback restore, legacy latch and drain settlement. The managed main opens its dispatcher through the new `dispatch.OpenControlled`, which takes a `LaunchControl`:
  - `Admit(intent)` runs before each new worker launch, each new pool-sweep start and each replay of a retained STARTING or UNKNOWN sweep (native admission unproved). A PENDING sweep replays unfenced;
  - an admitted launch holds F from its final control read through spawn and the saved ledger record. The dispatcher rechecks its context after F is acquired, so a cancel during the wait starts nothing;
  - `Boundary(recorded, settled)` runs between ticks only, after an uncancelled tick completed and its final ledger save succeeded;
  - a nil control is refused before ownership is taken.

  Lock order is U→L→F and O→F. F is never held while taking O or L, and F's holders never wait on a manager, a pulse or a join.
- **Durable pre-spawn intent (review P1).** Admission saves `launch-intent.json` in the service state root (intent id plus the dispatcher's random token, fsync and directory fsync) before the effect starts. Release removes it only when the outcome is durable (`release(recorded)`). An unrecorded intent blocks admission, stop ACK (passed to the model as IntentStates) and drain settlement. It is resolved by a recorded boundary of the same dispatcher, or at the next open when the saved ledger records it (`dispatch.Records`). This needs no ledger format change. Automatic no-orphan reconciliation of an unrecorded intent would be its own slice.
- **Stop acknowledgement.** The old observed-boundary rule was "ACKNOWLEDGED only when no owner and no RUNNING pulse". The fence replaces it:
  - stop is ACKNOWLEDGED once suppression is durable and the owner is absent or is the fenced managed main (owner lock PID equals the PID of a fresh pulse bound to the manifest);
  - a foreign or unknown owner stays PENDING;
  - the new `close` field reports OBSERVED or PENDING separately, so ACK never claims the dispatcher closed.
- **Drain.**
  - Drain applies only from RUNNING.
  - DRAINING is runnable for the main: the dispatcher supervises, heals and accounts while its fence refuses launches.
  - Settlement moved from the main's poll to the dispatcher tick boundary (review P2-2). The saved ledger is published by the progress save before `finish` accounts ended workers, so a poll could see it settled early. Under F, `Boundary` writes STOPPED at R+1, keeping the drain request, only when the tick was recorded and settled (no worker, no pending or in-flight sweep) and no intent is unresolved. The dispatcher then returns `ErrSettled`, which the main treats as a clean end without a hold.
  - Resume wins over a drain. No deadline ends a drain.
- **Legacy stop file.**
  - Presence is observed with an Lstat of the parent before and after (SameFile) plus the ancestor checks, and classified PRESENT, ABSENT or UNKNOWN.
  - Only the main latches RUNNING R to DRAINING R+1, with request `legacy-stop-r<R+1>`, at its poll or at an admission that observes PRESENT, under the same F (review P2-4). Removing the file keeps the latch.
  - UNKNOWN holds new opens.
  - Resume requires a fresh ABSENT: PRESENT is RESOURCE_COLLISION and UNKNOWN is UNCERTAIN_EFFECT.
- **Resume CAS.** Resume observes pins outside F. Under F it re-reads control, refuses RESOURCE_COLLISION if control changed, checks the legacy file, then writes RUNNING and records the request.

## Limits retained

- An intent left by a crash and not recorded in the saved ledger stays UNRESOLVED until an operator verifies no orphan and removes the marker. Automatic reconciliation is not delivered.
- Pre-existing dispatcher behavior, outside this slice: an immediate stop or interrupt between the progress save and `finish` can lose that tick's accounting for progress-granted workers. It can no longer settle a drain.
- Pins are rechecked once per poll, not per launch.
- Legacy latch ids are recorded only in control, not in the request ledger.
- Helpers, `run-helper` and service logging are deferred, and helpers still refuse UNSUPPORTED.
- Runtime restart debt stays NOT_OBSERVED.
- Journal pruning and type-wide systemd drop-ins remain open.
- Platform qualification (SERVICE500-004/005/010) stays NOT_RUN. The owner accepted it on disposable targets only and deferred it. No real launchd or systemd unit was installed.

## Evidence

The new and updated witnesses:

- Dispatcher, real spawn with a fake control: `TestSERVICE500_OpenControlledFencesLaunches`, `TestSERVICE500_UnsavedLaunchStaysUnrecorded` (save failure after spawn), `TestSERVICE500_CancelDuringFenceWaitLaunchesNothing`, `TestSERVICE500_BoundarySettlesRun`, `TestSERVICE500_CancelBeforeAccountingReachesNoBoundary`, `TestSERVICE500_OpenControlledFencesPoolSweepStart` and `TestSERVICE500_RetainedStartingSweepNeedsFence`. Removing the context recheck or the cancelled-tick boundary guard fails the two cancel witnesses.
- Service, temp HOME and fake launchd: `TestSERVICE500_FenceSerializesStopWithAdmittedLaunch`, `TestSERVICE500_UnrecordedLaunchBlocksAckAndSettlement`, `TestSERVICE500_LeftIntentResolvesOnlyFromLedger` (crash), `TestSERVICE500_DrainSupervisesThenSettlesStopped` (a boundary-reporting fake controller), `TestSERVICE500_ResumeDuringDrainWins` and `TestSERVICE500_LegacyStopFileLatchesDrain` (PRESENT at admission, then removal before the poll).

Focused package tests and `go vet` passed for `internal/tasks/service` (also with `-race`), `internal/tasks/dispatch` and `internal/tasks/cli`.
