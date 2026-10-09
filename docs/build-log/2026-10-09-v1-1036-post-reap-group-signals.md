# V1-1036: remaining spawners signal before the reap or only observe after it

Date: 2026-10-09
Ticket: V1-1036
Base: b2a0b8c69cfa8687ed23ecdeedc1e490a39b1089
Requirements: none added or changed. PGO-V0-008 (proposed) already says that no
runner sends a numeric group signal after the leader may have been reaped, and
PGO-V0-001 says that signal-0 probes grant no signal authority. This entry widens
the PGO-V0-008 traceability to the sites changed here and lists the sites that
still signal after the reap.

## Contract applied

A group signal is sent only while the exited leader is held unreaped
(`waitid(WEXITED|WNOWAIT)`). In that state its PID, and so its group ID, cannot
be reused. After the reap, a site may send only signal 0. A reused ID can then
make a gone group look live, so the site must treat "live" as unclean or
refused, and never as clean. No new abstraction: every change reuses
`internal/groupreap` (`Contain`, `Stop`, `Wait`, `Drain`).

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
- `TestV11036PostReapGroupCheckOnlyObservesAndFailsClosed`:
  - Pins the observation-only, fail-closed reason. There is no behaviour fix
    to revert.

Existing focused tests pass:

- store: `TestPoolProcessDescendants`, `TestPSR*`, `TestCALV0188`
- dispatch: `TestCALV0056*`, `TestSERVICE500*`, `TestCALV0187*`, `TestCALV0144*`
- releasegate: `TestRunGit*`

`go vet` passes on Darwin and with GOOS=linux. The GOOS=windows build passes
for the three packages.

## Audit outcome (no edits)

- `cmd/corvint/work_runner.go` (ticket site, **not changed: needs a design
  decision**):
  - After the reap, `workTerminateGroup` SIGKILLs the group and uses that
    signal's success as the residue detector that WQO-V0-032 requires.
  - Moving the kill before the reap would kill the residue before it can be
    detected.
  - Before the reap on Linux, neither kill nor signal 0 can tell a zombie-only
    group from residue without groupreap's unexported `/proc` quiet proof.
    `groupreap.Owner` kills unconditionally and reports no residue.
  - Options:
    - (a) groupreap exports a pre-reap residue observation, using the
      platform quiet proof, in a Wait/Drain variant.
    - (b) Adopt `Owner` and redefine residue as HOLD or a live probe.
    - (c) Keep post-reap detection and document its check-then-act window.
  - Owner choice.
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
  - Not changed: the Darwin file belongs to lane V1-1046, and changing the
    Linux path needs the same residue decision as the work runner.
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

Revert the V1-1036 commit. Each runner returns to its previous post-reap group
signal, and the seams and tests are removed.
