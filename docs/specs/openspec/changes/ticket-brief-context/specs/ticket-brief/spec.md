# Spec delta: `ticket-brief` (change `ticket-brief-context`)

## ADDED Requirements

The lane copies each statement below into `ticket-brief-v1.md`, inside its `## Requirements`
section, as a numbered definition line that begins `(proposed 2026-11-09; delivery not-started;
non-blocking)`.

### Requirement: TBC-V1-001 Brief from declared links only

Source: G25 item 3. `corvint context --ticket ID` MUST build the brief only from the ticket's own
fields (read with `corvint-tasks ticket show <ID> --json`; Corvint MUST NOT read the Tasks store
files directly), the requirements it cites, the flows it cites with their accepted links, and the affected
plan for its declared paths.

#### Scenario: Ticket with two requirement refs

- **WHEN** a ticket cites two requirement IDs and one path
- **THEN** the brief lists both requirements with their spec lines and the plan for that path

### Requirement: TBC-V1-002 No ranking

Source: G25 item 3, G25 "still removed". The brief MUST NOT rank, score or match terms, and every
item MUST name the link that produced it.

#### Scenario: Item provenance

- **WHEN** the brief lists a test
- **THEN** the item names the accepted link record or plan reason that selected it

### Requirement: TBC-V1-003 Missing links are stated

Source: invariant 2. A missing or dangling link MUST be listed as `NOT_OBSERVED` or
`dangling-requirement`, never filled by a guess.

#### Scenario: No flow cited

- **WHEN** the ticket cites no flow
- **THEN** the flows item is `NOT_OBSERVED`

### Requirement: TBC-V1-004 Asked for, never injected

Source: G19. The brief MUST be produced only by an explicit CLI or MCP call, and no Corvint hook may
insert it into an agent's context.

#### Scenario: Agent hook

- **WHEN** an agent hook fires at session start
- **THEN** it does not call `context`

### Requirement: TBC-V1-005 Ranking code removed

Source: G25 "still removed". The nine ranking files in `internal/contextindex` and the ranked path
of `cmd/corvint/taskcontext.go` MUST be removed, keeping `BuildContext` and `QueryIntent`.

#### Scenario: Build after removal

- **WHEN** the repository builds after the change
- **THEN** none of the nine files exists and `cmd/corvint-docs` still builds

### Requirement: TBC-V1-006 Keep-or-remove measurement

Source: G25 item 3; owner decisions R28 and R29 (2026-11-09); review docs F10. The brief ships in
1.0 non-blocking with no claim of measured benefit. In 1.0.x the brief MUST be measured on a fixed
task set, and it MUST be removed in a 1.0.x release unless the measurement meets the threshold the
owner sets. At the 1.0 release candidate the measurement is NOT_RUN, and that MUST NOT block the
release or remove the brief. The runs MUST use the owner's logged-in subscription CLIs on the
owner's machines, with no API keys and no spend cap (owner decisions R39 and R40, 2026-11-09).

Earlier wording, superseded by R28 and R29: "Before 1.0 stable, the brief MUST be measured on a
fixed task set, and it MUST be removed unless the measurement meets the threshold the owner sets."

Measurement design (proposed):

- Task set: 20 closed tickets of this repository with at least one `requirementRefs` ID and a
  declared path, chosen and hashed before any run; the list and hash are committed first.
- Arms: the same pinned agent version, prompt, tools and permissions; arm A starts from the ticket
  text, arm B also receives the brief as the first tool result it asks for. Three runs per task per
  arm, at the ticket's base commit.
- Metric: tool calls until the agent reports done; secondary: task success by the ticket's frozen
  acceptance checks.
- Proposed threshold for the owner: keep if the median tool-call count drops by at least 20% and the
  success rate is not lower; otherwise remove.

#### Scenario: Threshold missed

- **WHEN** the 1.0.x median tool-call reduction is 8%
- **THEN** the brief builder and the `context` command are removed in a 1.0.x release

#### Scenario: Not measured at the release candidate

- **WHEN** the 1.0 release candidate is cut with no measurement
- **THEN** the measurement is recorded as NOT_RUN, the brief ships non-blocking, and the 1.0
  documentation makes no benefit claim

### Requirement: TBC-V1-007 Preview, read-only, deterministic

Source: G25, invariant 4, owner decisions R28 and R29 (2026-11-09). The brief MUST be labelled
non-blocking with no claim of measured benefit (earlier: "labelled preview"), MUST write nothing,
and MUST be byte-identical for identical inputs.

#### Scenario: Repeated run

- **WHEN** the brief is built twice for the same ticket revision and commit
- **THEN** both outputs are byte-identical

### Requirement: TBC-V1-008 One MCP table row

Source: owner decision R30 (2026-11-09). If the brief is offered over MCP, its server, tool name,
input schema, output profile and error codes MUST be those of its row in the table of change
`mcp-tool-table`. The ranked `context` MCP form is removed.

#### Scenario: Tool list

- **WHEN** a client lists the tools of both servers
- **THEN** any brief tool appears only on the server its table row names, with that row's schema
