# 2026-10-07: retire ended detached runs of terminal attempts (V1-0931)

## Intent

Ticket V1-0931 notes that `<git-common-dir>/taskman-runs/` kept every detached run directory
forever. ATR-V0-010 caps a live attempt at 64 runs but never removes any. Proposed ATR-V0-015
(`docs/specs/corvint-tasks-attempt-runner-v0.md`), pending owner acceptance, retires the run
directories of terminal attempts on a write path. A run directory holds the record, result, two
8 MiB output segments and the supervisor log.

## Decision

The work runs in the detached supervisor, after its own `FINISHED` record. That process already
writes run state, and attach and other read paths stay read-only (invariant 4). A run qualifies
only when all of these hold:

- its record decodes and matches its own directory and attempt hash;
- it has ended (`FINISHED`, or both supervisor and command proven gone by start identity);
- it has been quiet for an hour.

Nothing is read from the store unless more than 16 runs qualify. Then one audited
`store.AttemptRecords` read must find each attempt present and in a terminal phase, at the run's
generation or later. A retry keeps the attempt ID at a newer generation. Of those runs, the newest
16 stay.

Each removed run is first renamed out of its attempt directory, so attach sees the whole run or
none of it. Leftovers from interrupted passes are deleted, and emptied attempt directories are
removed. If a run disappears after attach read its record, attach refuses `MISSING_EVIDENCE`
rather than `MALFORMED`. The scan is bounded at
1,024 attempt directories with 65 entries each.

## Bounds

- A live attempt keeps up to 64 runs (unchanged).
- Terminal attempts together keep at most 16 quiet ended runs, plus runs that cannot be proven
  ended (malformed, no command identity, a supervisor that died while `STARTING`) and runs changed
  within the hour.
- Each run is at most about 2×8 MiB of output plus small records.

## Evidence

- `go test -count=1 -run 'TestATRV|TestCALV0131|TestAttemptRun' ./internal/tasks/cli`: rc=0 in
  14.2 s, after the review fixes.
- `TestATRV0015_SupervisorRetiresEndedRunsOfTerminalAttempts` planted these runs:
  - 18 FINISHED runs and one dead RUNNING run under a released attempt;
  - one older run under a second released attempt;
  - a live-supervisor run, a malformed run, a misplaced directory, and three runs of a live
    attempt.

  - a run recorded at a newer generation than the terminal record;
  - a `.retired-*` leftover.

  After a detached run of the live attempt finished, its supervisor removed the 2 oldest FINISHED
  runs, the dead run, the second attempt's run and that attempt's emptied directory. It also
  deleted the leftover. Everything else stayed.
- `TestATRV0015_AttachOfARunRetiredMidReadIsMissing`: a run that is gone as a whole refuses
  `MISSING_EVIDENCE`, and a record without its result stays `MALFORMED`.
- Mutations that dropped the terminal-phase guard or the generation guard each made the test fail.
- `go vet ./internal/tasks/cli ./internal/tasks/store`: clean.

## Independent review

One Codex review (gpt-6-astra, read-only) of `5808c26c..f34c2386` requested changes. It raised
three points about this ticket:

- A retry could revive the attempt between the store read and the removal. This is narrowed by the
  generation guard: only that attempt's earlier-generation runs, ended and quiet for an hour, can
  be lost. No new-generation run can be lost. A full fix needs the store lock, which is an owner
  decision.
- An attach that races a removal reported `MALFORMED`. This is fixed by the rename-first removal and
  the `MISSING_EVIDENCE` replay.
- The bounded scan could starve retirement. This is documented with its precise condition.

## Limits

- Retirement runs only when some supervisor finishes.
- A pass sees only the first 1,024 attempt directories in directory order. Attempts beyond them
  stall while that window holds mostly live or absent attempts, or runs that cannot be proven
  ended.
- A retry racing the store read can lose earlier-generation runs, as described under the review.
- Attaching to a retired run refuses `MISSING_EVIDENCE`, and its `RUN_OUTCOME` stays in the
  journal.

The full `internal/tasks/store` and `internal/tasks/cli` package runs, Linux, and live store
qualification were NOT_RUN.

## Rollback

Revert the commit. Retirement stops, removed runs stay removed, and no record, receipt or wire
shape changes.
