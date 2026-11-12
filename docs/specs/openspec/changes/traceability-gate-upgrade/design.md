# Design: `traceability-gate-upgrade`

## Decisions

- Requirement ID shape. A requirement ID is text matching
  `[A-Z][A-Z0-9]*(-[A-Z0-9]+)*-[0-9]+`, for example `TTG-V0-960`, `NJB-002` or `REQ-1`. The match
  is the longest one at that position.
- Subtest citation. In a traceability row, a checked test name followed at once by `/` and a
  requirement ID is a subtest citation. Text after the ID (for example `_restore_satisfied`) belongs
  to the subtest name and is ignored. A `/` followed by anything else is ignored as today.
- Test body. The body of `TestName` in a file is the text from the line that matches the existing
  declaration pattern of TTG-V0-001 up to and including the first later line that is exactly `}`
  in column 0, or to the end of the file when no such line follows. Go files in this repository are
  held to `gofmt`, where a top-level function closes with `}` in column 0. A doc comment above the
  declaration is not part of the body. Amended in the build (review finding, 2026-11-11): when the
  declaration line closes every brace it opens and ends on a closing brace (a one-line test), the
  body is that line alone. A column-0 `}` inside a raw string still ends the body; the refusal says
  so rather than parsing Go.
- ID in body. The body contains the ID when the ID text appears and is not directly preceded by a
  letter, a digit or `-`, and not directly followed by a letter or a digit. So `REQ-1` does not
  match inside `REQ-12`, and it does match in `"REQ-1_case"`. Comments and string literals count.
- Several declarations. When more than one tracked test file declares `TestName`, the citation
  resolves if any one body contains the ID.
- Order of checks. The existing checks run first and are unchanged. A name that is exempt
  (`(PLANNED)`, `(not a test)`, `(removed)`, a selector, a wildcard) is not checked as a subtest
  citation either. A name that is not declared fails under TTG-V0-005 as today and gets no second
  failure line.
- Sources of test bodies. The index form reads bodies from the Git index. The range form reads
  them from HEAD's tree, the same source the name set uses (TTG-V0-002, TTG-V0-950).
- Citation of a requirement, for the last-citation rule. A requirement is cited at a tree when a
  traceability row of a spec that is not retired in that tree holds the ID written in full in the
  row's first cell, and the same row holds at least one test name that the gate checks and
  resolves in that tree. A subtest citation `TestName/REQ-ID` that resolves also cites `REQ-ID`,
  whatever the first cell holds.
- Last-citation rule. It runs only with `--range BASE..HEAD`. It compares the tree of BASE with
  the tree of HEAD, not each commit between them. Amended in the build (review finding,
  2026-11-11): "BASE" here is `git merge-base BASE HEAD`, so a BASE that moved on after HEAD
  branched reports nothing HEAD did not change; no merge base is refused naming both revisions.
  For BASE the name set and the rows both come from BASE's tree. A requirement cited at BASE and not cited at HEAD fails, unless one of these
  holds at HEAD:
  - the ID no longer appears in any `docs/specs/*.md` file that is not retired (the requirement
    left with its spec, or its spec is retired);
  - a traceability row holds the ID in its first cell and the text `(untested: REASON)` with a
    reason of at least one non-space character.
- Why the range form and not a stored list. A checked-in list of cited requirements would let the
  index form enforce the rule, but every lane that adds a citation would edit one shared generated
  file, which is the merge-conflict pattern `REQUIREMENTS.tsv` already causes. The range form needs
  no stored state.
- Output. The summary line of TTG-V0-008 is unchanged, because tools read it. The gate adds one
  line `traceability subtest citations: N checked`. The range form adds one line
  `traceability last citations: N requirements cited at BASE, M untested`.
- Failure lines. A subtest failure is
  `path:line: TestName/REQ-ID (no declaration of TestName contains REQ-ID; each body was read
  from its func line to the first column-0 }: FILE...)`, with the commit prefix
  of TTG-V0-950 in the range form. A last-citation failure is
  `REQ-ID: last test citation removed (BASE path:line: TestName)`, naming one citation at BASE.
  The exit status for any failure is the one the gate already uses.

## Alternatives not taken

- Require the ID in every cited test body. Most cited tests do not name their requirement today, so
  this would need a baseline of several thousand rows. It can be proposed later with a measured
  count.
- Parse Go with `go/parser` in a Go tool. It is exact for bodies, but it replaces a script that is
  wired into `make gate`, the doc gates and their fixtures. The lexical body rule is exact for
  `gofmt` output.

## Risks

- The eight existing subtest rows may fail when the cited body holds the ID in a different form.
  Task 2.1 measures this before the check is switched on, and the fix is to name the ID in the
  subtest, which is what the row already claims.
- The range form reads two full trees. Task 2.4 records its run time over 50 commits of `main`.

## Failure modes

- The body rule is lexical: an ID that appears in the cited body only by coincidence (a comment
  naming another rule) passes, the same limit as the existing name check.
- A test declared in two files under different build tags: either body satisfies the check.
- A written range of IDs is not expanded, so moving the last citation into a range reports the
  requirement as losing its last citation; the fix is to write the ID in full.
- Until a CI step runs the range form (task 2.5), TTG-V0-962 holds only where
  `make traceability-tests-check TRACE_RANGE=BASE..HEAD` is run by hand or by a lane.

## Acceptance evidence

- Fixture cases in `script/check-traceability-tests_test.sh` for TTG-V0-960 to TTG-V0-963, run by
  `make traceability-tests-test` (tasks 2.2 and 2.3).
- The index form on the lane head and the range form for `origin/main..HEAD` and the last 50
  first-parent commits of `main`, with counts and run time in the build-log entry (task 2.4).

## Rollback

Revert the script and fixture commits. The change adds no stored state and no file format, so the
previous script reads the same rows.
