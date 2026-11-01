# Triggered automation contract

This page says which Corvint commands can run as a stateless, read-only step in a trigger that
someone else owns: a CI job, a git hook, or a team automation that fires on push, branch change,
pull request or merge. It is operator guidance for existing commands. It adds no command, no
requirement and no new status label, and it is not a release or qualification claim. Every row
below was checked against the source it cites and run from a fresh clone on 2026-09-23 (darwin
arm64, Go 1.27.1, at `01b6804`); the worked example at the end is that run.

## What a triggered run is, and what it is not

- **Corvint runs no always-on component.** Each step below is one foreground process that starts,
  reads, prints and exits. Nothing is installed as a service, daemon, launch agent or watcher, and
  nothing keeps running between triggers. The 1.0 Core scope accepted by decision 0373
  (`docs/decisions/0373-corvint-1.0-scope-ratified-2026-09-23.md`) defines the Core loop as one
  native Go binary with no account, network, hosted service, database service, embeddings or daemon
  (`docs/specs/corvint-1.0-product-and-release-v1.md:90@efcd3ea7`) and rejects a permanent daemon in the
  default binary (`docs/specs/corvint-1.0-product-and-release-v1.md:206@b7b9e11e`). Decision 0081
  (`docs/decisions/0081-local-admin-console-accepted-2026-09-07.md:24-31@4e183223`) keeps even the optional
  console operator-started, foreground only, loopback only, with no service and no durable state.
  The trigger, its schedule, its runner and its retention belong to the automation host, not to
  Corvint.
- **A triggered run grants no authority.** A step's exit code and JSON are evidence for the policy
  that invoked it; they do not approve, merge, accept intent or waive a gate. Project-owned authority
  outranks syntax, history and learned traces (AGENTS.md invariant 3). Decision 0081 states that the
  console "carries no authority" (line 25), and decision 0373 took formal host FULL and protected
  authority off the Core path (row 6, line 19); a CI or hook invocation of the same binary gains
  nothing either path lacks. In particular: `affected` never claims omitted tests are safe to skip
  (`cmd/corvint/help.go:913-914@d5d6e05c`); a valid CEM establishes provenance completeness, not semantic
  correctness ([Change Evidence Map](CHANGE-EVIDENCE-MAP.md)); and `witness` exits 0 on a
  report whose obligations are all unproven (worked example below).
- **Network.** "Local-only; no network or telemetry" (root help support boundary)
  (`cmd/corvint/help.go:399@f1b66bd9`). Fetching the commits a step needs is the runner's job.

## Mutation classes (AGENTS.md invariant 4)

Invariant 4 allows read commands exactly two bounded writes, both under `.corvint/`. This page uses
three classes:

| Class | Meaning |
|---|---|
| `R0` | Writes nothing: no repository, trace, `.corvint/` or Git-directory state. |
| `R0+SOL` | As `R0`, except that an `unsupported-*` refusal may append one bounded row to `.corvint/self-observations.jsonl` (the self-observation ledger). |
| `W` | Writes local state beyond the two ledgers. Not a read-only step; listed only as an exclusion. |

The self-observation append happens only when that ledger path is gitignored by the root
`.gitignore` or `.corvint/.gitignore` together with its `.self-observations.*` temporaries
(`internal/observations/observations.go:266-268@93f6bda0`, `internal/observations/observations.go:687-694@b40a3bba`).
At `01b6804` the root `.gitignore` names the ledger but not its temporaries, so in a fresh clone
with no `.corvint/.gitignore` (which `corvint index` creates) no row is written; the worked example
and a forced `unsupported-impact-path-suffix` refusal both left `.corvint/` unchanged. A ledger
storage failure never alters the response (`cmd/corvint/help.go:405-406@3c9a8d56`).

The second ledger, `.corvint/unplanned-reads.jsonl`, is written only by host-adapter hook paths
while the operator marker `.corvint/unplanned-reads.enabled` exists
(`cmd/corvint/host_adapter.go:352@45edc402`, `cmd/corvint/host_adapter_experimental.go:111-128@765210a5`) and the
marker only by `corvint reads` (`cmd/corvint/reads.go:92-96@19937f8b`). None of the commands below writes
either file.

## Commands safe as a triggered step

Each takes `--root PATH` (default: the current directory) and reads the checkout at `HEAD`. Every
base must be a full 40-hex commit id that is present in the clone, so a shallow CI checkout must
fetch enough history to contain it (`cmd/corvint/help.go:875@a892984d`). On an argument or input refusal each
prints one JSON object with `"ok": false` to stderr and exits 2.

Scope: the table lists the steps that answer a question about one change against a fixed base,
which is what a CI job, git hook or team automation asks. Other verbs that help also describes as
read-only (`init`, `adopt`, `query`, `docs`, `harness`, `lrf` at `cmd/corvint/help.go:401-402@a54374e4`, and the
experimental `batch`, `depsource`, `necessity`, `surprise`, `answerability`, `kernel` and `reads`
in the command-maturity list at `cmd/corvint/help.go:375-384@80dd8f06`) are not classified here. They need a per-session task, packet or
request as input, or they are experimental without a stability promise. This page makes no claim
about their safety as a triggered step.

| Step | Class | Exit codes | Output profile (stdout) | Required inputs | 1.0 label |
|---|---|---|---|---|---|
| `corvint affected --base BASE` | `R0` | 0 plan written; 2 refusal or error | `affected-plan/0`, one canonical JSON line, `"mutates": false` | full `BASE`; a checkout whose `HEAD` is the change head | Core |
| `corvint cem verify --map MAP --expected-base BASE --target TARGET --artifacts ROOT` | `R0+SOL` | 0 accepted; 1 rejected (`"ok": false`); 2 refusal or error | JSON with `"tool": "cem-verify"`, `"mutates": false` and the `stableVerification` result; the map must be `cem/1.0` (decision 0525 refuses `cem/0.x` with `unsupported-spec`) | CEM path, full `BASE` and `TARGET`, absolute artifact root | Core |
| `corvint cem status` (same arguments, plus optional `--max-unknown N`, `--max-mechanical N` and the experimental `--max-deferred-intents N`) | `R0+SOL` | as `cem verify`; an exceeded ceiling or a deferral-record issue exits 1; `"state"` is `accepted`, `rejected` or `unsupported` | JSON with `"tool": "cem-status"`, `"mutates": false` | as `cem verify` | Core |
| `corvint witness --base BASE [--cem MAP] [--evidence FILE]... [--evidence-in-repo PATH]... [--json] [--check]` | `R0+SOL` | 0 report written, whatever it says; with `--check`, 1 when admission is not `COMPUTED` or any obligation is not closed; 2 refusal or error | `corvint-witness/0` (text headed `WITNESS corvint-witness/0`, or JSON with `--json`) | any `BASE` revision Git resolves to a commit (the report carries the full object ID); optional CEM path; a `--head` other than the checked-out `HEAD` is read from the object store and bound `COMMIT-PINNED` (AGW-V0-007); each `--evidence` test-run document (a Playwright receipt or RSpec provider document) bound to the head tree adds non-closing `CALLER_REPORTED` witnesses and an unreadable one exits 2 (AGW-V0-008, AGW-V0-010); `--evidence-in-repo` names one by a checkout-relative path and refuses an absolute, `..` or link-escaping path with exit 2 (AGW-V0-011) | experimental |
| `corvint impact --base BASE [--range-profile expanded-256]` | `R0+SOL` | 0 packet written; 2 refusal or error, including a range over capacity (`unsupported-impact-range`) | JSON with `"tool": "impact"`, `"mutates": false`; `context.profile` is `corvint-range-impact/0` | full `BASE`; the default range capacity is 100 paths, 256 with the explicit profile | Core |

Where each row was verified:

- `affected`: profile `cmd/corvint/affected.go:36@af9fb447`; exits `cmd/corvint/affected.go:746-800@47d84c87`; its
  refusals are emitted without the ledger recorder (`cmd/corvint/main.go:1104-1109@7ed11581`); help text
  "runs no test, writes no repository state" (`cmd/corvint/help.go:913@f024c3ab`).
- `cem verify` and `cem status`: exits `internal/cem/cli/cli.go:460-489@773eecd5` (0 when the envelope says
  `"ok": true`, 1 when it says false, 2 on a dispatch error); `ok`, `mutates` and `state`
  `internal/cem/workflow/stable_read.go:143-200@7e596fa3`; refusals pass through the ledger recorder
  (`cmd/corvint/main.go:1233@760ae507`, `cmd/corvint/main.go:1693-1708@7a65d7e8`).
- `witness`: profile `internal/witness/witness.go:28@9d3c5f4c`; exits `cmd/corvint/witness.go:276-324@bc64bcb6`;
  recorder `cmd/corvint/main.go:1195@acb85656`; help "does not mutate repository or trace state"
  (`cmd/corvint/help.go:453-454@b8d79ae9`). It builds the index in memory when no snapshot exists
  (`cmd/corvint/index_snapshot.go:103-108@15aca3d7`); the fresh-clone run wrote no snapshot store (`.git/corvint/index`).
- `impact --base`: profile `internal/contextindex/range_impact.go:24@49493543`; exits and ledger calls
  `cmd/corvint/main.go:1237-1242@53443dcf` and `cmd/corvint/main.go:1266-1354@17c42df8`; envelope
  `cmd/corvint/main.go:1510@dedd82e3`; range capacity from `corvint impact --help`. With no snapshot it builds
  the index in memory (`cmd/corvint/main.go:1270@7219f23e`) and writes no snapshot store (`.git/corvint/index`).
- 1.0 labels: `docs/specs/corvint-1.0-product-and-release-v1.md` classification table.
- Every exit code in the table was also observed: 0 in the worked example, 1 from `cem verify`
  with `--target` set to the base, 2 from `affected`, `witness` and `impact` given a short base and
  from `cem verify` given a missing map.

### Runtime and timeout

Measured on 2026-09-23 on a darwin arm64 host **under heavy load from a concurrent benchmark**, so
these are inflated and are not a performance claim. Two runs of the worked example over a 17-path
range (1 s resolution): `affected` 8 and 9 s, `cem verify` 2 and 2 s, `cem status` 1 and 2 s,
`witness` 7 and 10 s, `impact --base` 8 and 10 s. One run in a working checkout under `time`: 7.7 s,
2.6 s, 2.0 s, 12.0 s and 8.8 s wall.

Corvint sets no whole-command timeout on these steps. Some internal Git steps carry their own
deadline and fail the command when it expires: the index build and the range-impact diff use 30 s
(`internal/contextindex/git.go:26@8f4f0b4d`, `internal/contextindex/index.go:349@09a263a5`,
`internal/contextindex/range_impact.go:131@09a263a5`), and `affected` reads `git status` under 10 s
(`internal/liveverify/affected/dirty.go:34@bb3abea6`, `internal/liveverify/affected/dirty.go:50@d3f19770`). Otherwise
the process cancels only on `SIGINT` or `SIGTERM` (`cmd/corvint/main.go:1768@5434511f`,
`cmd/corvint/signals_unix.go:12-13@4a4dfd65`); the exit code a signal produces is not documented here
(unknown). The automation host must set its own step timeout. No timeout has been qualified; 5
minutes per step is a suggestion that leaves more than 20 times the loaded measurements as
headroom, and it is a hang detector, not a budget.

## Not read-only: excluded as triggered read-only steps

| Command | Why it is excluded | Source |
|---|---|---|
| `corvint cem report` | Writes the review report, by default to `$GIT_DIR/corvint/cem-review.md` (or to `--output`: repository-relative, or absolute outside the repository), and returns `"mutates": true`. `--preview` (DCW-V0-345) and `--format json` write nothing and return `"mutates": false`. | `internal/cem/workflow/workflow.go:25@c21ea11c`, `internal/cem/workflow/stable_read.go:338-354@19c5c7be` |
| `make dogfood-check BASE=...` | Once its preconditions pass it rewrites `.corvint/dogfood-report.json`, builds verifier binaries into `$GIT_DIR/corvint/`, and uses `${XDG_CACHE_HOME:-$HOME/.cache}/corvint-go-build-cache`. It is an authoring-time step, not a gate prerequisite, because it needs untracked `.corvint/` artifacts a clean checkout never has. Its wrapper builds the verifiers first; in a fresh clone the check then fails before the report rewrite (below). | `script/dogfood-check.sh:87-105@70896eeb`, `internal/dogfoodflow/check.go:367-376@8edfe182`, `Makefile:36-39@6362815e` |
| `make dogfood-change`, `make dogfood-seal`, `corvint index`, `cem begin`, `prepare`, `cite`, `mark`, `cover`, `discriminate`, `anchor` | Write the dogfood report, a commit, the index snapshot, CEM maps, the patch cache or a Git note by design. | `script/dogfood-change.sh:38@f8f6aa02`, `script/dogfood-seal.sh:28-31@5c2ac3f4`, `SOP-V0-002` for `index`, `cmd/corvint/help.go:406-408@bfe2edf4` and `corvint cem --help` |

`corvint dogfood check`, and so `script/dogfood-check.sh`, exits 0 on `PASS`, 1 on `FAIL`, 2 on
`REFUSE` or a Git error, and 129, 130 or 143 on `HUP`, `INT` or `TERM`
(`cmd/corvint/signals_unix.go:19@1f4cca5e`); `make` reports any non-zero
recipe exit as 2, which the worked example observed.

**Fresh-clone limitation (V1-0182, by design).** The task-store title of ticket V1-0182 states it
verbatim: "dogfood-check: in a fresh clone the check fails `dogfood-report-missing` before any
verifier runs, so a reviewer cannot reproduce the override-verifier comparison". The worked example
reproduces it: `internal/dogfoodflow/check.go:367-376@8edfe182` exits 1 with `FAIL dogfood-report-missing` because
`.corvint/dogfood-report.json` is gitignored and only the author's `dogfood-change` writes it. A
fresh-clone trigger can therefore verify a sealed CEM with `cem verify`, but cannot reproduce the
author's dogfood-check verifier comparison. That comparison is author-only evidence
(`DCW-V0-017`, `docs/DOGFOOD.md` step 11); on a bind commit the failure now also prints a
`review:` line naming the `cem verify` command a reviewer runs instead. The worked example below
pins a commit that predates that line.

The former pull-request wrapper `examples/cem/verify-pr.sh` and its `docs/CEM-CI.md` page read
`cem/0.x` maps only; both were removed with `cem/0.x` by V1-1278 S3 (decision 0525) and remain in
Git history.

## Worked example: a fresh clone at a fixed commit

The script clones a source repository with `git clone --no-local`, checks out `01b6804` detached,
builds `corvint` from that tree into the work directory, runs every step above against the sealed
CEM of V1-0204 (bind commit `ace0a96`, base `a6a6b8b`, the pin's first parent), runs
`make dogfood-check` to show the fresh-clone limitation, and compares the worktree status (with
ignored files), the `.corvint/` file list and the Git-directory entries before and after. It needs
`git`, Go 1.27.1, `make` and `jq`. `SOURCE_REPO` is any clone that contains the pinned commit.

```sh
#!/usr/bin/env bash
# usage: automation-example.sh SOURCE_REPO WORK_DIR
set -u
src=${1:?usage: automation-example.sh SOURCE_REPO WORK_DIR}
work=${2:?usage: automation-example.sh SOURCE_REPO WORK_DIR}
pin=01b6804063713a24f6d922f00a46a85326e6e132   # fixed commit (merge of PR #122)
base=a6a6b8b66c44c486fc86daddfab3fd2d931a31dc  # its first parent, the CEM's baseRevision
bind=ace0a96bd5ffcfa2af8013e23a1cf3220b46c24f  # the sealed bind commit of V1-0204
map=.corvint/changes/$bind.cem.json

git clone --no-local --quiet "$src" "$work/repo" || exit 2
cd "$work/repo" || exit 2
git checkout --quiet --detach "$pin" || exit 2
GOTOOLCHAIN=local go build -trimpath -o "$work/corvint" ./cmd/corvint || exit 2
snapshot() { git status --porcelain --ignored --untracked-files=all; find .corvint -type f | sort; ls -A "$(git rev-parse --absolute-git-dir)"; }
before=$(snapshot)

step() {
  local name=$1 start status
  shift
  start=$(date +%s)
  "$@" > "$work/$name.out" 2> "$work/$name.err"
  status=$?
  printf '%s exit=%d seconds=%d\n' "$name" "$status" "$(( $(date +%s) - start ))"
}
step affected "$work/corvint" affected --base "$base"
step cem-verify "$work/corvint" cem verify --map "$map" --expected-base "$base" --target "$bind"
step cem-status "$work/corvint" cem status --map "$map" --expected-base "$base" --target "$bind"
step witness "$work/corvint" witness --base "$base" --cem "$map" --json
step impact "$work/corvint" impact --base "$base"
step dogfood-check make -s dogfood-check BASE="$base"

jq -c '{profile, ok, mutates, selected: (.plan.selected | length), unknowns: (.plan.unknowns | length)}' "$work/affected.out"
jq -c '{tool, ok, mutates, valid: .verification.valid, assurance: .verification.assurance}' "$work/cem-verify.out"
jq -c '{tool, ok, mutates, state, counts}' "$work/cem-status.out"
jq -c '{profile, summary}' "$work/witness.out"
jq -c '{tool, ok, mutates, profile: .context.profile}' "$work/impact.out"
cat "$work/dogfood-check.err"
if [ "$before" = "$(snapshot)" ]; then echo 'clone unchanged: worktree, .corvint/ and git dir entries'; else echo 'CLONE CHANGED'; fi
```

Observed on 2026-09-23, darwin arm64, host under heavy benchmark load, run as
`automation-example.sh <clone> <empty work dir>` (script exit 0):

```text
affected exit=0 seconds=9
cem-verify exit=0 seconds=2
cem-status exit=0 seconds=2
witness exit=0 seconds=10
impact exit=0 seconds=10
dogfood-check exit=2 seconds=2
{"profile":"affected-plan/0","ok":true,"mutates":false,"selected":31,"unknowns":0}
{"tool":"cem-verify","ok":true,"mutates":false,"valid":true,"assurance":"canonical"}
{"tool":"cem-status","ok":true,"mutates":false,"state":"ready-for-ci","counts":{"mechanical":0,"supported":39,"total":39,"unknown":0}}
{"profile":"corvint-witness/0","summary":{"opened":66,"analysed":57,"closed":0,"unproven":57,"notRun":9,"witnessesExamined":39,"witnessesClosing":0,"determinablePerMille":863,"provenPerMille":0}}
{"tool":"impact","ok":true,"mutates":false,"profile":"corvint-range-impact/0"}
dogfood-check: FAIL dogfood-report-missing
  fix: run make dogfood-change BASE=a6a6b8b66c44c486fc86daddfab3fd2d931a31dc on this HEAD until it reports complete
make: *** [dogfood-check] Error 1
clone unchanged: worktree, .corvint/ and git dir entries
```

What this shows: every contract step exited 0 against the pinned commit; the sealed CEM verifies
with canonical assurance; `witness` exits 0 while reporting 57 unproven and 9 not-run obligations,
so its exit code is not a gate; `dogfood-check` hits the V1-0182 limitation (script exit 1, which
`make` reports as 2); and no step changed the clone's worktree, `.corvint/` files or Git-directory
entries. Building `corvint` writes only to the work directory and the Go build cache outside the
clone.

## Session hooks: `frontier-authority-unavailable`

For an adopter that does not produce Change Evidence Map (CEM) or Obligation Coverage Map (OCM)
artifacts, this degradation is **expected and benign by itself**, not a fault or a
misconfiguration. It means the hook cannot claim Frontier authority; it does not mean the query
failed, tests failed, or the adopter must enable another component. Other errors and missing
evidence must still be investigated separately.

**Current behavior, verified 2026-10-10 (V1-1143; related V1-0869 and V1-0243):** every successful
`harness event` receipt starts with this literal string in `degradations`, regardless of whether
CEM/OCM artifacts exist (`internal/gokernel/harness.go:444@a4869098`). The harness does not consult
`internal/frontier`. Stop emits a fixed `UNAVAILABLE` frontier with `shouldContinue: false`
(`internal/gokernel/harness.go:483-489@19864b9a`); this is an abstention, not proof that the change
is complete. The code is also admitted by the observations ledger and host adapter vocabularies
(`internal/observations/observations.go:50-57@493847da`,
`cmd/corvint/host_adapter.go:733-742@c1243f78`). Repeated appearances are not evidence of repeated
failures, and adding artifacts does not remove this message.

### What the artifacts enable, and what remains unavailable

A CEM binds a committed base-to-target patch to cited evidence. An OCM binds a pinned owning
specification's obligations to that CEM's hunks and selected test claims. The explicit
`corvint frontier` command composes verified CEM/OCM inputs with lexical relevance and test-claim
qualification into a bounded, Git-pinned advisory review queue. Without those inputs, that
change-and-intent-specific queue cannot be assessed; it remains unknown, not an empty or closed
frontier. Normal context queries and impact reads do not require adopting that workflow.

**No CEM/OCM producer establishes execution authority.** The current `frontier/0` profile leaves
linked test obligations open because test evidence is caller-reported
(`docs/specs/change-frontier-v0.md:41-45@e5292c79`). Closing authority would require an accepted
execution-attestation root and profile, plus harness integration; that is not an adopter setup
option today. Accepted [decision 0009](decisions/0009-harness-authority-boundary.md) retains the
local boundary and permits a separate non-authority workflow policy, not Frontier closure
(`docs/decisions/0009-harness-authority-boundary.md:33-38@d8539a99`).

### Minimum path to an advisory Frontier report

No action is needed merely to keep using the hooks. If the adopter wants a change-evidence
review queue, the minimum inputs and steps are:

1. Select a committed change, its full immutable base and target commit IDs, and an existing
   owning spec in the supported OCM requirements format. Keep all maps on that same base/target;
   missing declared intent remains unassessed.
2. Follow [CEM preparation](DOGFOOD.md#4-produce-the-cem): `corvint cem prepare`, cite evidence or
   honestly mark unknown hunks, commit the CEM sidecar, and verify it against the final target.
   Preparation alone does not prove complete evidence. These are explicit authoring operations,
   not work performed by a read command or session hook.
3. Follow [OCM preparation](DOGFOOD.md#5-close-every-owning-spec-obligation): `corvint ocm prepare`
   for each owning spec, link supported hunks and test claims, retain unknown/unassessed obligations,
   and inspect `ocm status`. OCM artifacts stay local and outside their own mapped patch.
4. Run the read-only preview for each scope, using the verified maps and their exact pins:

   ```sh
   corvint frontier --cem CEM_PATH --ocm OCM_PATH \
     --expected-base FULL_BASE_SHA --target FULL_TARGET_SHA --json
   ```

   Inspect the report, including every unknown and required action. Exit 1 can mean a valid open
   frontier, not an invocation fault. Neither valid input nor passing tests grants authority or
   changes the unconditional hook degradation. Keep the project's required checks and review.

### Causes and remedies

| Observation | Meaning | Action |
|---|---|---|
| Only `frontier-authority-unavailable`, no CEM/OCM workflow | Expected capability limitation; the code does not diagnose artifact absence. | Continue normal use. Adopt the advisory workflow above only if useful. |
| The same code after preparing valid CEM/OCM artifacts | Expected: the hook does not consult Frontier, and caller-reported evidence cannot establish authority. | Inspect the explicit Frontier report; do not regenerate artifacts to silence the hook. |
| Explicit `frontier` invocation refuses missing, stale or mismatched inputs | A separate evidence/input problem, not diagnosed by this generic hook code. | Check its error, map paths and exact base/target/intent pins; explicitly regenerate affected artifacts if needed. Do not treat missing evidence as success. |
| Other codes, such as `corvint-event-rejected:dogfood-event-index-snapshot-stale` | A separate hook failure with its own remediation. | Follow that code's diagnostic; do not dismiss it because the Frontier code is benign. |
| Repeated visible notices | Host output policy or installed adapter version can affect presentation. | Compare the installed adapter with [decision 0161](decisions/0161-claude-adapter-quiet-notices-2026-09-12.md). Current Claude routine post-tool receipts use model-visible additionalContext; genuine faults retain a notice. This is not a user suppression setting. |

The Claude routine receipt path preserves the degradation codes in additionalContext
(`cmd/corvint/host_adapter.go:718-730@78ff0af5`). Presentation differs by host and event; quieting a
notice is not removing the degradation or acquiring authority.

### Not delivered by this guidance

The degradation payload is still a list of strings, and no per-degradation `reason` distinguishes
artifact absence from unavailable authority. The Stop receipt's `frontier.reason` repeats the generic
code; `AHI-054` (decision 0519, V1-1157) adds the fixed `frontier.cause`
`no-accepted-closing-authority`, which says why the degradation holds but never diagnoses a missing
or unverified CEM. An adopter acknowledge/suppress option is **not implemented**: the owner declined
it in decision 0519, because missing evidence stays visible. This guidance (V1-1143) adds no new
authority, wire contract, or numbered requirement.
