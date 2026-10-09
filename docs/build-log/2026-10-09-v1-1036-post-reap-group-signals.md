# V1-1036: remaining spawners signal before the reap or only observe after it

Date: 2026-10-09
Ticket: V1-1036
Base: b2a0b8c69cfa8687ed23ecdeedc1e490a39b1089
Requirements: no new IDs. PGO-V0-008 (proposed) already says that no runner
sends a numeric group signal after the leader may have been reaped, and
PGO-V0-001 says that signal-0 probes grant no signal authority. PGO-V0-008's
proposed text now names `groupreap.WaitResidue`. This entry widens the
PGO-V0-008 traceability to the sites changed here and lists the sites that
still signal after the reap. WQO-V0-032's residue result is unchanged.

## Contract applied

A group signal is sent only while the exited leader is held unreaped
(`waitid(WEXITED|WNOWAIT)`). In that state its PID, and so its group ID, cannot
be reused. After the reap, a site may send only signal 0. A reused ID can then
make a gone group look live, so the site must treat "live" as unclean or
refused, and never as clean. Every change reuses `internal/groupreap` (`Contain`, `Stop`, `Wait`, `Drain`).
The one new export is `WaitResidue`, chosen by the coordinator as option (a)
for the work runner.

## Changed sites

- `internal/tasks/store/pool_process_unix.go`, both pool runners:
  - Containment now uses `groupreap.Contain`.
  - Timeout, context and lease kills use `groupreap.Stop` (leader only).
  - The reap uses `groupreap.Wait` for the uncaptured runner and
    `groupreap.Drain` for the captured runner. Drain keeps a pipe holder as
    `exec.ErrWaitDelay`, so a held pipe is still classed UNKNOWN
    (`TestPSRCapturedDescendants`).
  - Removed: the post-reap `SIGKILL` (uncaptured runner) and the post-reap
    `killGate` (captured runner). They were check-then-act after `ps` and
    signal-0 probes.
  - The probes stay as observations only, through the `poolGroupSignal` seam.
    A live-looking group still makes the run unclean.
- `internal/tasks/dispatch/process_unix.go` `launch`: the SIGKILL to an
  unidentified worker's group now runs before the `cmd.Wait` goroutine starts.
  Before, the kill raced a Wait that might already have reaped the leader. It
  now goes through the `launchGroupKill` seam.
- `cmd/corvint/work_runner.go` (decision: option (a)):
  - Residue (WQO-V0-032) is decided by `groupreap.WaitResidue` while the
    exited leader is held unreaped. That function runs the owner's existing
    quiet proof (Linux `/proc` `procGroupQuiet`, PGO-V0-006; Darwin signal 0,
    where EPERM means zombie-only). It then SIGKILLs the group and reaps.
    A failed proof or exit observation reports residue.
  - Before, `workTerminateGroup` sent a group SIGKILL after the reap and
    treated its success as residue. Before, a deferred `workKillGroup` also
    signalled after the reap. Both are removed.
  - Overrun and cancellation now use `groupreap.Stop` (leader only) through
    `workStop`. The group is swept when the leader's exit is observed.
  - Rejected options:
    - (b) changes WQO-V0-032 semantics.
    - (c) keeps a post-reap SIGKILL that can reach a reused group, which is
      unsafe and not fail-closed.
  - `workKillGroup` keeps its behaviour for the binding/materialization
    cancellation callers, which this ticket does not cover.
  - Timing differs at the edges. Old: a member counted only if it still
    existed after the reap. New: a member counts if it was live when the
    leader's exit was observed. A plain background child (the WQO-V0-034
    fixture) is residue under both.
- `internal/releasegate/git_unix.go` `processGroupGone`: documented and tested
  reason, with no change in behaviour. It runs after `groupreap.Wait` has swept
  the group before the reap. It sends signal 0 only, through the `groupProbe`
  seam. A reused ID can only make `runGitAt` refuse ("did not disappear"),
  never accept.

## Evidence (Darwin arm64, Darwin 25.6.0)

Each new test fails when only its fix is reverted and passes with the change:

- `TestV11036PoolPostReapObservationSendsNoGroupKill` (captured=false/true):
  - With the post-reap group SIGKILL restored in both runners, it fails with
    "post-reap group kill sent to [pid]".
  - It passes with the change.
- `TestV11036LaunchKillsUnidentifiedGroupBeforeReap`:
  - With the old order restored (Wait goroutine started first, then the kill),
    it fails with "leader was reaped before the group kill returned: no such
    process".
  - It passes with the change.
- `TestV11036WorkRunnerResidueIsObservedBeforeReap` inverts the pre-reap
  observation:
  - With the base work runner restored (only the seam variable added), it
    fails both ways:
    - a live member is reported as residue ("post-reap signal decided
      residue").
    - a quiet group passes ("observed residue not reported").
  - It passes with the change.
- `TestWaitResidueObservesAndSignalsBeforeReap` (live member, quiet group,
  unproved) and `TestWaitResidueAfterFailedObservationIsResidueWithoutSignal`
  check two things:
  - the proof runs with the leader unreaped and every group SIGKILL is sent
    before the reap.
  - an unproved or failed observation reports residue.
- `TestV11036PostReapGroupCheckOnlyObservesAndFailsClosed`:
  - Pins the observation-only, fail-closed reason. There is no behaviour fix
    to revert.

Existing focused tests pass:

- the whole `internal/groupreap` package
- `cmd/corvint` `-run '^(TestWork|TestV11036)'`, which includes
  WQO-V0-034 `TestWorkRunnerGroupDescendantCleanup` and
  `TestWorkRunnerEscapedPipeHolder`

- store: `TestPoolProcessDescendants`, `TestPSR*`, `TestCALV0188`
- dispatch: `TestCALV0056*`, `TestSERVICE500*`, `TestCALV0187*`, `TestCALV0144*`
- releasegate: `TestRunGit*`

`go vet` passes on Darwin and with GOOS=linux for all five packages. The
GOOS=windows build of `cmd/corvint` and `internal/...` passes.

## Audit outcome (no edits)

- `internal/tasks/store/gate_process_unix.go` `killGate`, used by
  `lease_gate.go` `execute`:
  - The timeout timer, `context.AfterFunc` and output-overflow kills race
    `cmd.Wait`. A kill that fires as Wait reaps can be sent after the reap.
  - Not changed (audit-only site). Candidate follow-up: adopt
    `groupreap.Contain`/`Stop`/`Drain` as the pool runners now do.
- `internal/dashboard/repository`:
  - The Darwin path (`process_wait_darwin.go`) detects and stops the group
    while the leader is unreaped. It is compliant.
  - The Linux path (`process_wait_linux.go`) reaps through `command.Wait`, then
    runs `quiescent` (signal 0), then `terminate`/`force` (SIGTERM/SIGKILL to
    the numeric group). That is a post-reap check-then-act.
  - The `force` call on a start or attach failure in `process.go` has no PID
    and sends nothing.
  - Not changed: the Darwin file belongs to lane V1-1046. The Linux path
    could adopt `WaitResidue` as the work runner did.
- `internal/opencodequalification/terminal_posix.go`:
  - `go child.Wait()` reaps concurrently.
  - The deferred group SIGTERM and the final group SIGKILL after the wait
    returns are sent after the reap.
  - Qualification harness only. Not changed.
- `internal/testsupport/pipedrain_unix.go` (owned by lane V1-1046):
  - A test `t.Cleanup` sends `kill(-leader, SIGKILL)` after `command.Wait`
    has reaped.
  - Test-only best-effort cleanup. Not changed.

## Limits

- Linux runtime: NOT_RUN on this host (Linux vet only).
- Platforms without waitid keep the legacy order, as PGO-V0-008 states.
- On Darwin, `os.Process` has no pidfd, so `Stop` keeps the single-PID window
  of the standard library.
- `make gate`: NOT_RUN (lane policy).
- Dogfood CEM: NOT_RUN in this lane.

## Rollback

Revert the V1-1036 commits. Each runner returns to its previous post-reap
group signal, `WaitResidue` and the seams are removed, and the work runner
again detects residue by a post-reap SIGKILL.
