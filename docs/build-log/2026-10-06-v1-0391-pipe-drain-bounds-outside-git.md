## 2026-10-06 V1-0391: pipe-drain bounds outside Git acquisition detect a held pipe, not a slow reader

Follows V1-0390. `os/exec` starts the `WaitDelay` timer when the child exits, so a one-second bound
fails a successful child with `exec: WaitDelay expired before I/O complete` whenever the goroutine
copying its output is not scheduled within a second on a loaded host.

The bounds named by the ticket (`doccompiler`, `taskman`, `liveverify` mutation and the work
executable binding) were raised to one minute with a held-pipe comment in ca21d046, without tests.
This change adds the missing starved-reader tests and fixes the equivalent production sites found by
a repository-wide `WaitDelay` search:

- `criterionexperiment` test and Tasks runs: one second to one minute, through one `ownGroup` helper.
- `testrunner` phases without a graceful interrupt: one second to one minute. With a graceful
  interrupt the five-second value is also the grace before `os/exec` kills the interrupted leader,
  so it is unchanged.
- `tasks/store` workflow Core context query: one second to one minute.

Each is still a hang detector: cancellation and `groupreap` kill the owned group, which closes the
pipes, so only a descendant that keeps them open waits out the bound. `taskman`, the work executable
binding and `testrunner` extract their containment setup (`containRead`, `workContainVersion`,
`containPhase`) so a test can exercise the production configuration.

Left as found: `tasks/dispatch` reader (its one-second `WaitDelay` is coupled to a one-second
retirement deadline, so raising it alone changes nothing); the attempt runner (its spec states the
one-second limit and production output is an `*os.File`, which needs no copy goroutine); the pool
executor (an `*os.File` pipe, whose own one-second read join fails closed as `UNKNOWN`); the pool
sweep and probe (not group-reaped before `Wait`, so a longer bound would let an in-group descendant
delay a sweep); and sub-second or multi-second bounds in `tools/`, `interop/`, `conformance/`,
`procgroup` and `processidentity`.

Evidence: `TestConfigureProcessSurvivesStarvedReader` (doccompiler, mutate),
`TestContainReadSurvivesStarvedReader`, `TestWorkContainVersionSurvivesStarvedReader`,
`TestOwnGroupSurvivesStarvedReader` and `TestContainPhaseSurvivesStarvedReader` delay the first
output write by three seconds and pass; the doccompiler test fails with the production error when
its bound is set back to one second. The existing cancellation, held-descendant and executable
binding tests of each touched package pass. No spec states a changed bound.
Rollback: revert the change; loaded hosts then fail successful subprocesses again.
