# Spec delta: e2e-inventory-discovery

The lane copies each ADDED statement below into `application-flow-understanding-v1.md`, inside its
`## Requirements` section, as a numbered definition line that begins
`(proposed 2026-11-09; V1-5252; delivery experimental)`, and marks each REMOVED line retired in
place.

## ADDED Requirements

### Requirement: AFU-V1-146 E2E selection reads the committed inventory and runs no project code

Source: owner answer 4, G11, owner decisions R10 and R11 (2026-11-09), invariant 4; replaces the
2026-11-09 rewrites of AFU-V1-140 and AFU-V1-144. The E2E selection MUST take its test inventory
from the single committed inventory under `.corvint/` (change `test-inventory`), written only by
`corvint inventory refresh` with the Playwright lister marked `executing`, with one entry per test
keyed by the canonical test ID (RNC-V1-012), and MUST NOT need a hand-written inventory file.
`corvint affected` and `corvint watch` MUST NOT execute project code to obtain the inventory, and
`corvint watch` MUST NOT refresh it.

#### Scenario: A new test is added

- **WHEN** a commit adds a Playwright test and `corvint inventory refresh` is run again
- **THEN** the new test is in the inventory without any edit to a provider file

#### Scenario: Config with side effects

- **WHEN** the Playwright config contains code with side effects
- **THEN** `corvint affected` starts no child process for it

### Requirement: AFU-V1-147 An unassigned test always runs

Source: G11; replaces the 2026-11-09 rewrite of AFU-V1-141. A discovered E2E test that no accepted,
non-stale link record ties to a flow MUST be selected for every change and MUST be counted as
`unassigned` in the plan.

#### Scenario: Test with no accepted link

- **WHEN** the inventory holds a test with no accepted link
- **THEN** the test is selected for every change and the plan's `unassigned` count includes it

### Requirement: AFU-V1-148 Absent, stale and partly stale inventories

Source: owner answer 4, G4, owner decision R10 (2026-11-09), review T-M2 and T-M3; replaces the
2026-11-09 rewrites of AFU-V1-142 and AFU-V1-143. The plan MUST use the inventory's Playwright
entries only when the inventory binds the runner config, the lockfile and the inventory profile of
the planned tree. Then:

- absent inventory: the Playwright runner's declared full command (RNC-V1-013) MUST run with reason
  `inventory-absent`;
- config, lockfile or profile binding differs: every E2E test MUST be selected with reason
  `inventory-stale`;
- a test file added or changed after the inventory was written: the tests in that file MUST be
  selected and counted `unassigned`; other entries stay usable;
- malformed input, over a bound, or an unknown profile: typed refusal, exit 2.

The plan MUST NOT report that no E2E test is required in any of these cases.

#### Scenario: One test file changed

- **WHEN** a test file changed after the inventory was written and nothing else changed
- **THEN** that file's tests are selected and counted `unassigned`, and other E2E tests are
  selected only by their links

#### Scenario: Config changed

- **WHEN** `playwright.config.ts` changed after the inventory was written
- **THEN** every E2E test is selected with reason `inventory-stale`

#### Scenario: No inventory

- **WHEN** the repository has no committed inventory
- **THEN** the Playwright full command runs and the receipt names `inventory-absent`

### Requirement: AFU-V1-149 A provider file can add, never remove

Source: G2, G11, settled S5; replaces the 2026-11-09 rewrite of AFU-V1-145. An explicit
`--provider FILE` MUST be able to add tests to the E2E selection, MUST NOT remove a test the
inventory lists, and every difference between the two MUST be reported.

#### Scenario: Provider file lists fewer tests

- **WHEN** the provider file omits a test that the inventory lists
- **THEN** the omitted test is still selected and the result names it under `provider-omits`

## REMOVED Requirements

Each ID below stays listed as retired and is never reused (owner decision R11 (2026-11-09)). Each
entry gives the meaning in integ `9ec3a29d0` (`docs/specs/application-flow-understanding-v1.md`,
lines 766-799) and the meaning proposed in this folder earlier on 2026-11-09.

### Requirement: AFU-V1-140 (retired)

Integ meaning: inventory from Playwright discovery; `.corvint/flows.json` member `playwright`
(`config`, `discovery`) and one (project, test file) entry per receipt unit. Earlier proposal
meaning: a pinned inventory from an explicit step, keyed by file path plus title path. Superseded
by AFU-V1-146.

#### Scenario: Retired ID

- **WHEN** the spec index is generated
- **THEN** AFU-V1-140 is listed as retired and no test cites it

### Requirement: AFU-V1-141 (retired)

Integ meaning: an unlinked test is selected with reason `unlinked` and listed under
`plan.e2e.inventory.unlinked`. Earlier proposal meaning: an unassigned test always runs and is
counted. Superseded by AFU-V1-147.

#### Scenario: Retired ID

- **WHEN** the spec index is generated
- **THEN** AFU-V1-141 is listed as retired and no test cites it

### Requirement: AFU-V1-142 (retired)

Integ meaning: an underivable inventory fails closed with scope `FULL`, reason
`inventory-underivable` and the unknown `E2E_INVENTORY_UNDERIVABLE`, naming `corvint affected
discovery` as the repair. Earlier proposal meaning: a stale or absent inventory widens to every E2E
test; only malformed input is refused. Superseded by AFU-V1-148.

#### Scenario: Retired ID

- **WHEN** the spec index is generated
- **THEN** AFU-V1-142 is listed as retired and no test cites it

### Requirement: AFU-V1-143 (retired)

Integ meaning: a receipt is used only when `VerifyPlaywrightDiscovery` reports `MATCHED` against
HEAD, the config bytes and the planned tree's source digest. Earlier proposal meaning: the
inventory is bound to the runner config, the lockfile and the test files of the planned tree.
Superseded by AFU-V1-148, which binds config, lockfile and profile and treats test files per file.

#### Scenario: Retired ID

- **WHEN** the spec index is generated
- **THEN** AFU-V1-143 is listed as retired and no test cites it

### Requirement: AFU-V1-144 (retired)

Integ meaning: the plan read executes no project code; the listing comes from the separate
`corvint affected discovery` step. Earlier proposal meaning: the same, without naming the step.
Superseded by AFU-V1-146, with the write verb `corvint inventory refresh`.

#### Scenario: Retired ID

- **WHEN** the spec index is generated
- **THEN** AFU-V1-144 is listed as retired and no test cites it

### Requirement: AFU-V1-145 (retired)

Integ meaning: a hand-written provider still decides; `advice.test_selection` stays the selection
to run and `plan.e2e.inventory.providerOmits` lists omitted derived files. Earlier proposal
meaning: a provider file can add, never remove. Superseded by AFU-V1-149, which reverses the integ
meaning.

#### Scenario: Retired ID

- **WHEN** the spec index is generated
- **THEN** AFU-V1-145 is listed as retired and no test cites it
