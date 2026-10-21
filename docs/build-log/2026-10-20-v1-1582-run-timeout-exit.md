# 2026-10-20 Attempt runner: V1-1582 and V1-2103 re-examined at origin/main (no product change)

Tickets: V1-1582 (GitHub issue 1026) and V1-2103 (GitHub issue 1116). Spec:
`docs/specs/corvint-tasks-attempt-runner-v0.md`. BASE: `2b0cfce94ae2cea248c3ccab4b301af99af572d6`.
No requirement is added or amended, and no code changes.

## V1-1582: a timed-out attached run reports MALFORMED under contention

Finding: issue 1026's symptom came from the pre-launch path, not from a post-launch outcome write.
V1-1558 (`c79f6d47f`, ATR-V0-050..053 and ATR-V0-066) already delivered the fix, and it cites
issue 1026.

- On the owner's reproduction base `55abb815`, `finish` always sets one run item. An envelope with
  items `[]` therefore cannot come from a started run.
- `prepare` returned `errorResult(cmd, err)` with the bare context error for its heartbeat,
  attempt read, renew and re-read. These four steps share one 60-second write context, and
  `wire.CodeOf` maps an uncoded error to `MALFORMED`.
- Exit 1, `MALFORMED`, the warning `context deadline exceeded` and no item is exactly that path. The
  command was never started.
- The ticket's hypothesis does not hold. The outcome write does not share the run's context:
  `recordOutcome` uses its own `writerContext()` deadline. Since `c79f6d47f` it also maps an ended
  context to `LOCK_TIMEOUT` and retries under one request id.
- The reported elapsed time of about 150 s is not explained by the code: `prepare` is bounded at
  60 s. Treat it as `NOT_OBSERVED`.

Acceptance mapping, with every test passing at BASE:

| V1-1582 criterion | Requirement | Test (`internal/tasks/cli`) |
| --- | --- | --- |
| Timed-out run under contention exits 124 or 127, never 1/`MALFORMED` | ATR-V0-051 | `TestATRV0051_ContendedTimedOutRunExits127WithOneItem`, `TestATRV0051_OutcomeWriteRetriesUntilTheLockFrees` |
| One run item whenever the command started | ATR-V0-052 | `TestATRV0051_ContendedTimedOutRunExits127WithOneItem`, `TestATRV0004_UnrecordedOutcomeExits127` |
| Ended-context store write maps to retryable `LOCK_TIMEOUT` | ATR-V0-050 | `TestATRV0050_ContendedPrelaunchWriteIsLockTimeout` (and `internal/tasks/wire`) |
| Outcome write retried before giving up | ATR-V0-051 | `TestATRV0051_OutcomeWriteRetriesUntilTheLockFrees` |
| Detached claim ceiling or SIGINT is not `MALFORMED` | ATR-V0-053, ATR-V0-066 | `TestATRV0053_DetachedClaimCeilingAndInterruptAreLockTimeout`, `TestATRV0066_KeptLockTimeoutRelaunchesOnSameRequestID` |

The failing-first reproduction is already on record in
`docs/build-log/2026-10-17-v1-1558-contended-store-writes.md`: at `072a306f1`, three of the four
contention tests fail with the issue 1026 envelope.

Residual, outside this ticket's contention scope: ATR-V0-051 says a started run never reports
`MALFORMED` for a store write. `finish` can still report it in two cases:

- the outcome write returns an uncoded error that is not a context error, such as a filesystem
  error, because `wire.CodeOf` defaults to `MALFORMED`;
- the outcome receipt is refused without a code (`refusal`).

Both cases still exit 127 with one item. Choosing a different code for an uncoded write failure is
a separate decision.

## V1-2103: TestATRV0004_InterruptStopsTheRun reports SURVIVORS on macos-15

Finding: this is a product defect, and it was fixed in `00c904e78` (with test follow-up
`3140d8ca1`, ATR-V0-153); see `docs/build-log/2026-10-20-v1-2103-darwin-fork-group-kill.md`.

- The failing scheduled run, 39416257984 at `f3ca72248`, predates that fix.
- Its 10.46 s matches the runner's 10-second `FinishBounded` bound expiring in the pre-reap
  quiet-proof loop. A group SIGKILL had missed a member forked during the stop. The 2-second
  post-reap probe cap would have ended earlier.
- The survivor check already waits within its bounded allowance, and the runner now repeats the
  SIGKILL while a live member remains, so no test-only change or longer sleep is warranted.

Re-verification at BASE on this Darwin host:

- `TestATRV0004_InterruptStopsTheRun` with `GOMAXPROCS=2 -count=40`: 40/40 pass, host load
  average 37 to 48.

Remaining closeout: the ticket's criterion asks for a passing scheduled release-gates run. The
latest scheduled runs (39416257984, 39258757168, 39121848489) all predate `00c904e78`. The 10-18
and 10-19 runs failed elsewhere: `TestMCPV0038IndexDeadlineHasDistinctCode` on macos-15 and
`TestTQDV0021_RealProbes` on ubuntu-24.04.

## Limits

- Linux: NOT_RUN.
- No new hosted release-gates run was observed.
