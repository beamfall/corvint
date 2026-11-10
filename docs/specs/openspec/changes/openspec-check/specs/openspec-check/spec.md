# Spec delta: `openspec-check` (change `openspec-check`)

## ADDED Requirements

### Requirement: OSC-V1-001 Change folder layout

Each active change folder directly under `docs/specs/openspec/changes/` (other than `archive/`) MUST
hold `proposal.md` (Why, What Changes, Impact, Non-goals, and the intent and delivery status lines),
`design.md` (Decisions, Failure modes, Acceptance evidence, Rollback), `tasks.md` (numbered checkbox
items, each naming at least one requirement ID and a proof) and at least one
`specs/<capability>/spec.md` with ADDED, MODIFIED or REMOVED Requirements, each written as
`### Requirement: <ID> <title>` with at least one `#### Scenario:` block or, for REMOVED, a reason.
The folder MAY hold the `.openspec.yaml` file that `openspec new change` writes. Whether the folder
passes the OpenSpec command is checked by OSC-V1-012, not by this rule.
(G14; PROD-INT-011; owner decision H12 (2026-11-10))

#### Scenario: No design file

- **GIVEN** an active change folder without `design.md`
- **WHEN** the check runs
- **THEN** it fails and names the folder and the missing file

#### Scenario: A requirement without a scenario

- **GIVEN** an ADDED requirement with no `#### Scenario:` block
- **WHEN** the check runs
- **THEN** it fails and names the requirement ID

### Requirement: OSC-V1-002 Requirement IDs in deltas

An ADDED requirement ID MUST NOT exist in `docs/specs/REQUIREMENTS.tsv` at the commit before the
change folder was first added, or in any other active or archived change folder. A definition that
the change itself adds during its apply stage (OSC-V1-005) is allowed. A MODIFIED or REMOVED requirement ID MUST exist in `REQUIREMENTS.tsv`. A
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
code commits. When the proposal header names a canonical spec in another repository (for example
`beamfall/corvint-tasks`), this repository's check MUST NOT look for the IDs under `docs/specs/`:
it MUST require each ticked item to name the other repository's commit that defines them, and
report the item as `external-spec-not-verified`. That status is accepted as a reported status that
does not fail the check: it is listed in the output with the item and the named commit, and it is
never counted as a pass. A ticked item that names no commit in the other repository MUST fail. The
same check run in the owning repository verifies the IDs there. (owner decision R41 (2026-11-09);
G14; owner decision H9 (2026-11-10))

#### Scenario: Out-of-order tick

- **GIVEN** `tasks.md` with 2.1 unticked and 2.2 ticked
- **WHEN** the check runs
- **THEN** it fails and names task 2.2

#### Scenario: Code without the canonical spec

- **GIVEN** a ticked task naming an ADDED ID and code commits for the change in the range
- **WHEN** no canonical spec under `docs/specs/` holds the ID at the range head
- **THEN** the check fails and names the ID

#### Scenario: A requirement owned by another repository

- **GIVEN** a proposal header naming a canonical spec in `beamfall/corvint-tasks` and a ticked task
  that names the commit there which defines its IDs
- **WHEN** the check runs in this repository
- **THEN** it reports the item as `external-spec-not-verified`, lists the named commit, and exits
  zero when nothing else fails

### Requirement: OSC-V1-006 Archive stage

A change whose tasks are all ticked and whose every delta ID is present in the canonical specs MUST
be moved to `docs/specs/openspec/changes/archive/YYYY-MM-DD-<name>/` by a pure rename. The move is
made with `openspec archive <name> --skip-specs`, run from `docs/specs`; `--skip-specs` is required
because the canonical specs are the flat files under `docs/specs/` (OSC-V1-007). The date in the
folder name is the one the command writes and MUST be within one day of the archive commit's UTC
date. The check MUST fail when a finished change is still under `changes/` at the range head, when
an archived folder holds an unticked task, and when any commit after the archive commit changes a
file in an archived folder. (owner decision R41 (2026-11-09); owner decision H12 (2026-11-10))

#### Scenario: A finished change left in place

- **GIVEN** an active folder with every task ticked and every delta ID in the canonical specs
- **WHEN** the check runs
- **THEN** it fails and names the archive path the folder must move to

#### Scenario: An edited archive

- **GIVEN** an archived folder
- **WHEN** a later commit changes its `tasks.md`
- **THEN** the check fails and names the commit

#### Scenario: Archived with an open task

- **GIVEN** a folder under `changes/archive/` whose `tasks.md` holds an unticked item
- **WHEN** the check runs
- **THEN** it fails and names the folder and the task

### Requirement: OSC-V1-007 One OpenSpec root under `docs/specs/`

The repository MUST hold exactly one OpenSpec root: `docs/specs/openspec/`, with a `config.yaml`
that names the schema `spec-driven`, beside `changes/`. The check MUST fail if that `config.yaml` is
missing or names another schema, or if the repository root holds an `openspec/` directory or an
`.openspec.yaml` file. `interop/spec-toolchains/` MUST be excluded. The canonical specs stay the
flat files directly under `docs/specs/`: the check MUST fail if `docs/specs/openspec/specs/` exists,
because a second set of canonical specs would then exist. Documentation MAY say that the change
folders pass `openspec validate`; it MUST NOT say that `openspec archive` without `--skip-specs` is
supported. (G14; owner decision H12 (2026-11-10))

#### Scenario: A root openspec directory

- **GIVEN** a commit that adds `openspec/changes/x/proposal.md` at the repository root
- **WHEN** the check runs
- **THEN** it fails and points to `docs/specs/openspec/changes/`

#### Scenario: No config file

- **GIVEN** a tree without `docs/specs/openspec/config.yaml`
- **WHEN** the check runs
- **THEN** it fails and names the missing file

#### Scenario: A second set of canonical specs

- **GIVEN** a commit that adds `docs/specs/openspec/specs/watch/spec.md`
- **WHEN** the check runs
- **THEN** it fails and says the canonical spec belongs directly under `docs/specs/`

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

### Requirement: OSC-V1-012 The OpenSpec command validates every change folder

The Make target `openspec-validate` MUST run `openspec validate --all` from `docs/specs` with the
OpenSpec command (`@fission-ai/openspec`) at the one version pinned in the Makefile, with telemetry
off (`OPENSPEC_TELEMETRY=0`), and MUST fail when any change folder fails validation. Warnings from
`--strict` are reported and do not fail the target.

- Locally, when the command is absent or its version differs from the pin, the target MUST report
  NOT_RUN with the reason and exit zero.
- In CI, an absent command or a version mismatch MUST fail the job.
- The Go check (OSC-V1-001 to OSC-V1-011) MUST NOT need the command, the network or npm.
- The default product build MUST NOT gain an npm dependency: the command is a developer and CI
  tool only, and no shipped binary calls it.

(owner decision H12 (2026-11-10); G14)

#### Scenario: A folder the command refuses

- **GIVEN** a change folder whose delta has a requirement with no scenario
- **WHEN** `make openspec-validate` runs with the pinned command installed
- **THEN** it exits non-zero and prints the command's finding for that folder

#### Scenario: The command is not installed locally

- **GIVEN** a developer machine without the OpenSpec command
- **WHEN** `make openspec-validate` runs outside CI
- **THEN** it prints NOT_RUN with the reason and exits zero

#### Scenario: The command is missing in CI

- **GIVEN** a CI job in which the OpenSpec command is absent or at another version
- **WHEN** `make openspec-validate` runs
- **THEN** it exits non-zero

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
