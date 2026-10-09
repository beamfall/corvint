# V1-1040: adapter exit kills procgroup groups (AHI-049, proposed)

Date: 2026-10-09. Ticket: V1-1040. Lane base: `b2a0b8c69cfa8687ed23ecdeedc1e490a39b1089`;
branch starts at the reviewed V1-1036 head `dad48885`. Requirement: `AHI-049` (proposed) in
`docs/specs/agent-harness-integration-v0.md`. It extends `AHI-048` and is not accepted.

## Decision

`AHI-048` records a group in the `internal/groupreap` live registry only when its spawner starts
the child through `StartLive`, `Drain` or an `Owner`. `internal/procgroup` `Run` started its own
Setpgid leader with plain `command.Start`, so `exitProcess` did not kill it. `captureSourceHandoff`
reaches `Run` through `sourceGit`, and the `AHI-017` watchdog can abandon that path.

`Run` now records the group through `groupreap.StartLive`. It releases the record with the new
`groupreap.ReleaseLive` after `cleanupProcessGroupBeforeReap` and before `close(reapAllowed)`, while
the exited leader is still unreaped. The `AHI-048` invariant holds: the exit kill sends one pre-reap
group signal, and no numeric group signal follows the reap.

A `JoinAncestorProcessGroup` run leads no group, and `StartLive` does not record it. A start that
loses the race to the exit kill fails with `groupreap.ErrExiting`, which is reported as a process
start failure. The post-reap signal-0 quiescence probe in `proveProcessGroupQuiescent` is
unchanged after an observed exit. It is a probe, not a kill.

Review follow-up (independent review of `dad48885..cc2378cb`, P2). The first version released the
record only after `BeforeStop` and the group cleanup, even when `waitid` had already failed (for
example with ECHILD after another reaper). During that window a concurrent `KillLive` could SIGKILL
a numeric group ID whose leader might already be reaped and reused. `Run` now releases the record
as soon as an exit observation fails:

- The observing goroutine releases it at the failure.
- `run` releases it again when it receives the failure, before `BeforeStop`.

After a failed observation, `run` sends no group signal of its own. It skips
`terminateProcessGroup`, `cleanupProcessGroupBeforeReap` and the post-reap quiescence probe, and
it reports cleanup as unproved (`process-cleanup-failed`, `OwnedProcessGroupCleanup` false). This
matches the groupreap rule (V1-0652) that a failed observation confers no group-signal authority.

After `BeforeStop`, `run` also polls the exit observation without blocking. As a result, an exit
or failure that lands during `BeforeStop` decides the signal.

## Inventory

Method:

- I built the `golang.org/x/tools` v0.49.0 `cmd/callgraph` offline and ran `-algo=vta` over
  `./cmd/corvint`, which produced 144288 edges.
- I ran a BFS from the three points where an adapter abandons running work and then exits:
  - `recoveredHostAdapterOutput` (the AHI-017 watchdog);
  - `dogfoodEvent` (the LCP-V0-008 deadline);
  - `piAdapterResult`.
- I narrowed the string-dispatched `runContext` edges by hand:
  - `invokeLegacyClaudeEvent` runs `harness event`.
  - `captureSourceHandoff` and `piToolResult` run `context --task`.
- The native hook's 2 s watchdog is a host SIGKILL, so it is not an abandon point.

| Spawner | file:line | Reachable from an exiting adapter? | Registered? |
|---|---|---|---|
| `procgroup.configureProcessCommand` | `internal/procgroup/process_posix.go:33` | yes (`captureSourceHandoff` → `sourceGit` → `procgroup.Run`) | yes, by this change (`process.go:264` start, `:393` pre-reap release, `:311`/`:350` release on a failed observation) |
| `gokernel.configureProcess` | `internal/gokernel/process_unix.go:12` | yes | yes (`StartLive`, `repository.go:137`) |
| `contextindex.configureProcess` | `internal/contextindex/process_unix.go:12` | yes | yes (`Drain`, `git_execution.go:62`) |
| `worksource.setupGitProcess` | `internal/worksource/process_unix.go:13` | yes | yes (`Drain`, `git.go:217`) |
| `cem/gitrun.containChild` | `internal/cem/gitrun/gitrun_unix.go:14` | yes | yes (`StartLive`, `gitrun.go:269`, `session.go:132`) |
| `gitstatus.runLookup` (`xcrun --find git`) | `internal/gitstatus/executable.go:93` | yes | yes (`StartLive`, `:98`); stays in the worker group in an owned worker |
| `groupreap.StartWith` / `containLeader` | `internal/groupreap/owner.go:177`, `owner_waitid.go:19` | yes (`dogfoodflow.RunAggregateWorker`) | yes (`liveGroups.start`, `owner.go:187`) |
| `cmd/corvint.workContain` | `cmd/corvint/work_process_unix.go:14` | no | no |
| `doccompiler.configureProcess` (unregistered `groupreap.Run`) | `internal/doccompiler/process_unix.go:12` | no | no |
| `taskman.containRead` | `internal/taskman/process_unix.go:34` | no | no |
| `liveverify/mutate.configureProcess` | `internal/liveverify/mutate/process_unix.go:14` | no | no |
| `groupreap.RunContainedRetiring` | `internal/groupreap/escape.go:118` | no | no |

Some spawners are not linked into `cmd/corvint`, so they cannot run under `exitProcess`:

- `testrunner.containPhase`
- the `tasks/store` gate and pool runners
- `criterionexperiment.ownGroup`
- `releasegate.runGitProcess`
- `dashboard/repository.prepareContainment`
- `liveverify/gorunner.configureCommand`
- the Setsid launchers of `tasks/dispatch`, `tasks/supervisor`, `tasks/cli` attempts and
  `opencodequalification`

Other binaries never call `exitProcess`:

- `cmd/corvint-go-test-provider`
- `tools/*`
- `conformance/*`
- `interop/cem01-go`

The xcrun owned-worker criterion was already met before this change, by `68147c39`, an ancestor of
the base. This change keeps it, and `TestAHI048LookupKeepsOwnedWorkerGroup` covers it.

## Evidence

All runs were on Darwin with `GOTOOLCHAIN=local GOMAXPROCS=3 go test -p 1 -count=1 -race`.

- **Fails on base.** `TestAHI049ExitProcessKillsProcgroupGroup` was run at `dad48885` with only the
  new test file copied in. Result: `FAIL (61.07s) ... child <pid> outlived exitProcess`. No helper
  processes were left over.
- **Passes after.** With this change, `TestAHI049ExitProcessKillsProcgroupGroup` and
  `TestAHI048ExitProcessKillsLiveChildGroups` both pass in `./cmd/corvint` (`-run TestAHI04[89]`).
- **Release before reap.** `TestAHI049RunReleasesGroupBeforeReap` passes with `-count=3`. It
  requires exactly one release, and requires that release while `waitid(WNOWAIT)` still sees the
  leader.
- **Focused packages.** `./internal/procgroup`, `./internal/groupreap` and `./internal/gitstatus`
  all pass.
- **Failed observation (review follow-up).** `TestAHI049FailedExitObservationSendsNoGroupSignal`
  injects ECHILD while the leader is live. It requires three things:
  - the record is released before `BeforeStop`;
  - no group signal is sent (a TERM-trap marker is absent and the leader is still live);
  - cleanup is reported unproved.

  Against `cc2378cb` it fails with "group still recorded for the exit kill after a failed exit
  observation". With that check disabled, it still fails on cleanup reported as proved.
- **Static checks.** `go vet` passes for those packages, and for `GOOS=windows` on `procgroup` and
  `groupreap`. `gofmt` is clean.

The procgroup tests that start a command through `startProcessCommand` and reap it with
`command.Wait` now release the record first. Without that release, `KillLive` would keep a reaped
leader's group.

## Limits

- NOT_RUN: `make gate`.
- NOT_RUN: Linux (the `waitid` path is shared, but this change was observed only on Darwin).
- NOT_RUN: dogfood CEM binding.
- The inventory is static, with hand-narrowed dispatch edges. It is evidence for the current tree,
  not a guarantee for future dispatch.
- `KillLive` no longer names a group after a failed observation. One residual window remains in
  `run`: if the observation fails after the poll that follows `BeforeStop`, while `run` is already
  signalling, that signal can still reach the group.
