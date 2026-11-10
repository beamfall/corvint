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

### Requirement: AFU-V1-155 Gaps lists unassigned tests natively

Source: G11, G5. `corvint flows gaps` MUST run in `corvint` installed alone and MUST list each
inventory E2E test with no accepted, non-stale link record under code `unassigned`, with its
`test_id`. A link record counts as accepted only when a person accepted it (owner decision R12,
2026-11-09).

#### Scenario: One test is unassigned

- **GIVEN** an inventory of three E2E tests and accepted links for two
- **WHEN** the user runs `corvint flows gaps` with only `corvint` installed
- **THEN** the third test is listed as `unassigned` and the total `unassigned` count is 1

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
