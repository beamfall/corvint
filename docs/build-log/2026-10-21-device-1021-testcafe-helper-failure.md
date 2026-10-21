# 2026-10-21 device-1021: TestCafe helper failure is never a pass, plus device ticket triage

Lane device-1021 (pre-1.0 sweep), base `f284484ebc613c8b61995f7af3ac53d170973be1`.

## Problem

V1-0605: on macOS, TestCafe 3.7.6 with a fresh isolated HOME printed `EEXIST` during the copy of
its browser helper into `$HOME/.testcafe-browser-tools` and aborted. The `testcafe` parser did not
read native output, so a run that printed the helper failure but still wrote a passing JSON report
with exit 0 was a complete PASSED observation.

## Decision

`TRE-V0-114` (test-runner-execution-v0, accepted under the owner's standing auto-accept of
2026-10-11): a native output line naming `.testcafe-browser-tools` with a Node system-error code
records `browser-helper-failed` with the native line, so the observation is incomplete. The
executor does not create or repair the helper; fresh-HOME first-use setup is an unsupported tuple
and a caller provisions the helper in its own task-owned HOME. The detail is the native line
itself, so the native failure is retained rather than summarised.

## Triage of the other lane tickets

- V1-2121: already delivered. Run 39121848489 ran head `0fce983`, before `f8c33fba7` (V1-1771),
  which replaced the `caller_deadline` case's 60 ms sleep against a 50 ms timer with a wait on the
  caller context (`internal/mcp/bridge/git_timeout_test.go:80@14c7ba94`). Scheduled release-gates runs
  39258757168 and 39416257984 passed `internal/mcp/bridge` on macos-15.
- V1-1553: the device-cleanup contract `TRE-V0-087` landed in `547048cc9` and `aacaac24d`; the
  live emulator/Appium witnesses need a booted device and an Appium server, outside this lane.
- V1-1246: the defect is in the Computer Use server's `getApp`, which this repository does not
  contain; skipped as external.

## Evidence

`TestTestCafeBrowserHelperFailureIsNeverAPass` fails without the fix (passing report plus native
EEXIST stderr was `complete=true problems=[]`) and passes with it.

## NOT_RUN

Live fresh-HOME TestCafe reproduction: the local TestCafe tree is incomplete and a reinstall needs
owner approval. Live Android/Appium cleanup witnesses (V1-1553).

## Rollback

Remove `internal/testrunner/dynamic/testcafe.go` and call `reconcileMochaSelection` from `Parse` again; no plan, invocation or wire bytes change.
