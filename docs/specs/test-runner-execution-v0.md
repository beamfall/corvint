# Test runner execution V0

Owner: Russell Lewis
Date: 2026-10-01
Intent status: proposed; TRE-V0-021..023 accepted (decision 0454; V1-0620)
Delivery status: experimental
Authoritative inputs: owner instructions 2026-10-01, native V1-0591..0596,
`docs/specs/live-proof-carrying-verification-v0.md`, `docs/specs/go-live-test-provider-v0.md`.

## Agent digest
- Claim: Optional fixed runner profiles execute trusted local tests and retain bounded native reports with explicit qualification limits.
- Status: proposed; TRE-V0-021..023 accepted (decision 0454; V1-0620); technical profile/experimental prototype; accepted owner target is all main test runners; no stable CEM1.0 promotion.
- Exists: 59 concrete experimental profiles in `internal/testrunner`; implementation and live qualification are tracked separately.
- Blocked on: every runner's actual runtime/platform qualification and CEM/Tasks integration.
- Read next: Requirements; Runner inventory; Acceptance and rollback.

## User and measurable job

A reviewer of a mixed-language change needs actual test execution, with the main runners for every
supported language. Static selection and imported result files are useful evidence but do not fulfill
execution support. All existing affected-runner registrations remain in scope. The sixteen candidate
analyzer families need explicit runtime/domain disposition; their presence is not qualification.

## Requirements

- `TRE-V0-001`: Adapters MUST declare explicit runner/runtime/report profiles. Fixed invocation
  construction and native parsing MUST be separate pure operations; unknown runners refuse. Execution
  requires independently admitted executable identities and exact plan approval, never discovery alone.
- `TRE-V0-002`: Inputs MUST identify the source snapshot, selectors, configuration, executable/tool
  digests and report inventory. Dependencies, application and device identities MUST be qualified or
  explicitly unknown. Hashing the primary executable alone cannot attest its children or toolchain.
- `TRE-V0-003`: A bounded invocation MAY declare BUILD, DISCOVER, TEST and DECODE phases, each using
  an independently pinned tool. Every phase and original observation MUST remain inspectable. A failed
  test may still decode its report; build/collection/decode errors never count as assertion failures.
- `TRE-V0-004`: Profile-owned reporter/configuration files MUST be fixed bytes, digest-bound and
  written only beneath a fresh caller-selected report directory. No arbitrary shell hook, implicit
  dependency download or plan-provided executable may run. Configuration is trusted local code, not
  accepted intent or operating-system containment.
- `TRE-V0-005`: Report paths/patterns MUST resolve deterministically beneath that new directory;
  symlinks, traversal, special files, duplicate or missing expected shards and absent reports refuse.
  Inputs cap at 32 reports, 4 MiB per report, 4096 tests and 32 attempts per test.
- `TRE-V0-006`: Native test/suite/project/parameter identity, status and all reported retries MUST be
  retained. Skip/disabled/no-tests, unrecognized states, contradictory counts/exit, incomplete suites,
  collection/setup/build/infrastructure failures, timeout, interruption and overflow cannot become pass.
  Unreported retry details remain NOT_REPORTED, never an invented single attempt.
- `TRE-V0-007`: Parsing MUST verify native completion/count semantics and reject ambiguous identities.
  Report completeness describes observed native inventory, not requirement coverage or test adequacy.
  Expected selector coverage MUST be checked separately; one language's pass cannot hide another gap.
- `TRE-V0-008`: Execution MUST enforce bounded time/output and retire owned ordinary descendants on
  exit, timeout, overflow and interruption. Trusted local runner code is not an OS security sandbox;
  hostile detached sessions and unqualified device/app authority remain explicit limits.
- `TRE-V0-009`: Each supported tuple MUST have actual runner-generated fixture evidence for pass,
  assertion failure, skip, zero tests, collection/build/infra error, retries when supported and cleanup.
  Parser tests or generic JUnit import do not qualify execution. Missing runtime/device tuples remain
  release blockers; they cannot be silently removed from the accepted owner scope.
- `TRE-V0-010`: Provider-neutral receipts MUST compose with canonical CEM hunks and Tasks criterion,
  acceptance, generation and candidate identities. FAILED alone MUST NOT establish a killed control;
  exact reviewed assertion/control relevance remains a separate experiment obligation. Default Core,
  frozen CEM0.x and Tasks policy remain unchanged until their own versioned migration is delivered.
- `TRE-V0-110`: (accepted under the owner's 2026-10-11 standing lane auto-accept; V1-1541) For
  `minitest` and `rails-test`, a test whose native result carries a `Minitest::UnexpectedError`
  MUST be `FAILED` with failure kind `UNKNOWN` (the cause is an exception, not an assertion) and a
  message naming only the exception class (`UnexpectedError: <Class>`, at most 1024 bytes; the
  exception text is not retained). Other cases keep their native outcomes and the run stays
  complete. A skipped or passed result is never reclassified.
- `TRE-V0-111`: (accepted under the owner's 2026-10-11 standing lane auto-accept; V1-1537) The
  `ctest` profile MUST run CTest with `--test-dir` set to a run-owned `ctest-run/` directory in the
  report directory, whose generated `CTestTestfile.cmake` only includes the caller's build tree
  (`subdirs("<Root>")`). CTest's own bookkeeping (`Testing/`, including `Testing/Temporary/`
  `LastTest.log`, `LastTestsFailed.log` and `CTestCostData.txt`) is not written into the build
  tree and stays in the report directory evidence (`ctest-run/Testing/Temporary/`). Tests still run
  from their own build directories, so files a test itself writes there are outside this
  requirement. A `Root` that is not absolute or holds a character CMake would
  re-parse inside a quoted argument (`"`, `\`, `$`, `;`, NUL, CR, LF) is refused, never quoted.
- `TRE-V0-113`: (accepted under the owner's 2026-10-11 standing lane auto-accept; V1-1537 review)
  The run-owned test directory MUST keep the top-level configuration CTest reads from its test
  directory. Its `CTestCustom.cmake` includes the build tree's `CTestCustom.cmake`, or else its
  `CTestCustom.ctest` (CTest's precedence), so pre- and post-test hooks still run and a failing hook
  still fails the run. The build tree's `DartConfiguration.tcl` (default test `TimeOut` and other
  test settings) is copied with only its `BuildDirectory` entry pointed at the run directory, which
  keeps CTest's bookkeeping out of the build tree. A `DartConfiguration.tcl` that cannot be read, or
  that has other than exactly one `BuildDirectory` entry, is refused rather than dropped. Paths reach
  CMake with forward slashes, so a Windows `Root` such as `C:\build` is admitted, and the
  metacharacter refusal applies to the converted path.
- `TRE-V0-112`: (accepted under the owner's 2026-10-11 standing lane auto-accept; V1-1535) The
  `swift-testing` profile MUST refuse, before execution, a package directory (`Root`, or `Root`
  joined with `Project`) whose symlink-resolved base name is case-insensitively a reserved
  toolchain package identity, because SwiftPM derives the identity from the resolved directory.
  A package directory that cannot be resolved is refused. The reserved set holds only the observed
  collision, `swift-testing`. Other names, including `Testing` and a symlink named `swift-testing`
  that resolves elsewhere, are admitted.
- `TRE-V0-114`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0605) A
  `testcafe` observation whose native stdout or stderr holds a line naming
  `.testcafe-browser-tools` together with a Node system-error code (`E` and three or more capital
  letters, then `:`) MUST record `browser-helper-failed` with that native line (trimmed, at most
  240 bytes, valid UTF-8) as its detail, so the observation is incomplete even when the native
  JSON report and exit claim a pass. A run that aborted without a report keeps `missing-report`
  as well. The executor MUST NOT create, copy into or repair the helper directory: first-use
  helper setup in a fresh HOME is an unsupported tuple, and a caller that needs one provisions
  the helper in its own task-owned HOME before the run.
- `TRE-V0-115`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2156) For
  `unittest` and `test-unit`, a named test (or `unittest` subtest) whose native result is an error,
  not an assertion failure, MUST be `FAILED` with failure kind `UNKNOWN` and a message naming only
  the exception class (`Error: <Class>`, at most 1024 bytes; the exception text is not retained),
  as `TRE-V0-110` does for `minitest` and `rails-test` (`UnexpectedError: <Class>`). A `unittest`
  class or module fixture error, which names no single test, stays `UNKNOWN`. Other cases keep
  their native outcomes and the run stays complete.
- `TRE-V0-116`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1538) The
  executor's private `TMPDIR` (`<reportDir>/.tmp`) is executor scratch, never a report. `Execute`
  MUST remove it on every return path, after process retirement, device cleanup and report reads,
  including timeout, interruption and execution-boundary failures. Residue that cannot be removed
  MUST be recorded as the execution problem `temp-cleanup` (detail at most 1024 bytes), which
  makes the observation incomplete. Process retirement, the private `HOME` and every report
  stream are unchanged.
- `TRE-V0-117`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2182) For
  `xcode-xctest`, a summary with no destination, result `unknown`, all counts zero and no
  failures MUST map by its exact native shape: an empty test tree with `xcodebuild` exit 65 and
  the native stderr block `The following build commands failed:` MUST record `BUILD_FAILURE` (no
  test row, observation incomplete); a single empty `Test Plan` node with result `unknown` and
  exit 0 MUST record `NO_TESTS`. Any other destination-less shape, including either shape with a
  different exit or without that stderr block, stays `INVALID_NATIVE_REPORT`.
- `TRE-V0-118`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1567) For
  `sql-sqllogictest-sqlite`, a native diagnostic line `<sourceFile>:<line>: query failed` (the
  engine rejected or could not execute a query record) MUST record `SQL_QUERY_REJECTED` with that
  line as detail, so the observation is incomplete and its suite verdict is not retained. A
  native result mismatch (`wrong result ...`) stays a complete `FAILED` suite. A diagnostic that
  names another file does not match.
- `TRE-V0-119`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2341) For
  `sql-sqllogictest-sqlite`, a native stderr line `<sourceFile>:<line>: statement error` MUST record
  `SQL_STATEMENT_UNRESOLVED` with that line as detail, so the observation is incomplete and its
  suite verdict is not retained. The native runner prints that line both for a `statement ok`
  record the engine rejected and for a `statement error` record it accepted, so the streams cannot
  separate a rejection from an assertion mismatch. A diagnostic that names another file does not
  match.
- `TRE-V0-120`: (experimental under the standing lane auto-accept; V1-2589) When a test fails and its
  captured output shows `exec: "<tool>": executable file not found in $PATH`, the shared boundary MUST
  record `undeclared-tool` naming the tool, so the observation is incomplete and the verdict is an
  environment problem (the tool must be declared in the request tools), not a test failure. A passing
  test, or a failure without that message, is unchanged.
- `TRE-V0-820`: (experimental under the standing lane auto-accept; V1-1554) The Android
  instrumentation profiles (`android-junit`, `espresso`, `uiautomator`) MUST bind the installed
  APKs: the request's `inputFiles` MUST name one or two `.apk` files (the test APK, and the app
  APK unless the instrumentation targets its own package), each with its SHA-256, and `Build`
  MUST refuse a request without them. The single TEST phase MUST run one fixed device shell
  script that, before the instrumentation starts, resolves the instrumentation's target package
  from `pm list instrumentation`, reads `pm path` for the test package and the target package,
  requires exactly one `package:` path for each (a split install is unobservable), hashes each
  installed APK with `sha256sum`, and requires the set of installed digests to equal the set of
  bound `.apk` digests. Only then does it print the fixed line `INSTALLED_APK_IDENTITY: verified`
  and run `am instrument`. Otherwise it prints `INSTALLED_APK_IDENTITY: <reason>` (`unresolved`,
  `unobservable`, `mismatch`) and exits without instrumenting. The parser MUST require the
  verified line as the first stdout line and MUST record `INSTALLED_APK_UNVERIFIED` with the
  reason otherwise, so the observation is incomplete. After `am instrument` returns, the same
  script MUST hash the installed APKs again and print one fixed last line:
  `INSTALLED_APK_IDENTITY: unchanged` only when the second digest set equals the first, otherwise
  `INSTALLED_APK_IDENTITY: changed` (or `unobservable` when the second hash cannot be taken). The
  parser MUST require `unchanged` as the last stdout line and MUST record
  `INSTALLED_APK_UNVERIFIED` otherwise, so a reinstall during the run (`adb install -r`) makes the
  observation incomplete even when every test passed. Because the parser checks fixed lines, an
  adb build that drops the remote exit status cannot pass a mismatch or a change.

## Runner inventory

The dynamic lane covers the fifteen JS/TS affected registrations: Vitest, Jest, AVA, Mocha, node:test,
Playwright, Bun, Deno, Cypress, WebdriverIO, TestCafe, Nightwatch, Detox and both Storybook runners;
Python pytest/unittest; Ruby RSpec/Minitest/Test::Unit/Rails. Jasmine, accepted as an addition by
decision 0437, is one further fixed dynamic profile without an affected registration (see Jasmine
(experimental)). The native lane covers Go, Cargo/nextest
and doctests, .NET xUnit/NUnit/MSTest with separate VSTest/MTP profiles, CTest/GoogleTest/Catch2, CMocka, Boost.Test and Ginkgo v2.
The platform lane covers Java JUnit/TestNG/Gradle/Maven, Kotlin kotlin-test/Kotest/Android families,
Swift Testing/XCTest/SwiftPM/Xcode, Bats/ShellSpec and remaining domain runner inventory. Analyzer
candidates never imply that SQL/shader/data compilation is assertion testing. Additional main-runner
ambiguity needs explicit owner disposition; absence of evidence is not an approved exclusion. Decision 0437 closes the CEM 1.0 main-framework set.

The concrete count is 22 dynamic, 18 native, 16 platform, two SQL and one Appium Android profiles. The registry also
lists six unavailable IDs: `appium`, `pgtap`, `sqllogictest`, `shader-behavior`, `html-behavior` and
`structured-data-behavior`. The two SQL profiles use the explicit IDs `sql-pgtap` and
`sql-sqllogictest-sqlite`; the older unqualified names do not silently alias them. SQL coverage does
not resolve arbitrary Appium backends or behavioral qualification for shaders, HTML and structured data. The latter
need an actual domain/application assertion harness, not syntax validation presented as tests.

Cypress 16.1.1 and Rails 8.1.4 now have shared-executor runtime witnesses. MTP has separate
xUnit/NUnit/MSTest profiles; all 18 native profiles have macOS arm64 shared-executor witnesses for every TRE-V0-009 case (Native qualification matrix 2026-10-17 (V1-0593)). SwiftPM 6.4 macOS serial
XCTest and Robolectric 4.16.1/API35 also have shared-executor witnesses. These do not erase remaining
limits: TestCafe's current common-executor remote-Chrome CLI witness is qualified in
`/private/tmp/cem10-build/dynamic/testcafe-cli-proof/qualification.json`; its prior permission
refusal and first-use helper failure remain retained. Detox lacks a qualified task application/device;
legacy Storybook collection is witnessed (a complete FAILED run, `storybook-test-runner/collection`,
Dynamic qualification matrix 2026-10-17 (V1-0592); the earlier runtime-incompatibility block is
superseded). Playwright now retains actual pinned Chrome DOM pass/fail/skip/retry/zero/collection cases;
a timeout initially left its native detached browser group alive (V1-0608), with durable cleanup
repair independently reviewed and locally qualified for native cooperating Playwright; TRE-V0-034 adds
executor-owned retirement of escaped descendants for every profile. The prior no-browser fixture remains retained. Nightwatch 3.16.0 now has a real
Chrome for Testing 154 session witness (see Dynamic qualification matrix 2026-10-17 (V1-0592)). Other OS versions, retries, parameterized cases and lifecycle negatives remain open
where their adapter provenance says so.

### Boost.Test JUnit sink (experimental)

`boost-test-junit` reads one caller-built Boost.Test executable (V1-0861, an
addition under V1-0593 accepted by decision 0437, which answered V1-0642).
The pinned reading is Boost 1.92.0, official archive
`https://archives.boost.io/release/1.92.0/source/boost_1_92_0.tar.bz2`
(199030664 bytes, SHA-256
`5c1d40cb8e19adbf740a4ec2da35b3e58f3f5804b1dce44deb53df72193cbc6c`, release
commit `afdfa32505af73e3d208144b3f623f0096cb62b6`, `BOOST_VERSION 109200`,
BSL-1.0). The pins, fixture source and actual runner-generated reports are in
`internal/testrunner/native/testdata/boosttest-provenance.json`.

The format is the JUNIT log sink. Boost's detailed XML report omits disabled
cases (it only counts them), and the XML log at error level has no per-case
status, so neither retains a stable identity for every outcome. The JUnit sink
emits one row per case, including disabled and filtered-out cases, with the
suite path as `classname`. Stdout parsing is not used.

- `TRE-V0-036`: `boost-test-junit` MUST execute the pinned test executable with
  exactly 11 fixed argv elements: `--log_format=JUNIT`, `--log_level=error`,
  `--log_sink=<ReportDir>/boost-junit.xml`, `--report_level=no`, `--random=0`,
  `--result_code=yes`, `--build_info=no`, `--color_output=no`,
  `--show_progress=no`, `--catch_system_errors=yes` and `--auto_start_dbg=no`,
  with an empty phase environment. `Target` is the master test suite (module)
  name, one identifier of at most 128 bytes. Expected identities are 1..4096
  unique `Target::suite/.../case` paths of one to eight identifier segments.
  Selectors MUST be unique expected identities and add exactly one 12th element,
  `--run_test=<path_1>:<path_2>...`, bounded at the shared 4096-byte argv limit;
  repeated `--run_test` arguments are a native setup error and are never
  emitted. Build refuses project, configuration, reporter, report-file and
  auxiliary-tool overrides. Success exit is 0 and failure exit is 201
  (`exit_test_failure`); 200 (`exit_exception_failure`) is not admitted.
  Proposed (V1-0861).
- `TRE-V0-037`: The dedicated parser MUST accept exactly one `boost-junit.xml`
  report within the per-report bound, decoded as a closed tree: one optional XML
  declaration, one `testsuite` root with exactly `tests`, `skipped`, `errors`,
  `failures`, `id="0"`, `name` and `time`, depth at most three, bounded nodes, no
  namespaces, duplicate attributes, comments or directives. `name` MUST equal
  Target. Each `testcase` has exactly `assertions`, `name`, `time` and an
  optional `classname`, and either one `skipped` (no attributes or text) or any
  number of `failure`/`error` entries with exactly `message` and `type` (Boost
  writes one per failed assertion), plus optional `system-out`/`system-err`. The identity is Target, `::`, then the `classname`
  segments split on `.` and the case `name`, joined with `/`; a non-identifier
  segment or duplicate identity refuses the report. Proposed (V1-0861).
- `TRE-V0-038`: A row without an outcome is `PASSED`; `skipped` is `SKIPPED`; a
  row whose entries are all `failure` with type `assertion error` or `fatal
  error` is `FAILED` with cause `ASSERTION`. These make the observation incomplete, so shared `Normalize` sets
  every public state to `UNKNOWN`: an `error` row (uncaught exception, timeout,
  system or user error; `BOOST_ERROR_ENTRY`), any other failure type
  (`BOOST_FAILURE_TYPE`), a synthetic `-setup-teardown` or `-timed-execution`
  row (`BOOST_SUITE_FIXTURE_FAILURE`), suite-level log output
  (`BOOST_RUNNER_LOG`), a row outside the expected inventory
  (`BOOST_SURPLUS_TEST`), an absent expected identity (`BOOST_MISSING_TEST`), a
  non-selected row that was not skipped while selectors exist
  (`BOOST_UNSELECTED_EXECUTED`), no rows (`BOOST_NO_TESTS`), counters that
  disagree with the rows (`tests` = non-skipped rows, `skipped` = skipped rows,
  `errors` = aborted rows, meaning rows with an `error` entry or a `fatal error`
  failure, and `failures` = failed rows minus aborted rows;
  `BOOST_COUNT_MISMATCH`), exit 0 with a failed row or 201 without one
  (`BOOST_EXIT_REPORT_CONTRADICTION`), any other exit
  (`BOOST_EXIT_UNSUPPORTED`), and any grammar refusal (`BOOST_INVALID_REPORT`).
  A failed, skipped or unreported test is never a pass. Retry information is
  `NOT_REPORTED`. Proposed (V1-0861).
- `TRE-V0-039`: Maintained tests MUST replay the actual Boost 1.92.0 JUnit
  bytes retained in the provenance file (pass, assertion failure, disabled,
  suite-fixture failure, uncaught exception, selected run, selected disabled
  case, empty no-match sink, several failed checks and a failed `REQUIRE` in
  one run, and a failed check followed by an exception) and mutate them into every contradiction above.
  An opt-in live test (`CORVINT_BOOST_ROOT`) MUST compile the pinned fixture
  against a `BOOST_VERSION 109200` header tree and run it through the common
  executor. Proposed (V1-0861).

Recorded limits. JUnit names are normalized by Boost (`/` becomes `.` and space
becomes `_`), so identity is the normalized identifier path; a native name
outside the identifier grammar, such as a template or manually registered case
with punctuation, refuses the report rather than being guessed. Explicitly
selecting a disabled case enables and runs it; non-selected and natively
disabled cases are both reported as disabled skips and cannot be told apart.
Expected-failure (`expected_failures`) accounting, data-driven and template test
cases, `depends_on` chains and timeouts are not qualified; they fall into the
incomplete rows above. Exit 200, including a no-match filter and setup errors
with an empty or absent sink, is incomplete. Timeout and interruption are owned
by the shared executor (`PROCESS_INCOMPLETE`); no Boost-native timeout was
observed. Only Apple clang 21.0.0 on macOS arm64 with the header-only
`boost/test/included/unit_test.hpp` variant was observed; the shared-library
and static-library variants and other compilers or platforms are NOT_RUN.

Acceptance evidence: `TestBoostTestRecordedJUnitWitnesses`,
`TestBoostTestBoundaryContradictions`, `TestBoostTestClosedBuild`,
`TestBoostTestRegistryDispatch` and opt-in `TestBoostTestLiveExecution`, which
passed live on 2026-10-08 against the checksum-verified archive (build log
`2026-10-08-boost-test-profile.md`). Rollback removes the additive
`boost-test-junit` profile, its tests and provenance file; no shared executor,
other profile, queue, store or frozen wire changes.

## Acceptance and rollback

Parallel worker evidence lives in private task scratch paths with exact versions and raw reports.
The real SwiftPM 6.4 parallel xUnit witness reports a skipped XCTest as passed; accepting that format alone
is disqualified. The separate serial macOS native-text profile validates lifecycle, skip-body and
failure-event counts, including setup exceptions and multiple assertions in one method. Stable Cargo JSON is not inferred from nightly-only libtest. Existing report parsers
may be reused only under their actual binding and status semantics.

Focused package tests, actual available runner invocations, independent cross-review and CEM
bind/check/seal are required for the prototype. Full CEM1.0 release qualification stays V1-0596;
no feature is promoted by this document. Rollback removes the optional runner adapter/companion and
preserves historical receipts. No queue, store, existing source tree or frozen wire is overwritten.

## Traceability

Experimental implementation and partial actual qualification are retained in V1-0592..0594. The
following evidence supports the prototype, not stable acceptance or automatic ticket completion.

| Requirements | Source/tests | Retained scoped review or actual evidence |
| --- | --- | --- |
| TRE-V0-001, 004 | Adapter `Build`/`Parse`; `cmd/corvint-test-runner` `TestClosedRequestAndPlanAdmission`, `TestActualGoPlanRunReceipt` | `runner-cli-rereview.json`: independently supplied tools, exact plan/template and fresh receipt reservation PASS |
| TRE-V0-002..003, 005 | `execute_unix.go`; `TestExecuteIndependentBindings`, `TestExecuteFixedTemplatesAndReportInventory`, `TestBoundedReportEnumeration` | `shared-executor-rereview.json`: fixed phases/templates and bounded rooted report inventory PASS |
| TRE-V0-002, 005 | `TestLargePinnedArtifactSeparateFromSourceBound` | `large-artifact-review.json`: streamed config/reporter pins up to 256 MiB; source/report bounds remain 4 MiB per file; no transitive dependency claim |
| TRE-V0-006..007 | Adapter native-format and contradiction tests; `validate_test.go` `TestNormalizeBoundaryAndExitProfiles`, `TestAggregateInventoryDoesNotInventExecutedCases`; `native_test.go` `TestGoActualBuildFailureStream` (captured go1.27.2 build-fail stream, V1-1375) | Dynamic/platform reviews and `native-trx-final-review.json` PASS; native retry/skip/count/error evidence retained |
| TRE-V0-008 | `TestExecuteRetiresDescendants`, `TestExecuteEnvironmentAndArtifactBounds` | Shared executor review PASS for qualified ordinary process groups; hostile detached sessions and other OS containment unqualified |
| TRE-V0-009 | Adapter READMEs, `testdata` provenance, opt-in `TestMTPLiveExecution` and `TestSQLLiveExecution` | PARTIAL live matrices; current JVM/SwiftPM/Robolectric requalification in `platform-latest-execution-proof.jsonl` and `jvm-latest-execution-proof.jsonl` |
| TRE-V0-010 | Separate criterion experiment, Tasks native boundary and CEM candidate references | Cross-language native claim/source/runner join and semantic discrimination remain NOT_PRODUCED |
| TRE-V0-011..012 | `sql/sql.go`, `pgtap.go`, `sqllogictest.go`; `TestNativeMatrix`, `TestTAPRefusals`, `TestSQLiteRefusals`, `TestFixedInvocations`, `TestClosedConnection` | `sql/review.json` PASS; ten actual shared-executor receipts, native format provenance and disposable cluster cleanup retained |
| TRE-V0-015 | `execute_unix.go`; `TestExecuteExplicitPrimaryTestWithoutArguments` (pinned `pwd` with no argv; implicit, ambiguous, BUILD, DISCOVER, DECODE, auxiliary and implicit-primary-name refusals) | `cmocka-independent-review-r2/REVIEW.json` PASS_BOUNDED (F4); root decision `e4de2129…e161` ACCEPTED_WITH_CONDITIONS |
| TRE-V0-016..017 | `native/cmocka.go`; `TestCMockaActualDualFormatWitnesses`, `TestCMockaBoundaryContradictions`, `TestCMockaClosedBuild`, `TestCMockaRegistryDispatchAndTargetBoundary`, `TestCMockaPlanBindsTargetAndFixedEnvironment`, opt-in `TestCMockaNativeReceiptReadback` | Nine actual macOS arm64 CMocka 2.0.2 receipts in `cmocka-profile-proposal-r1/proof`; review PASS_BOUNDED; fresh requalification on the integration base NOT_RUN |
| TRE-V0-018..020 | `native/ginkgo.go`; `TestGinkgoNativeProjectionStates`, `TestGinkgoReportCountAndBound`, `TestGinkgoParserRefusals`, `TestGinkgoStrictJSON`, `TestGinkgoBeforeSuiteSkipShape`, `TestGinkgoOrderedFollowOnFailure`, `TestGinkgoClosedBuild`, `TestGinkgoFocusBound`, `TestGinkgoRegistryDispatchAndTargetBoundary`, `TestGinkgoPlanBindsTargetAndFixedArgv` | Synthetic source-derived reports; Ginkgo v2.33.0 read pins in `native/testdata/ginkgo-provenance.json`; live runtime qualification: 10 actual witnesses on the 2026-10-17 host tuple (2026-10-17, see "Ginkgo v2 live qualification"), other tuples NOT_RUN |
| TRE-V0-036..039 | `native/boosttest.go`; `TestBoostTestRecordedJUnitWitnesses`, `TestBoostTestBoundaryContradictions`, `TestBoostTestClosedBuild`, `TestBoostTestRegistryDispatch`, opt-in `TestBoostTestLiveExecution` | Actual Boost 1.92.0 (archive SHA-256 verified) JUnit reports from Apple clang 21.0.0 on macOS arm64 in `native/testdata/boosttest-provenance.json`; live shared-executor run PASS on 2026-10-08; other compilers, platforms and library variants NOT_RUN |

Review and live manifests above are under `/private/tmp/cem10-build` for this build; their exact
source/report hashes govern reuse. Current source and committed fixture provenance provide the
repository trace. A previous passed receipt does not cover subsequently changed input bytes.

Individually pinned configuration/reporter runtime artifacts, including JUnit and Android SDK jars,
are stream-hashed up to 256 MiB before and after execution phases. Source inventory and native
reports retain their independent 4 MiB per-file bounds. This does not prove full dependency closure.

## SQL observation extension

- `TRE-V0-011`: SQL assertion/suite profiles use explicit independently pinned native tools and configuration; no automatic database service, network connection, credential discovery or arbitrary argv is introduced. Raw pgTAP verdicts remain authoritative even when psql exits0. Missing/error output produces uncertainty.
- `TRE-V0-012`: File-aggregate sqllogictest reports declare SUITE_ONLY and native executed/skipped counts. They never invent per-query IDs or attempts. An empty suite cannot establish observed test inventory; skipped-only is SKIPPED. SQL errors without a complete denominator are incomplete. Source/runtime/database state and full dependency authority remain explicit qualification limits.

`Input.SourceFile` is admitted caller-bound fixed-invocation metadata derived from the pinned script
`Project` and source inventory. It is not native reported provenance or immutable Git attestation.
pgTAP `CASE` IDs combine that script with native assertion ordinal/description. SQLite sqllogictest
emits one `SUITE_ONLY` observation with native `executedCount` and `skippedCount`; no per-query ID
or retry attempt is invented. Missing, negative, overflowing or contradictory aggregate counts
remain incomplete; an empty suite is incomplete and a skipped-only suite is SKIPPED.

Actual evidence uses PostgreSQL 17.11/pgTAP 1.3.4 and original SQLite sqllogictest revision
`db57eba95d7c412bb413da5480c8be24109a8faf` with SQLite 3.53.0. pgTAP `not ok` remains a failure
even with psql exit 0. Rust sqllogictest CLI 0.29.1 file-level JUnit cannot distinguish empty or
all-skipped record files from executed passes and is not accepted as this profile. The adapter
starts no database service; disposable qualification clusters were shut down with absence of
postmaster PID files and `pg_ctl` no-server results retained. Database state/authority, server
identity and runtime dependency closure remain NOT_OBSERVED.

## Appium Android execution slice

- `TRE-V0-013`: The optional `appium-uiautomator2-wdio` profile MUST fix one declared canonical loopback Appium endpoint, exact emulator UDID, Android/UiAutomator2 capability tuple, one pinned JavaScript spec and native WDIO JSON reporter. Generated configuration MUST admit only closed typed launch settings and must not retain project connection overrides, alwaysMatch, extra suites/specs, hooks, services or retry scheduling. Build and Parse MUST remain pure; no server or emulator is started by the profile.
- `TRE-V0-014`: Native observations MUST contain exactly the declared device and one canonical selected file URI. The executor passes cloned caller Target, SourceRoot and literal Selectors for comparison, without assigning Git authority. Missing, surplus, redirected or malformed identities and setup/session/collection/zero-test failures produce incomplete observations. Each retained case carries its native source file. Server, device, installed app, image and complete dependency authority remain NOT_OBSERVED.

Appium3.8.0, UiAutomator2 8.7.0, WDIO9.32 and Node22.23.3 were exercised on a separately owned cached Android emulator. Six current-source common-executor cases retain pass/fail/skip, zero tests, setup and collection observations. `/private/tmp/cem10-build/mobile/repair1-freeze.json` binds production, reports and receipts; independent `/private/tmp/cem10-build/mobile/review-repair1.json` is PASS and includes actual native ConfigParser/session-sanitizer regressions for redirected device/endpoint and unpinned suite injection. `repair1-cleanup.json` verifies SIGTERM retired owned server/emulator groups and ports while preserving existing devices. These local tuples do not qualify other Appium backends, physical devices or all platforms. Rollback removes the optional registry entry/profile; native report bytes and historical receipts remain retained.

The later Cargo1.98.1 integration-target qualification retains a real failed assertion and ignored
case as FAILED/SKIPPED with complete inventory, and an exact nonexistent selector as incomplete
NO_TESTS despite exit0. `/private/tmp/cem10-build/next-qualification/qualification.json` retains
fixed invocations, pins and original stdout/stderr. Failure cause and retry history are not invented;
Cargo binary/doctest negative matrices remain open.

The executor normally bounds the test deadline and process-group cleanup. The fixed
Playwright graceful-interrupt path permits at most five additional seconds for native cleanup
before hard fallback; timeout and cancellation still produce incomplete observations. Default
profiles do not acquire this behavior. The optional invocation flag is omitted when false so
older serialized plans and their native identities remain replayable.

`/private/tmp/cem10-build/browser-lifecycle/review-repair1.json` is independent PASS for the
Playwright lifecycle repair. Actual timeout and interruption each retired the native Chrome
group and all observed helpers; normal browser execution also retired its group. An uncooperative
leader receives bounded fallback. Historical Go and Playwright plan/receipt bytes remain identical
when the optional flag is absent; current execution of an older Playwright plan requires replanning.
Historical Go candidate assembly replay remains VERIFIED. V1-0608 stays OPEN until integration
and native completion; cooperative native cleanup does not qualify hostile detached execution.
TRE-V0-034 (2026-10-08) retires escaped owned descendants without runner cooperation; its exact
Playwright 1.63 browser requalification is NOT_RUN (see the V1-0608 build log).

## Runner observation code ownership

These codes describe validation failures in retained runner observations. They do not establish
criterion adequacy or classify an arbitrary process failure as an assertion failure. Each row
names only the checks at its cited emitters; shared normalization makes observations with
problems incomplete and replaces resolved test/attempt states with UNKNOWN.

| Code | Implemented check | Emitters |
| --- | --- | --- |
| `aggregate-count-bound` | A supplied executed/skipped count is negative or above MaxTests, or their SUITE_ONLY sum exceeds MaxTests. | `internal/testrunner/validate.go:51@a0671397`; `internal/testrunner/validate.go:59@5b4e95af` |
| `aggregate-count-missing` | A SUITE_ONLY observation omits executedCount or skippedCount. | `internal/testrunner/validate.go:55@5e7e342a` |
| `aggregate-state-conflict` | A SUITE_ONLY PASSED row has zero executed cases, or a SKIPPED row has a nonzero executed count. | `internal/testrunner/validate.go:62@2dd1d409` |
| `ambiguous-expected-selector` | An expected native test identity is empty or repeated. | `internal/testrunner/validate.go:93@98f1f50a` |
| `ambiguous-test-identity` | An observed native test identity is empty or repeated, or a Node top-level event at 1:1 named after its file cannot be told apart from Node's file-level event. | `internal/testrunner/validate.go:69@789f20a9` |
| `attempt-bound` | An observed test has more than MaxAttempts retained attempts. | `internal/testrunner/validate.go:82@6aa55fa4` |
| `attempt-outcome-conflict` | Playwright result statuses contradict its expected/unexpected/flaky/skipped category, or retained attempts contradict the final state (including a flaky result without at least two attempts ending in pass). | `internal/testrunner/dynamic/native.go:443@21e82322`; `internal/testrunner/dynamic/native.go:447@21e82322`; `internal/testrunner/dynamic/native.go:451@21e82322`; `internal/testrunner/dynamic/native.go:455@21e82322`; `internal/testrunner/dynamic/validation.go:158@24bd514f` |
| `attempt-sequence` | A Playwright result retry number differs from its zero-based result position. | `internal/testrunner/dynamic/native.go:413@93e0a03b` |
| `browser-helper-failed` | A TestCafe run's native stdout or stderr shows its per-HOME browser helper (`.testcafe-browser-tools`) failing with a Node system error such as `EEXIST` (TRE-V0-114). | `internal/testrunner/dynamic/testcafe.go:16@7a5ca728` |
| `case-error-conflict` | WDIO reports a native error for a nonfailed case, or Node reports a non-TODO pass event with an error. | `internal/testrunner/dynamic/native.go:195@7456805d`; `internal/testrunner/dynamic/browser.go:139@89d45ae9` |
| `collection-or-hook-error` | A Jest/Vitest file reports failed status without a failed assertion row in that file. | `internal/testrunner/dynamic/parse.go:272@11da83fa` |
| `collection-or-plan-error` | The AVA TAP plan count differs from the observed sequential outcome count. | `internal/testrunner/dynamic/native.go:347@e3eaf1ea` |
| `conflicting-outcomes` | A TestCafe skipped case also has errors, or an XML skipped case also has failure/error elements. | `internal/testrunner/dynamic/native.go:85@4df23ffc`; `internal/testrunner/dynamic/browser.go:51@02fd57db` |
| `contradictory-exit` | A shared-boundary success exit accompanies an observed FAILED test. | `internal/testrunner/validate.go:122@8df4cef9` |
| `count-mismatch` | A parsed native report total or category count disagrees with its observed rows, outcomes or attempts; the compared denominator is format-specific; or Node reports a negative skipped/todo count. | `internal/testrunner/dynamic/native.go:111@9a0a1c7f`; `internal/testrunner/dynamic/native.go:115@74f3bece`; `internal/testrunner/dynamic/native.go:230@b6957ba2`; `internal/testrunner/dynamic/native.go:259@ad1ded29`; `internal/testrunner/dynamic/native.go:473@0546fe58`; `internal/testrunner/dynamic/browser.go:57@312cf3e9`; `internal/testrunner/dynamic/browser.go:102@de598de5`; `internal/testrunner/dynamic/browser.go:159@6329112d`; `internal/testrunner/dynamic/browser.go:255@d7afa062`; `internal/testrunner/dynamic/parse.go:283@6144e481`; `internal/testrunner/dynamic/parse.go:290@9b100a65`; `internal/testrunner/dynamic/parse.go:348@c89d4e2d`; `internal/testrunner/dynamic/parse.go:375@5e2333a0`; `internal/testrunner/dynamic/parse.go:399@a3fc5178` |
| `device-cleanup` | Device cleanup after a timed-out or interrupted TEST phase was not clean: a step failed, timed out, or its session identity was unobserved (TRE-V0-087). | `internal/testrunner/execute_unix.go:223@ab878870` |
| `exception-conflict` | An RSpec example maps to PASSED while retaining an exception object. | `internal/testrunner/dynamic/parse.go:370@3ad339db` |
| `exit-report-conflict` | A dynamic runner exits zero while a parsed test is FAILED, TIMED_OUT or INTERRUPTED. | `internal/testrunner/dynamic/parse.go:150@a9d4d7aa` |
| `hook-error` | A WDIO hook has a native error or literal failed state. | `internal/testrunner/dynamic/browser.go:153@9b21db50` |
| `hook-or-unmatched-result` | The Cypress/Mocha result-status identity count differs from the collected test-row count. | `internal/testrunner/dynamic/browser.go:105@7ff4ce18` |
| `invalid-exit-profile` | Exit-code lists exceed their bound, contain values outside 0..255, or repeat a value within or across lists. | `internal/testrunner/validate.go:101@0b03b013` |
| `invalid-prior-attempt` | A retained nonfinal attempt is neither FAILED nor TIMED_OUT. | `internal/testrunner/dynamic/validation.go:162@71956e90` |
| `jasmine-count-mismatch` | Jasmine's jasmineStarted totalSpecsDefined differs from the number of reported specDone events. | `internal/testrunner/dynamic/jasmine.go:165@962a83f4` |
| `jasmine-file-outside-root` | A Jasmine spec filename is empty, relative or outside the source root, or the source root is empty. | `internal/testrunner/dynamic/jasmine.go:135@33b872d5` |
| `jasmine-global-error` | Jasmine's jasmineDone carries top-level failed expectations, such as a top-level afterAll error. | `internal/testrunner/dynamic/jasmine.go:169@80706a67` |
| `jasmine-identity-conflict` | A Jasmine suite or spec has no consistent reported parent chain (unknown parent, cycle, empty suite description, or a suite fullName that is not its parent's fullName, one space and its description), or a spec has an empty id or description or a fullName that is not its parent suite's fullName, one space and its description. | `internal/testrunner/dynamic/jasmine.go:111@29130745`; `internal/testrunner/dynamic/jasmine.go:130@d6a5d62d` |
| `jasmine-incomplete` | Jasmine's overallStatus is incomplete, for example for a focused run or no specs found. | `internal/testrunner/dynamic/jasmine.go:181@7713363c` |
| `jasmine-outcome-conflict` | A Jasmine spec that is not failed retains failed expectations. | `internal/testrunner/dynamic/jasmine.go:151@205985f0` |
| `jasmine-parallel-unqualified` | Jasmine reports parallel mode, which this profile has not qualified. | `internal/testrunner/dynamic/jasmine.go:78@087c3074` |
| `jasmine-selector-without-specs` | A Jasmine spec-file selector names a file with no reported spec, including a missing path Jasmine silently ignores. | `internal/testrunner/dynamic/jasmine.go:161@4416642f` |
| `jasmine-status-conflict` | Jasmine's overallStatus is passed with a failed spec, suite or global error, or failed without any of them. | `internal/testrunner/dynamic/jasmine.go:174@c13e083e`; `internal/testrunner/dynamic/jasmine.go:178@fb40e1ea` |
| `jasmine-suite-error` | A Jasmine suite is failed or retains failed expectations, such as a beforeAll or afterAll error. | `internal/testrunner/dynamic/jasmine.go:115@e24ba611` |
| `jasmine-unknown-overall-status` | Jasmine's overallStatus is not passed, failed or incomplete. | `internal/testrunner/dynamic/jasmine.go:183@cacab051` |
| `jasmine-unselected-file` | A Jasmine run with spec-file selectors reports a spec from a file outside their root-joined paths. | `internal/testrunner/dynamic/jasmine.go:143@cddb7713` |
| `missing-attempt` | A non-skipped Playwright test has no results, or a test passed to retained-attempt validation has no attempts. | `internal/testrunner/dynamic/native.go:460@92208046`; `internal/testrunner/dynamic/validation.go:153@a1b5f038` |
| `missing-count` | An XML suite containing direct testcase rows omits its tests count. | `internal/testrunner/dynamic/native.go:60@e722f2b9` |
| `missing-name` | An XML testcase has an empty name. | `internal/testrunner/dynamic/native.go:75@e3950df9` |
| `missing-native-id` | A Playwright spec has an empty native ID. | `internal/testrunner/dynamic/native.go:396@dceb991e` |
| `missing-report` | A non-AVA dynamic runner supplies no report files. | `internal/testrunner/dynamic/parse.go:60@9dbf26dd` |
| `missing-selected-test` | An expected native test identity is absent from observed identities. | `internal/testrunner/validate.go:97@034a7064` |
| `missing-summary` | The Node event stream ends without a final summary having an empty file field. | `internal/testrunner/dynamic/native.go:256@3d3dab29` |
| `negative-count` | A Nightwatch completed test has a negative retries, failed or errors count. | `internal/testrunner/dynamic/browser.go:217@0993c5a5` |
| `nightwatch-hook-error` | A Nightwatch completed section without a matching completed test reports errors, failures or literal fail status. | `internal/testrunner/dynamic/browser.go:197@7dccaec6` |
| `nightwatch-runtime-error` | Nightwatch reports system error text, a nonzero error count or error messages; or lastError is not a decodable NightwatchAssertError supported by a retained failed attempt. | `internal/testrunner/dynamic/browser.go:192@558311a9`; `internal/testrunner/dynamic/browser.go:251@79465e5a` |
| `nightwatch-test-error` | A Nightwatch completed test has a positive errors count. | `internal/testrunner/dynamic/browser.go:237@342f8ce7` |
| `no-executed-inventory` | A SUITE_ONLY observation has both counts present and their sum is zero. | `internal/testrunner/validate.go:66@710cfe9e` |
| `node-run-unsuccessful` | The final Node summary reports success=false without an observed failed, timed-out or interrupted test. | `internal/testrunner/dynamic/native.go:241@5e6d82cd` |
| `node-runtime-error` | A Node test error has a failureType other than testCodeFailure, testTimeoutFailure or cancelledByParent. | `internal/testrunner/dynamic/native.go:205@40fd52ec` |
| `node-suite-error` | A Node suite fail/error is not explained by subtestsFailed plus an already observed failed, timed-out or interrupted test. | `internal/testrunner/dynamic/native.go:166@7e3f3a32` |
| `outside-example-errors` | RSpec reports a nonzero errors_outside_of_examples_count. | `internal/testrunner/dynamic/parse.go:351@503ba44e` |
| `playwright-global-error` | The Playwright report contains a top-level error entry. | `internal/testrunner/dynamic/native.go:383@7d964fc3` |
| `process-containment` | Escaped-descendant containment of an executor phase was incomplete: a retained escaped identity survived the bounded observation, or the structural proof could not be completed. | `internal/testrunner/execute_unix.go:271@aafefb4d` |
| `retry-history-missing` | Nightwatch declares positive retries without the same number of retained prior attempts; or Jest/Vitest indicates multiple invocations or passing-with-failure-messages without reconstructable attempt history. | `internal/testrunner/dynamic/browser.go:224@f37527eb`; `internal/testrunner/dynamic/parse.go:262@7bb3fc94`; `internal/testrunner/dynamic/parse.go:262@7bb3fc94` |
| `runner-exit` | The shared boundary receives a negative exit code or one outside all admitted success/failure/outcome-neutral lists. | `internal/testrunner/validate.go:119@a90e3e46` |
| `runner-unsuccessful` | Jest/Vitest reports success=false with assertion rows but no observed FAILED test. | `internal/testrunner/dynamic/parse.go:304@4b70a549` |
| `runtime-error` | Jest/Vitest reports a positive runtime-error suite count. | `internal/testrunner/dynamic/parse.go:296@be9d02c7` |
| `section-test-conflict` | Nightwatch completed-section and completed-test records map to different states for the same name. | `internal/testrunner/dynamic/browser.go:211@7936fd1f` |
| `selector-without-tests` | A Mocha file selector names a file with no observed test, including a missing path Mocha only warns about. | `internal/testrunner/dynamic/mocha_selection.go:46@1584fa21` |
| `setup-or-runtime-error` | An XML testcase contains one or more error elements. | `internal/testrunner/dynamic/native.go:72@b1ab2b72` |
| `success-conflict` | Jest/Vitest declares success with a failed assertion, or Node declares success with failed/cancelled counts or existing observation problems. | `internal/testrunner/dynamic/native.go:233@f9a16178`; `internal/testrunner/dynamic/parse.go:287@04a1c2a9` |
| `tap-bailout` | The AVA TAP stream contains a line beginning Bail out!. | `internal/testrunner/dynamic/native.go:293@b75d2c79` |
| `temp-cleanup` | The executor's private TMPDIR (`<reportDir>/.tmp`) could not be removed after retirement, device cleanup and report reads (TRE-V0-116). | `internal/testrunner/execute_unix.go:197@f623956b` |
| `test-bound` | Observed inventory exceeds MaxTests at the shared boundary. | `internal/testrunner/validate.go:41@e88d43a4` |
| `undeclared-tool` | A failing test's output shows a tool missing from the runner's PATH (TRE-V0-120). | `internal/testrunner/validate.go:129@183022a0` |
| `unclassified-exit` | An admitted shared-boundary failure exit has no observed FAILED test. | `internal/testrunner/validate.go:125@94987f7f` |
| `unexpected-observed-test` | A Mocha run with file selectors and expected identities observes a test outside them. | `internal/testrunner/dynamic/mocha_selection.go:41@4ac5bca8` |
| `unexplained-exit` | A dynamic runner exits nonzero without failed/timed-out/interrupted tests or any previously recorded observation problem. | `internal/testrunner/dynamic/parse.go:147@67d7a680` |
| `unknown-attempt-state` | A Playwright attempt status maps to UNKNOWN, or the shared boundary sees an attempt state outside its admitted state enumeration. | `internal/testrunner/validate.go:86@94ec202d`; `internal/testrunner/dynamic/native.go:425@4a8c35e3` |
| `unknown-expected-status` | A Playwright test with results has an expectedStatus that maps to UNKNOWN. | `internal/testrunner/dynamic/native.go:432@07ce9705` |
| `unknown-granularity` | A shared-boundary granularity is neither empty, CASE nor SUITE_ONLY. | `internal/testrunner/validate.go:47@50f6c9dd` |
| `unknown-node-event-kind` | A Node pass/fail event is neither a suite nor a test in its details.type. | `internal/testrunner/dynamic/native.go:184@9933433d` |
| `unknown-retry-information` | RetryInformation is not RETAINED, NOT_REPORTED or NOT_APPLICABLE. | `internal/testrunner/validate.go:133@b82d5be8` |
| `unknown-state` | A dynamic parsed test has state UNKNOWN. | `internal/testrunner/dynamic/parse.go:137@53bac795` |
| `unknown-suite-state` | A Jest/Vitest file status maps to UNKNOWN. | `internal/testrunner/dynamic/parse.go:275@045a51e8` |
| `unknown-test-state` | A shared-boundary test state is outside its admitted state enumeration. | `internal/testrunner/validate.go:73@8c1af2e1` |
| `unresolved-test-state` | A shared-boundary test state is UNKNOWN, INTERRUPTED or TIMED_OUT. | `internal/testrunner/validate.go:79@80befa5b` |
| `unselected-test-file` | A Mocha test with file selectors reports a native file outside their lexical root-joined paths. | `internal/testrunner/dynamic/mocha_selection.go:38@490b448f` |

## Explicit argument-free TEST phases and CMocka (experimental)

This slice makes one shared executor contract change, which applies to every
runner profile, and adds a dedicated CMocka tuple. It does not establish all-C,
JNI, affected-selection, Tasks criterion or stable-release authority. The root
decision record `cmocka-zero-argv-root-decision/ROOT-DECISION.json` (sha256
`e4de2129c7498d62fb8b292dc2c20b532d2d00a5c88d23382c1ae23a9204e161`) accepted the
executor widening with the conditions restated in `TRE-V0-015`.

- `TRE-V0-015`: An argument-free invocation MUST use exactly one explicitly
  declared `TEST` phase whose tool is `primary`. The fixed profile emits
  `Argv: []` and its environment on that phase. Empty implicit invocations,
  mixed implicit/explicit invocations, and empty argv on `BUILD`, `DISCOVER`,
  `DECODE`, an auxiliary tool or an implicit primary name remain rejected.
  Independent executable identity, regular-file/size checks, fixed-plan equality,
  source/config/reporter pins, environment bounds, report inventory, timeout and
  process retirement MUST remain enforced. This is a shared executor contract
  change available to every runner profile's explicit TEST/primary phase, not a
  CMocka-specific permission guard. In this slice only `cmocka-xml` emits empty
  argv; no other runner profile's fixed plan changes, and a later profile that
  wants empty argv needs its own admission.
- `TRE-V0-016`: `cmocka-xml` MUST execute one caller-prepared, independently pinned
  CMocka 2.0.2 test executable with fixed `STANDARD,XML` output and one fresh
  `cmocka.xml`. The existing request `Target` names one native group. Build
  requires 1..64 unique expected `group::test` identities, using bounded ASCII
  identifiers; zero or one literal selector is allowed and a selector must be
  the sole expected identity. Wildcards, multi-selection and caller configuration,
  reporter, project or auxiliary-tool overrides are rejected. Empty expected
  inventory is not a production admission exception.
- `TRE-V0-017`: The dedicated parser MUST validate the native group against
  `Input.Target`, exact expected case inventory, unique identities, bounded native
  XML grammar, counts, exit and both original STANDARD streams. XML and STANDARD
  must agree. Missing STANDARD output, unknown diagnostics, retries/multiple
  documents, malformed or contradictory counts and fixture errors remain
  incomplete. CMocka XML uses `failure` for ordinary failures and fixture errors;
  the cause remains `UNKNOWN`. A native group teardown error can leave XML and
  exit successful, so its STANDARD diagnostic MUST prevent completion. Shared
  `Normalize` stays unchanged: incomplete public states become `UNKNOWN` while
  original reports, diagnostics and identities remain retained.

The native runner runs once; retry history is `NOT_REPORTED`. Actual zero/no-match
qualification supplies a valid expected identity which is absent from native
execution. This yields incomplete evidence even when native exit is zero. Builds
and static linking occur outside the test receipt with separately retained tool,
source, archive and compiler evidence; full SDK/loader closure is not inferred.

Acceptance evidence is the bounded macOS arm64 CMocka 2.0.2 static tuple:
pass/fail/skip, exact selected pass with a same-prefix decoy, selected skip, zero,
per-test setup, per-test teardown, group setup, group teardown and wrong Target.
`TestExecuteExplicitPrimaryTestWithoutArguments` verifies real pinned `pwd`
execution with no arguments and refusal of the other empty-argv forms.
`TestCMockaActualDualFormatWitnesses`, `TestCMockaBoundaryContradictions`,
`TestCMockaClosedBuild`, `TestCMockaRegistryDispatchAndTargetBoundary`,
`TestCMockaPlanBindsTargetAndFixedEnvironment` and the opt-in
`TestCMockaNativeReceiptReadback` cover profile admission and both parsing layers.
Native timeout/interruption qualification, publisher signature authentication,
Unity and broader C framework coverage remain outside this evidence.

Rollback has two independent parts: removing the additive `cmocka-xml` profile,
and reverting the shared executor condition so every phase again requires at
least one argv element (the previous `len(p.Argv) == 0` refusal). Reverting the
executor without removing the profile makes every CMocka plan refuse with
`argv bound`; no other profile is affected. Historical refusals and native
reports are retained under their original hashes. CEM binding, live
requalification on the integration base and native Tasks completion remain
separate closeout steps.

## Ginkgo v2 (experimental)

This slice adds the dedicated `ginkgo-v2` native profile for one caller-prepared
Ginkgo v2 suite binary built from a Go test package. It does not change the
shared executor: Ginkgo always emits non-empty argv, and `TRE-V0-015` empty argv
remains exclusive to `cmocka-xml`. The pinned reading is Ginkgo v2.33.0 at commit
`9f941496ce264d03b91f103e4ec4a19bbc75ce97`; the read pins are retained in
`internal/testrunner/native/testdata/ginkgo-provenance.json`.

- `TRE-V0-018`: `ginkgo-v2` MUST execute one independently pinned suite binary
  with exactly 17 fixed argv elements: `-test.run=^<GoWrapper>$`,
  `-test.timeout=0`, `-ginkgo.json-report=<ReportDir>/ginkgo.json`, seed 1, no
  randomization, no fail-fast, no fail-on-pending, fail-on-empty, one flake
  attempt, no dry run, a 1h suite timeout, a 1s grace period, no sleep on failure,
  no progress polling, output interception `none` and no color. `Target` is
  `GoWrapper::SuiteDescription`; expected identities are
  `GoWrapper::SuiteDescription::FullText`, where FullText joins non-empty
  container texts and the leaf text with one space. Build requires 1..4096
  unique expected identities, unique literal selectors drawn from them, a pinned
  literal `_test.go` Project and an absolute symlink-resolved Root. It fixes
  `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, `GOWORK=off` and empty
  `GOFLAGS`, and rejects caller configuration, reporter, report-file and
  auxiliary-tool overrides. No `-test.parallel`, `-test.count` or
  `-test.shuffle` argument is emitted.
- `TRE-V0-019`: Selection MUST add exactly one 18th element,
  `-ginkgo.focus=^<QuoteMeta(SuiteDescription)> (<QuoteMeta(FullText_1)>|...)$`.
  Ginkgo matches focus unanchored against `SuiteDescription + " " + FullText`,
  so the suite description is inside the anchored expression and the selected
  texts form one alternation. The whole element is bounded at the shared 4096-byte
  argv limit and a longer selection refuses. The report's
  `SuiteConfig.FocusStrings` MUST equal that one string with a selector and be
  empty without one.
- `TRE-V0-020`: The dedicated parser MUST accept exactly one `ginkgo.json`
  report within the per-report bound containing exactly one suite. It decodes
  strict UTF-8 JSON with duplicate-key, unknown-key, missing-key, nesting and
  trailing-content refusal; enums accept only the exact pinned table strings, so
  JSON null and unknown strings refuse. `SuiteDescription` MUST equal Target,
  `SuitePath` MUST equal the executor's cleaned Root without filesystem reads,
  and the 25 `SuiteConfig` fields MUST equal the fixed invocation. Counts,
  inventory, eligibility, suite-level nodes, failure context and exit/suite
  coherence MUST agree, or the observation is incomplete and shared `Normalize`
  makes public states `UNKNOWN`.

Native mapping: a passed It with one attempt and no Failure is `PASSED`; a failed
It whose Failure context is `leaf-node` is `FAILED` with cause `UNKNOWN`; a runtime
`Skip()` (skipped with one attempt) is `SKIPPED` with its message; pending and
non-selected skipped specs with zero attempts and no Failure are `SKIPPED`
without an attempt. These are incomplete: an eligible spec with zero attempts
(`GINKGO_NOT_ENTERED`), a non-selected spec that ran, a Failure on a passed or
filtered spec, a failure with `in-container` or `top-level` context
(`GINKGO_HOOK_FAILURE`), any suite-level node other than passed without a Failure, any non-It leaf
node, panicked/aborted/interrupted/timedout states, retries or repeats,
`ParallelProcess` other than 1, AdditionalFailures, programmatic focus, any
`SpecialSuiteFailureReasons`, count or inventory mismatch, exit 0 without a
successful unfocused suite, exit 1 with `SuiteSucceeded` true, exit 1 without a
failed spec, suite node or reason, and any other exit. Problem codes are
`GINKGO_INVALID_REPORT`, `GINKGO_TARGET_MISMATCH`, `GINKGO_SUITE_PATH_MISMATCH`,
`GINKGO_CONFIG_MISMATCH`, `GINKGO_RETRY_UNSUPPORTED`, `GINKGO_PARALLEL_UNSUPPORTED`,
`GINKGO_ADDITIONAL_FAILURE`, `GINKGO_SUITE_PROBLEM`, `GINKGO_UNSUPPORTED_NODE`,
`GINKGO_INVENTORY_MISMATCH`, `GINKGO_UNSELECTED_EXECUTED`,
`GINKGO_UNEXPECTED_FAILURE`, `GINKGO_NOT_ENTERED`, `GINKGO_HOOK_FAILURE`,
`GINKGO_UNRESOLVED_STATE`, `GINKGO_COUNT_MISMATCH`, `GINKGO_PROGRAMMATIC_FOCUS`,
`GINKGO_SUITE_FAILURE_REASON`, `GINKGO_EXIT_SUITE_CONTRADICTION`,
`GINKGO_UNEXPLAINED_SUITE_FAILURE` and `GINKGO_EXIT_UNSUPPORTED`.

Recorded limits. A leaf failure in an Ordered container leaves later specs
skipped with zero attempts and a populated Failure; they are `GINKGO_NOT_ENTERED`
and the run stays incomplete. The argv is non-verbose, and Ginkgo emits
AdditionalFailures only in verbose mode, so their absence is not proof of no
follow-on failure; present ones are incomplete. `SuitePath` comes from
`os.Getwd` in the child; the executor sets the working directory to Root and
omits `PWD`, so the reported path is expected to be symlink-resolved, which is
why Build requires a resolved Root (inference from source reading, not observed).
Skip in BeforeSuite records the reason `Suite skipped in BeforeSuite` while
`SuiteSucceeded` may stay true; any reason is incomplete. Module identity rests on
the caller-pinned executable digest and strict schema parsing; a
`debug/buildinfo` module-sum check is NOT_IMPLEMENTED because the verified module
sum is NOT_OBSERVED.

Acceptance evidence is unit-level plus the 2026-10-17 live qualification: `TestGinkgoNativeProjectionStates`,
`TestGinkgoReportCountAndBound`, `TestGinkgoParserRefusals`,
`TestGinkgoStrictJSON`, `TestGinkgoBeforeSuiteSkipShape` (source-derived model of
the BeforeSuite skip case), `TestGinkgoOrderedFollowOnFailure`,
`TestGinkgoClosedBuild`, `TestGinkgoFocusBound`,
`TestGinkgoRegistryDispatchAndTargetBoundary` and
`TestGinkgoPlanBindsTargetAndFixedArgv`. Their reports are synthetic, shaped from
the pinned source reading. Actual runner-generated qualification (`TRE-V0-009`) for the
darwin/arm64 tuple with Ginkgo 2.33.0 and Gomega 1.40.0 is recorded by 10 corvint-test-runner
witnesses in "Ginkgo v2 live qualification" below, which supersedes the earlier NOT_RUN
statement for that tuple (V1-1540). Other tuples remain unqualified release blockers.

Rollback removes the additive `ginkgo-v2` profile, its tests and provenance file.
No shared executor, other profile, queue, store or frozen wire changes.

## Stable selection expectation (experimental)

Nightwatch native identities are `ModulePath::Name::TestEnv::SessionID::Case`. A
fresh WebDriver session ID is unknown when a plan is admitted, so exact
`ExpectedTests` cannot be pre-admitted for a browser run. This slice adds a
separate, closed selection expectation. Native identities keep their session
part, and historical plan, receipt and identity bytes are unchanged.

- `TRE-V0-021`: A request MAY carry `expectedSelection` `{version, matcher,
  tests}`, encoded with `omitempty` so a request without it serializes to the
  historical bytes and plan identity. The only version is
  `corvint-test-selection/1`. The only matcher is `nightwatch-session-elided/1`,
  bound to runner `nightwatch`. Document decoding stays closed: unknown or
  duplicate fields refuse. Status: accepted (decision 0454; V1-0620).
- `TRE-V0-022`: Plan and run admission MUST refuse, before any launch, a
  selection with another version, an unknown matcher or a matcher bound to
  another runner, one combined with non-empty `ExpectedTests`, zero or more than
  4096 identities, a duplicate identity, or an identity that is not exactly four
  non-empty `::`-separated components of at most 4096 bytes without control
  characters or an edge colon. The admitted selection is part of the approved
  plan digest. Status: accepted (decision 0454; V1-0620).
- `TRE-V0-023`: After native observation, shared `Normalize` MUST project each
  observed test from its structured file, suite and name fields: its identity must be
  `File::Suite::TestEnv::SessionID::Name` with a non-empty session that has no
  separator or edge colon, and the projection is `File::Suite::TestEnv::Name`.
  Coverage is exact: an unprojectable test
  (`unmatchable-selected-test`), two native tests projecting to one identity
  (`aliased-selected-test`), an observed identity outside the selection
  (`extra-selected-test`), an absent selected identity (`missing-selected-test`)
  or a selection that fails `TRE-V0-022` (`invalid-test-selection`) makes the
  observation incomplete. Public states become `UNKNOWN`, and native IDs,
  including session IDs, stay retained. Status: accepted (decision 0454; V1-0620).

- `TRE-V0-034`: The Unix executor MUST run every phase through `groupreap.RunContained`
  (`PGO-V0-007`; `RunContainedRetiring` when the plan also requests `TRE-V0-030`
  retirement, which then runs first while the exited leader is unreaped, after the owned tree
  is frozen and its escaped identities recorded, so a descendant that retirement cannot prove
  and orphans is still retired by that identity; an incomplete freeze or record runs the
  structural pass first), so a descendant that leaves the phase's process group (a detached
  browser in its own session, for example) is retired when the phase ends normally,
  times out or is interrupted. A non-graceful timeout or interruption signals only the
  phase leader, leaving the remaining group and escaped descendants attached to it for
  the structural sweep; graceful SIGINT and its bounded fallback are unchanged. Incomplete
  containment appends a `process-containment` execution problem, so shared normalization
  makes the observation incomplete. A sampled escaped descendant that lost its owned parent
  is retired only under its unchanged sampled PID and start time. Status: proposed (V1-0608).

| Requirements | Source/tests | Evidence |
| --- | --- | --- |
| TRE-V0-021 | `selection.go`, `types.go`, `document.go`; `TestSelectionIsAdditiveToHistoricalBytes`, `TestHistoricalPlanAndReceiptBytesSurviveSelectionContract` | Frozen plan and receipt bytes generated at `0c94c66c` decode and re-encode byte-identically with an unchanged plan digest |
| TRE-V0-022 | `AdmitSelection`, `registry.Build`; `TestSelectionAdmissionIsClosed`, `TestNightwatchSelectionPreAdmittedAcrossFreshSessions` | Invalid selections refuse at `plan` and at `run` before the report directory exists |
| TRE-V0-023 | `selectionProblems`, `Normalize`; `TestSelectionMatchesFreshSessionsAndKeepsNativeIdentity`, `TestSelectionRefusesInexactMatches`, `TestNightwatchSelectionAcrossFreshSessions`, `TestNightwatchSelectionPreAdmittedAcrossFreshSessions` | Synthetic session IDs: the runner-generated Nightwatch fixture replayed under two session IDs, and a pinned stand-in executable that picks its session at launch; live: two real Nightwatch 3.16.0 / ChromeDriver 155.0.8059.79 / Chrome 155.0.8059.80 runs through `corvint-test-runner` under one admitted selection observed distinct WebDriver sessions, each complete PASSED/FAILED/SKIPPED, and a wrong selection stayed incomplete (2026-10-15 V1-0612/V1-0620 build log) |
| TRE-V0-034 | `execute_unix.go` (`RunContained`, leader-only cancel, `containmentDetail`); `TestExecuteRetiresEscapedDetachedDescendants` (timeout, graceful timeout, interruption, normal exit); `TestRunContainedRetiringKeepsUnprovenOrphan`, `TestRunContainedRetiringRetiresHelperStoppedByFailedFreezes`, `TestFreezeOwnedReportsStoppedBeforeFailure` (batch F composition with `TRE-V0-030`) | Darwin arm64: fails at base (detached session survives), passes after; live Playwright 1.61.1 + Chromium 1228 through `corvint-test-runner`: a test-spawned detached browser left 9 (timeout) and 8 (SIGINT) survivors at base and none after, with normal, timeout and SIGINT runs retiring every observed process and no pre-existing Chrome process lost; Linux and exact Playwright 1.63 runs NOT_RUN; V1-1545/V1-1434 (2026-10-21): the lifecycle tests expire the declared timeout through the `executionLimit` seam only after `groupreap.EscapedSampled` reports every expected detached PID as sampled (TimeoutSeconds 120 is a hang bound only), and the escape helper publishes its pid file atomically; with a 4-5 s pre-readiness delay injected, base fails (`readiness marker missing`, `detached browser never started`) and the fix passes, plus `-count=10` passes at load ~9-25 |

Recorded limits. One live tuple was run on macOS arm64 (2026-10-15):
Nightwatch 3.16.0 with ChromeDriver 155.0.8059.79 driving host Chrome
155.0.8059.80, which deviates from the 154.0.8037.59 tuple the tickets
recorded. Nightwatch names its shard after the browser version, so the
explicit report manifest pins that version, and its ChromeDriver service
needs `cat` on the executor's PATH (declared as a tool). The matcher still
trusts the parser's structured fields; the live runs show those fields carry
real WebDriver session IDs for this tuple only. Other session-bearing runners,
including WebdriverIO, need their own matcher version.

Rollback removes `expectedSelection`, `selection.go` and its admission call.
Plans without the field are unaffected, and plans that carry it then refuse as
unknown fields.

## Mocha affected selection reconciliation (experimental)

The TypeScript affected adapter already registers Mocha units as
`typescript:mocha:PATH`, and the dynamic lane has a `mocha` profile with the same
runner ID. Mocha 12.0.3 still exits zero in two cases that break an exact
selection: it merges configured `spec` files with positional file selectors, and
it only warns about a positional selector that matches no file. Before this
slice both runs produced complete observations.

- `TRE-V0-024`: For runner `mocha`, `Parse` MUST reconcile the native inventory
  with the run's literal selectors and expected identities. Each selector is
  joined lexically to `SourceRoot` unless absolute; no path is resolved, so
  `Parse` stays pure. A test whose native file is outside those paths
  (`unselected-test-file`), a selector with no observed test
  (`selector-without-tests`) and, when expected identities are present, an
  observed identity outside them (`unexpected-observed-test`) make the
  observation incomplete. A non-canonical root, a directory selector or a
  subset of expected identities therefore stays incomplete. A run without
  selectors skips all three checks and keeps its earlier inventory behaviour.
  Status: proposed (V1-0598).
- `TRE-V0-025`: Every concrete JavaScript/TypeScript affected runner ID MUST
  name a dynamic execution profile with the same ID, and the affected `unknown`
  runner MUST NOT name one. Status: proposed (V1-0598).

| Requirements | Source/tests | Evidence |
| --- | --- | --- |
| TRE-V0-024 | `mocha_selection.go`, `Parse`; `TestMochaSelectionReconciliation`, `TestMochaSelectionLeavesOtherRunnersUnchanged`, `TestMochaActualSelectionQualification`, `TestMochaBuildExecuteParse` | Synthetic profile-owned reports, plus an opt-in run of real Mocha 12.0.3 on Node 22.23.3 over six Git fixtures |
| TRE-V0-025 | `TestRunnerRegistrationsHaveExecutionProfiles` | The fifteen runner constants in the TypeScript adapter against `dynamic.Runners()` |

The opt-in qualification derives Mocha selectors from the affected plan only
when the plan is `BOUNDED` and every selected unit is Mocha-owned. Its six
fixtures are: an exact selection, which reconciles selected file, expected
identity and observed identity; a no-match selector beside a real one; a
selected file with no tests; a change no test reaches; a `.mocharc.json` whose
`spec` adds another file; and Mocha plus Jest configuration. The last three
plans are `UNKNOWN` and yield no selectors. The configured-spec fixture also
runs the narrowed selector anyway, and its receipt is incomplete. The fixture
worktrees are unchanged after each run.

Recorded limits. One local tuple was run: Mocha 12.0.3 installed with npm under
a scratch directory, Node 22.23.3, macOS arm64. ESM, TypeScript loaders,
parallel mode, root hooks and `--recursive` directory selection are NOT_RUN.
Configured Mocha discovery stays a selection unknown, not a resolved spec list.
Full dependency closure stays NOT_OBSERVED. The plan-to-selector derivation is
qualification code, not a shipped command.

Rollback removes `mocha_selection.go`, its call in `Parse` and the tests above.
Mocha observations then return to their earlier completeness, and no wire,
plan or receipt field changes.

## .NET TRX failure evidence (experimental)

VSTest and Microsoft.Testing.Platform (MTP) both write TeamTest TRX. Go XML
projection matches attributes by local name, so a namespace alias could
replace a native outcome, and failure evidence outside the projected fields
could disappear. These checks keep such evidence from becoming a pass.

- `TRE-V0-026`: VSTest and MTP TRX parsing MUST admit only the qualified
  TeamTest subset under the single default namespace declared on `TestRun`.
  Any namespaced, prefixed, undeclared-prefix, `xml:`-reserved, duplicate or
  namespace-redeclaring attribute or element, and any unknown element
  (including `FatalError` or unqualified result children), MUST refuse before
  projection, so no alias can replace a native outcome. A `Passed` row with
  any `ErrorInfo` (including empty or stack-only), summary `ErrorInfo`, an
  outcome that is not an exact qualified value, a nonzero exceptional counter
  (any counter other than `total`, `executed`, `passed`, `failed` and
  `notExecuted`) or a summary/row contradiction MUST leave the observation
  incomplete. Native counters MUST reconcile with rows; reconciled `Failed` and
  `NotExecuted` rows stay complete with their native failed or skipped state.
  Status: proposed (V1-0600).

| Requirements | Source/tests | Evidence |
| --- | --- | --- |
| TRE-V0-026 | `native/trx_xml.go` `checkNativeTRXXML`, `parseTRX`, `parseMTP`; `TestTRXConflictingEvidence`, `TestLiveNativeReports`, `TestMTPNativeMatrix`, opt-in `TestVSTestLiveExecution` | Seventeen mutations over eight captured VSTest/MTP reports (NUnit, MSTest, xUnit) refuse or stay incomplete; unmodified reports stay complete; live VSTest pass/fail/skip requalified with .NET SDK 9.0.316 |

Recorded limits. The four originally reproduced false passes were already
refused at `722904f9`; V1-0600 adds the cross-framework and alias regression
matrix and live requalification, not a parser change. MTP live pass/fail/skip
classification was requalified on net9.0 under `TRE-V0-027`; the net10.0
fixture build was not rerun (no .NET 10 runtime locally). Other TRX producers
and VSTest/MTP versions remain unqualified and refuse on unknown structure.
Rollback removes the test matrix and this section; parsers are unchanged.

## MTP native socket path admission (experimental)

Microsoft.Testing.Platform 2.4.1 creates Unix-domain-socket pipes named
`TMPDIR/<name>`; the executor sets `TMPDIR` to `<reportDir>/.tmp`. When the
socket path exceeds the native limit, the test host aborts (exit 134) before it
writes any report, so a long report directory made every MTP profile unusable.

- `TRE-V0-027`: The MTP profiles MUST refuse at build time, before launch, any
  report directory for which `len(<reportDir>/.tmp) + 1 + 46` exceeds 103 bytes.
  Here 46 bytes is the longest observed pipe name, `MONITORTOHOST_` plus 32 hex
  digits, and 103 bytes is the macOS limit that MTP enforces. The refusal MUST
  name the computed and maximum lengths. The same bound applies on Linux, so
  admission is never looser than on the qualified host. The profile MUST NOT set
  `TESTINGPLATFORM_PIPE_DIRECTORY`; doing so would change the identity of every
  existing invocation. A native socket abort that bypasses admission MUST NOT
  produce a complete observation. Status: proposed (V1-0601).

| Requirements | Source/tests | Evidence |
| --- | --- | --- |
| TRE-V0-027 | `native/mtp.go` `buildMTP`, `testrunner.ExecutionTempDir`; `TestMTPSocketPathAdmission`, opt-in `TestMTPLiveExecution` (`long-report-dir`) | Boundary (51-byte report directory) admitted and 52 bytes refused; the reproduced 68-byte directory is refused. Live net9.0 NUnit, MSTest and xUnit pass/fail/skip/zero/infra runs complete at 49 to 51 bytes. Forced long paths abort natively, with no report and no observation |

Recorded limits. The bound was measured directly on macOS with SDK 9.0.316
and runtime 9.0.18. A `TMPDIR` of 56 bytes runs; 57 bytes aborts on the
`MONITORTOHOST_` pipe. Other MTP versions may use other pipe names and are
unqualified. Linux and .NET 10 are NOT_RUN. Callers needing longer report
directories must choose a shorter report root. Rollback removes the admission
check and this section; the executor `TMPDIR` value is unchanged.

## Nonregular runner document admission (experimental)

A runner request, plan or tools document that named a FIFO without a writer
blocked `os.Open` before any validation, and the signal context could not
interrupt it (V1-0624). Admission now refuses such a path before a blocking open.

- `TRE-V0-028`: The companion MUST refuse a request, plan or tools document that
  is not a regular file before any blocking open, document validation or runner
  execution, with the stable refusal `runner refused: regular document required`
  and exit 1. Admission stats the path, opens it nonblocking and requires the
  opened descriptor to be the same regular file. The executor's independently
  pinned tools and configuration/reporter files, including the Gradle build
  manifest, MUST open nonblocking after their no-follow regular-file checks and
  require the opened descriptor to be the file the no-follow check saw, so a FIFO
  or final symlink swapped in after the check refuses instead of blocking or
  being followed. Regular documents, historical bytes and approved-plan
  semantics are unchanged. Status: proposed (V1-0624).

| Requirements | Source/tests | Evidence |
| --- | --- | --- |
| TRE-V0-028 | `cmd/corvint-test-runner` `read`; `execute_unix.go` `checkTool`, `checkPinnedFile`, `regularAbsolutePath`, `openCheckedRegular`; `TestNonregularRunnerDocumentsRefusedBeforeBlockingOpen`, `TestPinnedGradleManifestFIFORefusedBeforeExecution`, `TestCheckPinnedFileRefusesFIFOAndFinalSymlink`, `TestOpenCheckedRegularRefusesSwapAfterCheck` | Process-level child runs with a 20-second deadline; the base companion blocked on a FIFO request and tools document and stayed blocked after SIGINT (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-08-runner-fifo-admission.md`) |

Recorded limits. A swap between the no-follow check and the open is tested by
replacing the path between a recorded `Lstat` and the open helper, not by racing
a live swap. Swapped parent directories remain trusted. Parent
directories of a document path are trusted, as before. This is not hostile
filesystem authority.

Rollback restores the blocking opens; no wire, plan or receipt bytes change.

## SwiftPM XCTest transport and lifecycle (experimental)

Swift 6.4 (swiftlang-6.4.0.34.1) on macOS 26.6.2 (25G83) arm64 is the only
qualified tuple. Linux, other Swift versions and Swift Testing in this profile
remain NOT_OBSERVED.

- `TRE-V0-029`: SwiftPM XCTest execution evidence MUST come only from the
  serial native text profile `swift-xctest`, which keeps pass, assertion failure
  and XCTSkip distinct. That profile MUST NOT request `--parallel` or
  `--xunit-output` and MUST refuse any report file. The actual parallel xUnit
  file, in which XCTSkip is an ordinary passing testcase without a `file`
  attribute, is retained only as a negative witness. TCQ JUnit import MUST leave
  its rows unkeyed, so they cannot key criterion evidence. Status: proposed
  (V1-0597).
- `TRE-V0-030`: A plan whose invocation sets `retireDetachedDescendants` MUST,
  on timeout, interruption and normal leader exit, retire every process the
  phase owns that left the leader's process group, before the leader is reaped.
  Ownership MUST be proved only by ppid ancestry from the identity-verified live
  leader or by a fresh random per-phase `CORVINT_TEST_RUNNER_OWNER` token in
  the process's initial environment; the plan cannot supply that key. Each pid
  plus kernel start time MUST be re-verified immediately before SIGSTOP and
  SIGKILL, so unrelated, pre-phase, zombie and reused identities are never
  signalled. An unreadable identity or owner token is uncertainty, never proof
  of exit or non-ownership, and a failed later scan still kills processes
  already stopped. A token read refused while the identity still reads as
  alive MUST be retried for a bounded nominal wait (1 s per identity, 2 s per
  retirement call, polled every 5 ms) before it is retained as a problem,
  because Darwin refuses `KERN_PROCARGS2` for an exiting or not yet executed
  process while `kern.proc.pid` still reports it alive; an identity that
  persists past that bound is still retained as a problem (amended, accepted in
  decision 0519, V1-1152). Owned processes that cannot be signalled (foreign uid,
  refused stop, unreadable identity) or survive SIGKILL, and an unconverged,
  failed or unbounded scan, MUST be
  retained as `unretired` or `problems` in the execution's `retirement` and MUST
  become an `execution-boundary` problem, so no complete or passing observation
  hides them. The flag excludes graceful interrupt and is refused before launch
  where retirement is unproven (everything except Darwin arm64/amd64). New
  `swift-xctest` plans set it. Historical plans and receipts keep their bytes
  and identities, so existing receipts still bind. A historical `swift-xctest`
  plan that differs from the rebuilt profile only by the absent flag MUST be
  refused for new execution with a re-plan diagnosis, because running it
  without retirement could hide a leak and running it with retirement would
  break its receipt's invocation binding. No plan may add the flag where its
  profile does not. The retirement record MUST be retained before any fallible
  post-run check. Status: proposed (V1-0613).

| Requirements | Source/tests | Evidence |
| --- | --- | --- |
| TRE-V0-029 | `platform/xctest.go`; `TestSwiftPMXUnitSkipLossCannotSatisfyExecution`, `tcq` `TestSwiftPMXUnitRowsStayUnkeyed`, opt-in `TestSwiftPMXCTestLiveThreeOutcomes` | Actual `swiftpm-xunit-parallel.xml` and shared-executor `swift-xctest-three.txt` fixtures; live three-outcome run through `/usr/bin/swift`, exit 1, complete PASSED/FAILED(ASSERTION)/SKIPPED |
| TRE-V0-030 | `internal/groupreap/retire*.go`, `execute_unix.go`, `platform/xctest.go`; `TestRetirerProvesOwnershipBeforeSignalling`, `TestRetirerReportsUnconvergedForkStorm`, `TestRetirerKeepsReadFailuresAsUncertainty`, `TestRetirerWaitsOutRefusedTokenReads`, `TestRetirerRetiresDetachedDescendants`, `TestExecuteRetiresDetachedDescendants`, `TestExecuteRetirementAdmission`, `TestExecuteRefusesUnprovenRetirement`, `TestRetirementFailureHidesPassingObservation`, `TestHistoricalPlanByteIdentity`, `corvint-test-runner` `TestPreRetirementXCTestPlanRefusedForExecution`, `TestRetirementSurvivesLaterExecutionFailure`, opt-in `TestSwiftPMXCTestLiveDetachedTeardown` | Live `/usr/bin/swift` `ProofTests.Hang/testHang`: without retirement the detached xctest and Foundation `/bin/sleep` helper survive the group kill; with it both are retired by ancestry on timeout and on interruption. Batch F integration with `TRE-V0-034` (live rerun, Darwin arm64): the unflagged run leaves no survivors and no retirement report, and the flagged runs still report both by ancestry |

Recorded limits. The shared executor admits only a regular-file executable, so
the live witness uses the `/usr/bin/swift` shim; the toolchain `swift` symlink is
refused. Generic JUnit profiles (bun-test, deno-test, pytest) still decode this
XML as complete if a caller pins a wrapper that writes it; that is the
trusted-local-executable boundary of `TRE-V0-004`, not SwiftPM qualification. AFU
run ingest of arbitrary JUnit files is outside CEM execution and unchanged.

Retirement limits. PID reuse is possible only between the final identity
check and the signal. Processes started through launchd or XPC, and
non-descendants that scrub the owner token, are NOT_OBSERVED and not retired.
Linux and other platforms refuse the flag rather than claim cleanup. The group
kill remains containment only; `retirement` is the durable cleanup record. An
unrelated same-uid process that stays unreadable for the whole token wait adds
up to 1 s to that retirement call (2 s for all such identities together) and is
still retained as a problem; the measured Darwin exit window was under 10 ms
idle and up to about 120 ms under fork and CPU load (V1-1152).

Rollback removes the two fixtures, their tests and this section. The
`swift-xctest` profile and historical plan and receipt bytes are unchanged.
Rolling back `TRE-V0-030` removes the flag from `buildXCTest`, the
`retirement` field and `internal/groupreap/retire*.go`; plans that set the
flag would then be refused as unknown fields rather than run uncleaned.

## Jasmine (experimental)

Decision 0437 accepted Jasmine as an addition to the CEM 1.0 main-framework set under V1-0592.
This slice adds the fixed experimental dynamic profile `jasmine` (V1-0860) for one pinned tuple:
the `jasmine` 7.0.0 CLI with `jasmine-core` 7.0.2 on Node 22.23.3, macOS 26.6.2 arm64. Jasmine 7
ships no machine-readable file reporter; the only official reporter package,
`@jasminejs/reporters` 1.1.0, contains a console reporter alone. The profile therefore embeds one
closed, pinned reporter shim, `internal/testrunner/dynamic/reporters/jasmine.cjs`, that copies named
fields of Jasmine's own reporter events (`jasmineStarted`, `suiteDone`, `specDone`, `jasmineDone`)
into one report and decides no outcome. The Go parser owns every validation.

- `TRE-V0-031`: The `jasmine` profile MUST execute one independently pinned Jasmine CLI entry point
  (`node_modules/jasmine/bin/jasmine.js`, run by the pinned `node` tool) with the fixed argv
  `--reporter=<ReportDir>/jasmine.cjs`, then `--config=<Config>` only when a configuration is
  supplied, then the literal spec-file selectors. It writes the embedded shim into the fresh report
  directory, declares exactly one report, `jasmine.json`, and admits exit 0 as success and exit 3
  (failed) as the only failure exit; Jasmine's 1 (load error), 2 (incomplete) and 4 (premature exit)
  remain unadmitted. A selector containing `=` (which Jasmine reads as an environment assignment) or
  a backslash (a glob escape), or equal to a Jasmine subcommand name (`init`, `examples`, `help`,
  `version`, `enumerate`; `init` and `examples` write project files), refuses before launch, together
  with every shared dynamic selector refusal. The
  acquisition pins are the npm registry tarballs `jasmine-7.0.0.tgz` (sha256 `9cc640c5…efafb`),
  `jasmine-core-7.0.2.tgz` (sha256 `b28b620d…136a5`) with their npm sha512 integrity values, the
  lockfile, the entry point and the shim hash, retained in
  `internal/testrunner/dynamic/testdata/jasmine/provenance.json`. Status: proposed (V1-0860).
- `TRE-V0-032`: The parser MUST accept exactly the `corvint-jasmine/0` record with its started and
  done events, at most MaxTests specs and suites and unique non-empty suite IDs; anything else
  refuses. A test identity is the spec's source-root-relative file, `::`, and Jasmine's native
  fullName, which MUST equal its parent suite's reported fullName, one space and the spec description
  (the description alone at top level); each suite's fullName MUST relate to its parent's the same
  way, so the chain spells the suite descriptions joined by single spaces
  (`jasmine-identity-conflict`). Validation compares the reported strings in place and builds no
  joined name. Only native `passed` maps to PASSED; `failed` maps
  to FAILED; `pending`, `notApplicable` and `excluded` map to SKIPPED; any other status is UNKNOWN.
  Retry information is NOT_APPLICABLE because Jasmine has no retries. A failed suite or suite error,
  a global error, parallel mode, a spec-count mismatch, an outcome or overall-status contradiction,
  an incomplete or unknown overall status, a file outside the root, or a missing report makes the
  observation incomplete, so no PASSED state survives. Status: proposed (V1-0860).
- `TRE-V0-033`: When selectors are present, every reported spec file MUST be one of the
  root-joined selected files (`jasmine-unselected-file`) and every selected file MUST report at least
  one spec (`jasmine-selector-without-specs`), because Jasmine silently ignores a missing file and
  exits 0. Without selectors no reconciliation applies. Status: proposed (V1-0860).

| Requirements | Source/tests | Evidence |
| --- | --- | --- |
| TRE-V0-031 | `build.go`, `reporters/jasmine.cjs`; `TestJasmineBuildIsFixed`, opt-in `TestJasmineBuildExecuteParse` | `testdata/jasmine/provenance.json`: npm integrity values re-verified against the tarball bytes; shim hash checked by the fixture tests |
| TRE-V0-032 | `jasmine.go`; `TestJasmineRunnerGeneratedReports`, `TestJasmineParserRefusals`, `TestJasmineIdentityValidationDoesNotAmplify`, opt-in `TestJasmineBuildExecuteParse` | Seven runner-generated reports (mixed pass/fail/xit/pending/nested, passing, beforeAll/afterAll hooks, top-level afterAll, focused, empty, missing selector) from the pinned tuple; live common-executor run: exit 3, complete, 2 passed, 1 failed, 2 skipped; a load error refuses with no report |
| TRE-V0-033 | `jasmine.go`; `TestJasmineRunnerGeneratedReports` (`missing.json`), `TestJasmineParserRefusals`, opt-in `TestJasmineBuildExecuteParse` | Live common-executor run with a missing selected file: exit 0, incomplete with `jasmine-selector-without-specs` |

Recorded limits. One tuple only: other Jasmine, jasmine-core or Node versions and other operating
systems are NOT_RUN. Parallel mode refuses as unqualified and is NOT_RUN. ESM `.mjs` specs,
TypeScript loaders, helpers and `spec/support` default-config discovery in a configured project are
NOT_OBSERVED. The shim and entry point are hashed, but the transitive dependency closure of the
installed tree is not proven. Jasmine affected-test selection is not added. A spec in a file
outside the root keeps its absolute filename as an identity and is incomplete.

Rollback removes the additive `jasmine` profile, the embedded shim, `jasmine.go`, its tests and
`testdata/jasmine`. No shared executor, other profile, queue, store or frozen wire changes.

## Killed-run receipts (experimental)

A runner-killed phase has no native exit status. The executor already records
`exitCode` `-1` for it, but the strict wire parser behind `DecodeDocument`
refused every negative number, so a timed-out run's receipt could not be read
back (V1-1025). This slice states the exit-code domain and makes the producer
and both receipt decoders agree.

- `TRE-V0-040`: A receipt phase MUST record `exitCode` as the native exit
  status 0..255 when its process exited normally, and exactly `-1` when it
  reported no exit status: it was signal-terminated by the runner's timeout,
  interruption or output-overflow kill or by an external signal, or it never
  started. `-1` is outside every admitted exit profile, so shared `Normalize`
  reports `runner-exit` (with `timeout`, `interrupted` or `output-overflow` when
  that phase flag is set), the observation is incomplete and resolved states
  become `UNKNOWN`. A killed run is never reported complete. No signal number is
  recorded; the phase flags carry the termination classification. Status:
  proposed (V1-1025).
- `TRE-V0-041`: Runner plan and receipt decoders (`DecodeDocument` and the CEM
  candidate runner binding) MUST admit a negative number only as an `exitCode`
  object member equal to `-1`, and refuse any other negative number with
  `negative-document-integer`. They MUST refuse with `killed-run-complete` a
  receipt whose observation is complete while a phase has exit code `-1` or a
  set `timedOut`, `interrupted` or `overflow` flag. Duplicate, unknown and
  trailing fields still refuse, and an admitted receipt re-encodes
  byte-identically. Status: proposed (V1-1025).

| Requirements | Source/tests | Evidence |
| --- | --- | --- |
| TRE-V0-040 | `execute_unix.go`, `Normalize`; `TestKilledRunReceiptRoundTrip` | An actual executor run killed at its one-second timeout records phase exit `-1` with `timedOut`, and normalizes incomplete |
| TRE-V0-041 | `document.go` `StructuralBytes`, `CheckReceipt`; `cemcandidate` `runnerBinding`; `TestKilledRunReceiptRoundTrip`, `TestRunnerBindingAdmitsKilledRunReceipt` | Before the change the same receipt failed with `invalid-json: negative numbers are outside the wire integer range`; after it decodes and re-encodes byte-identically, and the named refusals hold |

| Code | Implemented check | Emitters |
| --- | --- | --- |
| `negative-document-integer` | A plan or receipt document carries a negative number other than an `exitCode` member equal to `-1`. | `internal/testrunner/document.go` `StructuralBytes` |
| `killed-run-complete` | A receipt claims a complete observation alongside a killed, timed-out, interrupted or overflowed phase. | `internal/testrunner/document.go` `CheckReceipt` |

A live Swift 6.4 swift-xctest run of a hanging XCTest, killed at a 20-second
timeout, produced a receipt with TEST exit `-1` and `timedOut`. It was
incomplete (`runner-exit`, `timeout`), and the base decoder refused it as
`invalid-json`; the changed decoder reads it back byte-identically.

Failure modes. A receipt
whose producer omitted the termination flags on a `-1` phase still decodes, and
still cannot be complete. Masking blanks only the sign byte of a tokenized
number, so string contents that spell `-1` are data.

Non-goals. No signal-number or termination-cause field is added; that would
change every receipt's bytes. Exit-profile lists keep their 0..255 domain. The
CEM wire parser is unchanged.

Rollback removes `StructuralBytes`, `CheckReceipt` and the two call sites.
Killed-run receipts then refuse again as `invalid-json`; no other receipt or
plan byte, digest or decode result changes.

## Gradle per-run user home and daemon ownership (experimental)

Java derives `user.home` from the account database, not `HOME`, so the fresh
executor HOME did not scope Gradle: every `gradle-junit` run read the
account's `~/.gradle` properties, init scripts and caches and registered its
single-use daemon in the shared daemon registry (V1-0622). Gradle 9.6.1 with
OpenJDK 17.0.20.1 on macOS 26.6.2 arm64 is the only observed tuple.

- `TRE-V0-042`: The `gradle-junit` profile MUST pass `--gradle-user-home`
  fixed at `<ReportDir>/.home/.gradle`, beneath the executor's fresh private
  HOME, together with `--offline` and `--no-daemon`. Its argv MUST NOT carry
  `--daemon`, `--stop`, `--status`, `--foreground`, `--continuous`, any
  `-Dorg.gradle.daemon*` or `-Dgradle.user.home` member, and its environment
  only `JAVA_HOME`. The account's Gradle user home, its credentials, init
  scripts, caches and daemon registry are therefore never an input or output of
  a run. External module dependencies cannot resolve offline in an empty home,
  so test libraries MUST be project-local pinned `InputFiles`; their names and
  digests are the library provenance retained by the plan. Status: proposed
  (V1-0622).
- `TRE-V0-043`: New `gradle-junit` plans MUST set `retireDetachedDescendants`
  (`TRE-V0-030`), so the forked single-use daemon and its test worker, which
  leave the client's process group, are retired by ancestry on timeout,
  interruption and leader exit, with every survivor retained as `unretired` or
  a problem. Cleanup MUST NOT issue `gradle --stop`, `killall` or any
  registry-wide or name-based signal, so ambient daemons, including those of
  the account home, are untouched. Historical plans and receipts keep their
  bytes and identities. A plan whose invocation is exactly the historical
  `gradle-junit` form (the rebuilt one without the flag and without the
  `--gradle-user-home` pair) MUST be refused for new execution with the
  diagnosis `plan predates per-run Gradle user home; re-plan it`; any other
  difference keeps the generic profile-mismatch refusal. Execution is refused before launch where retirement is unproven.
  Status: proposed (V1-0622).

| Requirements | Source/tests | Evidence |
| --- | --- | --- |
| TRE-V0-042 | `platform/jvm_build.go` `gradleUserHome`; `TestGradlePerRunUserHomeAndDaemonOwnership`, opt-in `TestGradleLiveOwnership` | Failing before the change (no `--gradle-user-home`, no retirement). Live run: every scenario creates `<ReportDir>/.home/.gradle/daemon`; no new entry in the account daemon directory came from these runs |
| TRE-V0-043 | `platform/jvm_build.go`; `corvint-test-runner` `admittedInvocation` `preIsolationGradle`; `TestPreIsolationGradlePlanRefusedForExecution`, `TestPinnedGradleManifestFIFORefusedBeforeExecution`, opt-in `TestGradleLiveOwnership` | Live: pass/fail/`@Disabled` give exit 1, complete `proof.ProofTest#passes()` PASSED, `#fails()` FAILED, `#skipped()` SKIPPED; a no-match filter gives exit 1 with no report; a 90 s timeout and an interruption after test-body readiness each give exit `-1`, incomplete, and two processes retired by `ANCESTRY` with no `unretired` entry and no recorded test-body ancestor alive; a scoped ambient sentinel daemon survived all runs |

Failure modes. A project whose build needs a remote repository fails offline
in the empty home; that is retained as a native setup failure, not resolved
from the shared cache. Settings or init code under the project remain trusted
local dependencies (`TRE-V0-004`).

Non-goals. A shared read-only dependency cache (`GRADLE_RO_DEP_CACHE`), wrapper
distribution download, configuration cache and build cache are not admitted.

Recorded limits. Linux and every non-Darwin platform now refuse `gradle-junit`
execution because `TRE-V0-030` retirement is unproven there. This known limit
is accepted for now; Linux retirement is a separate follow-up, and Linux live
qualification is NOT_OBSERVED. Gradle processes started outside the run's
ancestry (for example by a toolchain provisioning service) are not retired.

Rollback removes `gradleUserHome`, the four argv members and the flag from
`buildJVMProject`, plus these tests. Plans made before rollback would then be refused
as not matching the rebuilt profile; historical receipts still bind.

## Storybook test-runner skip tags (experimental)

The legacy `storybook-test-runner` profile accepted a non-empty `Target` and
silently ignored it, so a caller's skip selection was lost (V1-0612). The
native `--skipTags` option of `@storybook/test-runner` 0.26.0 is a runner
option, not forwarded to Jest; it sets `STORYBOOK_SKIP_TAGS`, which the runner
splits on `,` and which overrides configured `tags.skip`, and it turns each
matching story into a skipped Jest test.

- `TRE-V0-044`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0612) For `storybook-test-runner`, an empty `Target` MUST leave the
  historical argv byte-identical. A non-empty `Target` MUST be a
  comma-separated list of 1..16 distinct tags, each matching
  `[A-Za-z0-9][A-Za-z0-9_-]{0,63}`, and MUST be appended as exactly one final
  `--skipTags <Target>` pair after any `--url`. Any other value, including
  option-like, empty, duplicated, whitespace, path, shell or non-ASCII members,
  MUST be refused before a plan exists, so no argv is injected and skip
  semantics are never silently omitted.

| Requirements | Source/tests | Evidence |
| --- | --- | --- |
| TRE-V0-044 | `dynamic/build.go` `storybookSkipTags`; `TestStorybookSkipTagsFixedArgv` | Failing before the change (base argv had no `--skipTags`); option semantics read from the 0.26.0 package source; live Storybook 10.6.1 / test-runner 0.26.0 / Jest 30.5.2 through `corvint-test-runner`: `Target=skip-test` gave complete PASSED, FAILED and SKIPPED with exit 1, an empty `Target` ran the tagged story, and no process survived (2026-10-15 V1-0612/V1-0620 build log) |

Recorded limits. One live tuple was run on macOS arm64 (2026-10-15) with
Node 22.23.3 and host Chrome 155.0.8059.80, not the 154.0.8037.59 the tickets
recorded. The tuple qualifies only without an optional
`.storybook/test-runner` config: with one, collection still fails natively
(`module.register` unsupported under Jest) and the profile reports it as
incomplete, so that configuration stays BLOCKED. The original failure's input
hashes were not retained; the reproduction's are in the build log. Include and
exclude tags are not admitted.

Rollback removes `storybookSkipTags` and the `--skipTags` append. A plan with a
non-empty `Target` would then rebuild without the pair and be refused as not
matching the profile; empty-`Target` plans are unchanged.

## RSpec exec prefix and container path mapping (experimental)

GitHub #1333 (V1-2101): `runnerAdmission: corvint-test-runner/0` admits only
corvint-test-runner plans and receipts, but the `rspec` profile invoked the
executable directly with `--format json --out <host reportDir>/rspec.json`. A
containerised run therefore needed an unbound wrapper that mounted the report
directory at the identical host path. The Ruby provider's
`--exec-prefix`/`--formatter corvint-rspec-json/0` document records
shared-example inclusion lines, but no admission profile accepts that document.

Route chosen: the exec prefix, host-to-container path mapping and the Corvint
formatter become optional members of the `rspec` plan request (route A of the
issue). They are part of the plan's request, so the plan identity, the
receipt's `planSha256` and the runner's rebuild of the invocation at `run` all
bind them, and the one admitted profile `corvint-test-runner/0` and its
existing `CEM-V1-281` to `CEM-V1-285` checks admit the receipt without any new
admission code. The other route, admitting the Ruby provider's document, was
not taken: it would add a second, provider-shaped admission profile with
different binding semantics to the CEM admission surface.

- `TRE-V0-081`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2101) A plan request
  MAY carry `execPrefix` (an argv prefix), `pathMap` (host-to-container path
  pairs) and `formatter`, only for `rspec`. The registry and the dynamic
  builder MUST refuse any of them for every other runner before a plan
  exists. The executable stays the independently pinned `--executable`
  identity, and the prefix words follow it in argv order before the fixed
  RSpec arguments. A present prefix MUST hold at most 64 words, each non-empty, at most
  4096 bytes and free of NUL, CR and LF. A secret-shaped prefix MUST be
  refused, as it would be retained in the plan.
- `TRE-V0-082`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2101) A `pathMap`
  MUST have a non-empty `execPrefix`, and MUST hold 1..8 mappings. Each host
  path MUST be absolute, clean and not the root, and each container path MUST
  be absolute, clean, slash-separated and not `/`. No two host paths, and no
  two container paths, may nest. The report directory MUST lie under one
  mapping. Every runner-visible path the profile writes into argv (the report,
  the formatter, an absolute `--options` file and absolute selectors) MUST be
  rewritten through the mapping, and an absolute path that is not clean or
  lies outside every mapping MUST be refused, so `..` cannot escape a mount. Relative selectors and configuration pass unchanged and
  resolve in the prefix's working directory. Report files are still read only
  from the fresh host report directory.
- `TRE-V0-083`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2101) The prefix,
  mapping and formatter MUST be bound into the plan identity. A receipt MUST
  NOT be replayable under a different prefix, mapping or formatter: `run`
  MUST refuse a plan whose invocation no longer matches the profile rebuilt
  from its request, and plain CEM admission MUST refuse a receipt whose
  `planSha256` no longer matches the edited plan as `runner-receipt-tampered`.
- `TRE-V0-084`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2101) The only
  admitted `formatter` value is `corvint-rspec-json/0`, the Ruby provider's
  Corvint formatter. Under it the profile MUST write those exact formatter
  bytes as the profile-owned file `corvint_rspec_formatter.rb` beneath the
  fresh report directory. Its argv MUST be `--require <formatter>`,
  `--format CorvintRSpecJsonFormatter` and `--out <rspec-corvint.json>`, both
  paths mapped. The report inventory is exactly `rspec-corvint.json`. Any
  other formatter value MUST be refused.
- `TRE-V0-085`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2101) The
  `rspec-corvint.json` parser MUST require `shared_group_inclusion_backtrace`
  on every example. It MUST retain each frame's `inclusion_location`, in
  native order, as that test's `inclusionLocations`, and MUST refuse a
  missing or null backtrace, more than 64 frames, or a frame whose location
  is missing, empty, over 4096 bytes or multi-line. An example with an empty
  backtrace carries no member. The stock `rspec.json` report keeps its
  historical parse and never decodes that member.
- `TRE-V0-086`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2101) With
  `execPrefix`, `pathMap` and `formatter` all absent, the `rspec` argv, report
  inventory and parse, and every other profile, MUST stay byte-identical. The
  new request and test members MUST be omitted from JSON when empty, so
  historical plan and receipt bytes and identities do not change.

Failure modes. A prefix that runs a different image or tool than the caller
intended is caught only through the bound plan identity, not by the runner. An
unmapped report directory, overlapping mappings, an unknown formatter, or a
prefix on another runner is refused before a plan exists. A report lacking
the inclusion backtrace is refused as unparseable rather than read as an
example with no inclusions. Editing the prefix, mapping or formatter after
approval is refused by `run` without launching anything, and by CEM
admission as tampered.

| Requirements | Source/tests | Evidence |
| --- | --- | --- |
| TRE-V0-081 | `testrunner/types.go` `Request.HasRSpecOnlyMembers`; `registry/registry.go` `Build`; `dynamic/build.go`; `dynamic/rspec_prefix.go` `rspecPathMapper`; `TestRSpecOnlyMembersRefusedForOtherRunners`, `TestRSpecExecPrefixRefusals` | Focused unit tests, 2026-10-20 |
| TRE-V0-082 | `dynamic/rspec_prefix.go` `rspecPathMapper`, `buildRSpec`; `TestRSpecExecPrefixPathMapArgv`, `TestRSpecExecPrefixRefusals`; `cmd/corvint-test-runner` `TestContainerisedRSpecPlanRunReceipt` | A fake docker-shaped prefix maps `--require` and `--out` back to its host mount and refuses an unmapped path; the runner read the report from the host directory |
| TRE-V0-083 | `cmd/corvint-test-runner` `admittedInvocation`; `TestContainerisedRSpecPlanRunReceipt` subtests; `cemcandidate` `TestCEMV1280AdmitsContainerisedRSpecReceipt` | Edited prefix, mapping or formatter refused by `run` with no report directory created, and by plain admission as `runner-receipt-tampered` |
| TRE-V0-084 | `rubytestprovider/runner.go` `CorvintFormatterSource`, `FormatterClass`; `dynamic/rspec_prefix.go` `buildRSpec`; `TestRSpecExecPrefixPathMapArgv` | The fake container found the profile-written formatter at the mapped path |
| TRE-V0-085 | `dynamic/parse.go` `parseRSpec`; `dynamic/rspec_prefix.go` `inclusionLocations`; `TestRSpecCorvintInclusionLocations`, `TestContainerisedRSpecPlanRunReceipt` | The admitted receipt carries `inclusionLocations` for each shared-example inclusion |
| TRE-V0-086 | `TestRSpecHistoricalArgvUnchanged`; existing `cmd/corvint-test-runner` historical byte tests | Historical rspec argv and frozen plan/receipt bytes unchanged |

Recorded limits.
- Live qualification is NOT_RUN: no Ruby, RSpec or Docker ran. The tests use
  a sh stand-in for the container tool and a synthetic Corvint-formatter
  report, not one generated by RSpec.
- The image identity is bound only as far as the prefix text binds it: the
  plan pins a tag-only image name, not its content, unless the prefix names
  an `@sha256:` digest. The dependency closure inside the container stays
  NOT_OBSERVED.
- The container's working directory is set by the prefix. Relative selectors
  and `--options` resolve there, not under the plan's `root`.
- The formatter's `corvint_runtime.container` identifier is ignored by this
  parser and is not checked against anything.
- The runner sets `HOME` to a fresh directory, so tool plugins found through
  the user's home (for example `docker compose` CLI plugins) are unavailable.
  Use `docker run` or a standalone binary.
- The report directory is created 0700 on the host. A container running as a
  different non-root uid may be unable to write it on Linux.
- Inclusion locations are retained as RSpec reports them, so they may be
  container-relative paths.

Rollback removes the three request members, `InclusionLocations`,
`dynamic/rspec_prefix.go`, the registry guard, the exported formatter accessors
and these tests, and this section. Plans carrying the new members would then
fail closed decoding. Historical plans and receipts are unaffected.

## Native qualification matrix 2026-10-17 (V1-0593)

This section records actual `TRE-V0-009` qualification of the 18 native
profiles on one host tuple: macOS 26 (Darwin 25.6.0) arm64, Apple clang 21.0.0.
Every witness ran through `corvint-test-runner plan` then `run --approve`, so it
used the shared executor. Each case directory
`/private/tmp/cem10-build/native-2026-10-17/cases/<case>/` keeps the request,
plan, tools, receipt and summary (command, CLI and executable SHA-256). Its `r/`
holds the raw native report and streams. `CLI-BUILDS.txt` names the CLI build
behind each case. Three live failures led to these requirements. Each failure
was incomplete, never falsely complete.

- `TRE-V0-045`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0593) `cargo-doctest` MUST refuse a selector containing Unicode
  whitespace before a plan exists. Rustdoc splits `--test-args` on whitespace,
  and every generated doctest name (`src/lib.rs - item (line N)`) contains
  whitespace. Such a selector otherwise silently runs zero doctests.
- `TRE-V0-046`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0593) `catch2` MUST pass its exact selectors as one quoted, comma-joined
  test-spec argument. Catch2 3.16.0 ORs the names in that argument and ANDs
  separate arguments. The existing refusal of wildcard, tag, exclusion,
  escape, quote and option-like selectors is unchanged, and a selector whose
  prefix is `exclude:` (any case) MUST be refused too, because Catch2 3.16.0
  interprets it as an exclusion even inside quotes.
- `TRE-V0-047`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0593) `catch2` MUST accept native exit 4 (`AllTestsSkippedExitCode`) as
  success only when the report has at least one test and every test is
  `SKIPPED`. Exit 4 with any other report, and exit 4 for any other runner,
  stays an exit/report contradiction and is incomplete.

| Requirements | Source/tests | Evidence |
| --- | --- | --- |
| TRE-V0-045 | `native/native.go` `Build`; `TestBuildExactSelectorsAndFixedProfiles` | Failing before: `cases/cargo-doctest-pass` selected 0 of 1 doctests (`NO_TESTS`, incomplete). After: `cases/cargo-doctest-select-refused` is refused at plan |
| TRE-V0-046 | `native/native.go` `Build`; `TestCatch2ExactSelectionAndAllSkippedExit`, `TestCatch2RefusesExcludePrefix` | Failing before: `cases/catch2-mixed` gave exit 2 with no tests. After: `cases/catch2-mixed-r2` gave exit 42, complete PASSED, FAILED and SKIPPED |
| TRE-V0-047 | `native/native.go` `Parse`; `TestCatch2ExactSelectionAndAllSkippedExit` | Failing before: `cases/catch2-skip` gave exit 4 and `EXIT_REPORT_CONTRADICTION`. After: `cases/catch2-skip-r2` gave exit 4, complete SKIPPED. A passed-only or empty report with exit 4 stays incomplete |

Matrix. W means an actual shared-executor witness, P means parser-only and N
means none. Retry is not applicable to any native profile, so it is not a
column. Nextest pins `retries=0`, Ginkgo pins `flake-attempts=1`, and no other
native profile has a retry option. Nextest retry parsing stays P. Case names
are the directory names under `cases/`.

| Profile | Tuple | pass / fail / skip | zero | collection, build or infra | timeout and cleanup |
| --- | --- | --- | --- | --- | --- |
| go-test | go1.27.2 | W `go-pass`, `go-fail`, `go-skip`, `go-mixed` | W `go-zero-nomatch`, `go-zero-notestfiles` | W `go-buildfail` | W `go-timeout` |
| cargo-test | Rust/Cargo 1.99.1 stable | W `cargo-test-pass`, `-mixed`, `-skip` | W `cargo-test-zero` | W `cargo-test-buildfail` | W `cargo-test-timeout` |
| cargo-doctest | Rust/Cargo 1.99.1 | W `cargo-doctest-mixed` (pass, fail, `ignore`) | W `cargo-doctest-zero` | W `cargo-doctest-buildfail-lib` | W `cargo-doctest-timeout` |
| cargo-integration | Rust/Cargo 1.99.1 | W `cargo-integration-pass`, `-mixed`, `-skip` | W `cargo-integration-zero` | W `cargo-integration-missingtarget` | W `cargo-integration-timeout` |
| cargo-bin | Rust/Cargo 1.99.1 | W `cargo-bin-pass`, `-mixed`, `-skip` | W `cargo-bin-zero` | W `cargo-bin-missingtarget` | W `cargo-bin-timeout` |
| nextest | cargo-nextest 0.9.146, Rust 1.99.1 | W `nextest-pass`, `nextest-mixed` | W `nextest-zero` | W `nextest-buildfail` | W `nextest-timeout` |
| dotnet-vstest-{nunit,mstest,xunit} | SDK 9.0.316, runtime 9.0.18, Test SDK 17.14.1; NUnit 4.3.2 with adapter 5.0.0; MSTest 3.6.4; xunit 2.9.2 with runner 2.8.2 | W `vstest-<fw>-pass`, `-fail`, `-skip`, `-mixed` | W `vstest-<fw>-zero` | W `vstest-<fw>-collection`, `-infra` | W `vstest-<fw>-timeout` |
| dotnet-mtp-{nunit,mstest,xunit} | runtime 9.0.18, MTP and TrxReport 2.4.1; NUnit 5.0.0 with adapter 6.3.0; MSTest.Sdk 4.4.1; xunit.v3.mtp-v2 4.0.1 | W `mtp-<fw>-pass`, `-fail`, `-skip`, `-mixed` | W `mtp-<fw>-zero` (exit 8) | W `mtp-<fw>-collection`, `-infra` | W `mtp-<fw>-timeout` |
| ctest | CMake/CTest 4.4.4 | W `ctest-pass`, `-fail`, `-skip`, `-mixed` | W `ctest-zero`, `ctest-disabled-r2` | W `ctest-missing` | W `ctest-timeout` |
| googletest | GoogleTest 1.18.0 | W `googletest-pass`, `-fail`, `-skip`, `-disabled`, `-mixed` | W `googletest-zero` | W `googletest-infra`, `-collection` | W `googletest-timeout` |
| catch2 | Catch2 3.16.0 amalgamated | W `catch2-pass-r2`, `-fail-r2`, `-skip-r2`, `-hidden-r2`, `-mixed-r2` | W `catch2-zero-r2` | W `catch2-infra-r2`, `-collection-r2` | W `catch2-timeout-r2` |
| cmocka-xml | CMocka 2.0.2 static | W `cmocka-mixed`, `-pass-only`, `-skip-only` | W `cmocka-zero`, `-wrong-target` | W `cmocka-test-setup`, `-test-teardown`, `-group-setup`, `-group-teardown` | W `cmocka-timeout` |
| boost-test-junit | Boost 1.92.0 headers | W `boost-mixed`, `-pass`, `-selected-pass-r2`; opt-in `TestBoostTestLiveExecution` PASS | W `boost-zero` (exit 200, no report) | W `boost-infra`, `boost-exception` | W `boost-timeout` |
| ginkgo-v2 | Ginkgo v2.33.0 with Gomega 1.40.0, `go test -c` suite binaries | W `ginkgo-mixed` (pass, fail, runtime skip, pending), `ginkgo-pass`, `-fail`, `-skip` | W `ginkgo-zero` (exit 1, fail-on-empty) | W `ginkgo-beforesuite-fail`, `-beforesuite-skip`, `-ordered`, `-wrong-target` | W `ginkgo-timeout` |

Totals cover 108 applicable cases, 18 profiles times six cases. Before this
work there were 37 W, 30 P and 41 N. After it there are 108 W, 0 P and 0 N.
Every zero-test, build, collection, infrastructure and timeout witness is
incomplete, and its tests are `UNKNOWN` or absent. Every timeout fixture
spawned a descendant `/bin/sleep` with a unique argument. After each serial
batch, `pgrep` found none of them alive (`cleanup-check.txt`).

Observed native behavior retained as fail-closed:
- CTest exits 8 with "No tests were found" for a disabled-only selection.
- MTP exits 8 for zero tests.
- Catch2 exits 2 for a test spec that matches nothing.
- Boost.Test exits 200 with no report for a selector that matches nothing.
- Boost.Test lists filtered-out tests as skipped, so `ExpectedTests` must be
  the full inventory (`boost-selected-pass` MISMATCH retained).
- GoogleTest 1.18.0 reports a `SetUpTestSuite` failure as an unnamed testcase.
  Parsing it fails with `native suite count mismatch` and stays incomplete.

The `cases/` directory also holds MISMATCH runs whose expectation, not the
adapter, was wrong: `ctest-disabled` and `boost-selected-pass`. Their
corrected reruns are listed above.

Toolchains not already on the host were installed in user space under
`/private/tmp/cem10-build/toolchains-2026-10-17/`, with the owner's approval.
Each was checksum-verified. The 2026-10-17 V1-0593 build log lists versions,
URLs and checksums.

Ginkgo v2 live qualification. The fixture module `example.com/gq` pins
`github.com/onsi/ginkgo/v2` v2.33.0 (`h1:C8gBA6Uc2ZEubiV+SXiu5tZnMTwEmXHgkJwGozKtZf8=`)
and `github.com/onsi/gomega` v1.40.0 in `go.sum`, resolved through the Go proxy
and checksum database (`go mod verify`: all modules verified). Its suite
binaries were built offline with `go test -c`; their digests are in
`fixtures/ginkgo/SHA256SUMS`. The `ginkgo-v2` adapter did not change. These
runs replace, for this host tuple, the NOT_RUN statement in "Ginkgo v2
(experimental)" and confirm three of its source-derived limits:
- The reported `SuitePath` equals the symlink-resolved Root in every run.
- A runtime `Skip()` in `BeforeSuite` exits 0 with `SuiteSucceeded` true and the
  reason `Suite skipped in BeforeSuite`; it is incomplete
  (`GINKGO_SUITE_FAILURE_REASON`, `GINKGO_NOT_ENTERED`).
- A leaf failure in an `Ordered` container leaves the next spec skipped with
  zero attempts; it is incomplete (`GINKGO_NOT_ENTERED`).
A suite with no specs exits 1 with the fail-on-empty reason and is incomplete.
A `BeforeSuite` failure is incomplete (`GINKGO_SUITE_PROBLEM`). A Target whose
description differs from the report is refused (`GINKGO_TARGET_MISMATCH`).

Release blockers (TRE-V0-009 tuples not qualified):
- Linux (amd64 and arm64) and Windows tuples for all 18 profiles. Missing: a
  Linux or Windows host.
- Other runtimes. Missing: .NET 8 and 10 runtimes, other MTP and adapter
  versions, a nightly Rust toolchain and other compilers. Stable Cargo JSON is
  still not inferred, and MTP support is not inferred from VSTest.
- Publisher signatures. Missing: `gpg` to verify the Catch2 and CMocka `.asc`
  files; only digests were verified. Five transitive NuGet packages fail
  `dotnet nuget verify` with expired repository countersignatures
  (NU3028/NU3037): microsoft.bcl.asyncinterfaces 6.0.0,
  microsoft.win32.registry 5.0.0, system.diagnostics.diagnosticsource 5.0.0,
  system.security.accesscontrol 5.0.0 and system.security.principal.windows 5.0.0.

Rollback has three parts. Removing the cargo-doctest whitespace refusal
restores silent zero-test selection. Restoring one argument per selector in
`catch2` makes its plans rebuild differently, so existing plans are refused.
Narrowing Catch2 success exits back to {0} makes all-skipped runs incomplete
again. Historical receipts still bind.

## Dynamic qualification matrix 2026-10-17 (V1-0592)

V1-0592 re-ran every dynamic profile's P (pass), F (assertion failure), S (skip), Z (zero tests),
C (collection/setup/build error), R (native retry) and T (timeout and interrupt) cases through
`corvint-test-runner plan`/`run` on macOS 26 arm64 (Rails in a network-disabled Linux arm64
container). Each case directory under `/private/tmp/cem10-build/dynamic-2026-10-17/<profile>/<case>/`
retains the request, plan, receipt, raw native report, phase stdout/stderr and `summary.json` with
the version tuple; prefixed directories (`*-prefix`, `*-launchfail`, `*-noconnect`, ...) retain the
pre-repair or environment-failure runs. W is a witness; W! is witnessed and fail-closed (reported
incomplete, never a pass); WF is witnessed as a complete FAILED run (the native runner attributes
the error to a test); I is interrupt-only (the executor timeout is not witnessed); NA means no native
feature. Before: W 75, parser-only 7, none 61, NA 11. After: W 135 (4 W!, 1 WF), I 1, parser-only 0,
none 7, NA 11.

| Profile | Tuple | P F S | Z | C | R | T |
| --- | --- | --- | --- | --- | --- | --- |
| mocha | Mocha 12.0.3, Node 22.23.3 | W | W | W | W | W |
| jasmine | Jasmine 7.0.0 / core 7.0.2 | W | W | W | NA | W |
| node-test | Node 22.23.3 | W | W | W | NA | W |
| jest | Jest 30.5.2 | W | W | W | W (plain retry W!) | W |
| vitest | Vitest 5.0.3, Vite 8.3.4 | W | W | W | W! | W |
| ava | AVA 8.0.1 | W | W | W | NA | W |
| bun-test | Bun 1.3.11 | W | W | W | W! | W |
| deno-test | Deno 2.9.7 | W | W | W | NA | W |
| playwright | 1.63.0; Chromium 153.0.8010.12, Firefox 155.0, WebKit 26.6 | W | W | W | W | W |
| cypress | 16.1.1 (Electron 41.7.0); Chrome for Testing 154.0.8037.59 | W | W | W | W | W |
| webdriverio | 9.32.0; Chrome for Testing + ChromeDriver 154.0.8037.59 | W | W | W | W! | W |
| testcafe | 3.7.6; remote chrome-headless-shell 153.0.8010.12 | W | W | W | W | W |
| nightwatch | 3.16.0; Chrome for Testing + ChromeDriver 154.0.8037.59 | W | W | W | W | W |
| detox | none | N | N | N | NA | N |
| storybook-test-runner | 0.26.0, Storybook 10.6.1, Jest 30.5.2, Chromium 153.0.8010.12 | W | W | WF | N | I |
| storybook-vitest | Storybook 10.6.1, Vitest 5.0.3, Chromium 153.0.8010.12 | W | W | W | W! | W |
| pytest | pytest 8.4.2, CPython 3.13.15 | W | W | W | NA | W |
| unittest | CPython 3.13.15 and 3.9.6 | W | W | W | NA | W |
| rspec | rspec-core 3.13.6, Ruby 2.6.10 | W | W | W | NA | W |
| minitest | Minitest 5.11.3, Ruby 2.6.10 | W | W | W | NA | W |
| test-unit | test-unit 3.2.9, Ruby 2.6.10 | W | W | W | NA | W |
| rails-test | Rails 8.1.4, Ruby 3.4.11, Minitest 6.0.6 (Linux arm64 container) | W | W | W | NA | W |

No Z, C or T witness became a pass. Every Z witness, every executor timeout and interrupt witness,
the WebdriverIO and Nightwatch missing-shard runs and every W! retry produced an incomplete receipt.
Three setup or collection witnesses are complete FAILED runs, because the native runner reports the
error as a failed test: the Jest and RSpec `before`-hook failures (`jest/hook`, `rspec/hook`; their
module-level collection witnesses `jest/collection` and `rspec/collection` are incomplete) and the
Storybook module-load failure (`storybook-test-runner/collection`: the statically indexed story's
`smoke-test` fails with "The story was missing", attempt failure kind UNKNOWN). That FAILED row is
the runner's own verdict and is not a pass, but the receipt cannot tell a module-load error from a
render failure. `storybook-test-runner/timeout` is also a complete FAILED run: Jest's pinned 15 s
test timeout ended the hanging play function before the 60 s executor timeout
(`timedOut: false`), so storybook-test-runner T is interrupt-only (`storybook-test-runner/interrupt`
is incomplete). Four adapter defects found by the actual runs are repaired:

- `TRE-V0-060`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0592; amended by
  the V1-0592 review follow-up) For `node-test`, a top-level test event at 1:1 named after its file
  (Node's synthetic file-level event, or a real test with that name and position, which Node 22.23.3
  reports with identical fields) MUST NOT become a test, a `no-tests` verdict or a
  `collection-or-hook-error` verdict. It MUST record `ambiguous-test-identity`, leaving the run
  incomplete, and MUST still reconcile against the native summary counts.
- `TRE-V0-061`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0592) For `ava`,
  a TAP comment `# N test(s) remaining in <file>` MUST record `collection-or-hook-error`, so a
  failing `before` hook that left declared tests unrun cannot yield a complete run.
- `TRE-V0-062`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0592) For
  `bun-test`, a suite `line` attribute MUST be admitted, and a failing `(unnamed)` case without a
  `line` (Bun's hook report) MUST be UNKNOWN with `setup-or-runtime-error`, reconciled as the
  suite's declared failure, never a FAILED test.
- `TRE-V0-063`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0592) For
  `storybook-test-runner`, the sole skipped `no-op` row of a file (the runner's placeholder for a
  CSF file without stories) MUST be reconciled against the native counts but MUST NOT become a
  test or retained retry information, so a run of only such files reports `no-tests`.

| Requirements | Source/tests | Evidence |
| --- | --- | --- |
| TRE-V0-060 | `dynamic/native.go` `parseNode`, `nodeFileEvent`; `TestNodeFileLevelEventsAreNotTests` | `node-test/zero-prefix` (complete PASSED before) and `collection-prefix` (complete FAILED before); `zero` and `collection` incomplete after; real collisions `filename-collision`, `filename-collision-fail`, `basename-collision` (dropped or misread before the amendment); `*-fixed` re-runs incomplete `ambiguous-test-identity`, `mixed-fixed` complete |
| TRE-V0-061 | `dynamic/native.go` `parseTAP`; `TestAVARemainingTestsAfterHookFailure` | `ava/hook-prefix` (complete FAILED before); `ava/hook` incomplete after |
| TRE-V0-062 | `dynamic/native.go` `parseXML`, `dynamic/validation.go` `closedXML`; `TestBunNativeJUnitShapes` | `bun-test/mixed-prefix` (refused before), `hook-prefix`; `mixed` complete and `hook` incomplete after |
| TRE-V0-063 | `dynamic/parse.go` `parseJest`; `TestStorybookRunnerEmptyCSFPlaceholderIsNotATest` | `storybook-test-runner/zero-prefix` (complete SKIPPED, exit 0, before); `zero` incomplete `no-tests` after |

The fixtures are the raw reports of those runs, hash-pinned in
`internal/testrunner/dynamic/testdata/qualification-0592/provenance.json` with their receipt hashes.

Browser launch notes. The executor clears the environment, so task-local launchers
(`/private/tmp/cem10-build/toolchains-2026-10-17/`, hashes in the build log) set the browser
path: WebKit needs `DYLD_FRAMEWORK_PATH`, Cypress its cache and binary path, TestCafe a remote
browser, and the legacy Storybook runner a static build served on `127.0.0.1:6107`. Tool
dependencies are declared as executor tools (`cat` for ChromeDriver under Nightwatch, `mkfifo`
for TestCafe's window helper). Nightwatch names its report after the session browser
(`CHROME_154.0.8037.59__a.json`), so the shard manifest must name that file.

Release blockers that remain, each with the missing piece:

- Detox (all cases): no Detox CLI install, no Detox-instrumented iOS `.app` or Android APK, no
  `applesimutils` (a Homebrew-tap install outside the user-space rule) and no booted simulator or
  emulator. Execution stays NOT_OBSERVED.
- TestCafe local browser CLI: macOS Screen Recording permission for the browser is not granted
  (`UnableToAccessScreenRecordingAPIError`, `testcafe/local-screen-recording-blocked`); only the
  remote-browser path is qualified. Chrome for Testing 154 `--headless=new` never followed the
  remote `/browser/connect` redirect (`testcafe/remote-cft154-noconnect`), so the remote witness
  uses chrome-headless-shell 153.0.8010.12.
- storybook-test-runner T: no executor-timeout witness, recorded as not applicable at the qualified
  settings (V1-1544). The profile pins the runner's Jest config and refuses a caller `Config`, so its
  per-test timeout is the pinned 15 s Jest timeout, which fires before any executor timeout of 15 s
  or more and yields a complete FAILED run (`storybook-test-runner/timeout`, executor timeout 60 s).
  The executor's `TimeoutSeconds` (1..1800) is still caller-chosen, so an executor timeout below the
  Jest timeout plus runner and browser start-up is admitted but has no witness: the retained
  Storybook fixture is gone and a new one needs a package install this lane did not run. Only the
  interrupt path (`storybook-test-runner/interrupt`, incomplete) is qualified.
- storybook-test-runner R: the profile pins the runner config and admits no Jest
  `test-runner-jest.config.js`; the runner has no native retry flag. The optional
  `.storybook/test-runner` config remains BLOCKED as recorded under TRE-V0-044.
- W! retries stay fail-closed: Vitest 5.0.3 and storybook-vitest JSON omit per-attempt history
  (`retry-history-missing`); Bun 1.3.11 JUnit and the WebdriverIO 9.32.0 JSON reporter repeat a
  retried test's identity (`ambiguous-test-identity`). A qualified retry needs a native report that
  carries attempts.
- Minitest reported an erroring test (`Minitest::UnexpectedError`) as UNKNOWN, which made the whole
  run unresolved (`minitest/mixed`); assertion failures are witnessed separately
  (`minitest/mixed-noerror`). Superseded by TRE-V0-110 (V1-1541): the erroring case is now FAILED
  with cause UNKNOWN.

Rollback reverts the four parser changes and their tests and fixtures; the matrix stays as
retained evidence. Plans and receipts are unchanged in shape.

## Platform qualification matrix 2026-10-17 (V1-0594)

Gap matrix for the 16 platform, two SQL and one Appium Android profiles against the
`TRE-V0-009` cases: P pass, F assertion failure, S skip, Z zero tests, B collection or
build error, T timeout with cleanup. Retry (R) is not applicable: none of these profiles
requests or reports native retries. Cell values:

- A: actual witness retained on 2026-10-17 through `corvint-test-runner plan`/`run`, with
  version tuple, raw native report, receipt and command.
- H: historic actual native bytes committed as a parser fixture, or a spec-recorded live
  run. Its raw report, receipt and command were purged from `/private/tmp/cem10-build`,
  so it counts as parser-only.
- -: no evidence.

An A or H cell records that a witness exists for the case, not that the profile classified it
as the case name says. The prose below states each outcome. In particular the
`sql-sqllogictest-sqlite` B cell is a retained witness of an engine-rejected query, which the
profile now refuses as `SQL_QUERY_REJECTED` (`TRE-V0-118`); it is not a build-error
classification, and the native runner has no separate build phase.

| Profile | P | F | S | Z | B | T |
| --- | --- | --- | --- | --- | --- | --- |
| swift-testing | A | A | A | A | A | A |
| swift-xctest | A | A | A | A | A | A |
| xcode-xctest | A | A | A | A | A | A |
| junit-platform | A | A | A | A | A | A |
| junit4 | A | A | A | A | A | A |
| testng | A | A | A | A | A | A |
| kotlin-test | A | A | A | A | A | A |
| kotest | A | A | A | A | A | A |
| gradle-junit | A | A | A | A | A | A |
| maven-surefire | A | A | A | A | A | A |
| robolectric | A | A | A | A | A | A |
| android-junit | A | A | A | A | A | A-r |
| espresso | A | A | A | A | A | A-r |
| uiautomator | A | A | A | A | A | A-r |
| bats | A | A | A | A | A | A |
| shellspec | A | A | A | A | A | A |
| sql-pgtap | A | A | A | A | A | A |
| sql-sqllogictest-sqlite | A | A | A | A | A | A |
| appium-uiautomator2-wdio | A | A | A | A | A | A |

A-r: the runner timeout is witnessed and every host process was retired, but the on-device
instrumentation was not retired by the runner (see the Android residual below). It is not a
clean timeout-with-cleanup witness.

Totals over 114 applicable cells:

| | A | A-r | H (parser-only) | NONE |
| --- | --- | --- | --- | --- |
| Before | 0 | 0 | 71 | 43 |
| After (Apple slice) | 18 | 0 | 59 | 37 |
| After (this section) | 111 | 3 | 0 | 0 |

`shader-behavior`, `html-behavior` and `structured-data-behavior` stay unavailable registry
IDs. Their disposition is unchanged: domain compiler success is not assertion-test success.

Apple tuple: macOS 26.6.2 (25G83) arm64; Swift 6.4 (swiftlang-6.4.0.34.1),
`/usr/bin/swift` sha256 `b8763cf2…`; Xcode 27.0 (27A266a), `/usr/bin/xcodebuild`;
independently pinned `xcresulttool` 25115, schema 0.4.0. Evidence lives under
`/private/tmp/cem10-build/platform-2026-10-17/`. Each case directory under `runs/` holds
`request.json`, `plan.json`, `receipt.json`, `command.txt` and the raw `report/`. The
fixtures are under `fixtures/`.

- **swift-testing** (`runs/swift-testing/`):
  - `pass-fail-skip`: complete. Results are PASSED, FAILED (ASSERTION) and SKIPPED; the
    `.disabled` trait is never a pass.
  - `zero` (exit 0): incomplete with `NO_TESTS`.
  - `build-error`: incomplete with `INVALID_NATIVE_REPORT`, because there is no event stream.
  - `timeout` (60 s): incomplete with `PROCESS_TIMEOUT`, and no process survived.
  - The live run found an adapter bug, fixed by `TRE-V0-075`. Before the fix, a selected run
    executed nothing (`selector-anchor-bug-before-fix`).
- **swift-xctest** (`runs/swift-xctest/`):
  - `five-outcomes`: complete. Results are PASSED, FAILED (ASSERTION), SKIPPED from `XCTSkip`,
    and FAILED (INFRASTRUCTURE) for both a thrown error and a `setUpWithError` error.
  - `zero`, `build-error` and `timeout` (45 s) are each incomplete.
  - In `timeout`, the detached `xctest` process and its helper were retired by ancestry, and
    no process survived.
  - Opt-in `TestSwiftPMXCTestLiveThreeOutcomes` and `TestSwiftPMXCTestLiveDetachedTeardown`
    passed on the same package (`runs/swift-xctest/go-live/`). In the contained-only,
    timeout and interrupt modes, no process survived.
- **xcode-xctest** (`runs/xcode-xctest/`), using a handcrafted `XcodeProof.xcodeproj` and
  its shared scheme:
  - `pass-fail-skip`: exit 65. Both xcresult DECODE phases completed, and the results are
    PASSED, FAILED (ASSERTION) and SKIPPED.
  - `zero`: exit 0. The summary has no destination, so the run was refused as incomplete;
    `TRE-V0-117` now maps this exact shape to `NO_TESTS`.
  - `build-error`: exit 65, incomplete; `TRE-V0-117` now records `BUILD_FAILURE` for it.
  - `timeout` (60 s): `testHang` had started, the decode never ran, the run is incomplete,
    and no `xctest` or `xcodebuild` process survived.

Non-Apple tuple, installed user-space under `/private/tmp/cem10-build/toolchains-2026-10-17/`
(owner-approved 2026-10-17; browsers excluded): OpenJDK 17.0.20.1; JUnit Platform Console
Standalone 6.1.3; TestNG 7.12.0; Kotlin 2.4.0; Kotest 6.2.5; Robolectric 4.16.1 with
`android-all-instrumented` 13954326 (jar sha256 `06c4c860…`); Gradle 9.6.1; Maven 3.9.16
with Surefire 3.0.2 as reported; Bats Core 1.14.0; ShellSpec 0.28.1; pgTAP 1.3.4 on
PostgreSQL 17.11; `sqllogictest` db57eba9 on SQLite; Android SDK 36 build, AGP 9.5.0-alpha10
(resolved from `9.+`), API 34 `google_apis` arm64 emulator `cem10-platform-0594` (fingerprint
`google/sdk_gphone64_arm64/emu64a:14/UE1A.230829.050/12077443`); Appium 3.8.0, UiAutomator2
8.7.0, WebdriverIO 9.32.0 and Node 22.23.3. `RUNNER-SHAS` maps every receipt to the runner
binary that wrote it.

- **JVM direct profiles** (junit-platform, junit4, testng, kotlin-test, kotest):
  `pass-fail-skip` is complete with PASSED, FAILED (ASSERTION), SKIPPED and FAILED
  (INFRASTRUCTURE) for a thrown exception. `zero` is incomplete (`NO_TESTS`,
  `EXIT_CONTRADICTION`). `build-error` uses a missing class: junit-platform, junit4 and
  kotlin-test report a FAILED (INFRASTRUCTURE) row; testng reports `CONFIGURATION_FAILED` or a
  missing report; kotest is refused as contradictory JUnit outcomes. `timeout` is incomplete
  with `PROCESS_TIMEOUT` and no surviving process.
- **robolectric**: same four outcomes. `build-error-missing-dependency` is FAILED
  (INFRASTRUCTURE) `initializationError`. `build-error-sdk-not-enabled` found the
  `TRE-V0-077` false pass (`runs/robolectric/pre-0077/`) and is now incomplete.
- **gradle-junit** and **maven-surefire**: `pass-fail-skip` is complete (Gradle classifies the
  thrown exception as FAILED UNKNOWN). `zero` and `build-error` are incomplete: no native
  report. `timeout` is incomplete; the Gradle daemon and worker were retired by ancestry.
- **android-junit**, **espresso**, **uiautomator**: each request binds `emulator-5584`, the
  instrumentation component and both APK hashes. `device-identity.txt` in every case records
  the AVD name, fingerprint, SDK, ABI and the installed `base.apk` sha256, which matched the
  built APKs. `pass-fail-skip` is complete with PASSED, FAILED and SKIPPED (`@Ignore`);
  `ComparisonFailure` and Espresso assertion errors are FAILED UNKNOWN. `zero` (an APK with no
  tests) is `NO_TESTS`. `build-error` is a static initializer failure (FAILED UNKNOWN), because
  a compile error produces no installable APK. `timeout` is incomplete with `PROCESS_TIMEOUT`.
- **bats**, **shellspec**: complete pass-fail-skip; `zero`, `build-error` and `timeout` are
  incomplete.
- **sql-pgtap**, **sql-sqllogictest-sqlite**: separate pass, fail, skip and infrastructure
  cases; `zero` is refused; `timeout` is `PROCESS_INCOMPLETE`. `build-error` differs by profile.
  pgTAP (`runs/sql-pgtap/build-error/`, also `pre-0076/`): psql exit 3 on a syntax error is
  incomplete with `SQL_PROCESS_ERROR`, `no-tests` and `runner-exit`. sqllogictest
  (`runs/sql-sqllogictest-sqlite/build-error/`): the invalid statement is a failing query, not a
  build error. The native runner exited 1 and the receipt is `complete: true` with one FAILED
  `UNKNOWN` suite (`broken.slt:1: query failed`), no problems and no receipt error. It was not
  refused (V1-1564); `TRE-V0-118` now refuses it as `SQL_QUERY_REJECTED`. pgTAP
  `timeout` found the `TRE-V0-076` residual server session (`runs/sql-pgtap/pre-0076/`).
- **appium-uiautomator2-wdio**: an Appium server on `127.0.0.1:4734`, owned by the
  qualification harness. `pass-fail-skip` reads the real `greeting` view (PASSED, FAILED,
  SKIPPED). `zero` is refused for missing session identity. `build-error` (a spec syntax error)
  is incomplete. `timeout` is incomplete, and no client process survived. Appium retired the
  orphaned session after `newCommandTimeout` (60 s), and the harness then stopped the server.

Requirements added by the live witnesses:

- `TRE-V0-075`: (proposed; V1-0594) For `swift-testing`, each selector MUST remain the
  exact module-qualified discovery name, without `/`. Its SwiftPM `--filter` MUST be
  anchored as `^<quoted selector>(/|$)`. This is because SwiftPM 6.4 matches the full Swift
  Testing ID, which ends in `/<File>.swift:<line>:<column>` for functions. A selector
  MUST NOT match a longer name or a different module.
- `TRE-V0-076`: (proposed; V1-0594) `sql-pgtap` MUST start psql with
  `PGOPTIONS=-c client_connection_check_interval=1000`. Retiring psql on timeout or
  interruption otherwise leaves its server backend running the statement, with its
  transaction open.
- `TRE-V0-077`: (proposed; V1-0594) A JUnit Vintage leaf whose unique ID ends in a
  `[runner:…]` segment, reported without an outcome, MUST NOT be a pass. It MUST be
  `CONTAINER_WITHOUT_TESTS`, and the observation MUST be incomplete. Example: a Robolectric
  `@Config(sdk=…)` outside `enabledSdks` filters out every child.

| Requirements | Source/tests | Evidence |
| --- | --- | --- |
| TRE-V0-075 | `platform/platform.go` swift-testing `Build`; `TestSwiftTestingSelectorMatchesSourceSuffixedID` | Live before/after: `runs/swift-testing/selector-anchor-bug-before-fix` (warning "No matching test cases were run", `NO_TESTS`), then `runs/swift-testing/pass-fail-skip` complete with three rows |
| TRE-V0-076 | `sql/sql.go` pgTAP `Build`; `TestFixedInvocations` | Before: `runs/sql-pgtap/pre-0076/timeout-residual-backend-before-fix/cleanup-check.txt` (backend still in `pg_sleep`). After: `runs/sql-pgtap/timeout/cleanup-check.txt` (0 rows) |
| TRE-V0-077 | `platform/xml.go` `parseJUnit`; `TestVintageRunnerLeafIsNotPass` | Before: `runs/robolectric/pre-0077/build-error-sdk-not-enabled` (complete, one PASSED `[runner:proof.BrokenTest]`). After: `runs/robolectric/build-error-sdk-not-enabled` (incomplete, `CONTAINER_WITHOUT_TESTS`) |

Open residuals, retained and not fixed in this slice:

- Android timeout: after a runner timeout, the instrumentation (`mFinished=false`) and the
  target app process stayed alive on the device. Killing the adb client does not end a
  device-side process, and the runner has no device cleanup phase. Only
  `am force-stop <target package>` ended it; `force-stop` of the test package did not. Every
  T case did that by hand (`runs/{android-junit,espresso,uiautomator}/timeout/cleanup-check.txt`).
  This is a device-cleanup blocker for those three profiles. Addressed by `TRE-V0-087`
  (V1-1553, section "Device cleanup after timeout or interrupt"); its live re-run is NOT_RUN.
- The installed-APK identity is checked by the qualification harness, not bound by the receipt.
- Failure kinds stay UNKNOWN for Android assertion errors, pgTAP, sqllogictest, WebdriverIO
  and Gradle exceptions.
- Kotest reports two errors for one failure, and the build-error case is refused rather than
  classified. Bats `timeout` reports a count mismatch instead of a plain timeout.
- `sql-sqllogictest-sqlite` recorded a query the engine rejects as a complete FAILED suite.
  The native per-record diagnostics do separate them (`query failed` on stderr against
  `wrong result - expected [...] got [...]` on stdout, captured fixtures `sqlite/infra` and
  `sqlite/fail`), so `TRE-V0-118` refuses the rejected query. A `statement ok` record the
  engine rejects prints `<file>:<line>: statement error` (sqllogictest.c), which `TRE-V0-119`
  refuses as `SQL_STATEMENT_UNRESOLVED`; a live capture of it is NOT_RUN.
- `xcode-xctest` `zero` and `build-error` were refused because destination reports disagree;
  `TRE-V0-117` maps both exact shapes.
- Gradle and Maven write `build/` and `target/` into the fixture tree. Java ignores `TMPDIR`
  on macOS.
- The Robolectric SDK jar the toolchain first supplied (12650502) is not the one Robolectric
  4.16.1 requests for API 35. The lane resolved 13954326 instead.
- Retry (R) is not applicable to any of these profiles.

Rollback reverts each fix and its test: the `(/|$)` anchor goes back to `$`, the `PGOPTIONS`
entry is removed, and the Vintage runner-leaf refusal is removed. This section is removed with
them. Swift Testing selectors would then select nothing again on SwiftPM 6.4. pgTAP timeouts
would again leave server sessions running, and a filtered Robolectric runner would again be a
pass.

## Command-line help and listing admission (experimental)

Issue beamfall/corvint#1327 (V1-2095): `corvint-test-runner --help` and an unknown verb such as
`profiles` reached the executable identity check and were refused with exit 1, so an operator
could not read usage without first pinning an executable. The pin protects execution, not help.

- `TRE-V0-078`: (accepted under the owner's 2026-10-11 standing lane auto-accept; V1-2095)
  `--help`, `-h`, `-help` or `help` as the command, and `--help`, `-h` or `-help` after any
  command, MUST print usage to stdout and exit 0 without `--executable` or
  `--executable-sha256`. Per-command help for `plan` and `run` MUST list that command's flags.
  Help MUST NOT read a request or plan, derive a plan or execute anything.
- `TRE-V0-079`: (accepted under the owner's 2026-10-11 standing lane auto-accept; V1-2095) The
  read-only listing verbs `runners` and `profiles` MUST need no executable pin. `profiles` is an
  alias of `runners` and emits the identical runner profile ID JSON. An unknown command MUST be
  refused as unknown, with usage on stderr and exit 2, before any identity check.
- `TRE-V0-080`: (accepted under the owner's 2026-10-11 standing lane auto-accept; V1-2095) The
  executing verbs `plan` and `run` MUST still refuse without both independent pins, exit 1, and
  the refusal MUST name each missing flag (`--executable`, `--executable-sha256`).

Failure modes: a help flag given as another flag's value (`--request --help`) is treated as a
help request, which only prints usage; it never weakens execution admission. Help is detected
before the repeated-flag check, so a repeated `--help` still prints usage.

Rollback: revert the command dispatch in `cmd/corvint-test-runner/main.go`; plan and receipt
documents, profiles and identities are unchanged.

| Requirements | Source/tests | Evidence |
| --- | --- | --- |
| TRE-V0-078 | `cmd/corvint-test-runner/main.go` `command`, `usage`; `TestHelpNeedsNoExecutablePin` | `go test ./cmd/corvint-test-runner`; built binary `--help` printed usage, exit 0 |
| TRE-V0-079 | `command` listing dispatch; `TestListingVerbsNeedNoExecutablePin` | built binary `profiles` printed the runner ID JSON, exit 0 |
| TRE-V0-080 | `missingIdentity`; `TestExecutingVerbsNameMissingPinFlags` | built binary `plan` refused naming both flags, exit 1 |

## Device cleanup after timeout or interrupt (experimental)

V1-1553: a timed-out or interrupted `android-junit`, `espresso`, `uiautomator` or
`appium-uiautomator2-wdio` run retired the host process group, but the device kept running.
Killing the adb client does not end a device-side process. The 2026-10-17 platform matrix
retained the instrumentation (`mFinished=false`) and the target app alive after every Android
timeout; only `am force-stop <target package>` ended them, and `force-stop` of the test package
did not (`runs/{android-junit,espresso,uiautomator}/timeout/cleanup-check.txt`). The Appium
timeout left its WebDriver session open until `newCommandTimeout`, and Appium 3.8.0 refuses
session enumeration without the `session_discovery` feature
(`runs/appium-uiautomator2-wdio/timeout/cleanup-check.txt`), so the runner must know the session
identity itself.

- `TRE-V0-087`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1553) A
  device-bound profile MUST declare fixed, bounded device-cleanup steps in its invocation, and
  the executor MUST run them, and only them, after the TEST phase timed out or was interrupted.
  (1) The Android instrumentation profiles MUST resolve the instrumentation's target package on
  the device from `pm list instrumentation`, force-stop it and then the test package, and then
  verify that the instrumentation is no longer active and the target process is gone. A probe
  that fails MUST fail the verify step, never count as an absence: `dumpsys` must succeed and
  print its `ACTIVITY MANAGER RUNNING PROCESSES` header, and `pidof` must answer status 1 with
  no output. Each step MUST print a fixed last line (`stopped`, `verified`), so an adb build that
  drops the remote exit status cannot pass a step silently. (2) The Appium profile's fixed
  configuration hooks MUST record the WebDriver session id, and the id of each replacement
  session `browser.reloadSession()` creates (the WDIO `onReload` hook), one newline-terminated
  id per line beneath the fresh report directory; cleanup MUST send `DELETE /session/<id>` for
  every recorded id (at most eight) to the plan's canonical loopback endpoint. The record MUST
  be a regular file, opened without blocking. Only a whole HTTP 200 body read before the step
  deadline that is the W3C envelope `{"value":null}` counts as deleted, and a 404 `invalid
  session id` counts as already absent. (3) Every step MUST run under its own bound (10 s, process
  group contained) even when the run's context was cancelled, and later steps MUST still run
  after an earlier failure. (4) The execution MUST record `deviceCleanup` with outcome
  `not-required`, `clean` or `incomplete` and each step's status (`ok`, `failed`, `timed-out`,
  `unobserved`) with bounded detail. A cleanup that is not clean MUST add the
  `device-cleanup` execution problem, so the observation is incomplete; it MUST NOT be
  reported as a clean stop. (5) Admission MUST refuse a cleanup step that names an undeclared
  tool, holds more than 32 arguments or a NUL, combines a tool with a WebDriver endpoint, names
  a non-canonical endpoint or a non-local session file, or exceeds four steps, before any phase
  runs. (6) `deviceCleanup` MUST be omitted from plan JSON when empty, so historical plan bytes
  and identities do not change; `corvint-test-runner run` MUST refuse a device-bound plan
  written before cleanup existed rather than run it without cleanup. An Android plan, which
  differs only by its steps, gets the re-plan diagnosis; an Appium plan also differs in its
  wrapper and gets the generic profile-mismatch refusal.

Failure modes. An instrumentation not listed by `pm` (uninstalled, or a mistyped component) is
reported `failed` with `instrumentation-target-unresolved`, never as stopped. A force-stop that
does not take effect fails the verify step. A failing or headerless `dumpsys` fails it with
`instrumentation-probe-failed`, and a `pidof` that is missing or errors with
`process-probe-failed`. A transport that swallows the remote exit status
fails on the missing fixed line. A session id that was never recorded (the run timed out before
the WebDriver session started) is `unobserved`, so cleanup is incomplete even when no session
existed. A record that is not a regular file (a FIFO never blocks the open), is not
newline-terminated, or holds more than eight ids is `unobserved`. A refused or unreachable
delete, or a 200 whose body is not the null-value envelope, is `failed`; a response body that
stalls past the step deadline is `timed-out`.

| Requirements | Source/tests | Evidence |
| --- | --- | --- |
| TRE-V0-087 | `testrunner/types.go` `CleanupStep`; `execution.go` `DeviceCleanup`; `device_cleanup_unix.go` `checkDeviceCleanup`, `cleanDevice`; `platform/android.go` `androidCleanup`; `mobile/mobile.go` `Build`; `cmd/corvint-test-runner/main.go` `admittedInvocation`; `TestDeviceCleanupAdmission`, `TestDeviceCleanupIsBoundedAndRecorded`, `TestDeviceCleanupOnlyAfterTimeoutOrInterrupt`, `TestWebDriverSessionDeletionProvesAbsence`, `TestHistoricalPlanByteIdentity`, `platform` `TestAndroidTimeoutStopsDeviceProcesses`, `mobile` `TestTimeoutDeletesAppiumSession`, `TestNativeConfigIsolation`, `cmd/corvint-test-runner` `TestPreCleanupDevicePlanRefusedForExecution` | A recording fake `adb` with a stateful fake device shell, and an `httptest` WebDriver endpoint; the installed WDIO 9.32 ConfigParser loaded the wrapper and Node 22.23.3 ran its `before` hook and the ConfigParser-normalized `onReload` hook, which recorded both session ids; focused unit tests, 2026-10-20 |

Recorded limits.
- Live qualification is NOT_RUN: no device, emulator or Appium server ran. The ticket's live
  witnesses for the four profiles remain outstanding.
- `pidof` checks only the target's main process; `:remote` process names are not verified.
- The `dumpsys` section header is taken from AOSP source, not from the 2026-10-17 captures. If
  a device omits it, every verify step fails closed as `instrumentation-probe-failed`.
- A run killed after Appium created a session but before the `before` or `onReload` hook
  recorded it leaves that session unrecorded; cleanup then deletes only the recorded ids. A
  session the test creates itself (a second `remote()` client) is not tracked.
- Whether `dumpsys activity processes` lists the active instrumentation on every API level is
  inferred from the 2026-10-17 API 35 evidence, not observed across versions.
- Cleanup runs only after timeout or interrupt. A run that completes but leaves a process is
  out of scope.

Rollback removes `DeviceCleanup` from the invocation and execution, `device_cleanup_unix.go`,
the Android steps, the Appium session hook and delete step, the runner's pre-cleanup plan
refusal, these tests and this section. Device-bound plans written with cleanup would then fail
closed decoding; historical plans and receipts are unaffected.

## Minitest unexpected-error cases (experimental)

V1-1541: the profile-owned Minitest reporter mapped a test that raises (`Minitest::UnexpectedError`)
to `UNKNOWN`, so the shared normalizer voided every case of the run, although Minitest attributes
the error to one named test and counts it as an error, never as a pass.

Decision: FAILED-with-cause rather than UNKNOWN. The native runner names the case and never counts
it as passed, so FAILED invents no certainty; UNKNOWN discarded the other cases' outcomes. The
Python `unittest` and Ruby `test-unit` profiles now follow the same rule (`TRE-V0-115`, V1-2156).

Rollback: restore the `UNKNOWN` state in `dynamic/reporters/minitest_reporter.rb`; plans, receipts
and parsers are unchanged in shape.

| Requirements | Source/tests | Evidence |
| --- | --- | --- |
| TRE-V0-110 | `dynamic/reporters/minitest_reporter.rb` `record`; `TestMinitestUnexpectedErrorFailsOnlyThatCaseLive` | Live Minitest 5.11.3 / Ruby 2.6.10: one raising and one passing test give native exit 1, a complete run, PASSED and FAILED/UNKNOWN; the test fails with the previous reporter |

## CTest run-owned test directory (experimental)

V1-1537: CTest always writes `Testing/Temporary` (`LastTest.log`, `LastTestsFailed.log`,
`CTestCostData.txt`) into the directory it runs from, so the `ctest` profile left residue in the
caller's build tree.

Decision: the log is retained as report-directory evidence but is not a parsed report, so a large
test log cannot exceed the report byte bound and turn a complete run incomplete; the JUnit report
stays the only parsed input.

Rollback: restore the earlier `ctest` argv without `--test-dir` and drop the generated file.

| Requirements | Source/tests | Evidence |
| --- | --- | --- |
| TRE-V0-111 | `native/native.go` `Build` (`ctest`); `TestCTestRunsFromRunOwnedTestDirectory`, `TestCTestRunLeavesBuildDirectoryUnchanged` | Live CTest 4.4.4 (macOS arm64): one passing and one failing test that write nothing give a complete run, the build tree digest set is unchanged (no `Testing/` entry) and `LastTest.log` sits under the report directory; the live test fails with the previous argv (the build tree gained `Testing/Temporary/*`) |
| TRE-V0-113 | `native/native.go` `ctestRunFiles`; `TestCTestRunsFromRunOwnedTestDirectory`, `TestCTestCarriesDartConfiguration`, `TestCTestWindowsRootUsesForwardSlashes` (Windows only), `TestCTestRunKeepsBuildTreeConfiguration` | Live CTest 4.4.4 (macOS arm64): a `CTestCustom.cmake` pre-test hook that exits 3 makes the run incomplete, and a `DartConfiguration.tcl` `TimeOut: 1` fails a 5 s test; both subtests fail against the wrapper without the carried files (the hook was skipped and the run passed). Copying `DartConfiguration.tcl` unchanged sent `Testing/` back into the build tree, which is why `BuildDirectory` is relocated. The Windows test is type-checked by `make windows-vet` and NOT_RUN on Windows |

## swift-testing reserved package identity (experimental)

V1-1535: SwiftPM derives a package's identity from its directory name. A fixture package in a
directory named `swift-testing` is compiled with `-package-name swift_testing`, which collides with
the toolchain swift-testing package; Swift 6.4 then fails the build with `error: fatalError` and the
run reports a build failure that is the fixture's fault, not the code under test.

Decision: reserve only the identity a live run showed to fail; a directory named `Testing` built and
passed on the same toolchain, so it is not reserved. A newly observed collision is added to the set
with its own live witness.

Rollback: remove the reserved-identity check; the profile's argv is unchanged.

| Requirements | Source/tests | Evidence |
| --- | --- | --- |
| TRE-V0-112 | `platform/platform.go` `Build` (`swift-testing`), `reservedSwiftPackageIdentity`; `TestSwiftTestingRefusesReservedPackageIdentity`; `platform/README.md` | Live Swift 6.4 (Xcode toolchain, macOS arm64) on 2026-10-20: identical one-library, one-test packages in directories `okpkg` and `Testing` pass; in `swift-testing` the build fails with `error: fatalError` (`-package-name swift_testing`). Live symlink probe: an alias `realbad-alias -> swift-testing` fails (`module 'Testing' is in package 'swift_testing'`), and `l/swift-testing -> ../okpkg` builds and passes |

## TestCafe browser helper failure (experimental)

V1-0605: TestCafe 3.7.6 on macOS copies its bundled `TestCafe Browser Tools.app` into
`$HOME/.testcafe-browser-tools` on first use. With a fresh isolated HOME and a remote Chrome, the
2026-10-01 qualification run printed `EEXIST` from `mkdir .home/.testcafe-browser-tools/TestCafe
Browser Tools.app/Contents` and aborted before useful results, twice. A concurrent upstream
helper copy is the suspected cause; it is not proven. A task-owned synchronous copy of the bundled
helper into the isolated HOME before the run let the later mixed qualification pass, but that is a
setup workaround, not a framework fix.

`TRE-V0-114` (in Requirements) governs this check.

Failure modes. An errno line that does not name the helper directory (a missing test file, say)
and a helper path without a system error do not match. A helper failure TestCafe reports only
inside its JSON test errors, or not at all, is not detected by this check; the native report's
own failed tests still apply.

Recorded limits.
- Reproduction is NOT_RUN on this lane: the local TestCafe 3.7.6 tree under
  `/private/tmp/cem10-build/toolchains-2026-10-17/testcafe` is incomplete (no `package.json` or
  library JavaScript in `testcafe-browser-tools`), and a reinstall needs owner approval. Whether
  the EEXIST is a helper-copy race stays a suspicion. Fresh-HOME repeated live runs with
  descendant cleanup remain outstanding.

Rollback: remove `dynamic/testcafe.go` and call `reconcileMochaSelection` from `Parse` again; the TestCafe invocation is
unchanged.

| Requirements | Source/tests | Evidence |
| --- | --- | --- |
| TRE-V0-114 | `dynamic/parse.go` `Parse`; `dynamic/testcafe.go` `reconcileNativeOutput`, `testCafeHelperFailure`; `TestTestCafeBrowserHelperFailureIsNeverAPass` | Focused unit test with the native EEXIST line from the V1-0605 qualification console; before the fix a passing report with that stderr was a complete PASSED observation, 2026-10-21 |
| TRE-V0-115 | `dynamic/reporters/unittest_reporter.py` `addError`, `addSubTest`; `dynamic/reporters/testunit_reporter.rb` `test_finished`; `TestUnittestAndTestUnitErrorFailsOnlyThatCaseLive` | Live Python 3.9.6 unittest and Ruby 2.6.10 bundled test-unit: one raising and one passing test give native exit 1, a complete run, PASSED and FAILED/UNKNOWN `Error: <Class>`; the test fails with the previous reporters, 2026-10-21 |

## Private TMPDIR removal, Xcode destination-less summaries and SQLite rejected queries (experimental)

V1-1538: on timeout, a .NET MTP host left `clr-debug-pipe-*` FIFOs in `<reportDir>/.tmp`. The
2026-10-17 matrix runs also left `go-build*`, `rustdoctest*` and Xcode `TemporaryDirectory.*`
directories there. `TRE-V0-116` removes the whole private `TMPDIR` after retirement, device
cleanup and report reads. Retirement itself is unchanged.

V1-2182: the 2026-10-17 `xcode-xctest` `zero` and `build-error` witnesses (Xcode 27.0 27A266a,
xcresulttool 25115, schema 0.4.0, macOS 26.6.2) were refused because their destination-less
summaries looked like a destination disagreement. `TRE-V0-117` maps the two exact native shapes.
A build failure has no test row, so it reuses the native Go `BUILD_FAILURE` problem code, not a
`BUILD` failure kind.

V1-1567: `TRE-V0-118` refuses a sqllogictest query the engine rejects. The captured
`db57eba9`/SQLite 3.53.0 streams show the native signal: `case.slt:1: query failed` on stderr for
a query on a missing relation, and `case.slt:4: wrong result - expected [2] got [1]` on stdout
for a mismatch.

Failure modes. If removing `.tmp` fails (a file the user cannot delete, say), the run is
incomplete with `temp-cleanup`; a residue is never hidden. An Xcode destination-less summary
with any count, failure, destination or other tree shape stays `INVALID_NATIVE_REPORT`. A
sqllogictest rejected `statement` record was not detected until `TRE-V0-119` (V1-2341); other
runtime errors are not detected.

Recorded limits.
- The `xcode-xctest` mappings are proved against the retained native bytes (parser fixtures).
  A fresh live re-run is NOT_RUN.
- The sqllogictest runner and source under `toolchains-2026-10-17/sqllogictest` were purged, so
  `TRE-V0-118` is proved against the committed captured native streams. A fresh live run, and
  the native text for a rejected `statement ok` record, are NOT_RUN.
- Process-level proof of `TRE-V0-116` uses the executor helper: a FIFO plus nested scratch,
  left by both a timed-out and a normally exiting phase. A live MTP re-run is NOT_RUN.

Rollback: remove the `.tmp` removal defer in `execute_unix.go`, the destination-less switch in
`platform/xcode.go` and the `query failed` check in `sql/sqllogictest.go`, together with their
tests and fixtures. Residue would then stay in report directories, those Xcode runs would be
refused again, and rejected queries would again be complete FAILED suites.
For `TRE-V0-119`, remove the `statement error` check and its test; such records would again be
complete FAILED suites.

| Requirements | Source/tests | Evidence |
| --- | --- | --- |
| TRE-V0-116 | `execute_unix.go` `Execute`, `boundedDetail`; `TestExecuteRemovesPrivateTempResidue` | Timeout and normal-exit helper phases leave a FIFO and nested scratch in `TMPDIR`; both subtests fail without the removal and pass with it, 2026-10-21 |
| TRE-V0-117 | `platform/xcode.go` `parseXcode`; `TestXcodeBuildFailureAndZeroTests`; fixtures `xcode-build-error-*`, `xcode-zero-*` | Retained 2026-10-17 native bytes give `BUILD_FAILURE` and `NO_TESTS`; four shape mutations stay `INVALID_NATIVE_REPORT`; the test fails without the mapping, 2026-10-21 |
| TRE-V0-118 | `sql/sqllogictest.go` `parseSQLite`; `TestSQLiteRejectedQueryIsNotAnAssertion`, `TestNativeMatrix` | Captured native `sqlite/infra` is incomplete with `SQL_QUERY_REJECTED`; `sqlite/fail` stays a complete FAILED suite; the test fails without the check, 2026-10-21 |
| TRE-V0-119 | `sql/sqllogictest.go` `parseSQLite`; `TestSQLiteStatementErrorIsNotAVerdict` | The `statement error` line shape is read from sqllogictest.c (`fprintf(stderr, "%s:%d: statement error\n", ...)` for both statement directions); the test fails without the check, 2026-10-22. A live capture is NOT_RUN |
| TRE-V0-120 | `validate.go` `undeclaredTools`; `TestNormalizeUndeclaredToolIsNamedNotAFailure` | A go test shelling out to git with no PATH is an `undeclared-tool` problem naming git, not a FAILED verdict; passing and unrelated failures unchanged. |

## Android installed-APK identity (experimental)

V1-1554: a platform lane request bound the built test and app APK digests, but only the lane
harness compared them with the APKs installed on the emulator; the runner ran the
instrumentation against whatever was installed.

Requirement: `TRE-V0-820` (in Requirements above).

Non-goals. The runner still never installs, clears or grants: installation stays a caller
admission obligation. It does not parse the APK manifest; it compares digests as sets, so it
does not say which bound APK is which package. Split APKs and dynamic feature modules are not
supported and fail closed as `unobservable`.

Failure modes. An uninstalled or mistyped component is `unresolved`. A missing package, a
failing `pm path`, more than one `package:` path or a failing `sha256sum` is `unobservable`. A
reinstall from another build, or a bound APK that is not installed, is `mismatch`. A reinstall
after the first hash that is still in place when the instrumentation ends is `changed`. Stdout from
before this requirement (no verified line) is `INSTALLED_APK_UNVERIFIED`, so a historical raw
capture is not a complete observation. An Android plan written before this requirement differs
in its TEST argv, so `corvint-test-runner run` refuses it and asks for a re-plan.

| Requirements | Source/tests | Evidence |
| --- | --- | --- |
| TRE-V0-820 | `platform/android.go` `buildAndroid`, `androidIdentityScript`, `parseAndroid`; `TestAndroidInstalledAPKIdentity`, `TestAndroidRequiresBoundAPKs`, `TestNativeAndroidOutcomesAndNeutralTransportExit`, `TestAndroidTimeoutStopsDeviceProcesses` | A recording fake `adb` with a stateful fake device shell (`pm path`, `sha256sum`) runs the real script: a matching install is verified and instrumented, a mismatching, split or missing install is never instrumented and is incomplete, including when the transport drops the exit status; an install replaced during the instrumentation is `changed` and incomplete (`reinstalled-during-run`); focused unit tests, 2026-10-31 |

Recorded limits.
- The live witness is NOT_RUN: no emulator or adb was available to the lane host. Whether every
  supported emulator image ships toybox `sha256sum` is inferred from AOSP toybox, not observed.
- The check and the instrumentation run in one shell session and the APKs are hashed before and
  after it. A reinstall that is reverted to the bound bytes before the second hash (A to B to A),
  and which process image the device actually loaded, are not observed: two hashes bound the
  window, they do not close it.

Rollback removes the `.apk` input requirement, the identity script and the parser's first- and
last-line checks, with their tests and this section. Android TEST argv returns to the bare `am instrument`
command, so plans written with the script would need a re-plan.
