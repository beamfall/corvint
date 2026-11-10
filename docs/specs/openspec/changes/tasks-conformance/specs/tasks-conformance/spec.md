# Spec delta: `tasks-conformance` (change `tasks-conformance`)

## ADDED Requirements

### Requirement: TCF-V1-001 Schema v1 files and shared fixture

corvint-tasks MUST publish schema files for `taskman-ticket/1` and `taskman-receipt-ref/1` (proposed
names) and a fixture of valid and invalid documents in its repository. A receipt reference MUST carry
the receipt ID of TIN-V1-012. (owner decision R17 (2026-11-09); G9; TIN-V1-004)

#### Scenario: An invalid fixture document

- **GIVEN** the fixture document with a missing ticket ID
- **WHEN** the schema check runs
- **THEN** the document is rejected and the check names the missing member

### Requirement: TCF-V1-002 Published supported-command list

corvint-tasks MUST publish a supported-command list naming each command, its arguments, the
requirement it serves (PROD-TASK-001 to 009, I1 or I2) and the schema of its JSON output. The list
MUST NOT contain an `mcp` command. (owner decision R17 (2026-11-09); PROD-TASK-007)

#### Scenario: Reading the list

- **GIVEN** the corvint-tasks 1.0 release
- **WHEN** the supported-command list is read
- **THEN** every entry names a requirement and an output schema, and no entry is `mcp`

### Requirement: TCF-V1-003 Unlisted commands are experimental

`corvint-tasks help` MUST mark every command not on the supported-command list as experimental. (owner
decision R17 (2026-11-09))

#### Scenario: An unlisted command

- **GIVEN** a command present in the binary but not on the list
- **WHEN** a user runs `corvint-tasks help`
- **THEN** the command is shown as experimental

### Requirement: TCF-V1-004 Nine requirements traced to tests

Each of PROD-TASK-001 to 009 MUST be traced to at least one passing test at the Tasks tag commit.
(G9; PROD-TASK-010)

#### Scenario: An untraced requirement

- **GIVEN** PROD-TASK-006 with no test in the trace
- **WHEN** the record is produced
- **THEN** the record marks the trace check failed and names PROD-TASK-006

### Requirement: TCF-V1-005 Listed commands checked against schema v1

Every command on the supported-command list MUST have a test whose JSON output passes its declared
schema. (owner decision R17 (2026-11-09); PROD-TASK-010)

#### Scenario: Output drift

- **GIVEN** a listed command whose output gains an undeclared member
- **WHEN** its schema check runs
- **THEN** the check fails and names the command and member

### Requirement: TCF-V1-006 At least 8 concurrent processes never double claim

A test MUST run at least 8 separate processes claiming from one store with overlapping scopes, and the
store event log MUST show zero tickets with two live claims at any time. (G9; PROD-TASK-010)

#### Scenario: A double claim

- **GIVEN** a run where two processes both hold a claim on one ticket
- **WHEN** the event log is checked
- **THEN** the concurrency check fails and names the ticket and both claimants

### Requirement: TCF-V1-007 Recovery after kill -9 during a write

A test MUST kill a writer with SIGKILL at injected points during a write and then show that `receipt
audit` and `queue status` succeed and that a ticket ID and content hash diff shows nothing lost or
duplicated, with the interrupted write either fully present or absent. (G9; PROD-TASK-010)

#### Scenario: Kill during completion

- **GIVEN** a writer killed while writing a completion receipt
- **WHEN** `receipt audit` runs
- **THEN** it reports zero failures and the receipt is either complete or absent

### Requirement: TCF-V1-008 Conformance record `taskman-conformance/1`

One command MUST produce a `taskman-conformance/1` record at the Tasks tag commit holding the Tasks
version and commit, the results of TCF-V1-004 to 007, `receipt audit` with zero failures, and the
SHA-256 of the schema files and supported-command list. (G9; TIN-V1-002)

#### Scenario: Record read by Corvint

- **GIVEN** a record whose schema hash differs from Corvint's vendored pin
- **WHEN** the Corvint RC check reads it
- **THEN** it refuses the RC and names the mismatch

### Requirement: TCF-V1-009 Trial subject `beamfall/core`

The 21-day trial MUST run on the frozen Tasks RC binary in Corvint and `beamfall/core`, with a ticket
ID and content hash diff at start and end and at least 50 ticket writes and 5 completions per
repository. A store format change MUST restart the clock. (owner decision R18 (2026-11-09); G9;
answer 12; REL-V1-010)

#### Scenario: Low activity

- **GIVEN** `beamfall/core` with 3 completions at day 21
- **WHEN** the trial record is read
- **THEN** the trial is reported below its activity floor

### Requirement: TCF-V1-010 Three targets from the Tasks repository

corvint-tasks MUST release linux/amd64, linux/arm64 and darwin/arm64 binaries with checksums and a
build attestation from its own repository, and the Tasks tag MUST precede the Corvint RC. (G9;
TIN-V1-003; REL-V1-008)

#### Scenario: Tag order

- **GIVEN** no corvint-tasks 1.0.x tag
- **WHEN** the Corvint RC is prepared
- **THEN** the RC check refuses with `tasks-tag-missing`
