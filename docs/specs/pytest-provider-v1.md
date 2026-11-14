# pytest provider V1

Owner: Russell Lewis
Date: 2026-11-12
Requirement prefix: `PYR-V1`
Intent status: accepted automatically under G14 (decision 0596, 2026-11-10; V1-5456)
Delivery status: experimental; partially delivered
Health: KEEP
Authoritative inputs: change folder `docs/specs/openspec/changes/pytest-provider/` (`proposal.md`,
`design.md`, `tasks.md` and `specs/pytest-provider/spec.md`), `docs/specs/PRODUCT-PLAN-1.0.md`
(PROD-TEST-006, PROD-TEST-008, PROD-TEST-011), `docs/specs/runner-contract-v1.md` (RNC-V1) and
`docs/decisions/0596-accept-product-plan-1-0-2026-11-10.md` (owner decisions R7 to R11, R13, R15
and R19).

## Agent digest
- Claim: `corvint test` runs the pytest files a plan selects through the compiled-in runner contract, one run per rootdir at tier `file`, with per-test results.
- Status: accepted automatically under G14 (decision 0596, 2026-11-10; V1-5456) / experimental; partially delivered
- Exists: pytest on the contract adapter of `corvint test` (tier `file`, per-test receipt members, typed refusals, environment record), the JUnit report, pytest kinds in the inventory, the harness's pytest adapter.
- Blocked on: watch (`watch-seven-runners`), the owner's kind declaration (`test-inventory`), a live pytest run, and the pydantic, NMS-V1 and FIH-V1 measurements.
- Read next: Requirements; Surfaces; Traceability.

## User and boundary

An agent, a developer or the GitHub Action runs the pytest tests a change selects. Before this
change `corvint affected` printed an advisory `python -m pytest` command for the selected files and
`corvint test` ran each pytest instance's full command, so no per-test outcome or duration reached
a receipt.

This spec is the canonical home of requirements PYR-V1-001 to PYR-V1-010, copied from the change's
`specs/pytest-provider/spec.md`. Design decisions, failure modes and rollback stay in the change's
`design.md`, which owns them; this file adds only the delivery record (Surfaces, Traceability).

## Requirements

- `PYR-V1-001`: (accepted automatically under G14, decision 0596; V1-5456) Discovery

The provider MUST supply the pytest lister, marked `executing`, to `corvint inventory refresh`
(change `test-inventory`, owner decision R10 (2026-11-09)). The lister MUST inventory pytest node
IDs for each configuration root through pytest's own collection, using the configuration search
order pytest.ini, .pytest.ini, pyproject.toml, tox.ini, setup.cfg, and MUST key each entry by the
canonical test ID (RNC-V1-012). `corvint affected` and `corvint watch` MUST NOT run it. Each
configuration root (rootdir) is one runner instance (RNC-V1-014, owner decision R15 (2026-11-09)).
(G1, G23)
Delivery: delivered. The `pytest-collect` lister of change `test-inventory` collects through pytest
from each rootdir (`--rootdir=.`, `-c CONFIG`), one instance per configuration root found in
pytest's search order (with pytest.toml first, as pytest 9 reads it), and keys each node ID by
canonical test ID; `corvint affected` starts no lister (RNC-V1-002).

**Scenario: pyproject root**

- GIVEN a repository whose pytest configuration is in `pyproject.toml`
- WHEN `corvint inventory refresh` runs
- THEN every collected node ID is listed with its file and canonical test ID

**Scenario: Two rootdirs**

- GIVEN pytest configurations in `services/a/pyproject.toml` and `services/b/pytest.ini`
- WHEN `corvint inventory refresh` runs
- THEN the inventory holds two pytest instances, one per root

- `PYR-V1-002`: (accepted automatically under G14, decision 0596; V1-5456) Selection granularity

The provider MUST run the files the plan selected, with tier `file` and an explicit tier label in
the plan, the receipt and watch (owner decision R8 (2026-11-09)). Python file-level selection is
recorded as a release exception; the Python line collector is a 1.0.x ticket (owner decision R9
(2026-11-09)). The provider MUST narrow below file level only when the line evidence for pytest
has passed the FIH-V1 proof (change `line-evidence`). Every selection MUST carry a reason.
(answer 3, G2)
Delivery: delivered for the plan and the receipt; NOT_PRODUCED for watch (change
`watch-seven-runners` owns `corvint watch` for pytest). Each pytest file is one contract unit of
its instance, the contract request carries tier `file`, the receipt invocation carries `tier` and
`exact`, and stderr prints `corvint-test: CONTRACT <instance> tier=file exact=B tests=N`. No FIH-V1
verdict for Python line evidence exists, so nothing narrows below the file.

**Scenario: Line evidence unproven**

- GIVEN no passed FIH-V1 verdict for pytest line evidence
- WHEN a plan selects a test function
- THEN its whole file runs and the selection carries tier `file`

- `PYR-V1-003`: (accepted automatically under G14, decision 0596; V1-5456) Run through `corvint test`

`corvint test` MUST run the plan's pytest selections through the compiled-in runner contract, one
invocation per configuration root, and MUST NOT require an external provider executable. (G23)
Delivery: delivered. The contract adapter of `corvint test` (RNC-V1-019) runs each instance's
selected files in one invocation from its root through `internal/testrunner/contract`; no provider
executable is involved. The instance must have a current inventory record, else it runs its full
command (RNC-V1-002). A live pytest run is NOT_RUN (pytest is not installed here; a stub answers at its path).

**Scenario: Mixed plan**

- GIVEN a plan with Go and pytest selections
- WHEN `corvint test` runs it
- THEN both runners execute and one receipt records both

- `PYR-V1-004`: (accepted automatically under G14, decision 0596; V1-5456) Per-test results

The receipt MUST record, per canonical test ID, the outcome (passed, failed, skipped, error,
xfailed, xpassed) and the duration of the call phase, as additive members of `corvint-test-run/0`.
Outcomes feed the run result record PROD-TEST-020 (change `run-evidence`, owner decision R7
(2026-11-09)); durations follow change `per-test-durations` (owner decision R19 (2026-11-09)).
(G4, G23)
Delivery: delivered as additive members of `corvint-test-run/0`: each runner invocation's `tests`
lists canonical ID, outcome, `runnerOutcome` (passed, failed, skipped, error, xfailed, xpassed) and
the call-phase `durationMillis` read from pytest's own JUnit XML report (`-o
junit_duration_report=call`). A non-strict XPASS is reported by pytest's JUnit writer as passed, so
it reads `passed` (NOT_VERIFIED against a live pytest). Each file outcome carries the summed JUnit
time of its tests under `pytest-junit` (PTD-V1-002); the run evidence rows copy the receipt
(RVD-V1-006). In xunit1 a testcase's `file` is the file that defines the test, while its
classname is built from the node ID pytest collected it under; an inherited base-class method or
an imported test function therefore names another file. The parser splits the classname against
the modules of the selected files first, then the inventory's files, then the `file` attribute's
own module, and attributes the testcase to the file whose module prefixes it; a classname no
candidate prefixes, or two of one group prefix, still refuses the report (review P2-b,
2026-11-13). A collector record or a file that failed to collect carries no time, so its file is
NOT_MEASURED (PTD-V1-003).

**Scenario: Failing test**

- GIVEN one selected test fails
- WHEN the run ends
- THEN the receipt names that test's canonical ID as failed with its duration and the run
  result is FAIL

- `PYR-V1-005`: (accepted automatically under G14, decision 0596; V1-5456) Kind label

Each pytest test MUST carry `kind` and `kindSource`: e2e when it uses pytest-playwright fixtures,
integration when it carries the pytest marker `integration` (or a marker named in the owner's
kind declaration of change `test-inventory`), unit otherwise, UNKNOWN on conflict. The label MUST
NOT change selection in 1.0. (G13, review T-m4)
Delivery: partial. The lister (version 2) labels each test from its own source: e2e for a
pytest-playwright fixture parameter or `usefixtures` when a manifest or the file names the plugin,
integration for the `integration` marker on the function, its class or the module, UNKNOWN on both
or when the function is not in the file, unit otherwise. NOT_PRODUCED: markers named in the owner's
kind declaration (no such declaration exists in change `test-inventory` in this tree), and fixtures
applied by a conftest, autouse or an inherited method, which a source scan does not see.

**Scenario: Playwright fixture**

- GIVEN a test taking the `page` fixture from pytest-playwright
- WHEN inventory runs
- THEN its kind is e2e with `kindSource` runner

**Scenario: Integration marker**

- GIVEN a test decorated with `@pytest.mark.integration`
- WHEN inventory runs
- THEN its kind is integration

- `PYR-V1-006`: (accepted automatically under G14, decision 0596; V1-5456) Fail closed on runner trouble

A missing interpreter or pytest, zero collected selections, an internal or usage error, or an
invalid capture MUST produce a non-PASS result with a typed reason. Selected tests absent from the
results are `NOT_RUN` and the run is `INCOMPLETE` (RNC-V1-006, review T-m2). (G1, G10)
Delivery: delivered. Typed reasons: `runner-unavailable` (the command's discovery probe fails),
`no-tests-collected` (exit 5), `runner-exit` (exit 2, 3, 4 or outside 0 to 5), an unreadable report,
`tests-deselected` (pytest's printed deselected count), and a selected test with no result. The
expected tests are the selected files plus every test the committed inventory's pytest collection
lists in them (RNC-V1-006), so a test a plugin deselects with no trace in the output (`-p
no:terminal`) is NOT_RUN. A selected file the inventory lists no test in (a changed or new test
file, TIV-V1-008) runs whole by its file node ID (the RNC-V1-002 widening) and is reconciled
against the testcases pytest reported for it, so editing or adding a test file passes with its new
tests in the receipt (review P2-c, 2026-11-13); such a file that reports no testcase, or only a
collection error, still fails closed with `tests-deselected: completeness cannot be established`.
For that file a plugin that deselects a test under `-p no:terminal` is not seen (no collected list
vouches for it until the next refresh). Each of the other cases makes the run INCOMPLETE (exit 3). A runner-unavailable instance leaves the other runners running instead of
exiting 2 before any run, as the existing provider adapter does (STR-V0-8705). A plugin that
deselects a test during collection too hides it from the inventory as well (NOT_PRODUCED: only an
injected collection plugin could see it).

**Scenario: Nothing collected**

- GIVEN the selected file no longer contains tests
- WHEN `corvint test` runs
- THEN the selected tests are `NOT_RUN`, the run is `INCOMPLETE` with reason
  `no-tests-collected`, and it is not PASS

**Scenario: Plugin deselects a selected test**

- GIVEN a plugin hook that deselects one of the selected tests
- WHEN `corvint test` runs
- THEN that test is `NOT_RUN` and the run is `INCOMPLETE`

- `PYR-V1-007`: (accepted automatically under G14, decision 0596; V1-5456) Advisory command and run agree

The advisory `python -m pytest` command printed by `corvint affected` MUST name exactly the tests
that `corvint test` runs for the same plan. (AFP-V0-472..477; G23)
Delivery: partial. Both name the test files the plan selected for its Python units, and a CLI test
compares them. NOT_PRODUCED: a selected file pytest's `python_files` does not admit, or a selection
over the advisory command's file bound, is named by `corvint test` but not by the advisory command.

**Scenario: Same plan**

- GIVEN one plan
- WHEN the advisory command and the `corvint test` invocation are compared
- THEN their test lists are equal

- `PYR-V1-008`: (accepted automatically under G14, decision 0596; V1-5456) Environment recorded

The receipt MUST record the Python version, the pytest version and the hash of the lockfile in use,
when present. (G1)
Delivery: delivered. Each pytest contract invocation's `environment` records the Python and pytest
versions from pytest's session header (else the interpreter version the discovery probe read, else
`NOT_OBSERVED`) and the nearest pytest lockfile (uv.lock, poetry.lock, Pipfile.lock, pdm.lock) with
its SHA-256. A full-command run (a fallback or the kill switch) records no environment.

**Scenario: Lockfile present**

- GIVEN a `uv.lock` at the root
- WHEN the run ends
- THEN the receipt carries its SHA-256 and both versions

- `PYR-V1-009`: (accepted automatically under G14, decision 0596; V1-5456) Kill switch and fallbacks

The provider MUST honour `CORVINT_SELECTION=full` and `full:pytest` (grammar owned by NMS-V1-008,
owner decision R13 (2026-11-09)) on every pytest instance, and MUST print every full-suite fallback
with its reason. (G10, G1)
Delivery: delivered for `corvint test`, through the one kill-switch parser
(`internal/testrunner/killswitch`, RNC-V1-008): `full` plans FULL with reason `kill-switch`,
`full:pytest` runs every pytest instance's full command with that reason while other runners stay
narrowed, printed as `corvint-test: KILL-SWITCH ...`, and an unknown value exits 2 with
`kill-switch-value-unknown`. An instance that runs its full command for another reason names it in
the receipt.

**Scenario: Per-runner switch**

- GIVEN `CORVINT_SELECTION=full:pytest`
- WHEN `corvint test` runs a narrowed plan
- THEN pytest runs its full suite with reason `kill-switch` and other runners stay narrowed

- `PYR-V1-010`: (accepted automatically under G14, decision 0596; V1-5456) Harness adapter

The provider MUST expose the "run named tests at a commit, return per-test outcomes" call used by
FIH-V1-008. (G3, G23)
Delivery: delivered. `tools/fault-harness` has a pytest adapter that runs named node IDs at the
checked-out commit through the contract and returns one outcome per test; its dependent units are
every collected test file. NOT_PRODUCED: mapping the plan's `python:` units to pytest entries in the
harness's plan step.

**Scenario: Harness call**

- GIVEN a fault case naming three node IDs
- WHEN the harness calls the adapter
- THEN it receives three per-test outcomes

## Surfaces

Delivered by ticket V1-5456 (build-logs `docs/build-log/2026-11-12-pytest-provider.md` and
`docs/build-log/2026-11-13-jest-pytest-on-the-contract-path.md`):

- `internal/testrunner/contract` (PYR-V1-003, PYR-V1-004, PYR-V1-006): the pytest runner asks
  pytest for its own JUnit XML report (`--rootdir=. --junitxml=REPORT -o junit_family=xunit1 -o
  junit_duration_report=call`), which needs no plugin, and reads it back to canonical IDs: file,
  then the classname parts after the file's dotted module name, then the name with its parameter
  text. A collector testcase (empty classname) is a file-level error or skip. A report without a
  file attribute or with a renamed classname is refused. `Result.runnerOutcome` keeps pytest's own
  word. `PytestExitReason`, `PytestHeader`, `PytestVersion` and `PytestDeselected` read exit
  statuses and output lines. The pytest-json-report reader stays for documents that use it.
- `internal/sessiontest` (PYR-V1-002 to PYR-V1-004, PYR-V1-006, PYR-V1-008, PYR-V1-009): pytest
  runs on main's contract adapter (RNC-V1-019); this change adds the pytest exit reasons, the
  deselected count and the completeness check, the runner-unavailable reason, and the receipt
  invocation's `environment`. The change's first `runner` adapter was replaced by the contract adapter when
  the branch merged (2026-11-13).
- `internal/testinventory` (PYR-V1-001, PYR-V1-005): `pytest-collect` version 2 adds `kind` and
  `kindSource` (`runner`, `default`, `conflict`, `unresolved`) to each pytest test record.
  `NearestLockfile` is exported for the environment record.
- `tools/fault-harness` (PYR-V1-010): adapter `pytest` and flag `--pytest`; a named node's ID is
  read with the node-ID parser, so a parameter id holding `::` stays one segment.
- NOT_PRODUCED: the tier label in watch; markers from the owner's kind declaration; a live pytest
  run and `corvint test` on pydantic/pydantic at the PRR-V1 pin; the NMS-V1 and FIH-V1 reports for
  pytest; the harness's mapping of `python:` plan units.

## Traceability

| Requirement | Evidence |
|---|---|
| PYR-V1-001 | `TestPYRV1001PyprojectRootIsListed`, `TestPYRV1001TwoRootdirsAreTwoInstances`, `TestTIVV1004PytestLister` (`internal/testinventory`); `TestRNCV1002AffectedRunsNoListerInAPytestProject` (`cmd/corvint`) |
| PYR-V1-002 | `TestPYRV1002PytestFilesRunAtTierFile` (`internal/sessiontest`); `TestPYRV1003CLIRunsPytestSelection` (`cmd/corvint`); watch NOT_PRODUCED |
| PYR-V1-003 | `TestPYRV1002PytestFilesRunAtTierFile`, `TestPYRV1003PytestOnlyPlanIsNarrowed`, `TestPYRV1003MixedPlanRunsGoAndPytest` (`internal/sessiontest`); `TestRNCV1003PytestSelection` (`internal/testrunner/contract`); `TestPYRV1003CLIRunsPytestSelection` (`cmd/corvint`) |
| PYR-V1-004 | `TestPYRV1004PytestJUnitReport`, `TestPYRV1004PytestCrossModuleTestcases`, `TestPTDV1002PytestJUnitTimes` (`internal/testrunner/contract`); `TestPYRV1003MixedPlanRunsGoAndPytest`, `TestPYRV1004FailingTestFailsTheRun`, `TestPTDV1002PytestReceiptDurations` (`internal/sessiontest`) |
| PYR-V1-005 | `TestPYRV1005KindLabels` (`internal/testinventory`); the owner's kind declaration NOT_PRODUCED |
| PYR-V1-006 | `TestPYRV1006RunnerTroubleFailsClosed` (plugin deselect under `-p no:terminal`, a changed file that reports no testcase or fails to collect), `TestPYRV1006EditedFileRunsWholeAndPasses`, `TestRNCV1006OmittedInventoriedTestIsNotRun` (`internal/sessiontest`); `TestPYRV1006PytestJUnitRefusals`, `TestPYRV1006XdistMergedReport`, `TestPYRV1006PytestOutputSignals` (`internal/testrunner/contract`); `TestPYRV1003CLIRunsPytestSelection` (`cmd/corvint`, a changed test file with a new test passes) |
| PYR-V1-007 | `TestPYRV1007AdvisoryCommandNamesTheRunTests` (`cmd/corvint`) |
| PYR-V1-008 | `TestPYRV1003MixedPlanRunsGoAndPytest`, `TestPYRV1004FailingTestFailsTheRun` (`internal/sessiontest`); `TestPYRV1006PytestOutputSignals` (`internal/testrunner/contract`); `TestPYRV1003CLIRunsPytestSelection` (`cmd/corvint`) |
| PYR-V1-009 | `TestPYRV1009KillSwitch`, `TestPYRV1009FallbacksNameTheirReason` (`internal/sessiontest`); `TestPYRV1009CLIKillSwitch` (`cmd/corvint`) |
| PYR-V1-010 | `TestPYRV1010PytestHarnessAdapter`, `TestPYRV1010PytestHarnessFailedMeasurements`, `TestPYRV1010PytestHarnessParameterWithSeparator`, `TestAdapterContract_FIH_V1_008` (`tools/fault-harness`) |

## Non-goals

A unittest runner beyond what pytest itself collects; any change to Python static selection rules;
line-level narrowing of pytest-playwright tests (G2); reviving pymutate (G26); a Python line
collector in 1.0 (owner decision R9).

## Rollback

Revert the change's commits; pytest selections return to each instance's full command. The kill
switch (PYR-V1-009) restores the full pytest suite without a revert. PYR-V1-001 to PYR-V1-010 stay
listed as retired if the change is withdrawn and are never reused.
