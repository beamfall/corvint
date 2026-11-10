# Spec delta: `review-evidence-check` (change `review-evidence-check`)

## ADDED Requirements

The lane copies each statement below into `review-evidence-check-v1.md`, inside its
`## Requirements` section, as a numbered definition line that begins `(proposed 2026-11-09;
delivery not-started; preview)`.

### Requirement: RVE-V1-001 The plan is recomputed, not trusted

Source: G25 item 2. `corvint review` MUST recompute the affected plan for the range and MUST NOT
take the selection from a receipt.

#### Scenario: Receipt with a narrow selection

- **WHEN** a receipt ran fewer tests than the recomputed plan selects
- **THEN** each missing test is reported as `selected-not-run`

### Requirement: RVE-V1-002 Selected tests must have passed on the head tree

Source: G25 item 2. A test selected for a changed line MUST be reported as `selected-not-run` unless
a receipt bound to the head tree records it as passed.

#### Scenario: Test failed

- **WHEN** the receipt records a selected test as failed
- **THEN** the finding is `selected-not-run` with reason `failed`

### Requirement: RVE-V1-003 Untested variations of changed flows

Source: G25 item 2, G6. A changed flow MUST produce `untested-variation` for each variation with no
linked test or whose linked tests have no passing outcome.

#### Scenario: Variation without link

- **WHEN** a changed flow has a variation with no accepted link
- **THEN** that variation is reported

### Requirement: RVE-V1-004 Unassigned E2E tests

Source: G25 item 2, G11. Each inventory E2E test without an accepted, non-stale link MUST be
reported as `unassigned-e2e`.

#### Scenario: New Playwright test

- **WHEN** the range adds a Playwright test with no link
- **THEN** it is reported as `unassigned-e2e`

### Requirement: RVE-V1-005 Stale docs through a file

Source: G25 item 2, owner answer 13, owner decision R4 (2026-11-09), review docs F3. Stale claims
MUST come only from a saved docs check file bound to the head revision; without one, or when
corvint-docs is absent, the kind is `NOT_OBSERVED`. A `NOT_OBSERVED` docs input MUST NOT block
`CLEAN`, so a testing-only install can reach `CLEAN`. `stale-docs` findings MUST be marked advisory
in the body and MUST NOT change the verdict; a consumer fails on them only when its opt-in setting
is on.

#### Scenario: No docs check file

- **WHEN** `--docs-check` is not given and every other input is observed with no finding
- **THEN** the inputs list shows docs `NOT_OBSERVED` and the verdict is `CLEAN`

Earlier scenario outcome, superseded by R4 and F3: "the verdict is not `CLEAN`".

### Requirement: RVE-V1-006 Ticket criteria without evidence

Source: G25 item 2, G9. With `--ticket ID`, each acceptance criterion with no recorded evidence MUST
be reported as `criterion-without-evidence`.

#### Scenario: One criterion has a receipt

- **WHEN** a ticket has two criteria and evidence is recorded for one
- **THEN** the other is reported

### Requirement: RVE-V1-007 Verdict and exit code

Source: G25 item 2, invariant 2. The verdict MUST be `CLEAN` only when every input other than the
docs input is observed and no non-advisory finding exists (the docs input follows RVE-V1-005), `INCOMPLETE` when any input is missing or the plan is not bounded, and
`FINDINGS` otherwise, with exit codes 0, 3 and 1.

#### Scenario: All inputs, no findings

- **WHEN** all inputs are present and every check passes
- **THEN** the verdict is `CLEAN` and the exit code is 0

### Requirement: RVE-V1-008 One body for all consumers

Source: G25 item 2, G16, owner decision R4 (2026-11-09), review docs F3. Every consumer of `review`
(the Action comment and the Tasks completion gate) MUST read the same `corvint-review-check/0` body.
Each consumer MUST treat the body as advisory unless its opt-in setting makes it blocking. The
agent end-of-turn report is not a consumer; it reads `corvint affected` and receipts (AGT-V1-013).
Earlier wording, superseded: "The Action comment, the end-of-turn report and the Tasks gate MUST
read the same body", with a Tasks gate that "passes only on `verdict: CLEAN`".

#### Scenario: Tasks gate without opt-in

- **WHEN** a ticket gate records `review` for the ticket's range and the policy has no opt-in
- **THEN** it stores the body's digest and its result is set by `affected` plus the receipt, not by
  the verdict

#### Scenario: Tasks gate with opt-in

- **WHEN** the policy opts in and the verdict is `FINDINGS`
- **THEN** the gate fails and names the findings

### Requirement: RVE-V1-009 Preview, deterministic, read-only; old review removed

Source: G25, invariant 4, review docs F3. The check MUST be labelled preview, MUST be byte-identical
for identical inputs, MUST write nothing, MUST NOT be a release gate, MUST NOT be required by any
release-blocking gate or report, and the guidance-based review MUST be removed.

#### Scenario: Repeated run

- **WHEN** the check runs twice on the same inputs
- **THEN** both outputs are byte-identical and `git status` shows no change

### Requirement: RVE-V1-010 One MCP table row

Source: owner decision R30 (2026-11-09). If `review` is offered over MCP, its server, tool name,
input schema, output profile and error codes MUST be those of its row in the table of change
`mcp-tool-table`. Integ 9ec3a29d0 has no `review` MCP tool (`internal/capcatalog/mcp.go`).

#### Scenario: Tool list

- **WHEN** a client lists the tools of both servers
- **THEN** any `review` tool appears only on the server its table row names, with that row's schema
