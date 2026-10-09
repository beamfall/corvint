# V1-0371: JavaScript adapters signal the owned group only before the leader is reaped

Date: 2026-10-09
Ticket: V1-0371
Requirement: AHI-050 (proposed; acceptance is human-owned). It refines the accepted AHI-009
normal-exit sentence and conflicts with it.

## Intent

A same-group descendant that ignores SIGTERM and closes its stdio must not outlive an OpenCode or
Gemini adapter timeout or cancellation. Completion must not run ahead of the final cleanup decision,
and a normal exit must not leave such a descendant behind silently. The fix must also follow the
process-group rule used by `internal/procgroup` and `internal/groupreap` (V1-0373, V1-1036/1040/1047):
a numeric group signal is sent only while group ownership is proven, which means while the leader is
unreaped.

## Finding on the base

The 2026-09-26 fix (`docs/build-log/2026-09-26-v1-0371-adapter-descendant-cleanup.md`) killed the
descendant, but it sent the group SIGKILL from timers and from the `exit`/`close` path after Node
had already reaped the leader. At that point the group ID can belong to an unrelated process. The new
spy records any non-zero group signal sent after the leader's `exit` was emitted. On the base it
recorded late OpenCode `SIGKILL`s on the timeout path, and the Gemini normal exit reported success.

## Decision

- Node reaps a child only on the event-loop thread, and records `exitCode`/`signalCode` before it
  emits `exit`. An unrecorded exit therefore proves that the leader is unreaped.
- On a timeout or cancellation, each adapter sends SIGTERM to the group and blocks the event loop
  for its termination grace with `Atomics.wait`: 25 ms for OpenCode and 10 ms for Gemini. It then
  sends SIGKILL to the group. The leader cannot be reaped in between.
- The SIGTERM grace is kept because Corvint handles SIGTERM by reaping its own live Git groups
  (`exitProcess`, `groupreap.KillLive`).
- A delivered pre-reap SIGKILL confirms cleanup. After the reap the adapter sends only a
  `kill(-pid, 0)` probe, and only `ESRCH` confirms cleanup. A reused ID can therefore produce only a
  false "unconfirmed", never a false success.
- A normal exit with a surviving descendant completes as `corvint-process-cleanup-unconfirmed` and
  is not signalled.
- The deadlines, `deadlineMs`, `FALLBACK` shapes and exit codes are unchanged.
- The adapter versions are bumped under AHI-020: OpenCode 0.7.10 and Gemini 0.2.7.
- Two alternatives were rejected:
  - A shell or sentinel wrapper to pin the group adds a process and a shell dependency.
  - An immediate SIGKILL skips Corvint's own SIGTERM cleanup.

## Evidence

All runs used Node v22.23.3 on Darwin arm64, with a fixture and a real binary built from this tree.

- **Fails on the base:** the six new `V1-0371` tests in `integrations/host-adapters.test.mjs`
  (timeout/cancellation, normal exit and failed group kill, for both hosts) run 6, pass 0, fail 6.
  - The OpenCode timeout test recorded late group SIGKILLs.
  - The Gemini normal exit returned no degradation.
  - The failed-kill tests failed through the late-signal assertion.
- **After the fix:**
  - The six tests pass.
  - The whole file passes 65/65.
  - The OpenCode package tests (cockpit, inspector, task-metrics, ui-presentation, workbench) pass
    40/40.
  - The Go test `cmd/corvint` `TestHostAdapterJavaScriptHosts|TestHostAdapterJavaScriptHarnessInterruption|TestAHI016OverBoundPromptMatchesCrossHostBoundaryCases`
    passes.

## Limits

- NOT_RUN:
  - Linux.
  - `make gate` and the repository-wide Go suite.
  - The dogfood CEM bind.
- NOT_OBSERVED: whether Bun, which OpenCode embeds, reaps children off the event-loop thread. If it
  does, the pre-reap proof does not hold there, and only the probe keeps the result honest.
- Residual windows: another child's exit dispatch (the same window as Node's `child.kill`), or a
  foreign in-process `waitpid(-1)`, can reap the leader early.
- Each termination blocks the host event loop for the grace.
- A killed member that init has not yet reaped reads as unconfirmed.
- Descendants that left the owned group are not covered.

## Rollback

Revert the adapter changes, the tests and the version bumps. The adapters then again send the group
SIGKILL after the reap.
