# V1-0371: JavaScript adapters signal the owned group only before the leader is reaped

Date: 2026-10-09
Ticket: V1-0371
Requirement: AHI-050 (accepted by decision 0474 on the c05bf0a4 text). It supersedes the earlier
AHI-009 normal-exit SIGKILL and EPERM-as-confirmed text, which now points to AHI-050.

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

- An unrecorded `exitCode`/`signalCode` proves the leader unreaped only outside libuv's reap batch
  (see "Review repair" below). Every group signal is therefore decided in a `setImmediate` turn.
- On a timeout or cancellation, each adapter, in that turn, sends SIGTERM to the group and blocks the
  event loop for its termination grace with `Atomics.wait`: 25 ms for OpenCode and 10 ms for Gemini.
  It then sends SIGKILL to the group. The leader cannot be reaped in between. OpenCode terminations
  requested before the turn share one hold.
- The SIGTERM grace is kept because Corvint handles SIGTERM by reaping its own live Git groups
  (`exitProcess`, `groupreap.KillLive`).
- A delivered pre-reap SIGKILL confirms cleanup. After the reap the adapter sends only a
  `kill(-pid, 0)` probe, and only `ESRCH` confirms cleanup. A reused ID can therefore produce only a
  false "unconfirmed", never a false success.
- A normal exit with a surviving descendant completes as `corvint-process-cleanup-unconfirmed` and
  is not signalled.
- The deadlines, `deadlineMs`, `FALLBACK` shapes and exit codes are unchanged.
- The adapter versions are bumped under AHI-020: OpenCode 0.7.10 and Gemini 0.2.7, then 0.7.11
  and 0.2.8 for the review repairs.
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
- NOT_OBSERVED: whether Bun, which OpenCode embeds, reaps children off the event-loop thread or
  outside the poll phase.
- Unsupported conditions: such reaping, or a foreign in-process `waitpid(-1)`, can reap the leader
  early. That voids the signalling proof: a group signal may reach a reused ID, and a delivered
  SIGKILL confirms cleanup without a probe, so neither safety nor an honest result is guaranteed.
- Each termination turn blocks the host event loop for one grace: 25.5 to 30.4 ms for sixteen
  OpenCode cancellations in one turn. Cancellations in separate turns each block for 25 ms.
- OpenCode leaves its stdio pipes open after its bounded completion until the holder exits; it
  reads and discards their late output. Gemini destroys them because its process must exit.
- A killed member that init has not yet reaped reads as unconfirmed.
- Descendants that left the owned group are not covered.

## Review repair

The independent review (verdict FAIL) found three issues; each is repaired with a regression that
fails on `7a4078b8` and passes after.

- **P1, unsound proof.** libuv's `uv__wait_children` (`src/unix/process.c`, v1.53.0 as bundled by
  Node v22.23.3) reaps every exited child in one pass and only then dispatches exit callbacks one by
  one; Node's `onexit` sets
  `exitCode` inside its own callback, and ticks and microtasks run between callbacks. So inside a
  sibling's exit callback the leader can be reaped while its fields still read null, and the
  adapter signalled a possibly reused group. Repair: a termination request only records its code;
  the group decision runs in a `setImmediate`. The check phase runs after the poll phase, where both
  the Linux SIGCHLD handler and the Darwin kqueue `EVFILT_PROC` path call `uv__wait_children` to
  completion, so every leader reaped by then has its exit recorded, and no reap can occur between
  that reread and the signals sent in the same synchronous turn. A request cancelled by completion
  is dropped; a completion before the decision probes only.
  Tests: `V1-0371 OpenCode|Gemini never signals a leader reaped in the same batch as a sibling exit`.
  A sibling and the leader are blocked into zombies (witnessed by `ps`), the termination is requested
  from the sibling's exit callback, and the test requires `ps` to show the leader gone with exit
  fields null, then no non-zero signal to its group. On `7a4078b8` both record `SIGTERM`.
- **P2, Gemini hang.** After the reap Gemini sent nothing but still resolved only on `close`, which
  a descendant holding the leader's stdout delays indefinitely. Repair: one grace after its group
  decision, Gemini settles without `close`, destroys its pipe ends and reports through the same
  cleanup decision. OpenCode already completes 100 ms after a termination request (`reapTimer`).
  Test: `V1-0371 Gemini completes with unconfirmed cleanup when a descendant keeps the leader's
  stdout open` (1500 ms host kill, `sleep 20` descendant). On `7a4078b8` it waits for the
  descendant.
- **Cost.** Sixteen OpenCode runners cancelled by one `abort()` blocked the thread 434 to 472 ms on
  `7a4078b8` (five rounds; 0.9 to 2.2 ms on `dd90cfa6`). With one shared hold per turn: 25.5 to
  30.4 ms. Test: `V1-0371 OpenCode concurrent cancellations share one SIGTERM grace` counts
  `Atomics.wait` calls: 16 on `7a4078b8`, 1 after.

After the repair: the ten `V1-0371` tests pass three runs in a row; the Go wrapper
`TestHostAdapterJavaScriptHosts|TestHostAdapterJavaScriptHarnessInterruption|TestAHI016` passes with
the whole file (69 tests); the OpenCode package tests pass 40/40. `host-package-versions-check` was
run before committing, so it read the earlier history and passed wrongly; the committed repair
left both bumps older than the shipped change. The second review repair bumps OpenCode to 0.7.11
and Gemini to 0.2.8.

AHI-050 wording changed, for owner re-confirmation (ID and intent unchanged):

- The proof sentence "For Node, which reaps a child only on its event-loop thread and records
  `exitCode` or `signalCode` before it emits `exit`, an unrecorded exit is that proof" became: the
  unrecorded fields are that proof only when read outside libuv's reap batch, so every group signal
  is decided in a later `setImmediate` turn that rereads them.
- Added: "OpenCode terminations requested before that turn share one hold."
- Added: completion after a termination does not wait for `close` (OpenCode 100 ms after the
  request, Gemini one grace after its decision, closing its pipe ends).
- Acceptance: six tests became ten, with the same-batch, inherited-pipe and shared-grace cases.
- Failure modes: "blocked for the grace on each termination" became one grace per termination turn,
  with the measured numbers; "another child's exit dispatch" was removed as a residual window, and
  "outside the poll phase" was added to the unobserved-runtime window.
- Rollback and traceability name the new functions. `go-only-cutover-v0.md` got the matching two
  sentences.

## Second review repair

The re-review of `fd48aee7` confirmed the P1 fix, the inherited-stdout fix and the cost (549 to
32 ms in its measurement) and found three issues.

- **P2, Gemini hook exit.** Settling destroyed the pipes but kept the live leader's process handle
  referenced, so a TERM-ignoring leader that survived a failed group SIGKILL kept the hook running
  after its report. `settle` now calls `child.unref()`; its timers are already cleared. Test:
  `V1-0371 Gemini exits after reporting a leader whose group kill failed` (1500 ms host kill, group
  SIGKILL failure injected through the existing spy). It fails on `fd48aee7`, where the hook runs
  until the leader's `sleep 20` ends.
- **P2, AHI-020.** OpenCode 0.7.11 and Gemini 0.2.8; `sh script/check-host-package-versions.sh`
  passes on the committed tree.
- **P3, wording.** AHI-050's failure modes and the limits above said the probe keeps the result
  honest after foreign or off-thread reaping. `cleanupConfirmed` accepts a delivered SIGKILL without
  probing, so that reaping voids the proof; the text now says neither safety nor an honest result
  is guaranteed there. AHI-050 also gains the hook-exit sentence and the eleventh acceptance test.

## Rollback

Revert the adapter changes, the tests and the version bumps. The adapters then again send the group
SIGKILL after the reap.
