# 2026-10-06: V1-0862 Darwin pressure signal and level reason

## Intent

Ticket V1-0862, GitHub [beamfall/corvint#636](https://github.com/beamfall/corvint/issues/636). On
macOS the CAL-V0-068 dispatch throttle (source `darwin-sysctl-host`) stayed at level 2 for a whole
day while the host had spare capacity. `vm.swapusage` used is sticky: macOS keeps pages in swap
after pressure passes until their owner touches them. The swap fraction (0.85) therefore never fell
below `calmSwap`, and the level could not be released. The load average was also inflated by
Virtualization.framework vCPU threads. Operators could not see which signal held the level.

## Change

- Spec `docs/specs/corvint-tasks-agent-leases-v0.md` adds CAL-V0-109..110 as a V1-0862 amendment
  inside S18. It amends the CAL-V0-068 sampling text (macOS sysctl names, one memory signal per
  OS) and its non-goals, and adds a slice row, two failure-mode rows, traceability rows and the
  issue to the authoritative inputs. The delivery status in the spec, `README.md` and `INDEX.json`
  names the amendment.
- CAL-V0-109: the macOS sample now runs `/usr/sbin/sysctl vm.loadavg
  kern.memorystatus_vm_pressure_level hw.logicalcpu` and records the kernel level
  (`memoryPressureLevel`, `memoryPressureKnown`). Level 1 (normal) is calm, 2 (warn) is high and 4
  (critical) is critical. The macOS sample no longer reads swap. A sample that carries a kernel
  level uses it even when swap is also present. A missing, duplicated or other level is UNKNOWN and
  never falls back to swap. `parseDarwinSwap`, which no longer had a caller, is removed. Linux is
  unchanged.
- CAL-V0-110: `PressureState.reason` records the sorted signals (`load`, `memory`, `swap`) that set
  the level when it changes. A release clears it, and holding, UNKNOWN and restart keep it.
  `dispatch status` reports `reason` (`NONE`, the joined signals, or `UNKNOWN` for an older record)
  and `memoryPressureLevel`. The `throttled` event carries both as details and names them in its
  message.
- The ledger refuses a reason outside the closed set, an unsorted list, a reason at level 0, or a
  kernel level outside 1, 2 and 4 or without its flag.

## Decisions

- **Kernel level rather than swap-out rate.** The kernel level is one more name in the existing
  sysctl call. A swap-out rate needs a previous sample and a rate threshold, which means more state
  and new configuration. The kernel already applies its own hysteresis to the level.
- **No fallback to swap on macOS.** If the level were missing, using swap would bring back the
  sticky signal and would claim knowledge that the sample lacks. The sample is UNKNOWN instead, and
  CAL-V0-068 keeps the level.
- **Fixed kernel-level mapping, existing configuration unchanged.** The kernel levels are discrete,
  so no new thresholds were added. The swap thresholds remain required by validation and apply only
  on Linux.
- **The reason is recorded at the level change.** The ticket asks for "the signal that set the
  current level". Reporting the signal that currently prevents a release is a non-goal; the status
  inputs still show each value.
- **The CPU-utilisation signal is not delivered.** `host_processor_info` needs cgo or a sampling
  command such as `iostat`, which takes a second or more. Neither is cheap or clean. The load average
  stays the load signal, and `reason: load` shows when it holds the level.
- **Independent review.** Codex (read-only) reviewed the first commit and reported one P3 finding:
  the throttled message for an UNKNOWN sample left out the memory signal that CAL-V0-110 requires
  it to name. The finding was verified and fixed. That message now carries the load and memory
  signals (`memory pressure level N`, `swap F`, or `memory UNKNOWN`), and
  TestCALV0110_ThrottledEventReportsReason covers a partly UNKNOWN sample and a fully UNKNOWN one.
  Codex found no other correctness or portability defect, and the requirement locators and test
  names resolve. Codex also noted that the freshness of the installed Corvint tools was not
  verified in this lane; that is retained as NOT_RUN.

## Evidence

On Darwin, `GOMAXPROCS=2 GOTOOLCHAIN=local go test -p 1 -count=1 -timeout 30m`:

- `./internal/tasks/dispatch/` (whole package) PASSED.
- `-run 'Dispatch|CALV0068|CALV0110' ./internal/tasks/cli/` PASSED.
- New tests PASSED: TestCALV0109_StickySwapWithNormalPressureReleases (swap 0.95 with kernel level 1
  releases level 2 after the dwell; the same swap without a kernel level still holds),
  TestCALV0109_KernelMemoryLevelMapping, TestCALV0109_DarwinSysctlParsing,
  TestCALV0110_ReasonRecordedAtLevelChange, TestCALV0110_ThrottledEventReportsReason,
  TestCALV0110_LedgerReasonAndMemoryLevelValidated and TestCALV0110_DispatchStatusReason.
- `CORVINT_ISSUE497_LIVE=1` TestIssue497_LiveSampler PASSED on this Mac: source
  `darwin-sysctl-host`, 12 CPUs, memory signal `memory`, kernel level 1.
- In a `golang:1.27.1` Linux container (aarch64), the focused pressure tests in
  `internal/tasks/dispatch` and `internal/tasks/cli` PASSED. The live sampler there used the swap
  signal with source `linux-proc-host`.
- Mutation checks, each reverted afterwards: disabling the kernel-level branch failed
  TestCALV0109_StickySwapWithNormalPressureReleases, and dropping the reason append failed two
  CAL-V0-110 tests.
- `go vet` PASSED for GOOS darwin, linux and windows on `internal/tasks/dispatch` and
  `internal/tasks/cli`. `gofmt -l` was clean.
- These checks PASSED: `make spec-requirements-check requirement-definitions-check
  traceability-tests-check decision-numbers-check line-citations-check error-code-ownership-check
  unbounded-readers-check use-case-receipts-check diagnostic-coverage-check`.

## Not run

- NOT_RUN: a live dispatcher under real macOS memory pressure (kernel level 2 or 4), `make gate`,
  the full `internal/tasks/cli` and `cmd/corvint` suites, CEM dogfood binding, and native completion
  of V1-0862, and a freshness check of the installed Corvint Core and Tasks binaries.
- Not delivered: the optional CPU-utilisation signal, and any correction of the load average for
  virtualization threads.

## Rollback

Revert the commit. An older binary refuses a ledger whose pressure record carries `reason` or a
kernel level, so before a downgrade, start the dispatcher once without `pressure`, which drops the
record. Running workers are unaffected either way.
