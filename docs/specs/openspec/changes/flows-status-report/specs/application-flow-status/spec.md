# Spec delta: `application-flow-status` (change `flows-status-report`)

## ADDED Requirements

The lane copies each statement below into `application-flow-status-v1.md`, inside its
`## Requirements` section, as a numbered definition line that begins
`(proposed 2026-11-09; V1-5257; delivery not-started)`.

### Requirement: AFL-V1-001 One native report per flow and variation

Source: G5, G11. `corvint flows status` MUST run with `corvint` installed alone and MUST report,
for each flow and variation in `.corvint/flows.json`, the tests tied to it by accepted, non-stale
link records, plus the `unassigned` count of the E2E inventory.

#### Scenario: corvint installed alone

- **GIVEN** a repository with two flows, link records and no `corvint-docs` on `PATH`
- **WHEN** the user runs `corvint flows status`
- **THEN** the report lists both flows, their variations and their linked tests

### Requirement: AFL-V1-002 Results only from named writers

Source: G5, G24. The report MUST take a test result only from a `corvint-test-run/0` receipt
written by `corvint test` or from a `test-run-evidence/0` file written by `flows ingest`, MUST show
`NOT_OBSERVED` for any test with neither, and MUST mark a result from another commit as stale with
its source commit.

#### Scenario: One test has no recorded result

- **WHEN** the receipt covers one linked test and not another
- **THEN** the first shows its outcome and the second shows `NOT_OBSERVED`

### Requirement: AFL-V1-003 The MCP tool returns the same report

Source: G5; owner decision R30 (2026-11-09). `corvint.flows.status` on `corvint mcp` MUST return the
same records as the command for the same inputs, and its tool name, input schema, output profile and
error codes MUST match its row in the table of change `mcp-tool-table`.

#### Scenario: Command and tool agree

- **WHEN** the command and the tool run on the same inputs
- **THEN** both return byte-identical report bodies

### Requirement: AFL-V1-004 `--base` matches `flows impact`

Source: G5, G6. With `--base FULL_SHA`, the report MUST contain exactly the flows and variations
that `corvint flows impact` reports as touched for the same base and head.

#### Scenario: One flow touched

- **WHEN** a change touches one route of one flow
- **THEN** the report and `flows impact` name the same single flow and variation

### Requirement: AFL-V1-005 Bounded, read-only and deterministic

Source: invariant 4. The report MUST be bounded with a stable order, truncation marker and totals,
MUST write nothing, and MUST be byte-identical for identical inputs.

#### Scenario: Repeated run

- **WHEN** the command runs twice on the same inputs
- **THEN** both outputs are byte-identical and `git status` shows no change

### Requirement: AFL-V1-006 Invalid input is refused whole

Source: G5, invariant 2. A malformed, oversized or unknown-profile flow file, receipt or evidence
file MUST produce a typed refusal with exit 2 and MUST NOT produce a partial report.

#### Scenario: Unknown flow file version

- **WHEN** `.corvint/flows.json` declares profile `corvint-flows/9`
- **THEN** the command exits 2 with a typed refusal naming the profile

### Requirement: AFL-V1-007 `flows impact` is native

Source: G5 (docs F2). `corvint flows impact --base FULL_SHA` MUST run with `corvint` installed
alone, computing the touched flows and variations and their linked tests on the shared flow model
and readers with the reach of APK-V0-010. The `corvint-docs` copy of the verb MUST be removed and
replaced by a notice naming `corvint flows impact`. The `corvint.flows.impact` tool MUST stay on
`corvint mcp` and return the same records as the command.

#### Scenario: corvint installed alone

- **GIVEN** a repository with `.corvint/flows.json`, link records and no `corvint-docs` on `PATH`
- **WHEN** the user runs `corvint flows impact --base FULL_SHA`
- **THEN** the touched flows, variations and linked tests are printed and the exit code is 0

#### Scenario: Tool and command agree

- **WHEN** `corvint.flows.impact` and the command run on the same inputs
- **THEN** both return byte-identical report bodies

Failure mode: the flow file has an unknown profile or the graph is over its bound: typed refusal,
exit 2, no partial answer (as AFL-V1-006). Acceptance: the two scenarios as Go tests, the release
smoke test with `corvint-docs` absent, and the APK-V0-007 comparison test. Rollback: revert the
commit; `runFlowsShim` again forwards `flows impact` to `corvint-docs`, and APK-V0-007 returns to
blocked.
