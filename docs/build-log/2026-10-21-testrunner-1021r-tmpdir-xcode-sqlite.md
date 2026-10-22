# 2026-10-21 testrunner-1021r: private TMPDIR removal, Xcode destination-less summaries, SQLite rejected queries

Lane `testrunner-1021r`, base `ef2a01db52d5436c262725d136060c9eb95e8580`. Tickets V1-1538,
V1-2182 and V1-1567 delivered; V1-1543 skipped. Requirements `TRE-V0-116`, `TRE-V0-117` and
`TRE-V0-118` in `docs/specs/test-runner-execution-v0.md` are accepted under the owner's standing
auto-accept of lane requirements (2026-10-11).

## Problem

- V1-1538: after a timeout, the executor's private `TMPDIR` (`<reportDir>/.tmp`) kept native
  residue: .NET MTP `clr-debug-pipe-*` FIFOs, plus `go-build*`, `rustdoctest*` and Xcode
  `TemporaryDirectory.*` directories in the 2026-10-17 matrix runs.
- V1-2182: the `xcode-xctest` `zero` and `build-error` witnesses were refused as
  `INVALID_NATIVE_REPORT` ("destination reports disagree"), because their summaries name no
  destination.
- V1-1567: `sql-sqllogictest-sqlite` recorded an engine-rejected query as a complete FAILED
  suite, as if it were an assertion mismatch.

## Decisions

- `Execute` removes the whole `.tmp` with `os.Root.RemoveAll` in a defer that runs after device
  cleanup and report reads. Residue that will not go is the execution problem `temp-cleanup`,
  never hidden. The private HOME and process retirement are unchanged.
- Xcode: only two exact destination-less shapes are mapped. One is an empty tree with exit 65
  and native `The following build commands failed:` stderr, which gives `BUILD_FAILURE`. The
  other is one empty `Test Plan` node with exit 0, which gives `NO_TESTS` through the `Parse`
  empty-inventory rule. A build failure has no test row, so the `BUILD` failure kind cannot
  carry it. It therefore reuses the existing native Go problem code `BUILD_FAILURE` instead of a
  new code.
- SQLite: V1-1567 assumed that no native signal separates a rejected statement from a
  mismatch. That assumption is wrong. The committed captured streams (`db57eba9`, SQLite 3.53.0)
  print `case.slt:1: query failed` on stderr for a missing relation, and
  `case.slt:4: wrong result - expected [2] got [1]` on stdout for a mismatch. A file-anchored
  `query failed` line records `SQL_QUERY_REJECTED`, so the observation is incomplete.
  `TestNativeMatrix` now expects `sqlite/infra` to be incomplete.
- The platform matrix legend now says that an A/H cell is witness existence, not
  classification, and names the sqllogictest B cell explicitly.
- V1-1543 is skipped. The TestCafe tree under `toolchains-2026-10-17/testcafe/node_modules` has
  no `package.json` in any of its 264 packages, so TestCafe cannot start. The
  `remote-cft154-noconnect` evidence keeps only a Chrome profile, with no console or command.
  Reproducing it needs an npm reinstall from the network, which is outside this lane.

## Evidence

- `TestExecuteRemovesPrivateTempResidue` (timeout and exit subtests): fails without the removal
  ("private TMPDIR residue survived"), passes with it. It ran together with the retirement and
  environment executor tests.
- `TestXcodeBuildFailureAndZeroTests`: the byte-exact 2026-10-17 fixtures give `BUILD_FAILURE`
  and `NO_TESTS`, and four shape mutations stay `INVALID_NATIVE_REPORT`. Without the mapping it
  fails with "Xcode destination reports disagree".
- `TestSQLiteRejectedQueryIsNotAnAssertion` and `TestNativeMatrix`: fail without the check.

## NOT_RUN

- Live re-runs: MTP timeout, Xcode `zero` and `build-error`, and sqllogictest. The sqllogictest
  runner and source were purged.
- The native text for a rejected `statement ok` record was not witnessed and is not detected.
- Tier 2 affected units are left to batch CI.

## Rollback

Revert the `.tmp` defer and `boundedDetail` in `internal/testrunner/execute_unix.go`, the
destination-less switch in `internal/testrunner/platform/xcode.go`, and the `query failed`
check in `internal/testrunner/sql/sqllogictest.go`. Revert their tests, fixtures and spec text
with them.
