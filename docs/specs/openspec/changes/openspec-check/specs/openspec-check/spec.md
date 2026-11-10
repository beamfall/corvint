# Spec delta: `openspec-check` (change `openspec-check`)

## ADDED Requirements

### Requirement: OSC-V1-001 Change folder layout

Each active change folder directly under `docs/specs/openspec/changes/` (other than `archive/`) MUST
hold `proposal.md` (Why, What Changes, Impact, Non-goals, and the intent and delivery status lines),
`design.md` (Decisions, Failure modes, Acceptance evidence, Rollback), `tasks.md` (numbered checkbox
items, each naming at least one requirement ID and a proof) and at least one
`specs/<capability>/spec.md` with ADDED, MODIFIED or REMOVED Requirements, each written as
`### Requirement: <ID> <title>` with at least one `#### Scenario:` block or, for REMOVED, a reason.
(G14; PROD-INT-011)

#### Scenario: No design file

- **GIVEN** an active change folder without `design.md`
- **WHEN** the check runs
- **THEN** it fails and names the folder and the missing file

#### Scenario: A requirement without a scenario

- **GIVEN** an ADDED requirement with no `#### Scenario:` block
- **WHEN** the check runs
- **THEN** it fails and names the requirement ID

### Requirement: OSC-V1-002 Requirement IDs in deltas

An ADDED requirement ID MUST NOT exist in `docs/specs/REQUIREMENTS.tsv` or in any other active or
archived change folder. A MODIFIED or REMOVED requirement ID MUST exist in `REQUIREMENTS.tsv`. A
retired ID MUST NOT be added again. (G14; owner decision R11 (2026-11-09) on fresh IDs)

#### Scenario: An ADDED ID already in use

- **GIVEN** a delta that adds an ID present in `REQUIREMENTS.tsv`
- **WHEN** the check runs
- **THEN** it fails and names the ID and the spec that holds it

### Requirement: OSC-V1-003 Commits bind to a change

Every non-merge commit in the checked range that touches a path outside `docs/` MUST carry either a
`Change: <name>` trailer naming an active or archived change folder, or a `Requirement: <ID>` trailer
naming an existing accepted requirement. (owner decision R41 (2026-11-09); PROD-INT-011)

#### Scenario: A code commit with no trailer

- **GIVEN** a commit changing a Go file with neither trailer
- **WHEN** the check runs over a range that contains it
- **THEN** it fails and names the commit and the paths

### Requirement: OSC-V1-004 Propose stage: the folder comes first

For every commit that belongs to a change and touches a path outside `docs/`, the change folder MUST
exist as an active folder in that commit's first parent. (owner decision R41 (2026-11-09); G14)

#### Scenario: Folder and code in one commit

- **GIVEN** a commit that adds `docs/specs/openspec/changes/x/` and a Go file with `Change: x`
- **WHEN** the check runs
- **THEN** it fails with the reason that the proposal was not committed before the code

#### Scenario: Folder first

- **GIVEN** a commit that adds the folder and a later commit with `Change: x` that adds code
- **WHEN** the check runs
- **THEN** the propose-stage rule passes

### Requirement: OSC-V1-005 Apply stage: tasks in order, canonical spec in the same change

In `tasks.md`, a ticked item MUST NOT follow an unticked item. For every ticked item, each ADDED or
MODIFIED requirement ID it names MUST appear in a canonical spec directly under `docs/specs/` at the
range head, and each REMOVED ID MUST be marked retired there, within the same range as the change's
code commits. (owner decision R41 (2026-11-09); G14)

#### Scenario: Out-of-order tick

- **GIVEN** `tasks.md` with 2.1 unticked and 2.2 ticked
- **WHEN** the check runs
- **THEN** it fails and names task 2.2

#### Scenario: Code without the canonical spec

- **GIVEN** a ticked task naming an ADDED ID and code commits for the change in the range
- **WHEN** no canonical spec under `docs/specs/` holds the ID at the range head
- **THEN** the check fails and names the ID

### Requirement: OSC-V1-006 Archive stage

A change whose tasks are all ticked and whose every delta ID is present in the canonical specs MUST
be moved to `docs/specs/openspec/changes/archive/YYYY-MM-DD-<name>/` by a pure rename, dated with the
archive commit's UTC date. The check MUST fail when a finished change is still under `changes/` at
the range head, and when any commit after the archive commit changes a file in an archived folder.
(owner decision R41 (2026-11-09))

#### Scenario: A finished change left in place

- **GIVEN** an active folder with every task ticked and every delta ID in the canonical specs
- **WHEN** the check runs
- **THEN** it fails and names the archive path the folder must move to

#### Scenario: An edited archive

- **GIVEN** an archived folder
- **WHEN** a later commit changes its `tasks.md`
- **THEN** the check fails and names the commit

### Requirement: OSC-V1-007 G14 boundaries

The check MUST fail if the repository root holds an `openspec/` directory or an `.openspec.yaml`
file, or if a Makefile target or a workflow under `.github/workflows/` runs an `openspec` command.
`interop/spec-toolchains/` MUST be excluded. Documentation MUST NOT claim OpenSpec CLI
compatibility. (G14)

#### Scenario: A root openspec directory

- **GIVEN** a commit that adds `openspec/changes/x/proposal.md` at the repository root
- **WHEN** the check runs
- **THEN** it fails and points to `docs/specs/openspec/changes/`

### Requirement: OSC-V1-008 Spec health class in each canonical spec header

Each spec directly under `docs/specs/` MUST carry a header line `Health:` with one of `KEEP`,
`MERGE`, `RETIRE` or `OUT-OF-1.0`. A `RETIRE` or `OUT-OF-1.0` spec MUST also carry a retired or
outside-1.0 status line and stays in place. `make spec-requirements` MUST project the class into
`INDEX.json` and the README table. (owner decision R37 (2026-11-09); PROD-INT-010)

#### Scenario: A spec with no class

- **GIVEN** a spec header without a `Health:` line
- **WHEN** the check runs
- **THEN** it fails and names the spec

### Requirement: OSC-V1-009 Accepted intent needs a log entry

A proposal whose header says the intent is accepted MUST have a matching entry in the acceptance log
defined by the change `acceptance-record`. Without an entry, the intent MUST read "proposed" and the
delivery "not-started" or "experimental". (G14)

#### Scenario: Self-declared acceptance

- **GIVEN** a proposal header "Intent status: accepted" and no acceptance log entry for it
- **WHEN** the check runs
- **THEN** it fails and names the proposal

### Requirement: OSC-V1-010 Generated header rows and acceptance mode carried over

`REQUIREMENTS.tsv`, `INDEX.json` and the README table MUST stay generated from spec headers and
checked for staleness (SRG-V0-012), and each requirement MUST keep its recorded acceptance mode with
the audit command `make spec-acceptance-audit` (SRG-V0-009, SRG-V0-010). (owner decision R37
(2026-11-09): PRS-V1-018 (c) and (d))

#### Scenario: A hand-edited index

- **GIVEN** an `INDEX.json` edited by hand so it differs from the generator output
- **WHEN** `make spec-requirements-check` runs
- **THEN** it fails

### Requirement: OSC-V1-011 Runs in CI and fails closed

The check MUST run as the Make target `openspec-check` inside `doc-gates` and the `focused-docs`
gate. When it cannot read the history it needs, it MUST refuse with `history-unavailable` and a
non-zero exit. Without a range it MUST report the stage rules as NOT_RUN. Folders listed in the
committed baseline file are reported but do not fail; the baseline MUST only shrink. (G14)

#### Scenario: A shallow clone

- **GIVEN** a depth-1 checkout and a range of ten commits
- **WHEN** the check runs
- **THEN** it exits non-zero with `history-unavailable`

## MODIFIED Requirements

### Requirement: PRS-V1-018 Spec-driven development improvements in 1.0

Stable 1.0 MUST ship two spec-driven development improvements: (c) spec header rows generated by
`make spec-requirements` (delivered by SRG-V0-012), and (d) a recorded acceptance mode per
requirement carried by `REQUIREMENTS.tsv`, with one audit command listing the requirements the owner
never read back (delivered by SRG-V0-009 and SRG-V0-010). Clauses (a) record citation class,
(b) dogfood-check OCM coverage line and (e) `dogfood finish-lane` are dropped. (Changed by owner
decision R37 (2026-11-09) and G8; tickets V1-1344, V1-1345 and V1-1348 close as superseded.)

#### Scenario: A dropped clause

- **GIVEN** the 1.0 release object after M0
- **WHEN** its criteria are listed
- **THEN** none requires the record citation class, the OCM coverage line or `dogfood finish-lane`
