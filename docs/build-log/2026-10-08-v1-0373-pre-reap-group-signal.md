# V1-0373 / V1-0652: signal the process group before the leader is reaped

Date: 2026-10-08
Tickets: V1-0373, V1-0652
Requirement: PGO-V0-008 (proposed; acceptance is human-owned)

## Intent

Every listed runner signals its process group only while the exited leader is
held unreaped, where waitid is available, keeping each runner's timeout,
cancellation and error classification. A numeric group ID confers kill
authority only while the leader's PID is pinned; after the reap an empty group's
ID can be reused by an unrelated process that becomes a group leader.

## V1-0652 finding: confirmed by code-level argument

Base f33ea8ef. Each path below sent `kill(-pid, SIGKILL)` when the leader could
already be reaped:

1. `groupreap.wait`: when `leaderUnreaped` failed (ECHILD after any foreign
   `wait4(-1)` reaper, or another waitid error), it called `command.Wait()` and then
   signalled the group, discarding the signal error. `RunContainedRetiring` had
   the same fallback.
2. Group-kill `exec.Cmd.Cancel` functions (doccompiler, liveverify/mutate,
   taskman, criterionexperiment, gokernel, contextindex, worksource). os/exec's
   `watchCtx` selects between the wait result and `ctx.Done()`, so `Cancel` can
   run after `Process.Wait` has reaped the leader; the group kill then targets
   a released ID.
3. `gitrun.killGroup` and the releasegate `ctx.Done` branch signalled the group
   while `groupreap.Wait` in another goroutine could already have reaped.
4. contextindex `terminateProcessGroup` and worksource `cleanupGitProcess` ran
   after `command.Wait()` by design (post-reap sweep, needed so that a pipe
   holder is reported as incomplete capture before it is killed).

No deterministic PID-reuse reproduction was built (it needs PID wrap-around);
the regression tests below pin the corrected ordering instead.

Darwin finding: `waitid(P_PID, pid, WEXITED|WNOWAIT)` also returns for a
stopped or continued child (golang/go#19314). A scratch probe on Darwin 25.6.0
observed `si_signo=20`, `si_code=5` (CLD_STOPPED) for a SIGSTOPped child, so
the old `leaderUnreaped` could report an exit while the leader was alive and the
group would be swept early. `TestLeaderUnreapedIgnoresStopAndContinue` fails on
the unfixed classification and passes now.

## Change

- `internal/groupreap`: `leaderUnreaped` ignores `CLD_STOPPED`/`CLD_TRAPPED`/
  `CLD_CONTINUED` reports and polls (10 ms) until a real exit. A failed exit
  observation on a waitid platform reaps without any group signal (Wait, Run,
  RunContainedRetiring). New `Contain` (Setpgid plus a `Cancel` that calls
  `Stop`), `Stop` (leader-only `os.Process.Kill` where waitid is available, so a
  reaped leader is refused; group signal elsewhere) and `Drain` (owns non-file
  output pipes; sweeps the group after the drain or its delay and before the
  reap; returns `exec.ErrWaitDelay` for a pipe holder past the delay; context
  cancellation signals a running group at once). `observeExit` is a test seam.
- Callers: doccompiler, liveverify/mutate, taskman, criterionexperiment and
  gokernel use `groupreap.Contain` with their existing `WaitDelay`; contextindex
  and worksource use `Contain` plus `Drain` with their existing delays and drop
  the post-reap sweep; releasegate and gitrun `killGroup` use `groupreap.Stop`;
  the gitrun_unix.go comment now describes Stop.

One shared helper replaces seven Cancel copies and two post-reap sweeps.

`internal/contextindex` sources are pinned analyzer inputs, so the analyzer
schema moves from `corvint-analyzer/112` to `/113` with a new audit digest
(TestAnalyzerSchemaInputs). The change is process handling only; no pack
facts or encoding change.

Independent review (Codex) found that Drain's cancellation branch signalled
the group before the exit observation, so cancellation racing a foreign reap
could still send a post-reap group signal. Drain now stops only the leader on
cancellation and sweeps the group after the observation succeeds;
TestDrainCancellationAfterForeignReapSendsNoGroupSignal fails on the earlier
code and passes now.

## Limits (recorded, not fixed)

- Platforms without waitid (aix, dragonfly, freebsd, netbsd, openbsd,
  solaris; not 1.0 platforms, PRS-V1-004) keep the legacy post-reap group signal.
- Darwin `os.Process` has no pidfd: a leader-only kill that races the reap has
  the same single-PID window as the standard library's default cancellation.
- Pre-reap signal errors stay non-fatal so no previously working command is
  refused; on Linux an EPERM would mean a surviving member owned by another
  user, which these runners never create.
- The Owner's exit wait (`owner_waitid.go`) uses the same `leaderUnreaped`,
  so the Darwin stop fix applies there too. A leader that stays stopped is now
  waited for on Darwin, as it already was on Linux, instead of being swept
  while alive; context cancellation or the runner's timeout still kills it.
- A failed exit observation now leaves cleanup unproved instead of sending an
  unsafe signal; RunContained already reported that case as incomplete.

## Verification

- `GOMAXPROCS=3 go test -p 1 -count=1 -timeout 30m` on Darwin arm64 for
  internal/groupreap, contextindex, worksource, doccompiler, liveverify/mutate,
  taskman, criterionexperiment, gokernel, releasegate, cem/gitrun and
  testrunner (results in the lane report).
- New groupreap tests: TestDrainSignalsGroupBeforeLeaderIsReaped,
  TestDrainCapturesOutputBeforeSweep,
  TestDrainPipeHolderIsIncompleteCaptureAndKilled,
  TestDrainCancellationSignalsGroupPromptly,
  TestFailedExitObservationSendsNoGroupSignal (Wait, Drain, RunContained),
  TestWaitAfterForeignReapSendsNoGroupSignal, TestStopAfterReapSendsNoGroupSignal,
  TestContainCancellationSweepsGroupBeforeReap,
  TestLeaderUnreapedIgnoresStopAndContinue,
  TestDrainCancellationAfterForeignReapSendsNoGroupSignal.
- `go vet` native and with GOOS=linux and GOOS=windows for touched packages;
  GOOS=freebsd builds groupreap, contextindex and worksource (the freebsd vet of
  the wider set fails in internal/cem/publish, pre-existing, V1-0393).

## NOT_RUN

- Linux runtime execution of the new tests (Linux vet only on this host).
- Corvint dogfood (`dogfood-change`/CEM): degraded this session
  (`corvint-event-rejected:dogfood-event-deadline`).
- `make gate` and the full `go test ./...` (lane policy).

## Rollback

Revert the V1-0373 commit. It restores the group-kill Cancel functions, the
post-reap sweeps in contextindex and worksource, and the legacy fallback order;
no stored state or wire format changes.
