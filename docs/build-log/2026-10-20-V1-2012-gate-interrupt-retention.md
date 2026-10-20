# V1-2012: interrupted `gate run` keeps its result and output

Ticket V1-2012, the successor of CORE-FINISH-INTERRUPTED-TRANSCRIPT-PRESERVATION-1 after decision
0544. Requirement CAL-V0-319 in
[corvint-tasks-agent-leases-v0](../specs/corvint-tasks-agent-leases-v0.md), accepted under the
owner's standing auto-accept of lane requirements (2026-10-11). This applies that standing
acceptance; it is not a new owner decision.

## Reproduction

`corvint-tasks` was built from base `f3ca72248` into a scratch directory. A temporary test harness
(not committed) built a disposable store with the CLI test fixtures: policy version 3 with one
`COMMAND` gate `slow` running `/bin/sh -c 'echo out-start; echo err-start >&2; echo out-two; exec
/bin/sleep 60'` (timeout 120 s), two tickets claimed and submitted at the base tree, and a clean
detached worktree. For each attempt the harness started the built binary's `gate run`, waited 3 s,
sent SIGINT (first attempt) or SIGTERM (second), then read `ps`, `attempt show` and the attempt's
gate result and output evidence. No real store was touched.

Before the change (base binary), both signals gave the same result:

- exit status 1 after about 3.2 s; stdout `{"codes":["MALFORMED"],...,"outcome":"ERROR",
  "retryable":false,...,"warnings":["context canceled"]}`; empty stderr;
- the gate's `/bin/sleep 60` was running before the signal and gone after it: the process group
  was killed, with no survivor;
- `attempt show`: phase `BUILT`, lease and holder unchanged, `gateResults: []`;
- the three lines the gate printed were not retained anywhere: the CLI does not stream gate output,
  and `runGate`'s capture buffer was dropped when `GateRun` returned `ctx.Err()`.

So the transcript and any interrupted outcome were lost, and the code was `MALFORMED`.

After the change (same harness, rebuilt binary), both signals gave:

- exit status 0 after about 3.8 s; outcome `OK`, receipt committed, `reusedFrom: null`, and the
  warning `gate run interrupted: the gate's process group was stopped and the run's result and
  captured output were recorded (CAL-V0-319)`;
- no surviving `/bin/sleep 60`;
- the attempt holds one gate result: `state FAILED`, `outcomeClass SIGNAL`, `signal SIGKILL`,
  `exitCode null`, `porcelainClean true`, and `output` evidence of 28 bytes:
  `out-start\nerr-start\nout-two\n`.

## Decisions

- **Record the run the store already knows how to record.** The executed gate has already been
  stopped and reaped when the context ends, so its result is an ordinary §7.1 gate result. It is
  recorded through the same lease write, now run under `context.WithoutCancel` with a 30 s bound,
  the pattern the pool commands already use for their interrupted observations.
- **No new outcome class.** TCP-00 §7.1 closes the gate outcome set. Adding `INTERRUPTED` would
  make an earlier build refuse to decode the attempt's results. The killed gate is `SIGNAL`
  `SIGKILL` and `FAILED`. The fact that the CLI was interrupted travels in the report
  (`GateInterrupted`) and the command warning.
- **Bounded transcript.** The retained output is the gate's existing `output` evidence, capped at
  the executable gate's 16 MiB limit. No new file or tail buffer was added.
- **Unchanged cases.** An interrupt before the gate starts, or during a reuse, still records
  nothing. A run that finished before the interrupt is recorded with its own outcome.

## Verification

- `TestCALV0319_InterruptedGateRunRecordsItsResult` (`internal/tasks/store`) passes with the
  change. Without the `lease_gate.go` change it fails with `context canceled` and no result.
- Focused runs: `TestCALV0016_*`, `TestCALV0078_GateRunContentionAfterExecutionIsReported`, and
  the full `internal/tasks/store` and `internal/tasks/cli` packages (see the PR).

## Limits

- SIGKILL of the CLI itself still records nothing. A supervisor that escalates to SIGKILL within
  the record bound can cut the write; journal recovery handles that as any writer crash.
- An interrupted `gate run` now exits 0 with outcome `OK`, as a `FAILED` gate does. Callers read
  the interruption from the warning or the recorded result.
