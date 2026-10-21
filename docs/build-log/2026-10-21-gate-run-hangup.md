# V1-2033: `gate run` keeps its transcript on INT, TERM and now HUP

This entry covers spike ticket V1-2033: does `corvint-tasks gate run` keep the gate transcript when it is
interrupted by INT, TERM or HUP? The requirement is CAL-V0-319 in
[corvint-tasks-agent-leases-v0](../specs/corvint-tasks-agent-leases-v0.md). The amendment was accepted under
the owner's standing auto-accept of lane requirements (2026-10-11). Spike decisions were delegated
to the orchestrator under decision 0548.

## Finding

- **INT and TERM were already delivered by V1-2012 (CAL-V0-319).** In `internal/tasks/cli/lease.go`,
  the `gate run` branch wraps `store.GateRun` in
  `signal.NotifyContext(writerContext(), os.Interrupt, syscall.SIGTERM)`. `GateRun` stops the gate's
  process group and then records a `FAILED` result with the captured output. The store test
  `TestCALV0319_InterruptedGateRunRecordsItsResult` pins this.
- **HUP was not handled.** SIGHUP was missing from that list, so Go's default action ended the CLI
  at once. Before the fix, the new subprocess test sent SIGHUP to a running `gate run`. The result
  was "gate group … survived the SIGHUP (runner signal: hangup)". The gate's own process group was
  orphaned (it kept running) and nothing was recorded.

A closed terminal or a dropped agent session sends exactly this signal. The other long-running
`corvint-tasks` verbs already treat HUP like TERM: `run --attempt` (`runnerSignals`, ATR-V0-004),
`dispatch` and `service`.

## Change

- SIGHUP is added to the `gate run` NotifyContext.
- CAL-V0-319 now reads "SIGINT, SIGTERM or, since V1-2033, SIGHUP".
- The new test `TestCALV0319_HangupInterruptsGateRun` (`internal/tasks/cli`, darwin and linux) works like
  the ATR-V0-004 hangup test:
  - It re-executes the test binary as a real `gate run`, sends SIGHUP, and requires the gate group to be gone.
  - It requires the interruption warning in the envelope and a recorded receipt.
  - Without the fix it fails; with the fix it passes.

## Not covered

SIGKILL is still out of scope, as in the CAL-V0-319 non-goals. `run --attempt` passes its command's output
straight through rather than capturing it, so it has no transcript to retain. That is by design (ATR) and was
not changed.
