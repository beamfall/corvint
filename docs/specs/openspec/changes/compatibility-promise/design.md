# Design: `compatibility-promise`

## Decisions

- Scope. Decided 2026-11-09 under R42, D10: the promise covers only surfaces that have a contract
  test. Previews and caches are excluded by name. A removal needs one minor release of notice.
- Form. Decided 2026-11-09 under R42, D10a: one table in `docs/COMPATIBILITY.md`, one row per
  surface, with a `stable` or `preview` column. Columns: surface, kind (command, flag, exit code,
  JSON field set, file, action input, action output, tool name), stability, contract test, notes.
- Source. The stable rows are generated from the existing contract tests (CMP-V1-002). The
  generator reads a list that maps each surface of CMP-V1-003 to its test; a surface with no test
  gets no stable row and is named in the build-log entry.
- Labels. No label is renamed at 1.0 (CMP-V1-006). The digit is part of the name. A breaking
  change to a stable profile takes the next digit and the old one is served for one minor
  release.
- Check. One check compares the committed page with the generated one and confirms that every
  named contract test exists. It runs with the documentation gates.
- Not decided here: the plan requirement ID that the promise serves. The plan edit is listed for
  the owner in the decision report; this folder does not edit the plan.

## Failure modes

- A stable row names a test that was deleted: the check fails and names the row.
- A hand edit to `docs/COMPATIBILITY.md`: the check fails on the difference from the generated
  table.
- Fewer than half of the listed surfaces have a contract test: the lane stops and reports to the
  owner before it writes the table (CMP-V1-003). This is the condition the decision named as one
  that would change it.
- A `/0` label under a stable row is read as unstable: the page states, above the table, that
  the digit is part of the name and that the column carries the stability.
- A stable format's label differs from what this folder assumes: the lane reads the label from
  the code; which label each format carries today is UNVERIFIED.
- A preview surface changes without a release note: the release record check of change
  `release-process` is the place that catches it; this change adds no second check.

## Acceptance evidence

- `docs/COMPATIBILITY.md` generated, with every stable row naming an existing contract test.
- The check's tests: a missing test, a hand-edited page and an unlisted surface.
- The count of surfaces in CMP-V1-003 with and without a contract test, in the build-log entry.
- The label read from the code for each of the six stable formats, in the build-log entry.

## Rollback

- The page, generator and check are additive. Reverting the change's commits returns
  `docs/COMPATIBILITY.md` to the placeholder of UDC-V1-007.
- A promise that has shipped in a release is not withdrawn by a revert; narrowing it needs an
  owner decision and a release note.
