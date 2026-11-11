# Spec delta: traceability test gate (`TTG-V0`)

Target spec: `docs/specs/traceability-test-gate-v0.md`. Status of every requirement below:
experimental until the owner accepts the intent of `traceability-gate-upgrade`.

## ADDED Requirements

### Requirement: TTG-V0-960 Subtest citation

In a traceability row, the gate MUST treat a checked test name that is followed at once by `/` and
a requirement ID as a subtest citation `TestName/REQ-ID`. A requirement ID is the longest text at
that position matching `[A-Z][A-Z0-9]*(-[A-Z0-9]+)*-[0-9]+`. A subtest citation MUST resolve only
when `TestName` is declared (TTG-V0-001) and TTG-V0-961 holds. A `/` followed by other text MUST be
handled as before this change.

#### Scenario: A subtest citation resolves

- **GIVEN** a tracked test file whose `func TestX(` body contains the text `REQ-1`
- **WHEN** a traceability row cites `TestX/REQ-1`
- **THEN** the gate passes and counts one subtest citation

#### Scenario: A suffix that is not a requirement ID

- **GIVEN** a declared `TestX`
- **WHEN** a traceability row cites `TestX/happy_path`
- **THEN** the gate checks `TestX` only and counts no subtest citation

### Requirement: TTG-V0-961 Requirement ID in the cited test body

For a subtest citation `TestName/REQ-ID`, the gate MUST fail unless the body of at least one tracked
declaration of `TestName` contains `REQ-ID`. The body is the declaration line alone when that line
closes every brace it opens and ends on a closing brace, where a brace inside a string literal, a
rune literal or a comment that begins and ends on that line is not counted; otherwise it is the text from the
declaration line to the first later line that is exactly `}` in column 0, or to the end of the
file. The ID is contained
only when it is not directly preceded by a letter, a digit or `-` and not directly followed by a
letter or a digit. The failure line MUST name the spec path, the row line and `TestName/REQ-ID`.

#### Scenario: The cited body lacks the ID

- **GIVEN** a declared `TestX` whose body does not contain `REQ-1`
- **WHEN** a traceability row cites `TestX/REQ-1`
- **THEN** the gate fails and prints the spec path, the line and `TestX/REQ-1`

#### Scenario: A longer ID does not satisfy a shorter one

- **GIVEN** a declared `TestX` whose body contains `REQ-12` and not `REQ-1` on its own
- **WHEN** a traceability row cites `TestX/REQ-1`
- **THEN** the gate fails

### Requirement: TTG-V0-962 Removal of a requirement's last test citation

With `--range BASE..HEAD` the gate MUST compare the tree of `git merge-base BASE HEAD` (called
BASE below) with the tree of HEAD, and MUST refuse, naming both revisions, when there is none. A
requirement is cited at a tree when a traceability row of a spec that is not retired in that tree
holds its ID in full in the first cell together with at least one test name that resolves in that
tree, or when a subtest citation naming the ID resolves in that tree. The gate MUST fail for every
requirement that is cited at BASE and not cited at HEAD, unless at HEAD the ID appears in no
`docs/specs/*.md` file that is not retired, or TTG-V0-963 applies. The failure line MUST name the
requirement ID and one citation it had at BASE.

#### Scenario: The only citing test is deleted

- **GIVEN** at BASE a row for `REQ-1` cites only `TestX`, and `TestX` is declared
- **WHEN** HEAD deletes `TestX` and removes the name from the row, and `REQ-1` is still stated in
  the spec
- **THEN** the range form fails and names `REQ-1`

#### Scenario: A second citation remains

- **GIVEN** at BASE a row for `REQ-1` cites `TestX` and `TestY`
- **WHEN** HEAD deletes `TestX` and removes that name from the row
- **THEN** the range form passes

### Requirement: TTG-V0-963 Stated untested marker and counts

A traceability row at HEAD that holds a requirement ID in its first cell and the text
`(untested: REASON)`, where REASON has at least one non-space character, MUST exempt that
requirement from TTG-V0-962. The gate MUST print one added line with the number of subtest
citations checked, and in the range form one added line with the number of requirements cited at
BASE and the number exempted by this marker. The summary line of TTG-V0-008 MUST NOT change.

#### Scenario: The removal is stated

- **GIVEN** at BASE a row for `REQ-1` cites only `TestX`
- **WHEN** HEAD deletes `TestX` and the row reads `(untested: covered by the measured run)`
- **THEN** the range form passes and reports one untested requirement

#### Scenario: The marker has no reason

- **GIVEN** the same change, with the row reading `(untested: )`
- **WHEN** the range form runs
- **THEN** it fails and names `REQ-1`
