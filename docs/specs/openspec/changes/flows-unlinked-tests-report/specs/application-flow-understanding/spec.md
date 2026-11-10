# Spec delta: `application-flow-understanding` (change `flows-unlinked-tests-report`)

## ADDED Requirements

The lane copies each statement below into `application-flow-understanding-v1.md`, inside its
`## Requirements` section, as a numbered definition line that begins
`(proposed 2026-11-09; V1-5255; delivery experimental)`.

Fresh IDs (owner decision R11, 2026-11-09): this delta uses AFU-V1-155 to AFU-V1-159. AFU-V1-150 to
AFU-V1-154 are retired: they keep the meanings recorded for lane `flowrep` in
`docs/specs/REQUIREMENTS.tsv` at integ 9ec3a29d0 and are never reused. Earlier drafts of this delta
rewrote 150..154 in place; that rewrite is withdrawn. Mapping: 150 to 155, 151 to 156, 152 to 157,
153 to 158, 154 to 159. AFU-V1-140 to 145 also stay retired (change `e2e-inventory-discovery`).
AFU-V1-166 and AFU-V1-167 were added on 2026-11-09 for owner answers 9 and 15. AFU-V1-160 to 165
are in use at integ 9ec3a29d0, so the block continues at 166.

### Requirement: AFU-V1-155 Gaps lists unassigned tests natively

Source: G11, G5. `corvint flows gaps` MUST run in `corvint` installed alone and MUST list each
inventory E2E test with no accepted, non-stale link record under code `unassigned`, with its
`test_id`. A link record counts as accepted only when a person accepted it (owner decision R12,
2026-11-09). The codes `unassigned`, `stale-link`, `dangling-link` and `no-e2e-proof` MUST be new
values of the existing code field of `application-flow-gaps/1`, with no new profile version, and
the profile text MUST gain one sentence: a reader MUST ignore a code it does not know (decided
2026-11-09 under R42, N1).

#### Scenario: One test is unassigned

- **GIVEN** an inventory of three E2E tests and accepted links for two
- **WHEN** the user runs `corvint flows gaps` with only `corvint` installed
- **THEN** the third test is listed as `unassigned` and the total `unassigned` count is 1

#### Scenario: A reader meets a code it does not know

- **GIVEN** a reader of `application-flow-gaps/1` written before `no-e2e-proof` existed
- **WHEN** it reads a report holding a `no-e2e-proof` record
- **THEN** the profile requires it to ignore that record and read the rest

### Requirement: AFU-V1-156 The MCP tool returns the same report

Source: G5; owner decision R30 (2026-11-09). `corvint.flows.gaps` on `corvint mcp` MUST return the
same records as the command for the same inputs, and its tool name, input schema, output profile and
error codes MUST match its row in the table of change `mcp-tool-table`.

#### Scenario: Command and tool agree

- **WHEN** the command and the tool run on the same inputs
- **THEN** both return the same records, codes and inventory state

### Requirement: AFU-V1-157 Stale and dangling links are reported

Source: G11. The report MUST list each link record whose cited test content hash or routes changed
since its pinned commit as `stale-link`, and each link record whose test or variation no longer
exists as `dangling-link`; a test of a stale link is also counted `unassigned`.

#### Scenario: Test renamed

- **WHEN** the title path of a linked test changed
- **THEN** the old record is `dangling-link` and the renamed test is `unassigned`

### Requirement: AFU-V1-158 An incomplete inventory is stated; optional ratchet

Source: G11, owner answer 4. When the inventory is absent, stale or partial, the report MUST say so
with a reason and exit 3. With `--ratchet --base FULL_SHA`, off by default, the command MUST exit 1
when the head has more `unassigned` tests than the base.

#### Scenario: Ratchet

- **WHEN** a change adds one Playwright test with no link and the ratchet is enabled
- **THEN** the command exits 1 and names the new test

### Requirement: AFU-V1-159 Bounded, read-only and deterministic

Source: invariant 4. The report MUST be bounded with a stable order, truncation marker and totals,
MUST write nothing, and MUST be byte-identical for identical inputs.

#### Scenario: Repeated run

- **WHEN** the command runs twice on the same inputs
- **THEN** both outputs are byte-identical and `git status` shows no change

### Requirement: AFU-V1-166 A linked test of any kind covers a variation

Source: owner answer 9; PROD-TEST-005. The report MUST count a variation as covered when at least
one accepted, non-stale link record attaches a test of any kind (unit, integration or E2E) to it.
`no-test` MUST be reported only for a variation with no such link. The `unassigned` list stays
limited to inventory E2E tests. The link record holds no `kind`: the report MUST read each linked
test's kind from the test inventory by its canonical `test_id`, with `unit` as the default and the
kind source reported (`affected-plan-kinds` decision 2). A link whose `test_id` is not in the
inventory MUST be reported as `dangling-link` (decided 2026-11-09 under R42, N2).

#### Scenario: Only a unit test is linked

- **GIVEN** a variation whose only accepted link names a Go unit test
- **WHEN** the user runs `corvint flows gaps`
- **THEN** the variation has no `no-test` record
- **AND** the unit test is not listed as `unassigned`

#### Scenario: Linked test missing from the inventory

- **GIVEN** an accepted link whose `test_id` is not in the test inventory
- **WHEN** the user runs `corvint flows gaps`
- **THEN** the link is reported as `dangling-link` and no kind is guessed for it

#### Scenario: The only link is stale

- **GIVEN** a variation whose only link record is `stale-link`
- **WHEN** the user runs `corvint flows gaps`
- **THEN** the variation is reported as `no-test`

### Requirement: AFU-V1-167 A flow with no E2E test is reported as `no-e2e-proof`

Source: owner answer 15; PROD-TEST-005. The report MUST emit one `no-e2e-proof` record, with the
flow ID, for each flow that has no accepted, non-stale link to an E2E test on any of its
variations. The record MUST NOT change the exit code, with or without `--ratchet`, and MUST be
omitted when the inventory is absent, stale or partial (AFU-V1-158 then applies).

#### Scenario: Flow covered by unit tests only

- **GIVEN** a flow whose two variations are each linked to a unit test and to no E2E test
- **WHEN** the user runs `corvint flows gaps`
- **THEN** the report has one `no-e2e-proof` record naming the flow and no `no-test` record
- **AND** the exit code is 0

#### Scenario: One E2E link is enough

- **WHEN** one variation of the flow gains an accepted link to an inventory E2E test
- **THEN** the flow has no `no-e2e-proof` record
