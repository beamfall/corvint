# Spec delta: `user-docs` (change `user-docs`)

## ADDED Requirements

### Requirement: UDC-V1-001 First-run page per product

Each product (testing, documentation, Tasks), the GitHub Action and the agent packages MUST have a
first-run page with the install command, the first command, its expected output and the abstention
and refusal cases. (product gap (a); awaits Q35; PROD-INT-004)

#### Scenario: Documentation alone

- **GIVEN** a user with only the documentation binaries installed
- **WHEN** they follow the documentation first-run page
- **THEN** every command on the page runs and prints the output shown

### Requirement: UDC-V1-002 Configuration reference checked against code

`docs/CONFIGURATION.md` MUST list every `.corvint/` file and environment variable a shipped binary
reads in a user repository, including `CORVINT_SELECTION`. A check MUST fail when a declared constant
has no row or a row has no constant. (product gap (b); awaits Q35)

#### Scenario: A new variable

- **GIVEN** a new environment variable read by `corvint`
- **WHEN** it has no row in `docs/CONFIGURATION.md`
- **THEN** `make doc-gates` fails and names the variable

### Requirement: UDC-V1-003 Upgrade messages for removed verbs and files

Each verb removed before 1.0 (`cem`, `runs`, `test-plan`, `tests lint`, the analyzer verbs) MUST
print one line naming its replacement or "no replacement" and the `docs/UPGRADING.md` anchor, and exit
with the one code reserved for removed verbs. `docs/UPGRADING.md` MUST describe migrating
`.corvint/e2e-flows.json` and CEM files. (product gap (c); awaits Q35)

#### Scenario: An old script

- **GIVEN** a script that runs `corvint test-plan`
- **WHEN** it runs against 1.0
- **THEN** the command prints the replacement listed for `test-plan` in `docs/UPGRADING.md` (or "no
  replacement") and the anchor, and exits with the removed-verb code

### Requirement: UDC-V1-004 Error and exit-code catalogue checked against constants

`docs/ERRORS.md` MUST list every exit code and typed reason each product returns, with meaning and
user action. A check MUST fail on any difference from the reason registries. (product gap (d); awaits
Q35)

#### Scenario: A new reason

- **GIVEN** the reason `FLOWS_SCHEMA_UNSUPPORTED` added in code
- **WHEN** `docs/ERRORS.md` has no row for it
- **THEN** `make doc-gates` fails and names it

### Requirement: UDC-V1-005 Changelog and release notes per tag

`CHANGELOG.md` MUST have one section per tag, and the release workflow MUST refuse a tag whose version
has no section. (product gap (e); awaits Q35; PROD-INT-012)

#### Scenario: A tag without notes

- **GIVEN** a pushed `v1.0.0-rc.1` tag and no matching section
- **WHEN** the release workflow starts
- **THEN** it stops before building and names the missing section

### Requirement: UDC-V1-006 Pages live in the repository

User pages MUST live under `docs/` in the product repository and be readable without a site build.
(product gap (f); awaits Q35)

#### Scenario: Offline reading

- **GIVEN** a clone of the repository with no network
- **WHEN** a user opens `docs/first-run/`
- **THEN** every first-run page is present as Markdown

### Requirement: UDC-V1-007 A 1.x compatibility statement

`docs/COMPATIBILITY.md` MUST hold the owner's 1.x compatibility statement. Until the owner decides, it
MUST say the promise is not yet decided. (product gap (g); awaits Q35, NEW-PROC-3)

#### Scenario: Before the owner decides

- **GIVEN** no owner decision on the promise
- **WHEN** a user reads `docs/COMPATIBILITY.md`
- **THEN** it states that the 1.x compatibility promise is not yet decided

### Requirement: UDC-V1-008 No-telemetry statement

`docs/INSTALL.md` and each first-run page MUST state that Corvint products send no data off the
machine. (product gap (h); settled without the owner; checklist O27; PROD-INT-006)

#### Scenario: Reading the install guide

- **GIVEN** `docs/INSTALL.md`
- **WHEN** a user searches for telemetry
- **THEN** the statement is found

### Requirement: UDC-V1-009 Uninstall paragraph

`docs/INSTALL.md` MUST tell the user to remove the binaries, remove the MCP and hook entries they added
to host configuration, and optionally delete `.corvint/` and `.taskman/` from each repository. It MUST
NOT refer to a VSIX that 1.0 does not ship from this repository. (product gap (i); settled without
the owner; checklist O27; PROD-INT-006)

#### Scenario: Removing Corvint

- **GIVEN** a user who followed the uninstall paragraph
- **WHEN** they search the machine for Corvint binaries and host entries
- **THEN** none remain, and repository state is removed only if they chose to delete it

### Requirement: UDC-V1-010 `SECURITY.md` in each public repository

`beamfall/corvint-action`, `beamfall/corvint-agents`, `beamfall/corvint-tasks` and
`beamfall/corvint-vscode` MUST each carry a `SECURITY.md` derived from the root `SECURITY.md` and
naming that repository. (product gap (j); settled without the owner; checklist O28; PROD-INT-018;
owner decision R33 (2026-11-09))

#### Scenario: A new public repository

- **GIVEN** `beamfall/corvint-vscode`
- **WHEN** its default branch is read
- **THEN** `SECURITY.md` is present and names `corvint-vscode`

### Requirement: UDC-V1-011 Native Windows statement

`docs/INSTALL.md` and each first-run page MUST state that native Windows is unsupported in 1.0 and
give the WSL path. (product gap (k); answer 16; PROD-INT-015)

#### Scenario: A Windows reader

- **GIVEN** the install guide
- **WHEN** a reader looks for Windows
- **THEN** the statement and the WSL path are found

### Requirement: UDC-V1-012 Documentation checks run in `doc-gates`

The configuration, error catalogue, removed-verb and first-run staleness checks MUST run in
`make doc-gates`. (product gaps (a) to (d))

#### Scenario: Gate run

- **GIVEN** a clean checkout at the RC SHA
- **WHEN** `make doc-gates` runs
- **THEN** each of the four checks reports a result
