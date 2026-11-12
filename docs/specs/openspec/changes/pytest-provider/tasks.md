# Tasks: pytest-provider

Build after changes `runner-contract`, `canonical-test-id`, `test-inventory` and `run-evidence`
(owner decision R38 (2026-11-09) build order). Each task names its requirement IDs and its proof.

- [ ] 1 File the ticket, reserve prefix `PYR-V1` and set `requirementRefs` to the ten IDs. Proof:
   `ticket show` read back.
- [x] 2 First commit: create `pytest-provider-v1.md`, its `docs/specs/README.md` row and the
   regenerated index. Requirements: PYR-V1-001..010. Proof: spec diff (V1-5456, first commit).
- [x] 3 pytest lister for `corvint inventory refresh`, one instance per rootdir. Requirements:
   PYR-V1-001. Proof: fixture inventory test. The `pytest-collect` lister of change
   `test-inventory` already collects per rootdir; `TestPYRV1001PyprojectRootIsListed` and
   `TestPYRV1001TwoRootdirsAreTwoInstances` (`internal/testinventory`) prove both scenarios, and
   `TestRNCV1002AffectedRunsNoListerInAPytestProject` (`cmd/corvint`) proves affected runs none.
- [x] 4 Wire the compiled-in pytest runner into `corvint test` at tier `file`; per-test outcomes and
   durations keyed by canonical ID. Requirements: PYR-V1-002, 003, 004. Proof: mixed-plan and
   failing-test fixtures: `TestPYRV1002PytestFilesRunAtTierFile`, `TestPYRV1003PytestOnlyPlanIsNarrowed`,
   `TestPYRV1003MixedPlanRunsGoAndPytest`, `TestPYRV1004FailingTestFailsTheRun` (`internal/sessiontest`),
   `TestPYRV1004PytestJUnitReport` (`internal/testrunner/contract`), `TestPYRV1003CLIRunsPytestSelection`
   (`cmd/corvint`). The tier label is in the plan (manifest units), the receipt and on stderr; watch
   is NOT_PRODUCED (change `watch-seven-runners` owns `corvint watch`).
- [x] 5 Fail-closed paths including xdist and plugin deselect. Requirements: PYR-V1-006. Proof: one
   fixture per path: `TestPYRV1006RunnerTroubleFailsClosed` (no interpreter, pytest not importable,
   nothing collected, usage and internal errors, invalid report, plugin deselect, a file with no
   result; `internal/sessiontest`), `TestPYRV1006PytestJUnitRefusals`, `TestPYRV1006XdistMergedReport`,
   `TestPYRV1006PytestOutputSignals` (`internal/testrunner/contract`). Fixtures are hand-written from
   pytest's and xdist's source; no live pytest ran (NOT_RUN).
- [x] 6 Kind labels; environment record; advisory parity test. Requirements: PYR-V1-005, 007, 008.
   Proof: fixture tests. `TestPYRV1005KindLabels` (`internal/testinventory`; lister `pytest-collect`
   version 2 labels each test from its own source: pytest-playwright fixtures, the `integration`
   marker); `TestPYRV1003MixedPlanRunsGoAndPytest`, `TestPYRV1004FailingTestFailsTheRun`
   (`internal/sessiontest`, environment record, delivered with task 4);
   `TestPYRV1007AdvisoryCommandNamesTheRunTests` (`cmd/corvint`). The owner's kind declaration of
   change `test-inventory` does not exist in this tree, so only the `integration` marker is read
   (NOT_PRODUCED).
- [x] 7 Kill switch. Requirements: PYR-V1-009. Proof: per-runner switch test:
   `TestPYRV1009KillSwitch`, `TestPYRV1009FallbacksNameTheirReason` (`internal/sessiontest`),
   `TestPYRV1009SelectionGrammar` (`internal/selectionswitch`), `TestPYRV1009CLIKillSwitch`
   (`cmd/corvint`). No `CORVINT_SELECTION` parser existed in this tree; `internal/selectionswitch`
   implements exactly the NMS-V1-008 grammar, which owns it, for change `narrowing-measurement` to
   reuse. Built with task 4, since the switch decides which units task 4 plans.
- [x] 8 Harness adapter for FIH-V1. Requirements: PYR-V1-010. Proof: harness call test:
   `TestPYRV1010PytestHarnessAdapter`, `TestPYRV1010PytestHarnessFailedMeasurements`,
   `TestAdapterContract_FIH_V1_008` (`tools/fault-harness`). Its dependent units are every collected
   test file (a conservative superset); mapping the plan's `python:` units to pytest entries in the
   harness is NOT_PRODUCED.
- [ ] 9 Tests under "Acceptance evidence"; build-log entry; complete through the Tasks gate. Proof:
   retained receipts, `receipt audit`. Tests and the build-log entry
   (`docs/build-log/2026-11-12-pytest-provider.md`) are written; the fixture repositories are
   hand-written per requirement. Left open: the Tasks-gate completion (orchestrator) and the live
   evidence, NOT_PRODUCED: `corvint test` on pydantic/pydantic at the PRR-V1 pin and the NMS-V1 and
   FIH-V1 reports for pytest need a live pytest, network access and the NMS-V1 build.
