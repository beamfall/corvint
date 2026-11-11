# Traceability gate upgrade: subtest citations and the last test citation (V1-4803)

Change `traceability-gate-upgrade` (proposal, design and spec delta under
`docs/specs/openspec/changes/traceability-gate-upgrade/`), ticket V1-4803, requirements TTG-V0-960
to TTG-V0-963 in `docs/specs/traceability-test-gate-v0.md`. The intent is proposed: the four
requirements are experimental and nothing here accepts them.

## What was built

- `script/check-traceability-tests.sh` stays POSIX `sh` with inline Perl reading the Git index or
  the range's trees. The per-line name classification and the traceability-section scan were each
  moved into one Perl function (`row_names`, `scan_rows`) so the existing check and the new
  per-tree citation scan apply the same exemptions; the existing check's results are unchanged.
- TTG-V0-960, TTG-V0-961: a checked, declared name followed at once by `/REQ-ID` is a subtest
  citation; it fails unless a declaration body of that name, read lexically from its `func` line to
  the next column-0 `}`, holds the ID with the boundary rule of the design. Each distinct spec path
  and citation is counted and reported once (in the range form at the first commit that holds it).
- TTG-V0-962, TTG-V0-963: with `--range BASE..HEAD` the gate loads BASE's test declarations and
  rows, and HEAD's, and fails for each requirement cited at BASE but not at HEAD unless HEAD's live
  specs no longer state it or its HEAD row states `(untested: REASON)` with a non-blank reason.
- Output: the TTG-V0-008 summary line is byte-identical; `traceability subtest citations: N checked`
  follows the existing lines, and the range form adds
  `traceability last citations: N requirements cited at BASE, M untested`.
- Fixtures in `script/check-traceability-tests_test.sh`: subtest resolves and is counted; body
  lacks the ID; a doc comment and a later function do not count; `REQ-12` does not satisfy
  `REQ-1`; a suffix that is not an ID; an undeclared name gets only its TTG-V0-005 line; a second
  declaration under another build tag resolves; the selector, `(PLANNED)`, `(not a test)` and
  `(removed)` exemptions; and in a separate fixture repository the only citing test deleted
  (fails, naming BASE's citation; the index form passes), a second citation remains, the marker
  with and without a reason, the requirement removed, the spec retired, a resolving subtest
  citation keeping its requirement cited, and a BASE subtest citation that never resolved not
  counting. The fixture fails on the base checker, and each of eight one-line mutations of the new
  code (body end, ID boundary, empty reason, mention check, subtest-cited ID, retired skip, BASE
  body check, the lost report) fails it.

## Measurement before the change (task 2.1)

Subtest citations with a requirement-ID suffix in `docs/specs/*.md` at `7b28ab461c05`:

| Spec line | Citation | In a traceability section | Cited body holds the ID |
|---|---|---|---|
| `go-production-kernel-migration-v0.md:1191` | `TestImpactRanksSamePackageTestsByDeclarationReferences` / `GPK-V0-056` | yes | yes (`internal/contextindex/ranking_regression_test.go`) |
| `go-production-kernel-migration-v0.md:1208` | `TestImpactDisclosesOmittedDirectGoCallers` / `GPK-V0-075` | yes | yes (`internal/contextindex/impact_callers_test.go`) |
| `local-completion-policy-v0.md:53` | `TestLocalCompletionRealEvidenceWorkflow` / `LCP-V0-883` | no (requirement prose) | not declared: removed under decision 0594 |
| `native-jvm-bridge-analyzer-candidates-v0.md:115`, `:122`, `:123`, `:125`, `:126` | five citations / `NJB-001` to `NJB-005` | no (`## Matrix and evidence`) | yes, all five (`internal/analyzernativebridge/analyzer_test.go`) |

So the gate checks two subtest citations today, and both hold their ID. No test file was edited.

## Measurement after the change (task 2.4)

| Command | Exit | Wall | Output |
|---|---|---|---|
| `make traceability-tests-check` (lane head) | 0 | 0.55 s | 4 planned; 907 marked removed, 112 retired; 2 subtest citations checked |
| `make traceability-tests-check TRACE_RANGE=origin/main..HEAD` (`origin/main` was `b13c6fe4c58b`, 3 commits past the lane base) | 0 | 1.72 s | 2 subtest citations; 2568 requirements cited at BASE, 0 untested |
| `make traceability-tests-check TRACE_RANGE=7b28ab461c05..HEAD` | 0 | 1.73 s | same counts |
| `make traceability-tests-check TRACE_RANGE=538e823f4a1d..origin/main` (the last 50 first-parent commits of `main`) | 2 (script exit 1) | 11.8 s | 4277 requirements cited at BASE, 0 untested; 516 lost their last test citation; 5094 TTG-V0-950 lines |

Over those 50 commits, 516 requirements lost their last resolving test citation while their IDs
stayed in live specs. Attributing each by running the range form over each adjacent pair of
first-parent commits (111 s for 50 pairs): 494 at `b54ac564d898` (PR #176, the decision 0594 cut,
834,927 deleted lines), 21 at `fea1de858708` (PR #166: AFP-V0-490 to 493, twelve CCE-V0 IDs,
CTF-V0-010, PSM-V0-001, SKH-V0-001, SKH-V0-189, WSD-V0-005) and 1 at `568f962f973b` (PR #167:
CIN-V0-2304). Most of the 494 are rows whose tests the cut marked `(removed)`; the largest groups are
CEM-V1 (107), LCP-V0 (37), AHI (25), FPK-V0 (18) and SPC-V0 (15). Eleven are ticket IDs such as
`V1-0313` in a first cell, which match the requirement ID shape; the rule counts them as written.
They are not fixed in this change; the full list went to the orchestrator for tickets.

The 5094 TTG-V0-950 lines are the existing per-commit rule: rows of an earlier commit in the range
(5072 of them at `59b31f12ed98`) name tests that `main` no longer declares. A historical range
therefore fails on it whatever this change does; a pull request range does not, because its rows
and its head move together.

## Decisions taken in the build

- A marker written after a subtest suffix (`` `TestX/REQ-1` (PLANNED) ``) is not recognized,
  because the design keeps the exemption checks unchanged and they read the text right after the
  name. No row at the base uses that form. A planned subtest citation is written `TestX (PLANNED)`.
- The range first compared BASE as given, as the design said. Review finding 2 changed that: see
  "Review fixes" below.
- No CI step runs the range form yet (task 2.5): the `.github` edit is a separate change.

## Review fixes (round 1)

An independent review of the first build raised five findings. Each was checked against the code
before any change; the spec amendment (`91c32387a`) precedes the script commit (`79fefeb42`).

- One-line declaration (P1, confirmed by a repro: `func TestOne(t *testing.T) { t.Skip() }`
  followed by a function holding `REQ-1` resolved `TestOne/REQ-1`). The body is now the declaration
  line alone when that line closes every brace it opens and ends on a closing brace (TTG-V0-961).
  The body also now starts at the `func` keyword, not at blank lines the declaration pattern's
  leading `\s*` could span.
- Literal BASE (P2, confirmed in the code). The range form compares `git merge-base BASE HEAD`
  with HEAD and refuses a range with no merge base, naming both revisions, exit 2 (TTG-V0-962).
  The 50-commit measurement above is unchanged by this (its BASE is an ancestor): same 516 lines,
  10.0 s.
- Raw-string brace (P2, confirmed by a repro). Not parsed: TTG-V0-961 and the trust-boundary table
  state that a column-0 `}` in a raw string ends the body, and the refusal now reads
  `(no declaration of T contains ID; each body was read from its func line to the first column-0 }: FILE...)`.
- Quoted brace on the declaration line (P2, batch review, confirmed by a fixture that fails on the
  earlier script). `func TestX(t *testing.T) { _ = "{" }` counted the quoted brace, was not read as
  a one-line test and took the next function as its body; a declaration line whose only closing
  brace was quoted was read as a one-line test. Braces inside a string literal, a rune literal or
  a comment that begins and ends on the declaration line are no longer counted (TTG-V0-961).
- `(PLANNED)` demotion (P3). A fixture pins that a demoted name does not cite, so the requirement
  is reported as losing its last citation unless the row states `(untested: REASON)`.
- Unstated rules (P3). TTG-V0-960 and TTG-V0-962 now say that one resolving name cites every
  first-cell ID of its row, that a marker after a subtest suffix is not recognised, and that ticket
  IDs match the ID shape. No behaviour changed for these.

The fixture fails on the previous script, and reverting the one-line rule, the merge base or the
no-merge-base refusal each fails it.

## Not run

- `make gate` and the repository-wide Go tests: not required for a script-only change; no Go file
  changed.
