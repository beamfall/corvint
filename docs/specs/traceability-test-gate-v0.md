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
- Status: accepted (owner instruction 2026-09-07) / implemented
- Exists: `script/check-traceability-tests.sh`, fixture test `script/check-traceability-tests_test.sh`, `make` target `traceability-tests-check` in `gate`.
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
  open across any heading strictly deeper than it.
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
  gate`.

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
| A capitalized table-row word that happens to start with the matched prefix but names no verification function | fail, indistinguishable from a real citation; see TTG-V0-005 |
| An untracked or Git-ignored `_test.go` file declares the only matching function | its declarations are invisible to TTG-V0-001; the citing row fails |
| An unstaged edit adds or removes a declaration in a tracked `_test.go` file | the edit is invisible; the gate continues to evaluate the indexed blob per TTG-V0-002 |

## Acceptance criteria

Acceptance rests on the gate running clean over the current indexed `docs/specs/*.md` corpus as a
`gate` prerequisite and on `script/check-traceability-tests_test.sh` proving that an untracked test
function cannot satisfy an indexed traceability row while staging the same function makes it
visible.

| Requirement | Evidence |
|---|---|
| TTG-V0-001, TTG-V0-002 | `script/check-traceability-tests_test.sh` untracked-then-staged fixture |
| TTG-V0-003, TTG-V0-004, TTG-V0-007 | direct reading of the per-file heading-state loop in `script/check-traceability-tests.sh` |
| TTG-V0-005, TTG-V0-006 | direct reading of the per-line match loop in `script/check-traceability-tests.sh` |
| TTG-V0-008, TTG-V0-009 | direct reading of the summary and failure-report block in `script/check-traceability-tests.sh` |
| TTG-V0-010 | `Makefile` target `traceability-tests-check` and its membership in the `gate` target's prerequisite list |

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| TTG-V0-001, TTG-V0-002 | the indexed `git ls-files` collection and per-file function-name scan in `script/check-traceability-tests.sh` | `script/check-traceability-tests_test.sh` |
| TTG-V0-003, TTG-V0-004, TTG-V0-007 | the per-file heading and active-section state machine in `script/check-traceability-tests.sh` | reading of the script |
| TTG-V0-005, TTG-V0-006 | the per-line name match and the `(PLANNED)` exemption in `script/check-traceability-tests.sh` | reading of the script |
| TTG-V0-008, TTG-V0-009 | the summary print and the failure report in `script/check-traceability-tests.sh` | reading of the script |
| TTG-V0-010 | `traceability-tests-check` in `Makefile` and its place in the `gate` prerequisite list | `Makefile` |

## Rollout, rollback, and drift

The gate is already live in `gate`; rollback is removing `traceability-tests-check` from the `gate`
prerequisite list, which leaves every existing spec traceability table unchecked and does not
change any other gate's behavior. Drift rule: a spec traceability row that names a verification
function before that function exists MUST use the `(PLANNED)` marker immediately after the name;
renaming or deleting a cited function without updating or re-marking its row is exactly the drift
this gate exists to catch, so such a change MUST update the citing row in the same commit.

## Unresolved

Whether false positives from prefix-matching prose (an ordinary capitalized word that happens to
start with the matched prefix, landing in a traceability table row) are common enough to warrant a
narrower match — for example requiring the token be wrapped in backticks — is not decided here;
today the fix is to phrase such rows without that word shape.
