# Spec delta: `spec-set-cleanup` (SSC-V1)

Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started

## ADDED Requirements

### Requirement: SSC-V1-001 A single workflow document

`docs/SPEC-DRIVEN-DEVELOPMENT.md` SHALL be the only active document that describes how a change is
made. It SHALL describe the propose, apply and archive stages of owner decision R41 (2026-11-09) and
point to `openspec-check` (OSC-V1) for their exact rules. Every other active document that refers
to the change workflow SHALL point to it and SHALL NOT describe CEM binding, OCM maps, dogfood
sealing or lane closeout as steps of a change.

#### Scenario: a contributor looks for the change process
- **GIVEN** the cleanup batches C1 to C3 have merged
- **WHEN** the active documents are searched for the stage names propose, apply and archive
- **THEN** only `docs/SPEC-DRIVEN-DEVELOPMENT.md` describes them, and every other match is a one-line
  pointer to it

#### Scenario: an old workflow document remains
- **GIVEN** `docs/V1-WORKFLOWS.md` is still in the tree and listed as active
- **WHEN** C1 is checked
- **THEN** C1 is not finished until the file is removed and its readers are edited in the same commit

### Requirement: SSC-V1-002 Archive marking

An archived spec SHALL stay at its path and carry a `Health: RETIRE` or `Health: OUT-OF-1.0` line,
`Intent status: retired <date> (<decision or owner answer>)`, `Delivery status: retired`, a
`Retirement base:` commit and a `Retirement note:`. An archived doc SHALL stay at its path with an
`Archived:` line naming its date, its owner answer and the active document to read instead, and
SHALL be listed only under "Archived" in `docs/README.md`.

#### Scenario: a spec whose code a batch removes
- **GIVEN** removal batch B5 deletes the Swift provider
- **WHEN** B5 merges
- **THEN** `swift-affected-test-understanding-v0.md` carries the archive lines in the same merge
  commit and its body is unchanged

#### Scenario: an archived doc
- **GIVEN** `docs/PORTFOLIO-0.6.md` is archived in C1
- **WHEN** a reader opens `docs/README.md`
- **THEN** the file appears only under "Archived", and its first line after the title names the
  active document to read instead

### Requirement: SSC-V1-003 Archived requirement IDs are kept and never reused

`REQUIREMENTS.tsv` SHALL keep a row for every requirement ID of an archived spec, with its retired
status. No archived ID SHALL be added again by any change.

#### Scenario: regeneration after archiving
- **GIVEN** C1 archives a spec with ten requirement IDs
- **WHEN** `make spec-requirements` regenerates `REQUIREMENTS.tsv`
- **THEN** the ten rows remain, marked retired

#### Scenario: a change reuses an archived ID
- **GIVEN** a change folder whose spec delta ADDs an ID that belongs to an archived spec
- **WHEN** `openspec-check` runs
- **THEN** it fails and names the ID and the archived spec

### Requirement: SSC-V1-004 No active document refers to an archived or deleted one

A doc gate SHALL fail when an active document links to, or names by path, an archived or deleted
document as current guidance. Links under a heading named "Archived" or "History", or on a line that
says the target is archived, are allowed. `docs/decisions/`, `docs/build-log/`,
`docs/RELEASE-NOTES*.md`, archived documents and archived change folders are not checked. The gate
SHALL report without failing until C3 and SHALL block from C3 on.

#### Scenario: a dangling pointer
- **GIVEN** `docs/CHANGE-EVIDENCE-MAP.md` is archived in B6
- **WHEN** `docs/README.md` still lists it among the active documents
- **THEN** the gate names `docs/README.md`, the line and the archived target

#### Scenario: a history reference
- **GIVEN** a build-log entry links to `docs/CHANGE-FLOW.md`, which B6 deletes
- **WHEN** the gate runs
- **THEN** the entry is not checked and the gate passes

### Requirement: SSC-V1-005 The generated index lists only active specs by default

The generated spec index and the `docs/specs/README.md` table SHALL list only active specs (Health
KEEP or MERGE) by default, and archived specs SHALL be listed separately with their retirement
lines. Because this changes a generated format, it SHALL NOT be built before the owner accepts it
(G14); until then the index keeps its shape and adds the health class only.

#### Scenario: default listing
- **GIVEN** the owner has accepted this requirement and C1 has merged
- **WHEN** `make spec-requirements` runs
- **THEN** `INDEX.json` and the README table contain no spec whose Health is RETIRE or OUT-OF-1.0,
  and the archived listing contains each of them

#### Scenario: owner has not answered
- **GIVEN** the owner has not accepted this requirement
- **WHEN** C1 merges
- **THEN** `INDEX.json` keeps every spec and gains a `health` field per entry

### Requirement: SSC-V1-006 A spec is archived in the same change that removes its code

A spec whose implementation a `source-removal` batch deletes SHALL be archived in the same merge
commit as that batch. A spec whose code is still in the tree SHALL NOT be archived by a
cleanup-only batch. The cleanup-only batches C1 to C3 SHALL finish by 2026-12-01; rows of a batch
gated later SHALL land with that batch and SHALL NOT be moved to another batch.

#### Scenario: archiving ahead of removal
- **GIVEN** `lsp-quality-platform-v0.md` describes code that B4 removes
- **WHEN** C1 is prepared
- **THEN** the spec is not in C1; it is archived in the B4 merge commit

#### Scenario: a gated batch
- **GIVEN** B6 still waits for the companion decision
- **WHEN** the cleanup target date passes
- **THEN** the B6 rows keep their current class and the table keeps their gate

### Requirement: SSC-V1-007 Tasks specs leave only after corvint-tasks PR #4

The 37 specs classed "move to beamfall/corvint-tasks" SHALL be deleted from this repository only in
B10, after `beamfall/corvint-tasks` PR #4 merges (RMV-V1-027), and their requirement prefixes SHALL
remain unavailable to new changes.

#### Scenario: PR #4 still open
- **GIVEN** `beamfall/corvint-tasks` PR #4 is open
- **WHEN** a change proposes deleting `corvint-tasks-cli-v0.md`
- **THEN** the deletion waits and the spec stays with its retired header

### Requirement: SSC-V1-008 Rollback by git revert; nothing deleted from history

Each cleanup batch SHALL be one merge commit whose `git revert -m 1` restores the previous headers,
documents and generated files. No step SHALL rewrite history; a deleted document SHALL stay
reachable at the parent commit named in the batch's build-log entry.

#### Scenario: undo C1
- **GIVEN** C1 merged as commit X
- **WHEN** C1 must be undone
- **THEN** `git revert -m 1 X` restores the previous spec headers and documents, and
  `make spec-requirements-check` passes

#### Scenario: recover a deleted doc
- **GIVEN** `docs/V1-WORKFLOWS.md` was deleted in C1
- **WHEN** a reader needs its text
- **THEN** `git show <C1 parent>:docs/V1-WORKFLOWS.md` returns it
