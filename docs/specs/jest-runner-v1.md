# Jest runner V1

Owner: Russell Lewis
Date: 2026-11-12
Intent status: accepted
Delivery status: not-started
Authoritative inputs: change folder `docs/specs/openspec/changes/jest-provider/` (`proposal.md`,
`design.md`, `tasks.md` and `specs/jest-runner/spec.md`), `docs/specs/PRODUCT-PLAN-1.0.md`
(PROD-TEST-001, PROD-TEST-004, PROD-TEST-020) and
`docs/decisions/0596-accept-product-plan-1-0-2026-11-10.md` (owner decisions R7, R10, R11, R13,
R14, R16 and R19).

## Agent digest
- Claim: Jest runs inside the compiled-in runner contract: named discovery, anchored name selection with an honest `exact`, per-test results, full runs for odd setups.
- Status: accepted / not-started. Change `jest-provider` auto-accepted under G14 (decision 0596); ticket V1-5455.
- Exists: nothing beyond the runner contract's Jest file selection and report reader.
- Blocked on: the narrowed `corvint test` path (runner-contract task 5).
- Read next: Requirements.

## User and boundary

A developer or agent working in a JavaScript or TypeScript repository that tests with Jest runs
`corvint test`, `corvint affected`, `corvint watch` or the MCP affected tool. Before this change
Jest was detected by the TypeScript affected analysis and the runner contract could run Jest files
by path, but `corvint test` ran a Jest project only through its declared full command, the
inventory named Jest tests only by file, and no check showed a Jest project end to end. The provider
binary `corvint-js-test-provider` had no Jest path, and owner decision R14 retires provider binaries.

This spec is the canonical home of requirements JST-V1-001 to JST-V1-010, copied from the change's
`specs/jest-runner/spec.md`. Design decisions, failure modes and rollback stay in the change's
`design.md`, which owns them; this file adds only the delivery record (Surfaces, Traceability).

## Requirements

- `JST-V1-001`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5455) Jest is a built-in runner

Source: owner decisions R7 and R14 (2026-11-09); G23 (owner answer). `corvint` MUST run Jest under
the compiled-in runner contract (RNC-V1-001). Jest support MUST NOT need a provider executable or a
`providers.jest` configuration entry.

**Scenario: Jest project with no provider**
- GIVEN a repository with `jest.config.js` and no provider configuration
- WHEN `corvint test` runs a selection that includes Jest tests
- THEN Corvint launches Jest itself and records per-test outcomes

- `JST-V1-002`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5455) Jest discovery into the inventory

Source: owner decisions R10 and R11 (2026-11-09). `corvint inventory refresh` MUST list Jest test
files with `jest --listTests` and test names with Jest's own collection, record the lister as
executing (TIV-V1-004) and name each test by canonical test ID (CTI-V1-004).

**Scenario: nested describe blocks**
- GIVEN `src/a.test.ts` with `describe("cart")` containing `it("adds item")`
- WHEN the inventory is refreshed
- THEN it lists `jest:src/a.test.ts::cart::adds item`

- `JST-V1-003`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5455) Selection arguments and exact

Source: owner decision R16 (2026-11-09). For selected Jest tests, Corvint MUST pass the selected
files with `--runTestsByPath` and one `-t` pattern, anchored at both ends, matching the selected
full names with every regular expression character escaped. `exact` MUST be true only when no
unselected test in those files has a selected full name.

**Scenario: name with regular expression characters**
- GIVEN a selected test named `handles (a+b)`
- WHEN `corvint test` builds the Jest command
- THEN the `-t` pattern matches that name literally and no other test

- `JST-V1-004`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5455) Results from Jest's JSON report

Source: owner decisions R7 and R19 (2026-11-09). Corvint MUST write Jest's JSON report to a private
path and MUST map each assertion result to its canonical test ID from `ancestorTitles` and `title`,
with its outcome and its `duration` (PTD-V1-002).

**Scenario: passing test with duration**
- GIVEN a report where `cart > adds item` passed in 14 ms
- WHEN the receipt is written
- THEN the outcome for `jest:src/a.test.ts::cart::adds item` is passed with `durationMs` 14

- `JST-V1-005`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5455) Missing results are NOT_RUN

Source: every unknown runs with a named reason; PROD-TEST-020. A selected Jest test absent from the
report MUST be recorded NOT_RUN and the run MUST be INCOMPLETE. A missing or unparsable report MUST
make the run INCOMPLETE and MUST keep Jest's exit code.

**Scenario: report missing**
- GIVEN Jest exits before writing its report
- WHEN Corvint reads results
- THEN the run is INCOMPLETE and the exit code is Jest's

- `JST-V1-006`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5455) Default command

Source: G23 (owner answer). With no declared command, the Jest command MUST be `npx jest` run from
the instance root (RNC-V1-013).

**Scenario: no declared command**
- GIVEN a Jest instance at `apps/web` with no declared command
- WHEN it runs
- THEN Corvint runs `npx jest` with `apps/web` as the working directory

- `JST-V1-007`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5455) Instances and full:jest

Source: owner decisions R10 and R13 (2026-11-09). Each Jest root and config MUST be a separate
runner instance (TIV-V1-003). `CORVINT_SELECTION=full:jest` MUST run every Jest instance in full.

**Scenario: two roots**
- GIVEN Jest instances at `apps/a` and `apps/b`
- WHEN `CORVINT_SELECTION=full:jest` is set
- THEN both instances run in full

- `JST-V1-008`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5455) Unsupported setups run in full

Source: every unknown runs with a named reason. A Jest version below the documented minimum, a
config Corvint cannot read, or a configured runner or reporter that prevents the JSON report MUST
run the instance's full command with reason `jest-unsupported-config`.

**Scenario: custom test runner**
- GIVEN a Jest config that sets a custom `testRunner` without JSON results
- WHEN a plan selects tests in that instance
- THEN the instance runs in full with `jest-unsupported-config`

- `JST-V1-009`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5455) End-to-end check

Source: G23 (owner answer, open item on the Jest path through `corvint test`); RNC-V1-010. A Jest
fixture with two roots MUST be planned and run through `corvint affected`, `corvint test`, `corvint
watch` and the MCP affected tool, and every selected test in each receipt MUST match the inventory.
This result is the evidence for RNC-V1-010 (Jest proven end to end).

**Scenario: one-line change**
- GIVEN the fixture and a change to one source file used by one Jest test
- WHEN the four consumers plan and run it
- THEN each selects the same tests, and the receipt lists them with outcomes and durations

- `JST-V1-010`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5455) No Jest provider binary ships

Source: owner decision R14 (2026-11-09). The 1.0 release binary list MUST contain no Jest provider
binary.

**Scenario: release list**
- GIVEN the release binary list
- WHEN the release check runs
- THEN no entry names a Jest provider

## Non-goals

No `corvint-jest-test-provider` binary and no Jest subcommand in `corvint-js-test-provider` (owner
decision R14); no line evidence (change `line-evidence`); no Jest-specific flow links (G13); no
support for a Jest project run through a wrapper other than the declared command, which runs the
declared full command.

## Rollback

Revert the change's commits. Jest projects then run their declared full command through `corvint
test`, and the inventory names Jest tests by file again. Receipts written with Jest canonical IDs
stay readable as data. JST-V1-001 to JST-V1-010 stay listed as retired if the change is withdrawn
and are never reused.
