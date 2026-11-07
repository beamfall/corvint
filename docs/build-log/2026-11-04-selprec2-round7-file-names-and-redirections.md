<!-- SPDX-License-Identifier: Apache-2.0 -->
# Selection precision 2, round 7: runtime-rooted file names, redirections and empty compounds

Date: 2026-11-04. Lane selprec2, on `8f8b0686f`, merged with base `6f12f4704` as `5dbd61a0c`.
Tickets: V1-4173, V1-4437.
Requirements: AFP-V0-2203, AFP-V0-2204, AFP-V0-2240 and AFP-V0-2243 amended in place; no new IDs.
Orchestrator-made, owner-delegated decisions, taken in-task: soundness over precision.

## Independent findings

The independent review of `8f8b0686f` returned three P1 findings:

1. P1, planner and rule (c): round 6 still placed a single-component anchored run by the
   package's literals. The run `"/lib.js"` joined to a root taken at run time
   (`os.ReadFile(os.Getenv("FIXTURES") + "/lib.js")`) reads `app/x/repo/lib.js`, yet its package
   was dropped for that path.
2. P1, shell: an empty compound (`case x in esac`) leaves no command of its own. In
   `tmp=$(mktemp -d) && case x in esac & wait` the list scan saw no compound member and missed
   the `&` that backgrounds the assignment, so the name became a temp root.
3. P1, shell: a redirection on the assignment (`tmp=$(mktemp -d) 3>&9 || :`) can fail. dash
   then skips the assignment and keeps an inherited `tmp`, so `$tmp/internal/x/y.json` is a
   repository path.

Both shell cases were confirmed by running them under bash and dash: each read the repository
file.

## Decisions

- File names (AFP-V0-2203, AFP-V0-2240): an anchored single-component run that is the dirty
  path's file name names it at any depth, in the planner (`namesFileOnlyWithin`) and the CI
  selector (`namesWithin`) together. A single-component run that only matches a directory
  component (`"/repo"`) is still resolved from a base or placed. That keeps the narrowing the
  AFP-V0-2203 measurement recorded and leaves the case inside the accepted V1-0290 and
  AFP-V0-072 lone-token limitation, which the failure modes now name. Rejected: withdrawing the
  anchored placement altogether, and treating every `os.Getenv` join as an unbounded reader
  (roots passed as parameters are no better bounded). The unanchored analogue
  (`filepath.Join(os.Getenv("FIXTURES"), "lib.js")`) stays the existing AFP-V0-072 residual.
- Redirections (AFP-V0-2204): an assignment command with any word besides the assignment (a
  redirection before or after it) is no temp root.
- Empty compounds (AFP-V0-2243): the list scan refuses a member whose lead operator is not the
  previous member's trail. The only way they differ is that a command vanished between them, as
  an empty `case` does.
- Lexer: a `&&` or `||` operator advanced the reader by one character, so the second `&` or `|`
  ended an empty command and the next command's lead became `&` or `|`. The reader now moves past
  the whole operator. Lead consumers (`unconditionalAssignment` and the fresh-name check in
  `fresh.go`) refuse `&&`, `||`, `&` and `|` alike, so only the new lead comparison depends on it.
- Base: the lane head was not a descendant of base `6f12f4704`, so the base was merged in. The
  conflicts in `internal/langplugin/decl/decl.go` and `internal/langplugin/langplugin.go` were
  resolved as the union of both TypeScript path lists (`internal/browsercov` and
  `internal/webdialect`).

## Acceptance evidence

- `TestRuntimeRootedFileNamesStaySelected_AFPV02203` (planner) and
  `TestRuleCRuntimeRootedFileNames_AFPV02240` (CI selector index and verdict): packages that
  actually read `os.Getenv("FIXTURES") + "/lib.js"`, `"/other.js"` and `"/repo"`. Only the
  `"/lib.js"` reader is selected for `app/x/repo/lib.js`.
- Amended: `TestRuntimeRootedRunsKeepComponentRuns_AFPV02203` and
  `TestRuleCRuntimeRootedRuns_AFPV02240` now keep the lone `"/lib.js"` holder.
  `TestFileNameOnlyReadersMustPlaceThePath_AFPV0072` now selects the anchored
  `root + "/README.md"` package for every `README.md` probe.
- `TestEmptyCompoundsAndRedirectedAssignmentsKeepTokens_AFPV02243` (`internal/liveverify/affected/shell`)
  has ten cases that keep the token (empty `case` in the background under bash and dash,
  across a newline, after `||`, with an empty arm and after a further list member; redirections
  `3>&9 ||`, `3>&9 &&`, `2>/dev/null` and a leading `3>&9`). Three controls still make temp roots:
  the empty `case` on its own line, a list continued over a newline, and a redirection on the
  right side. Three of the cases also run under their shell with `tmp=.` and must print the
  repository file.
- Fault injection: with `git stash` restoring the pre-fix `naming.go`, `readers.go`, `shell.go`
  and `lex.go`, nine of the ten kept cases of the new shell test fail; the tenth (a further list
  member before the `&`) was already refused by the round-6 scan and stays as a guard. Both
  `AFPV02240` tests fail (verdict missing `lone`, and `readers(app/x/repo/lib.js) = []`), both
  `AFPV02203` tests fail, and the amended `AFPV0072` test fails on all three `README.md` probes.
- These full packages pass through `script/heavy.sh`, one package per command:
  `internal/liveverify/affected/shell`, `internal/liveverify/affected/golang`,
  `internal/liveverify/affected`, `internal/gateselect`, `tools/gate-affected-select`,
  `tools/gate-ledger`, `tools/unbounded-readers`, `internal/sessiontest`, `internal/langplugin`
  and `internal/liveverify/affected/languages`.

## Limits

`fresh.go` (TSP-V0-004) is suspected to have the same redirection, empty-compound and background
weaknesses for fresh names; it is not changed here. The repository selection widening is
NOT_OBSERVED. `make gate` and `cmd/corvint` are NOT_RUN under the scoped instructions. The CI
selector reaches CI only with the next `CORVINT_PR_TOOL_SOURCE` re-pin. Rollback: revert this
round's code commit. Round 7 only widens selection.
