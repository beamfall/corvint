# V1-1652 and V1-1582: impact deadline retry and a contended-write duplicate

Date: 2026-10-20. Base: `9a63cf8f787eb50bf749296c80df1c36c196c7f7` (origin/main).
Tickets: V1-1652 (spec `docs/specs/daily-change-evidence-workflow-v0.md`, DCW-V0-075, accepted
under the owner's standing auto-accept of lane requirements, 2026-10-11) and V1-1582 (duplicate of
V1-1558).

## V1-1652: problem

Each Git phase of an index build has a 30-second deadline (`gitDeadline`). Under host load the
coordination-time `impact` of `dogfood change` overran it. The refusal carries no error code, so
`prechangeImpact` recorded a blocking `NOT_PRODUCED exit-2` row and the change was incomplete. There
was no retry, and the deadline was undocumented.

## V1-1652: decisions

- Retry and abstain in `internal/dogfoodflow`, not in the CLI. Giving the refusal a code would
  change the bytes of every read command's error, and `cmd/corvint` belongs to another lane. Instead,
  the exact uncoded envelope with exit 2 and empty stdout is matched.
- One bounded retry, then a non-blocking `impact-deadline-exceeded` row. This row is not a
  DCW-V0-025 digest-bound abstention. A deadline overrun is a host condition, not a stable
  repository fact, so check does not rerun it. Check proves only that the retained stdout is empty
  and the stderr is exactly the envelope. Both change and check print a NOTE line, so the missing
  impact evidence stays visible.
- No override. The deadline stays 30 seconds and is now documented in the spec. `git.go` is not
  edited. Every production source of `internal/contextindex` is an input to the
  `TestAnalyzerSchemaInputs` audit, so even a const-to-var change would need an analyzer schema
  bump, and that bump invalidates every index pack. Instead, the slow-Git test overruns a shorter
  caller deadline, which ends the same per-phase context and produces the same refusal.

## V1-1582: disposition

V1-1582 and V1-1558 were filed from the same issue (1026). Commit `c79f6d47f` delivered V1-1558,
and every V1-1582 acceptance criterion maps to one of its accepted requirements and tests:

- A timed-out attached run under writer contention exits 127 with one item, never `MALFORMED`
  (ATR-V0-051, `TestATRV0051_ContendedTimedOutRunExits127WithOneItem`).
- A deadline-exceeded store write is `LOCK_TIMEOUT` (ATR-V0-050,
  `TestATRV0050_ContendedPrelaunchWriteIsLockTimeout`,
  `TestATRV0050_ContextEndedIsRetryableLockTimeout`).
- The outcome write is retried within the 60-second budget
  (`TestATRV0051_OutcomeWriteRetriesUntilTheLockFrees`).
- `claim --detach` ceiling expiry and SIGINT both report `LOCK_TIMEOUT` (ATR-V0-053,
  `TestATRV0053_DetachedClaimCeilingAndInterruptAreLockTimeout`).

This lane changed no code for V1-1582. It added a duplicate note to the ATR spec and reran those
tests on this base: all PASS on Darwin.

## Evidence

- `TestDCWV0075GitDeadlineIsThirtySeconds` and `TestDCWV0075SlowGitOverrunsTheDeadline`
  (`internal/contextindex`) cover the deadline value and an injected slow-Git overrun. `TestAnalyzerSchemaInputs` still passes.
- `TestDCWV0075OneOverrunIsRetriedOnce` and
  `TestDCWV0075SecondOverrunIsAVisibleNonBlockingAbstention` (`internal/dogfoodflow`) cover the
  retry and the admitted row.

## Limits

- A live overrun under real host load is NOT_OBSERVED.
- The retained envelope is not digest-bound.
- If the deadline message is reworded, the row reverts to the blocking `exit-2`.
- V1-1582 on Linux is NOT_RUN.
