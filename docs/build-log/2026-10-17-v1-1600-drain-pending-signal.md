# V1-1600: drain-pending signal for live workers

Date: 2026-10-17. Base: `072a306f1dbd89c2f29b20a3434a0f25864ebbf7` (origin/main).
Ticket: V1-1600, from GitHub issue 1043. Spec: `docs/specs/corvint-tasks-attempt-runner-v0.md`,
ATR-V0-058..059, accepted under the owner's standing auto-accept of lane requirements
(2026-10-11).

## Decisions

- Mechanism. The signal works like the V1-1549 context notice: a file in the dispatcher state
  directory, named to the worker in its environment. Every worker gets the path at launch, so a
  worker launched before the drain can poll it.
- Source of truth. The service control's `DRAINING` state, read through an optional
  `DrainObserver` extension of `LaunchControl`. Existing controls and test fakes are unchanged.
  The read takes no fence: the control file is replaced atomically, and the signal is advisory.
- Claim. Issue 1043's criterion 2 allows a refusal or a notice. A notice was chosen. During a
  drain the fence admits no new worker, so a claim comes from a worker already running.
  Refusing it would change the store's admission semantics without stopping the work in flight.
  The warning rides on every claim result, including refusals, replays and a detached claim's
  kept result.

## Evidence

- `TestATRV0058_DrainSignalReachesLiveWorkerAndClears` covers:
  - a worker launched before the drain records `CORVINT_DISPATCH_DRAIN_FILE`, waits for the file,
    and reads it once the control reports the drain
  - one `PENDING` alert over two draining ticks
  - a failed control read keeps the file
  - lifting removes it with one `LIFTED` alert
- `TestATRV0058_UncontrolledDispatcherClearsLeftoverSignal` and
  `TestATRV0058_LaunchControlReportsDrain` (the service's install, drain and resume path).
- `TestATRV0059_ClaimCarriesDrainNotice` covers:
  - no notice without the signal
  - a notice on replayed, refused and detached claims while it is present
  - none after it is removed
- Linux: NOT_RUN.
