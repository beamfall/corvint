# 2026-10-22 lane flakes-oc-1022: procgroup escalation and cmd/corvint deadline flakes (V1-2446, V1-2433)

## Problem

- V1-2446: `TestPWPV0030InterruptGraceEscalatesToSIGTERM` (`internal/procgroup`) recorded
  `SIGKILL` / `process-timeout` instead of `SIGTERM` / exit 143 in release-gates run 39797313530
  (macos-15, full-gate load).
- V1-2433: on ubuntu-24.04 shards of run 39794835161, `TestWorkFinalCheckClosingContext/WQO-V0-032/deadline`
  failed `initial capture: ADAPTER_FAILED`, and `TestContextExpandDeadlineExpiryRefusesAsDeadlineExceeded`
  failed `cat-file all: the root is not its Git toplevel, want deadline-exceeded`.

## Root causes

- V1-2446 (test only): after the 300 ms interrupt grace, Run sends SIGTERM and then waits the
  default 100 ms terminate grace before SIGKILL. The TERM trap has to fork and exec `touch` before
  it exits, and on a loaded host that takes more than 100 ms. This is the same race V1-2319 fixed
  for `TestTerminateGraceLetsSlowTermHandlerFinish`. The product's escalation order is correct. A
  second, latent race: the 200 ms run timer could fire before `/bin/sh` had installed its traps.
- V1-2433 expand: the failing case took about 0 s; the 13 other cases took about 1 s each, for 13.38 s
  in total. So the deadline had not expired. The first Git call failed to start, and the deadline
  check found nothing to map. On Linux, exec of a freshly written script fails with ETXTBSY while a
  concurrent fork in the same process still holds the inherited write descriptor (Go issue 22315).
  `sourceGitScript` wrote the stand-in with `os.WriteFile` in this heavily parallel package.
  Separately, the 1 s wall-clock bound let load move the expiry to an earlier step than the one
  being tested.
- V1-2433 WQO (product): `observeWork` materializes the adapter in-process (`root.OpenFile`) and then
  execs it. A start refused with ETXTBSY because of a concurrent fork became `ADAPTER_FAILED`. The
  initial capture runs on `context.Background()`, so no deadline was involved.
  `internal/behaviorfalsify.startProcess` already retries this same class of failure.

## Fix

- `internal/procgroup/interrupt_posix_test.go`: `trapSpec` sets `TerminateGrace` to 10 s and
  `ShutdownTimeout` to 20 s. The shell also prints `ready` once its traps are installed, and
  `AfterStart` waits for that line (through a `StdoutTee`) before Run arms its timer.
- `cmd/corvint/work_runner.go`: `workStartAdapter` builds a fresh `exec.Cmd` for each attempt and
  retries an ETXTBSY start up to 5 times (5 to 200 ms backoff, cancellable). Other start errors are
  not retried. No spec requirement changes: a start that never happened is retried, and every
  receipt and error contract is unchanged.
- `cmd/corvint/source_handoff_unix_test.go`: `sourceGitScript` has a child `/bin/sh` write the
  stand-in, so the test process never holds its write descriptor. The expand deadline test now uses
  a 1 min hang-detector bound under a witnessed-deadline parent (`workFinalInterruption`). The
  deadline expires only when the stalled step's stand-in creates its `stalled` file, and the test
  asserts the stalled step was (or, for the range-refused recheck, was not) reached.

## Evidence

- Fail without the fix: V1-2446 with the old 100 ms grace plus a TERM trap delayed by 0.3 s
  reproduces the CI record exactly (`SIGKILL`, exit -1, `process-timeout`). The fixed spec passes
  with the same delayed trap. Breaking the product, by making `terminateProcessGroup` send SIGKILL
  first, fails the test. In `TestV12433WorkRunnerRetriesTextBusyAdapterStart`, disabling the retry
  fails both ETXTBSY cases. Removing the expand deadline mapping (V1-1412) fails 13 of 14 cases.
- Stress: `nice -n 15 go test -count=50 -run 'TestPWPV0030|TestPWPV0029SelfExit' ./internal/procgroup/`
  passed (49.7 s). A single compiled `cmd/corvint` test binary run with `-count=20` across the two
  flaky tests and the new retry test passed (104 s).
- Not reproduced: ETXTBSY cannot be reproduced on this macOS host, so the Linux mechanism is
  inferred from the CI timings and the `behaviorfalsify` precedent.

## Rollback

Revert the lane commits. The product change is limited to `workStartAdapter`. Reverting it brings
back the Linux ADAPTER_FAILED flake and changes no contract.
