# Boost.Test JUnit native profile (experimental, live macOS arm64 witness)

V1-0861 adds `boost-test-junit` as the eighteenth native profile (58 concrete
profiles) and proposes TRE-V0-036..039 in the Runner inventory of
`docs/specs/test-runner-execution-v0.md`. Decision 0437, which answered
V1-0642, accepted Boost.Test as a CEM 1.0 main framework under V1-0593. This
entry is base `2a93b5a182966f72836cd28bf569dc44c0f5220c`.

## Format decision

Three Boost.Test 1.92.0 machine-readable outputs were probed live on the same
fixture. The detailed XML report (`--report_format=XML --report_level=detailed`)
omits disabled cases and only counts them, so a disabled case has no identity.
The XML log at `error` level carries errors only, with no per-case status. The
JUNIT log sink writes one `testcase` per case, including disabled and
filtered-out cases as `<skipped/>`, with the suite path as `classname`. It is
the one fixed format. Repeated `--run_test` arguments were a native setup error
(exit 200, empty sink), so selection uses one colon-joined filter.

## Acquisition

`https://archives.boost.io/release/1.92.0/source/boost_1_92_0.tar.bz2` was
downloaded into lane scratch only: 199030664 bytes, SHA-256
`5c1d40cb8e19adbf740a4ec2da35b3e58f3f5804b1dce44deb53df72193cbc6c`. This
matches the archive's published `.json` (commit
`afdfa32505af73e3d208144b3f623f0096cb62b6`) and the boost.org 1.92.0 release
page. Only `boost/` and `LICENSE_1_0.txt` were extracted; `version.hpp` reads
`BOOST_VERSION 109200`. The fixture was built with `/usr/bin/c++` (Apple clang
21.0.0, arm64) against the header-only `boost/test/included/unit_test.hpp`.
Nothing was installed outside lane scratch.

## Evidence

- Failing before: at the base, `TestBoostTestRegistryDispatch` fails because the
  registry lists no `boost-test-junit` runner (count 0).
- Recorded fixtures: `native/testdata/boosttest-provenance.json` retains
  the fixture source and nine actual runs (bytes, exit, stdout, stderr). They
  cover pass, failure and disabled (exit 201); pass and disabled (0);
  suite-fixture failure (201, `math-setup-teardown` pseudo row); uncaught
  exception (201, `errors=1`); a selected run (0); a selected disabled case,
  which Boost enables and runs (201); a no-match filter (200, empty sink); two
  failed checks in one case plus a failed `BOOST_REQUIRE` (201, two `failure`
  entries in one row, `errors=1` for the aborted REQUIRE case); and a failed
  check followed by an exception (201, `failure` and `error` in one row).
- Passing after: `TestBoostTestRecordedJUnitWitnesses`,
  `TestBoostTestBoundaryContradictions` (40 mutations, each incomplete with
  public `UNKNOWN`), `TestBoostTestClosedBuild` and
  `TestBoostTestRegistryDispatch` pass.
- Live: `CORVINT_BOOST_ROOT=<scratch>/boost_1_92_0 go test -run
  TestBoostTestLiveExecution ./internal/testrunner/native` compiles the
  pinned fixture and runs it through `tr.Execute`. It passed. The complete
  cases are pass/fail/disabled at exit 201, a selected run at exit 0, and
  multiple and fatal assertions at exit 201. The incomplete cases all left
  every public state `UNKNOWN`: missing expected case, suite-fixture failure,
  uncaught exception, check then exception, and no-match exit 200.

## Independent review

Codex (`gpt-6-astra`, read-only) found two P2 defects in the first commit, both
fail-closed (`UNKNOWN`, never a pass). First, a case with several failed checks
was refused, because Boost writes one `failure` per assertion. Second, a failed
`BOOST_REQUIRE` raised a counter mismatch, because Boost counts it as aborted in
`errors`. Both were confirmed live and fixed. The parser now accepts several
outcome entries per row and counts aborted rows, and new native fixtures cover
both cases.

## Non-goals

No Boost installation, build integration, CMake/CTest discovery, shared or
static Boost.Test library variant, expected-failure accounting, data-driven or
template-case identity, `depends_on` qualification, or Tasks/CEM integration.
No shared executor or other profile changes.

## Failure modes

A Boost release that changes the JUnit grammar, counters or pseudo-row names
refuses or yields incomplete observations rather than passes. Name normalization
(`/` to `.`, space to `_`) can make two native names collide; a duplicate
identity refuses the report. Selecting a disabled case runs it, and non-selected
cases are indistinguishable from natively disabled ones in the sink.

## NOT_RUN

Other compilers (GCC, MSVC), Linux and Windows, the library variants, Boost
versions other than 1.92.0, native `timeout` decorators, and executor-level
timeout or interruption of a Boost binary. The repository-wide gate was not run
(lane policy); focused packages and doc gates only.

## Rollback

Remove `native/boosttest.go`, its tests and provenance file, the runner
from `Runners()`, `Build`, `Parse` and `failureExits`, and the spec subsection.
Historical receipts are untouched.
