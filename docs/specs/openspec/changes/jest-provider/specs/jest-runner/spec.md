# Spec delta: jest-runner

Capability spec: `docs/specs/jest-runner-v1.md` (new). This delta is a proposal only; the canonical
spec is updated in the same change (G14).

## ADDED Requirements

### Requirement: JST-V1-001 Jest is a built-in runner
Source: owner decisions R7 and R14 (2026-11-09); G23 (owner answer).
Status: proposed 2026-11-09; delivery not-started.
`corvint` MUST run Jest under the compiled-in runner contract (RNC-V1-001). Jest support MUST NOT
need a provider executable or a `providers.jest` configuration entry.

#### Scenario: Jest project with no provider
- **GIVEN** a repository with `jest.config.js` and no provider configuration
- **WHEN** `corvint test` runs a selection that includes Jest tests
- **THEN** Corvint launches Jest itself and records per-test outcomes

### Requirement: JST-V1-002 Jest discovery into the inventory
Source: owner decisions R10 and R11 (2026-11-09).
Status: proposed 2026-11-09; delivery not-started.
`corvint inventory refresh` MUST list Jest test files with `jest --listTests` and test names with
Jest's own collection, record the lister as executing (TIV-V1-004) and name each test by canonical
test ID (CTI-V1-004).

#### Scenario: nested describe blocks
- **GIVEN** `src/a.test.ts` with `describe("cart")` containing `it("adds item")`
- **WHEN** the inventory is refreshed
- **THEN** it lists `jest:src/a.test.ts::cart::adds item`

### Requirement: JST-V1-003 Selection arguments and exact
Source: owner decision R16 (2026-11-09).
Status: proposed 2026-11-09; delivery not-started.
For selected Jest tests, Corvint MUST pass the selected files with `--runTestsByPath` and one `-t`
pattern, anchored at both ends, matching the selected full names with every regular expression
character escaped. `exact` MUST be true only when no unselected test in those files has a selected
full name.

#### Scenario: name with regular expression characters
- **GIVEN** a selected test named `handles (a+b)`
- **WHEN** `corvint test` builds the Jest command
- **THEN** the `-t` pattern matches that name literally and no other test

### Requirement: JST-V1-004 Results from Jest's JSON report
Source: owner decisions R7 and R19 (2026-11-09).
Status: proposed 2026-11-09; delivery not-started.
Corvint MUST write Jest's JSON report to a private path and MUST map each assertion result to its
canonical test ID from `ancestorTitles` and `title`, with its outcome and its `duration`
(PTD-V1-002).

#### Scenario: passing test with duration
- **GIVEN** a report where `cart > adds item` passed in 14 ms
- **WHEN** the receipt is written
- **THEN** the outcome for `jest:src/a.test.ts::cart::adds item` is passed with `durationMs` 14

### Requirement: JST-V1-005 Missing results are NOT_RUN
Source: every unknown runs with a named reason; PROD-TEST-020.
Status: proposed 2026-11-09; delivery not-started.
A selected Jest test absent from the report MUST be recorded NOT_RUN and the run MUST be INCOMPLETE.
A missing or unparsable report MUST make the run INCOMPLETE and MUST keep Jest's exit code.

#### Scenario: report missing
- **GIVEN** Jest exits before writing its report
- **WHEN** Corvint reads results
- **THEN** the run is INCOMPLETE and the exit code is Jest's

### Requirement: JST-V1-006 Default command
Source: G23 (owner answer).
Status: proposed 2026-11-09; delivery not-started.
With no declared command, the Jest command MUST be `npx jest` run from the instance root
(RNC-V1-013).

#### Scenario: no declared command
- **GIVEN** a Jest instance at `apps/web` with no declared command
- **WHEN** it runs
- **THEN** Corvint runs `npx jest` with `apps/web` as the working directory

### Requirement: JST-V1-007 Instances and full:jest
Source: owner decisions R10 and R13 (2026-11-09).
Status: proposed 2026-11-09; delivery not-started.
Each Jest root and config MUST be a separate runner instance (TIV-V1-003). `CORVINT_SELECTION=full:jest`
MUST run every Jest instance in full.

#### Scenario: two roots
- **GIVEN** Jest instances at `apps/a` and `apps/b`
- **WHEN** `CORVINT_SELECTION=full:jest` is set
- **THEN** both instances run in full

### Requirement: JST-V1-008 Unsupported setups run in full
Source: every unknown runs with a named reason.
Status: proposed 2026-11-09; delivery not-started.
A Jest version below the documented minimum, a config Corvint cannot read, or a configured runner or
reporter that prevents the JSON report MUST run the instance's full command with reason
`jest-unsupported-config`.

#### Scenario: custom test runner
- **GIVEN** a Jest config that sets a custom `testRunner` without JSON results
- **WHEN** a plan selects tests in that instance
- **THEN** the instance runs in full with `jest-unsupported-config`

### Requirement: JST-V1-009 End-to-end check
Source: G23 (owner answer, open item on the Jest path through `corvint test`); RNC-V1-010.
Status: proposed 2026-11-09; delivery not-started.
A Jest fixture with two roots MUST be planned and run through `corvint affected`, `corvint test`,
`corvint watch` and the MCP affected tool, and every selected test in each receipt MUST match the
inventory. This result is the evidence for RNC-V1-010 (Jest proven end to end).

#### Scenario: one-line change
- **GIVEN** the fixture and a change to one source file used by one Jest test
- **WHEN** the four consumers plan and run it
- **THEN** each selects the same tests, and the receipt lists them with outcomes and durations

### Requirement: JST-V1-010 No Jest provider binary ships
Source: owner decision R14 (2026-11-09).
Status: proposed 2026-11-09; delivery not-started.
The 1.0 release binary list MUST contain no Jest provider binary.

#### Scenario: release list
- **GIVEN** the release binary list
- **WHEN** the release check runs
- **THEN** no entry names a Jest provider

## MODIFIED Requirements

None. The provider refusal of runners other than Playwright and Vitest in
`internal/sessiontest/declared.go` (integ 9ec3a29d0) is STR-V0-8701 behaviour for configured
providers; it is unchanged, because Jest does not use a provider.

## REMOVED Requirements

None.
