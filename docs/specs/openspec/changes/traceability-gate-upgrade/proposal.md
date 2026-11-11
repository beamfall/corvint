# Change proposal: the traceability gate resolves subtest citations and refuses the removal of a requirement's last test citation

Change name: `traceability-gate-upgrade`
Ticket: V1-4803 (milestone `v1-0`, P1)
Product requirement served: PROD-INT-005 in `docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: `docs/specs/traceability-test-gate-v0.md` (existing, prefix `TTG-V0`)
Requirement IDs this change adds: TTG-V0-960 to TTG-V0-963 (block TTG-V0-960 to TTG-V0-969 is
reserved for this change)
Requirement IDs this change modifies: none
Intent status: proposed (written 2026-11-11; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. No lane has been started for this change.

This change follows the G14 layout: `proposal.md` (this file), `design.md`, `tasks.md` and
`specs/traceability-test-gate/spec.md`.

## Why

PROD-INT-005 requires each accepted capability requirement to trace to a test or a measured result.
The gate that checks this today, `script/check-traceability-tests.sh`, proves only that a test
function named in a traceability row exists. The 1.0 plan includes test consolidation (V1-4812,
V1-4814 and later tickets), which merges, renames and deletes tests. Under the present gate a
consolidation can pass while it silently takes the last test away from a requirement: the lane
deletes the test and edits the row in the same change, and nothing fails. Ticket V1-4803 asks for
three checks and states that they must land before any test consolidation.

## What exists today

Verified against `origin/main` 7b28ab461c05 on 2026-11-11:

- `script/check-traceability-tests.sh` (247 lines, POSIX `sh` with inline Perl) reads the Git index,
  or with `--range BASE..HEAD` the trees of the range's first-parent commits (TTG-V0-950).
- It collects top-level `func TestName(` declarations (TTG-V0-001) and checks every `Test` name in a
  traceability table row (TTG-V0-005). Markers `(PLANNED)`, `(not a test)` and `(removed)` exempt a
  name (TTG-V0-006, TTG-V0-917, TTG-V0-952). A retired spec is skipped (TTG-V0-951).
- A citation written `TestName/SUFFIX` is checked as `TestName` only. The suffix is ignored. Eight
  rows in three specs already use a requirement ID as the suffix, for example
  `TestImpactDisclosesOmittedDirectGoCallers/GPK-V0-075`.
- The gate does not read test bodies and keeps no record of which requirements had a test citation.
- `make traceability-tests-check` runs the index form in `make gate` and in the CI doc gates. No CI
  step runs the `--range` form.

## What Changes

- A citation `TestName/REQ-ID`, where the suffix starts with a requirement ID, resolves only when
  `TestName` is declared and the body of a declaration of `TestName` contains that requirement ID
  (TTG-V0-960, TTG-V0-961). A body that lacks the ID fails and names the row.
- With `--range BASE..HEAD` the gate also compares the two trees: a requirement that has at least
  one resolved test citation at BASE and none at HEAD fails (TTG-V0-962).
- A row may state `(untested: REASON)` for a requirement whose last test is removed on purpose. The
  gate passes that requirement and prints the count (TTG-V0-963).
- The summary line of TTG-V0-008 keeps its text. The new counts are printed on added lines.

## Non-goals

- Requiring the requirement ID in the body of every cited test. A plain citation `TestName` is
  checked as today. Only the subtest form is checked against the body.
- Parsing Go. The body is found lexically (see `design.md`).
- Proving that a test asserts the requirement.
- Expanding a written range of IDs such as `TTG-V0-003 to TTG-V0-007`. Only IDs written in full in
  the row count.
- Running the `--range` form in CI. That needs a `.github` edit, which is a separate change in its
  own PR (task 2.5 files the ticket).
- Any change to the index form for rows that do not use the subtest form.

## Impact

- Files: `script/check-traceability-tests.sh`, `script/check-traceability-tests_test.sh`,
  `docs/specs/traceability-test-gate-v0.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md` (one paragraph),
  generated `docs/specs/REQUIREMENTS.tsv` and `INDEX.json`, one build-log entry. The eight existing
  subtest rows are fixed in the test files only if the measurement in task 2.1 shows they fail.
- Behaviour: a commit that passes today can fail when a subtest citation names an ID that the test
  body lacks, or (range form) when it removes the last test citation of a requirement.
- Rollback: revert the script and fixture commits. No stored state or file format is added.

## Failure modes

- A requirement ID appears in a test body only by coincidence (a comment naming another rule). The
  check passes; it is lexical. This is the same limit as the existing name check.
- A test declared in two files under different build tags: the body of either one satisfies the
  check.
- A row moves the last citation from one requirement spelling to a range form. The requirement is
  reported as losing its last citation, because ranges are not expanded. The fix is to write the ID
  in full.
- Until a CI step runs the range form, TTG-V0-962 is enforced only where a person or a lane runs
  `make traceability-tests-check TRACE_RANGE=BASE..HEAD`.

## Acceptance evidence

- Fixture tests in `script/check-traceability-tests_test.sh` for each requirement, including the
  three acceptance criteria of V1-4803.
- The index form passes on the lane head, and the range form passes for `origin/main..HEAD`.
- A recorded measurement: the number of subtest citations checked on `main`, and the result of the
  range form over the last 50 first-parent commits of `main`.
