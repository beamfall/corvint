# Tasks: per-test-durations

Build in this order. Each task names its requirement IDs and the proof that closes it.

- [x] 1 Create `docs/specs/per-test-durations-v1.md` with PTD-V1-001 to PTD-V1-009 from the spec
   delta as proposed, and regenerate `docs/specs/REQUIREMENTS.tsv`. Requirements: PTD-V1-001 to
   PTD-V1-009. Proof: the spec toolchain check passes.
   Done: V1-5449 filed; PTD-V1 reserved; `docs/specs/per-test-durations-v1.md` created and accepted
   under G14 (acceptance-log row `per-test-durations`).
- [x] 2 Add the add-only receipt fields and the `NOT_MEASURED` and `duration-invalid` rules.
   Requirements: PTD-V1-001, PTD-V1-003, PTD-V1-007. Proof: schema round-trip tests; an older reader
   test that ignores the fields.
- [x] 3 Mark file-granularity durations and keep invocation time separate.
   Requirements: PTD-V1-004, PTD-V1-005. Proof: a file-level fixture; an invocation time test.
- [x] 4 Copy durations into run evidence rows and wire the consumers.
   Requirements: PTD-V1-008, PTD-V1-009. Proof: run evidence fixture; cost figure fixture with one
   unmeasured test labelled incomplete.
   Done: run evidence rows copy the receipt's duration members; `internal/testduration` `Cost` and
   `Median` are the measured-only rule the cost figure (NMS-V1-002) and the fault-injection harness
   (FIH-V1-012) call. Neither consumer is on this tree; each adopts the rule in its own change.
- [x] 5 Record the trace and the delivery state in the spec.
   Requirements: all. Proof: the trace check maps every PTD-V1 ID to a test.
- [x] 6 Read durations in the Go, Vitest, Jest, Playwright, pytest and RSpec adapters.
   Requirements: PTD-V1-002, PTD-V1-006. Proof: one report fixture per runner, including a retry.
   The `internal/testrunner` adapters read every runner's report time, with a fixture each
   (V1-5449). In the `corvint test` receipt, per runner: Go, package-level `Elapsed`, only for a
   package whose tests ran; Vitest, the provider receipt's test `durationMs`, summed per file unit;
   Playwright, the provider receipt's `attemptDetails` durations, summed per test and per file unit,
   including a retry (`attemptDurationsMs` stays on the `corvint-test-runner` observation, since a
   receipt outcome is a file); Jest, pytest and RSpec (V1-5542, 2026-11-13), the contract adapter's
   sum of the report times of each selected file's tests (assertion `duration`, JUnit `time`,
   `run_time`), NOT_MEASURED when one is absent or a selected test is NOT_RUN, invalid when one is
   invalid: `TestPTDV1002ContractFileDurations`, `TestPTDV1002PytestReceiptDurations`,
   `TestRNCV1019ContractRunRecordsPerTestResults` (`internal/sessiontest`),
   `TestPTDV1007JestReportDurations`, `TestPTDV1002RSpecReportDurations`,
   `TestPTDV1002PytestJUnitTimes` (`internal/testrunner/contract`). Live Jest, pytest and RSpec runs
   are NOT_RUN (none is installed on the lane host); the reports are hand-written in each runner's
   documented shape.
- [x] 7 Add the time field to the Minitest reporter and read it.
   Requirements: PTD-V1-002. Proof: Minitest fixture with measured durations.
   Corvint's Minitest reporter records `DurationMs` from `Minitest::Result#time` and the reader
   takes it as `minitest-reporter` (V1-5449): `TestPTDV1002MinitestReporterDuration` and
   `TestPTDV1002MinitestReporterOutput` (`internal/testrunner/dynamic`) pin the reporter source and
   parse a report in its exact output shape. A `corvint test` receipt runs Minitest through its
   full command, since the contract has no Minitest selection (RNC-V1-004), and reads no report,
   so its Minitest outcomes are NOT_MEASURED under PTD-V1-003, never estimated:
   `TestPTDV1003MinitestFullCommandIsNotMeasured` (`internal/sessiontest`, V1-5542). A Minitest
   contract adapter, which would carry the measured time into the receipt, belongs to change
   `ruby-providers`.
