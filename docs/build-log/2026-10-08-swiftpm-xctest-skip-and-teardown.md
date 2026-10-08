# 2026-10-08: SwiftPM XCTest skip loss and detached teardown (V1-0597, V1-0613)

## Intent

V1-0597 records that SwiftPM 6.4's parallel xUnit file reports an XCTSkip as an ordinary pass.
This entry retains that witness, binds the serial native transport as the only SwiftPM XCTest
execution path, and adds proposed `TRE-V0-024` to `docs/specs/test-runner-execution-v0.md`.
V1-0613 adds proposed `TRE-V0-025`: bounded, ownership-proved retirement of the detached
xctest and helper processes SwiftPM leaves outside the runner's process group.
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

## V1-0613: detached XCTest teardown

Failing before (actual, shared executor without retirement): the scratch fixture
`/private/tmp/claude-501/swiftpm-td/fx` `ProofTests.Hang/testHang` starts a Foundation
`Process` running `/bin/sleep 600`, writes "xctestpid helperpid" to the `CORVINT_SWIFT_READY`
path and hangs. With a 30 s bound the run timed out and the receipt was incomplete
(`PROCESS_TIMEOUT`, `timeout`, `no-tests`, `runner-exit`). The xctest process (60944) and the
helper (60986) both survived the group SIGKILL, because each has its own process group. The
test then killed them after re-checking their identities. Evidence:
`/private/tmp/claude-501/swiftpm-td/live2/teardown-contained-only.json` (sha256
`7d09d119…ba92`). An earlier standalone probe (`repro.py`) showed the same survival chain:
swift-test 37926, then xctest 38438, then sleep 38450.

Design: `internal/groupreap` gains a `Retirer`.

- Ownership is proved only in two ways: ppid ancestry from the identity-verified live leader,
  or a random per-phase `CORVINT_TEST_RUNNER_OWNER` token read from KERN_PROCARGS2. Plans
  cannot set that key.
- Teardown first SIGSTOPs owned processes until no new one appears (at most 32 rounds). It
  then SIGKILLs each pid whose start time it has re-verified, and polls for up to 3 s.
- It runs in `cmd.Cancel` before the group kill (timeout and interruption). It also runs after
  the leader exits but before the leader is reaped, so normal exit is covered too.
- Foreign-uid, stop-refused or surviving processes, and an unconverged or unbounded scan, are
  retained in `execution.retirement`. They fail Execute, which yields an `execution-boundary`
  problem, so the observation is incomplete.
- New `swift-xctest` plans set `retireDetachedDescendants`. The field is omitempty, so
  historical plan and receipt bytes and identities are unchanged, and existing receipts still
  bind. `corvint-test-runner run` refuses a historical `swift-xctest` plan that differs only
  by the absent flag, with a re-plan diagnosis.

Passing after:

- Live, through `/usr/bin/swift` and the same fixture:
  - Timeout run: xctest 70855 and helper 70885 were retired by ancestry, nothing survived, and
    the retirement record was clean (`teardown-timeout.json`, sha256 `508d5192…718d`).
  - Interruption run: xctest 80476 and helper 80495 were retired, nothing survived, and the
    record was clean (`teardown-interrupt.json`, sha256 `a8a9fb77…6df9`).
  - Both observations stay incomplete, as they should after a timeout or interruption.
  - The three-outcome witness still passes with retirement enabled (`three.json`).
- Focused tests:
  - `groupreap`: ownership refusals against an injected table (unrelated, reused pid,
    pre-phase, zombie, foreign uid, immortal, fork storm) and real-table retirement in three
    modes: none, live leader, and post-exit token.
  - `testrunner`: retirement on timeout, interruption and normal exit; the no-retirement
    baseline survives; admission refusals; refusal off Darwin; a cleanup failure cannot
    leave a complete observation; plan byte identity.
  - `corvint-test-runner`: the historical plan and receipt keep their bytes; a
    pre-retirement `swift-xctest` plan is refused for execution with a re-plan diagnosis; a
    plan cannot add the flag outside its profile.

Review (codex, gpt-6-astra, on a8bcd149) found three issues, all fixed in the follow-up commit:

- P1: identity and owner-token read errors were treated as "gone" or "not owned", which
  could falsely certify cleanup. They are now retained as `identity unreadable` or as
  problems. `TestRetirerKeepsReadFailuresAsUncertainty` failed on a8bcd149 and passes after
  the fix. A probe found 0 of 684 live same-uid processes with an unreadable token, so this
  does not add false failures on this host.
- P2: historical `swift-xctest` plans no longer matched the rebuilt profile. They were first
  admitted with retirement on. Re-review then found that this breaks the receipt's
  invocation binding in `cemcandidate`. The final rule is the explicit re-plan refusal
  described above.
- P2: a scan failure skipped the kill of processes it had already stopped. Stopped processes
  are now always killed.

While fixing the P1 issue, the first executor run recorded a spurious `owner token unreadable`
problem. The affected processes were unrelated same-uid processes that were between fork and
exec, or exiting, while being scanned. Token reads are now retried up to five times while the
process identity persists. After that, six repeated runs of the executor and groupreap
retirement tests passed. The live rerun after the fixes also passed:

- the contained-only baseline again left 13597 and 13647 running;
- the timeout run retired 21401 and 21413;
- the interrupt run retired 29766 and 29782.

Evidence is in `/private/tmp/claude-501/swiftpm-td/live3/` (`teardown-timeout.json`, sha256
`b1511d83…1e4e`; `teardown-interrupt.json`, sha256 `11db8b76…ee9d`).

Non-goals:

- Linux and Windows retirement. Those platforms refuse the flag before launch.
- Processes started through launchd or XPC.
- Retiring processes that are neither descendants nor token holders.
- Graceful-interrupt profiles such as Playwright.

Failure modes:

- A pid could be reused between the final check and the signal.
- A descendant that scrubs its environment and is reparented before the ancestry snapshot is
  not seen.
- A timed-out phase's receipt keeps `exitCode` -1. The closed document decoder refuses that
  value independently of this change, which was observed while writing the decode
  round-trip test. That is a pre-existing limit and is fail-closed.

Rollback: remove `retire*.go`, the `wait` hook, the `retirement` field and the flag in
`buildXCTest`. A plan that already sets the flag would then be refused as having an unknown
field.

NOT_RUN: Linux and Windows execution of the refusal test (cross-vet only); Swift versions
other than 6.4; `make gate`.

Re-review (codex, on 5df456b1) found two P2 issues, both fixed in the final commit:

- Admitting historical plans with retirement on broke receipt binding; they are now refused
  for execution, as described above.
- The retirement record was merged only after the post-run binding and write checks, so an
  earlier failure could discard it. It is now merged immediately after the run.
  `TestRetirementSurvivesLaterExecutionFailure` failed on 5df456b1 and passes after the fix.
