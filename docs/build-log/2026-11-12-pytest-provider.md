# 2026-11-12: pytest provider (V1-5456)

Change `pytest-provider`: tasks 2 to 8 are built and task 9's tests and this entry are written;
task 1 (ticket, prefix, `requirementRefs`) and the Tasks-gate completion belong to the
orchestrator. Requirements PYR-V1-001 to PYR-V1-010 are in `docs/specs/pytest-provider-v1.md`,
where each Delivery line names what is built and what is NOT_PRODUCED. The change builds on
`runner-contract` (RNC-V1), `canonical-test-id` (CTI-V1), `test-inventory` (TIV-V1) and
`fault-injection-harness` (FIH-V1) at the batch head `00267cfb9`. It does not import
`internal/testrunner/registry`, `internal/testevidence` or any provider protocol package, which G23
removes.

## What changed

- `corvint test` now runs the plan's pytest files through the compiled-in runner contract. A new
  `runner` adapter runs one pytest invocation per rootdir, from the rootdir, at tier `file`. The
  runner unit's file set is the advisory command's file set, computed by one function. The receipt
  adds per-test results (canonical ID, outcome, pytest's own outcome word, call-phase duration),
  typed reasons and an environment record.
- The contract's pytest runner uses pytest's built-in JUnit XML report instead of the
  pytest-json-report plugin.
- The inventory's pytest lister (version 2) labels each test's kind.
- `internal/selectionswitch` reads `CORVINT_SELECTION`.
- `tools/fault-harness` has a pytest adapter.

## Differences from the proposal, followed by intent

1. **JUnit XML, not `capture.py` or pytest-json-report.** The proposal named the embedded
   `registry/python/capture.py` and the contract used pytest-json-report. `capture.py` lives in
   `internal/testrunner/registry` beside the provider protocol types (`internal/testevidence`), which
   G23 removes; loading it would also need a file on `PYTHONPATH`. pytest-json-report is a
   third-party plugin, and pytest exits 4 on its flags where the plugin is absent. pytest's own
   `--junitxml` with `-o junit_family=xunit1` carries the file of every testcase, and pytest builds
   classname and name from the node ID (`junitxml.mangle_test_address`), so the canonical ID is
   recovered exactly. `-o junit_duration_report=call` gives the call-phase duration PYR-V1-004 asks
   for. One loss: a non-strict XPASS is written as a pass, so `runnerOutcome` reads `passed` for it
   (NOT_VERIFIED). `TestRNCV1003PytestSelection` now expects the JUnit arguments. The
   pytest-json-report reader stays, so `TestRNCV1005PytestReport` is unchanged.
2. **runner-unavailable leaves the other runners running.** Design failure mode 1 said "exit 2
   before running anything". The requirement (PYR-V1-006) asks for a non-PASS result with a typed
   reason. The runner adapter follows the provider adapter (STR-V0-8705): the instance's files are
   NOT_RUN with `runner-unavailable`, the run is INCOMPLETE (exit 3), and Go and other runners still
   run. An exit 2 would discard their results.
3. **Deselection is detected from pytest's printed count.** JUnit XML does not list deselected
   items. On a narrowed run, the `N deselected` count pytest prints (collection line or summary)
   makes the run INCOMPLETE with `tests-deselected`. At tier `file` the selection names files, not
   tests, so the deselected test is not named. A run whose terminal reporter is disabled (`-p
   no:terminal`) prints no count; that case is not detected.
4. **Kinds come from each test's source, not from collection.** `--collect-only -q` prints node IDs
   only. The lister scans each test file for the function's parameters, `usefixtures`, its
   decorators, and class and module `pytestmark`. e2e needs a pytest-playwright fixture plus
   evidence the plugin is in use: a dependency manifest naming `pytest-playwright` in the rootdir or
   repository root, or the file mentioning `playwright`. Otherwise a `page` fixture is the
   project's own. A conftest, autouse or inherited fixture is not seen, so such a test reads unit.
   The owner's kind declaration does not exist in this tree (NOT_PRODUCED). The lister version
   moved from 1 to 2, so an inventory listed before this change reads `inventory-stale` for pytest.
5. **The kill-switch parser is new, with the NMS-V1-008 grammar exactly.** No `CORVINT_SELECTION`
   parser existed in this tree (no `internal/selectionswitch`, no planner parser). The package
   implements only the two forms NMS-V1-008 defines and its `kill-switch-value-unknown` refusal, and
   names NMS-V1-008 as the owner, so change `narrowing-measurement` reuses it rather than adding a
   second grammar. In `corvint test`, `full` plans FULL; `full:go` runs every package of the Go
   universe whole, and any other named runner runs its instances' full commands, each unit with
   reason code `kill-switch`, while the runners not named stay narrowed. A forced Vitest or
   Playwright skips its provider and runs its full command.
6. **Where pytest still runs its full command.** A pytest selection keeps its language full command,
   printed as `corvint-test: FULL <unit>: runner fallback: ...` with reason code `runner-fallback`,
   in three cases: the local declaration names a `python` full command, no declared instance owns
   the file, or another selection or frontier already widened Python. The `ci` profile runs only Go
   (STR-V0-012) and is unchanged. A FULL plan runs the instance full commands, not the contract, so
   it has no per-test results.

## Assertions extended

- `TestSTRV0004AdapterCapabilityManifest` pins the new `runner` adapter under
  `corvint-test-adapter/0`, as STR-V0-8701 did for `provider`.
- `TestRNCV1003PytestSelection` expects the JUnit report arguments (difference 1).
- `TestTIVV1004PytestLister` expects each test's kind. Its `TestK` is not in the fixture source, so it
  is `UNKNOWN`/`unresolved`.
- `TestAdapterContract_FIH_V1_008` no longer expects pytest to be refused.

## Evidence

Focused tests, each run alone through `script/heavy.sh`: `internal/testrunner/contract`,
`internal/sessiontest`, `internal/testinventory`, `internal/selectionswitch`,
`tools/fault-harness` (all pass), and `cmd/corvint` with `-run` over the `corvint test`, advisory,
inventory, runner-contract and PYR-V1 tests (pass). Fixtures are hand-written from pytest's
`src/_pytest/junitxml.py` and xdist's crash report. Each fixture cites its source.

## Not run

- A live pytest run. pytest is not installed on this host; stubs answer at its path (NOT_RUN).
- `corvint test` on pydantic/pydantic at the PRR-V1 pin, and the NMS-V1 and FIH-V1 reports for
  pytest (NOT_RUN: no network, no pytest, no NMS-V1 build).
- `openspec validate` (the CLI is not installed).
- `make doc-gates`, dogfood-check and seal (decision 0594, lane rules).

## Rollback

Revert the change's commits. pytest selections then run each instance's full command again, the
inventory's pytest lister returns to version 1, and `CORVINT_SELECTION` is ignored by `corvint
test`. PYR-V1-001 to PYR-V1-010 stay listed as retired if the change is withdrawn.
