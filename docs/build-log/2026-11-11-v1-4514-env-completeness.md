<!-- SPDX-License-Identifier: Apache-2.0 -->
# V1-4514: line narrowing environment completeness (lane linereen)

Ticket V1-4514 (re-enable line-level test narrowing after binding FC and the inherited
environment). Base `origin/main` `b55635262`; spec commit `34b847c30`, implementation `42efe9df0`.
Observed 2026-11-11 on the Unraid lane host (go1.27.3 linux/amd64, 12 CPUs).

## Audit on origin/main, before any change

| AC | State | Evidence |
|---|---|---|
| 1 FC/CC/CXX-style toolchain variables | PARTIAL | `internal/linecov/runner.go` `RunnerID` refuses `UncarriedEnvVars` only when `go env -json -changed` reports them. `FC=x go env -json -changed FC` prints `{}`, so FC never refused, and neither did GO_EXTLINK_ENABLED (also unreported), GCC or the compiler variables cmd/go reads with `os.Getenv` (GOCOMPILEDEBUG, GOCLOBBERDEADHASH, GOSSAFUNC, GOSSADIR). CC, CXX, CGO_*FLAGS, GODEBUG and PKG_CONFIG were refused (`TestRunnerIDRefusesUncarriedEnvironment`). |
| 1 inherited process environment | PARTIAL | A test's `os.Getenv`, `os.LookupEnv`, `os.Environ`, `syscall.Getenv` and `syscall.Environ` reads are bound per record (`GLS-V0-017`, `envaudit.go`, `syscallenv.go`) and compared at selection (`GLS-V0-018`, `decide.go`). A whole-environment read binds `collector-modified` and is always kept. An instrumented Go child's reads are the parent test's (`TestFaultInjection` `e2e.TestTool`). An uninstrumented child makes the record divergent (`LPCV-V0-954`, `collect.go`). Not bound: the Go runtime's own reads of GOTRACEBACK and GOROOT (`runtime.GOROOT`), the dynamic loader's and the C library's (`LD_*`, `DYLD_*`, GLIBC_TUNABLES), and C code's `getenv`. |
| 2 regression tests | PARTIAL | `TestFaultInjection` changes `LINECOV_FIXTURE_*` variables between collection and selection; nothing changed FC, and nothing named `CI` end to end. |
| 3 qualify timeout | MET (no test of the timeout path) | `LPCV-V0-7603`: mutants run under the index's recorded deadline, and zero, unknown and mismatched deadlines are refused (`TestQualificationDeadline`, `qualify_test.go` `test-deadline-mismatch`). Nothing tested what a suite run that times out counts as. |
| 4 frozen held-out evaluation | NOT_MET for this change | The committed `.github/linecov/qualification.json` (seed 20261109, 20 mutants, 0 misses) binds qualified-source blobs, and this change edits `runner.go` and `collect.go`. |

## Decisions

- Refuse rather than model the unreported toolchain variables (`GLS-V0-051`), matching the
  `GLS-V0-009` policy for CC and the like. FC and GO_EXTLINK_ENABLED are read through `go env -json`,
  which also sees a `go env -w` file. The other variables are read from the environment, because
  `go env` prints `""` for them whatever their value.
- Carry the runtime- and loader-read variables (`GLS-V0-052`) instead of refusing them. A host
  that sets GOROOT or LD_LIBRARY_PATH the same way for collection and selection still narrows. An
  unset variable adds nothing, so every identity read without one is byte-identical to before:
  `linecov qualified` at `origin/main` on this host still matches the committed record's runner.
- Bind C code through the whole environment (`GLS-V0-053`) rather than refusing cgo builds:
  `-race` with CGO_ENABLED=1 is the CI runner, and standard-library cgo reads its resolver variables
  through `os`. Corvint has no non-standard C code (`go list -e -deps -test ./...` prints nothing in
  1.2 s), so this changes no Corvint record.
- The timeout path needed no fix (`GLS-V0-054`). go1.27.3 test2json reports a timed-out package
  as `fail` with no result for the running test, so `settle` marks it a crash and every declared
  test counts as failing. A timeout can only add misses.

## Results

Failing first, on `34b847c30` with the tests added and the lists declared but not wired:
`TestRunnerIDRefusesUnreportedToolchainEnvironment` (FC, GO_EXTLINK_ENABLED, GCC, GOCOMPILEDEBUG,
GOCLOBBERDEADHASH, GOSSAFUNC, GOSSADIR, AR, GCCGO and a `go env -w` FC each produced an identity),
`TestRunnerIDBindsRuntimeReadEnvironment` and `TestCollectBindsWholeEnvironmentForCCode` (no
record bound `*`) failed. `TestAffectedLineFCOrCIChangeKeepsStaticPlan` failed only in its FC case
(NARROWED `calc=whole user=exclude` with FC set); its CI cases already passed.
`TestQualifySuiteTimeoutCountsEveryTestFailing` passed: it documents existing behaviour.

After `42efe9df0`, through `script/heavy.sh`, one package per command: `./internal/linecov/` ok
(61.1 s), `./tools/linecov/` ok (18.2 s), `./internal/linecov/watch/` ok,
`./tools/ci-reuse-plan/ -run TestLinecov` ok, `./internal/sessiontest/ -run
'TestRunnerEnv|TestLocalQualification'` ok, and `./cmd/corvint/` with the line-selection `-run` set
ok (15.4 s).

Admission: `linecov qualified -exec test-confine -root . -record .github/linecov/qualification.json`
under `GOFLAGS=-race CGO_ENABLED=1`, with test-confine built as `ci.yml` builds it (SHA-256
`11a8f7e4…c25107`), reports `29 sources match the record` at `origin/main`. At `42efe9df0` it
refuses: `internal/linecov/collect.go … is blob 89055fb4…, the qualification qualified
"1240a7ef…"`. Until the owner regenerates the record, every selection is SHADOW `unqualified`.

## Not done

- AC4, the frozen held-out evaluation: `NOT_DONE`. The record's harness on the 2026-11-09 lane took
  25.4 min of qualify on 32 CPUs after a three-part collection. Its result is admissible only as a
  record this lane must not write. The run needed after merge, from a clean checkout at the merged
  commit: build `linecov` (`go build -o linecov ./tools/linecov`) and test-confine as `ci.yml`
  does; `CORVINT_CONFINE_ROOT=$PWD GOFLAGS=-race CGO_ENABLED=1 linecov collect -root . -packages
  <the record's 20 suite packages> -parallel <nproc> -test-timeout 50m -exec test-confine -out
  part-N.idx` in parts, then `linecov merge -out merged.idx part-*.idx`; `linecov qualify -root . -index merged.idx -packages
  <the record's 17 packages> -suite <the 20> -seed 20261109 -start S -count N -controls 2 -exec
  test-confine -report rep-K.json` over consecutive site ranges; `linecov qualify -summarize
  -record .github/linecov/qualification.json rep-*.json`. Re-enabling narrowing stays the owner's
  step (`CORVINT_LINECOV_NARROWING`, `GLS-V0-034`).
- `.github/workflows/linecov-miss.yml:197` and `main-miss.yml:183` unset only GOMAXPROCS, GOGC,
  GOMEMLIMIT and GORACE before the shell-built runner string with `runtime=""`. They should also
  unset GOTRACEBACK, GOROOT, `LD_*`, `DYLD_*` and GLIBC_TUNABLES (`GLS-V0-052`), so a rerun
  executes under the runtime its runner names. This lane may not edit `.github/**`.
- Children: an uninstrumented child is already divergent. An instrumented child is bound by the
  module-wide listing (`GLS-V0-053`), which is conservative for a module with any C code.
