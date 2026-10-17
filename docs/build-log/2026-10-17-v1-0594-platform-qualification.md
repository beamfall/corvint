# 2026-10-17 V1-0594: platform runner qualification matrix and Swift Testing selector fix

Gap matrix. The 16 platform, two SQL and one Appium Android profiles were checked against the
`TRE-V0-009` cases: pass, assertion failure, skip, zero tests, build or collection error, and
timeout with cleanup. That is 114 applicable cells; retry is not applicable to any of these
profiles. At base `8cd20e92b`:

- No cell had a retained actual witness. The earlier raw reports, receipts and commands under
  `/private/tmp/cem10-build` had been purged.
- 71 cells had historic native bytes committed as parser fixtures, or a spec-recorded live run.
  These count as parser-only.
- 43 cells had no evidence.

After this change, 18 cells have retained actual witnesses, 59 are parser-only and 37 have no
evidence. The full matrix and the evidence paths are in the new "Platform qualification matrix
2026-10-17 (V1-0594)" section of `docs/specs/test-runner-execution-v0.md`.

Closed cases. All 18 are the Apple profiles: swift-testing, swift-xctest and xcode-xctest, each
for pass, failure, skip, zero, build error and timeout.

- Host: macOS 26.6.2 (25G83) arm64.
- Swift: 6.4 (swiftlang-6.4.0.34.1), through the regular-file `/usr/bin/swift` (sha256 `b8763cf2…`).
- Xcode: 27.0 (27A266a), through `/usr/bin/xcodebuild` (sha256 `cdd556d4…`).
- xcresulttool: 25115, schema 0.4.0, pinned separately as a tool (sha256 `6e5a6ee6…`).

Every case went through `corvint-test-runner plan` and `run --experimental --trusted-local`
from this branch's build. Results:

- A skip was never accepted as a pass. That covers Swift Testing `.disabled`, `XCTSkip` and
  the Xcode skip.
- Zero-test runs and build errors stayed incomplete.
- Each timeout left no surviving process. SwiftPM XCTest retired its detached `xctest` process
  and helper by ancestry.
- The opt-in SwiftPM live tests (`TRE-V0-029`, `TRE-V0-030`) passed on the same fixture package.

Bug found and fixed. The swift-testing profile anchored each selector as `^name$`. SwiftPM 6.4
matches `--filter` against the full Swift Testing ID, `Module.name()/File.swift:line:col`, so the
anchored filter matched nothing. Every selected run became a `NO_TESTS` refusal; it failed closed
but could never pass. The anchor is now `^name(/|$)` (`TRE-V0-075`, proposed). The before and
after live runs are retained, and `TestSwiftTestingSelectorMatchesSourceSuffixedID` checks the
exact and non-widening match. The earlier swift-testing witness ran without selectors, which is
why the bug went unseen.

Not closed. The session permission system refused network downloads (Gradle 9.6.1, Maven
3.9.16) and reads of user dependency caches, so nothing was installed. The JVM, Kotlin,
Robolectric, Android, Bats, ShellSpec, pgTAP, sqllogictest and Appium cells keep their earlier
classification. The spec section lists each missing tool, device or runtime as a release
blocker. A fixture whose directory was named `swift-testing` failed to build: its package
identity `swift_testing` collides with the SDK's Testing module package. The fixture was renamed.
The failed run is retained as `runs/swift-testing/build-error-package-identity`. It is a fixture
naming hazard, not an adapter defect.

Rollback: revert the anchor in `internal/testrunner/platform/platform.go`, and remove the test,
the spec section and its README row text.
