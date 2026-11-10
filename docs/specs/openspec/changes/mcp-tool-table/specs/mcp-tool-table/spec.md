# Spec delta: `mcp-tool-table` (change `mcp-tool-table`)

## ADDED Requirements

### Requirement: MTT-V1-001 One table fixes every MCP tool

One authored table MUST list every MCP tool Corvint serves exactly once, with its name, server,
description, input schema reference, output profile, typed error codes, availability and CLI
equivalent. Each row MUST also carry a description for every input property. The description
column is what DMF-V1-008 (change `docs-mcp-hardening`) checks, so one table and one drift check
enforce it. The typed-error column MUST list the DMF-V1-010 members `server-busy` and `cancelled`
for every tool. Tool names MUST be unique across both servers. (owner decision R30 (2026-11-09);
PROD-DOCS-001; docs-mcp verdicts, suggested amendments)

The docs rows are re-described, not pruned: after `corvint.docs_recommend_tests` is removed, the
19 remaining `corvint.docs_*` rows stay. A docs row MAY be removed before the freeze only when a
fixture test shows that the tool's answer is a strict subset of another tool's answer for the
same input; any other removal waits for 1.1 with one release of notice. (decided 2026-11-09
under R42, B9)

The table MUST also hold one row for `corvint.docs_how` on the `corvint-docs mcp` server. It
answers a how-to question from the application map, keeps the `site-knowledge-how/0` output
profile of `corvint-docs how --json`, and lands with change `how-rewrite` (HOW-V1-011). (decided
2026-11-09 under R42, B10)

#### Scenario: A duplicate name

- **GIVEN** two rows named `corvint.status`, one per server
- **WHEN** the table test runs
- **THEN** it fails and names the duplicate

#### Scenario: Two rows share a description

- **GIVEN** two `corvint.docs_*` rows with the same description, or a row whose input property has
  no description
- **WHEN** the table test runs
- **THEN** it fails and names the rows or the property

#### Scenario: A row without the shared error codes

- **GIVEN** a row whose typed-error column lacks `server-busy` or `cancelled`
- **WHEN** the table test runs
- **THEN** it fails and names the row and the missing code

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

`corvint mcp` MUST serve only `impact`, `status`, `affected`, `test_evidence` (labelled preview),
the flow report tools and `help`; `test_validity` is removed with the G24 batch and a call to it
returns the unknown-tool error (decided 2026-11-09 under R42, M12). A request for the removed
`docs` or `corpus` profile MUST return a typed
refusal that names `corvint-docs mcp`. (owner decision R30 (2026-11-09))

Earlier wording, superseded by M12: "MUST serve only `impact`, `status`, `affected`,
`test_evidence`, `test_validity`, the flow report tools and `help`."

#### Scenario: A call to the removed tool

- **GIVEN** `corvint mcp` started after the G24 batch
- **WHEN** a client lists tools and then calls `test_validity`
- **THEN** the list has no `test_validity` tool, `test_evidence` is labelled preview, and the call
  returns the unknown-tool error

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
inputs. (PROD-DOCS-010; PROD-TEST-009) Two stated exceptions apply to the `affected` tool, where
the command exits 2 and the tool selects all tests with a reason: an unknown `CORVINT_SELECTION`
value (MTT-V1-008) and a stale binary (MTT-V1-011; decided 2026-11-09 under R42, M1).

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

### Requirement: MTT-V1-011 The `affected` tool on a stale binary selects all tests

On a stale binary (the running binary is older than the source) the `affected` tool MUST return a
successful tool result holding a select-all plan with reason `self-binary-stale` and a one-line
rebuild hint, and MUST NOT return a narrowed plan or a tool error. The CLI and CI keep exit 2. This
follows the pattern of owner decision R13 (2026-11-09) and MTT-V1-008, and is a stated exception to
MTT-V1-009. (decided 2026-11-09 under R42, M1; PROD-TEST-009)

#### Scenario: Stale binary through MCP

- **GIVEN** a `corvint mcp` server whose binary is older than the source
- **WHEN** a client calls the `affected` tool
- **THEN** the call succeeds, every test is selected, the reason is `self-binary-stale`, and the
  result carries a one-line rebuild hint

#### Scenario: Stale binary on the command line

- **GIVEN** the same stale binary
- **WHEN** `corvint affected --base FULL_SHA` runs
- **THEN** it exits 2, as before

Failure mode: the tool returns a narrowed plan or a tool error on a stale binary; an agent then
runs too few tests, retries or skips tests. Acceptance: one Go test of the `affected` tool on a
stale binary (select-all, reason, hint, no tool error) and one of the CLI exit code. Rollback:
revert the commit; the tool then has no stated stale-binary rule and the ID is retired, not reused.
