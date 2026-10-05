# Tasks user service: launch fence, drain and legacy stop file

Owner request: [issue 500](https://github.com/beamfall/corvint/issues/500), native ticket V1-0697.
Governing spec: [corvint-tasks user service](../specs/corvint-tasks-user-service-v0.md), SERVICE500-001/003.

## Decisions

- **One short fence, F (`fence.lock`).** F serializes every control write with every launch admission. The writes are stop, drain, resume, install bind, rollback restore, legacy latch and drain settlement. The managed main opens its dispatcher through the new `dispatch.OpenControlled`, which takes a `LaunchFence`:
  - the fence runs before each new worker launch and each new pool-sweep start;
  - an admitted launch holds F from its final control read through spawn and the saved ledger record;
  - a nil fence is refused before ownership is taken.

  Lock order is U→L→F and O→F. F is never held while taking O or L, and F's holders never wait on a manager, a pulse or a join.
- **Stop acknowledgement.** The old observed-boundary rule was "ACKNOWLEDGED only when no owner and no RUNNING pulse". The fence replaces it:
  - stop is ACKNOWLEDGED once suppression is durable and the owner is absent or is the fenced managed main (owner lock PID equals the PID of a fresh pulse bound to the manifest);
  - a foreign or unknown owner stays PENDING;
  - the new `close` field reports OBSERVED or PENDING separately, so ACK never claims the dispatcher closed.
- **Drain.**
  - Drain applies only from RUNNING.
  - DRAINING is runnable for the main: the dispatcher supervises, heals and accounts while its fence refuses launches.
  - On each poll, the main's `govern` step runs under F. It writes STOPPED at R+1 only when the saved ledger records no worker and no pending pool sweep (`dispatch.Settled`), and it keeps the drain request.
  - Resume wins over a drain. No deadline ends a drain.
- **Legacy stop file.**
  - Presence is observed with an Lstat of the parent before and after (SameFile) plus the ancestor checks, and classified PRESENT, ABSENT or UNKNOWN.
  - Only the main latches RUNNING R to DRAINING R+1, with request `legacy-stop-r<R+1>`. Removing the file keeps the latch.
  - UNKNOWN holds new opens.
  - Resume requires a fresh ABSENT: PRESENT is RESOURCE_COLLISION and UNKNOWN is UNCERTAIN_EFFECT.
- **Resume CAS.** Resume observes pins outside F. Under F it re-reads control, refuses RESOURCE_COLLISION if control changed, checks the legacy file, then writes RUNNING and records the request.

## Limits retained

- There is no separate durable pre-spawn intent record. If the ledger save fails after a spawn, F is released while the worker is known only in memory, so a drain could settle early. Ambiguous-intent reconciliation is not delivered.
- Pins are rechecked once per poll, not per launch.
- Legacy latch ids are recorded only in control, not in the request ledger.
- Helpers, `run-helper` and service logging are deferred, and helpers still refuse UNSUPPORTED.
- Runtime restart debt stays NOT_OBSERVED.
- Journal pruning and type-wide systemd drop-ins remain open.
- Platform qualification (SERVICE500-004/005/010) stays NOT_RUN. The owner accepted it on disposable targets only and deferred it. No real launchd or systemd unit was installed.

## Evidence

The new and updated witnesses:

- `TestSERVICE500_OpenControlledFencesLaunches` and `TestSERVICE500_OpenControlledFencesPoolSweepStart` use the real dispatcher with a fake spawn host.
- `TestSERVICE500_FenceSerializesStopWithAdmittedLaunch`, `TestSERVICE500_DrainSupervisesThenSettlesStopped`, `TestSERVICE500_ResumeDuringDrainWins` and `TestSERVICE500_LegacyStopFileLatchesDrain` use a temp HOME, a fake launchd and an in-process fake controller.

Focused package tests and `go vet` passed for `internal/tasks/service` (also with `-race`), `internal/tasks/dispatch` and `internal/tasks/cli`.
