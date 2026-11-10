# Spec delta: hub-read-scopes

The lane copies each statement below into `affected-plan-selection-v0.md`. Each one becomes a
numbered definition line that begins `(proposed 2026-11-10; delivery not-started; V1-5201)`.

## ADDED Requirements

### Requirement: AFP-V0-107 Whole-repository tests live in their own package

A test that reads the whole repository (it lists the repository root, reads Git metadata, or
builds a graph over every package) MUST live in a test package that holds only such tests when
its former package declares a read scope. That package MUST be listed in
`.corvint/unbounded-readers.json` with its reason. (PROD-TEST-001)

#### Scenario: A moved whole-repository test

- **GIVEN** `TestGoImportsDifferentialAgainstScanner` lives in a sibling package of
  `internal/contextindex`
- **WHEN** any path changes
- **THEN** the sibling package is selected as an unbounded reader

### Requirement: AFP-V0-108 A hub scope needs a passing confined run

A read scope for a package that leaves the unbounded set MUST be declared only after that
package's tests pass under the Landlock wrapper with exactly that scope. The build log MUST name
the run. (PROD-TEST-002)

#### Scenario: The confined run fails

- **GIVEN** a hub whose tests read a path outside the candidate scope
- **WHEN** the confined run fails
- **THEN** the scope is not declared and the hub stays in the unbounded set

### Requirement: AFP-V0-109 No saving is claimed for an unproven hub

A package without a passing confined run MUST stay selected on every change, and the build log
MUST NOT count it as freed. (PROD-TEST-002)

#### Scenario: Stage 2 is blocked

- **GIVEN** a `cmd/corvint` test that cannot move out of the package
- **WHEN** the lane closes
- **THEN** `cmd/corvint` is still listed as an unbounded reader and its blocker is recorded

### Requirement: AFP-V0-110 Docs-only selection is measured on a fixed diff

The change MUST record the plan for the diff of pull request 178 from base `2913fdc7e` before and
after it, with the selected Go packages and their test counts. A scoped hub MUST be absent from
the plan after the change. (PROD-TEST-001)

#### Scenario: Stage 1 measured

- **GIVEN** `internal/contextindex` declares a read scope that excludes `AGENTS.md`
- **WHEN** the fixed diff is planned
- **THEN** the plan does not select `internal/contextindex`
