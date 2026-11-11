# Spec delta: affected-change-status

The lane copies each statement below into `affected-plan-selection-v0.md`, inside its
requirements section, as a numbered definition line that begins
`(proposed 2026-11-11; delivery not-started; V1-5193, V1-5194)`. It also replaces the sentence in
AFP-V0-9133 that says narrowing "is not admitted here" with a pointer to AFP-V0-9962, and the
words "selected on every dirty path" in AFP-V0-9134 with a pointer to AFP-V0-9963.

## ADDED Requirements

### Requirement: AFP-V0-9960 Change status is a plan input

The affected planner MUST accept, for each dirty path, one change status against the base:
`added`, `deleted`, `type-changed`, `modified` or `unknown`. A rename MUST be reported as
`deleted` for the old path and `added` for the new path. `modified` MUST mean a regular file at
both sides; a changed symlink or submodule entry, which can redirect a listing, MUST be
`type-changed`. A path whose status is absent or cannot
be established MUST be `unknown`. `corvint affected --base` MUST compute the status from the base
tree and pass it to the planner.

#### Scenario: Untracked file

- **GIVEN** a file that is absent at the base and untracked in the worktree
- **WHEN** `corvint affected --base` plans the change
- **THEN** the path's status is `added`

#### Scenario: No base

- **GIVEN** a caller that passes dirty paths with no status
- **WHEN** the planner selects
- **THEN** every path's status is `unknown` and the plan equals the plan before this change

### Requirement: AFP-V0-9961 Name-listing reads are recorded apart from content reads

The shell plugin MUST record a name-listing read, separate from a content read, for a directory
operand of a command whose output depends only on entry names and kinds. The admitted forms are a
closed list: `ls` with no flags or only `-1`, `-a`, `-A` (direct children); the same with `-R`
(all descendants); `find` with an empty expression or only `-name`, `-iname`, `-path`, `-type`,
`-maxdepth`, `-mindepth`, `-print`, `-print0` with literal arguments; and `tree` with no flags.
Every other form MUST stay a content read. When a unit holds a content read and a name-listing
read of the same directory, the content read MUST decide.

#### Scenario: Long listing is a content read

- **GIVEN** a shell test that runs `ls -l script | wc -l`
- **WHEN** the plugin builds the unit
- **THEN** the unit has a content read of `script/` and no name-listing read

#### Scenario: Plain listing is a name-listing read

- **GIVEN** a shell test that runs `ls script | wc -l`
- **WHEN** the plugin builds the unit
- **THEN** the unit has a name-listing read of the direct children of `script/`

### Requirement: AFP-V0-9962 A name-listing read selects only on a status that can change a listing

A name-listing read MUST select its unit for a dirty path within the read's depth whose status is
`added`, `deleted`, `type-changed` or `unknown`, and MUST NOT select it for a path whose status
is `modified`.

#### Scenario: Edit of an existing file

- **GIVEN** a test whose only read of `script/` is `ls script | wc -l`
- **WHEN** an existing file in `script/` is edited
- **THEN** the test is not selected by that read

#### Scenario: Added file

- **GIVEN** the same test
- **WHEN** a file is added to `script/`
- **THEN** the test is selected

#### Scenario: Unknown status

- **GIVEN** the same test and a dirty path in `script/` with status `unknown`
- **WHEN** the planner selects
- **THEN** the test is selected

### Requirement: AFP-V0-9963 A name-only unbounded walker follows the same rule

An unbounded reader with the reason `shell:tree-walk` whose every walking command is an admitted
name-only form MUST be selected for a dirty path only when the path's status is `added`,
`deleted`, `type-changed` or `unknown`. A unit with at least one content-searching walker MUST be
selected on every dirty path.

#### Scenario: Name walker and an edit

- **GIVEN** a test whose only walker is `find . -name '*.md'`
- **WHEN** an existing Go file is edited
- **THEN** the test is not selected by the walker

#### Scenario: Content searcher and an edit

- **GIVEN** a test that runs `grep -rn TODO .`
- **WHEN** an existing Go file is edited
- **THEN** the test is selected

### Requirement: AFP-V0-9964 The graph cache carries name-listing reads under a new version

The graph cache MUST store name-listing reads, and its version MUST change with this field. A
cache written under an older version MUST be rebuilt and MUST NOT be read as holding no
name-listing reads. Change status MUST NOT be cached.

#### Scenario: Older cache

- **GIVEN** a graph cache written before this change
- **WHEN** the planner loads it
- **THEN** the cache is refused by version and the graph is rebuilt
