# Spec delta: `acceptance-record` (change `acceptance-record`)

## ADDED Requirements

### Requirement: ACR-V1-001 One decision record accepts the plan as amended

One decision record MUST accept `docs/specs/PRODUCT-PLAN-1.0.md` as amended, naming the plan commit
SHA, and MUST be signed by the owner before it counts as accepted. (answer 18; owner decision R1
(2026-11-09))

#### Scenario: Plan changed after signing

- **GIVEN** a signed record naming plan SHA A
- **WHEN** the plan on `origin/main` is at SHA B with different content
- **THEN** the audit reports the record as stale

### Requirement: ACR-V1-002 Drafted wording is named

The record MUST list every requirement whose wording was written rather than quoted from an owner
answer. (owner decision R1 (2026-11-09))

#### Scenario: Reading the record

- **GIVEN** the decision record
- **WHEN** a reader looks up a PROD requirement written without an owner answer
- **THEN** the requirement is in the drafted wording list

### Requirement: ACR-V1-003 In-flight work stays experimental

The record MUST state that work in flight when it is signed stays experimental until its own change
is accepted. (owner decision R1 (2026-11-09))

#### Scenario: A lane in flight

- **GIVEN** a change folder with intent "proposed" when the record is signed
- **WHEN** its delivery is reported
- **THEN** it is reported as experimental, not delivered

### Requirement: ACR-V1-004 Supersession table with quoted text

The record MUST hold a table quoting each superseded text at a named commit and naming its
replacement, covering at least decision 0433 item 1, answer 3, answer 11, the G9 MCP row, R26,
decision 0594 item 4 and the dogfood sections of `AGENTS.md`. (owner decision R1 (2026-11-09); G9;
owner decisions R8, R9, R17, R39 and R40 (2026-11-09))

#### Scenario: A stale quote

- **GIVEN** a table row whose quote does not match its source at the cited commit
- **WHEN** the audit runs
- **THEN** it names the row

### Requirement: ACR-V1-005 Python file-level selection is a release exception

The record MUST list Python file-level selection as a release exception with a link to the 1.0.x
ticket for the Python line collector. (owner decision R9 (2026-11-09))

#### Scenario: Release review

- **GIVEN** the RC release record
- **WHEN** a reader looks for Python selection precision
- **THEN** the exception and the ticket are listed

### Requirement: ACR-V1-006 Acceptance log with one row per requirement

`docs/specs/openspec/acceptance-log.tsv` MUST hold one row for every PROD requirement, every checklist
O-row and every accepted change, with mode `owner`, `auto` or `pending-owner`, date, owner reference
and rationale. Only the owner MUST write `owner` rows. (G14)

#### Scenario: A missing row

- **GIVEN** a PROD requirement with no log row
- **WHEN** `make spec-acceptance-audit` runs
- **THEN** it names the requirement

### Requirement: ACR-V1-007 Log modes agree with spec markers

The audit MUST check that each log mode agrees with the requirement's spec marker: `owner` with
`(accepted, decision NNNN; V1-NNNN)` or `(accepted YYYY-MM-DD; V1-NNNN)`, `auto` with the
`standing` marker, `pending-owner` with no acceptance marker. (G14; SRG-V0-009; SRG-V0-010; file
`docs/SPEC-DRIVEN-DEVELOPMENT.md` lines 113 to 114 (integ 9ec3a29d0))

#### Scenario: Mode mismatch

- **GIVEN** a requirement marked `standing` in its spec and `owner` in the log
- **WHEN** the audit runs
- **THEN** it names the requirement and both modes

### Requirement: ACR-V1-008 Meaning changes go back to the owner

A change that alters an accepted requirement's meaning MUST set its log row to `pending-owner`, and a
lane MUST NOT write an `owner` row. (G14)

#### Scenario: A lane edits accepted text

- **GIVEN** a lane change that narrows an `owner` requirement
- **WHEN** the change is reviewed
- **THEN** its log row reads `pending-owner` until the owner reads it back

### Requirement: ACR-V1-009 Signed release record before RC and stable

The RC publish and the stable promotion MUST each have an owner-signed release record listing each
requirement's evidence or demotion, NOT_RUN and NOT_OBSERVED items and deferred P2 and P3 tickets.
(G10; REL-V1-013)

#### Scenario: Unsigned record

- **GIVEN** an RC tag and an unsigned release record
- **WHEN** the release workflow starts the publish step
- **THEN** it refuses and names the missing signature

### Requirement: ACR-V1-010 Release record format readable by the workflow

The release record MUST use one documented file format under `docs/releases/` that the release
workflow reads without network access beyond the repository. (G10)

#### Scenario: Malformed record

- **GIVEN** a release record missing the NOT_RUN list
- **WHEN** the workflow reads it
- **THEN** it refuses and names the missing section
