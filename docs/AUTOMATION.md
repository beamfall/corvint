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
  (`docs/specs/corvint-1.0-product-and-release-v1.md:42`) and rejects a permanent daemon in the
  default binary (`docs/specs/corvint-1.0-product-and-release-v1.md:145`). Decision 0081
  (`docs/decisions/0081-local-admin-console-accepted-2026-09-07.md:24-31`) keeps even the optional
  console operator-started, foreground only, loopback only, with no service and no durable state.
  The trigger, its schedule, its runner and its retention belong to the automation host, not to
  Corvint.
- **A triggered run grants no authority.** A step's exit code and JSON are evidence for the policy
  that invoked it; they do not approve, merge, accept intent or waive a gate. Project-owned authority
  outranks syntax, history and learned traces (AGENTS.md invariant 3). Decision 0081 states that the
  console "carries no authority" (line 25), and decision 0373 took formal host FULL and protected
  authority off the Core path (row 6, line 19); a CI or hook invocation of the same binary gains
  nothing either path lacks. In particular: `affected` never claims omitted tests are safe to skip
  (`cmd/corvint/help.go:610-611`); a valid CEM establishes provenance completeness, not semantic
  correctness ([CEM in CI](CEM-CI.md#policy-and-failure-behavior)); and `witness` exits 0 on a
  report whose obligations are all unproven (worked example below).
- **Network.** "All commands are local-only and make no network or telemetry request"
  (`cmd/corvint/help.go:352`). Fetching the commits a step needs is the runner's job.

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
(`internal/observations/observations.go:253-255`, `internal/observations/observations.go:712-719`).
At `01b6804` the root `.gitignore` names the ledger but not its temporaries, so in a fresh clone
with no `.corvint/.gitignore` (which `corvint index` creates) no row is written; the worked example
and a forced `unsupported-impact-path-suffix` refusal both left `.corvint/` unchanged. A ledger
storage failure never alters the response (`cmd/corvint/help.go:355-359`).

The second ledger, `.corvint/unplanned-reads.jsonl`, is written only by host-adapter hook paths
while the operator marker `.corvint/unplanned-reads.enabled` exists
(`cmd/corvint/host_adapter.go:322`, `cmd/corvint/host_adapter_experimental.go:102-120`) and the
marker only by `corvint reads` (`cmd/corvint/reads.go:92-96`). None of the commands below writes
either file.

## Commands safe as a triggered step

Each takes `--root PATH` (default: the current directory) and reads the checkout at `HEAD`. Every
base must be a full 40-hex commit id that is present in the clone, so a shallow CI checkout must
fetch enough history to contain it (`cmd/corvint/help.go:627`). On an argument or input refusal each
prints one JSON object with `"ok": false` to stderr and exits 2.

Scope: the table lists the steps that answer a question about one change against a fixed base,
which is what a CI job, git hook or team automation asks. Other verbs that help also describes as
read-only (`init`, `adopt`, `query`, `docs`, `harness`, `lrf` at `cmd/corvint/help.go:354`, and the
experimental `batch`, `depsource`, `necessity`, `surprise`, `answerability`, `kernel` and `reads`
at `cmd/corvint/help.go:312-328`) are not classified here. They need a per-session task, packet or
request as input, or they are experimental without a stability promise. This page makes no claim
about their safety as a triggered step.

| Step | Class | Exit codes | Output profile (stdout) | Required inputs | 1.0 label |
|---|---|---|---|---|---|
| `corvint affected --base BASE` | `R0` | 0 plan written; 2 refusal or error | `affected-plan/0`, one canonical JSON line, `"mutates": false` | full `BASE`; a checkout whose `HEAD` is the change head | Core |
| `corvint cem verify --map MAP --expected-base BASE --target TARGET [--max-unknown N] [--max-mechanical N]` | `R0+SOL` | 0 valid and within caps; 1 invalid or over a cap (`"ok": false`); 2 refusal or error | JSON with `"tool": "cem-verify"`, `"mutates": false` (the example map is `cem/0.2`) | CEM path, full `BASE` and `TARGET` (both required for `cem/0.2`) | Core |
| `corvint cem status` (same arguments) | `R0+SOL` | as `cem verify`; `"state"` is `ready-for-ci`, `incomplete` or `invalid` | JSON with `"tool": "cem-status"`, `"mutates": false` | as `cem verify` | Core |
| `corvint witness --base BASE [--cem MAP] [--json]` | `R0+SOL` | 0 report written, whatever it says; 2 refusal or error | `corvint-witness/0` (text headed `WITNESS corvint-witness/0`, or JSON with `--json`) | full `BASE`; optional CEM path; `--head`, if given, must equal the checked-out `HEAD` | experimental |
| `corvint impact --base BASE [--range-profile expanded-256]` | `R0+SOL` | 0 packet written; 2 refusal or error, including a range over capacity (`unsupported-impact-range`) | JSON with `"tool": "impact"`, `"mutates": false`; `context.profile` is `corvint-range-impact/0` | full `BASE`; the default range capacity is 100 paths, 256 with the explicit profile | Core |

Where each row was verified:

- `affected`: profile `cmd/corvint/affected.go:30`; exits `cmd/corvint/affected.go:312-335`; its
  refusals are emitted without the ledger recorder (`cmd/corvint/main.go:940-945`); help text
  "runs no test, writes no repository state" (`cmd/corvint/help.go:610`).
- `cem verify` and `cem status`: exits `internal/cem/cli/cli.go:320-339` (0 when the envelope says
  `"ok": true`, 1 when it says false, 2 on a dispatch error); `ok`, `mutates` and `state`
  `internal/cem/workflow/read.go:59-87`; refusals pass through the ledger recorder
  (`cmd/corvint/main.go:1047`, `cmd/corvint/main.go:1398-1413`).
- `witness`: profile `internal/witness/witness.go:27`; exits `cmd/corvint/witness.go:105-138`;
  recorder `cmd/corvint/main.go:1009`; help "does not mutate repository or trace state"
  (`cmd/corvint/help.go:393-394`). It builds the index in memory when no snapshot exists
  (`cmd/corvint/index_snapshot.go:79-84`); the fresh-clone run wrote no snapshot store (`.git/corvint/index`).
- `impact --base`: profile `internal/contextindex/range_impact.go:24`; exits and ledger calls
  `cmd/corvint/main.go:1051-1056` and `cmd/corvint/main.go:1077-1136`; envelope
  `cmd/corvint/main.go:1261`; range capacity from `corvint impact --help`. With no snapshot it builds
  the index in memory (`cmd/corvint/main.go:1080`) and writes no snapshot store (`.git/corvint/index`).
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
(`internal/contextindex/git.go:24`, `internal/contextindex/index.go:308`,
`internal/contextindex/range_impact.go:74`), and `affected` reads `git status` under 10 s
(`internal/liveverify/affected/dirty.go:34`, `internal/liveverify/affected/dirty.go:50`). Otherwise
the process cancels only on `SIGINT` or `SIGTERM` (`cmd/corvint/main.go:1461`,
`cmd/corvint/signals_unix.go:10-11`); the exit code a signal produces is not documented here
(unknown). The automation host must set its own step timeout. No timeout has been qualified; 5
minutes per step is a suggestion that leaves more than 20 times the loaded measurements as
headroom, and it is a hang detector, not a budget.

## Not read-only: excluded as triggered read-only steps

| Command | Why it is excluded | Source |
|---|---|---|
| `corvint cem report` | Writes the review report, by default to `$GIT_DIR/corvint/cem-review.md`, and returns `"mutates": true`. | `internal/cem/workflow/workflow.go:29`, `internal/cem/workflow/read.go:205-237` |
| `make dogfood-check BASE=...` | Once its preconditions pass it rewrites `.corvint/dogfood-report.json`, builds verifier binaries into `$GIT_DIR/corvint/`, and uses `/tmp/corvint-go-build-cache`. It is an authoring-time step, not a gate prerequisite, because it needs untracked `.corvint/` artifacts a clean checkout never has. In a fresh clone it fails before any of those writes (below). | `script/dogfood-check.sh:113-135`, `script/dogfood-check.sh:165-178`, `Makefile:35-38` |
| `make dogfood-change`, `make dogfood-seal`, `corvint index`, `cem begin`, `prepare`, `cite`, `mark`, `cover`, `discriminate`, `anchor` | Write the dogfood report, a commit, the index snapshot, CEM maps, the patch cache or a Git note by design. | `script/dogfood-change.sh:27`, `script/dogfood-seal.sh:17-20`, `SOP-V0-002` for `index`, `cmd/corvint/help.go:361-363` and `corvint cem --help` |

`script/dogfood-check.sh` exits 0 on `PASS`, 1 on `FAIL`, 2 on `REFUSE` or a Git error (the
missing-`rg` refusal exits 1, `script/dogfood-check.sh:8-10`), and 129, 130
or 143 on `HUP`, `INT` or `TERM` (`script/dogfood-check.sh:88-90`); `make` reports any non-zero
recipe exit as 2, which the worked example observed.

**Fresh-clone limitation (V1-0182, open).** The task-store title of ticket V1-0182 states it
verbatim: "dogfood-check: in a fresh clone the check fails `dogfood-report-missing` before any
verifier runs, so a reviewer cannot reproduce the override-verifier comparison". The worked example
reproduces it: `script/dogfood-check.sh:305-308` exits 1 with `FAIL dogfood-report-missing` because
`.corvint/dogfood-report.json` is gitignored and only the author's `dogfood-change` writes it. A
fresh-clone trigger can therefore verify a sealed CEM with `cem verify`, but cannot reproduce the
author's dogfood-check verifier comparison.

The existing pull-request wrapper [`examples/cem/verify-pr.sh`](../examples/cem/verify-pr.sh),
documented with its own 0/1/2/3 exit table in [CEM in CI](CEM-CI.md), runs `cem verify` from a
trusted base checkout. It creates a temporary directory inside that checkout and removes it on exit
(`examples/cem/verify-pr.sh:101-102`).

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
