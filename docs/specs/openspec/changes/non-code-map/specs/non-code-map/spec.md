# Spec delta: `non-code-map` (change `non-code-map`)

## ADDED Requirements

### Requirement: NCM-V1-001 Full by default

A changed non-code path that no runner attributes and no proven, accepted entry covers MUST select
every test of every runner in the repository, with reason `unowned-path` and the path printed. A
non-code path a Go rule already attributes is owned and MUST NOT force a full run: it selects its
attribution (the enclosing Go package directory owns the file, so its tests and dependents are
selected; go.mod/go.sum readers; a package's testdata; a declared Go reader). A runner that was
removed or is unavailable MUST select all tests, with reason `runner-removed` and the runner or path
printed. Paths declared as documentation keep the `DECLARED_DOCUMENTATION` class and add no test.
(G15b, G2; owner decision R31 (2026-11-09); owner decision R32 (2026-11-09); orchestrator ruling
2026-11-11, `docs/build-log/2026-11-11-ncm-owned-paths.md`)

#### Scenario: Fixture change, no entry

- **GIVEN** a change to `tests/fixtures/users.json` that no Go package encloses or names, no runner
  owns, and no map entry covers
- **WHEN** `corvint affected --base FULL_SHA` runs
- **THEN** every test is selected and the reason names the path

#### Scenario: Unowned path

- **GIVEN** a change to `tools/notes.xyz` that no runner attributes and no map entry covers
- **WHEN** `corvint affected --base FULL_SHA` runs
- **THEN** every test is selected with reason `unowned-path` naming the path

#### Scenario: Data beside a package

- **GIVEN** a change to `internal/app/data.json`, which the Go package `internal/app` encloses
- **WHEN** `corvint affected --base FULL_SHA` runs
- **THEN** `internal/app` and its dependents are selected and no full run is added for that path

#### Scenario: Declared documentation

- **GIVEN** a change to a path listed in `.corvint/documentation-paths.json` only
- **WHEN** `corvint affected --base FULL_SHA` runs
- **THEN** no test is added for that path

### Requirement: NCM-V1-002 Map entry record

An entry MUST hold `path` (or `glob`), `tests`, `evidence`, `accepted_by`, `accepted_at`,
`pinned_commit` and `proof` (the identifier of the fault case that proved it). (G15b)

#### Scenario: Missing proof

- **GIVEN** an entry without `proof`
- **WHEN** the map is read
- **THEN** the entry is reported `proposed` and narrows nothing

### Requirement: NCM-V1-003 Recordings only propose

Entries derived from recorded test reads MUST be written as proposals to stdout and MUST NOT narrow
until a person accepts them. (G15b)

#### Scenario: Proposal output

- **GIVEN** a recorded run that read `config/app.yml` from three tests
- **WHEN** a proposal is requested
- **THEN** stdout holds a proposed entry and the map file is unchanged

### Requirement: NCM-V1-004 Narrowing needs a caught injected fault

An entry MUST narrow only if its `proof` names a fault case in the FIH-V1 non-code set where a fault
in the mapped file was killed and the killing test is within the entry's `tests`. (G15b, G3)

#### Scenario: Fault not caught

- **GIVEN** an accepted entry whose proving fault killed no test
- **WHEN** the map is read
- **THEN** the entry does not narrow and is reported `unproven`

### Requirement: NCM-V1-005 Paths never narrowed

The following MUST always select everything, whatever the map says: `go.mod`, `go.sum` and every
other lockfile; the toolchain pin; `.github/**`; `Makefile`; runner configuration; release
configuration; generator inputs; build-tag inputs; snapshot update paths; the map file itself;
`.corvint/test-read-scopes.json`; secret and environment configuration. A `go:embed` input is owned
by its embedding package (NCM-V1-001) and selects that package and its dependents. (G15b;
orchestrator ruling 2026-11-11)

#### Scenario: Lockfile

- **GIVEN** a change to `package-lock.json` and an entry claiming two tests
- **WHEN** the plan is computed
- **THEN** every test is selected with reason `never-narrowed-path`

### Requirement: NCM-V1-006 Corvint configuration runs everything

A change to a Corvint configuration file under `.corvint/`, including the map file, MUST select
everything, with reason `corvint-config` and the path printed. Configuration means every file under
`.corvint/` except: the flow file (`.corvint/flows.json`, or the deprecated `.corvint/e2e-flows.json`) and its link
records, whose change MUST select the tests linked to the changed flows; and the two private ledgers `self-observations.jsonl` and
`unplanned-reads.jsonl`, whose change MUST add no test. (G16, narrowed by owner decision R31
(2026-11-09))

Earlier wording, superseded by R31: "A change to any `.corvint/` declaration or to the map file MUST
select everything."

#### Scenario: Map edit

- **GIVEN** a change that adds one map entry
- **WHEN** the plan is computed for that change
- **THEN** every test is selected with reason `corvint-config`

#### Scenario: Flow file edit

- **GIVEN** a change to `.corvint/flows.json` that edits one flow linked to two tests
- **WHEN** the plan is computed for that change
- **THEN** exactly those two tests are added for that path

#### Scenario: Ledger append

- **GIVEN** a change that only appends to `.corvint/self-observations.jsonl`
- **WHEN** the plan is computed for that change
- **THEN** no test is added for that path

### Requirement: NCM-V1-007 Versioned and strict

The map MUST carry the profile version `corvint-data-map/0` (review docs F17: the first version is
`/0`, as for every other declaration new in 1.0), and a reader MUST refuse an unknown version or
unknown member with `data-map-invalid`. (G15b; strictness mirrors `internal/gateselect/readers.go:241`)

#### Scenario: Newer version

- **GIVEN** `corvint-data-map/1`
- **WHEN** a 1.0 reader loads it
- **THEN** it exits 2 with `data-map-invalid`

### Requirement: NCM-V1-008 Stale entries stop narrowing

An entry whose cited test is gone, or whose `pinned_commit` is not an ancestor of the head, MUST NOT
narrow and MUST be reported `stale`. (G15b)

#### Scenario: Deleted test

- **GIVEN** an entry citing a test that was removed
- **WHEN** the plan is computed
- **THEN** the entry is `stale` and the path selects everything

### Requirement: NCM-V1-009 Deterministic and read-only

Reading the map and planning with it MUST write nothing and MUST give byte-identical output for the
same inputs. (invariant 4; G15b)

#### Scenario: Repeated run

- **GIVEN** the same commit and map
- **WHEN** the plan is computed twice
- **THEN** both outputs are identical and the worktree is unchanged
