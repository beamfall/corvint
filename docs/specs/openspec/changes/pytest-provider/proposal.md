# Change proposal: pytest discovery, selection, run and result reporting through `corvint test`

Change name: `pytest-provider`
Ticket: none filed yet (task 1 files it in `.taskman/`, milestone `v1-0`)
Product requirement served: PROD-TEST-006, PROD-TEST-008 and PROD-TEST-011 in
`docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: `docs/specs/pytest-provider-v1.md`, a new spec with its own prefix `PYR-V1`;
selection rules stay in `docs/specs/affected-plan-python-playwright-v0.md`
Requirement IDs this change adds: PYR-V1-001 to PYR-V1-010
Owner answers this change implements: answer 3, G1, G2, G4, G10, G13, G23, G26
(`owner-answers-2026-11-09.md`), and owner decisions R7, R8, R9, R10, R11, R13, R15 and R19
(2026-11-09)
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. No lane has been started for this change.
Layout: this folder follows G14 (`proposal.md`, `design.md`, `tasks.md`,
`specs/pytest-provider/spec.md`).

## Why

pytest is one of the seven runners that block 1.0 (G1) and one of the four with a narrowing bar
(G4). Today `corvint affected` can select pytest files, but `corvint test` cannot run them, and the
compiled-in pytest runner code has no caller. G23 asks for one compiled-in runner contract for all
seven runners, keeping `registry.RunnerPlugin`.

### What exists today (verified in `integ` at 9ec3a29d0)

- Selection. `internal/liveverify/affected/python/` builds the static Python graph (conftest
  loading at `python.go:133`; an unresolved `pytest_plugins` assignment raises the frontier
  `python:pytest-plugins-unresolved`, `python.go:34-37`). `cmd/corvint/affected.go:1856` calls
  `advisoryPytestCheck` (`cmd/corvint/affected_pytest.go:79`, AFP-V0-472..477), which prints an
  advisory `python -m pytest` command for the selected files and finds configuration in pytest's
  search order (`affected_pytest.go:48`). The spec is `affected-plan-python-playwright-v0.md`.
- Run. `corvint test` admits one provider executable, `corvint-js-test-provider`, with runners
  `playwright` and `vitest` only (`internal/sessiontest/declared.go:40-55`, STR-V0-8701 in the
  `integ` copy of `session-test-runner-v0.md`). No pytest path exists.
- Compiled-in runner with no caller. `internal/testrunner/registry/python_plugin.go:46`
  (`PythonPlugin`) accepts `pytest` and `unittest` and embeds `python/capture.py` (`:22`), which
  collects through `pytest_collection_finish` and records outcomes through
  `pytest_runtest_logreport` (`capture.py:123-158`). It reports "unqualified" for qualification
  (`python_plugin.go:57`). A repository-wide search finds no non-test caller of `PythonPlugin`.
  `registry` is imported in production only by `tools/runner-reference`,
  `internal/testrunner/golang` (itself unimported) and `internal/testrunner/swiftselection`.
- Receipts. `corvint test` writes `corvint-test-run/0` (`internal/sessiontest/manifest.go:28`) with
  elapsed time per invocation only (`internal/sessiontest/run.go:85`).
- Mutation. `internal/liveverify/pymutate` (FPK-V0-019) has no importers outside itself and is
  removed by G26.

### What is missing

- `corvint test` cannot run pytest; the selected pytest files are advice only.
- No per-test outcome or duration reaches a `corvint test` receipt for pytest.
- No test-kind label (G13).
- No fault-injection adapter (FIH-V1) and no narrowing measurement (NMS-V1) for pytest.
- No proof that line-level evidence is safe for pytest. pytest ships at file level with an explicit
  tier label (owner decision R8 (2026-11-09)); this is recorded as a release exception and the
  Python line collector is a 1.0.x ticket (owner decision R9 (2026-11-09)).

## What Changes

- Supply the pytest lister (`executing`) to `corvint inventory refresh`, one instance per rootdir.
- Wire `PythonPlugin` (or its successor under the G23 contract) into `corvint test`, so a plan's
  pytest selections run in one pytest invocation per configuration root, at tier `file`.
- Add per-test outcome, duration and kind to the run receipt as additive members, keyed by
  canonical test ID.
- Keep the advisory command and assert that it names the same tests `corvint test` runs.

## Capabilities

### New Capabilities

- `pytest-provider`: discovery, run and result reporting for pytest.

### Modified Capabilities

- `session-test-runner`: admits pytest through the compiled-in contract.
- `affected-plan-python-playwright`: unchanged selection; adds the run link.

## Non-goals

- No unittest runner in 1.0 beyond what pytest itself collects.
- No change to Python static selection rules.
- No browser E2E narrowing for pytest-playwright tests by line evidence (G2); they are labelled e2e
  and selected whole with a reason.
- No revival of pymutate (G26).
- No Python line collector in 1.0 (owner decision R9 (2026-11-09)).

## Impact

- `cmd/corvint`, `internal/sessiontest`, `internal/testrunner/registry`. The external provider
  protocol packages are removed by another change (G23); this change must not depend on them.
- G4 risk: pytest at file level may not reach the narrowing bar; NMS-V1 reports it.
