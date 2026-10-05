# PMR-V2-006 collector cleanup handles and fixtures (#395, V1-0542)

The Codex re-review of `9e3c82d8..690ac633` confirmed the round-3 fixes for:

- the pidfd ordering;
- the fork-and-exit refusal;
- the `WaitDelay` default;
- the capture and child-discovery latches.

It raised three P2 findings. Each one was checked against the source before it was changed.

Findings and fixes:

- P2-1, a failed freeze dropped a killable process. Confirmed: a freeze timeout closed the pidfd,
  and the caller never added that process to the kill pass. A member stopped by a tracer, or stuck in
  uninterruptible I/O, therefore stayed alive.
  - The walk now keeps every pidfd whose birth it confirmed in a handle set. That set is separate
    from the confirmed-frozen members it walks.
  - The kill pass signals every handle. The wait to confirm the kills has its own bound, because a
    timed-out freeze has already used the walk deadline.
  - The failed freeze is still reported unresolved. A pidfd opened for an unconfirmed birth is still
    closed unsignalled.
- P2-2, the fork-and-exit fixture could hang. Confirmed: `os.WriteFile` blocks opening the FIFO when
  the reader was never forked, and freezing the leader before that fork made this permanent.
  - The test now holds the FIFO open read-write, so neither open blocks and the release write fits
    the pipe buffer. The write also has a deadline.
  - Before cleanup, the test confirms the reader is a child of the leader with the FIFO as its
    stdin. The hook asserts that the reader is listed.
- P2-3, the thread test did not tell the fix apart from a leader-only check. Confirmed: SIGKILL ends
  the helper either way. Two fixtures were added:
  - `TestProcessCollectorV2CleanupKillsPartlyStoppedMember` seizes one non-leader thread of a Go
    helper with `PTRACE_SEIZE` and resumes it from each group stop. The leader stops while that
    thread runs. The walk must never list that member's children, must report it "not confirmed
    frozen", and must kill it through its handle. This is also the P2-1 timeout regression.
  - `TestProcessCollectorV2CleanupWalksLateChild` uses a subreaper helper. Its shell child is killed
    after the first listing, so the orphan is adopted late. A later pass must list the orphan and
    kill it.
  - The `CLONE_PARENT` claim is narrowed to a stated untested limit.

The misdated round-2 entry was renamed to `2026-10-04-postmerge-host-collector-review-fixes.md`.

Mutation evidence, Linux arm64 container: each revert below, applied alone, made its test fail.

| Revert | Failing test |
| --- | --- |
| Drop the birth start check | `CleanupSkipsRecycledPID` |
| Drop the `Capture` latch | `FailedCaptureEmitsNoProof` |
| Leave `WaitDelay` unset | `EscapedOutputDoesNotStallWait` |
| Keep handles only for frozen members | `CleanupKillsPartlyStoppedMember` |
| Check only the leader thread | `CleanupKillsPartlyStoppedMember` |
| List each member once | `CleanupWalksLateChild` |

Limits retained:

- The ptrace fixture skips as `NOT_OBSERVED` where ptrace is refused.
- The fork-and-exit fixture change is test-only and has no mutation.

Rollback: revert this change.
