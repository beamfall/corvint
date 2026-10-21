# 2026-10-20 Process-group cleanup in the tools and qualification scripts (V1-1387)

Human-owned intent: `PGO-V0-018` and `PGO-V0-019` in `docs/specs/process-group-owner-v0.md`,
accepted under the owner's standing auto-accept (2026-10-11). Follows `PGO-V0-009` (product code)
and `PGO-V0-017` (V1-2111, `RepeatKillWhileLive`).

BASE: `90ab9f33ebf5fa9af222dc8132b0928769c9f45a`.

## Problem

`PGO-V0-009` removed post-reap group signals from product code only. At BASE the development tools
and scripts still signalled a numeric group after the leader could have been reaped, and some waits
had no bound:

- `tools/beamfall-shadow/process_unix.go:19,57` and `main.go:727` sent `kill(-pid, SIGKILL)` after
  `cmd.Wait`, or from a `CommandContext` cancel that races it.
- `tools/beamfall-shadow/wrapper/main.go:95-120` sent SIGTERM and SIGKILL to `-pid` after the reap.
- `tools/ci-test-slices` killed the slice group around `cmd.Wait`.
- `script/qualify-checkpoint-handoff.py:77-80`, `script/measure-lsp-quality.py:23-35` and
  `script/qualify-lsp.py:34-35` called `os.killpg` after the leader could be reaped, then an
  unbounded `communicate()`.

## Decision

- Owner gains `Signal(sig)` (`PGO-V0-018`). It sends a graceful stop while the leader is pinned and
  is refused once retirement begins. It also gains `HeldGroupQuiet`, the pre-reap quiet proof for
  callers that wait for residue themselves.
- The Go tools start through `groupreap.StartWith` with `RepeatKillWhileLive` (each forks through
  `go` or `sh`) and retire through `FinishBounded`.
  - Stops go through `Owner.Stop`; the wrapper's grace goes through `Owner.Signal(SIGTERM)`.
  - An unproved retirement is HOLD: shadow `CLEANUP_ERROR` / `PROCESS_GROUP_HOLD`, wrapper exit 124.
  - A host without the Owner is unsupported before start.
  - `exec.Command` replaces `CommandContext` in the shadow harness, so the standard library never
    signals a reaped PID.
- Linux post-reap probes first reap zombies reparented to the subreaper. They run only after the
  leader reap, so `wait4(-1)` cannot take the leader.
- Python 3.9 has no `waitid`. `script/process_group.py` `OwnedGroup` therefore pins the group ID
  with a keeper process that leads the group and is reaped last; the command joins it with
  `setpgid`.
  - Retirement repeats SIGKILL while output is held, drains within 10 s, sends one final SIGKILL,
    then reaps the keeper.
  - An unproved retirement raises `GroupHold`. Signalling after retirement is refused.
  - Darwin's EPERM for a zombie-only group is treated as no member, as in `owner_waitid.go`.
  - Python stays out of gate paths (`GOC-V0-008`), so this is witnessed by a scripted run rather
    than a maintained test.

## Evidence (Darwin arm64)

- `go test ./internal/groupreap -run TestPGOV0018`: PASS.
- Full packages pass:
  - `tools/ci-test-slices` (5.4 s);
  - `tools/beamfall-shadow` (2.4 s), with five `TestPGOV0019Shadow*` cases;
  - `tools/beamfall-shadow/wrapper`, with `TestPGOV0019Wrapper*` and the existing
    early-signal launch tests.
- The new tests record kill/signal/probe/reap order. Each requires exactly one reap, the group
  kill before it, and only probes after it. HOLD cases require no reap.
- Without-fix: at BASE the tools have no injectable Owner host, so the ordering tests cannot run
  against the old code. The post-reap signals are cited above by line.
- `go vet` is clean for darwin, linux, freebsd, openbsd and windows on the touched packages.
- Python witness, run from a scratch script against `script/process_group.py` (Python 3.9.6):

  | Case | Witness result | Elapsed | Member PID | Member dead | Keeper reaped | Post-retire signal refused |
  |---|---|---|---|---|---|---|
  | `descendant-holds-pipe` (`sleep 300 &`, stdout inherited) | PASS | 2.01 s | 91626 | yes | yes | yes |
  | `descendant-detached-from-pipe` | PASS | 0.03 s | 91814 | yes | yes | yes |

  - `descendant-holds-pipe`: the 2 s communicate timed out, then retirement finished.
  - Before the EPERM handling, the second final `killpg` raised `PermissionError` on the
    zombie-only group, which is why that handling was added.
  - The converted call sites were also checked:
    - `measure-lsp-quality.command` with a 1 s timeout returned in 1.01 s, `timedOut`, with the
      lingering member dead;
    - `qualify-lsp.run` with a residual member returned in 0.01 s, with the member dead.

## Limits

- A descendant that leaves the group (setsid) is outside every proof here.
- Linux runs are NOT_RUN locally, including `TestTimeoutReapsDescendantAndReceiptsCleanup`, which
  skips on Darwin. They come from PR CI.
- `internal/liveverify/mutate/gotest.go` still calls `terminateProcessGroup` after `command.Run()`
  returns, a post-reap group kill of the same class. It is out of this ticket's scope and reported
  separately.

## Rollback

Revert the V1-1387 commit. The tools and scripts return to their own group handling; no store or
wire format changes.
