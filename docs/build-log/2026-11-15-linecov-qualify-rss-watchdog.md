# linecov qualify: a memory watchdog on each go test run (GLS-V0-058, GLS-V0-059)

Ticket V1-5336 (P1). Spec: `docs/specs/go-line-selection-v0.md` `GLS-V0-058`, `GLS-V0-059`.

## Finding

The ticket says `failingTests` ran `go test` with no process group. That part was already fixed
on main by V1-5331 (`linecov.RunIsolated`, `GLS-V0-056`). The run still had no memory bound. A
loop mutant at `internal/liveverify/jsresolve/jsresolve.go:546` never advances the lexer and keeps
allocating. The kernel OOM-killed `jsresolve.test` at 22 GB of anonymous RSS, and systemd then
stopped every process in the lane's scope (rc=143). The test deadline did not help, because the
host ran out of memory long before it expired.

## Change

- `internal/linecov.RunIsolatedRSS(cmd, maxRSS)` runs a command as `RunIsolated` does, in its own
  group, while a watchdog samples the summed resident set of every process in that group. On
  Linux it reads `/proc/PID/stat` every 50 ms: pgrp is field 5 and rss field 24, and the parse
  resumes after comm's last parenthesis. This needs no privileges and no systemd. Past the limit,
  the watchdog sends SIGKILL to the group and the call returns `*RSSLimitError`, not the exit
  error. `RunIsolated` is `RunIsolatedRSS(cmd, 0)`, with no watchdog.
- Other platforms have no portable, unprivileged group measurement, so `RSSWatchdogEnforced()`
  is false there and only the deadline bounds the run (`pgroup_rss_other.go`).
- `linecov qualify -max-rss SIZE` takes bytes or a `KiB`/`MiB`/`GiB` suffix, defaults to `8GiB`
  and must be positive. The report records `maxRSSBytes` and `maxRSSEnforced`. Where the limit is
  unenforced, qualify says so on stderr.
- `failingTests` returns `errRSSLimit` with the package, the limit and the observed size, and
  reads no output from the killed run. The qualifier's existing rule turns that error into an
  `invalid` mutant with the error as its `detail`. A baseline fails. The interrupt path
  (`GLS-V0-057`) is unchanged.
- The two new sources join `linecov.QualifiedSources`, as `TestQualifiedSourcesBindEveryPackageSource`
  requires.

## Evidence

- `TestRunIsolatedRSSKillsGroupOverLimit` (`internal/linecov`, Linux): `sh` leads the group and
  re-runs the test binary as a grandchild that allocates and touches 1 MiB per step, capped at
  4 GiB. With a 256 MiB limit the run returns `*RSSLimitError` in about 0.2 s, the grandchild is
  gone, and a `sleep` sibling in the test's own group is still alive.
- `TestQualifyRunOverRSSLimitIsInvalid` (`tools/linecov`, Linux): `failingTests` runs a fixture
  module whose test allocates without bound, with a 512 MiB limit. That leaves room for the go
  command, the compiler and the linker in the same group. It returns `errRSSLimit` naming
  `512 MiB limit` and `running p` in about 0.5 s. The test binary is gone and the sibling and the
  caller survive. Negative control: with the watchdog off (`RunIsolatedRSS(cmd, 0)`), the same
  test fails with `got <nil>`, because the fixture reaches its 4 GiB cap and passes.
- `TestRunIsolatedRSSUnderLimitKeepsResult`, `TestRSSWatchdogEnforcedOnlyOnLinux`,
  `TestMaxRSSFlag`.

## Consequence

`tools/linecov/qualify.go` and `internal/linecov/pgroup.go` are qualified sources (`GLS-V0-034`),
so `.github/linecov/qualification.json` stops admitting, as it did after V1-5331. Every plan stays
SHADOW until the owner regenerates the record, which is the fail-closed state. A memory-heavy
suite, if one exists, would need a larger `-max-rss` when the record is regenerated. The default
leaves room for this repository's packages, and every killed run is listed by name in the report.
