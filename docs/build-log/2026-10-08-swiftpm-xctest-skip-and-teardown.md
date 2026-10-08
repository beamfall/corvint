# 2026-10-08: SwiftPM XCTest skip loss and detached teardown (V1-0597, V1-0613)

## Intent

V1-0597 records that SwiftPM 6.4's parallel xUnit file reports an XCTSkip as an ordinary pass.
This entry retains that witness, binds the serial native transport as the only SwiftPM XCTest
execution path, and adds proposed `TRE-V0-024` to `docs/specs/test-runner-execution-v0.md`.
The tuple is Swift 6.4 (swiftlang-6.4.0.34.1, swift-driver 1.168.6), macOS 26.6.2 (25G83),
arm64. Owner acceptance is pending.

## V1-0597: skip loss

Failing before (actual): in a scratch package with `testPass`, `testFail` (XCTAssertEqual) and
`testSkip` (throws XCTSkip), `swift test --enable-xctest --disable-swift-testing --parallel
--xunit-output parallel.xml` exited 1. Its XML (sha256 `97ba203e…3210`) has three testcases. One
has `<failure>`, and `testSkip` is an empty passing testcase with no `skipped` element and no
`file` attribute. Before this change no fixture or test retained this loss.

Passing after:

- `swift-xctest-three.txt` is the shared-executor stdout of the same package. The scratch path
  is normalized to `/fixture/swift-xctest-proof`. `swiftpm-xunit-parallel.xml` is the
  unmodified XML. Both are in `native-report-fixtures.json`.
- `TestSwiftPMXUnitSkipLossCannotSatisfyExecution` checks three things. The native stdout parses
  complete as PASSED, FAILED/ASSERTION and SKIPPED. `swift-xctest` refuses the XML, both alone
  and alongside valid stdout. Build requests neither `--parallel` nor `--xunit-output` and
  declares no report path.
- `tcq` `TestSwiftPMXUnitRowsStayUnkeyed` finds zero keyed and three unkeyed rows, so the
  disguised skip cannot key criterion evidence.
- Live: `TestSwiftPMXCTestLiveThreeOutcomes` passed with
  `CORVINT_SWIFTPM_LIVE_EXE=/usr/bin/swift` (sha256 `b8763cf2…10e9`). The run went through
  Build, then shared Execute, then Parse and Normalize, and ended with exit 1 and a complete
  PASSED/FAILED(ASSERTION)/SKIPPED observation. Retained evidence:
  `/private/tmp/claude-501/swiftpm-td/live1/three.json` (sha256 `eef2ace3…8911`).

Findings:

- The toolchain `swift` is a symlink to `swift-frontend`. Executor admission requires a regular
  file, so it refuses the symlink. The `/usr/bin/swift` xcrun shim is a regular file and works.
- In a scratch probe across all registry runners, the bun-test, deno-test and pytest generic
  JUnit parsers decode this XML as complete. A CEM receipt can reach that only when a caller
  pins an executable that writes SwiftPM XML under another runner's fixed argv. That is the
  trusted-local-executable limit of `TRE-V0-004`, recorded in the spec rather than fixed here.
  AFU JUnit run ingest is not CEM execution and is unchanged.

Non-goals: Linux SwiftPM, Swift Testing through this profile, other Swift versions, and any
fingerprinting of SwiftPM XML in generic JUnit parsers.

Failure modes: if a future SwiftPM emits `<skipped/>`, the negative fixture still witnesses 6.4
only. A new tuple needs its own fixture before the profile may change.

Rollback: remove the two fixtures, the inventory count change, the two tests, the opt-in live
test and the spec section. The profile and historical plan and receipt bytes are untouched.

NOT_RUN: Linux; Swift versions other than 6.4; `make gate` (owner policy for scoped work).
