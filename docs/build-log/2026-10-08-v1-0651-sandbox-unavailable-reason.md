# Mutation runner names a refused nested sandbox

Date: 2026-10-08
Task: V1-0651; TCQ-V0-059 (proposed)
Base: 388fb832

Inside an enclosing macOS sandbox that forbids nesting (Codex CLI workspace-write), every
`go test` the mutation runner launches dies before Go starts. `/usr/bin/sandbox-exec` writes
`sandbox-exec: sandbox_apply: Operation not permitted` and exits 71. The runner classified this
as a broken run and returned the fixed `mutate: run ended without a verdict`. So the
discrimination witness said only `mutation runner failed for T: mutate: run ended without a
verdict`, and nothing separated a host that cannot nest the sandbox from a broken run.

Decision. `inconclusive` now reads one fixed launcher marker. The output must be only the
refusal line, at most 160 bytes: the `sandbox-exec: sandbox_apply: ` prefix, non-empty
printable-ASCII errno text, then a newline. That output returns the exported sentinel
`mutate.ErrSandboxUnavailable` with the fixed text
`mutate: sandbox unavailable: the host refused to apply the sandbox-exec profile`. All other
output returns the old generic error. Neither error echoes output bytes, including the errno
text.

- Spoofing. All runs use `go test -json`, so test output arrives wrapped in JSON events and
  cannot form the bare line. Even a spoofed line only moves a not-run reason between two fixed
  texts. It never yields a verdict.
- Semantics. The outcome is still `runBroken`. The witness stays `not-run` with zero counts,
  and `prove --mutate` still exits 2 `unsupported-prove-mutation` with its fixed message.

Rejected alternative: turning the refusal into an `Unsupported` report, like a host without a
sandbox (`cited tests cannot be sandboxed`). That would change `prove --mutate` from exit 2 to a
`NOT_RUN` row, and this ticket's acceptance holds other semantics unchanged. It remains a
possible follow-up.

Evidence:

- Reproduction. A deny-default outer `sandbox-exec` profile, even one allowing `file*`,
  `process*`, `mach*`, `ipc*`, `sysctl*`, `system*`, `iokit*`, `network*` and `signal`,
  makes a nested `sandbox-exec` print the refusal line and exit 71.
- Live qualification. The compiled `internal/cem/workflow` test binary ran
  `TestDiscriminateRecordsWitnessAndReportDowngrades` inside that outer profile. The test still
  fails as expected (`discriminates:0 notRun:2`) because the host cannot run sandboxed tests.
  The witness detail, read through a scratch log line that was not committed, is now
  `mutation runner failed for pkg/calc/calc_test.go: mutate: sandbox unavailable: the host
  refused to apply the sandbox-exec profile`.
- Deterministic tests:
  - `TestRunReportsASandboxLauncherRefusalAsUnavailable` injects a fake launcher (`sh -c`
    printing the refusal line, exit 71) through `findSandbox` and drives `Run` end to end.
  - `TestInconclusiveNamesALauncherRefusalWithoutEchoingIt` covers these generic-error cases:
    refusal with output before or after, inside a JSON test event, no newline, empty errno,
    control characters, overlong, profile parse error and empty output.
  - `TestInconclusiveDoesNotExposeTestOutput` and `TestRunRefusesToJudgeARunWithoutAVerdict` are
    unchanged and pass.
- Focused `GOMAXPROCS=3 go test -p 1 -count=1` passes for `internal/liveverify/mutate` and
  `internal/cem/workflow`.

Non-goals:

- Linux `bwrap` launcher failures, whose messages were not observed on a Linux host.
- Skipping workflow tests inside a nested sandbox.
- Any change to the `prove` message.

Rollback: remove `ErrSandboxUnavailable` and `launcherRefused`. The refusal then reads as the
generic error again, with no wire, state or exit-status change.
