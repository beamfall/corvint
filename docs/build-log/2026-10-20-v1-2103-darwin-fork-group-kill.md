# 2026-10-20 Attempt runner: repeat the group SIGKILL while a member forked during the stop is live (V1-2103)

Human-owned intent: `docs/specs/corvint-tasks-attempt-runner-v0.md`, section "Group stop under a
concurrent fork (ATR-V0-153)", accepted under the owner's standing auto-accept (2026-10-11).
GitHub #1116 (nightly release-gates failing). `PGO-V0-001` in
`docs/specs/process-group-owner-v0.md` names the opt-in exception.

BASE: `94d212c8c04c6c702ff238f7fade2ab7d3b2f06e`.

## Problem

Scheduled run 39416257984 (macos-15) failed `TestATRV0004_InterruptStopsTheRun` with exit 2
(`SURVIVORS`, `CLEANUP_HOLD`) after 10.46 s instead of `128+15`, `REFUSED` and `RunInterrupted`.
The 10.46 s matches the runner's 10-second `FinishBounded` bound running out inside the Owner's
pre-reap quiet-proof loop: a live member stayed in the group after the single group SIGKILL.

The test interrupts `/bin/sh -c "touch F; sleep 60"` as soon as `F` exists, which is the moment the
shell forks `sleep`. On Darwin, `killpg` can miss a child that a fork running alongside it adds to
the group. A scratch probe that stopped such a shell 300 times on this host left the child alive
three times, for example:

```
50877 1 50858 S sleep 60
```

(PID, PPID 1 after reparenting, the stopped group's PGID, state `S`), next to the zombie leader the
Owner still held. No second signal was ever sent, so the bound always expired. CI load widens the
window. The defect is in production code: any attempt command that forks while it is interrupted,
times out or loses its lease can hit it.

## Decision

`groupreap.Primitives` gains `RepeatKillWhileLive`. When it is set, each pre-reap quiet-proof
observation of a live member repeats the group SIGKILL. The leader is still unreaped at that point,
so its PID pins the group ID and the signal reaches only the owned group; nothing is sent once the
reap starts. A failed repeat HOLDs without a reap.

Only the attempt runner sets it (`execute` in `internal/tasks/cli/attempt_run.go`; the detached
supervisor uses the same path). `PGO-V0-001`, `PGO-V0-011` and `PGO-V0-012` require exactly one group
signal for the other callers (dispatch reader, CEM Git runner, dogfood aggregate worker, Linux
service descendants probe), and their tests assert it. Changing them is a separate decision, so it
is not made here. The 10-second bound is unchanged and the test is neither skipped nor relaxed.

## Evidence

All on this Darwin host through `script/heavy.sh`:

- `go test ./internal/groupreap`: ok. With `RepeatKillWhileLive` disabled in `waitPreReapQuiet`
  (scratch mutation, reverted), `TestOwnerRepeatKillWhileLiveSignalsOnlyBeforeReap` and
  `TestOwnerRepeatKillFailureHoldsWithoutReaping` fail. `TestOwnerRepeatKillRetiresMemberForkedDuringStop`
  passed 300 stops even with the mutation, so it guards against regressions but does not reliably
  reproduce the race on this host.
- `TestATRV0004_InterruptStopsTheRun -count=20`: ok, run idle twice.
- The same test under load, with ten `yes` burners and `GOMAXPROCS=2` (burners stopped afterwards):
  - Before the test change it failed 1 to 3 times in 20 in a second way. The shell missed the group
    SIGKILL while forking `sleep`, the child was killed, and the shell printed
    `Killed: 9 sleep 60` and exited 137. The runner still returned 128+15, `REFUSED`, `INTERRUPTED`
    and `RELEASED`, and recorded the shell's own exit 137 as `ATR-V0-004` requires. The test
    asserted only signal 9.
  - The test now accepts either the shell's death by the group SIGKILL or its own exit 128+9.
    After that: 20/20 twice and 40/40, with one exit-137 case logged.
- The full `./internal/tasks/cli` package: ok (667 s).
- `go vet` on both packages and `make windows-vet`: clean.

## Limits

- Linux: NOT_RUN on this host. Linux delivers a group signal to a child forked alongside it, so the
  repeat is not expected to fire there.
- A member that keeps forking faster than the repeats can still reach the bound and HOLD.
- #1116 stays open until a scheduled release-gates run passes.
