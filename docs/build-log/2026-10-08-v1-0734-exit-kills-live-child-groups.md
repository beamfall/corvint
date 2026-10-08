# 2026-10-08: Adapter exit kills live child process groups (V1-0734)

## Intent

Ticket V1-0734: when the hook adapter watchdog (`AHI-017`) or a dogfood event deadline
(`LCP-V0-008`) abandons a Git read and main calls `os.Exit`, exec's asynchronous context
cancellation (a process-group SIGKILL) may not have run yet. The Git child and its descendants
then outlive the adapter (3.7-4.8 s reported in the slow-Git cases). The change adds proposed
`AHI-048` to `docs/specs/agent-harness-integration-v0.md`: `exitProcess` SIGKILLs every live child
process group synchronously before `os.Exit`.

## Findings on the base

- On this host (load average about 20), base `f33ea8ef` did not reproduce the reported seconds.
  `TestAHI044HookAdaptersFailOpen` slow-Git cases outlived the adapter by 0-1 ms at `-count=1`,
  `-count=3`, and `-parallel 48 -count=3`. Under `-parallel 48`, several cold-start and
  stdout-closed cases left real (not slow) Git children alive 29-103 ms after the adapter exited.
- `internal/procgroup` has no registry, and the hook Git spawns do not go through it. Those spawns
  are in `internal/gokernel`, `internal/contextindex`, `internal/cem/gitrun` and the
  `internal/gitstatus` `xcrun` lookup. `internal/groupreap` already owns group retirement.

## Decisions

- **Registry in `groupreap`.** `StartLive` records a group the child leads, from start until
  `Wait`, `WaitPipes` or an `Owner` releases it. Release happens while the exited leader is still
  unreaped (`waitid WNOWAIT`). `KillLive` holds an exclusive gate, and starts and releases share
  that gate. So the kill never signals a reaped leader's group, never misses a started one, and
  refuses a later start with `ErrExiting`. Groups are recorded only on Darwin and Linux. On other
  Unix platforms, `Wait` reaps before it could release.
- **`contextindex` keeps its pipe-drain contract.** Switching it to `groupreap.Wait` would kill a
  pipe-holding descendant at leader exit and turn the `WaitDelay` incomplete-capture error into a
  silently short read. `TestBuildWithGitExecutionOwnsPipesAndCancellation/cancel-false` caught
  this. The new `WaitPipes` only releases the record at leader exit and then calls `command.Wait`.
  `terminateProcessGroup` stays as it was. A pipe holder is unrecorded during the drain, and that
  limit is listed under `AHI-048`'s failure modes.
- **The `xcrun --find git` lookup is contained** (`Contain` + `StartLive` + `Wait`), so an exit
  that abandons the lookup also retires it.
- **The outlive bound is asserted for slow-Git cases only.** The run-to-run evidence is below: a
  Git that is running, not sleeping, can take longer than 100 ms to act on the SIGKILL under heavy
  host load, even though it was recorded and signalled. Other cases log the time
  (decision 0082).
- **Analyzer audit SHA repinned, schema kept at `/112`.** `contextindex/git.go` and
  `gitstatus/executable.go` changed only process lifecycle, not extraction or encoding.
- `cmd/corvint/main.go` is unchanged, so the use-case receipts need no repin.

## Evidence

- Discriminating: `TestAHI048ExitProcessKillsLiveChildGroups` runs `exitProcess` in a helper
  holding a recorded `sh` leader and a `sleep 300` member. It passes with the kill. With
  `KillLive` removed from `exitProcess`, it fails: `child … outlived exitProcess` at its one-minute
  hang detector.
- `GOMAXPROCS=3 go test -p 1 -count=5 -run 'TestAHI044HookAdaptersFailOpen$|TestAHI048'
  ./cmd/corvint/` PASS on the final code. All 55 slow-Git runs outlived the adapter by 0 ms. Across
  all 385 runs, the maximum was 2 ms.
- Stress: `-parallel 48 -count=5` PASS on the final code. The 55 slow-Git runs measured 0-13 ms,
  and other cases reached up to 33 ms. An earlier stress run, before `WaitPipes`, measured 0-4 ms
  for slow Git and up to 268 ms for other cases. A temporary diagnostic (since removed) logged
  every `KillLive` signal. It showed
  that every Git child still alive at adapter exit had been recorded and SIGKILLed. Those children
  were in states `R`, `U`, `?` or `?E` and still being reaped. `EPERM` from `killpg` appeared only
  for groups already exiting.
- `internal/groupreap` `TestAHI048*` cover the following. All pass under `-race -count=3`:
  - the kill and the start refused after it;
  - recording only for children that lead their own group;
  - `Wait` and `Owner` release before the reap;
  - `WaitPipes` releasing the record without killing the rest of the group;
  - concurrent starts and releases against one kill.
- Race and focused package tests, vet (including `GOOS=windows` and `GOOS=linux`), gofmt and the
  doc gates are listed in the lane report.

## NOT_RUN

- `make dogfood-change`/CEM binding and seal (lane rules: local commit only); `make gate`; full
  `go test ./...`.
- Real Claude Code and Codex hosts. Linux and other platforms were not run; only cross-OS
  `go vet` was.
- The reported 3.7-4.8 s did not reproduce on the base. The slow-Git bound therefore guards the
  regression but does not by itself discriminate on this host. The helper-process test does.

## Rollback

Revert `internal/groupreap/live.go`, `WaitPipes`, the `StartLive`/`Contain` call sites, the
release calls in `groupreap.Wait` and the `Owner` reap, the `KillLive` call in `exitProcess`, the
analyzer audit SHA, and the `AHI-048` text. Children then again outlive an exiting adapter until
their own deadline.
