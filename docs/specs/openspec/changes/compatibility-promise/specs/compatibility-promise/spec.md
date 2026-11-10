# Spec delta: `compatibility-promise` (change `compatibility-promise`)

## ADDED Requirements

### Requirement: CMP-V1-001 One compatibility table with a stability column

`docs/COMPATIBILITY.md` MUST hold the 1.x compatibility promise as one table with one row per
surface and a stability column whose value is `stable` or `preview`. A surface that has no row
MUST be treated as `preview`. (decided 2026-11-09 under R42, D10 and D10a)

#### Scenario: A surface without a row

- **GIVEN** an output format that has no row in `docs/COMPATIBILITY.md`
- **WHEN** a user asks whether it can change in a 1.x release
- **THEN** the page says that unlisted surfaces are `preview` and can change

### Requirement: CMP-V1-002 Only contract-tested surfaces are stable

A row MUST be `stable` only when a contract test covers the surface, and the row MUST name that
test. The stable rows MUST be generated from the existing contract tests; a row MUST NOT be
written by hand. (decided 2026-11-09 under R42, D10)

#### Scenario: A stable row without a test

- **GIVEN** a `stable` row whose named contract test does not exist
- **WHEN** the compatibility check runs
- **THEN** it fails and names the row

#### Scenario: Regenerating the table

- **GIVEN** the contract tests are unchanged
- **WHEN** the table is regenerated
- **THEN** `docs/COMPATIBILITY.md` is byte-identical

### Requirement: CMP-V1-003 The stable set

The stable set MUST be exactly these surfaces, each subject to CMP-V1-002. (decided 2026-11-09
under R42, D10 and D10a)

1. The supported-command tables of `corvint`, `corvint-docs` and `corvint-tasks`: verbs, flags
   and exit codes.
2. The documented `--json` fields of these formats: flow file, plan, test-run receipt, run
   evidence, ticket schema v1 and receipt reference v1.
3. The documented files under `.corvint/` and in the Tasks store.
4. The action's inputs and outputs, the action's major tag, and the Corvint version range that
   each agent package declares.
5. The MCP tool names in the tool table.

Which label each format in item 2 carries today is UNVERIFIED; the lane reads each label from
the code before it writes the row. If fewer than half of the surfaces above have a contract
test today, the lane MUST stop and report that to the owner before it writes the table.

#### Scenario: A listed surface has no contract test

- **GIVEN** the exit codes of one supported verb have no contract test
- **WHEN** the table is generated
- **THEN** that verb's exit codes have no `stable` row, and the change's build-log entry names
  the gap

### Requirement: CMP-V1-004 What stable means

A `stable` surface MUST change only by addition within 1.x. Removing or renaming a stable verb,
flag, field, file, input, output or tool name MUST be announced one minor release before it
happens, and the old name MUST keep working as an alias during that release. (decided
2026-11-09 under R42, D10 and D10a)

#### Scenario: Removing a stable flag

- **GIVEN** a stable flag that release 1.3 wants to remove
- **WHEN** release 1.3 is prepared
- **THEN** the flag still works in 1.3 with a deprecation notice, and it is removed no earlier
  than 1.4

#### Scenario: Adding a field

- **GIVEN** a stable `--json` format
- **WHEN** a 1.x release adds a field to it
- **THEN** every documented field keeps its name, type and meaning

### Requirement: CMP-V1-005 Preview surfaces are excluded by name

`docs/COMPATIBILITY.md` MUST list these as `preview`, by name: `corvint-feature-map/0`,
`corvint-data-map/0`, `corvint-change-report/0`, `corvint-review-check/0`,
`corvint-flow-link-proposals/0`, `corvint-harness-event/0`, the MCP input and output schemas,
the Go packages, the index and every cache. A `preview` surface MAY change in any release, and
the release notes MUST name the change. (decided 2026-11-09 under R42, D10 and D10a)

#### Scenario: A cache format changes

- **GIVEN** a 1.x release that changes the index encoding
- **WHEN** a user upgrades
- **THEN** the index is rebuilt, the release notes name the change, and no promise is broken

#### Scenario: An MCP schema changes

- **GIVEN** a 1.x release that adds a required input to an MCP tool
- **WHEN** a reader checks the promise
- **THEN** the tool name is `stable` and its input schema is `preview`

### Requirement: CMP-V1-006 No label is renamed at 1.0

No profile label MUST be renamed at 1.0. The digit in a label is part of the name and MUST NOT
be read as a stability mark; the stability column of CMP-V1-001 carries that meaning. A
breaking change to a stable profile MUST take the next digit, and the old profile MUST be served
for one minor release. (decided 2026-11-09 under R42, D10a)

#### Scenario: A stable profile whose label ends in `/0`

- **GIVEN** a stable format whose label ends in `/0`
- **WHEN** 1.0 ships
- **THEN** the label is unchanged and its row reads `stable`

#### Scenario: A breaking change to a stable profile

- **GIVEN** a stable profile `name/0` that needs a breaking change in 1.4
- **WHEN** 1.4 ships
- **THEN** 1.4 writes `name/1` and still serves `name/0`, and `name/0` is removed no earlier
  than 1.5
