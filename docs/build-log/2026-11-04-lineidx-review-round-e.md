# Go line selection: review round e fixes

Tickets: V1-4364, V1-4366. Lane base: `6f12f4704235dc8f9bea19fc12995bfb68c746c3`; reviewed head
`c42ee0ed1ea5e7941309669d973ff953311d2cbe`. Observed 2026-11-04 on the WSL lane host (go1.27.3
linux/amd64). Delivery remains experimental; this entry records no promotion or qualification.
This is the lane's last fix round: line narrowing that a test cannot prove sound refuses instead
(`runner-unavailable`, static plan).

## Findings

1. Review P1: collection runs every test under `-test.v=test2json`, but `GLS-V0-020` treated a
   declared `-v=false` or `-json=false` as neutral. At execution those flags follow `corvint test`'s
   `-json`, so go test passes `-test.v=false` after its injected `-test.v=test2json` (verbosity
   off), and `-json=false` drops the injection. A test that calls `F` only when
   `!testing.Verbose()` recorded no coverage of `F` but ran it for real, so a change to `F` could
   exclude a failing test with matching runner identities.
2. Self-review: the runner identity did not carry environment that changes what a test builds or
   executes when it is set in the environment or a GOENV file: `GODEBUG`, the cgo toolchain and
   flags (`CC`, `CXX`, `FC`, `CGO_*FLAGS`, `PKG_CONFIG`) and a `go.work` workspace (`GOWORK`).
3. Self-review: a declared profile `go.env` variable outside the Go settings the identity reads
   (for example `APP_MODE`, `GOMAXPROCS`, `GOGC`, `GO111MODULE`) reaches the tests but not
   collection.

## Decisions

- Fail closed, no modelling of effective verbosity: a declared `-v`, `-json` or `-test.v`, in any
  spelling or value, makes `RunnerEnv` refuse line narrowing (`runner-unavailable`). In GOFLAGS the
  existing non-build-flag refusal already covers them. `-p`, `-failfast` and `-count=1` stay neutral.
- `RunnerID` reads `GOWORK` and refuses any value but empty or `off`, then reads
  `go env -json -changed` for `GODEBUG` and the cgo toolchain variables and refuses any that differ
  from the default. A toolchain that cannot answer `-changed` is `runner-unavailable`.
- `RunnerEnv` refuses a declared `go.env` variable outside `runnerGoEnv` (the Go settings the
  identity reads plus settings that change no executed code: caches, module proxy and checksum
  settings, telemetry) and the GOAMD64-style architecture feature variables.
- Residual limit, not closed: tests can still read arbitrary process environment (`CI`,
  `GOMAXPROCS` set outside the profile, `HOME`) and host properties (CPU count, time, network).
  Collection records the collection host's behaviour. Refusing all process environment would
  disable every narrowing; the spec now lists it under non-goals and the main full-run miss check
  (`GLS-V0-046`, `GLS-V0-047`) remains the backstop.

## Fault injection and fail-before evidence

- `TestRunnerEnvDeclaredVerbosityKeepsInverseVerboseTest` (sessiontest): `TestQuiet` calls `F` only
  when `!testing.Verbose()`; after `F` changes, `-count=1 -v=false` and `-count=1 -json=false` must
  refuse or keep `TestQuiet`, and the real run must fail. Before the fix `Narrow` left `TestQuiet`
  out for both flags.
- `TestCorvintTestDeclaredVerbosityKeepsInverseVerboseTest` (cmd/corvint): the same through
  `corvint test`'s declared flags. Before the fix the refusal was nil and the selection SHADOW.
- `TestRunnerIDRefusesUncarriedEnvironment` (linecov): `GODEBUG`, `CC`, `CXX`, `CGO_CFLAGS`,
  `CGO_LDFLAGS` and a `go.work` path are refused by name; unset and `GOWORK=off` are read. Before
  the fix all six were read.
- `TestRunnerEnvRefusesUncarriedDeclaredEnv` (sessiontest): the CI and repo profile variables are
  kept; `APP_MODE`, `GODEBUG`, `GOMAXPROCS`, `GOGC`, `CC`, `CGO_CFLAGS` and `GO111MODULE` are
  refused by name. Before the fix all seven were accepted.
- Retained under `~/lanes/lineidx-evidence/round-e/` on the lane host.

## Not in scope

- The `tools/linecov qualify` timeout mismatch (`qualify.go:159`) noted by the review is tracked
  separately.
