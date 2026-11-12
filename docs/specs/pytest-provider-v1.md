# pytest provider V1

Owner: Russell Lewis
Date: 2026-11-12
Requirement prefix: `PYR-V1`
Intent status: proposed
Delivery status: not-started
Authoritative inputs: change folder `docs/specs/openspec/changes/pytest-provider/` (`proposal.md`,
`design.md`, `tasks.md` and `specs/pytest-provider/spec.md`), `docs/specs/PRODUCT-PLAN-1.0.md`
(PROD-TEST-006, PROD-TEST-008, PROD-TEST-011), `docs/specs/runner-contract-v1.md` (RNC-V1) and
`docs/decisions/0596-accept-product-plan-1-0-2026-11-10.md` (owner decisions R7 to R11, R13, R15
and R19).

## Agent digest
- Claim: `corvint test` runs the pytest files a plan selects through the compiled-in runner contract, one run per rootdir at tier `file`, with per-test results.
- Status: proposed / not-started. Change `pytest-provider` follows `runner-contract`, `canonical-test-id` and `test-inventory` in the R38 testing order.
- Exists: the change folder; ticket V1-5456 delivers it.
- Blocked on: nothing in this tree; `run-evidence` and `per-test-durations` are not built, so their records are cited, not used.
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

- `PYR-V1-001`: (proposed 2026-11-09; delivery not-started) Discovery

The provider MUST supply the pytest lister, marked `executing`, to `corvint inventory refresh`
(change `test-inventory`, owner decision R10 (2026-11-09)). The lister MUST inventory pytest node
IDs for each configuration root through pytest's own collection, using the configuration search
order pytest.ini, .pytest.ini, pyproject.toml, tox.ini, setup.cfg, and MUST key each entry by the
canonical test ID (RNC-V1-012). `corvint affected` and `corvint watch` MUST NOT run it. Each
configuration root (rootdir) is one runner instance (RNC-V1-014, owner decision R15 (2026-11-09)).
(G1, G23)

**Scenario: pyproject root**

- GIVEN a repository whose pytest configuration is in `pyproject.toml`
- WHEN `corvint inventory refresh` runs
- THEN every collected node ID is listed with its file and canonical test ID

**Scenario: Two rootdirs**

- GIVEN pytest configurations in `services/a/pyproject.toml` and `services/b/pytest.ini`
- WHEN `corvint inventory refresh` runs
- THEN the inventory holds two pytest instances, one per root

- `PYR-V1-002`: (proposed 2026-11-09; delivery not-started) Selection granularity

The provider MUST run the files the plan selected, with tier `file` and an explicit tier label in
the plan, the receipt and watch (owner decision R8 (2026-11-09)). Python file-level selection is
recorded as a release exception; the Python line collector is a 1.0.x ticket (owner decision R9
(2026-11-09)). The provider MUST narrow below file level only when the line evidence for pytest
has passed the FIH-V1 proof (change `line-evidence`). Every selection MUST carry a reason.
(answer 3, G2)

**Scenario: Line evidence unproven**

- GIVEN no passed FIH-V1 verdict for pytest line evidence
- WHEN a plan selects a test function
- THEN its whole file runs and the selection carries tier `file`

- `PYR-V1-003`: (proposed 2026-11-09; delivery not-started) Run through `corvint test`

`corvint test` MUST run the plan's pytest selections through the compiled-in runner contract, one
invocation per configuration root, and MUST NOT require an external provider executable. (G23)

**Scenario: Mixed plan**

- GIVEN a plan with Go and pytest selections
- WHEN `corvint test` runs it
- THEN both runners execute and one receipt records both

- `PYR-V1-004`: (proposed 2026-11-09; delivery not-started) Per-test results

The receipt MUST record, per canonical test ID, the outcome (passed, failed, skipped, error,
xfailed, xpassed) and the duration of the call phase, as additive members of `corvint-test-run/0`.
Outcomes feed the run result record PROD-TEST-020 (change `run-evidence`, owner decision R7
(2026-11-09)); durations follow change `per-test-durations` (owner decision R19 (2026-11-09)).
(G4, G23)

**Scenario: Failing test**

- GIVEN one selected test fails
- WHEN the run ends
- THEN the receipt names that test's canonical ID as failed with its duration and the run
  result is FAIL

- `PYR-V1-005`: (proposed 2026-11-09; delivery not-started) Kind label

Each pytest test MUST carry `kind` and `kindSource`: e2e when it uses pytest-playwright fixtures,
integration when it carries the pytest marker `integration` (or a marker named in the owner's
kind declaration of change `test-inventory`), unit otherwise, UNKNOWN on conflict. The label MUST
NOT change selection in 1.0. (G13, review T-m4)

**Scenario: Playwright fixture**

- GIVEN a test taking the `page` fixture from pytest-playwright
- WHEN inventory runs
- THEN its kind is e2e with `kindSource` runner

**Scenario: Integration marker**

- GIVEN a test decorated with `@pytest.mark.integration`
- WHEN inventory runs
- THEN its kind is integration

- `PYR-V1-006`: (proposed 2026-11-09; delivery not-started) Fail closed on runner trouble

A missing interpreter or pytest, zero collected selections, an internal or usage error, or an
invalid capture MUST produce a non-PASS result with a typed reason. Selected tests absent from the
results are `NOT_RUN` and the run is `INCOMPLETE` (RNC-V1-006, review T-m2). (G1, G10)

**Scenario: Nothing collected**

- GIVEN the selected file no longer contains tests
- WHEN `corvint test` runs
- THEN the selected tests are `NOT_RUN`, the run is `INCOMPLETE` with reason
  `no-tests-collected`, and it is not PASS

**Scenario: Plugin deselects a selected test**

- GIVEN a plugin hook that deselects one of the selected tests
- WHEN `corvint test` runs
- THEN that test is `NOT_RUN` and the run is `INCOMPLETE`

- `PYR-V1-007`: (proposed 2026-11-09; delivery not-started) Advisory command and run agree

The advisory `python -m pytest` command printed by `corvint affected` MUST name exactly the tests
that `corvint test` runs for the same plan. (AFP-V0-472..477; G23)

**Scenario: Same plan**

- GIVEN one plan
- WHEN the advisory command and the `corvint test` invocation are compared
- THEN their test lists are equal

- `PYR-V1-008`: (proposed 2026-11-09; delivery not-started) Environment recorded

The receipt MUST record the Python version, the pytest version and the hash of the lockfile in use,
when present. (G1)

**Scenario: Lockfile present**

- GIVEN a `uv.lock` at the root
- WHEN the run ends
- THEN the receipt carries its SHA-256 and both versions

- `PYR-V1-009`: (proposed 2026-11-09; delivery not-started) Kill switch and fallbacks

The provider MUST honour `CORVINT_SELECTION=full` and `full:pytest` (grammar owned by NMS-V1-008,
owner decision R13 (2026-11-09)) on every pytest instance, and MUST print every full-suite fallback
with its reason. (G10, G1)

**Scenario: Per-runner switch**

- GIVEN `CORVINT_SELECTION=full:pytest`
- WHEN `corvint test` runs a narrowed plan
- THEN pytest runs its full suite with reason `kill-switch` and other runners stay narrowed

- `PYR-V1-010`: (proposed 2026-11-09; delivery not-started) Harness adapter

The provider MUST expose the "run named tests at a commit, return per-test outcomes" call used by
FIH-V1-008. (G3, G23)

**Scenario: Harness call**

- GIVEN a fault case naming three node IDs
- WHEN the harness calls the adapter
- THEN it receives three per-test outcomes

## Surfaces

Not yet built.

## Traceability

| Requirement | Evidence |
|---|---|
| PYR-V1-001 to PYR-V1-010 | NOT_PRODUCED: not yet built |

## Non-goals

A unittest runner beyond what pytest itself collects; any change to Python static selection rules;
line-level narrowing of pytest-playwright tests (G2); reviving pymutate (G26); a Python line
collector in 1.0 (owner decision R9).

## Rollback

Revert the change's commits; pytest selections return to each instance's full command. The kill
switch (PYR-V1-009) restores the full pytest suite without a revert. PYR-V1-001 to PYR-V1-010 stay
listed as retired if the change is withdrawn and are never reused.
