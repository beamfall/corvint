# Spec delta: `feature-map` (change `feature-map-from-flows`)

## ADDED Requirements

The lane copies each statement below into `feature-map-v1.md`, inside its `## Requirements`
section, as a numbered definition line that begins `(proposed 2026-11-09; delivery not-started;
preview)`.

### Requirement: FMP-V1-001 The map comes from the flow file

Source: G25 item 4, G6. `corvint-docs` MUST generate the feature map only from `.corvint/flows.json`
and accepted link records, with one entry per declared flow and its variations, each cited to the
flow file's commit.

#### Scenario: Two flows

- **WHEN** the flow file declares two flows with three variations
- **THEN** the map has two entries listing those variations and their citations

### Requirement: FMP-V1-002 No inferred features

Source: G25 item 4, invariant 3. The map MUST NOT contain a feature that the flow file does not
declare, and the regex discovery at `cmd/corvint/repository_guidance.go:281-282` MUST be removed.

#### Scenario: Unregistered handler

- **WHEN** the code has a route handler that no flow declares
- **THEN** the map does not list it as a feature

### Requirement: FMP-V1-003 Old verbs removed with a notice

Source: G25 item 4. `corvint feature`, `corvint features` and `corvint overview` MUST be removed, and
for one release each MUST print a notice naming its replacement and exit 2.

#### Scenario: Old verb

- **WHEN** a user runs `corvint features`
- **THEN** the output names the `corvint-docs` map command and the exit code is 2

### Requirement: FMP-V1-004 Proposals need acceptance

Source: G25 item 6, answer 10; owner decision R12 (2026-11-09). A flow declaration proposed by a
model MUST NOT appear in the map until a person accepts it, a test link MUST count only when a person
accepted it, and the map MUST count proposed, unaccepted declarations.

#### Scenario: Pending proposal

- **WHEN** a proposal adds a flow that is not accepted
- **THEN** the map omits it and `proposed-unaccepted` is 1

### Requirement: FMP-V1-005 Missing input is stated

Source: invariant 2. Without `.corvint/flows.json` the map MUST be `NOT_OBSERVED` with a reason.

#### Scenario: No flow file

- **WHEN** the repository has no flow file
- **THEN** the map is empty with state `NOT_OBSERVED`

### Requirement: FMP-V1-006 Preview, read-only, deterministic

Source: G25, invariant 4. The map MUST be labelled preview, MUST write only to an explicit output
path, and MUST be byte-identical for identical inputs.

#### Scenario: Repeated generation

- **WHEN** the map is generated twice from the same commit
- **THEN** both outputs are byte-identical

### Requirement: FMP-V1-007 One MCP home and one table row

Source: owner decision R30 (2026-11-09); review docs F4. If the feature map is offered over MCP, it
MUST be served only by `corvint-docs mcp`, with the tool name, input schema, output profile and error
codes given by its row in the table of change `mcp-tool-table`, and `corvint mcp` MUST NOT serve it.

#### Scenario: Docs server only

- **GIVEN** both servers running
- **WHEN** a client lists the tools of `corvint mcp`
- **THEN** no feature map tool is listed, and any feature map tool on `corvint-docs mcp` matches its
  table row
