# Corvint dogfood contract

Corvint development uses Corvint as its first context and change-evidence system. This is not a demo
path. The repository must exercise the same local CLI, wire format, limits, failure states, and
documentation contract expected of another project.

## Daily adopter path

This is the one ordered path from a change to a sealed CEM (`DCW-V0-013`). It joins task context,
impact, CEM, OCM, frontier, checks, independent review and the retained outcome. Sections 1 to 7 below
remain the normative detail for each step. A complete report, a passing check and a seal establish
structural closure only: the evidence is bound to immutable revisions and is internally consistent.
They do not establish that the change is correct, that its tests are adequate, or that the project
gate ran (`DCW-V0-015`).

### Inputs

| Input | Exact format | Example |
|---|---|---|
| `BASE` | Full 40-hex commit before the change's first commit; never `HEAD` | `BASE=$(git rev-parse origin/main)` taken when branching |
| `DOGFOOD_TASK` | One sentence describing the change, not project-operations wording (section 1) | `Deliver the documented daily change-evidence adopter path.` |
| `DOGFOOD_INTENTS_FILE` | Path to a file of 1 to 16 repository-relative spec paths, sorted, LF-terminated, no absolute path and no `.` or `..` segment. Each spec exists at `BASE` and has exactly one `## Requirements` heading (`rg -c '^## Requirements' SPEC` prints 1); a spec created in this change cannot be an intent (section 2) | file content `docs/specs/daily-change-evidence-workflow-v0.md` |
| `DOGFOOD_CITATIONS` | Path to a TSV file, not the rows. One `ORDINAL<TAB>PATH<TAB>START:END<TAB>RELATION` row per CEM hunk in `hunks` order, ordinals from 1, LF-terminated, at most 256 rows; the span must exist at `BASE` | row `1	AGENTS.md	26:28	specification` |
| `DOGFOOD_VERIFY_FILE` | Path to a file with one shell-free verification command per line, each at most 512 characters; `DOGFOOD_VERIFY` takes the same lines inline (section 7) | line `go test ./internal/lrfrepo` |
| `DOGFOOD_OUTCOME` | `passed`, `failed` or `blocked` | `passed` |

Every `dogfood-change` refusal caused by one of these inputs prints the step and reason, then a
`fix:` line naming the correction (`DCW-V0-014`).

### Steps and expected state

1. Orient before editing: retain `corvint query` and `corvint impact` receipts (section 1). A miss
   or abstention is recorded in `docs/BUILD-LOG.md`, not repaired by rewording the task.
2. Bind intent, implement, run the focused checks and commit (sections 2 and 3). Only ignored paths
   may remain modified.
3. Export every input except `DOGFOOD_CITATIONS`, then run `make dogfood-change BASE=$BASE`. Expected
   when `BASE` carries no shared `.corvint/change.cem.json` (the normal case once a seal has moved it):
   `dogfood-change: FAIL not-complete` listing only `cem-cite: citation-plan-not-provided` and
   `cem-status: not-ready` (policy issue `max-unknown-exceeded`, because no hunk is cited yet); every
   OCM row and `local-outcome` are produced, and `git status` shows `?? .corvint/change.cem.json`.
   When `BASE` still tracks an earlier unsealed `.corvint/change.cem.json`, this pass replaces it (the
   `CEM-PILOT-018` mismatch line triggers `--replace`), `git status` shows ` M .corvint/change.cem.json`,
   and the modified tracked sidecar adds `ocm-prepare-001: excluded-artifact-mismatch`,
   `ocm-status-001: exit-2` (`not-ready` on a later pass) and
   `ocm-aggregate: intent-scope-drift`; `cem-status` then also refuses it
   with `excluded-artifact-mismatch` (in `<git-dir>/corvint/cem-status.json` it is a
   `verification.issues` code, not a policy issue). The final seal removes the shared path.
4. Write the citation plan from the prepared map. The hunk count is
   `python3 -c "import json; print(len(json.load(open('.corvint/change.cem.json'))['hunks']))"`.
   Export `DOGFOOD_CITATIONS` and rerun `make dogfood-change BASE=$BASE`. `cem-cite` and, for an
   untracked sidecar, `cem-status` are now produced; the only remaining row is
   `local-outcome: record-index-failed`. A modified tracked sidecar additionally keeps the OCM and
   `cem-status` rows of step 3 and adds `prechange-impact: unsupported-impact-worktree`. Each row
   prints its own `fix:` line; `prechange-impact`, `ocm-prepare-NNN`, `ocm-status-NNN` and
   `local-outcome` name the uncommitted worktree, and all of them clear once the sidecar is committed.
5. Commit the sidecar: `git add .corvint/change.cem.json && git commit -m "chore: bind change evidence"`.
   From the first pass on, add commits rather than amending or rebasing: each clean pass records a
   local trace for its `HEAD`, and once that commit is no longer an ancestor of `HEAD` the query and
   the recorder refuse every later pass (`prechange-query: unsupported-query-trace-state`,
   `local-outcome: record-failed`, both reading `local trace store contains unreachable revision`).
6. Run `make dogfood-change BASE=$BASE` again. Expected: no output, exit 0, and
   `.corvint/dogfood-report.json` contains `"complete": true`. An uncited hunk instead leaves
   `cem-status: not-ready` (policy issue `max-unknown-exceeded`).
7. Optionally record requirement evidence: `corvint ocm link` or `corvint ocm mark` on
   `.corvint/change.ocm.NNN.json` (section 5), then rerun step 6 on the same `HEAD` so the aggregate
   includes it. Any later commit regenerates the maps with `--replace` and drops those records.
   Without them every requirement stays `unassessed`, which means "not assessed by this change".
8. Inspect what a reviewer sees: `corvint cem report` and `corvint ocm report` (section 6), and
   `corvint frontier --cem .corvint/change.cem.json --ocm .corvint/change.ocm.001.json
   --expected-base $BASE --target HEAD`, whose exit 1 is a valid open frontier.
9. Run `make dogfood-check BASE=$BASE`. Expected: CEM and OCM status JSON, then
   `dogfood-check: PASS`. A base whose committed CEM names a base outside its history also prints
   `dogfood-check: NOTE unbound-commits NOT_OBSERVED previous-cem-base-unavailable`; the note never
   changes the verdict.
10. Run `make dogfood-seal BASE=$BASE`. Expected: `dogfood-seal: PASS
    sealed=.corvint/changes/<bind-commit>.cem.json` and one rename-only commit.
11. Hand the branch and reports to an independent reviewer (section 6) and keep the outcome
    recorded by step 6 (section 7).

### Fail-closed outcomes

Each class below was reproduced in a scratch clone for V1-0010 unless marked NOT_OBSERVED. None
produces `"complete": true` or `dogfood-check: PASS`.

| Class | Trigger | `dogfood-change` | `dogfood-check` |
|---|---|---|---|
| Dirty | modified tracked or untracked file after the change | runs; the recorder reports `local-outcome: record-index-failed` | `REFUSE dirty-worktree` (exit 2) with the required-order line |
| Stale | commit after the last `dogfood-change` | the rerun returns to step 3 until the sidecar is recommitted | `FAIL dogfood-report-drift`, `fix:` names another base or head |
| Unknown | hunk not cited by `DOGFOOD_CITATIONS` | `cem-status: not-ready` | `FAIL dogfood-report-drift`, `fix:` names an incomplete report |
| Interrupted | `SIGTERM` during a run | exit 143, no report written, no citation stage left, sidecar unchanged | `FAIL dogfood-report-missing`, or `dogfood-report-drift` when an older report exists |
| Interrupted | `SIGINT` (Ctrl-C) | NOT_OBSERVED | NOT_OBSERVED |
| Unsupported | host without `rg` | `REFUSE unsupported-environment-missing-rg` | `REFUSE unsupported-environment-missing-rg` |
| Unsupported | intent without exactly one `## Requirements` heading | `ocm-prepare-001: invalid-requirements-section`, `ocm-aggregate: intent-scope-drift` | NOT_OBSERVED |
| Drift | OCM map marked or linked without a rerun | not applicable | `FAIL intent-scope-drift`, `fix:` reruns `dogfood-change` |
| Rewritten | amend or rebase after a recorded pass | `prechange-query: unsupported-query-trace-state`, `local-outcome: record-failed` and a `local trace store:` line; restoring the commit as an ancestor clears it | not reached |
| Sealed | check on the seal commit, or a change containing a seal | `REFUSE sealed-cem-in-change` | `REFUSE sealed-head` |

Test-claim linkage through `corvint ocm link` is NOT_OBSERVED in this path: it needs a Go test that
names the requirement ID, and the V1-0010 run exercised only `ocm mark`.

## Required loop for substantive changes

Before the first context call, start a private measurement receipt for the task. The harness records
wall latency, complete agent-visible Corvint response bytes, every available billed input/output token,
source opens, caller-authored broad searches, automatic widenings, reviewer-discovered critical
misses, verification state, and final outcome. A value the harness cannot observe is
`NOT_OBSERVED`—never estimated, copied from packet size, or omitted from the denominator.

### 1. Orient with Corvint

Before broad source reading, run the narrowest applicable Corvint queries and retain their JSON under
the private Git directory:

```console
$ corvint_git_dir=$(git rev-parse --absolute-git-dir)
$ mkdir -p "$corvint_git_dir/corvint"
$ corvint query --task "THE CHANGE" --limit 1 > "$corvint_git_dir/corvint/prechange-query.json"
$ corvint impact --base BASE_SHA --limit 20 > "$corvint_git_dir/corvint/prechange-impact.json"
```

Use the native `corvint` runtime. For range impact, `BASE_SHA` is the full immutable commit ID
immediately before the included changes, which must end at captured `HEAD`. For tracked Go paths,
use `corvint impact PATH... --limit 10`. A result limit is not a byte budget: retain the complete
response and every omission/uncertainty. Native impact refuses `--budget-bytes` with
`unsupported-impact-option`; never substitute a legacy runtime.

Range impact refuses a modified path or an untracked path that overlaps the Go build (`GPK-V0-060`),
so `prechange-impact` is then `NOT_PRODUCED unsupported-impact-worktree`.
If Corvint abstains or misses a critical item, continue with ordinary repository inspection and record
the miss in `docs/BUILD-LOG.md`. Never tune the current task into a held-out evaluation.
For `prechange-impact` only, a complete coordinator may retain `NOT_PRODUCED
unsupported-impact-range` as an explicit context abstention. It MUST retain the exact argv bytes,
base, target, exit status, and raw stdout/stderr digests in the private context-abstention artifact;
the strict checker validates those bindings. No other exit, malformed error, missing artifact, or
artifact drift qualifies. This is discovery accounting only: CEM/OCM closure, every selected check,
clean-target validation, and independent base/current verifier agreement remain mandatory.

A clean repository with recorded traces can intentionally refuse an authority-start query with
`unsupported-query-trace-state`. Preserve that failed receipt and follow the explicit
[post-record evidence route](AGENT-ROUTES.md#after-a-trace-state-refusal); do not remove traces or
change the task to force another query profile. A separate `context` result is discovery evidence,
not a replacement success for the refused query. Include both attempts in the task measurement.
`make dogfood-change` passes `DOGFOOD_TASK` as that query task, and its wording alone selects the
profile: a task naming project operations (for example "contributor workflow") selects
authority-start and refuses on a present trace store, while a task describing the change does not.
The identical change can therefore produce or refuse on wording; write `DOGFOOD_TASK` as a
description of the change before the first run. On that refusal the coordinator's `not-complete`
output names the task wording as its subject; the rule above still governs the retry.

### 2. Bind intent before promotion

Create or update the owning spec according to `docs/SPEC-DRIVEN-DEVELOPMENT.md`. A spec may ship in
the same change, but CEM 0.1 can cite only evidence present at its base revision. Such a bootstrap
hunk must cite older governing evidence or remain explicitly unknown; it must never cite itself or
pretend newly inferred intent was already authoritative. A change to this document is bound the
same way: `docs/DOGFOOD.md` states the loop's own obligations, so it is never a legal intent for a
change to itself, and the caller pins the relevant owning spec (for example
`docs/specs/ocm-v0-dogfood.md`) instead.

Corvint repository policy handles an honest base-absent bootstrap unknown only after OCM has validated
the complete intent scope set against the same base, target, CEM map digest, and patch digest. The
current patch grammar gives each created intent path exactly one hunk, so dogfood subtracts the
number of validated intent paths absent at the base before applying its zero-unknown ceiling. Raw
CEM dispositions and counts do not change. Any unknown outside that set still fails policy, and an
OCM failure grants no subtraction. This is decision 0055; no CEM or OCM wire profile changes.

### 3. Implement and verify

Implement against stable requirement IDs. Run focused tests after each logical slice, then the full
suite and compile gate. Run any frozen conformance or benchmark partition owned by the capability.
Failed, synthetic, and `NOT_RUN` results keep those labels.

### 4. Produce the CEM

The final dogfood coordination and checking steps are post-commit. Commit the implementation first,
then use this order; `BASE_SHA` is the commit before the change's first commit, never `HEAD` itself.
When local Git configuration `corvint.dogfood.anchor` names one anchor ref, both commands require
`BASE_SHA` to equal `git merge-base <anchor> HEAD` and refuse `base-not-anchored` otherwise. No
implicit anchor such as `origin/main` is selected. Without a configured anchor, the caller base
remains operative and the private report records the anchor as `NOT_OBSERVED`:

`dogfood-change` and `dogfood-check` require `rg` (ripgrep) on `PATH` to assert on their own evidence
output; a host missing it gets a `REFUSE unsupported-environment-missing-rg` line instead of a
misleading failure partway through the run.

```console
$ make dogfood-change BASE=BASE_SHA
$ git add .corvint/change.cem.json
$ git commit -m "chore: bind change evidence"
$ make dogfood-change BASE=BASE_SHA
$ make dogfood-check BASE=BASE_SHA
$ make dogfood-seal BASE=BASE_SHA
```

`dogfood-seal` reruns `dogfood-check` and, only on PASS, commits `chore: seal change evidence`,
which does nothing but rename `.corvint/change.cem.json` to
`.corvint/changes/<bind-commit>.cem.json`. The CEM is bound and checked at the fixed `cem/0.2`
path, and the seal moves it out of that one shared tracked path, so two open branches never edit
the same file (decision 0319). A sealed HEAD is refused by `dogfood-check` (`sealed-head`); check
its parent instead. `dogfood-change` refuses a change whose `BASE_SHA..HEAD` adds a sealed CEM
(`sealed-cem-in-change`): to rework a sealed branch, revert or drop the seal commit first.

Run `make gate` before the first `dogfood-change` or after the sidecar commit, not between them. Its
archive step requires a clean worktree (`conformance/release-artifact-v0/archive_run.go:422-424`),
and the prepared but uncommitted `.corvint/change.cem.json` is the one tracked path this loop leaves
modified; the OCM maps and the report are ignored paths and do not make the worktree dirty.

The first `dogfood-change` prepares and cites the tracked CEM. That CEM must itself be committed
before strict status can accept it; the `cem/0.2` profile self-excludes the sidecar from its mapped
patch so the commit does not create a recursive self-citation. The second `dogfood-change` resolves
the new `HEAD`; prepare resumes and cite is idempotent, so the worktree stays clean while the report is
regenerated with `"complete": true`. `dogfood-change` reruns prepare with `--replace` only when
prepare refused the existing map with the exact `CEM-PILOT-018` base-or-patch mismatch line; every
other prepare failure, including a transient `git-timeout`, is reported as `cem-prepare`
`NOT_PRODUCED` with its reason and leaves the committed map untouched for the operator to rerun.
`dogfood-check` then checks that committed change from a clean
worktree. Running it before commit is a workflow refusal, not evidence that there was no change.
`dogfood-change` builds `./cmd/corvint` from the current tree into the worktree's private Git
evidence directory and runs that binary. Every step publishes its stdout as
`<git-dir>/corvint/<step>.json` and its stderr as `<git-dir>/corvint/<step>.stderr`, so the refusal
envelope behind a reported reason is readable beside that step's output. The run clears
unpublished `*.stderr` from that directory at start, so no envelope there survives an earlier run.
The report's `packetCoverage` line (`DCW-V0-016`) states the cost of the two context packets the
run compiled: for `prechange-query` and `prechange-impact` in that order, the packet's own
`packet_bytes`, `budget_bytes`, `within_budget`, `included_results` and `omitted_results`, or
`NOT_PRODUCED` with `packet-not-compiled` (the step compiled none, including the
`unsupported-impact-range` abstention) or `packet-coverage-unreadable`. It never changes
`complete`, and reports written before it existed omit it.
`dogfood-check` independently builds one verifier from the current clean tree and one from a
private `git archive BASE_SHA`, runs both OCM and CEM status with
identical inputs and effective policy, and requires byte-identical stdout, stderr, and exit status.
Disagreement fails `verifier-disagreement`. The report records both binary SHA-256 digests and the
agreement result. Both verifiers build with `-trimpath`, so a digest depends on the source and
toolchain rather than on the private extraction directory. An explicit `CORVINT_BIN` override is accepted only when its `--version` output
is `Corvint VERSION (build N)` with `VERSION` matched exactly (decision 0314); during final checking it is an additional verifier that must agree
and whose digest is also recorded.

Because `.corvint/change.cem.json` is one tracked path and each change's base is the commit before
its first commit, interleaved sessions can leave committed work that no CEM binds. For a clean
worktree with a change, `dogfood-check` reads the `baseRevision` of the previous binding: the CEM
committed at `BASE_SHA`, or, when `BASE_SHA` has none, the CEM in the parent of the newest seal
commit reachable from it. It examines the non-merge commits reachable from `BASE_SHA` but
not from that revision. A commit there is bound when some CEM committed in that window covers it:
the sidecar commit and the commits after that CEM's own `baseRevision`. A seal commit, one that
only renames its parent's CEM to `.corvint/changes/<parent>.cem.json`, is covered by that parent;
any other commit that removes the CEM is not a seal. Every other commit is
reported on stderr as `dogfood-check: NOTE unbound-commits count=N window=PREV..BASE`, followed by
one `  unbound SHA` line per commit, newest first; nothing is printed when all are bound. When the
base has no committed CEM, a CEM in the window has no parseable ancestor `baseRevision`, or the
window exceeds 256 commits, it prints `dogfood-check: NOTE unbound-commits NOT_OBSERVED REASON`
(`previous-cem-absent`, `previous-cem-base-unavailable`, `window-cem-base-unavailable`,
`window-unavailable`, or `window-exceeds-256-commits`) instead of guessing. The note never changes
the check's verdict or exit status. It looks only at the window before this base, so a gap is
reported by the check of the first change that follows it; merge commits and history before the
previous binding are not examined. A commit covered only by a retroactive binding (below) is not
unbound and is not normally bound: it is reported on stderr as
`dogfood-check: NOTE retroactive-bound-commits count=N window=PREV..BASE`, followed by one
`  retroactive SHA binding=BINDING_SHA` line per commit, newest first, naming the newest such
binding. A normal binding of the same commit takes precedence. The retroactive label is read from
the binding commit's trailer and is not re-verified by the check, just as window CEMs are not.

#### Bind a landed range after the fact

A reported gap `BASE_SHA..TARGET_SHA` that has already landed is bound, never rewritten, with:

```console
$ DOGFOOD_CITATIONS=PLAN [DOGFOOD_UNKNOWN=UNKNOWN] script/dogfood-bind-range.sh BASE_SHA TARGET_SHA
$ git merge --no-ff -s ours -m "chore: merge retroactive binding LABEL" BINDING_SHA
```

The normal flow cannot do this: `cem prepare` writes only the fixed map path, and a mid-history
target already carries an older sidecar that `CEM-CB-009` requires to equal the verified map.

- `DOGFOOD-BIND-001`: the script refuses with exit 2 and `dogfood-bind-range: REFUSE REASON`, before
  any CEM work, when a revision does not resolve (`base-unavailable`, `target-unavailable`), the range
  is empty (`empty-range`), BASE is not an ancestor of TARGET (`base-not-ancestor-of-target`),
  TARGET is not an ancestor of `HEAD` (`target-not-landed`), the range exceeds 256 commits
  (`range-exceeds-256-commits`), `DOGFOOD_CITATIONS` names no file
  (`citation-plan-unavailable`), or `DOGFOOD_UNKNOWN` names no file (`unknown-plan-unavailable`). `CORVINT_BIN` follows the coordinator's version rule
  (`corvint-version-mismatch expected=V`); without it the current tree is built
  (`current-tree-corvint-build-failed`). The 256-commit refusal is not a promise that one piece fits:
  size each piece for both the 256-row citation-plan limit
  (`script/dogfood-bind-range.sh:93-100@3b3a27f5`) and the verifier's 1,024-logical-Git-operation
  budget (`internal/cem/gitrun/gitrun.go:30-51@524eb1ce`). For the current object-identity and
  tree-walk checks, use roughly 5.7 operations per changed path plus 10 per evidence record as a
  planning estimate; path depth, shared objects, and request-memo hits change the exact count, so
  reduce the piece before either bound rather than relying on the commit count.
  When a first-parent segment's TARGET precedes the merge that introduced the reported gap BASE onto
  that branch, the gap BASE is not the segment TARGET's ancestor. Use
  `git merge-base GAP_BASE SEGMENT_TARGET` as the segment BASE. The later piece ending at the merge
  then binds the merged-in `MERGE_BASE..GAP_BASE` changes as well as its first-parent changes; size
  and cite the complete canonical patch rather than omitting those foreign commits.
- `DOGFOOD-BIND-002`: preparation and citation run in a private, never-checked-out Git worktree at
  TARGET under `<git-dir>/corvint/`. The main worktree, its tracked sidecar, and
  `.corvint/dogfood-report.json` are never written, and the private worktree is removed on every exit,
  including interruption (exit 129, 130, or 143).
- `DOGFOOD-BIND-003`: the citation plan has the coordinator's row format and limits and is written
  against this range's uncited prepared map, published as
  `<git-dir>/corvint/bind-range.B12..T12.cem.json`. Without a plan the script reports
  `cem-cite NOT_PRODUCED citation-plan-not-provided`. A range needing more than one plan's rows is
  split into contiguous sub-ranges; nothing is split or cited automatically.
- `DOGFOOD-BIND-004`: the binding commit has TARGET as its only parent, TARGET's tree with only
  `.corvint/change.cem.json` replaced by the cited map, and the trailer
  `Corvint-Dogfood-Binding: retroactive`. Its canonical patch is therefore exactly BASE..TARGET.
  `cem status --expected-base BASE --target BINDING --max-unknown N --max-mechanical 0` must pass,
  where N is the row count of the `DOGFOOD-BIND-007` unknown plan (0 without one);
  its stdout is published as `<git-dir>/corvint/bind-range.B12..T12.cem-status.json`.
- `DOGFOOD-BIND-005`: a retroactive binding never claims pre-change context or a local outcome. A
  passing run reports `NOT_PRODUCED retroactive-binding` for `prechange-query`, `prechange-impact`,
  `local-outcome`, and `ocm-aggregate`, then prints
  `dogfood-bind-range: PASS retroactive binding=SHA range=BASE..TARGET` and the `next:` merge
  command. The script creates no ref and performs no merge; the operator decides to land it.
- `DOGFOOD-BIND-006`: merging the binding with the `ours` strategy changes no tracked content, so
  the current sidecar and every later change's base are unaffected, and the range becomes reachable
  as a retroactive binding for `dogfood-check`.
- `DOGFOOD-BIND-007`: a hunk whose intent no stable base content records (for example an intent
  added or removed inside the range) is not cited. A fixing commit that removes its backlog entry
  MUST cite a stable owning spec requirement or accepted decision instead. The producer reads a
  requested span from `baseRevision` (`internal/cem/workflow/commands.go:304-321@d53a3b0c`), but
  both its stability precheck and canonical target-drift verification reject that removed span
  (`internal/cem/verify/verify.go:425-480@92b33007`). A backlog entry that was the only recorded
  intent therefore binds as `NOT_PRODUCED`, never as an
  invented citation, with an unknown `no-evidence` row. Its detail is the
  `removed-intent.<blob OID>.<start>-<end>` pin that the refused `cem cite` names (`CEM-CB-005`,
  decision 0165); the script accepts that detail only when the OID is a regular blob in BASE's tree
  and the byte span is nonempty and inside it, else the plan is malformed. The pin locates the
  removed intent in immutable base content; it is not evidence, and the hunk stays `unknown`.
  Intent with no base content at all keeps an `intent-unfiled-...-no-base-clause` detail.
  `DOGFOOD_UNKNOWN` names a plan of distinct
  `HUNK<TAB>REASON<TAB>DETAIL` rows, with the citation plan's limits, a registered `cem mark`
  unknown reason, and a lower-case `[a-z0-9.-]` detail token. After citation each row is marked
  `unknown`, the script prints `dogfood-bind-range: NOTE cem-mark NOT_PRODUCED hunk=H reason=R
  detail=D`, and the binding commit message carries the same `NOT_PRODUCED` line per row before its
  trailer. A malformed plan fails as `cem-mark invalid-unknown-plan`.

Failure modes: a failing prepare, citation, binding-commit, or status step exits 1 with
`dogfood-bind-range: FAIL STEP REASON` (`cem-prepare`, `cem-cite` including
`invalid-citation-plan`, `cem-mark` including `invalid-unknown-plan`, `binding-commit-failed`, `cem-status`, or `cem-policy` after printing the
status line) and prints no `PASS` or `next:` line. A failure after the binding commit is written
leaves only an unreferenced commit object that `git gc` may prune. A citation that no longer
survives the change, or an uncited hunk absent from the unknown plan, fails status rather than
being marked.

Rollback: before the merge, drop the printed binding (no ref names it) and the private
`bind-range.*` evidence. After the merge the binding stays in landed history, and a
`git revert -m 1` changes no content, so the range stays reported as retroactively bound; withdraw
the claim by recording the correction in the change review, since rewriting landed history is out
of scope. Reverting `script/dogfood-bind-range.sh` and the `dogfood-check.sh` note restores the
earlier report, in which such a range is `unbound` again.

The coordinator accepts a caller-selected citation plan through `DOGFOOD_CITATIONS`. Each row is
exactly four nonempty TAB-separated fields: hunk ordinal or ID, base evidence path, inclusive line
span, and relation. A base evidence path means the producer reads the span from the CEM base tree;
it does not make base-only evidence admissible. The cited span must survive the change: cite base
lines the change leaves in place, because a span the candidate patch deletes or duplicates fails
the producer's `cite-span-not-stable` precheck. A hunk ID is derived from that hunk's content, paths, and line
ranges (`internal/cem/wire/canonical.go:68-79`), and an ordinal is its position in the canonical
worklist, so any later commit other than the sidecar commit can change both. Write the plan from the
map prepared for the final implementation commit (a run without `DOGFOOD_CITATIONS` prepares it and
reports `cem-cite NOT_PRODUCED citation-plan-not-provided`), and rewrite it after any further commit;
a row written against an earlier map refuses `unknown-hunk-id` or names a different hunk. Rows end in LF; other control bytes are invalid. The local coordinator freezes
and validates the whole file before citing, with independent limits of 4 MiB and 256 rows. An empty
file is a zero-citation no-op; normal CEM status still checks the map's completeness. Larger jobs
require separate explicit bounded plans, without automatic splitting or invented citations.

After preparation, a multiple-row plan applies its earlier rows to a unique private staging map.
Only the final successful public `cem cite --output .corvint/change.cem.json` publishes the complete
citation result. Invalid syntax, evidence or publication leaves the prepared map's citation bytes
unchanged. A one-row plan uses the ordinary direct update. This boundary covers citations only:
it does not roll back preparation, outcome recording or OCM. Interrupted runs retain either the
prepared map or the fully cited map; owned staging files are removed before OCM/status and on exit.
Private per-row diagnostics truthfully name the staging map until the final publication, so those
intermediate envelopes differ from direct citation despite identical successful final map bytes.

Publication uses the existing native cite publisher's relative-path validation, static symlink and
non-file rejection, private temporary files and atomic rename. It adds no locking, descriptor-held
race protection, fsync, crash-durability or concurrent-writer guarantee. Run one coordinator per
candidate. The public CLI's evidence validation and refusal codes remain authoritative.

The underlying CEM phase derives the exact committed base-to-target patch:

```console
$ corvint cem prepare --base BASE_SHA --target HEAD \
    --map .corvint/change.cem.json
$ git add .corvint/change.cem.json
$ git commit -m "chore: bind change evidence"
$ corvint cem status --map .corvint/change.cem.json \
    --expected-base BASE_SHA --target HEAD \
    --max-unknown BOOTSTRAP_UNKNOWN_COUNT --max-mechanical 0
```

Use worklist ordinals with `cem cite` or `cem mark` before the sidecar commit. Zero is the default
Corvint-repository policy for unknown and mechanical hunks; an exception must be explicit in the
change review. `BOOTSTRAP_UNKNOWN_COUNT` is zero unless the complete OCM aggregate validates; it is
the validated base-absent intent count, so the effective post-subtraction ceiling remains zero.
Preparation ignores an inherited target-side sidecar, then its returned literal
`--target HEAD` action independently resolves the final revision after the candidate is committed.
The sidecar is excluded from its own patch, avoiding recursive self-citation.

### 5. Close every owning-spec obligation

For a supported owning spec, create the local OCM reviewer artifact against the same target and CEM:

```console
$ corvint_target_sha=$(git rev-parse HEAD)
$ corvint_base_sha=BASE_SHA
$ corvint ocm prepare --target "$corvint_target_sha" --expected-base "$corvint_base_sha" \
    --intent docs/specs/OWNING-SPEC.md \
    --cem .corvint/change.cem.json --map .corvint/change.ocm.json
$ corvint ocm status --map .corvint/change.ocm.json --cem .corvint/change.cem.json \
    --expected-base "$corvint_base_sha" --target "$corvint_target_sha"
```

Use `ocm link` to bind each exact requirement to supported CEM hunks and re-extractable test claims,
or `ocm mark` to preserve an honest unknown. A requirement the change does not touch stays
`unassessed`: that reason means "not assessed by this change" and needs no mark (decision 0029). A
Go test anchors a requirement by naming its ID in a table case `name` or a `t.Run("...")` literal.
Include a descriptive word with the ID: numeric-only suffixes disappear during selector
normalization, so multiple bare IDs in one test function can collide. A whole-function selector
whose function-name anchor lacks the exact requirement ID is not a substitute. OCM is local-only in V0 and must not be added to its own
mapped patch. Structural closure remains distinct from whether the project gate ran or passed.

### 6. Review what another reviewer sees

```console
$ corvint cem report --map .corvint/change.cem.json \
    --expected-base BASE_SHA --target HEAD --max-unknown 0 --max-mechanical 0
$ corvint ocm report --map .corvint/change.ocm.json \
    --cem .corvint/change.cem.json --expected-base "$corvint_base_sha" \
    --target "$corvint_target_sha"
```

Read the report rather than trusting its exit code alone. It performs the same structural and
policy verification as strict status; standalone `verify` is reserved for machine/CI use. The
report stays local by default because
paths and digests are sensitive even though source and diff bodies are omitted.

### 7. Record the outcome

After all gates complete, dogfood coordination classifies the complete base-to-target Git changed
path set through the producer's exact target-revision admission. It preserves that complete set and
its digest in private dogfood evidence, then records all and only the admitted source subset with the
verification commands and `passed`, `failed`, or `blocked`. The coordinator reads those commands
from `DOGFOOD_VERIFY`, one command per line with blank lines skipped, or from the file named by
`DOGFOOD_VERIFY_FILE` (same shape; it takes precedence, and a missing file emits `local-outcome
NOT_PRODUCED verify-file-unavailable`). Each line becomes one recorder `--verify`, so the recorder's
512-character, shell-syntax-free grammar applies per command; a refused command names its first
offending byte and the admitted set. The outcome comes from `DOGFOOD_OUTCOME`. The commands and
the outcome must both be non-empty, or the step records nothing and emits `local-outcome
NOT_PRODUCED outcome-input-not-provided`. A successful classification with no
admitted source emits `local-outcome NOT_PRODUCED no-source-paths`, writes no trace, and does not by
itself fail coordination or final checking. It is not verification or learning evidence. Malformed
input, drift, and every other classification, record, or coordination failure remain fail-closed.
The local trace remains ignored and bounded. Record only material product friction in the build log:
missing context, excessive steps, unclear errors, false hard failures, unsafe output, or work that
required rereading raw source despite a supposedly sufficient packet.

Close the measurement receipt only after review. Compare full multi-turn agent token usage and task
correctness against a preregistered equal-tool baseline when one exists. Until paired baseline data
exists, report raw observations and product misses only; do not claim token savings from a single
dogfood run.

## Enrolled local completion policy

The separate [local completion policy](specs/local-completion-policy-v0.md) coordinates the loop
above for an explicitly enrolled change. Freeze a private JSON plan with the immutable `base`,
sorted owning `intents`, and nonempty `checks` (`id`, `argv`, `timeoutSeconds`, optional
`allowCemSidecarOnlyReuse`, default false). Use the repository's selected required gates; selection
is a review responsibility. Run `corvint dogfood begin --plan FILE` before work and preserve the
original context receipts. One active session owns a worktree because legacy coordination outputs
are shared. A different session must use its own worktree or wait for explicit completion/cancellation.

Within the same task, resume with `corvint dogfood status`; the unkeyed examples below use the
current `CODEX_THREAD_ID` or `CODEX_SESSION_ID`. For a handoff of the same enrolled work, retain the
original worktree and explicit 64-hex session key from its emitted `nextActions`. Resume there with
`corvint dogfood status --session-key KEY`. Every subsequent dogfood invocation must retain that
key: use the returned keyed `nextActions` for verify, review and finish instead of the unkeyed
examples below. Key selection occurs on each invocation. `inactive` for a new key says nothing
about the original enrollment. Do not infer another owner, re-enroll, cancel or clear state to
resume. If the original handle is missing, report that blocker. This preserves the existing
enrollment; it does not authorize taking over unrelated work (LCP-V0-002/003).

Before handing off, the sending session runs `corvint dogfood handoff --session-key KEY --anchors
"ANCHOR..."` with the task's requirement IDs, paths or symbols as anchors, and passes the emitted
document to the receiver. It names the key, root, bound revision, anchors, the dogfood prompt
packet's SHA-256 and bytes, and the degradation list. The receiver runs `corvint dogfood handoff
--session-key KEY --receipt FILE` before relying on its context. Exit 0 (`reresolved`) returns
`packetBase64`, whose decoded bytes hash to the receipt digest. This is the handoff packet compiled
from the anchors alone, not the packet of an earlier prompt event.
Exit 1 (`drifted`) lists the exact root, revision, enrollment, anchor or
packet difference and withholds the packet: report that drift and decide explicitly rather than
treating a recompiled context as the one handed over. Both steps are read-only. The receipt is
untrusted data with `authority: none` and satisfies no completion condition (SESSION-V0-017..019).

After implementation is committed, bind and commit the CEM sidecar with the existing commands.
Then prepare and link every OCM scope against that exact clean target; OCM preparation verifies the
committed CEM bytes. On that target, `corvint dogfood verify --check ID`
executes each selected check and captures its actual result. Checks require the exact tested commit unless their frozen plan
explicitly allows reuse across a validated CEM-only change. No source/spec drift qualifies.

`corvint dogfood finish` returns missing work or a digest-bound set of reviewer reports and check
observations. Read every report, assess citation meaning and test adequacy, and acknowledge only
that exact set with `corvint dogfood review --report-set DIGEST`. A subsequent `finish` supplies
deterministic coordinator inputs, records the selected-check outcome and runs strict independent
verification. Status becomes satisfied only after the checker succeeds and bindings remain current.
The final checker is outside the selected prerequisite check set. Coordinator `complete` and
`outputsAgree` alone are insufficient. Original pre-change receipts stay archived; a later
coordination query is never evidence of earlier chronology.

`status` and native events are read-only. An enrolled incomplete Stop permits one remediation;
recursive Stop releases with an unresolved notice. `cancel` explicitly records non-success and is
not a way to disguise a blocked gate. All local observations and inspection remain caller-owned;
they do not create execution authority, close a Frontier, or qualify the native host matrix.

## Dogfood acceptance

- `DOGFOOD-001`: every substantive Corvint implementation has an owning accepted or experimental spec.
- `DOGFOOD-002`: the pre-change query and impact receipts are generated before implementation when
  the installed revision supports them; misses remain visible.
- `DOGFOOD-003`: the final committed diff has a valid CEM and a manually inspected reviewer report.
- `DOGFOOD-004`: Corvint's own policy caps mechanical hunks and unknown hunks after subtraction of
  OCM-validated base-absent bootstrap intent hunks at zero unless the review explicitly records an
  exception; raw CEM unknowns remain visible.
- `DOGFOOD-005`: tests, conformance, frozen evaluations, and local outcome recording use the same
  public commands documented for adopters.
- `DOGFOOD-006`: self-use never fills an independent producer/consumer matrix cell and never proves
  the 30×30 product-value gate.
- `DOGFOOD-007`: every supported owning-spec requirement is linked to verified change and test-claim
  witnesses or remains explicitly unknown in a local OCM; linkage never implies a passing test.
- `DOGFOOD-008`: every substantive run has a private measurement receipt; agent-visible bytes, tokens,
  latency, source opens, manual broad searches, critical misses, and outcome are observed or explicitly
  `NOT_OBSERVED`.
- `DOGFOOD-009`: Corvint output is compact by default for machine consumers. Packet budgets are checked
  against the complete agent-visible response, and token savings are claimed only from paired
  full-session measurements with non-inferior correctness and no added critical miss.
- `DOGFOOD-010`: final checking reports every non-merge commit between the previous committed CEM's
  base and the change's base that no committed CEM in that window binds, or states why that set was
  not observed; the report is visible and does not alter the verdict.
- `DOGFOOD-011`: a landed commit range can be bound to one CEM after the fact only through an explicit
  binding commit labelled `Corvint-Dogfood-Binding: retroactive` whose pre-change context and local
  outcome are `NOT_PRODUCED`. Each piece respects the citation-row and verifier-operation bounds,
  uses an ancestry-valid merge base across merges, and cites stable owning intent or records
  backlog-only intent as `NOT_PRODUCED` (`DOGFOOD-BIND-001` to `DOGFOOD-BIND-007`).
- `DOGFOOD-012`: final checking reports commits covered only by a retroactive binding as
  `retroactive-bound-commits`, distinct from both normally bound and unbound commits.
- `DOGFOOD-013`: a checked change's CEM leaves the shared tracked path only through `dogfood-seal`,
  which commits after a passing check and only renames `.corvint/change.cem.json` to
  `.corvint/changes/<bind-commit>.cem.json`; a sealed HEAD is refused by final checking and a change
  that adds a sealed CEM is refused by `dogfood-change`.
- `DOGFOOD-014`: final checking counts a seal commit as bound by its parent and, when the base has no
  CEM, takes the previous binding from the parent of the newest seal reachable from the base.

## Feedback rule

One confusing manual step is a usability finding. Repeated manual glue is a missing product
primitive. Add the smallest primitive only after the dogfood trace shows where it saves time or
prevents an error. Do not add a daemon, database, UI, hosted service, or new protocol merely to make
the self-test look sophisticated.

For Corvint itself or explicit repository adoption, use [the shared stage guide](SELF-DEVELOPMENT.md)
to select applicable existing feature routes around this loop. Retain real outputs and concrete
exclusions; the guide adds no mandatory all-feature loop, new receipt schema or closing authority.
