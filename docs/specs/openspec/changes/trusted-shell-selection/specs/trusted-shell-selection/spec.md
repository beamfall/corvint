# Spec delta: `trusted-shell-selection` (change `trusted-shell-selection`)

## ADDED Requirements

### Requirement: CIN-V0-8724 The shell-test planner and selector come from the event base

On a pull request or merge-queue run, the `shell-test-selected` job MUST build `corvint` and
`tools/shell-test-select` from the event base in a worktree outside the tested checkout, stamped
with `-buildvcs=true`, and MUST plan the tested tree with that `corvint`
(`affected --accept-stale-binary`), admitting a plan with no degradation or exactly
`self-binary-stale` and treating any other degradation as FULL. It MUST NOT build or run the
candidate's planner or selector to choose steps. When the event base cannot be checked out or does
not build either tool, the job MUST run every shell-test step of both `Makefile`s that the candidate
defines and print a warning naming the event base. (V1-5097 criterion 1)

#### Scenario: The candidate edits the selector

- **GIVEN** a pull request whose `tools/shell-test-select` prints no step
- **WHEN** `shell-test-selected` runs
- **THEN** the steps the event base's selector picks for the plan run, and the summary names the
  event-base selector

#### Scenario: The base does not build

- **GIVEN** an event base whose `cmd/corvint` does not compile
- **WHEN** `shell-test-selected` runs
- **THEN** every shell-test step runs and a warning names the event base

### Requirement: CIN-V0-8725 The shell steps run are the union of the base and candidate lists

The job MUST run every step the event-base selector picks from the event base's `shell-test:` list
or from the candidate's list that the candidate `Makefile` defines. When a step picked from the base
list is not defined by the candidate, the job MUST run every shell-test step of both lists that the
candidate defines and MUST print a warning and a summary line naming each missing step. It MUST NOT
fail for that reason alone. (V1-5097 criteria 1 and 3)

#### Scenario: A step is dropped from the list

- **GIVEN** a pull request that removes `foo-test` from the `shell-test:` prerequisites but keeps
  its target
- **WHEN** the plan selects `foo-test`
- **THEN** `foo-test` runs

#### Scenario: A step is deleted

- **GIVEN** a pull request that deletes the `foo-test` target the base list names
- **WHEN** the plan selects `foo-test`
- **THEN** every shell-test step the candidate defines runs and a warning names `foo-test`

### Requirement: CIN-V0-8726 doc-gates also runs the base target list

On a pull request or merge-queue run the `doc-gates` job MUST, after `make doc-gates`, read
`DOC_GATE_TARGETS` from the event base's `Makefile` and run by name every listed gate that the
candidate's list leaves out and the candidate still defines. A base gate the candidate no longer
defines MUST be named in a warning and the summary. (V1-5097 criterion 2)

#### Scenario: A gate is dropped from the list

- **GIVEN** a pull request that removes `line-citations-check` from `DOC_GATE_TARGETS`
- **WHEN** `doc-gates` runs
- **THEN** `line-citations-check` still runs and a failure fails the job

### Requirement: CIN-V0-8727 Fault injection proves the trusted selection without a hosted run

An extract-and-run test MUST run the two steps' scripts from `ci.yml` in a fixture repository and
show: a candidate selector that selects nothing still runs the base-selected step; a dropped step
and a dropped gate still run; a deleted step forces FULL with its warning; a base that does not
build forces FULL with its warning; and the `Makefile` list reader agrees with `shell-test-select`'s
`ReadSteps` and with `DOC_GATE_TARGETS` on the repository's own `Makefile`. It MUST fail when the
step runs the candidate-built selector. (V1-5097 criterion 3)

#### Scenario: The base workflow fails the test

- **GIVEN** the step body of `b55635262` (`make -s shell-test-affected`)
- **WHEN** the test runs it with the candidate selector that prints nothing
- **THEN** the base-selected step does not run and the test fails
