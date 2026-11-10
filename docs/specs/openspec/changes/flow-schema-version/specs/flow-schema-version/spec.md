# Spec delta: `flow-schema-version` (change `flow-schema-version`)

## ADDED Requirements

### Requirement: FSV-V1-001 The profile carries the schema version

`.corvint/flows.json` MUST carry its schema version in the `profile` member as `corvint-flows/N`,
where N is a whole number written without leading zeros. A file with profile `corvint-flows/0` MUST
stay valid as version 0. (G5; PROD-INT-016)

#### Scenario: Today's file

- **GIVEN** a flow file with profile `corvint-flows/0` and only today's members
- **WHEN** the reader runs
- **THEN** it accepts the file as version 0

### Requirement: FSV-V1-002 A closed list of supported versions; any member change increments N

Each reader build MUST hold one closed list of supported versions. Adding, removing or renaming a
member, or changing a member's meaning, MUST increment N. (G5)

#### Scenario: A new member without a new version

- **GIVEN** a change that adds a member to the flow file but keeps the same N
- **WHEN** the member-list test compares the reader's members for that N with the member list
  recorded for N when it was released
- **THEN** the test fails, names the added member and says to increment N

### Requirement: FSV-V1-003 One shared reader

Only `internal/appflows` MUST parse `.corvint/flows.json`. The testing product and the documentation
product MUST both read the file through it. (G5; PROD-INT-004)

#### Scenario: A second parser

- **GIVEN** a package outside `internal/appflows` that opens `.corvint/flows.json`
- **WHEN** the source scan test runs
- **THEN** it fails and names the package

### Requirement: FSV-V1-004 An unsupported version fails closed with a typed reason

When the profile has the `corvint-flows/` prefix and a well-formed N outside the supported list, the
reader MUST refuse with `FLOWS_SCHEMA_UNSUPPORTED`, naming the found version and the supported list.
The affected plan MUST then select every E2E test with that reason. Unit and integration selection
MUST be unchanged. (G5; PROD-INT-016)

#### Scenario: A future schema

- **GIVEN** `.corvint/flows.json` with profile `corvint-flows/7` and a reader that supports 0 and 1
- **WHEN** the user runs `corvint affected --base FULL_SHA`
- **THEN** every E2E test is selected with reason `FLOWS_SCHEMA_UNSUPPORTED`, and the reason names
  version 7 and the supported versions 0 and 1

#### Scenario: A malformed profile

- **GIVEN** a profile `flows/1`
- **WHEN** the reader runs
- **THEN** it refuses with `FLOWS_DECLARATION_INVALID`, not `FLOWS_SCHEMA_UNSUPPORTED`

### Requirement: FSV-V1-005 Flow reports refuse rather than print partial output

`corvint flows` reports and every documentation-product report that reads the flow file MUST exit
non-zero with the reader's reason and print no partial report when the reader refuses. (G5;
PROD-DOCS-011)

#### Scenario: Gaps on a newer file

- **GIVEN** a flow file with an unsupported version
- **WHEN** the user runs `corvint flows gaps`
- **THEN** the command exits non-zero, prints `FLOWS_SCHEMA_UNSUPPORTED` and lists no gaps

### Requirement: FSV-V1-006 The writer emits a version its own reader supports

Every command that writes `.corvint/flows.json` MUST write the highest version in its build's
supported list, and that build's reader MUST accept the written file. (G5)

#### Scenario: Round trip

- **GIVEN** a build of `corvint`
- **WHEN** a flow file is written and then read by the same build
- **THEN** the read succeeds with the written version

### Requirement: FSV-V1-007 Release smoke test with `corvint` installed alone

The release workflow MUST install only the `corvint` binary from the candidate archive on an otherwise
empty `PATH` and run `corvint affected` and the PROD-DOCS-011 flow reports on a pinned fixture, once
with a supported flow file and once with an unsupported version. The candidate MUST NOT be offered for
approval if either run differs from its expected output. (G5; PROD-INT-016; PROD-INT-004)

#### Scenario: Smoke on the RC

- **GIVEN** the RC archive for linux/amd64
- **WHEN** the smoke job runs
- **THEN** the supported run prints the expected plan and reports, and the unsupported run plans every
  E2E test with `FLOWS_SCHEMA_UNSUPPORTED`
