# Spec delta: `change-report` (change `change-report-impact`)

## ADDED Requirements

The lane copies each statement below into `change-report-v1.md`, inside its `## Requirements`
section, as a numbered definition line that begins `(proposed 2026-11-09; delivery not-started;
preview)`.

### Requirement: CRI-V1-001 Five sections on the affected engine

Source: G25 item 1. `corvint impact --base` MUST report the sections `tests`, `flows`, `docs`,
`tickets` and `requirements`, and the `tests` section MUST equal the affected plan for the same
range.

#### Scenario: Agreement with affected

- **WHEN** `impact` and `affected` run on the same range
- **THEN** the `tests` section lists the same tests, reasons and scope

### Requirement: CRI-V1-002 Every section has an explicit state

Source: G25 item 1, invariant 2. Each section MUST carry `OBSERVED`, `NOT_OBSERVED` or `INCOMPLETE`
with a reason, and a missing input MUST never be shown as an empty `OBSERVED` section.

#### Scenario: No docs check file

- **WHEN** `--docs-check` is not given
- **THEN** the docs section is `NOT_OBSERVED` with reason `no-docs-check`

### Requirement: CRI-V1-003 Flows from declared links

Source: G25 item 1, G6. The `flows` section MUST name each touched flow and variation from
`.corvint/flows.json` and accepted link records, with the variations that have no test. It MUST
equal the answer of `corvint flows impact` (AFL-V1-007, change `flows-status-report`) for the same
range; `impact` MUST NOT remove that verb or the `corvint.flows.impact` tool (review docs F2).

#### Scenario: Untested variation

- **WHEN** a change touches a flow with one variation that has no linked test
- **THEN** that variation is listed with `untested: true`

### Requirement: CRI-V1-004 Docs through a file, not a library

Source: G25 item 1, owner answer 13. The `docs` section MUST come only from a saved `corvint-docs`
docs check file bound to the head revision, and `impact` MUST NOT import `corvint-docs` code.
`--docs-check FILE` MUST accept the one-report output of `corvint-docs docs check` (DMF-V1-002,
change `docs-mcp-hardening`) as well as a `docs flows check` output, and MUST name in the docs
section which of the two it read (docs-mcp verdicts, suggested amendments).

#### Scenario: Stale claim

- **WHEN** the docs check file marks one claim stale
- **THEN** the docs section lists that claim and its page

#### Scenario: The repository docs check report

- **GIVEN** a `corvint-docs docs check` report bound to the head revision with one `STALE` part
- **WHEN** `impact` runs with `--docs-check` naming that file
- **THEN** the docs section lists each stale item with its path and line and names the report kind

### Requirement: CRI-V1-005 Tickets and requirements with their match source

Source: G25 item 1, G9. Each ticket and requirement MUST carry how it was matched (commit message,
claim scope, `requirementRefs`, or changed spec line), and the Tasks store MUST be read only.
Tickets MUST be matched both by IDs in commit messages and by overlap of changed files with claim
scope, each labelled, with commit-cited tickets listed first (R42 (panel pick) C10).

#### Scenario: Ticket cited in a commit

- **WHEN** a commit in the range cites V1-5257
- **THEN** the tickets section lists V1-5257 with match `commit-message`

#### Scenario: Ticket matched only by claim scope

- **GIVEN** a ticket whose claim scope covers `internal/appflows/` and no commit citing it
- **WHEN** the range changes `internal/appflows/model.go`
- **THEN** the ticket is listed with match `claim-scope`, after every commit-cited ticket

### Requirement: CRI-V1-006 One body for CLI, MCP and Action

Source: G25 item 1, G16. The CLI JSON output, the `corvint.impact` MCP tool and the Action comment
data MUST be the same `corvint-change-report/0` body for the same inputs.

#### Scenario: MCP parity

- **WHEN** the CLI and the MCP tool run on the same range
- **THEN** both bodies are byte-identical

### Requirement: CRI-V1-007 The old impact engine is removed

Source: G25 item 1. The `contextindex` impact engine files and their command glue MUST be removed,
and no package may import them.

#### Scenario: Build after removal

- **WHEN** the repository builds after the change
- **THEN** none of the listed files exists and `go vet ./...` passes

### Requirement: CRI-V1-008 Preview, read-only, bounded

Source: G25, invariant 4. The report MUST be labelled preview in its output and help, MUST write
nothing, MUST be bounded per section, and MUST NOT be a release gate.

#### Scenario: Release checks

- **WHEN** the release gate list is assembled
- **THEN** `impact` is absent from it

### Requirement: CRI-V1-009 One MCP table row

Source: owner decision R30 (2026-11-09); review docs F4. `corvint.impact` MUST be served only by
`corvint mcp`, with the tool name, input schema, output profile `corvint-change-report/0` and error
codes given by its row in the table of change `mcp-tool-table`. Error codes of the old tool that the
new one does not return MUST be listed in that row as removed.

#### Scenario: Tool list

- **GIVEN** both servers running
- **WHEN** a client lists the tools of each server
- **THEN** `corvint.impact` appears only on `corvint mcp`, and its schema and error codes match its
  table row
