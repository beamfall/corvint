# 2026-10-07: dispatcher integration of detached attempt runs (V1-0936, V1-0858)

## Intent

Issue 651 asks that an agent session no longer has to stay open to poll a long detached command
(`corvint-tasks run --detach`, ATR-V0-008..015). Under the continuous dispatcher, a worker that
started a detached run and then ended had its whole process tree stopped, including the run's
supervisor, and its attempt handed off at once. The amendment CAL-V0-145..154 in
`docs/specs/corvint-tasks-agent-leases-v0.md` is proposed, pending owner acceptance. ATR-V0-016..020
were not needed: the attempt runner's record and its supervisor are unchanged.

## Decisions (delegated by the owner to the orchestrator)

1. **Identity-verified exclusion (CAL-V0-145, 151).** The stopped tree spares a process only when it
   is the recorded supervisor of a `RUNNING` run of an attempt the worker held. The supervisor's PID
   and start identity must match the record. It must lead its own session, and that session must be
   neither the worker leader's nor the dispatcher's. A missing, forged or contradicted record keeps
   today's stop.
2. **Deferred hand-off (CAL-V0-146).** With `heal.handoff` on, the hand-off of such an attempt waits.
   The bound is the run's launch time plus its timeout plus a 3-minute settle. The hand-off proceeds
   at `FINISHED`, at a lost supervisor, or at a missing or replaced record.
3. **Relaunch once (CAL-V0-149).** When the run finishes and no worker runs on the ticket, exactly one
   session of the same role is launched, through the normal roster, caps and admission. The ledger
   encoding is unchanged (lane #652 owns it). Dedupe uses a dispatcher-owned marker,
   `<state>/<program>/detached/<runId>.json` (`taskman-dispatch-detached-run/0`). The marker has
   these phases: `DEFERRED`, then `HANDED_OFF`, then `RELAUNCH`, then `LAUNCHED`. It is retired when
   the successor leaves the ledger or the relaunch becomes moot. The successor is named before the
   spawn, so a crash can lose a relaunch but cannot repeat one.
4. **Read-only status (CAL-V0-150).** `dispatch status` gains a `detachedRuns` array, present only
   while markers exist. It reads the markers and records, bounded, and writes nothing.
5. **Compact `run --attach --wait` output (CAL-V0-154).** Recorded as a limit; not implemented.

## Wire, configuration and compatibility

- New placeholder `{detachedRun}`: one line naming the run's outcome. It renders only in a role
  prompt; host argv, env and activityPaths refuse it. A host gets the same facts from
  `CORVINT_DISPATCH_RUN_ID`, `_ATTEMPT`, `_EXIT`, `_RESULT` and `_OUTPUT`.
- New details: `detachedRun` and `handoff` on `handoff`, `finished` and `launched` events; `runExit`
  on `launched`; `runState` on `alert`. The event kinds (CAL-V0-058), the ledger, requests,
  receipts and native state are unchanged.
- The CAL-V0-131 live format set gains `taskman-dispatch-detached-run/0`. A marker written by
  another version is refused as `UNSUPPORTED_VERSION` and kept unread, and its run is handed off
  without a deferral. Other bad markers are reported and removed. Because the set changed,
  CAL-V0-130 refuses in-place replacement by an earlier build, which therefore needs a drain.
- The CAL-V0-139 idle gate does not arm while markers are tracked.

## Limits

- Planting a record needs write access to the run directory. That access already allows a
  legitimate `run --detach`, so a plant grants nothing more.
- `STARTING` runs are neither exempt nor deferred for.
- Deferral and relaunch need `heal.handoff`; exclusion always applies.
- Roles with a `lane` configuration, and lane sessions, keep today's hand-off.
- Status lists only runs that this dispatcher tracks.
- An ambiguous spawn retires the marker without a relaunch, and `launch-failed` reports it.
- Each tick reads the run directory of every held live attempt: at most 65 entries and one 64 KiB
  record each.
- Windows and other platforms keep today's behaviour; they are checked only by `go vet`.

## Evidence

Tests are in `internal/tasks/dispatch` (Darwin and Linux build tag) and in `internal/tasks/cli`.
See the traceability rows for CAL-V0-145..154. The results of the run checks are recorded in
the lane hand-off.

NOT_RUN:
- Linux execution.
- Live dispatcher qualification with a real host session.
- Owner acceptance.
- `make gate`, by lane rule.

## Independent review (Codex, read-only)

Round 1 reported three P1 findings. Each was checked against the code before any change:

- **Fixed.** A forged record that names a session leader enclosing the dispatcher could spare the
  worker itself. `exemption.has` followed parents past the worker leader into the dispatcher's
  session. Reproduced: `TestCALV0151_EnclosingSessionLeaderSparesNoWorker` fails on the round-1
  code. The parent walk now stops at the worker leader and at the dispatcher, and a supervisor that
  is an ancestor of the dispatcher is ignored.
- **Accepted and documented.** A crash between the spawn and the ledger write leaves an untracked
  successor. This window exists for every launch before this change. The marker still prevents the
  outcome-bearing relaunch from being repeated. The window is recorded in the failure modes.
- **Accepted and documented.** The exemption is decided once per tick. A run that becomes
  `RUNNING` during a stop within that tick is stopped like a `STARTING` run. Re-reading run records
  every 100 ms of a kill loop is not worth that window.

Round 2 reported one P2 finding, which was fixed. An infrastructure retry of the same role did
not carry a finished run's outcome, and its session then made the relaunch moot, which lost the
outcome. Reproduced: `TestCALV0149_InfraRetryCarriesTheOutcome` fails on the round-2 code. The
retry is now the relaunch session. If its marker cannot be saved, the retry returns to `WAITING`
and keeps its identity.

## Out-of-scope finding

`TestCALV0127_ReloadRemovedRoleKeepsWorkers` and `TestCALV0127_ReloadKeepsLaunchDeadlines`
(`internal/tasks/dispatch`) leave orphan `sleep 300` processes after they pass. This was observed on
base 79536edd behaviour, during this lane's package runs. `TestCALV0143_WorkerLogsAreCappedWhileTheWorkerRuns`
leaves its `host.sh` worker running after the test. One full `internal/tasks/cli` run failed
`TestATRV0008_DetachedRunSurvivesItsLauncher` with "unassigned stage slot". The test passed three
times in isolation, and this change does not touch it, so the failure is a suspected flake. None of
these were filed: the lane rules forbid task-store writes.
