# V1-1047: four more post-reap process-group signal sites

Date: 2026-10-09. Base 02e84575. Host Darwin arm64; Linux arm64 checks ran in a local
`golang:1.27.1` / `alpine:3` container (no network).

## Decision

On Darwin and Linux, every group signal at these four sites is now sent while the leader is still
this process's unreaped child, or not sent at all. A failed exit observation, such as ECHILD after
another reaper collected the leader, unpins the PID, so no signal follows it. On Unix platforms
without waitid, the lease gate sends no group signal (see Limits). After a reap the numeric PID, and with it the group ID, can be
reused. A later signal could then reach an unrelated process. This follows PGO-V0-008: signal
before the reap. Each site is audited in the V1-1036 build log
(`2026-10-09-v1-1036-post-reap-group-signals.md`).

| Site | Base defect (confirmed) | Change |
| --- | --- | --- |
| `internal/tasks/store/lease_gate.go` `execute` | The timeout timer, `context.AfterFunc` and output-overflow callbacks called `killGate` (`kill(-pid, SIGKILL)`) concurrently with `cmd.Wait`. A pipe writer that outlives the leader overflows only after the reap, so its kill always follows the reap. | On Darwin and Linux (`gate_reap_waitid.go`), early kills use `stopGate` = `groupreap.Stop` (leader only, through `os.Process`, which refuses a reaped leader), and `reapGate` = `groupreap.Wait`, which SIGKILLs the group while the exited leader is unreaped. On other Unix platforms (`gate_reap_nowaitid.go`), `stopGate` kills the leader only and `reapGate` is plain `cmd.Wait`, so no group signal is sent. Non-unix keeps `Process.Kill`. `killGate`/`containGate` stay unchanged for the pool runners, which V1-1036 moves to groupreap. |
| `internal/dashboard/repository/process_wait_linux.go` | `command.Wait` reaped first. Then the residue check, `stopAndProve` (group SIGTERM/SIGKILL) and the timeout cleanup raced or followed the reap. | Mirrors the Darwin path. A `waitid(WEXITED\|WNOWAIT)` observation, which polls past si_code 4..6, holds the leader unreaped. Residue and the stop loop run before `reapAllowed` is closed. Signals go out only while the observation is pending or has succeeded (`pinned`). A failed observation sends nothing and leaves cleanup unproved. Linux signal 0 cannot tell a zombie from a live member, so residue and quiescence are read from `/proc` (pgrp match and a state other than Z/X, or a zombie with more than one task). A `/proc` read failure counts as residue. In the stop loop it still SIGKILLs the pinned group once, so a TERM-ignoring leader cannot block the wait, and it returns unproved cleanup instead of entering an unbounded kill loop. The post-reap `quiescent()` check stays as a signal-0 observation that can only fail closed. |
| `internal/opencodequalification/terminal_posix.go` | `go child.Wait()` reaped at once. The deferred group SIGTERM, the final group SIGKILL and a resize SIGWINCH could follow the reap, and on a normal host exit always did. | The waiter observes the exit unreaped (`exited`). It reaps only after the deferred cleanup closes `reapAllowed`. All signals go through `terminalSignal`, and none is sent after a failed observation. |
| `internal/testsupport/pipedrain_unix.go` `checkHeldPipe` | Test cleanup sent `kill(-leader)` after `command.Wait` reaped. It also sent `kill(descendant)` to a reparented non-child. | The pipe holder is `(read held <&3) &` on an inherited release pipe. Cleanup closes the write end, the holder reads EOF and exits, and no signal is sent. V1-1046 (b056326d/adb60e14) changed only `exitedUnreaped` in this file, so the hunks do not overlap. |

## Evidence

- `TestV11047LeaseGateKillsGroupBeforeTheReap` (store) fails on the base sources: `class=OUTPUT_LIMIT
  overflow=true`, from the post-reap overflow kill. With the change it passes on Darwin and in the Linux container.
- `TestV11047TerminalSignalsOnlyAnUnreapedChild` (opencodequalification): a hook checks each signal
  with `waitid(WNOHANG|WNOWAIT)`; ECHILD means the child was already reaped. On the base order (hook only) it
  fails with `signals [terminated killed] were sent after the child was reaped`. With the change it passes on
  Darwin and Linux.
- `TestResidueCleanupSignalsOnlyWhileLeaderUnreaped` (dashboard, darwin||linux) uses the same check
  on `terminate`/`force`. With the base `process_wait_linux.go` it fails in the Linux container: `group signals
  [terminated] were sent after the leader was reaped`. With the change it passes. The whole
  `internal/dashboard/repository` package passes on Linux arm64 (container) and Darwin. On Darwin the
  test passes on base too, because Darwin was already compliant.
- pipedrain: all six `CheckPipeDrainBound` callers pass on Darwin (cmd/corvint, testrunner,
  criterionexperiment, doccompiler, taskman, liveverify/mutate). No holder process remained afterwards
  (`ps`). There is no deterministic regression test: the cleanup now sends no signal at all, and an
  observer test would need to signal a non-child. This is recorded as a structural limit.
- Review follow-up (Codex FAIL on 9af04ded):
  - `TestV11047FailedExitObservationSendsNoGroupSignal` (dashboard, Linux) and
    `TestV11047TerminalFailedObservationSendsNoSignal` (opencodequalification) inject ECHILD after the real exit.
    With the 9af04ded behaviour they fail with `[terminated]` (Linux container) and `[terminated killed]` (Darwin).
  - `TestV11047UnreadableProcessTableStillKillsPinnedGroup` (dashboard, Linux) injects a `/proc` failure while
    cancelling a TERM-ignoring leader. With the 9af04ded early return it fails after 20 s:
    `wait blocked on a TERM-ignoring leader`.
  - All of these pass with the follow-up, both in the Linux container (with `-race` for dashboard and opencode)
    and on Darwin (with `-race`).
- `go vet` passes for darwin, linux, windows and freebsd (amd64) on all four packages. It also passes for
  openbsd, netbsd and solaris on store.
- PGO-V0-008 trace mentions (for integration): lease gate (`TestV11047LeaseGateKillsGroupBeforeTheReap`); dashboard
  Linux wait (`TestResidueCleanupSignalsOnlyWhileLeaderUnreaped`, `TestV11047FailedExitObservationSendsNoGroupSignal`,
  `TestV11047UnreadableProcessTableStillKillsPinnedGroup`); terminal harness
  (`TestV11047TerminalSignalsOnlyAnUnreapedChild`, `TestV11047TerminalFailedObservationSendsNoSignal`).

## Limits

- Linux amd64 runtime: NOT_RUN. Linux runtime evidence comes from an arm64 container only.
- The lease gate's early `Stop` keeps PGO-V0-008's Darwin limit. `os.Process` has no pidfd there, so a
  leader-only kill that races the reap has the standard library's single-PID window.
- Unix without waitid (FreeBSD, OpenBSD, NetBSD, DragonFly, Solaris, AIX): the lease gate sends no group
  signal, neither early nor at the reap. Descendants left in the gate's group are therefore not killed
  there; `WaitDelay` bounds how long they can hold the output pipe. These platforms are not 1.0 targets
  (groupreap's V1-0652 limitation), and `groupreap.Wait` itself still signals after the reap on them. That
  is left unchanged here because V1-1036 owns `internal/groupreap`. The dashboard and terminal sites are
  darwin/linux-only.
- A pending observation counts as pinned: the leader has not exited, so nobody can have reaped it. Between
  that check and the signal there is still the inherent single-check window.
- The Darwin dashboard wait (`process_wait_darwin.go`, V1-1046's file) still calls its stop loop after a
  failed observation. That is the same P1 shape and is left to its owner.
- The PGO-V0-008 trace row (`docs/specs/process-group-owner-v0.md`) is not edited here because V1-1036
  edits that row; the mentions above are for integration.

Rollback: revert this commit. No persisted state or wire contract changes.
