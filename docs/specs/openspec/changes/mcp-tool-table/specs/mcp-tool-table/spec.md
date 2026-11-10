# Spec delta: `mcp-tool-table` (change `mcp-tool-table`)

## ADDED Requirements

### Requirement: MTT-V1-001 One table fixes every MCP tool

One authored table MUST list every MCP tool Corvint serves exactly once, with its name, server, input
schema reference, output profile, typed error codes, availability and CLI equivalent. Tool names MUST
be unique across both servers. (owner decision R30 (2026-11-09); PROD-DOCS-001)

#### Scenario: A duplicate name

- **GIVEN** two rows named `corvint.status`, one per server
- **WHEN** the table test runs
- **THEN** it fails and names the duplicate

### Requirement: MTT-V1-002 `docs/MCP-SERVER.md` is generated from the table

`docs/MCP-SERVER.md` MUST be generated from the table, and a staleness check in `doc-gates` MUST fail
when the file differs from the generator output. (owner decision R30 (2026-11-09))

#### Scenario: A hand edit

- **GIVEN** a hand edit to a tool description in `docs/MCP-SERVER.md`
- **WHEN** `make doc-gates` runs
- **THEN** the staleness check fails

### Requirement: MTT-V1-003 The servers register exactly the table's tools

Each server MUST register exactly the tools and input schemas its table rows name. A test MUST fail on
a registered tool without a row or a row without a registered tool. (owner decision R30 (2026-11-09))

#### Scenario: An unlisted tool

- **GIVEN** a new tool registered on `corvint mcp` with no table row
- **WHEN** the drift test runs
- **THEN** it fails and names the tool

### Requirement: MTT-V1-004 `corvint-docs mcp` serves the documentation tools

`corvint-docs mcp` MUST serve the `docs_*` corpus tools, `docs_draft`, `docs_consume` and `help` over
stdio, and MUST work with only the documentation binaries installed. (owner decision R30
(2026-11-09); PROD-DOCS-001; PROD-INT-004)

#### Scenario: Documentation alone

- **GIVEN** a machine with only `corvint-docs` installed
- **WHEN** an MCP client starts `corvint-docs mcp` and lists tools
- **THEN** the documentation tools are listed and answer on a fixture repository

### Requirement: MTT-V1-005 `corvint mcp` keeps only testing and flow-report tools

`corvint mcp` MUST serve only `impact`, `status`, `affected`, `test_evidence`, `test_validity`, the
flow report tools and `help`. A request for the removed `docs` or `corpus` profile MUST return a typed
refusal that names `corvint-docs mcp`. (owner decision R30 (2026-11-09))

#### Scenario: An old docs profile

- **GIVEN** a client configured with `corvint mcp --profile docs`
- **WHEN** it starts the server
- **THEN** the server refuses with a typed reason naming `corvint-docs mcp`

### Requirement: MTT-V1-006 No Tasks MCP tools

Neither server MAY serve a Tasks tool. The table MUST carry a "no MCP tool" row stating that Tasks is
used through its CLI. (owner decision R17 (2026-11-09); PROD-TASK-007)

#### Scenario: Tool list

- **GIVEN** both servers started
- **WHEN** their tool lists are read
- **THEN** no tool name contains `tasks`, and the table's Tasks row has server `none`

### Requirement: MTT-V1-007 One naming convention

Every tool name MUST follow `corvint.<area>_<verb>` with exactly one dot. Renamed tools MUST list
their old names in the row's `replaces` column. (owner decision R30 (2026-11-09))

#### Scenario: A flow tool

- **GIVEN** the flow gaps tool
- **WHEN** the table is read
- **THEN** its name is `corvint.flows_gaps` and `replaces` lists `corvint.flows.gaps`

### Requirement: MTT-V1-008 `CORVINT_SELECTION` in the `affected` tool

The `affected` tool MUST read `CORVINT_SELECTION` with the CLI grammar, including `full:<runner>`. An
unknown value MUST select all tests and return a typed reason, never a narrowed plan. (owner decision
R13 (2026-11-09); PROD-TEST-013)

#### Scenario: An unknown value

- **GIVEN** `CORVINT_SELECTION=fastest` in the server environment
- **WHEN** a client calls the `affected` tool
- **THEN** every test is selected and the result carries the unknown-value reason

### Requirement: MTT-V1-009 Same answer as the command line

For every row with a CLI equivalent, the tool MUST return the same records as that command on the same
inputs. (PROD-DOCS-010; PROD-TEST-009)

#### Scenario: Affected parity

- **GIVEN** a fixture repository and a base commit
- **WHEN** the `affected` tool and `corvint affected --base FULL_SHA` run
- **THEN** both return the same selections

### Requirement: MTT-V1-010 Retired MCP binaries leave the release list

`corvint-mcp` and `corvint-test-validity-mcp` MUST leave `tools/release-binary/binaries.json` in the
removal batch that retires them. (owner decision R14 (2026-11-09); owner decision R30 (2026-11-09))

#### Scenario: Release list after the batch

- **GIVEN** the removal batch merged
- **WHEN** the release binaries check runs
- **THEN** neither `corvint-mcp` nor `corvint-test-validity-mcp` is listed
