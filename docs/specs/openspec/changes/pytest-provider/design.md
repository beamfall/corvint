# Design: pytest-provider

Status: proposed (2026-11-09). Requirement text lives in `specs/pytest-provider/spec.md`.

## Decisions

1. pytest selects by file with tier `file`, labelled in the plan, the receipt and watch (owner
   decision R8 (2026-11-09)). Python file-level is recorded as a release exception and the Python
   line collector is a 1.0.x ticket (owner decision R9 (2026-11-09)). Recorded risk: file-level
   selection may not meet the G4 narrowing bar for pytest on the held-out repo; the NMS-V1 report
   shows it either way.
2. The pytest lister is `executing` and runs only under `corvint inventory refresh` (owner decision
   R10 (2026-11-09)).
3. One runner instance per configuration root (rootdir) (owner decision R15 (2026-11-09)).
4. Per-test outcomes feed PROD-TEST-020 (owner decision R7 (2026-11-09)); durations come from change
   `per-test-durations`; entries use the canonical test ID (owner decision R11 (2026-11-09)).
5. Kill switch through NMS-V1-008 (owner decision R13 (2026-11-09)).
6. Missing results are `NOT_RUN` and the run `INCOMPLETE`, not `FAILED_MEASUREMENT` (review T-m2).
7. Integration kind from the pytest marker `integration` or the owner's kind declaration (review
   T-m4).

## Failure modes

- No Python interpreter or no pytest importable: refuse with `runner-unavailable`, exit 2, before
  running anything; the plan is still printed.
- pytest collects zero of the selected tests: the selected tests are `NOT_RUN` and the run is
  `INCOMPLETE`, never PASS (review T-m2 replaces the earlier `FAILED_MEASUREMENT`).
- pytest exits with an internal or usage error: the receipt records the exit code and the run is
  not PASS. pytest's documented exit codes are 0 to 5
  (`https://docs.pytest.org/en/stable/reference/exit-codes.html`; not re-checked on 2026-11-09,
  NOT_VERIFIED).
- `capture.py` output is oversized or has unknown members: `python-artifact-size` or
  `python-artifact-invalid` (`python_plugin.go:61-70`); the run fails closed.
- An unresolved conftest plugin frontier: the plan is UNKNOWN for Python and pytest runs in full.
- pytest-xdist is active: `capture.py` hooks run in the controller and in each worker; outcomes are
  merged by node ID, and a selected test reported by no process is `NOT_RUN` (review T-m5).
- A plugin reorders or deselects collected items (for example through
  `pytest_collection_modifyitems` or `-p` plugins): order is not relied on; a deselected selected
  test is `NOT_RUN` and the run `INCOMPLETE` (review T-m5).

## Acceptance evidence

- A pytest fixture repository with pyproject configuration, conftest fixtures, a marker-declared
  integration test and a pytest-playwright test; one Go test per requirement.
- `corvint test` on pydantic/pydantic at the PRR-V1 pin, receipt retained.
- NMS-V1 and FIH-V1 reports for pytest.
- Tests for the xdist and deselect failure modes.

## Rollback

Revert the change's commits; pytest selections return to advice only. Requirement IDs are retired,
not reused. The kill switch covers a bad selection in the field without a revert.

## Open questions for the owner

None.
