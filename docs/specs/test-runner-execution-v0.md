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
- Exists: 57 concrete experimental profiles in `internal/testrunner`; implementation and live qualification are tracked separately.
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

## Runner inventory

The dynamic lane covers the fifteen JS/TS affected registrations: Vitest, Jest, AVA, Mocha, node:test,
Playwright, Bun, Deno, Cypress, WebdriverIO, TestCafe, Nightwatch, Detox and both Storybook runners;
Python pytest/unittest; Ruby RSpec/Minitest/Test::Unit/Rails. The native lane covers Go, Cargo/nextest
and doctests, .NET xUnit/NUnit/MSTest with separate VSTest/MTP profiles, CTest/GoogleTest/Catch2, CMocka and Ginkgo v2.
The platform lane covers Java JUnit/TestNG/Gradle/Maven, Kotlin kotlin-test/Kotest/Android families,
Swift Testing/XCTest/SwiftPM/Xcode, Bats/ShellSpec and remaining domain runner inventory. Analyzer
candidates never imply that SQL/shader/data compilation is assertion testing. Additional main-runner
ambiguity needs explicit owner disposition; absence of evidence is not an approved exclusion. Decision 0437 closes the CEM 1.0 main-framework set.

The concrete count is 21 dynamic, 17 native, 16 platform, two SQL and one Appium Android profiles. The registry also
lists six unavailable IDs: `appium`, `pgtap`, `sqllogictest`, `shader-behavior`, `html-behavior` and
`structured-data-behavior`. The two SQL profiles use the explicit IDs `sql-pgtap` and
`sql-sqllogictest-sqlite`; the older unqualified names do not silently alias them. SQL coverage does
not resolve arbitrary Appium backends or behavioral qualification for shaders, HTML and structured data. The latter
need an actual domain/application assertion harness, not syntax validation presented as tests.

Cypress 16.1.1 and Rails 8.1.4 now have shared-executor runtime witnesses. MTP has separate
xUnit/NUnit/MSTest profiles and actual pass/fail/skip/zero/setup receipts. SwiftPM 6.4 macOS serial
XCTest and Robolectric 4.16.1/API35 also have shared-executor witnesses. These do not erase remaining
limits: TestCafe's current common-executor remote-Chrome CLI witness is qualified in
`/private/tmp/cem10-build/dynamic/testcafe-cli-proof/qualification.json`; its prior permission
refusal and first-use helper failure remain retained. Detox lacks a qualified task application/device;
legacy Storybook collection is blocked by the observed runtime incompatibility. Playwright now retains actual pinned Chrome DOM pass/fail/skip/retry/zero/collection cases;
a timeout initially left its native detached browser group alive (V1-0608), with durable cleanup
repair independently reviewed and locally qualified for native cooperating Playwright. The prior no-browser fixture remains retained. Nightwatch's current fixture
used `start_session:false`, so its browser execution remains NOT_OBSERVED. Other OS versions, retries, parameterized cases and lifecycle negatives remain open
where their adapter provenance says so.

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
| TRE-V0-006..007 | Adapter native-format and contradiction tests; `validate_test.go` `TestNormalizeBoundaryAndExitProfiles`, `TestAggregateInventoryDoesNotInventExecutedCases` | Dynamic/platform reviews and `native-trx-final-review.json` PASS; native retry/skip/count/error evidence retained |
| TRE-V0-008 | `TestExecuteRetiresDescendants`, `TestExecuteEnvironmentAndArtifactBounds` | Shared executor review PASS for qualified ordinary process groups; hostile detached sessions and other OS containment unqualified |
| TRE-V0-009 | Adapter READMEs, `testdata` provenance, opt-in `TestMTPLiveExecution` and `TestSQLLiveExecution` | PARTIAL live matrices; current JVM/SwiftPM/Robolectric requalification in `platform-latest-execution-proof.jsonl` and `jvm-latest-execution-proof.jsonl` |
| TRE-V0-010 | Separate criterion experiment, Tasks native boundary and CEM candidate references | Cross-language native claim/source/runner join and semantic discrimination remain NOT_PRODUCED |
| TRE-V0-011..012 | `sql/sql.go`, `pgtap.go`, `sqllogictest.go`; `TestNativeMatrix`, `TestTAPRefusals`, `TestSQLiteRefusals`, `TestFixedInvocations`, `TestClosedConnection` | `sql/review.json` PASS; ten actual shared-executor receipts, native format provenance and disposable cluster cleanup retained |
| TRE-V0-015 | `execute_unix.go`; `TestExecuteExplicitPrimaryTestWithoutArguments` (pinned `pwd` with no argv; implicit, ambiguous, BUILD, DISCOVER, DECODE, auxiliary and implicit-primary-name refusals) | `cmocka-independent-review-r2/REVIEW.json` PASS_BOUNDED (F4); root decision `e4de2129…e161` ACCEPTED_WITH_CONDITIONS |
| TRE-V0-016..017 | `native/cmocka.go`; `TestCMockaActualDualFormatWitnesses`, `TestCMockaBoundaryContradictions`, `TestCMockaClosedBuild`, `TestCMockaRegistryDispatchAndTargetBoundary`, `TestCMockaPlanBindsTargetAndFixedEnvironment`, opt-in `TestCMockaNativeReceiptReadback` | Nine actual macOS arm64 CMocka 2.0.2 receipts in `cmocka-profile-proposal-r1/proof`; review PASS_BOUNDED; fresh requalification on the integration base NOT_RUN |
| TRE-V0-018..020 | `native/ginkgo.go`; `TestGinkgoNativeProjectionStates`, `TestGinkgoReportCountAndBound`, `TestGinkgoParserRefusals`, `TestGinkgoStrictJSON`, `TestGinkgoBeforeSuiteSkipShape`, `TestGinkgoOrderedFollowOnFailure`, `TestGinkgoClosedBuild`, `TestGinkgoFocusBound`, `TestGinkgoRegistryDispatchAndTargetBoundary`, `TestGinkgoPlanBindsTargetAndFixedArgv` | Synthetic source-derived reports only; Ginkgo v2.33.0 read pins in `native/testdata/ginkgo-provenance.json`; actual runtime qualification NOT_RUN |

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
| `ambiguous-test-identity` | An observed native test identity is empty or repeated. | `internal/testrunner/validate.go:69@789f20a9` |
| `attempt-bound` | An observed test has more than MaxAttempts retained attempts. | `internal/testrunner/validate.go:82@6aa55fa4` |
| `attempt-outcome-conflict` | Playwright result statuses contradict its expected/unexpected/flaky/skipped category, or retained attempts contradict the final state (including a flaky result without at least two attempts ending in pass). | `internal/testrunner/dynamic/native.go:394@21e82322`; `internal/testrunner/dynamic/native.go:398@21e82322`; `internal/testrunner/dynamic/native.go:402@21e82322`; `internal/testrunner/dynamic/native.go:406@21e82322`; `internal/testrunner/dynamic/validation.go:158@24bd514f` |
| `attempt-sequence` | A Playwright result retry number differs from its zero-based result position. | `internal/testrunner/dynamic/native.go:364@93e0a03b` |
| `case-error-conflict` | WDIO reports a native error for a nonfailed case, or Node reports a non-TODO pass event with an error. | `internal/testrunner/dynamic/native.go:166@7456805d`; `internal/testrunner/dynamic/browser.go:139@89d45ae9` |
| `collection-or-hook-error` | A Jest/Vitest file reports failed status without a failed assertion row in that file. | `internal/testrunner/dynamic/parse.go:261@11da83fa` |
| `collection-or-plan-error` | The AVA TAP plan count differs from the observed sequential outcome count. | `internal/testrunner/dynamic/native.go:298@e3eaf1ea` |
| `conflicting-outcomes` | A TestCafe skipped case also has errors, or an XML skipped case also has failure/error elements. | `internal/testrunner/dynamic/native.go:75@4df23ffc`; `internal/testrunner/dynamic/browser.go:51@02fd57db` |
| `contradictory-exit` | A shared-boundary success exit accompanies an observed FAILED test. | `internal/testrunner/validate.go:122@8df4cef9` |
| `count-mismatch` | A parsed native report total or category count disagrees with its observed rows, outcomes or attempts; the compared denominator is format-specific; or Node reports a negative skipped/todo count. | `internal/testrunner/dynamic/native.go:99@9a0a1c7f`; `internal/testrunner/dynamic/native.go:103@74f3bece`; `internal/testrunner/dynamic/native.go:200@b6957ba2`; `internal/testrunner/dynamic/native.go:226@ad1ded29`; `internal/testrunner/dynamic/native.go:424@0546fe58`; `internal/testrunner/dynamic/browser.go:57@312cf3e9`; `internal/testrunner/dynamic/browser.go:102@de598de5`; `internal/testrunner/dynamic/browser.go:159@6329112d`; `internal/testrunner/dynamic/browser.go:255@d7afa062`; `internal/testrunner/dynamic/parse.go:272@6144e481`; `internal/testrunner/dynamic/parse.go:279@9b100a65`; `internal/testrunner/dynamic/parse.go:332@c89d4e2d`; `internal/testrunner/dynamic/parse.go:352@5e2333a0`; `internal/testrunner/dynamic/parse.go:376@a3fc5178` |
| `exception-conflict` | An RSpec example maps to PASSED while retaining an exception object. | `internal/testrunner/dynamic/parse.go:347@3ad339db` |
| `exit-report-conflict` | A dynamic runner exits zero while a parsed test is FAILED, TIMED_OUT or INTERRUPTED. | `internal/testrunner/dynamic/parse.go:148@a9d4d7aa` |
| `hook-error` | A WDIO hook has a native error or literal failed state. | `internal/testrunner/dynamic/browser.go:153@9b21db50` |
| `hook-or-unmatched-result` | The Cypress/Mocha result-status identity count differs from the collected test-row count. | `internal/testrunner/dynamic/browser.go:105@7ff4ce18` |
| `invalid-exit-profile` | Exit-code lists exceed their bound, contain values outside 0..255, or repeat a value within or across lists. | `internal/testrunner/validate.go:101@0b03b013` |
| `invalid-prior-attempt` | A retained nonfinal attempt is neither FAILED nor TIMED_OUT. | `internal/testrunner/dynamic/validation.go:162@71956e90` |
| `missing-attempt` | A non-skipped Playwright test has no results, or a test passed to retained-attempt validation has no attempts. | `internal/testrunner/dynamic/native.go:411@92208046`; `internal/testrunner/dynamic/validation.go:153@a1b5f038` |
| `missing-count` | An XML suite containing direct testcase rows omits its tests count. | `internal/testrunner/dynamic/native.go:57@e722f2b9` |
| `missing-name` | An XML testcase has an empty name. | `internal/testrunner/dynamic/native.go:72@e3950df9` |
| `missing-native-id` | A Playwright spec has an empty native ID. | `internal/testrunner/dynamic/native.go:347@dceb991e` |
| `missing-report` | A non-AVA dynamic runner supplies no report files. | `internal/testrunner/dynamic/parse.go:60@9dbf26dd` |
| `missing-selected-test` | An expected native test identity is absent from observed identities. | `internal/testrunner/validate.go:97@034a7064` |
| `missing-summary` | The Node event stream ends without a final summary having an empty file field. | `internal/testrunner/dynamic/native.go:223@3d3dab29` |
| `negative-count` | A Nightwatch completed test has a negative retries, failed or errors count. | `internal/testrunner/dynamic/browser.go:217@0993c5a5` |
| `nightwatch-hook-error` | A Nightwatch completed section without a matching completed test reports errors, failures or literal fail status. | `internal/testrunner/dynamic/browser.go:197@7dccaec6` |
| `nightwatch-runtime-error` | Nightwatch reports system error text, a nonzero error count or error messages; or lastError is not a decodable NightwatchAssertError supported by a retained failed attempt. | `internal/testrunner/dynamic/browser.go:192@558311a9`; `internal/testrunner/dynamic/browser.go:251@79465e5a` |
| `nightwatch-test-error` | A Nightwatch completed test has a positive errors count. | `internal/testrunner/dynamic/browser.go:237@342f8ce7` |
| `no-executed-inventory` | A SUITE_ONLY observation has both counts present and their sum is zero. | `internal/testrunner/validate.go:66@710cfe9e` |
| `node-run-unsuccessful` | The final Node summary reports success=false without an observed failed, timed-out or interrupted test. | `internal/testrunner/dynamic/native.go:211@5e6d82cd` |
| `node-runtime-error` | A Node test error has a failureType other than testCodeFailure, testTimeoutFailure or cancelledByParent. | `internal/testrunner/dynamic/native.go:176@40fd52ec` |
| `node-suite-error` | A Node suite fail/error is not explained by subtestsFailed plus an already observed failed, timed-out or interrupted test. | `internal/testrunner/dynamic/native.go:149@7e3f3a32` |
| `outside-example-errors` | RSpec reports a nonzero errors_outside_of_examples_count. | `internal/testrunner/dynamic/parse.go:335@503ba44e` |
| `playwright-global-error` | The Playwright report contains a top-level error entry. | `internal/testrunner/dynamic/native.go:334@7d964fc3` |
| `retry-history-missing` | Nightwatch declares positive retries without the same number of retained prior attempts; or Jest/Vitest indicates multiple invocations or passing-with-failure-messages without reconstructable attempt history. | `internal/testrunner/dynamic/browser.go:224@f37527eb`; `internal/testrunner/dynamic/parse.go:251@7bb3fc94`; `internal/testrunner/dynamic/parse.go:255@7bb3fc94` |
| `runner-exit` | The shared boundary receives a negative exit code or one outside all admitted success/failure/outcome-neutral lists. | `internal/testrunner/validate.go:119@a90e3e46` |
| `runner-unsuccessful` | Jest/Vitest reports success=false with assertion rows but no observed FAILED test. | `internal/testrunner/dynamic/parse.go:293@4b70a549` |
| `runtime-error` | Jest/Vitest reports a positive runtime-error suite count. | `internal/testrunner/dynamic/parse.go:285@be9d02c7` |
| `section-test-conflict` | Nightwatch completed-section and completed-test records map to different states for the same name. | `internal/testrunner/dynamic/browser.go:211@7936fd1f` |
| `setup-or-runtime-error` | An XML testcase contains one or more error elements. | `internal/testrunner/dynamic/native.go:69@b1ab2b72` |
| `success-conflict` | Jest/Vitest declares success with a failed assertion, or Node declares success with failed/cancelled counts or existing observation problems. | `internal/testrunner/dynamic/native.go:203@f9a16178`; `internal/testrunner/dynamic/parse.go:276@04a1c2a9` |
| `tap-bailout` | The AVA TAP stream contains a line beginning Bail out!. | `internal/testrunner/dynamic/native.go:250@b75d2c79` |
| `test-bound` | Observed inventory exceeds MaxTests at the shared boundary. | `internal/testrunner/validate.go:41@e88d43a4` |
| `unclassified-exit` | An admitted shared-boundary failure exit has no observed FAILED test. | `internal/testrunner/validate.go:125@94987f7f` |
| `unexplained-exit` | A dynamic runner exits nonzero without failed/timed-out/interrupted tests or any previously recorded observation problem. | `internal/testrunner/dynamic/parse.go:145@67d7a680` |
| `unknown-attempt-state` | A Playwright attempt status maps to UNKNOWN, or the shared boundary sees an attempt state outside its admitted state enumeration. | `internal/testrunner/validate.go:86@94ec202d`; `internal/testrunner/dynamic/native.go:376@4a8c35e3` |
| `unknown-expected-status` | A Playwright test with results has an expectedStatus that maps to UNKNOWN. | `internal/testrunner/dynamic/native.go:383@07ce9705` |
| `unknown-granularity` | A shared-boundary granularity is neither empty, CASE nor SUITE_ONLY. | `internal/testrunner/validate.go:47@50f6c9dd` |
| `unknown-node-event-kind` | A Node pass/fail event is neither a suite nor a test in its details.type. | `internal/testrunner/dynamic/native.go:155@9933433d` |
| `unknown-retry-information` | RetryInformation is not RETAINED, NOT_REPORTED or NOT_APPLICABLE. | `internal/testrunner/validate.go:128@b82d5be8` |
| `unknown-state` | A dynamic parsed test has state UNKNOWN. | `internal/testrunner/dynamic/parse.go:135@53bac795` |
| `unknown-suite-state` | A Jest/Vitest file status maps to UNKNOWN. | `internal/testrunner/dynamic/parse.go:264@045a51e8` |
| `unknown-test-state` | A shared-boundary test state is outside its admitted state enumeration. | `internal/testrunner/validate.go:73@8c1af2e1` |
| `unresolved-test-state` | A shared-boundary test state is UNKNOWN, INTERRUPTED or TIMED_OUT. | `internal/testrunner/validate.go:79@80befa5b` |

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

Acceptance evidence is unit-level only: `TestGinkgoNativeProjectionStates`,
`TestGinkgoReportCountAndBound`, `TestGinkgoParserRefusals`,
`TestGinkgoStrictJSON`, `TestGinkgoBeforeSuiteSkipShape` (source-derived model of
the BeforeSuite skip case), `TestGinkgoOrderedFollowOnFailure`,
`TestGinkgoClosedBuild`, `TestGinkgoFocusBound`,
`TestGinkgoRegistryDispatchAndTargetBoundary` and
`TestGinkgoPlanBindsTargetAndFixedArgv`. Their reports are synthetic, shaped from
the pinned source reading. Actual runner-generated qualification (`TRE-V0-009`)
is NOT_RUN: no Ginkgo binary or `github.com/onsi/ginkgo` module was present
locally, so every live pass/fail/skip/zero/lifecycle case remains a release
blocker for this tuple.

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

| Requirements | Source/tests | Evidence |
| --- | --- | --- |
| TRE-V0-021 | `selection.go`, `types.go`, `document.go`; `TestSelectionIsAdditiveToHistoricalBytes`, `TestHistoricalPlanAndReceiptBytesSurviveSelectionContract` | Frozen plan and receipt bytes generated at `0c94c66c` decode and re-encode byte-identically with an unchanged plan digest |
| TRE-V0-022 | `AdmitSelection`, `registry.Build`; `TestSelectionAdmissionIsClosed`, `TestNightwatchSelectionPreAdmittedAcrossFreshSessions` | Invalid selections refuse at `plan` and at `run` before the report directory exists |
| TRE-V0-023 | `selectionProblems`, `Normalize`; `TestSelectionMatchesFreshSessionsAndKeepsNativeIdentity`, `TestSelectionRefusesInexactMatches`, `TestNightwatchSelectionAcrossFreshSessions`, `TestNightwatchSelectionPreAdmittedAcrossFreshSessions` | Synthetic session IDs only: the runner-generated Nightwatch fixture replayed under two session IDs, and a pinned stand-in executable that picks its session at launch |

Recorded limits. A real pinned Nightwatch 3.16.0 browser run with fresh
WebDriver sessions is NOT_RUN: no Nightwatch package or ChromeDriver was
installed locally, and the earlier raw qualification file was no longer present.
The matcher trusts the parser's structured fields; it does not prove that the
session ID came from a live WebDriver server. Other session-bearing runners,
including WebdriverIO, need their own matcher version.

Rollback removes `expectedSelection`, `selection.go` and its admission call.
Plans without the field are unaffected, and plans that carry it then refuse as
unknown fields.

## Nonregular runner document admission (experimental)

A runner request, plan or tools document that named a FIFO without a writer
blocked `os.Open` before any validation, and the signal context could not
interrupt it (V1-0624). Admission now refuses such a path before a blocking open.

- `TRE-V0-024`: The companion MUST refuse a request, plan or tools document that
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
| TRE-V0-024 | `cmd/corvint-test-runner` `read`; `execute_unix.go` `checkTool`, `checkPinnedFile`, `openPinnedRegular`; `TestNonregularRunnerDocumentsRefusedBeforeBlockingOpen`, `TestPinnedGradleManifestFIFORefusedBeforeExecution`, `TestOpenPinnedRegularRefusesFIFOAndFinalSymlink` | Process-level child runs with a 20-second deadline; the base companion blocked on a FIFO request and tools document and stayed blocked after SIGINT (`docs/build-log/2026-10-08-runner-fifo-admission.md`) |

Recorded limits. The swap between the pinned-file check and its open is tested
through the open helper, not by racing a live swap. Parent
directories of a document path are trusted, as before. This is not hostile
filesystem authority.

Rollback restores the blocking opens; no wire, plan or receipt bytes change.
