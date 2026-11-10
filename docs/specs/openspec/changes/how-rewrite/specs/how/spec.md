# Spec delta: `how` (change `how-rewrite`)

## ADDED Requirements

### Requirement: HOW-V1-001 corvint-docs owns the verb

`how` MUST run in corvint-docs, and `corvint how` MUST only forward its argument vector unchanged to
corvint-docs. (decision 0595, CMS-V0-912/913; G5)

#### Scenario: Forwarding

- **GIVEN** corvint and corvint-docs installed
- **WHEN** `corvint how --map m.json --source e2e "sign in"` runs
- **THEN** corvint-docs receives the same arguments and its stdout and exit code are returned as-is

### Requirement: HOW-V1-002 Argument vector kept

The flags `--root`, `--map`, `--source [ALIAS:]DIR` (repeatable), `--repo ALIAS=ABSOLUTE_ROOT`
(repeatable), `--condition TEXT` (repeatable), `--ledger`, `--corpus`, `--budget N`, `--full`,
`--workspace FILE` and `--no-workspace`, and exactly one GOAL, MUST keep their current spelling and
meaning. (decision 0595; SKH-V0 IDs cited by `flows_appmap_how.go`)

#### Scenario: Recorded vectors

- **GIVEN** the golden argument vectors recorded at the base commit
- **WHEN** they run after any later change
- **THEN** each gives the same exit code and the same bytes

### Requirement: HOW-V1-003 Alias equivalence

`corvint-docs how ARGS` and `corvint-docs flows appmap how ARGS` MUST give identical output bytes
and exit codes. (SKH-V0-810)

#### Scenario: Same goal

- **GIVEN** one map, source and goal
- **WHEN** both forms run
- **THEN** their outputs are byte-identical

### Requirement: HOW-V1-004 Output schema kept

Output MUST be `site-knowledge-how/0` with states COMPLETE, INCOMPLETE, STALE and ABSTAINED, the
six gap codes listed above, and the proposal label. A schema change MUST use a new profile version.
(SKH-V0-063, SKH-V0-144)

#### Scenario: Missing oracle

- **GIVEN** a goal whose route has no assertion
- **WHEN** `how` runs
- **THEN** the state is INCOMPLETE with gap `no-oracle`

### Requirement: HOW-V1-005 Refusals exit 2

Invalid arguments, an oversized goal (over 256 bytes), unknown input profiles and an unbound source
alias MUST be refused with exit 2 and a typed message. (decision 0595; SKH-V0)

#### Scenario: Long goal

- **GIVEN** a 257-byte goal
- **WHEN** `how` runs
- **THEN** it exits 2 and prints nothing to stdout

### Requirement: HOW-V1-006 Read-only and deterministic

`how` MUST write no file and MUST give byte-identical output for the same inputs. (invariant 4;
`HowHelp`)

#### Scenario: Repeat

- **GIVEN** the same inputs
- **WHEN** `how` runs twice
- **THEN** outputs are identical and the worktree is unchanged

### Requirement: HOW-V1-007 Missing or old corvint-docs

`corvint how` MUST exit 2 with `NOT_INSTALLED` when corvint-docs is absent, and MUST pass through
the argument error of an older corvint-docs without `how`; it MUST refuse `--require-clean`.
(CMS-V0-913)

#### Scenario: Not installed

- **GIVEN** corvint alone on PATH
- **WHEN** `corvint how --map m.json --source e2e "x"` runs
- **THEN** it exits 2 with `NOT_INSTALLED` naming corvint-docs

### Requirement: HOW-V1-008 Help names the owner

`corvint how --help` and `corvint-docs how --help` MUST name corvint-docs as the owning binary and
show the usage line above. (decision 0595)

#### Scenario: Help text

- **GIVEN** both binaries
- **WHEN** help is requested from each
- **THEN** both show the corvint-docs usage line

### Requirement: HOW-V1-009 No new import edge

The `how` code path MUST import only the shared readers allowed by CMS-V0-915 and MUST NOT add an
edge to the corvint test layer; the import baseline MUST not grow. (CMS-V0-915/916)

#### Scenario: Import ratchet

- **GIVEN** this change applied
- **WHEN** `TestImportRatchet` runs
- **THEN** it passes with the baseline unchanged

### Requirement: HOW-V1-010 Boundary with native flow reports

`how` MUST stay in corvint-docs while gaps, status, impact and test-evidence reports become native
in corvint, and each verb MUST have exactly one home: with corvint installed alone, the four native
reports MUST run and `corvint how` MUST exit 2 with `NOT_INSTALLED`. (G5; review docs F6)

The earlier clause "and `how` MUST refuse a flow file whose schema version it does not know", with
its scenario "Unknown flow schema", is withdrawn: `how` reads no flow file, so the clause was not
testable. Flow-file versioning belongs to change `flow-schema-version`.

#### Scenario: corvint installed alone

- **GIVEN** corvint on PATH, corvint-docs absent, and a repository with `.corvint/flows.json`
- **WHEN** `corvint flows status` and `corvint how --map m.json --source e2e "x"` run
- **THEN** `flows status` exits 0 with a report, and `how` exits 2 with `NOT_INSTALLED` naming
  corvint-docs

### Requirement: HOW-V1-011 One MCP home and one table row

If `how` is offered over MCP, it MUST be served only by `corvint-docs mcp`, with the tool name, input
schema, output profile and error codes given by its row in the table of change `mcp-tool-table`, and
`corvint mcp` MUST NOT serve it. (owner decision R30, 2026-11-09; review docs F4)

#### Scenario: Testing server

- **GIVEN** both servers running
- **WHEN** a client lists the tools of `corvint mcp`
- **THEN** no `how` tool is listed, and any `how` tool on `corvint-docs mcp` matches its table row
