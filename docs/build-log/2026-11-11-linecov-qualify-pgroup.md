# linecov qualify: each go test run gets its own process group (GLS-V0-051, GLS-V0-052)

Ticket V1-5331 (P1). Spec: `docs/specs/go-line-selection-v0.md` `GLS-V0-051`, `GLS-V0-052`.

## Finding

`linecov qualify` ran every baseline and mutant `go test` with a plain `exec.Command` in
`failingTests` (`tools/linecov/qualify.go`). The run inherited the caller's process group, so a
test that signals its own group (`kill(0, SIGTERM)`, or `kill(-pid, ...)` once a mutant deletes
the assignment that set `pid`) reached the `go` process, `linecov` itself and every process that
shared the group, such as the shell or orchestrator script that started qualify. That is the
rc 143 the ticket reports. The collector already isolated test binaries (`GLS-V0-008`,
`internal/linecov/pgroup_unix.go`); the qualification harness did not reuse it.

Reproduced before the fix by `TestQualifyRunCannotSignalCaller`: a helper process, started by
the test in a group of its own, calls `failingTests` over a fixture package whose test runs
`syscall.Kill(0, syscall.SIGTERM)`. The helper died with `signal: terminated`.

## Change

- `failingTests` runs each package through `linecov.RunIsolated`: `Setpgid` on Unix, the group
  killed on cancellation and any remaining member killed after the run. A run its own group
  signalled has no package result, so the mutant stays `invalid` and a baseline package fails,
  as `GLS-V0-031` already says.
- Isolation means the terminal's Ctrl-C and the caller's SIGTERM no longer reach the run.
  `failingTests` therefore catches SIGINT and SIGTERM while a run is in flight, kills the run's
  group and returns `errQualifyInterrupted`; `mutant` returns that error, so qualify stops instead
  of counting the mutant `invalid` and taking the next site (`GLS-V0-052`).
- Windows has no POSIX group: `RunIsolated` already documents that only the `go` process is
  killed there (`internal/linecov/pgroup_other.go`), and that is unchanged.

## Evidence

- `TestQualifyRunCannotSignalCaller`, `TestQualifyInterruptKillsRunGroup`
  (`tools/linecov/qualify_pgroup_unix_test.go`): both failed before the fix (`signal:
  terminated`) and pass after it. The second also checks that the fixture's test binary, a
  grandchild in the run's group, is gone after the interruption.
- The whole `tools/linecov` package passes; `go vet` passes for linux and `GOOS=windows`.

## Consequence

`tools/linecov/qualify.go` is a qualified source (`GLS-V0-034`), so
`.github/linecov/qualification.json` no longer matches once this lands, and every line-selection
plan stays SHADOW until the record is regenerated. That is the fail-closed state. Regenerating
it is a `.github/**` change and is not part of this change.
