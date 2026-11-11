# Traceability test gate V0

Owner: Russell Lewis
Date: 2026-09-07
Requirement prefix: `TTG-V0`
Intent status: accepted (owner instruction 2026-09-07)
Delivery status: implemented
Authoritative inputs: `../../AGENTS.md` invariant 8, `../SPEC-DRIVEN-DEVELOPMENT.md` "Required
capability-spec shape" (item 9, the requirement-to-implementation traceability table), and
`documentation-citation-gate-v0.md` as the shape and tone template this spec follows.

## Agent digest
- Claim: A Go test function named in a spec traceability table must exist or be marked PLANNED.
- Status: accepted (owner instruction 2026-09-07) / implemented; TTG-V0-011 experimental under the standing lane auto-accept (2026-10-29; V1-3143); TTG-V0-012 experimental under the same (2026-10-30; V1-3235)
- Exists: `script/check-traceability-tests.sh`, fixture test `script/check-traceability-tests_test.sh`, `make` targets `traceability-tests-check` and `traceability-tests-test` in `gate`.
- Blocked on: nothing for this gate.
- Read next: Requirements; Non-goals; Trust boundary and failure modes.

## Human intent and scope

Corvint specs carry a traceability table mapping each requirement ID to the code that implements it
and the evidence that verified it. That evidence column commonly names a Go verification function
by its identifier so a reader can jump from the spec straight to the check. Nothing previously
caught a table row naming a verification function that was renamed, deleted, or never written —
the row would keep citing a name that no longer resolved, and a reader following it would find
nothing.

Affected user: an agent or engineer trusting a spec's traceability table to point at a real,
present verification function. Measurable job: for the Git index state that a commit would publish,
refuse a spec traceability table that names a Go function identifier starting with `Test` unless an
indexed Go test file declares a top-level function of that exact name, while leaving an explicitly
not-yet-built row unblocked.

## Verified current state

`script/check-traceability-tests.sh` is wired into `make gate` through the `traceability-tests-check`
target in `Makefile`, whose target list documents it as one of the fixed prerequisites of `gate`.
The script is POSIX `sh` that hands its logic to an inline Perl program; it is read-only, takes no
arguments, and requires only `git` and `perl` on `PATH`.

## Requirements

- `TTG-V0-001`: The gate MUST build its set of known Go verification function names by listing
  tracked repository paths with `git ls-files -z --cached`, keeping only paths whose name ends in
  `_test.go`, then reading each kept path's current Git-index blob in full and collecting every
  top-level declaration matching `func TestName(` at the start of a line (leading whitespace
  allowed, arbitrary parameter list, no receiver clause). A method with a receiver clause between
  `func` and the name is NOT collected, because the pattern admits no receiver group.
- `TTG-V0-002`: TTG-V0-001 MUST resolve both test paths and test-file content from the Git index,
  not from the working tree. An untracked `_test.go` file or an unstaged edit to a tracked one MUST
  NOT change what the gate accepts; staging that path or edit MUST change the result on the next
  run. Thus the gate evaluates the content a commit would publish, including staged additions,
  even when the corresponding working-tree file differs or is absent.
- `TTG-V0-003`: The gate MUST treat a heading of level 1 to 6 as opening a traceability section
  when its title text contains the substring "traceability", case-insensitively, anywhere in the
  title, not only when the heading reads exactly "Traceability". The section MUST close at the
  next heading whose level is less than or equal to the opening heading's level, and MUST stay
  open across any heading strictly deeper than it, including a deeper heading whose title also
  contains "traceability", which MUST NOT replace the opening heading's level.
- `TTG-V0-004`: Within an open traceability section the gate MUST inspect only lines that, after
  optional leading whitespace, begin with a pipe character (a Markdown table row, including a
  header-separator row); it MUST NOT inspect prose lines even inside an open section.
- `TTG-V0-005`: On each inspected line the gate MUST find every substring matching a `Test` prefix
  followed by one or more letters, digits, or underscores, bounded by word boundaries, and MUST
  fail unless that exact name is present in the set built under TTG-V0-001. The match is purely
  lexical: a table-row word that happens to start with that prefix and continue with word
  characters is indistinguishable to the gate from a deliberate function-name citation.
- `TTG-V0-006`: A matched name MUST be exempted from TTG-V0-005 when the text immediately
  following it in the same line, after skipping one optional closing backtick and any run of
  spaces or tabs, begins with the literal text `(PLANNED)`. Each such exemption MUST be counted
  toward a running total for the whole gate invocation. Any other name failing TTG-V0-005 MUST be
  recorded, not just the first, as `path:line: Name`.
- `TTG-V0-007`: The gate MUST evaluate every file matched by a non-recursive glob of
  `docs/specs/*.md`, processed one file at a time with independent traceability-section state, so
  a section open at the end of one file never carries into the next.
- `TTG-V0-008`: The gate MUST print a single summary line of the form "traceability tests: N
  planned" to standard output on every run, whether or not it goes on to fail, where N is the
  TTG-V0-006 total across all files.
- `TTG-V0-009`: When one or more names fail TTG-V0-005, the gate MUST print a header line followed
  by one line per recorded failure to standard error, in the `path:line: Name` form, and MUST exit
  non-zero. With no recorded failures it MUST exit zero and print nothing to standard error.
- `TTG-V0-010`: The gate MUST be wired as the `traceability-tests-check` `make` target and MUST be
  a member of the `gate` target's prerequisite list, so a failure under TTG-V0-009 fails `make
  gate`. Its fixture `script/check-traceability-tests_test.sh` MUST be wired as the
  `traceability-tests-test` target in the same prerequisite list, so a checker regression the
  fixture detects also fails `make gate`.
- `TTG-V0-011`: (experimental under the standing lane auto-accept, 2026-10-29; V1-3143; refines
  TTG-V0-005) A matched name immediately preceded by `.` that itself follows a word character or
  `)` is a Go selector, such as `toolgroup.TestValidity` or `(*T).TestRun`, naming an identifier of
  another package or type, and MUST NOT be checked under TTG-V0-005; a row cites a test function
  bare. Every other match on the same line is still checked. Non-goals: resolving bare type names
  or prose words, which still need rephrasing. Failure mode: a test cited only in qualified form
  goes unchecked.
- `TTG-V0-012`: (experimental under the standing lane auto-accept, 2026-10-30; V1-3235; refines
  TTG-V0-009) The test-function set stays the index snapshot of TTG-V0-001 and TTG-V0-002, so a
  name declared only in the working tree stays unresolved. When an untracked (not ignored) or
  unstaged modified `_test.go` file in the working tree declares an unresolved name, its failure
  line MUST name that file and say that the check reads the index and the file must be staged
  (`git add -- FILE`). A name no working-tree file declares keeps the plain `path:line: Name` line.
  Non-goals: reading working-tree content as evidence. Failure mode: a name declared only in a
  Git-ignored file gets no hint.
- `TTG-V0-917`: (experimental under the standing lane auto-accept, 2026-11-01; V1-4048; refines
  TTG-V0-005 and TTG-V0-006) A matched name whose following text, after an optional closing
  backtick and spaces or tabs, begins with the literal text `(not a test)` names no test function,
  such as a fixture word `TestLine`, and MUST NOT be checked or counted. Every other match on the
  line is still checked. Non-goals: guessing which words are prose. Failure mode: a real test cited
  with the marker goes unchecked, as a `(PLANNED)` one does, but without a count.
- `TTG-V0-950`: (experimental under the owner's standing auto-accept (2026-11-01); V1-3586)
  `script/check-traceability-tests.sh --range BASE..HEAD` (`make traceability-tests-check
  TRACE_RANGE=BASE..HEAD`) MUST check the traceability rows of every first-parent commit of the
  range, each read from that commit's tree, against the test functions declared in HEAD's tree, so
  a spec-first commit may cite a test that a later commit of the same change adds. A cited name
  that HEAD does not declare MUST fail, named once as `COMMIT path:line: Name` at the first commit
  that cites it; a malformed range or a range with no commit exits 2. The index check without
  `--range` is unchanged except that its failure report MUST name the range form as the route for
  a spec-first commit. Failure mode: the spec-first commit order that `SPEC-DRIVEN-DEVELOPMENT.md`
  requires failed the index check whenever the trace row cited the next commit's test, so lanes
  broke the order or squashed. Non-goal: proving the test asserts the requirement. Rollback: drop
  the option and the hint.
- `TTG-V0-951`: (experimental under the owner's standing auto-accept, 2026-11-09; decision 0594)
  A spec whose first 40 lines hold a `Delivery status: retired` header line MUST NOT be checked.
  Its body is kept as a record of removed code, so its rows name tests that left with that code.
  The gate MUST print the number of specs it skipped this way. Failure mode: a spec retired by
  mistake loses the check until its header is corrected. Non-goal: reading any other status word.
- `TTG-V0-952`: (experimental under the owner's standing auto-accept, 2026-11-09; decision 0594)
  In a spec that is not retired, a matched name whose following text, after an optional closing
  backtick and spaces or tabs, begins with the literal text `(removed)` records a test removed with
  its code. It MUST pass while no tracked test file declares the name, MUST be counted in the
  gate's output, and MUST fail, naming the row, while the name is declared. Failure mode: a row
  keeps the marker after its requirement gains a new test under another name; the row then names
  no live evidence, which the marker count keeps visible. Non-goal: proving the test once existed.
- `TTG-V0-960`: (proposed 2026-11-11 in change `traceability-gate-upgrade`; V1-4803; delivery
  experimental) In a traceability row, the gate MUST treat a checked test name that is followed at
  once by `/` and a requirement ID as a subtest citation `TestName/REQ-ID`. A requirement ID is the
  longest text at that position matching `[A-Z][A-Z0-9]*(-[A-Z0-9]+)*-[0-9]+`; text after it, such
  as `_restore_satisfied`, belongs to the subtest name and is ignored. The shape also matches ticket
  IDs such as `V1-0313`, which count as written. A subtest citation MUST resolve only when
  `TestName` is declared (TTG-V0-001) and TTG-V0-961 holds. A `/` followed by other text MUST be
  handled as before this change. A name exempt under TTG-V0-006, TTG-V0-011, TTG-V0-917, TTG-V0-952
  or the wildcard rule is not checked as a subtest citation, and an undeclared name fails under
  TTG-V0-005 only. A marker written after a subtest suffix, as in `TestName/REQ-ID (PLANNED)`, is
  not recognised, because those markers are read only right after the name. Non-goal: requiring the
  ID in the body of a plain `TestName` citation. Rollback: remove the subtest branch and its
  fixtures.
- `TTG-V0-961`: (proposed 2026-11-11 in change `traceability-gate-upgrade`; V1-4803; delivery
  experimental) For a subtest citation `TestName/REQ-ID`, the gate MUST fail unless the body of at
  least one tracked declaration of `TestName` contains `REQ-ID`. The body is the declaration line of
  TTG-V0-001 alone when that line closes every brace it opens and its last brace is a closing one,
  as in `func TestOne(t *testing.T) { t.Skip() }`, where a brace inside a string literal, a rune
  literal or a comment that begins and ends on that line is not counted; otherwise it is the text from the declaration
  line to the first later line that is exactly `}` in column 0, or to the end of the file. The ID is
  contained only when it is not directly preceded by a letter, a digit or `-` and not directly
  followed by a letter or a digit; comments and string literals count. The index form reads bodies
  from the Git index and the range form from HEAD's tree. The failure line MUST be `path:line:
  TestName/REQ-ID (no declaration of TestName contains REQ-ID; each body was read from its func line
  to the first column-0 }: FILE...)`, naming every file that declares `TestName`, with the commit
  prefix of TTG-V0-950 in the range form. Failure modes: an ID that appears in the body only by
  coincidence passes, because the check is lexical; a line that is exactly `}` in column 0 inside a
  raw string ends the body there, so an ID after it is not seen and the citation is refused, and the
  failure line says where the body was read to so the test can move the ID above that line or indent
  the string.
- `TTG-V0-962`: (proposed 2026-11-11 in change `traceability-gate-upgrade`; V1-4803; delivery
  experimental) With `--range BASE..HEAD` the gate MUST compare the tree of the merge base of BASE
  and HEAD (`git merge-base BASE HEAD`) with the tree of HEAD, so a BASE that has moved past the
  point HEAD branched from reports nothing HEAD did not change. When BASE and HEAD have no merge
  base the gate MUST refuse, naming both revisions, and exit 2; it MUST NOT fall back to BASE
  itself. A BASE or HEAD that is not a commit is refused as under TTG-V0-950. A requirement is cited
  at a tree when a traceability row of a spec that is not retired in that tree holds its ID in full
  in the first cell together with at least one test name that resolves in that tree, or when a
  subtest citation naming the ID resolves in that tree; the name set, bodies and rows all come from
  that tree. Any one resolving test name in a row marks every ID written in full in that row's first
  cell as cited. A name demoted to `(PLANNED)` does not resolve, so a requirement whose only
  citation is demoted counts as not cited at HEAD. The gate MUST fail for every requirement that is
  cited at the merge base and not cited at HEAD, unless at HEAD the ID appears in no
  `docs/specs/*.md` file that is not retired, or TTG-V0-963 applies. The failure line MUST be
  `REQ-ID: last test citation removed (MERGE-BASE path:line: TestName)`, naming one citation it had
  at the merge base by its 12-character commit prefix. Non-goal: expanding a written range of IDs;
  only IDs written in full count. Failure mode: until CI runs the range form, the rule holds only
  where someone runs it.
- `TTG-V0-963`: (proposed 2026-11-11 in change `traceability-gate-upgrade`; V1-4803; delivery
  experimental) A traceability row at HEAD that holds a requirement ID in its first cell and the
  text `(untested: REASON)`, where REASON has at least one non-space character, MUST exempt that
  requirement from TTG-V0-962. The gate MUST print one added line
  `traceability subtest citations: N checked`, and in the range form one added line
  `traceability last citations: N requirements cited at BASE, M untested`. The summary line of
  TTG-V0-008 MUST NOT change.

## Non-goals and simpler baseline

The simpler baseline would be no check at all, trusting authors to keep traceability tables
current by hand; TTG-V0-001 through TTG-V0-009 replace that trust with a single lexical existence
check. This gate does not run any Go test, does not check that a matched function's signature
takes `*testing.T` or is otherwise runnable by `go test`, does not check that a function actually
exercises the requirement its row names, does not check that a function is reachable from any
package's build graph, does not de-duplicate or warn on two different files declaring the same
function name, does not look inside a fenced code block differently than prose (a `Test...` token
inside a fenced block is not in a table row so it is already skipped by TTG-V0-004, not by any
code-block-aware logic), does not read any `docs/agent-memory/*.md` or nested `docs/specs/**`
subdirectory, and does not read Git history or any revision other than the current Git index.
It proves a name resolves to some declared function; it does not prove the row's claim about that
function is true.

## Trust boundary, limits, and failure modes

Spec Markdown and `_test.go` source are both untrusted input read from the Git index; the gate uses
`git ls-files` to enumerate paths and `git cat-file` to read blobs, with no network access and no
write to any file. Every failing name across every spec file is collected before the gate reports,
so one run's failure list is complete rather than first-failure-only; there is no partial-success
exit code.

| Failure | Behavior |
|---|---|
| Cited identifier not declared as a top-level func in any `_test.go` file the gate can see | fail, naming the citing file, line, and identifier |
| Cited identifier declared only as a method with a receiver clause | fail, same as above; a receiver clause is not matched by TTG-V0-001 |
| Cited identifier marked `(PLANNED)` immediately after the token | pass; counted in the summary total, not checked for existence |
| A word of the test-name shape marked `(not a test)` immediately after the token | pass; not checked and not counted (TTG-V0-917) |
| A capitalized table-row word that happens to start with the matched prefix but names no verification function | fail, indistinguishable from a real citation; see TTG-V0-005 |
| An untracked or Git-ignored `_test.go` file declares the only matching function | its declarations are invisible to TTG-V0-001; the citing row fails, naming the untracked file and the staging fix (TTG-V0-012); an ignored file gets no hint |
| A subtest citation's ID appears in the test only after a raw-string line that is exactly `}` in column 0 | the body ends at that line, the citation is refused, and the failure line names the files and says the body was read to the first column-0 `}` (TTG-V0-961) |
| The range form's BASE has moved past the point HEAD branched from | the comparison uses `git merge-base BASE HEAD`; no merge base is refused naming both revisions (TTG-V0-962) |
| A spec-first commit cites a test its change adds in the next commit | the index check fails at that commit naming the row and the `TRACE_RANGE=BASE..HEAD` route; the range check passes once the test is in HEAD, and fails naming the first citing commit when it never lands (TTG-V0-950) |
| An unstaged edit adds or removes a declaration in a tracked `_test.go` file | the edit is invisible; the gate continues to evaluate the indexed blob per TTG-V0-002; an added declaration's failure names the file and the staging fix (TTG-V0-012) |

## Acceptance criteria

Acceptance rests on the gate running clean over the current indexed `docs/specs/*.md` corpus as a
`gate` prerequisite and on `script/check-traceability-tests_test.sh` proving that an untracked test
function cannot satisfy an indexed traceability row while staging the same function makes it
visible, and that an undeclared name stays reported under a deeper, repeated traceability heading
but not after a heading at or above the opening depth.

| Requirement | Evidence |
|---|---|
| TTG-V0-001, TTG-V0-002 | `script/check-traceability-tests_test.sh` untracked-then-staged fixture |
| TTG-V0-003, TTG-V0-004, TTG-V0-007 | `script/check-traceability-tests_test.sh` control, nested-heading, and closed-section fixtures; direct reading of the per-file heading-state loop in `script/check-traceability-tests.sh` |
| TTG-V0-005, TTG-V0-006 | direct reading of the per-line match loop in `script/check-traceability-tests.sh` |
| TTG-V0-008, TTG-V0-009 | `script/check-traceability-tests_test.sh` `path:line: Name` failure reports; direct reading of the summary and failure-report block in `script/check-traceability-tests.sh` |
| TTG-V0-010 | `Makefile` targets `traceability-tests-check` and `traceability-tests-test` and their membership in the `gate` target's prerequisite list |
| TTG-V0-011 | `script/check-traceability-tests_test.sh` qualified fixture: `toolgroup.TestValidity` and `(*v).TestRun` pass, a bare undeclared name of that word on the same row is still reported (the fixture fails on the base checker) |
| TTG-V0-012 | `script/check-traceability-tests_test.sh` untracked and unstaged-edit fixtures: each failure names the working-tree file and `git add`; a never-declared name carries no hint; staging resolves the row (the fixture fails on the base checker) |
| TTG-V0-917 | `script/check-traceability-tests_test.sh` prose fixture: `` `TestLine` (not a test) `` and `TestCol (not a test)` pass, the same word unmarked on the next row is reported, and the marked word is not (the fixture fails on the base checker) |
| TTG-V0-951 | `script/check-traceability-tests_test.sh` retired fixture: a spec with the retired header and an undeclared name passes and is counted; the same body under `Delivery status: implemented` fails |
| TTG-V0-952 | `script/check-traceability-tests_test.sh` removed fixture: an undeclared name marked `(removed)` passes and is counted; a declared name with the marker fails naming the row |
| TTG-V0-950 | `script/check-traceability-tests_test.sh` spec-first fixture: the spec-only index check fails with the range hint, `--range BASE..HEAD` passes once the next commit adds the test and fails for a range that ends before it, and a never-landing test is named once at its first citing commit |
| TTG-V0-960, TTG-V0-961 | `script/check-traceability-tests_test.sh` subtest fixtures: an ID-suffixed citation whose body holds the ID resolves and is counted; a body without the ID fails naming the row; a suffix that is not an ID is checked by name only; an ID that is a prefix of a longer one fails; an ID in the function after a one-line declaration does not count; an ID after a column-0 `}` in a raw string is refused with the read-to note; a second declaration holding the ID resolves; each exempt marker skips the subtest check |
| TTG-V0-962, TTG-V0-963 | `script/check-traceability-tests_test.sh` last-citation fixtures: the range form fails when the only citing test is deleted and its row edited, and passes when a second citation remains, the spec is retired, the requirement text is removed, or the row states `(untested: REASON)`; an empty reason fails; a citation demoted to `(PLANNED)` fails; a stale sibling BASE compares at the merge base; a BASE that is not a commit and a BASE with no merge base are refused with exit 2 |

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| TTG-V0-001, TTG-V0-002 | the indexed `git ls-files` collection and per-file function-name scan in `script/check-traceability-tests.sh` | `script/check-traceability-tests_test.sh` |
| TTG-V0-003, TTG-V0-004, TTG-V0-007 | the per-file heading and active-section state machine in `script/check-traceability-tests.sh` | `script/check-traceability-tests_test.sh`; reading of the script |
| TTG-V0-005, TTG-V0-006 | the per-line name match and the `(PLANNED)` exemption in `script/check-traceability-tests.sh` | reading of the script |
| TTG-V0-008, TTG-V0-009 | the summary print and the failure report in `script/check-traceability-tests.sh` | `script/check-traceability-tests_test.sh`; reading of the script |
| TTG-V0-010 | `traceability-tests-check` and `traceability-tests-test` in `Makefile` and their place in the `gate` prerequisite list | `Makefile` |
| TTG-V0-011 | the selector skip in the per-line match loop of `script/check-traceability-tests.sh` | `script/check-traceability-tests_test.sh` |
| TTG-V0-012 | `unstaged_hint` in `script/check-traceability-tests.sh` | `script/check-traceability-tests_test.sh` |
| TTG-V0-917 | the `(not a test)` skip in the per-line match loop of `script/check-traceability-tests.sh` | `script/check-traceability-tests_test.sh` |
| TTG-V0-950 | the `--range` option and the per-commit loop in `script/check-traceability-tests.sh`; `TRACE_RANGE` in `Makefile` | `script/check-traceability-tests_test.sh` |
| TTG-V0-951 | the retired-header skip at the top of the per-spec loop of `script/check-traceability-tests.sh` | `script/check-traceability-tests_test.sh` |
| TTG-V0-952 | the `(removed)` branch in the per-line match loop of `script/check-traceability-tests.sh` | `script/check-traceability-tests_test.sh` |
| TTG-V0-960, TTG-V0-961 | the subtest-citation branch and `body_has_id` in `script/check-traceability-tests.sh` | `script/check-traceability-tests_test.sh` subtest fixtures |
| TTG-V0-962, TTG-V0-963 | `cited_requirements` and the BASE-against-HEAD comparison of the range form in `script/check-traceability-tests.sh` | `script/check-traceability-tests_test.sh` last-citation fixtures |

## Rollout, rollback, and drift

The gate is already live in `gate`; rollback is removing `traceability-tests-check` from the `gate`
prerequisite list, which leaves every existing spec traceability table unchecked and does not
change any other gate's behavior. Drift rule: a spec traceability row that names a verification
function before that function exists MUST use the `(PLANNED)` marker immediately after the name;
renaming or deleting a cited function without updating or re-marking its row is exactly the drift
this gate exists to catch, so such a change MUST update the citing row in the same commit. Roll back
TTG-V0-011 by removing the selector skip and its fixture; qualified identifiers then fail again.
Roll back TTG-V0-012 by removing `unstaged_hint` and its fixture cases; failures lose the hint only.
Roll back TTG-V0-917 by removing the `(not a test)` skip and its fixture; marked words then fail
again and need rephrasing.
Roll back TTG-V0-950 by removing `--range`, `TRACE_RANGE` and the hint; a spec-first commit can
then use `(PLANNED)` until its test lands.
Roll back TTG-V0-951 and TTG-V0-952 by removing the retired-header skip, the `(removed)` branch
and their fixtures; every row naming a test removed under decision 0594 then fails again and must
be rewritten without the name.
Roll back TTG-V0-960 to TTG-V0-963 by reverting the script and fixture commits of change
`traceability-gate-upgrade`; no stored state or file format is added, and `(untested: REASON)`
text left in a row is then inert.

## Unresolved

Whether false positives from prefix-matching prose (an ordinary capitalized word that happens to
start with the matched prefix, landing in a traceability table row) are common enough to warrant a
narrower match — for example requiring the token be wrapped in backticks — is not decided here;
today the fix is to phrase such rows without that word shape. TTG-V0-011 (V1-3143) settles only the Go selector case
(`pkg.TestName`); TTG-V0-917 (V1-4048) adds the `(not a test)` marker for a bare word that names
no test.
