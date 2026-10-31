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

### Commands

In any Git repository, run the steps from the installed binary at the worktree top level:
`corvint dogfood change $BASE`, `corvint dogfood check $BASE` and `corvint dogfood seal $BASE`
(`DCW-V0-020`). A merge of changes sealed that way is checked with the read-only
`corvint dogfood batch-check $BASE` (`DCW-V0-039`, section 4 "Check a merge batch of sealed
changes"). They read the inputs below from the environment, need no script, `VERSION`, Go
toolchain or `rg`, and use the running binary for every step and verifier. A subdirectory refuses
`not-repository-root` (`DCW-V0-021`). Before the first pass, ignore the private outputs and the
local trace store (`DCW-V0-023`); without them the worktree is dirty or the recorder refuses
`repository-identity-changed`. The entries, in `.gitignore` or `.git/info/exclude`, are:

```text
.corvint/dogfood-report.json
.corvint/change.ocm-intents
.corvint/change.ocm-status.json
.corvint/change.ocm.*.json
.corvint/self-observations.jsonl
.context-corvint/
```

Build a helper binary into scratch, `go build -o "$TMPDIR/NAME" ./cmd/NAME`, never with a bare
`go build ./cmd/NAME`, which writes `NAME` into the worktree, where the recorder counts it as
dirty; `dogfood change` names such an untracked `NAME` or `NAME.exe` beside a `cmd/NAME` directory
before it prepares (`DCW-V0-386`).

In Corvint's own tree the `make dogfood-change`, `make dogfood-check` and `make dogfood-seal` targets
used below wrap the same subverbs: they also require `VERSION` to match `CORVINT_BIN` and build
the current (and, for the check, base) tree first (`DCW-V0-022`). Elsewhere, read each
`make dogfood-X BASE=$BASE` below as `corvint dogfood X $BASE`. The `fix:` and
`required order:` lines the subverbs print name that subverb form in every repository (V1-0261).

Run the loop in a clone that owns its objects. A `git clone --reference` or `--shared` clone borrows
them through `.git/objects/info/alternates`, and every step refuses `unsupported-object-alternates`.
To repair it, run `git repack -a -d`, delete `.git/objects/info/alternates` and
`.git/objects/info/commit-graphs`, run `git commit-graph write --reachable`, then rerun (V1-0272).

### Inputs

| Input | Exact format | Example |
|---|---|---|
| `BASE` | Full 40-hex commit before the change's first commit; never `HEAD` | `BASE=$(git rev-parse origin/main)` taken when branching |
| `DOGFOOD_TASK` | One sentence describing the change, not project-operations wording (section 1) | `Deliver the documented daily change-evidence adopter path.` |
| `DOGFOOD_INTENTS_FILE` | Path to a file of 1 to 16 repository-relative spec paths, sorted, LF-terminated, no absolute path and no `.` or `..` segment. Each spec has exactly one `## Requirements` heading (`rg -c '^## Requirements' SPEC` prints 1) and normally exists at `BASE`. A spec this change creates MAY be listed as a same-change bootstrap intent scope (`OCM-V0-009`, decisions 0055 and 0170; section 2): its matching CEM hunk MUST stay explicitly `unknown` and never counts as historical evidence (so `DOGFOOD_CITATIONS` gives it no row or an `unknown` row; `dogfood inputs-check` refuses a row that cites it, `bootstrap-intent-not-unknown`, before any bind, `DCW-V0-449`), and dogfood subtracts it from the unknown ceiling only after every OCM map and the aggregate verify against the same exact base, target, map digest and patch digest; any OCM failure grants no subtraction. When no such spec governs the change, the file instead holds exactly the one line `#no-intent-declared` (`DCW-V0-024`): no OCM step runs, `ocm-prepare`, `ocm-status` and `ocm-aggregate` report `no-intent-declared`, `ocmStatus` is `NOT_ASSESSED`, and the check prints `dogfood-check: NOTE intent-linkage NOT_ASSESSED no-intent-declared`; an unset variable or an empty file still refuses `missing-intent-scope` | file content `docs/specs/daily-change-evidence-workflow-v0.md` |
| `DOGFOOD_CITATIONS` | Path to a TSV file, not the rows. `HUNK<TAB>PATH<TAB>START:END<TAB>RELATION` rows, `HUNK` being a hunk's ordinal in `hunks` order (from 1) or its full hunk ID; cite each hunk with the base lines that hunk itself depends on or implements, and a hunk cited against several bases takes one row each; one span cited for every hunk of a non-chore change with at least two hunks refuses `citation-plan-uniform` (`DCW-V0-048`); LF-terminated, at most 256 rows; the span must exist at `BASE`, and `PATH` may be any file there, code and tests as well as specs (`DCW-V0-041`). Only the hunk of an intent spec absent at `BASE` may be left out; leaving out any other unknown hunk, or an ordinal above the hunk count, refuses the whole plan (`DCW-V0-019`). A hunk no base evidence supports takes `HUNK<TAB>unknown<TAB>REASON<TAB>NOTE` instead, `REASON` being `insufficient-evidence` or `conflicting-evidence` and `NOTE` a one-line reason; it stays `unknown` with that reason, is admitted, and is counted in the check's `unknownShare` (`DCW-V0-046`). An intent left for later takes `deferred<TAB>PATH<TAB>REASON` (at most 4 rows, `REASON` one line of 1..200 bytes, `PATH` not also in `DOGFOOD_INTENTS_FILE`); it is not a cite row, runs `cem defer` before any cite, writes `.corvint/deferred-intents/<sha256>.json`, which the bind commit carries and whose hunk is cited for you, and bounds `cem status --max-deferred-intents` (`DCW-V0-370..372`) | row `1	AGENTS.md	26:28	specification` |
| `DOGFOOD_VERIFY_FILE` | Path to a file with one shell-free verification command per line, each at most 512 characters; `DOGFOOD_VERIFY` takes the same lines inline (section 7) | line `go test ./internal/lrfrepo` |
| `DOGFOOD_OUTCOME` | `passed`, `failed` or `blocked` | `passed` |
| `DOGFOOD_OCM_LINKS` | Optional path to a TSV file, not the rows. One `INTENT<TAB>REQUIREMENT<TAB>HUNK[,HUNK...]<TAB>TEST_PATH<TAB>CLAIM[,CLAIM...]` row per requirement the author links, LF-terminated, at most 256 rows; `INTENT` is listed in `DOGFOOD_INTENTS_FILE`, each hunk is a cited CEM ordinal or hunk ID, and each claim is a test selector at `HEAD` whose anchor contains the exact requirement ID (step 7, section 5) | row `docs/specs/local-admin-console-v0.md	LAC-V0-032	7,8	internal/console/roadmap_test.go	test:TestRoadmapSafeAutoRecheck/case:lac-v0-safe-auto-recheck` |
| `DOGFOOD_OUTPUT_CHANGE` | Optional, `dogfood-check` and `dogfood-seal` only. One requirement ID, set only when the change alters the canonical `cem status` output by construction, so the base verifier cannot agree (`DCW-V0-047`). Admitted only when `docs/specs/REQUIREMENTS.tsv` at `HEAD` maps the ID to a spec that `BASE..HEAD` modifies; the `cem status` comparison, and the `dogfood-ocm status` comparison, then run the tree verifier in both roles, and it records `base-independence=NOT_PROVED` with both stdout digests. Unset, the check is unchanged | `DOGFOOD_OUTPUT_CHANGE=DCW-V0-047` |

Every `dogfood-change` refusal caused by one of these inputs prints the step and reason, then a
`fix:` line naming the correction (`DCW-V0-014`).

### Steps and expected state

1. Orient before editing: read `corvint context --task` for the files the change touches and retain
   `corvint query` and `corvint impact` receipts (section 1). A miss
   or abstention is recorded in a `docs/build-log/` entry, not repaired by rewording the task.
2. Bind intent, implement, run the focused checks and commit (sections 2 and 3). Only ignored paths
   may remain modified.
3. Export every input except `DOGFOOD_CITATIONS`, then run `make dogfood-change BASE=$BASE`. Expected
   when `BASE` carries no shared `.corvint/change.cem.json` (the normal case once a seal has moved it):
   `dogfood-change: FAIL not-complete` listing only `cem-cite: citation-plan-not-provided`,
   `cem-status: not-ready` (policy issue `max-unknown-exceeded`, because no hunk is cited yet) and
   `local-outcome: record-index-failed`; every OCM row is produced (under the no-intent declaration,
   the three OCM rows report `no-intent-declared` instead and do not block), and `git status` shows
   `?? .corvint/change.cem.json`. The recorder runs last, so it sees the sidecar this pass prepared
   and refuses every pass whose sidecar is untracked or modified (`DCW-V0-015`).
   When `BASE` still tracks an earlier unsealed `.corvint/change.cem.json`, this pass replaces it (the
   `CEM-PILOT-018` mismatch line triggers `--replace`), `git status` shows ` M .corvint/change.cem.json`,
   and the modified tracked sidecar adds `ocm-prepare-001: excluded-artifact-mismatch`,
   `ocm-status-001: exit-2` (`not-ready` on a later pass) and
   `ocm-aggregate: intent-scope-drift`; `cem-status` then also refuses it
   with `excluded-artifact-mismatch` (in `<git-dir>/corvint/cem-status.json` it is a
   `verification.issues` code, not a policy issue). The final seal removes the shared path, so
   `dogfood-seal` refuses `unarchived-base-cem` while no `.corvint/changes/` file keeps the blob
   `BASE` tracked (`DCW-V0-027`). In a repository that already seals, `dogfood-change` refuses the
   same code before any step runs, and both refusals print, as their `fix:` line, the exact command
   that commits that blob as the inherited archive `.corvint/changes/<writer>.cem.json` (`<writer>`
   is the commit that last wrote the shared path) and nothing else; run it on the change branch and
   rerun with the same `BASE`. The archive is admitted with
   `NOTE inherited-cem-archived NOT_REVERIFIED`, cited by the plan like any other hunk, and accepted
   by `dogfood batch-check` (`DCW-V0-043`, proposed).
4. Write the citation plan from the prepared map. The hunk count is
   `python3 -c "import json; print(len(json.load(open('.corvint/change.cem.json'))['hunks']))"`.
   `corvint dogfood propose-citations $BASE > plan.tsv` prints a draft with one row per owed hunk,
   each citing a base line of a file the change leaves untouched that names that hunk's path, or
   leaving it `unknown`; it never cites a file the diff adds or modifies (`DCW-V0-150`,
   `DCW-V0-393`). It is not a plan until you have checked that every cited line is the evidence
   the hunk relies on. Run before any map exists, or with an unreadable or other-base map
   (`DCW-V0-394`), it drafts one row per hunk of the canonical `BASE..HEAD` patch, keyed by
   full hunk ID and labelled `map=NOT_PREPARED keyed=hunk-id`; ID keys survive a later commit
   reordering hunks, and the bind's plan precheck still judges every row (`DCW-V0-350`).
   A hunk whose only nearby evidence is its own modified file gets an `unknown` row that names
   that file's nearest stable heading or declaration as a review hint only (`DCW-V0-439`). It cites no one-line, JSON or `.agent-evidence/`
   record, and keeps `unknown` a hunk that names a requirement ID this same change adds, naming
   the ID, since same-change intent is not BASE evidence (`DCW-V0-440`). Write the build-log entry
   and commit it before drafting (`docs/CHANGE-FLOW.md`): a new file edited after the draft gets a
   new hunk ID, and the bind refuses the stale row as `citation-plan-map-mismatch`, naming the
   row, the stale ID and the current one (`DCW-V0-441`). Recover with
   `corvint dogfood propose-citations $BASE --refresh plan.tsv > plan.new`, which carries every
   current row verbatim, re-keys a re-hunked row by path and content or BASE range with a
   `re-keyed OLD -> NEW` note to review, and drafts any other hunk afresh (`DCW-V0-442`).
   A change to dogfood tooling (`internal/dogfoodflow/`, `internal/cem/`, `cmd/corvint/dogfood_*.go`)
   needs the lane-built binary: draft with `"$(git rev-parse --absolute-git-dir)/corvint/corvint" dogfood propose-citations $BASE`
   after a `make dogfood-inputs-check` built it, and leave `CORVINT_BIN` unset for closeout, which
   otherwise refuses `corvint-bin-predates-tree-tooling` before any build (`ASD-V0-087`).
   A plan written for an earlier map, such as nine rows kept after a later commit added a tenth
   hunk, refuses `cem-cite: citation-plan-map-mismatch` and cites nothing; rewrite it from the
   current map. So does the same plan rerun after a commit moved a hunk it named by ordinal to
   another ordinal (`DCW-V0-029`). Citations add to the map: `cem prepare` resumes a map whose
   base and patch still match, keeping every earlier citation, and a plan row that would change a
   hunk the map already cites refuses `cem-cite: citation-plan-recites-cited-hunk` before anything
   is cited (`DCW-V0-117`). To correct one already cited hunk, even one an earlier stage cited, keep
   the map and rerun with the entry the refusal's `fix:` line prints,
   `DOGFOOD_CITATION_REPLACE=HUNK_ID=DIGEST corvint dogfood change $BASE` (several entries are
   comma-separated): the plan's rows for that hunk replace its earlier citations and every other
   hunk keeps its own (`DCW-V0-390`). The digest pins the hunk's state when the refusal was printed,
   so a map changed since then refuses `citation-replace-digest-mismatch` and cites nothing
   (`DCW-V0-391`); keep the same export on later passes, which replay once the replacement is in
   the map (`DCW-V0-392`). Deleting `.corvint/change.cem.json` and rerunning the whole corrected plan
   still restarts the map. A pass that cites onto an already cited map prints `cem-cite: the plan
   was added to citations the map already carried` naming both recoveries (`DCW-V0-019`).
   Export `DOGFOOD_CITATIONS` and rerun `make dogfood-change BASE=$BASE`. `cem-cite` and, for an
   untracked sidecar, `cem-status` are now produced; the only remaining row is
   `local-outcome: record-index-failed`. A modified tracked sidecar additionally keeps the OCM and
   `cem-status` rows of step 3 and adds `coordination-time-impact: unsupported-impact-worktree`. Each row
   prints its own `fix:` line; `coordination-time-impact`, `ocm-prepare-NNN`, `ocm-status-NNN` and
   `local-outcome` name the uncommitted worktree, and all of them clear once the sidecar is committed.
5. Commit the sidecar: `git add .corvint/change.cem.json && git commit -m "chore: bind change evidence"`.
   A plan with `deferred` rows also wrote a record: add `.corvint/deferred-intents/` to the same commit
   (`DCW-V0-370`); `finish-lane` does this itself.
   From the first pass on, add commits rather than amending or rebasing: each clean pass records a
   local trace for its `HEAD`, and once that commit is no longer an ancestor of `HEAD` the query and
   the recorder refuse every later pass (`coordination-time-query: unsupported-query-trace-state`,
   `local-outcome: record-failed`, both reading `local trace store contains unreachable revision`).
   If that commit is gone for good, `corvint migrate-traces --dry-run` lists it in
   `stranded_revisions`, and `corvint migrate-traces --apply --plan-digest DIGEST` moves the trace,
   byte-preserved, to `.context-corvint/legacy-traces/` (LTPM-V0-012). `dogfood change` and
   `finish-lane` check this before they prepare anything, along with ignored `.corvint/change.ocm*`
   sidecars older than BASE (`DCW-V0-400`), and retire both themselves: the trace by that apply,
   the sidecars moved under `<git-dir>/corvint/stale-ocm/`, each printed as a `RETIRE` line
   (`DCW-V0-406`). Only what cannot be retired refuses `reused-worktree-state` with those exact
   commands. The trace store, the `<git-dir>/corvint/` evidence and the operation lock belong to one worktree: lanes that share a
   repository each run in their own `git worktree add` worktree or clone, and can then change,
   check and seal at the same time; switching one worktree between lanes strands the first lane's
   traces the same way (`DCW-V0-044`, proposed).
6. Run `make dogfood-change BASE=$BASE` again. Expected: non-blocking `NOTE` lines (section 1),
   then one closing `dogfood-change: PASS complete base=BASE target=TARGET report=PATH` line
   (`DCW-V0-342`), exit 0, and
   `.corvint/dogfood-report.json` contains `"complete": true`. An uncited hunk instead leaves
   `cem-status: not-ready` (policy issue `max-unknown-exceeded`), preceded by
   `cem-cite: citation-plan-map-mismatch` when the plan is nonempty.
7. Optionally link requirement evidence: rerun step 6 with `DOGFOOD_OCM_LINKS` naming the author's
   link plan, kept outside the repository. After each map is prepared, every row for that intent runs
   through `corvint ocm link` and reports `ocm-link-NNN`, where NNN is the plan row; a refused row
   leaves that requirement unlinked, prints a `fix:` line, and later rows still run (`DCW-V0-018`);
   a row whose intent map did not prepare reports `ocm-link-NNN: ocm-map-not-prepared` (`DCW-V0-028`).
   A supplied plan that is missing, malformed or empty, or that has a refused row, blocks
   `"complete": true`. The report records the plan's `ocmLinkPlan` sha256 and row count. Export the
   same plan on every later pass, including the post-commit clean rerun (section 4), because each
   new `HEAD` regenerates the maps with `--replace`. Nothing is linked without a row, and every
   unlinked requirement stays `unassessed`, which means "not assessed by this change".
   `corvint ocm mark` (section 5) is still manual and is dropped by the next commit.
8. Inspect what a reviewer sees: `corvint cem report` and `corvint ocm report` (section 6), and
   `corvint cem report --preview` when the inspection must write nothing (DCW-V0-345), and
   `corvint frontier --cem .corvint/change.cem.json --ocm .corvint/change.ocm.001.json
   --expected-base $BASE --target HEAD`, whose exit 1 is a valid open frontier. Frontier refuses
   every `cem/0.x` map with `unsupported-spec` (decision 0525); over the default `cem/1.0` map it
   returns the `frontier-stable-view/0` view (V1-1304), whose frontier state is always
   `NOT_PRODUCED`, and it needs an explicit artifact root. The `cem/1.0` reads in section 6 take the full target ID and `--artifacts`. These paths hold only before step 10;
   after the seal, `cem report` takes section 6's sealed form
   (`.corvint/changes/<bind-sha>.cem.json` and `--target <bind-sha>`).
9. Run `make dogfood-check BASE=$BASE` (without BASE it takes the `baseRevision` of the CEM
   committed at HEAD, prints `dogfood-check: BASE=<sha> from the CEM bound at HEAD`, and refuses
   naming BASE when none is bound; never `main`, `ASD-V0-085`). Expected: one `dogfood-check: SUMMARY cem=STATE hunks=N
   supported=N unknown=N mechanical=N unknownShare=U/M reasonedUnknown=N ocm=STATE ...
   report=.corvint/dogfood-report.json detail=<git-dir>/corvint/dogfood-check.stdout` line, then
   `dogfood-check: PASS` (`DCW-V0-033`). On stderr, each hunk whose stable citation tier is
   `cited` or `unknown` is warned with the `CEM-V1-081` fix; the warning never fails the check
   (`DCW-V0-112`). The warned hunks print as one aggregated `dogfood-check: WARN
   tier-below-anchored hunks=N cited=N unknown=N paths=N first="PATH" list=FILE cause="..." fix="..."`
   line whose private `<git-dir>/corvint/dogfood-check.tier-below-anchored` file holds one
   `path=... hunk=... tier=...` line per hunk (`DCW-V0-344`); without a writable evidence
   directory the per-hunk lines print instead (at most 50, then `more=N`). A hunk no stable BASE line could anchor (none defines a requirement ID it
   adds, none names its path or its basename of 6 bytes or more) is not warned but counted in one
   `dogfood-check: NOTE tier-below-anchored-unanchorable hunks=N` line (`DCW-V0-165`). A hunk whose basis cites a span of its own file is likewise not warned but counted in one
   `dogfood-check: NOTE tier-below-anchored-same-path hunks=N` line; an anchoring gate still refuses it (`DCW-V0-388`). `unknownShare` is the map's unknown hunks over its material
   (non-mechanical) hunks, and `reasonedUnknown` the unknown hunks kept with a recorded reason
   (`DCW-V0-046`); section 6 explains its `requirements`, `linked` and `unlinked` members.
   The detail file holds the full CEM and OCM status JSON; `DOGFOOD_VERBOSE=1` prints that JSON
   in place of the summary, and a failing CEM policy always prints it. A base whose committed CEM names a base outside its history also prints
   `dogfood-check: NOTE unbound-commits NOT_OBSERVED previous-cem-base-unavailable`; a base whose
   window since the previous bound CEM contains commits no sidecar covers instead prints one
   `dogfood-check: NOTE unbound-commits count=N window=PREVIOUS_BASE..BASE newest=SHA oldest=SHA
   before-base list=FILE` line whose private list file names every commit (`DCW-V0-343`);
   neither note changes the verdict.
10. Run `make dogfood-seal BASE=$BASE`. Expected: the step 9 lines, then `dogfood-seal: PASS
    sealed=.corvint/changes/<bind-commit>.cem.json` and one rename-only commit. When the step 9
    check passed on this HEAD and BASE and nothing it reads has changed since, the seal replays
    that verdict after `dogfood-check: NOTE reused-pass receipt=FILE inputs=sha256:...` instead of
    rerunning the verifiers (`DCW-V0-347`); any other state prints `reused-pass NOT_ADMITTED` and
    reruns the check. The bind and seal
    commits take `GIT_AUTHOR_*` and `GIT_COMMITTER_*` from the environment of the command that
    makes them (`DCW-V0-166`): set them in the same shell invocation, since an agent shell that
    does not keep exported variables between calls falls back to the configured identity.
11. Hand the branch and reports to an independent reviewer (section 6) and keep the outcome
    recorded by step 6 (section 7). A change that adds or changes an admission, edge or refusal
    rule first runs the adversarial refusal pass and cites its case list for the reviewer
    ([LANE-ORCHESTRATION](LANE-ORCHESTRATION.md) section 9, `ASD-V0-052`). The verifier digests and `outputsAgree` in the report's
    `dogfoodCheck` line are author-only evidence (`DCW-V0-017`): the check also needs the author's
    private `<git-dir>/corvint/local-outcome.json` and OCM maps, and nothing committed binds the
    report's digest, so a handed-off report is not the reviewer's own observation. In a fresh clone
    `make dogfood-check` on the bind commit fails `dogfood-report-missing` and prints a `review:`
    line naming the command below. From a fresh clone of the bind commit (the seal's parent), the
    reviewer verifies instead, with their own `corvint`:
    `corvint cem verify --map .corvint/change.cem.json --expected-base $BASE --target HEAD`
    (expected `"valid":true`, `"assurance":"canonical"`, exit 0); the same map through
    `corvint cem status` with `--max-unknown 0 --max-mechanical 0`, expected `ready-for-ci`
    (raise `--max-unknown` only to the count of the map's `"disposition": "unknown"` hunks whose
    `path` is absent at `$BASE` per `git cat-file -e $BASE:<path>`, never to a number taken from
    the handed-off report); for a `cem/1.0` map (V1-1276) both commands also take
    `--artifacts "$PWD"` and full commit IDs (`--target "$(git rev-parse HEAD)"`; the stable
    verifier refuses `HEAD`) and report the stable result, expected `"outcome":"ACCEPT"` with that
    base and target, and status `"state":"accepted"` (`CEM-V1-145`, `CEM-V1-147`); that
    `git diff-tree -r -M --no-commit-id --name-status HEAD SEAL` prints only
    `R100 .corvint/change.cem.json .corvint/changes/<bind-commit>.cem.json` (tab-separated); and
    the semantics of the cited hunks and the handed-off reports (section 6).
    OCM maps are local-only: `.gitignore` excludes every `.corvint/change.ocm*` file, so neither
    the bind commit nor the seal carries one (V1-0181). The handed-off report's
    `ocmStatus.scopes[].path` names each intent file. For each, the reviewer rebuilds a map in
    their clone with `corvint ocm prepare --map .corvint/change.ocm.json --cem
    .corvint/change.cem.json --intent PATH --expected-base $BASE --target BIND_OID` (the full bind
    commit ID; it writes only that ignored map) and reads it with the read-only `corvint ocm
    status` and the same `--map`, `--cem`, `--expected-base` and `--target`. The rebuild holds no
    author `ocm link` rows unless the `DOGFOOD_OCM_LINKS` plan is handed off too, so its linked
    count and digest are not comparable with the report's `mapSha256` and counts.

### Fail-closed outcomes

Each class below was reproduced in a scratch clone for V1-0010 unless marked NOT_OBSERVED. None
produces `"complete": true` or `dogfood-check: PASS`.

| Class | Trigger | `dogfood-change` | `dogfood-check` |
|---|---|---|---|
| Dirty | modified tracked or untracked file after the change | runs; the recorder reports `local-outcome: record-index-failed`. Untracked files count even when `git status` hides them: the recorder ignores `core.excludesFile`, so the `fix:` line names each untracked path outside `.corvint/`; move it out of the worktree or list it in the repository `.gitignore` (`DCW-V0-091`) | `REFUSE dirty-worktree` (exit 2) with the required-order line |
| Unsupported | repository-configured clean or process filter | NOT_OBSERVED | `REFUSE worktree-status-unavailable` (exit 2) with the reason and a `fix:` line; the filter never runs (`DCW-V0-045`) |
| Stale | commit after the last `dogfood-change` | the rerun returns to step 3 until the sidecar is recommitted | `FAIL dogfood-report-drift`, `fix:` names another base or head |
| Unknown | hunk neither cited nor kept unknown with a reason by `DOGFOOD_CITATIONS` | `cem-status: not-ready`; a nonempty plan also refuses `cem-cite: citation-plan-map-mismatch`, an empty one `cem-cite: empty-citation-plan` | `FAIL dogfood-report-drift`, `fix:` names an incomplete report |
| Interrupted | `SIGTERM` during a run | exit 143, no report written, no citation stage left, sidecar unchanged | `FAIL dogfood-report-missing`, or `dogfood-report-drift` when an older report exists |
| Interrupted | `SIGINT` (Ctrl-C) | NOT_OBSERVED | NOT_OBSERVED |
| Unsupported | run from a subdirectory of the worktree (reproduced for V1-0236) | `REFUSE not-repository-root` (exit 2) | `REFUSE not-repository-root` (exit 2) |
| Unsupported | intent without exactly one `## Requirements` heading | `ocm-prepare-001: invalid-requirements-section`, `ocm-aggregate: intent-scope-drift` | NOT_OBSERVED |
| Drift | OCM map marked or linked without a rerun | not applicable | `FAIL intent-scope-drift`, `fix:` reruns `dogfood-change` |
| Rewritten | amend or rebase after a recorded pass, or another lane run in the same worktree | before any prepare, each stranded trace is quarantined by `corvint migrate-traces --apply` and each ignored OCM sidecar older than BASE is moved under `<git-dir>/corvint/stale-ocm/`, one `RETIRE` line each (`DCW-V0-406`); what cannot be retired refuses `reused-worktree-state` (exit 2) with the `mv` and `corvint migrate-traces --apply --plan-digest` recovery lines (`DCW-V0-400`); a trace stranded mid-run still reports `coordination-time-query: unsupported-query-trace-state`, `local-outcome: record-failed` and a `local trace store:` line; restoring the commit as an ancestor, or quarantining the trace with `corvint migrate-traces`, clears it | not reached |
| Sealed | check on the seal commit, or a change containing a modified or unverifiable seal | `REFUSE sealed-cem-in-change`, one line per refused path | `REFUSE sealed-head` |
| Undeclared | `DOGFOOD_OUTPUT_CHANGE` names a requirement `REQUIREMENTS.tsv` at `HEAD` lacks, or whose spec `BASE..HEAD` does not modify, or is set for an enrolled aggregate check (`DCW-V0-047`; reproduced by `TestCheckDCWV0047DeclaredOutputChange` for V1-1279, the aggregate case NOT_OBSERVED) | not applicable | `REFUSE output-change-requirement-unknown`, `output-change-not-in-diff` or `output-change-aggregate-unsupported` (exit 2) before any verifier runs; without the variable an output change still fails `verifier-disagreement` |

Test-claim linkage through `DOGFOOD_OCM_LINKS` needs a Go test that names the requirement ID. It was
observed for V1-0142 by replaying the sealed LAC-V0-032 change: `ocm-link-001` PRODUCED and the
aggregate reported 1 of 32 linked.

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
$ corvint index --if-stale
$ corvint context --task "THE CHANGE" --limit 20 > "$corvint_git_dir/corvint/prechange-context.json"
$ corvint query --task "THE CHANGE" --limit 1 > "$corvint_git_dir/corvint/prechange-query.json"
$ corvint impact PATH... --limit 10 > "$corvint_git_dir/corvint/prechange-impact.json"
```

`context --task` is the orientation read: which tracked files a ticket-to-change task touches. The
frozen Beamfall roadmap orientation gate admitted it against a grep baseline before this route
changed (`CWT-V0-017`, `benchmarks/results/orientation-gate-beamfall-roadmap-v1.json`); a READY
packet still misses the gold file on some tasks, so its omissions stay open. `dogfood change` does
not read `prechange-context.json`; the query and impact receipts remain its contract.

Run `corvint index --if-stale` once per fresh clone or worktree (or after a rebuilt binary, whose
engine digest misses the old snapshot) before the first context, query or impact read
(V1-1640, `IDX-SNAP-V0-102`): without a snapshot of the current tree every read pays a full in-memory
build (measured 22.8 s and 813 MB per `context` call versus 5.8 s with a snapshot), and a read whose
build takes a second or more says so on stderr. `index` is the one verb that writes the snapshot
store; the reads stay read-only.

`dogfood change` never writes these two receipts. It reruns query and impact after the change as
its `coordination-time-query` and `coordination-time-impact` steps, under those file names, so the
receipts above stay as the agent wrote them (`DCW-V0-026`). It notes, without blocking, a receipt
that is absent (`NOTE prechange-query NOT_OBSERVED agent-receipt-absent`), carries no
`context.revision`, or names a tree other than BASE's (`NOTE prechange-impact STALE
agent-receipt-not-base-tree tree=... base-tree=...`), so write both before the first edit
(`DCW-V0-031`, proposed).

Use the native `corvint` runtime. `PATH...` names the tracked files the change intends to touch.
Range impact (`corvint impact --base BASE_SHA`) is not a pre-change step: before any edit it has
no changed path to measure and returns state `OUT_OF_SCOPE` with `changedPathCount` 0 (V1-0262).
It applies once the change exists, with `BASE_SHA` the full immutable commit ID immediately before
the included changes, which must end at captured `HEAD`; `dogfood change` runs it as its
`coordination-time-impact` step. A result limit is not a byte budget: retain the complete
response and every omission/uncertainty. Native range impact refuses `--budget-bytes` with
`unsupported-impact-option`; never substitute a legacy runtime.

Range impact refuses a modified path or an untracked path that overlaps the Go build (`GPK-V0-060`),
so the coordinator's `coordination-time-impact` row is then `NOT_PRODUCED unsupported-impact-worktree`.
If Corvint abstains or misses a critical item, continue with ordinary repository inspection and record
the miss in a `docs/build-log/` entry. Never tune the current task into a held-out evaluation.
For `coordination-time-impact` only, a complete coordinator may retain `NOT_PRODUCED
unsupported-impact-range` as an explicit context abstention, and, under `DCW-V0-025` (decision 0388),
`unsupported-impact-repository` (no Go module, as in a non-Go repository, or a native Go index over
the 128 MiB aggregate bound) or
`unsupported-impact-path` (a changed Go file at the module root), each under its own code. It MUST retain the exact argv bytes,
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
description of the change before the first run and preserve it on every retry.

The proposed experimental [query abstention contract](specs/dogfood-query-abstention-v0.md)
permits structural closure for only exit 2, empty stdout and the exact single-line stderr
`{"code": "unsupported-query-trace-state", "error": "native Go authority-start query requires an absent clean-tree local trace store", "ok": false}`
followed by one LF. The row remains `NOT_PRODUCED` with reason
`authority-start-trace-state-abstention`. Its optional `queryAbstentionEvidenceSha256` binds a
private `coordination-time-query-abstention.json` artifact under `<git-dir>/corvint/`, with the
original NUL argv/task digest, base/target, exit and raw output digests. `dogfood check` validates
that exact binding and independently replays the unchanged task through base/current and any
override verifier. It never runs the recorded executable. Other trace diagnostics, extra output,
missing/drifted evidence or replay disagreement block. CEM/OCM, selected checks, clean target and
local outcome remain required; query support and packet coverage remain unavailable. Older
reports without a query abstention remain compatible. This is a proposed completion amendment,
not query capability or release promotion.

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
remains operative and the private report records the anchor as `NOT_OBSERVED`.

A lane that merged its mainline forward binds against the last mainline commit it merged,
`git merge-base origin/main HEAD`, not its original unmoved base: `BASE..HEAD` would otherwise
attest the mainline's commits as this change's and can exceed the bounds. The refusal names the
anchored base on a `fix:` line. Without an anchor, `dogfood-change` prints
`NOTE merged-forward merge=<merge> merged=<parent>` before its passes when a merge on `HEAD`'s
first-parent path merged a descendant of `BASE` that is not a seal commit, and names that parent as
the base to rerun with; set `git config --local corvint.dogfood.anchor refs/remotes/origin/main`
to refuse instead (`DCW-V0-171`). A merged seal commit is a sealed lane or stacked predecessor and
keeps its own route below:

The `dogfood-change` and `dogfood-bind-range` shell tests assert on their own evidence output with
`rg` (ripgrep). A host without it runs them through a strict `grep -P` stand-in
(`script/test-rg-fallback.sh`, `DCW-V0-811`). Only a host whose grep also lacks `-P` gets a
`REFUSE unsupported-environment-missing-rg` line instead of a misleading failure partway through
the run.

First-run order (`DCW-V0-802`). Each command needs only the inputs named here, so supply them in
this order:

1. Commit the change. `corvint dogfood propose-citations BASE_SHA > draft.tsv` needs only `BASE_SHA`
   and that committed `HEAD`: no prepared map, `DOGFOOD_INTENTS_FILE`, `DOGFOOD_VERIFY_FILE` or
   `DOGFOOD_OUTCOME`. When it refuses `patch-unavailable reason=<code>`, `cem prepare` refuses the
   same patch with that code, so do not run `dogfood change` first: make every changed path a
   regular file with a textual hunk (no empty, mode-only, binary or symbolic-link change), commit,
   and rerun it.
2. Review the draft into `cites.tsv`. Write `intents.txt` (one owning spec path per line,
   `LC_ALL=C sort -u`) and `verify.txt` (one verification command per line), and choose
   `OUTCOME=passed|failed|blocked`.
3. `make dogfood-inputs-check BASE=BASE_SHA CITATIONS=cites.tsv INTENTS=intents.txt VERIFY=verify.txt OUTCOME=passed`
   checks every input and binds nothing. A missing `VERIFY` or `OUTCOME` is refused there as
   `outcome-input-not-provided missing=--verify,--outcome` before prepare, not after the bind commit
   (`DCW-V0-800`).
4. `make dogfood-finish` (or `make dogfood-closeout`) with the same variables.

The default is one command. Write the citation plan (one row per hunk; before any map exists,
`corvint dogfood propose-citations BASE` drafts it keyed by full hunk ID, so no probe run is
needed, `DCW-V0-350`), then run

```console
$ make dogfood-finish BASE=BASE_SHA CITATIONS=cites.tsv INTENTS=intents.txt VERIFY=verify.txt OUTCOME=passed
```

`make dogfood-finish` builds the base and tree verifiers as `dogfood-seal` does and runs
`corvint dogfood finish-lane --base BASE_SHA --citations FILE [--intents FILE] [--verify FILE]
[--outcome VALUE]` (`DCW-V0-060..062`): prepare, bind with the plan, commit only
`.corvint/change.cem.json` as `chore: bind change evidence`, rebind, check and seal, one
`dogfood-finish-lane:` line per step, one `NOT_PRODUCED rows=N` block with every row a pass did not
produce, then the check's `SUMMARY` and `PASS|FAIL` lines and the seal line. The flags take
precedence over `DOGFOOD_CITATIONS`, `DOGFOOD_INTENTS_FILE`, `DOGFOOD_VERIFY_FILE` and
`DOGFOOD_OUTCOME`. It requires a clean worktree (the sidecar a stopped run left is the one
exception) and a committed change ahead of `BASE_SHA`, and it never pushes. At the first step that
does not succeed it stops with that step's own reasons and `fix:` lines; fix the input and rerun the
same command. Keep the input files in a directory only this run writes, outside the tree, such as
`"$(git rev-parse --absolute-git-dir)/corvint/lane-inputs/"`, which is unique per worktree. The run
hashes each supplied `--citations`, `--intents` and `--verify` file before prepare and, when one
differs after a pass, refuses `REFUSE input-changed-during-run step=<step>` with one
`changed: <flag> <path>` line per file, instead of reporting the drift (`intent-scope-drift`,
`citation-plan-map-mismatch`) the rewritten file would cause (`DCW-V0-378`). A rerun from the bind commit continues at the rebind, and one from the seal commit
prints `seal SKIP already-sealed`. The prepare and bind passes run only the steps their decision
reads; the rebind is the full pass the check reads (`DCW-V0-115`). The prepare pass also cites
every plan row against the map it prepared into a removed probe output, so a row `cem cite` would
refuse, such as a span whose text recurs in the changed file, stops the run at `prepare` with every
refused row and its recovery span before any bind (`DCW-V0-334`, `DCW-V0-335`). The same pass
refuses every other input the bind would refuse before its first cite, naming the file and row: an
invalid intents manifest, a plan the bind's shape step refuses (`citation-plan-uniform`, an ordinal
past the prepared map or a hunk ID it no longer has as `citation-plan-map-mismatch`, the latter
with a `--refresh` fix, `DCW-V0-441`), and an over-bound intent set
(`DCW-V0-379`); a verify refusal names the offending line (`input: --verify FILE line N`). Stderr carries one
`TIMING <step> seconds=S commands=N` line per step and a `TIMING total` line naming
`<git-dir>/corvint/dogfood-finish-lane.timings.tsv`, the per-command times. An interrupted run
prints a `fix:` line with the exact restore and rerun that resume it (`DCW-V0-116`).

Before the costly base verifier build, `make dogfood-finish` runs `corvint dogfood inputs-check`
with the same inputs: the input checks and prepare pass alone, which bind and commit nothing and
leave no map, so a `|` in the verify file, a uniform plan, a stale ordinal, a cite of a span the
change edits or an empty file exits naming the file and line before any bind work (`DCW-V0-380`).
Run it alone, with the same variables, as `make dogfood-inputs-check` while writing the inputs.
It also refuses `reused-worktree-state` for a trace or OCM sidecar an earlier lane left in a reused
worktree and prints the `corvint migrate-traces --apply` recovery (`DCW-V0-400`). Before the first
bind, also run `make doc-gates` on the working tree: it is the doc-gate list closeout and CI run, so
a spec whose Agent digest `Status` no longer repeats its header `Status`, a stale
`REQUIREMENTS.tsv` or a missing read scope fails there, before any bind (`DCW-V0-399`).
Take the ordinals from `corvint dogfood propose-citations BASE` (the prepared map's order), never
from `git diff -U0`. A few checks need the bind's own state and stay with the bind:
`recites-cited-hunk`, `citation-stage-exists` and `unknown-reasons-not-recorded`.

`make dogfood-closeout` takes the same variables, runs `dogfood-finish` (so the pre-flight runs
first), then `batch-precheck`, and stops at the first step that fails (`DCW-V0-349`). CI and the
precheck both run `make doc-gates`, using the single `DOC_GATE_TARGETS` list and the existing
use-case conformance command. The precheck also retains its other drift checks. Its drift tests
run through `script/heavy.sh` (`BATCH_PRECHECK_HEAVY`); set `BATCH_PRECHECK_GO_WRAPPER` to one
executable path, such as `script/heavy.sh`, to run every `make` and `go run` step through that
host semaphore or offload hook too, so concurrent lane prechecks do not overload the host
(`ASD-V0-941`). Unset, those steps run directly.

Before pushing, the optional pre-push hook `script/pre-push-dogfood-seal.sh` runs
`script/dogfood-batch-check.sh` against each pushed branch's merge base with `origin/main` and
refuses `dogfood-seal-missing` while any commit, including one made after the seal, is not bound by
a sealed CEM (`DCW-V0-348`). Recover by binding and sealing the named commits with this section's
steps, against that merge base or the last seal commit, and push again. Install it as
`docs/LANE-ORCHESTRATION.md` section 3 describes.

Repair: the per-step commands remain for a step that needs a hand, in this order:

```console
$ make dogfood-change BASE=BASE_SHA
$ git add .corvint/change.cem.json
$ git commit -m "chore: bind change evidence"
$ make dogfood-change BASE=BASE_SHA
$ make dogfood-check BASE=BASE_SHA
$ make dogfood-seal BASE=BASE_SHA
```

A plan with `deferred` rows (`DCW-V0-370`) writes its record before the bind commit: add
`.corvint/deferred-intents/` beside the sidecar in that commit. If a bind stops after the record
was written, remove the uncommitted record before rerunning `finish-lane`, which otherwise refuses
`dirty-worktree`. The committed record is itself a hunk of the map at the bind commit, which the
bind pass could not see, so `finish-lane` settles it before the rebind (`DCW-V0-941`): one more
bind pass cites that hunk (`DCW-V0-372`) and, when only the sidecar changed, amends the bind
commit with it (`settle PASS bind=<commit>`). That amend records no trace: no clean pass has run
on the bind commit yet, since a rebind there would have seen the uncommitted sidecar. By hand,
rerun the bind pass after the bind commit and amend the commit with the sidecar before the rebind.

`dogfood-seal` reruns `dogfood-check` and, only on PASS, commits `chore: seal change evidence`,
which does nothing but rename `.corvint/change.cem.json` to
`.corvint/changes/<bind-commit>.cem.json`. The CEM is bound and checked at the fixed sidecar
path, and the seal moves it out of that one shared tracked path, so two open branches never edit
the same file (decision 0319). The check a seal runs reuses a passing `dogfood-check` of the same
BASE and HEAD (`DCW-V0-347`): a pass leaves the private receipt
`<git-dir>/corvint/dogfood-check-pass.json`, binding its verdict to BASE, HEAD and a digest of the
verifier binaries, the running `corvint`, the check options, the `CORVINT_*` and `DOGFOOD_*`
environment, and every file directly in `.corvint/` (except the two read-command ledgers) and in
the evidence directory. The seal (`DOGFOOD_CHECK_REUSE=1`) replays that verdict only while the digest
still matches; any difference, a plain `dogfood-check`, or an aggregate check removes the receipt and
reruns. A sealed HEAD is refused by `dogfood-check` (`sealed-head`); check its parent instead. A branch that merged or stacked on sealed changes binds against its own,
unmoved `BASE_SHA`: `dogfood-change` re-verifies each sealed CEM `BASE_SHA..HEAD` adds against its
own base and bind commit, as `dogfood batch-check` does, prints `NOTE sealed-cem-merged` for each,
and the plan cites each sealed file's hunk like any other. A modified, deleted, misnamed, drifted or
unverifiable sealed CEM still refuses `sealed-cem-in-change`, naming each path and its reason
(`DCW-V0-042`, proposed).

#### Stacking on a sealed predecessor

Binding from the shared `BASE_SHA` counts every merged sealed change's hunks, and the specs they
govern, toward this change's OCM aggregate bound. A lane built on one sealed, still unmerged
predecessor (a branch from the shared base that merged the predecessor's seal commit) binds against
that seal commit instead: run `dogfood-change`, `dogfood-check` and `dogfood-seal` with
`BASE=<predecessor seal commit>` and list only the lane's own specs in `DOGFOOD_INTENTS_FILE`. The
range then holds only the lane's hunks, and `corvint dogfood batch-check` verifies each sealed
change of the merged batch against its own base. Merge mainline into the predecessor, not into the
stacked lane, or the seal-commit range also carries the newer mainline commits. When a shared-base
bind refuses an aggregate bound while merged sealed changes are in the range, the `fix:` line names
each such change by bind and seal commit and this route (`DCW-V0-113`).

Run `make gate` before the first `dogfood-change` or after the sidecar commit, not between them. Its
archive step requires a clean worktree (`conformance/release-artifact-v0/archive_run.go:422-424`),
and the prepared but uncommitted `.corvint/change.cem.json` is the one tracked path this loop leaves
modified; the OCM maps and the report are ignored paths and do not make the worktree dirty.

The first `dogfood-change` prepares and cites the tracked CEM. That CEM must itself be committed
before strict status can accept it; the canonical patch self-excludes the sidecar, so the commit
does not create a recursive self-citation. Since V1-1276 `cem prepare` writes a `cem/1.0` map by
default (`CEM-V1-140`); the loop cites, counts, checks and seals it like a `cem/0.x` map and runs
its `cem status` with `--artifacts <absolute repository root>` (`CEM-V1-146`, `CEM-V1-147`). A
`cem/0.x` sidecar left by an earlier run is refused as a different spec and regenerated with
`--replace` on the first pass. `corvint cem prepare --spec` admits only `cem/1.0`; V1-1278 removed the
`cem/0.x` producer (decision 0525), so `--spec cem/0.2` is refused with `unsupported-spec`. A change that alters the `cem status`
output the loop records, or makes OCM read a map its base refuses, as V1-1276 itself does, declares `DOGFOOD_OUTPUT_CHANGE=DCW-V0-047`. The second `dogfood-change` resolves
the new `HEAD`; prepare resumes and re-citing the same plan is idempotent, so the worktree stays clean while the report is
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
run compiled: for `coordination-time-query` and `coordination-time-impact` in that order, the packet's own
`packet_bytes`, `budget_bytes`, `within_budget`, `included_results` and `omitted_results`, or
`NOT_PRODUCED` with `packet-not-compiled` (the step compiled none, including an
impact abstention) or `packet-coverage-unreadable`. It never changes
`complete`, and reports written before it existed omit it.
`dogfood-check` independently builds one verifier from the current clean tree and one from a
private `git archive BASE_SHA`, runs both OCM and CEM status with
identical inputs and effective policy, and requires byte-identical stdout, stderr, and exit status.
Disagreement fails `verifier-disagreement`. The report records both binary SHA-256 digests and the
agreement result. Both verifiers build with `-trimpath`, so a digest depends on the source and
toolchain rather than on the private extraction directory. An explicit `CORVINT_BIN` override is accepted only when its `--version` output
is `Corvint VERSION (build N)` with `VERSION` matched exactly (decision 0314); a PATH-only name (`CORVINT_BIN=corvint`) is resolved to its absolute path before it is passed as the verifier, and an unresolvable name refuses `corvint-bin-unresolved` (V1-0685); during final checking it is an additional verifier that must agree
and whose digest is also recorded. `corvint dogfood finish` runs its final check in-process with the
running binary in both verifier roles, so its two digests are equal and prove no base independence
(`LCP-V0-014`).

A change whose own canonical `cem status` output differs from the base verifier's by construction,
such as one adding a member to the verification object, can never agree with the base verifier.
It declares a declared output change instead of gating its output behind an opt-in flag
(`DCW-V0-047`): name, in the same change, the requirement that changes the output, regenerate
`docs/specs/REQUIREMENTS.tsv`, and run

```console
$ DOGFOOD_OUTPUT_CHANGE=REQUIREMENT-ID make dogfood-check BASE=BASE_SHA
$ DOGFOOD_OUTPUT_CHANGE=REQUIREMENT-ID make dogfood-seal BASE=BASE_SHA
```

The declaration is admitted only when `REQUIREMENTS.tsv` at `HEAD` maps the ID to a spec that
`BASE_SHA..HEAD` modifies; an empty range refuses it rather than passing
`no-change-clean-worktree`. The `cem status` comparison then runs the tree verifier in both roles,
still runs the base verifier, and requires the tree verifier to agree with itself and with any
`CORVINT_BIN` override. The SUMMARY line carries `base-independence=NOT_PROVED base=sha256:B
tree=sha256:T declared=ID` before `report=`, B and T being the digests of the two verifiers'
`cem status` stdout. A base verifier that refuses the read (an exit status other than 0 or
1, for example `unsupported-spec` for a map its release cannot read) records `base=NOT_PRODUCED
base-refusal=CODE` instead of its digest (V1-1307, decision 0535); the report's `dogfoodCheck` adds a `baseIndependence` member with the same
values, its `outputsAgree` then meaning the tree verifier's self-agreement, and the seal's PASS
line keeps the fields. The reviewer compares the two outputs against the declared requirement:
the declaration admits any `cem status` difference, not only the intended one. The OCM status and
`impact` abstention comparisons keep the byte-identical rule, and without the variable nothing
changes.

Because `.corvint/change.cem.json` is one tracked path and each change's base is the commit before
its first commit, interleaved sessions can leave committed work that no CEM binds. For a clean
worktree with a change, `dogfood-check` reads the `baseRevision` of the previous binding: the CEM
committed at `BASE_SHA`, or, when `BASE_SHA` has none, the CEM in the parent of the newest seal
commit reachable from it. It examines the non-merge commits reachable from `BASE_SHA` but
not from that revision. A commit there is bound when some CEM committed in that window covers it:
the sidecar commit and the commits after that CEM's own `baseRevision`. A seal commit, one that
only renames its parent's CEM to `.corvint/changes/<parent>.cem.json`, is covered by that parent;
any other commit that removes the CEM is not a seal. Every other commit is
reported on stderr in one line, `dogfood-check: NOTE unbound-commits count=N window=PREV..BASE
newest=SHA oldest=SHA before-base list=FILE`, where the private file
`<git-dir>/corvint/dogfood-check.unbound-commits` holds one `unbound SHA` line per commit, newest
first (`DCW-V0-343`); when that file cannot be written the `  unbound SHA` lines print inline
instead. Nothing is printed when all are bound. When the
base has no committed CEM, a CEM in the window has no parseable ancestor `baseRevision`, or the
window exceeds 256 commits, it prints `dogfood-check: NOTE unbound-commits NOT_OBSERVED REASON`
(`previous-cem-absent`, `previous-cem-base-unavailable`, `window-cem-base-unavailable`,
`window-unavailable`, or `window-exceeds-256-commits`) instead of guessing. The note never changes
the check's verdict or exit status. It looks only at the window before this base, so a gap is
reported by the check of the first change that follows it; merge commits and history before the
previous binding are not examined. A commit covered only by a retroactive binding (below) is not
unbound and is not normally bound: it is reported on stderr as
`dogfood-check: NOTE retroactive-bound-commits count=N window=PREV..BASE newest=SHA oldest=SHA
before-base list=FILE`, where `<git-dir>/corvint/dogfood-check.retroactive-bound-commits` holds one
`retroactive SHA binding=BINDING_SHA` line per commit, newest first, naming the newest such
binding (inline when the file cannot be written). A normal binding of the same commit takes precedence. The retroactive label is read from
the binding commit's trailer and is not re-verified by the check, just as window CEMs are not.

#### Check a merge batch of sealed changes

One change binds, checks and seals at any size up to the `cem/1.0` map caps (2,048 hunks, 4,096
evidence records): the candidate verifier's Git budget is 1,024 operations plus 4 per map
record, and `cem status` (and the stable admission inside `ocm prepare`) reruns with the opt-in
`--scaled-operations` bound after the `cem/1.0` protocol's fixed 1,024-operation refusal (`DCW-V0-373`; `dogfood check` needs a base verifier
that has the flag), and a local outcome over 200 changed paths records the complete nonlearning
aggregate instead of a learning trace (`DCW-V0-374`, up to 4,096 paths; learning stays at 200).
Intents stay bounded at 16 scopes; list the specs that govern the change and record the rest as
deferred intents (`DCW-V0-371`, `DCW-V0-376`). A merge of separately sealed lanes is still checked
here rather than rebound, because rebinding drifts the intent scope
(`ocm-aggregate: intent-scope-drift`) and repeats work each lane already proved. Refusals over a
bound that still holds (the map caps, `aggregate-candidate-limit`,
`aggregate-receipt-byte-limit`) and `dogfood-change: REFUSE sealed-cem-in-change` print a `fix:`
line naming this path (`DCW-V0-040`, amended by decision 0584). Instead (`DCW-V0-039`):

1. Bind, check and seal each change with this loop on its own branch, in its own worktree or clone
   (the local trace store of one change refuses another's unreachable commits), against its own
   base; the lanes may run at the same time (`DCW-V0-044`).
   [LANE-ORCHESTRATION](LANE-ORCHESTRATION.md) is the multi-lane procedure: one finish per lane
   with every input supplied up front, heavy commands through `script/heavy.sh`, and checkout cleanup.
2. Merge the sealed branches into the batch branch one at a time, two parents per merge, with a
   plain `git merge --no-ff`. When a branch conflicts with the batch, do not resolve it in the merge
   and do not use `-X ours` or `-X theirs`, which silently drop the other side's conflicting hunks:
   rebase that change's own work commits (not its old bind and seal commits) onto the batch head,
   resolve each conflict there, bind, check and seal it again with `BASE=<batch head>`, then merge it.
   Commit any other edit to the batch, such as an orchestrator's index or decision update or a
   hand-merged file, as its own sealed change on a branch and merge that.
3. From a clean worktree at the batch head, run `make dogfood-batch-check BASE=$BASE` (or
   `corvint dogfood batch-check $BASE`).

Non-composing constituents. Two changes can each build and seal alone and still fail to build once
merged, for example when one renames a function the other calls. `dogfood-batch-check`,
`dogfood-check` and `dogfood-finish` then refuse `current-tree-corvint-build-failed`, show the
compiler tail and print a `fix:` line naming this paragraph; a `base-corvint-build-failed` refusal
prints a `fix:` line saying that no change can be sealed on a `BASE` that does not build
(`DCW-V0-332`). Recover as for a conflict in step 2: rebase the later constituent's own work commits
onto the batch head, fix the build there, bind, check and seal it again with `BASE=<batch head>`,
then merge it. Do not fix the build in a direct batch commit, which fails `unbound-commits`.

Regenerated commits. A one-parent commit whose tree equals what `script/batch-regen.sh` produces
from its parent passes as `derivation=regenerated` with a `NOTE COMMIT` line: the batch-regen
commit (`ASD-V0-033`) and a lane's post-seal receipt or line-citation repin. Batch-check replays the
parent's script in a private clone outside the repository; a commit with any hand-edited hunk
does not reproduce and stays `unbound-commits` with `NOT_REPRODUCED tree-differs`. A repin commit
inside a lane's own range needs no hand citation rows for the paths it alone changes
(`DCW-V0-401`, `DCW-V0-398`).

The check is read-only: it writes no object, ref, index, worktree file, trace or ledger, runs no
repository-configured filter, hook or merge driver (a configuration Core's hardened status cannot
read safely refuses `worktree-status-unavailable`), and refuses a `TMPDIR` inside the repository. Its
constituents are the `.corvint/changes/<bind>.cem.json` files `BASE..HEAD` adds. For each it
requires the bind commit in `HEAD`, the sealed bytes equal to that commit's
`.corvint/change.cem.json`, and a `cem/1.0` map whose status, run with `--artifacts <absolute
repository root>`, reports `ok`, `accepted` and a stable `ACCEPT` of exactly that base and bind
(`CEM-V1-148`, `CEM-V1-202`; a zero exit alone is not evidence, and a `cem/0.x` `ready-for-ci`
line proves no map), printing `CONSTITUENT bind=... state=...`. A pre-cutover branch's own sealed
`cem/0.1`..`cem/0.3` map is a retired seal: it prints `NOTE sealed-cem-retired ...
NOT_REVERIFIED coverage=none`, covers no commit, and is recovered by merging `origin/main`,
removing the seal and rebinding the branch as one `cem/1.0` change (`CEM-V1-203`). It then requires every commit of the range to sit in
a constituent's own range, to be its rename-only seal commit, or to be a two-parent merge whose tree
is its first parent's tree or the tree `git merge-tree` derives with the default, `-Xours` or
`-Xtheirs` strategy (`MERGE <sha> derivation=...`), derived in a private scratch repository that
applies none of the repository's configuration or attributes, so a merge driver never runs and a
merge that relied on one fails. A merge derived with `-Xours` or `-Xtheirs`, or as its first
parent's tree while Git would merge the other side, prints one line per dropped conflict hunk
(`dropped-hunk path=... hunk=N kept=L<start>+<lines> dropped=<lines> side=<parent> id=<sha256>
coverage=...`) and fails `merge-side-dropped` unless a sealed change based on that merge restores
the dropped lines at `HEAD` (`coverage=restored:<bind>`) or the merge's message carries
`Corvint-Dropped-Hunk: <id> <reason>` for a hunk dropped on purpose (`coverage=disposition`); a
hunk it cannot recompute is `hunk=UNKNOWN` and fails (`DCW-V0-073`). Each failure prints `FAIL <reason> count=N`, the
listed commits, paths or binds, and one `fix:` line; it exits 1. A dirty worktree, an empty range,
a range over 1,024 commits or more than 256 sealed changes refuses with exit 2; split the range
by checking out a piece's last commit and using the previous piece's last commit as `BASE`.
A batch whose head tracks a `.corvint/change.cem.json` that `BASE` does not track with the same
blob fails `unsealed-cem-at-head`, so a bound but unsealed change is caught before the merge.
When `BASE` itself tracks an unsealed CEM, the inherited archive above (`DCW-V0-043`) is no
constituent: a lane that adds it, or a lone commit that renames the shared path to it, is covered,
and the head may then drop the shared path.
A lane that merged the mainline forward (`git merge origin/main`) uses the mainline commit it
merged as `BASE`, which is also its merge base with the mainline. Against its original base, a
mainline commit with no sealed CEM of its own, such as a Tasks policy carry, fails
`unbound-commits`, and the failure names the merge and its merged parent as the `BASE` to rerun
with; every commit of the lane is still checked (`DCW-V0-110`). Mainline commits are the mainline's
own check; a merged lane branch is not mainline, and its commits still need a binding. A refused
constituent repeats the verifier's reason code (`state=unsupported
code=unsupported-object-alternates`), and a code outside the sealed change, such as an
alternates-backed clone, prints its own `fix:` line (`DCW-V0-111`).

A `PASS` keeps these limits visible as `NOTE` lines: OCM and the local outcome of each change are
not reverified (each change's own check already did), the merged changes' interaction is not
observed, `conflict-side-dropped merges=N hunks=H restored=R dispositioned=D` when a merge dropped
a conflict side, and that one verifier ran. The CEM
alone does not say which paths a change declared as intents, so the unknown allowance is every
whole-file `no-evidence` hunk on a path absent at the change's base; each is printed as
`bootstrap-unknown PATH` and counted in `SUMMARY`, and is wider than the exact allowance the
change's own `dogfood check` applied. A retroactive binding (below) is not a constituent: its map
exists only in the binding commit, and the verifier reads a map only from a worktree path, which
the read-only check cannot write; a batch containing one fails `unbound-commits`. On the 2026-10-11 batch of 18 sealed lanes (354 paths, 873 hunks) a replay that
merged only the seal tips passed; the batch as landed failed on 18 hand-resolved merges and 16
direct orchestrator commits (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-10-dogfood-batch-check.md`).

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
  (`script/dogfood-bind-range.sh:149-156@3b3a27f5`) and the verifier's 1,024-logical-Git-operation
  budget (`internal/cem/gitrun/gitrun.go:32-38@a875e210`). For the current object-identity and
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
  split into contiguous sub-ranges; nothing is split or cited automatically. (proposed 2026-09-25,
  V1-0228, not accepted) As in the bind loop (`DCW-V0-019`), a nonempty plan written for another map
  fails `cem-cite citation-plan-map-mismatch` before any cite: a numeric selector that is not a
  canonical ordinal or exceeds the hunk count, or an unknown hunk named by neither the citation nor
  the `DOGFOOD_UNKNOWN` plan, by ordinal or ID.
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
  requested span from `baseRevision` (`internal/cem/workflow/commands.go:489-506@c79ac767`), but
  both its stability precheck and canonical target-drift verification reject that removed span
  (`internal/cem/verify/verify.go:422-482@166fa7f2`). A backlog entry that was the only recorded
  intent therefore binds as `NOT_PRODUCED`, never as an
  invented citation, with an unknown `no-evidence` row. Its detail is the
  `removed-intent.<blob OID>.<start>-<end>` pin that the refused `cem cite` names (`CEM-CB-005`,
  decision 0165); the script accepts that detail only when the OID is a regular blob in BASE's tree
  and the byte span is nonempty and inside it, else the plan is malformed. The pin locates the
  removed intent in immutable base content; it is not evidence, and the hunk stays `unknown`.
  Intent with no base content at all keeps an `intent-unfiled-...-no-base-clause` detail.
- `DOGFOOD-BIND-008`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1487) after
  a passing `cem status`, the script prints the guarding-tests `NOTE` of `DCW-V0-069` for the
  bound map, `dogfood-bind-range: NOTE guarding-tests NOT_PRODUCED guarding-tests-not-run hunks=N:`,
  counting with the same rule through `tools/dogfood-guard-hunks` (`CORVINT_GUARD_HUNKS_BIN` names a
  built helper; otherwise it is built from the current tree). The command names the binding as
  `--target`, to run from a worktree checked out at it. A map with no such hunk prints no note; a
  count the helper cannot produce prints `NOTE guarding-tests NOT_PRODUCED
  guarding-tests-count-unavailable`. Neither fails the binding, and the search never runs here.
  `DOGFOOD_UNKNOWN` names a plan of distinct
  `HUNK<TAB>REASON<TAB>DETAIL` rows, with the citation plan's limits, a registered `cem mark`
  unknown reason, and a lower-case `[a-z0-9.-]` detail token. After citation each row is marked
  `unknown`, the script prints `dogfood-bind-range: NOTE cem-mark NOT_PRODUCED hunk=H reason=R
  detail=D`, and the binding commit message carries the same `NOT_PRODUCED` line per row before its
  trailer. A malformed plan fails as `cem-mark invalid-unknown-plan`.

- `DCW-V0-824` to `DCW-V0-827` (experimental; V1-3365 to V1-3368, `docs/specs/daily-change-evidence-workflow-v0.md`)
  refine this contract: `DOGFOOD_CITATIONS=auto-inherit` drafts the plan from the sealed constituent
  CEMs of the range through `tools/dogfood-inherit-citations` (hunk ID, then added-line overlap;
  each span relocated to BASE by exact text) and leaves every uninherited hunk a visible
  `NOT_PRODUCED` unknown; a plan row may name `bytes:START:END` for `cem cite --bytes`; the script
  reads no caller stdin; and a `cem-policy` failure prints only the counts, the `policyIssues` and
  the status file path.

- `DCW-V0-828` to `DCW-V0-831` (experimental; V1-3369 to V1-3372) extend it: an uninherited hunk is
  proposed a cited class when TARGET's generator or checker reproduces or admits it (`spec-index`,
  `line-citation-repin`, `use-case-receipt-repin`) or when it is the byte-identical move of a sealed
  CEM (`sealed-cem-rename`, citing this document's seal clause); `--prepare-only` (or
  `DOGFOOD_CITATIONS=none`) writes the hunk map and exits 0 without binding; and
  `script/dogfood-bind-coverage.sh BASE TARGET` lists, read-only, the evidence covering each
  first-parent commit of the half-open range.

Failure modes: a failing prepare, citation, binding-commit, or status step exits 1 with
`dogfood-bind-range: FAIL STEP REASON` (`cem-prepare`, `cem-cite` including
`invalid-citation-plan`, `cem-mark` including `invalid-unknown-plan`, `binding-commit-failed`, `cem-status`, or `cem-policy` after printing the
bounded policy summary of `DCW-V0-825`) and prints no `PASS` or `next:` line. A failure after the binding commit is written
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
span, and relation. The relation is one of `call-site`, `decision`, `dependency`, `implementation`,
`incident`, `specification` and `test-claim`. The evidence path is any regular file at `BASE`, not
only a spec: cite the function a hunk edits or calls (`implementation`, `call-site`, `dependency`) or
the test that pins its behavior (`test-claim`), and give one hunk several rows to cite both its code
and its requirement, for example `3	internal/x/x.go	40:52	implementation` and
`3	docs/specs/x.md	12:14	specification` (`DCW-V0-041`). A file the change itself adds
cannot be its own evidence: it refuses `cem-cite: missing-evidence`, and an unregistered relation
refuses `dogfood-inputs-precheck cem-cite citation-plan-relation-unknown` before any CEM write,
naming the row and printing the full relation list; both print a `fix:` line naming the plan row.
`tests` is not a relation: test evidence takes `test-claim`, because the basis relations are the
frozen CEM wire set (`DCW-V0-402`). A hunk no row
cites stays `unknown` with reason `no-evidence`. A hunk that no base evidence supports may instead
stay `unknown` on purpose: the row `HUNK<TAB>unknown<TAB>REASON<TAB>NOTE` runs `cem mark` with
`REASON` (`insufficient-evidence` or `conflicting-evidence`), so the map and the sealed map record
it, and keeps the hunk ID, reason and `NOTE` in `<git-dir>/corvint/cem-unknown-reasons.tsv`
(notes from earlier staged plans are kept while their hunks stay reasoned unknown, and the file
changes only after a plan is cited). Such a hunk
is admitted by `cem-status`, `dogfood check` and `dogfood batch-check` and counted in
`unknownShare`; any other unknown row refuses `invalid-citation-plan` (`DCW-V0-046`). A base evidence path means the producer reads the span from the CEM base tree;
it does not make base-only evidence admissible. The cited span must survive the change: cite base
lines the change leaves in place, because a span the candidate patch deletes or duplicates fails
the producer's `cite-span-not-stable` precheck. A hunk ID is derived from that hunk's content, paths, and line
ranges (`internal/cem/wire/canonical.go:68-79`), and an ordinal is its position in the canonical
worklist, so any later commit other than the sidecar commit can change both. Write the plan from the
map prepared for the final implementation commit (a run without `DOGFOOD_CITATIONS` prepares it and
reports `cem-cite NOT_PRODUCED citation-plan-not-provided`), and rewrite it after any further commit.
Before any cite the coordinator binds a nonempty plan to the map it just prepared (`DCW-V0-019`): an
ordinal above the hunk count, a numeric selector that is not a canonical ordinal, or an unknown hunk
named by neither ordinal nor ID, refuses the whole plan as `citation-plan-map-mismatch`, except the
hunk of an intent path absent at `BASE`, which an author leaves out deliberately (section 2). While
more than 256 unknown hunks remain, one plan cannot name them all, so the unnamed-hunk rule is not
applied and split plans on a fresh map stay usable. A stale full hunk ID refuses `unknown-hunk-id`. The
coordinator also keeps, privately under `.git/corvint/citation-plan-binding`, the plan's digest and
the map's hunk IDs in order each time it accepts a plan; the same plan rerun on a map where an
ordinal row's hunk ID now sits at another ordinal refuses `citation-plan-map-mismatch`
(`DCW-V0-029`). A hunk whose content or range changed gets a new ID and is not caught this way, so
use full hunk IDs when a later commit may reorder or edit hunks. Rows end in LF; other control bytes are invalid. The local coordinator freezes
and validates the whole file before citing, with independent limits of 4 MiB and 256 rows. An empty
file refuses `cem-cite: empty-citation-plan` and cites nothing while the map still owes a hunk
(`DCW-V0-032`, accepted); on a map that owes none it is a zero-citation no-op. Unset
`DOGFOOD_CITATIONS` to prepare the map without citing. Larger jobs
require separate explicit bounded plans, without automatic splitting or invented citations.
A citation plan over 256 rows refuses `citation-plan-row-limit-exceeded`; an OCM-link plan over
256 rows refuses `ocm-link-plan-row-limit-exceeded`. Their fix text includes `limit=256` and the
observed row count. Trace path lists independently admit at most 200 distinct normalized paths
per list (`opened_paths` and `changed_paths`); excess refuses `trace-path-limit-exceeded` with
`limit=200` and the observed count. Both `record` and `dogfood-record` expose this as
their JSON refusal `code`, including stored-path overflow; internal admission and aggregate
fallback consumers retain the `admitted-path-limit` classification.
Citation staging does not relax trace or OCM-link limits.
For OCM links, one requirement row can explicitly list multiple cited hunks and claims; if more
than 256 requirement rows are necessary, split the implementation into separately bound commits.
A `dogfood change` over 200 changed paths records the nonlearning aggregate local outcome
(`DCW-V0-374`); the trace is never truncated and no changed path is omitted. A direct `record` or
`dogfood-record` without `--aggregate-over-limit` still refuses over 200 paths.

Write the plan per hunk: for each hunk, cite the base lines that hunk depends on or implements,
such as the function it edits or calls, the contract or test that pins it, or the spec section it
implements, so two hunks share a span only when they rest on the same lines. After the
`DCW-V0-019` binding passes, a plan whose rows all cite one span is refused before any cite as
`cem-cite NOT_PRODUCED citation-plan-uniform` when the map has at least two hunks, every hunk is
named by ordinal or full hunk ID, no row is an unknown row, and every citation row has the same
`PATH` and `START:END`, whatever its relation (`DCW-V0-048`). The `fix:` line names the hunk count
and the span, for example `hunks=3 span=docs/specs/x.md:12:14`. A plan that keeps a hunk unknown
with a reason, or a staged plan that names only some hunks, is not uniform. A chore change is
exempt: one whose `git diff --no-renames --name-only BASE HEAD` names at least one path, every
path being exactly `.corvint/change.cem.json` or `.corvint/changes/NAME.cem.json` with `NAME` one
nonempty path segment. A path Git quotes, a nested `NAME` or any other path makes the change
a non-chore.

Cite the guarding test (offered, not forced; `TCQ-V0-060`). For a hunk of non-test Go source
whose behavior a base `_test.go` test pins, add a `test-claim` row whose span contains that test's
whole declaration header, `func TestName(t *testing.T)`, for example
`2	internal/x/x_test.go	41:41	test-claim`. `cem verify --hunk-revert` then names exactly the
top-level tests that span declares (`TCQ-V0-061`): a span that declares none, such as a helper or
a subtest body, is reported `not-run` with reason `no-named-test`, and no test is inferred from the
file name or the rest of the file. The flag reverse-applies only that one hunk in a disposable
copy and reports `held` only when a named test passes at the target and fails with the hunk
reverted (`CEM-V1-088`); its report sits beside the unchanged verifier result. A hunk no test
guards stays cited or `unknown` as before.

Find the guarding test by observation rather than by guess (`TCQ-V0-062`). After `cem prepare`,
run the search and append its `guardingTests.plan` rows to the citation plan beside each hunk's
other citation:

```sh
corvint cem verify --map .corvint/change.cem.json --expected-base BASE \
  --target "$(git rev-parse HEAD)" --artifacts "$PWD" --guarding-tests | jq -r '.guardingTests.plan[]'
```

Each row names, by full hunk ID, a base test
of the hunk's own directory that the search observed pass at the target and fail with only that
hunk reverted; a hunk whose row is `unguarded` or `not-run` gets no row. The search writes
nothing and sets no tier: `held` still comes only from a later `--hunk-revert` run. It executes
the change's tests unconfined, with the same bounds as `--hunk-revert`, and refuses a `TMPDIR`
inside the repository (`CEM-V1-250`). `dogfood change` never runs it: while the map holds a hunk
the search would select, the report carries `guarding-tests` `NOT_PRODUCED`
`guarding-tests-not-run`, which does not block completion, and a `NOTE` prints this command with
the run's base and target (`DCW-V0-069`).

A chore change is a self-referential bind or seal: its map cites the map files the bind loop
itself writes. Once `cem prepare` writes `cem/1.0`, `dogfood change` marks a chore's prepared
map with the stable member `role`, `seal-chore` when a `.corvint/changes/` path is touched and
`bind-chore` otherwise, and `cem status --citation` reports it as `citationClass`
`{"class": "chore", ...}` beside the stable result, never inside it (`DCW-V0-049`,
`CEM-V1-095`, `CEM-V1-106`); a change map has no role and is class `change`. A Core `cem/0.x`
map never carries a role. Core prepares `cem/1.0` since V1-1276, so the mark is live for every
chore bound after it (`CEM-V1-147`). To measure citation
support with chore maps kept apart, run
`GOTOOLCHAIN=local go run ./tools/cem-citation-baseline --root . --maps .corvint/changes --summary SUMMARY.json > ROWS.jsonl`:
it writes one baseline row per map with its class, and a summary for all maps and for each
class; a `cem/0.x` map is classed by its paths under the chore rule above.

A record change touches only `docs/build-log/` or `.agent-evidence/` paths, with at most the two sidecar paths: name its build-log entry or decision in `DOGFOOD_INTENTS_FILE` (reported as `no-intent-declared`), and cite each hunk to BASE lines of the decision or entry it records with relation `decision`; one span may serve every hunk (`DCW-V0-057`, `DCW-V0-058`).
Its sealed map has no role and reads `citationClass` `{"class": "record", "role": "change"}`, which the baseline summarizes apart from `change` (`CEM-V1-170`, `DCW-V0-059`).

#### Close a merged cited-only delivery through `cem-verdict`

Before every `gate run cem-verdict`, run the read-only preflight with the gate's own inputs. It
charges no attempt and names `no-governed-hunk` or `tier-below-anchored` (`TCB-V0-871`):

```console
$ corvint cem gate-verdict --base BASE --candidate SEAL --predicate touched-anchored \
    --touch-paths '["PATH",...]' --portable-consumer "$(command -v cem01-go)"
```

A merged delivery whose sealed map recomputes its code hunks as `cited` refuses
`tier-below-anchored` over its code touchPaths. An anchored addendum under `docs/build-log/` is not
a closeout, because no touchPath governs it, so it refuses `no-governed-hunk`. The supported route
gates the delivered code itself (`TCB-V0-870`):

1. Plan and run the delivered code's tests with `corvint-test-runner` so the plan pins those files.
2. Commit the plan and receipt on top of the merged delivery.
3. Assemble a runner-bound map over the delivery's base and that commit. Use
   `corvint-cem-candidate assemble-stable` with runner pairs, and link each delivered hunk to the
   passed receipt (`CEM-V1-280..287`).
4. Seal the map.

Then rerun the preflight. It should GRANT, with each code hunk `cited` and satisfied on
`runner-input` (`CEM-V1-606`). Run the gate only after that.

### Staged binding of one oversized commit

Accepted, decision 0510 (V1-1107; `DCW-V0-037`). The normal coordinator supports one
commit with more than 256 hunks when its trace path lists and OCM-link plan still fit their
independent bounds. These stages cite one unchanged canonical patch, with no implicit citations.
The retroactive `script/dogfood-bind-range.sh` route requires every unknown hunk named in one
range plan; it cannot stage one oversized commit. Contiguous commit sub-ranges help only when
there are already suitable commit boundaries.

`make dogfood-finish` and `corvint dogfood finish-lane --citations PLAN` run this route
themselves when PLAN has more than 256 rows (`DCW-V0-446`, experimental; V1-3167). Write one
plan naming every hunk. Finish-lane splits it into stage plans of at most 256 rows, keeping all
rows of a hunk together. The last stage takes the hunks with the fewest rows and every deferral
row. Finish-lane prepares and cites each stage on the same retained map, then makes the one bind
commit, replays the last stage, and checks and seals as usual. Its output names each stage, as
in `prepare PASS hunks=300 stage=1/2` and `bind PASS stage=1/2`. Some plans cannot be split this
way, and those are refused before any step runs with `citation-plan-not-stageable` and a `fix:`
line (`DCW-V0-447`): a hunk with more than 256 rows, a hunk named by both its ordinal and its ID,
or a plan whose last stage would need more than 256 rows. A whole plan citing one span for every
hunk is still refused as `citation-plan-uniform`. Use the manual steps below only when you need
explicit control over the stage boundaries.

1. Commit the implementation, freeze its `BASE` and `HEAD`, and set the normal task, intent,
   verification and outcome inputs from the daily adopter path. Prepare without citing:
   `env -u DOGFOOD_CITATIONS make dogfood-change BASE="$BASE"`. The expected incomplete pass
   prepares `.corvint/change.cem.json` and reports `citation-plan-not-provided`.
2. Read that map's canonical hunk worklist. Write explicit LF-terminated TSV plans outside the
   worktree, each at most 256 rows and 4 MiB, using full hunk IDs and stable base evidence.
   The first plan may name a subset while more than 256 hunks are unknown. Once at most 256
   remain, the next plan must name **every** remaining unknown hunk. A 279-hunk patch can use
   256 rows followed by 23 rows. Multiple citations per hunk also consume rows, so size the
   plans accordingly; if the final required citations cannot fit, split the implementation.
3. Run `DOGFOOD_CITATIONS=/absolute/plan-1.tsv make dogfood-change BASE="$BASE"`, then the same
   command with each successive plan. Keep the same base and implementation HEAD, retain the
   CEM between runs, and do not use `--replace`, delete the map, or commit partial sidecars.
   `cem prepare` resumes the same map and citations accumulate. Each pass is still incomplete:
   unknown hunks remain until the last plan, and the sidecar is uncommitted. The warning that
   citations were added to earlier citations is expected during staging; its delete-and-rerun
   advice applies to correcting wrong earlier evidence, not to these additive stages.
4. Inspect `$(git rev-parse --absolute-git-dir)/corvint/cem-status.json` and the CEM: every hunk must be `supported` or kept
   `unknown` with a recorded reason (a bootstrap hunk, or a plan row under `DCW-V0-046`), with no
   other `unknown` hunk and `mechanical=0`. Then `git add .corvint/change.cem.json` and
   `git commit -m "chore: bind fully cited change evidence"`. This is the only sidecar commit.
5. Rerun `DOGFOOD_CITATIONS=/absolute/last-plan.tsv make dogfood-change BASE="$BASE"` with all
   the same normal inputs, then `make dogfood-check BASE="$BASE"`. The sidecar-only commit
   preserves the canonical patch; the last plan is an idempotent replay on the fully cited map.
   Require `"complete": true` and `dogfood-check: PASS`; its `SUMMARY` reports any admitted
   unknown hunks as `unknownShare` and `reasonedUnknown`.
   Inspect the reviewer reports and continue the normal seal route. Any further implementation
   commit invalidates these plans: prepare the new patch and rewrite the plans from its map.


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
$ corvint --root "$PWD" cem prepare --base BASE_SHA --target HEAD \
    --map .corvint/change.cem.json
$ git add .corvint/change.cem.json
$ git commit -m "chore: bind change evidence"
$ corvint --root "$PWD" cem status --map .corvint/change.cem.json \
    --expected-base BASE_SHA --target "$(git rev-parse HEAD)" --artifacts "$PWD" \
    --max-unknown BOOTSTRAP_UNKNOWN_COUNT+REASONED_UNKNOWN_COUNT --max-mechanical 0
```

Use worklist ordinals with `cem cite` or `cem mark` before the sidecar commit. Zero is the default
Corvint-repository policy for unknown and mechanical hunks; an exception must be explicit in the
change review. `BOOTSTRAP_UNKNOWN_COUNT` is zero unless the complete OCM aggregate validates; it is
the validated base-absent intent count. `REASONED_UNKNOWN_COUNT` is the map's other hunks kept
`unknown` with reason `insufficient-evidence` or `conflicting-evidence` (`DCW-V0-046`), so the
effective ceiling for unreasoned unknown hunks remains zero.
Preparation ignores an inherited target-side sidecar. A `cem/1.0` map, the prepare default since
V1-1276, is read only with a full `BASE_SHA`, a full target ID, an absolute `--root` and
`--artifacts <absolute repository root>`: prepare's returned action names the resolved base and
target and that root (`CEM-V1-146`), and the status read above resolves the final revision after
the candidate commit with `--target "$(git rev-parse HEAD)"`; `--target HEAD` or a missing
`--artifacts` is refused. The former `cem/0.2` route, the only one that returned the literal
`--target HEAD` action and read without `--artifacts`, was removed by decision 0525; prepare
now writes `cem/1.0` only (`--spec cem/1.0`).
The sidecar is excluded from its own patch, avoiding recursive self-citation.

### 5. Close every owning-spec obligation

For a supported owning spec, create the local OCM reviewer artifact against the same target and CEM:

```console
$ corvint_target_sha=$(git rev-parse HEAD)
$ corvint_base_sha=BASE_SHA
$ corvint ocm prepare --target "$corvint_target_sha" --expected-base "$corvint_base_sha" \
    --intent docs/specs/OWNING-SPEC.md \
    --cem .corvint/change.cem.json --map .corvint/change.ocm.001.json
$ corvint ocm status --map .corvint/change.ocm.001.json --cem .corvint/change.cem.json \
    --expected-base "$corvint_base_sha" --target "$corvint_target_sha"
```

Use `ocm link` to bind each exact requirement to supported CEM hunks and re-extractable test claims,
or `ocm mark` to preserve an honest unknown. A requirement the change does not touch stays
`unassessed`: that reason means "not assessed by this change" and needs no mark (decision 0029). A
Go test anchors a requirement by naming its ID in a table case `name` or a `t.Run("...")` literal.
Include a descriptive word with the ID: numeric-only suffixes disappear during selector
normalization, so multiple bare IDs in one test function can collide. A whole-function selector
whose function-name anchor lacks the exact requirement ID is not a substitute.

The ID must appear as an exact token: a requirement token continues through `A-Z`, `a-z`, `0-9`, `_`
and `-`, so the byte on each side of the ID must be some other byte (a space, quote or
parenthesis) or the anchor edge. Separate the ID from the case words with a space. Selector
normalization drops that distinction, so both spellings below select
`test:TestGrammar/case:cvi-v0-strict-grammar`, but only the space-separated one links `CVI-V0-002`:

```go
func TestGrammar(t *testing.T) {
	t.Run("CVI-V0-002 strict grammar", func(t *testing.T) { /* links CVI-V0-002 */ })
	// t.Run("CVI-V0-002-strict-grammar", ...) would be refused claim-obligation-mismatch.
}
```

The same rule refuses `_CVI-V0-002_`, `XCVI-V0-002` and `CVI-V0-0021` for `CVI-V0-002`. On such a
refusal `ocm link` appends which byte adjoins the ID and a space-separated example; the matcher
itself is unchanged (`OCM-V0-005`, `OCM-V0-017`). OCM is local-only in V0 and must not be added to its own
mapped patch. Structural closure remains distinct from whether the project gate ran or passed.

#### Choosing intents when the aggregate refuses a bound

The multi-intent aggregate (`OCM-V0-013`) sums every declared scope and refuses above 1 MiB of map
bytes, 8,192 claims or 4,096 obligations; the claim and obligation bounds are 16 scopes of one map's
512 claims and 256 obligations (V1-2971), and each map keeps its own bound. In practice the summed
byte bound refuses first. The refusal names the observed total, the bound and each
scope's share by its line in `.corvint/change.ocm-intents`, for example
`map-too-large: aggregate OCM map bytes exceed the limit: observed 1100000, bound 1048576; per scope
1=800000, 2=300000; narrow: list fewer intent scopes in the manifest and bind the rest as a separate staged
change` (`OCM-V0-021`, `OCM-V0-024`: the fixed suffix names the route below); `corvint dogfood finish` keeps the same text in the `detail` member of
its `dogfood-ocm-status` evidence row (`OCM-V0-022`). An obligation bound narrower than 16 maps'
worth would surface first: before any step runs or the previous evidence is cleared, `dogfood change` counts each listed intent's
requirements at `HEAD` and, over the aggregate bound, exits 2 with `REFUSE too-many-obligations`, the same
per-scope shares and a `fix:` line naming the largest fitting subset to keep and the rest to bind
as a staged change (`OCM-V0-028`), in seconds rather than after every OCM step. To get under the
bound (`OCM-V0-023`):

1. Keep the owning spec, the one whose requirements this change adds or implements. Use the
   per-scope shares to find the large scope that the change only inherits constraints from.
2. Drop an inherited-constraint spec from `DOGFOOD_INTENTS_FILE` instead, and cite the lines the
   change relies on from it in `DOGFOOD_CITATIONS` (relation `specification`). Record in the build
   log which spec was omitted and which of its requirements or constraints still apply; omission is
   not a claim that they hold.
3. When several owning specs together exceed the bound, split the change by owning intent into
   separately bound changes, then check them as a merge batch. One spec over 256 requirements is
   refused earlier, by `ocm prepare` for its own map, and needs an owner-reviewed restructuring of
   that spec, not a different scope selection.

Never shrink an owning spec, merge or drop obligation rows, or read a smaller scope set as a
smaller gate: the bounds are fixed and the refusal widens nothing.

### 6. Review what another reviewer sees

A reviewer receives the sealed branch. At the seal head the seal has moved the map to
`.corvint/changes/<bind-sha>.cem.json`, where `<bind-sha>` is the bind commit, the seal's parent
(`git rev-parse HEAD^`, also the name `dogfood-seal: PASS sealed=` prints). The map binds that
commit, so the report targets it, not `HEAD`:

```console
$ corvint --root "$PWD" cem status --map .corvint/changes/BIND_SHA.cem.json \
    --expected-base BASE_SHA --target BIND_SHA --artifacts "$PWD" --max-unknown 0 --max-mechanical 0
$ corvint --root "$PWD" cem report --map .corvint/changes/BIND_SHA.cem.json \
    --expected-base BASE_SHA --target BIND_SHA --artifacts "$PWD"
```

The default map is `cem/1.0` (`CEM-V1-140`): its reads take full 40-hex commit IDs, never `HEAD`,
an absolute repository root and an absolute `--artifacts` root. `cem status` enforces the policy
ceilings (`CEM-V1-145`); a stable `cem report` refuses them, so it runs after that status. The
former `cem/0.2` single report with the ceilings and no `--artifacts` was removed with `cem/0.x`
(decision 0525); every map is prepared as `--spec cem/1.0`.

Before the seal, at the bind commit, the author runs the same reads with
`--map .corvint/change.cem.json --target "$(git rev-parse HEAD)"`. At the seal head that path no
longer exists, and `HEAD` names the seal commit, so both forms fail there. The OCM maps are
ignored local files, so only the author can report them, before the seal:

```console
$ corvint ocm report --map .corvint/change.ocm.001.json \
    --cem .corvint/change.cem.json --expected-base "$corvint_base_sha" \
    --target "$corvint_target_sha"
```

Read the report rather than trusting its exit code alone. It performs the same structural and
policy verification as strict status; standalone `verify` is reserved for machine/CI use. The
report stays local by default because
paths and digests are sensitive even though source and diff bodies are omitted.

The `dogfood-check: SUMMARY` members `change-requirements`, `change-traced` and `change-untraced`
after `ocm=STATE` describe only the requirements this change touches (`OCM-V0-019`, renamed by
`OCM-V0-920` so they cannot be read as the aggregate's link count): `change-requirements` counts the
declared-intent requirements whose text a citation span covers or whose lines the diff adds or
changes, `change-traced` those of them that a `*_test.go` or `*_test.sh` line, a traceability table
row of the intent, or a `DOGFOOD_OCM_LINKS` obligation names as an exact ID, and `change-untraced`
the rest. A lane that adds four requirements with traced tests reads `change-requirements=4
change-traced=4 change-untraced=0`; a code-only change that cites no requirement reads
`change-requirements=0 change-traced=0 change-untraced=0` (`OCM-V0-020`). A nonzero
`change-untraced` names requirements the change touched without a test or row naming them; it does
not fail the check. `NOT_OBSERVED` in all three means an input could not be read. The intent-wide
totals and the aggregate's `linked` count stay in `corvint ocm report` and the detail file's
aggregate `coverage`.

### 7. Record the outcome

After all gates complete, dogfood coordination classifies the complete base-to-target Git changed
path set through the producer's exact target-revision admission. It preserves that complete set and
its digest in private dogfood evidence, then records all and only the admitted source subset with the
verification commands and `passed`, `failed`, or `blocked`. The coordinator reads those commands
from `DOGFOOD_VERIFY`, one command per line with blank lines skipped, or from the file named by
`DOGFOOD_VERIFY_FILE` (same shape; it takes precedence, and a missing file emits `local-outcome
NOT_PRODUCED verify-file-unavailable`). Each line becomes one recorder `--verify`, so the recorder's
512-character, shell-syntax-free grammar applies per command; a refused command names its first
offending byte and the admitted set. A command that needs quotes or regular-expression syntax, such
as a split `-run '^Test[A-C]'` group, goes in a script committed with the change and is listed as
`sh path/to/script.sh`; the refusal's fix line names this escape (`DCW-V0-065`). The outcome comes from `DOGFOOD_OUTCOME`. The commands and
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

After merges land, run `corvint-tasks ticket closeout-debt` and finish each listed candidate through the native claim, submit, gate and complete path (TCD-V0-010).

## Enrolled local completion policy

The separate [local completion policy](specs/local-completion-policy-v0.md) coordinates the loop
above for an explicitly enrolled change. Freeze a private JSON plan with the immutable `base`,
sorted owning `intents`, and nonempty `checks` (`id`, `argv` of 1-64 elements, `timeoutSeconds` of
1-3600, optional `allowCemSidecarOnlyReuse`, default false; split a longer run into several checks). Use the repository's selected required gates; selection
is a review responsibility. Run `corvint dogfood begin --plan FILE` before work and preserve the
original context receipts. One active session owns a worktree because legacy coordination outputs
are shared. A different session must use its own worktree or wait for explicit completion/cancellation.

A linked Git worktree is a supported checkout for the loop; it is run at that worktree's top level
like any other and does not refuse. The loop resolves its private evidence and session directory
with `git rev-parse --absolute-git-dir` (`internal/dogfoodflow/change.go:167@8a79d4fb`), which for a linked
worktree (whose `.git` is a file) is that worktree's own directory under the main repository's
`.git/worktrees/NAME`, so sessions and evidence stay per worktree.
The `.corvint/` outputs live in the worktree root. Repository-wide Git state (objects, refs, and
`info/exclude`) is shared with the main checkout, so the ignore entries need adding only once.

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

The owner-requested V1-1894 repair adds a bounded sealed continuation (`LCP-V0-040`),
amending decision 0534's earlier bind-only completion rule for this exact case. Keep the reviewed
bind evidence and seal normally. At an exact one-parent seal that only moves the unchanged CEM,
follow the keyed `verify` actions for the sealed commit, then `finish`, inspect and acknowledge
its new report set, and `finish` again. This also recovers a session whose check reruns already
cleared the old terminal. `target` names the seal; `evidenceTarget` names its original bind.
The archived CEM, OCM and prior outcome remain bound to that original commit. Completion requires
actual checks on the seal and a new strict-check pass. Repeated successful finish does not
rewrite evidence or duplicate outcomes.

The supported order with the one-command lane closeout (V1-3194, `LCP-V0-041`) is: enrolled
strict `finish` satisfied at the bind, then `make dogfood-closeout`, then the keyed `verify`
actions at the seal, `finish`, review of its new report set, and `finish` again. Do not edit
private evidence in between. The closeout's own `dogfood record` rewrites the coordinated local
outcome; the sealed `finish` accepts that one replacement only after proving it against Git, the
target's trace store and the coordinator report, and names both outcome digests in the new report
set. Any other change still refuses `sealed-evidence-drift`, whose detail names the artifact, the
reason and the pinned and current digests (`LCP-V0-042`); restore the named artifact rather than
re-pinning it by hand.

A signal or cancellation that ends `finish` during its final strict check reports
`finish-interrupted` with the step, the retained check logs and the rerun command (V1-3192,
`LCP-V0-043`); rerun that `finish`, which reuses the unchanged bound results. A genuine check
failure stays `final-check-failed` and names the same logs.

This continuation excludes aggregate-profile completion, context-abstention/deadline replay,
missing reviewed bind evidence, merge commits, extra changes and descendants of the seal. Those
cases remain incomplete with their evidence intact; a rename or old PASS alone never grants
completion. An older runtime cannot read the new continuation state: retain a compatible runtime
and never delete state to make an old binary accept it. For a staged over-cap batch, keep each
stage in its own worktree or clone at the stage's base; this feature does not combine stages.

A new base needs a fresh worktree and a new enrollment (V1-2177). When the base moves under an
active, unsatisfied enrollment, leave that enrollment ACTIVE in its own checkout until its
coordinator disposes of it, and never cancel it in place to make room. `corvint dogfood cancel`
on an active enrollment is refused with `cancel-reason-required`, naming the selected checks and
saved observations it would close, unless `--reason TEXT` records why it is being abandoned. The
reason is echoed as `cancelReason` in the result and is not persisted (LCP-V0-038). Cancelling an
already satisfied or cancelled enrollment needs no reason.

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
  that adds a sealed CEM is refused by `dogfood-change` unless each re-verifies as an unmodified
  sealed change (amended by `DCW-V0-042`, proposed). A seal that would drop a CEM `BASE` tracks
  and no `.corvint/changes/` file keeps is refused `unarchived-base-cem`.
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


## Experimental aggregate outcome candidate

The proposed [aggregate local outcome profile](specs/aggregate-local-outcome-v0.md) has an
experimental source candidate for an enrolled change whose actual legacy recorder refuses more
than 200 admitted current paths. It is **NOT_QUALIFIED** and is not a release/install instruction.
The candidate command is `corvint dogfood finish --session-key KEY
--aggregate-outcome-profile corvint-dogfood-aggregate-outcome/0` (one argv invocation).
It requires the original enrollment, passing selected checks, current reviewed reports and a
retained actual typed legacy refusal. It does not change the learning recorder's limit or open
its trace store.

The separate `/1` report carries `completionState`, the enrollment and the aggregate profile;
it omits the old `complete` member. Status is read-only. Pending recovery needs the same explicit
selector and preserved history. Do not delete a discriminator, reservation, stage or old evidence
to force a downgrade. Unsupported predecessor readers, console and receipt-bundle consumers
refuse the new profile. Held-change recovery and terminal qualification need their separately
admitted integration; the candidate alone does not complete an existing ticket or held change.

## Wrapper build cache

The wrappers select the first nonempty `CORVINT_GOCACHE`, caller `GOCACHE`, then
`${XDG_CACHE_HOME:-$HOME/.cache}/corvint-go-build-cache` for every build, including both check verifiers and the check
inside seal (`DCW-V0-038`, accepted, decision 0510). For a private cache,
use `CORVINT_GOCACHE=/absolute/private/cache make dogfood-check BASE=$BASE`, or
`make dogfood-check BASE=$BASE CORVINT_GOCACHE=/absolute/private/cache`; the Make targets
pass that value through. `GOCACHE=/absolute/private/cache` alone also works with Make or
direct wrapper invocation. Empty values fall through to the next choice. The selected directory
is used as supplied; no cache is cleaned. Base/current source selection, version checks and
strict verification stay unchanged. This does not establish containment for prior runs or cover
other build/cache paths outside these wrappers.

## Reading a ticket from a lane clone

A plain lane clone has the committed `.taskman` snapshot but not the queue's journal, which lives in
the primary checkout's `.git/taskman`. Its `corvint-tasks ticket show` answers from that snapshot
only, so a ticket created after the last committed snapshot is refused as `NOT_OBSERVED` in this
checkout, not as nonexistent (CTS-V0-010). A lane reads its own ticket read-only from the primary
worktree or one of its linked worktrees (`cd PRIMARY_WORKTREE && corvint-tasks ticket show ID`), or
from the ticket text its charter carries; it never runs Tasks mutations from the clone.
