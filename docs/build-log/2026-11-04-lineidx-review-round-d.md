# Go line selection: review round d fixes

Tickets: V1-4364, V1-4366. Lane base: `6f12f4704235dc8f9bea19fc12995bfb68c746c3`; reviewed head
`c0e745d75ab05ed0a7318c39f8f7b6e86dc1fd0f`. Observed 2026-11-04 on the WSL lane host (go1.27.3
linux/amd64). Delivery remains experimental; this entry records no promotion or qualification.

## Findings

1. Collection ran each test binary without the flags `go test -json` passes it. go test adds
   `-test.paniconexit0`, `-test.v=test2json` and, for a `-timeout` above 0 (default 10m),
   `-test.timeout` (`cmd/go/internal/test/testflag.go`, `test.go`). So `testing.Verbose()` was
   false and `t.Deadline()` reported none while the index was collected, but true and a deadline
   in the runs the index selects for. A test that reaches code only in one of those states was
   recorded without it, and a change there could exclude a test that fails. The `GLS-V0-020`
   claim that `-v`, `-json` and `-timeout` "change no executed code" was wrong.
2. `RunnerEnv` rebuilt GOFLAGS from the process environment alone, so a value saved with
   `go env -w GOFLAGS=...` (for example `-tags=feature`) was lost once a declared build flag was
   added. The identity then named another build than the one go test runs.

## Decisions

- The runner identity names the test runtime as `test="..."` (`GLS-V0-009`). Collection passes
  exactly those flags to every test binary, and `corvint affected --line-test-timeout D`,
  `corvint test` (the profile's `-timeout`) and `tools/linecov collect|select|ci -test-timeout D`
  name the deadline of the runs. An index whose identity has no `test=` field, or another one, is
  a `runner-mismatch`, so every index collected before this change falls back to the static plan
  once. The index format is unchanged.
- `tools/linecov qualify` and `qualified` compare the current build settings under the test
  runtime the index or record names, so a record still says which runtime it qualified.
- The effective GOFLAGS (`go env GOFLAGS`) may carry flags that go test passes to the test binary
  (`-v`, `-short`, `-run`, `-timeout`, `-count=2`, `-cover`, `-failfast`, ...). Collection does not
  apply them, so reading the identity now fails for any GOFLAGS entry that is not a build flag,
  `-p` or `-count=1`, and selection is `runner-unavailable`.
- `RunnerEnv` reads `go env GOFLAGS` under the profile's environment in the module root before it
  adds the declared build flags; a failing read is `runner-unavailable`. `-timeout` is no longer
  neutral: it sets the identity's deadline, and a value that is not a duration is
  `runner-unavailable`.
- CI collects and selects under `-test-timeout 50m` / `--line-test-timeout 50m`, the `-timeout`
  of the go-product shards' `go test -json` runs (separate `.github` commit).

## Fault injection and fail-before evidence

- `TestFaultInjection` gains the fixture package `rt`: `TestLoud` calls `Loud` only when
  `testing.Verbose()` is true, `TestTimed` calls `Timed` only when `t.Deadline()` reports one.
  A panic inserted at each function's entry must keep the failing test. Before the fix both cases
  were a MISS (`rt.TestLoud` and `rt.TestTimed` fail on the mutant but were not kept; kept `[]`).
  After it both are kept; the in-order record keeps the whole package, a conservative superset.
- `TestRunnerEnvKeepsSavedGoFlags`: a GOENV file saving `GOFLAGS=-tags=feature`, GOFLAGS unset in
  the environment and `-race` declared. Before the fix GOFLAGS was `-race`; after it
  `-tags=feature -race`, and the runner identity names `goflags="-tags=feature -race"`.
- `TestRunnerEnvSavedTestFlagOrFailedGoEnvFallsBack` (added in the pre-handback self-review): a
  saved `GOFLAGS=-short` with `-race` declared reaches RunnerEnv as `-short -race`, and the runner
  identity read refuses it by name; with the GOFLAGS check stubbed out the identity was read and
  the test failed. A `go env GOFLAGS` that fails (an invalid GOTOOLCHAIN) refuses the environment.
- Negative controls: `TestRunnerIDBindsTestRuntime` (identities at 10m, 50m and 0 differ and are
  stable, the old format names no runtime, each GOFLAGS test flag is refused by name) and
  `TestAffectedLineTestRuntimeMismatchKeepsStaticPlan` (50m and 0 against a 10m index are
  `runner-mismatch`, `GOFLAGS=-short` is `runner-unavailable`, the default and 10m reach SHADOW
  `unqualified`; the plan is byte-identical in every case).

## Measurements

`tools/linecov collect -packages internal/sessiontest,internal/linecov -parallel 4` on the clean
reviewed tree, two runs each, old collector versus new: 24.6 s and 24.9 s versus 22.1 s and
23.9 s, so verbose mode costs nothing measurable here. The index grows by 65 bytes (the `test=`
field). Focused packages after the fixes: `internal/linecov` 13.1 s, `internal/sessiontest`
3.9 s, `tools/linecov` 23.6 s, `tools/ci-reuse-plan` 9.3 s, `cmd/corvint` (line selection tests)
6.5 s, all passing.

## Limits

- Until this change is on main, the CI selection step builds `corvint` from a main commit that has
  no `--line-test-timeout`, so it refuses and the step stops with the static plan (FALLBACK). Every
  index collected before the change is a `runner-mismatch` until main recollects.

- `tools/linecov qualify` runs each mutant's suite with `go test -json -timeout 2m` (its own
  `-test-timeout`), so verbose mode and the presence of a deadline match the index, but not the
  deadline's value. A test that branches on that value is covered only by the identity check,
  which falls back when the selecting run's deadline differs from the index's.
