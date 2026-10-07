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
`store.AttemptRecords` read must find each attempt present and in a terminal phase. Of those
runs, the newest 16 stay, and emptied attempt directories are removed. The scan is bounded at
1,024 attempt directories with 65 entries each.

## Bounds

- A live attempt keeps up to 64 runs (unchanged).
- Terminal attempts together keep at most 16 quiet ended runs, plus runs that cannot be proven
  ended (malformed, no command identity, a supervisor that died while `STARTING`) and runs changed
  within the hour.
- Each run is at most about 2×8 MiB of output plus small records.

## Evidence

- `go test -count=1 -run 'TestATRV|TestCALV0131|TestAttemptRun' ./internal/tasks/cli`: rc=0 in
  17.5 s.
- `TestATRV0015_SupervisorRetiresEndedRunsOfTerminalAttempts` planted these runs:
  - 18 FINISHED runs and one dead RUNNING run under a released attempt;
  - one older run under a second released attempt;
  - a live-supervisor run, a malformed run, a misplaced directory, and three runs of a live
    attempt.

  After a detached run of the live attempt finished, its supervisor removed the 2 oldest FINISHED
  runs, the dead run, the second attempt's run and that attempt's emptied directory. Everything
  else stayed.
- A mutation that dropped the terminal-phase guard made the test fail.
- `go vet ./internal/tasks/cli ./internal/tasks/store`: clean.

## Limits

- Retirement runs only when some supervisor finishes.
- A large backlog shrinks over several passes.
- Attaching to a retired run refuses `MISSING_EVIDENCE`, and its `RUN_OUTCOME` stays in the
  journal.

The full `internal/tasks/store` and `internal/tasks/cli` package runs, Linux, and live store
qualification were NOT_RUN.

## Rollback

Revert the commit. Retirement stops, removed runs stay removed, and no record, receipt or wire
shape changes.
