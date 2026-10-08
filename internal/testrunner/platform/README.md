# Platform runner profiles (experimental)

Build constructs fixed arguments; Parse decodes original native observations.
Neither admits an executable, proves dependency closure, maps assertions to an
accepted criterion, or authorizes application/device changes. The caller uses the
shared executor with independently pinned tools, configuration and source inputs.

Sixteen concrete profiles exist:

| Runner | Supported native profile | Qualification witness |
| --- | --- | --- |
| junit-platform | Console Standalone 6.1.3, Jupiter engine | Java pass/assertion/exception/skip |
| junit4 | Console Standalone 6.1.3, Vintage engine | Java assertion failure; remaining matrix open |
| testng | 7.12.0 native XMLReporter, pinned suite XML | Java pass/assertion/exception/skip |
| kotlin-test | Kotlin 2.4.0 JVM, kotlin-test-junit5, Console 6.1.3 | Actual compiled Kotlin four-case matrix; shared executor witness |
| kotest | 6.2.5 JVM engine, Console 6.1.3 native XML | Actual compiled StringSpec four-case matrix; shared executor witness |
| gradle-junit | Gradle 9.6.1 native Test task XML | Actual Java/Jupiter four-case matrix and fixed init script |
| maven-surefire | Maven 3.9.16 / Surefire 3.6.0 XML 3.0.2 | Actual Java/Jupiter four-case matrix, rerun offline |
| swift-xctest | SwiftPM 6.4 macOS serial native XCTest text | Pass/two assertions/exception/skip; setup and build failures; shared executor witness |
| robolectric | Robolectric 4.16.1, API 35, Console 6.1.3 Vintage | Actual Android runtime four-case matrix; initialization failure and zero tests; shared executor witness |
| swift-testing | Swift 6.4, native version-0 event stream | Pass/expectation failure/disabled |
| xcode-xctest | Xcode 27.0 macOS XCTest, xcresulttool schema 0.4.0 | Swift and Objective-C pass/assertion/skip |
| bats | Bats Core 1.14.0 native TAP | Pass/failure/skip; failure kind remains unknown |
| shellspec | ShellSpec 0.28.1 native JUnit formatter | Pass/expectation failure/skip |
| android-junit | AndroidJUnitRunner 1.7.0, raw instrumentation stream | Android emulator pass/assertion/exception/skip and zero tests; shared executor witness |
| espresso | Espresso 3.5.1 using AndroidJUnitRunner | TV emulator pass/native view assertion/skip; separate phone SystemUI ANR witness |
| uiautomator | UIAutomator 2.3.0 using AndroidJUnitRunner | Phone emulator native UI query pass/assertion/skip |

Full negative, retry, platform/version and lifecycle qualification remains open.
Fixtures and parser tests are not substitutes for those required live witnesses.
Appium and domain-specific SQL/shader/HTML/data runner
IDs remain explicit unavailable obligations; they are not removed from release
scope. SwiftPM XCTest uses the serial native text protocol because its parallel
XML writer reported a skipped case as an ordinary passing testcase. Kotlin JS/native, Apple UI/device and physical
Android device variants are not qualified by the JVM/macOS/local-emulator rows.

## Invocation details and authority limits

Gradle requires one literal project directory, its pinned `build.gradle` or
`build.gradle.kts`, a fully qualified task path such as `:test`, and pinned Java.
A fixed init script selects that native Test task, forces a fresh XML location,
disables up-to-date reuse, and rejects no-match selection. Selectors require a
fully package-qualified class and method with a non-uppercase package prefix;
Gradle's uppercase simple-class shortcuts can otherwise match multiple packages. Project plugins and
other task/configuration code remain trusted local dependencies. Any reported
retry extension or duplicate identity refuses until a retry profile is qualified.

Maven requires the project's pinned `pom.xml`, pinned Java and optionally a pinned
settings XML in Reporter. The POM must set Surefire `reportsDirectory` to
`${corvint.reportsDirectory}`. Surefire has no equivalent user property to force
this setting. Missing wiring produces absent fresh reports and refusal; old target
reports are never imported. The fixed invocation is offline, requires tests and
sets `surefire.rerunFailingTestsCount=0`. Project plugin behavior and other POM
execution remain explicit caller dependencies. `JAVA_HOME` is derived from the
independently pinned Java executable for both launcher profiles.

Kotlin JVM profiles load the independently pinned Console jar and a literal
compiled-classes directory with its prepared runtime jars. Full artifact closure
is a separate input obligation. Kotlin-test selectors are exact class#method or
native unique IDs; Kotest selectors are native unique IDs. Reports retain full
engine/spec/test identity, including native parameter/nesting segments.

Android profiles execute only an explicit local `emulator-NNNN` serial and the
literal `package/androidx.test.runner.AndroidJUnitRunner` component. They never
install APKs, clear data or grant permissions. APK identity, existing adb server,
emulator image, application state and SDK dependencies remain caller admission
and qualification obligations. Adb exit zero is outcome-neutral: native per-test
start/terminal records, ordinal/count consistency, aggregate summary and final
instrumentation code determine observed outcomes. A native exception without
recognized assertion evidence remains UNKNOWN failure kind. Espresso and
UIAutomator sharing this transport does not imply they share semantic coverage.

Aggregate outcomes retain failure kind/message without fabricated attempts.
Native retry history stays NOT_REPORTED unless actually available. A complete
report means complete observed inventory, never full expected-selector coverage,
criterion acceptance, mutation kill or Tasks completion.

Actual raw reports, source fixtures, APK checksums, runtime versions and launcher
transcripts are retained in `/private/tmp/cem10-build/platform` during the build.
See testdata/README.md for fixture normalization and provenance.

SwiftPM XCTest requires the selected package's pinned Package.swift and exact
Module.Class/testMethod selectors. The qualified serial macOS protocol validates
case lifecycle, suite nesting, failure-event counts and skipped-body counts. A
setup exception can mark a body skipped while the method still fails with an
infrastructure error. Linux output, retries and arbitrary custom observers remain
unqualified. The Swift 6.4 --skip-update option is deprecated.

TRE-V0-024 (V1-0597) retains the actual skip loss: `swiftpm-xunit-parallel.xml` is
Swift 6.4 (swiftlang-6.4.0.34.1), macOS 26.6.2 arm64, `--parallel --xunit-output`,
where XCTSkip is an ordinary passing testcase without a `file` attribute. The
swift-xctest profile never requests that transport and refuses any report file;
TCQ JUnit import leaves those rows unkeyed. The opt-in
`TestSwiftPMXCTestLiveThreeOutcomes` (`CORVINT_SWIFTPM_LIVE_ROOT`,
`CORVINT_SWIFTPM_LIVE_EXE`, optional `CORVINT_SWIFTPM_LIVE_OUT`) drives the shared
executor with the regular-file `/usr/bin/swift` shim; the toolchain `swift` is a
symlink, which executor admission refuses. A caller-pinned wrapper that writes this
XML under a generic JUnit profile (bun-test, deno-test, pytest) would still decode
it as complete: that is the trusted-local-executable limit of TRE-V0-004.

TRE-V0-025 (V1-0613): SwiftPM 6.4 runs the xctest child and Foundation `Process`
helpers outside swift-test's process group, so the group kill alone leaves them.
New swift-xctest plans set `retireDetachedDescendants`; the executor then stops and
kills owned processes proved by ancestry or the per-phase owner token, and records
them in the execution's `retirement`. The opt-in `TestSwiftPMXCTestLiveDetachedTeardown`
runs `ProofTests.Hang/testHang` with a `CORVINT_SWIFT_READY` marker path, which the
fixture writes as "xctestpid helperpid" before hanging. Darwin arm64/amd64 only.

Robolectric requires prepared compiled classes/runtime jars, pinned Console jar
and the exact pinned API-35 android-all-instrumented artifact for Robolectric
4.16.1. The fixed invocation sets offline mode and enabledSdks=35. Other SDKs,
resource/application variants and complete dependency closure remain unqualified.
