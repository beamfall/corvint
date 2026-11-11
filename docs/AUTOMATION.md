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
  default binary (`docs/specs/corvint-1.0-product-and-release-v1.md:203@b7b9e11e`). Decision 0081
  (`docs/decisions/0081-local-admin-console-accepted-2026-09-07.md:24-31@4e183223`) keeps even the optional
  console operator-started, foreground only, loopback only, with no service and no durable state;
  decision 0594 (`docs/decisions/0594-refocus-on-test-selection-2026-11-09.md`) removed that console.
  The trigger, its schedule, its runner and its retention belong to the automation host, not to
  Corvint.
- **A triggered run grants no authority.** A step's exit code and JSON are evidence for the policy
  that invoked it; they do not approve, merge, accept intent or waive a gate. Project-owned authority
  outranks syntax, history and learned traces (AGENTS.md invariant 3). Decision 0081 stated that the
  since-removed console "carries no authority" (line 25), and decision 0373 took formal host FULL and protected
  authority off the Core path (row 6, line 19); a CI or hook invocation of the same binary gains
  nothing either path lacks. In particular: `affected` never claims omitted tests are safe to skip
  (`cmd/corvint/help.go:510-511@d5d6e05c`).
- **Network.** "Local-only; no network or telemetry" (root help support boundary)
  (`cmd/corvint/help.go:290@306e7664`). Fetching the commits a step needs is the runner's job.

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
(`internal/observations/observations.go:309-311@f504ccce`, `internal/observations/observations.go:733-740@b40a3bba`).
At `01b6804` the root `.gitignore` names the ledger but not its temporaries, so in a fresh clone
with no `.corvint/.gitignore` (which `corvint index` creates) no row is written; the worked example
and a forced `unsupported-impact-path-suffix` refusal both left `.corvint/` unchanged. A ledger
storage failure never alters the response (`cmd/corvint/help.go:293-295@96bde7a3`).

## Commands safe as a triggered step

Each takes `--root PATH` (default: the current directory) and reads the checkout at `HEAD`. Every
base must be a full 40-hex commit id that is present in the clone, so a shallow CI checkout must
fetch enough history to contain it (`cmd/corvint/help.go:450@a892984d`). On an argument or input refusal each
prints one JSON object with `"ok": false` to stderr and exits 2.

Scope: the table lists the steps that answer a question about one change against a fixed base,
which is what a CI job, git hook or team automation asks. Other verbs that help also describes as
read-only (`init`, `adopt` and `docs`) are not classified here. They need a per-session task or
request as input, or they are experimental without a stability promise. This page makes no claim
about their safety as a triggered step.

| Step | Class | Exit codes | Output profile (stdout) | Required inputs | 1.0 label |
|---|---|---|---|---|---|
| `corvint affected --base BASE` | `R0` | 0 plan written; 2 refusal or error | `affected-plan/0`, one canonical JSON line, `"mutates": false` | full `BASE`; a checkout whose `HEAD` is the change head | Core |
| `corvint impact --base BASE [--range-profile expanded-256]` | `R0+SOL` | 0 packet written; 2 refusal or error, including a range over capacity (`unsupported-impact-range`) | JSON with `"tool": "impact"`, `"mutates": false`; `context.profile` is `corvint-range-impact/0` | full `BASE`; the default range capacity is 100 paths, 256 with the explicit profile | Core |

Where each row was verified:

- `affected`: profile `cmd/corvint/affected.go:39@af9fb447`; exits `cmd/corvint/affected.go:869-935@3676ae1b`; its
  refusals are emitted without the ledger recorder (`cmd/corvint/main.go:746-751@7ed11581`); help text
  "runs no test, writes no repository state" (`cmd/corvint/help.go:510@f024c3ab`).
- `impact --base`: profile `internal/contextindex/range_impact.go:24@49493543`; exits and ledger calls
  as for `affected`; envelope
  `cmd/corvint/main.go:932@dedd82e3`; range capacity from `corvint impact --help`. With no snapshot it builds
  the index in memory (`cmd/corvint/main.go:810@7219f23e`) and writes no snapshot store (`.git/corvint/index`).
- 1.0 labels: `docs/specs/corvint-1.0-product-and-release-v1.md` classification table.
- Every exit code in the table was also observed: 0 in the worked example and 2 from `affected` and `impact` given a short base.

### Runtime and timeout

Measured on 2026-09-23 on a darwin arm64 host **under heavy load from a concurrent benchmark**, so
these are inflated and are not a performance claim. Two runs of the worked example over a 17-path
range (1 s resolution): `affected` 8 and 9 s, `cem verify` 2 and 2 s, `cem status` 1 and 2 s,
`witness` 7 and 10 s, `impact --base` 8 and 10 s. One run in a working checkout under `time`: 7.7 s,
2.6 s, 2.0 s, 12.0 s and 8.8 s wall. `witness` was removed by decision 0594; its figures are kept
as part of that dated run.

Corvint sets no whole-command timeout on these steps. Some internal Git steps carry their own
deadline and fail the command when it expires: the index build and the range-impact diff use 30 s
(`internal/contextindex/git.go:27@e8c8d0ea`, `internal/contextindex/index.go:344@c925936b`,
`internal/contextindex/range_impact.go:124@c925936b`), and `affected` reads `git status` under 10 s
(`internal/liveverify/affected/dirty.go:37@bb3abea6`, `internal/liveverify/affected/dirty.go:53@d3f19770`). Otherwise
the process cancels only on `SIGINT` or `SIGTERM` (`cmd/corvint/main.go:1188@5434511f`,
`cmd/corvint/signals_unix.go:11-12@4a4dfd65`); the exit code a signal produces is not documented here
(unknown). The automation host must set its own step timeout. No timeout has been qualified; 5
minutes per step is a suggestion that leaves more than 20 times the loaded measurements as
headroom, and it is a hang detector, not a budget.

## Worked example: a fresh clone at a fixed commit

The script clones a source repository with `git clone --no-local`, checks out `01b6804` detached,
builds `corvint` from that tree into the work directory, runs every step above against the sealed
CEM of V1-0204 (bind commit `ace0a96`, base `a6a6b8b`, the pin's first parent), runs
`make dogfood-check` to show the fresh-clone limitation, and compares the worktree status (with
ignored files), the `.corvint/` file list and the Git-directory entries before and after. It needs
`git`, Go 1.27.1, `make` and `jq`. `SOURCE_REPO` is any clone that contains the pinned commit.

This is a record of the 2026-09-23 run at that pinned commit, not a current recipe. Decision 0594
removed `corvint witness` and the dogfood make targets (`make dogfood-check` included), so a
checkout after that decision has neither; the script still reproduces only at `01b6804`. For a
current triggered step, use the rows in the table above.

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

## Adopter CI recipe

`script/adopter-ci-advisory.sh RELEASE_DIR [BASE]` (ADS-V0-650) is the reusable step an adopting
repository adds to its own CI. Copy it into the adopter at a pinned Corvint commit; it is
Apache-2.0 like the rest of Corvint. It downloads nothing. Given a directory that holds a release
archive and the release `SHA256SUMS`, it:

1. selects `corvint_<os>_<arch>.tar.gz` for the runner and refuses unless `SHA256SUMS` names it
   exactly once and the archive matches, before extracting anything;
2. extracts into a fresh directory, verifies the archive's own `SHA256SUMS`, and probes
   `corvint --version`;
3. runs `corvint adopt --setup` and, when `BASE` is a full 40-hex commit, `corvint affected --base
   BASE`, both against `CORVINT_REPO` (default: the current directory).

Steps 1 and 2 exit 1 on any failure, and no binary runs before both checksums pass. The step-3
verbs are advisory: each one's stdout, stderr and exit code are kept in `CORVINT_ADVISORY_OUT`
(default `$RUNNER_TEMP/corvint-advisory`), a failure prints a `::warning` annotation, and the
script still exits 0. Under GitHub Actions it also writes a step summary and adds the verified
directory to `GITHUB_PATH`. Release archives are unsigned, so the checksums detect changed bytes
but not who published them (`docs/INSTALL.md`).

A GitHub Actions job for a pull request, with the release tag pinned by the adopter:

```yaml
corvint-advisory:
  runs-on: ubuntu-latest
  continue-on-error: true          # advisory: never blocks the pull request
  permissions:
    contents: read
  steps:
    - uses: actions/checkout@v4
      with:
        fetch-depth: 0             # BASE must be present in the clone
    - name: Download the pinned Corvint release
      env:
        GH_TOKEN: ${{ github.token }}
      run: |
        mkdir -p "$RUNNER_TEMP/corvint-release"
        gh release download v1.0.0-rc.3 --repo beamfall/corvint \
          --pattern 'corvint_linux_amd64.tar.gz' --pattern SHA256SUMS \
          --dir "$RUNNER_TEMP/corvint-release"
    - name: Corvint advisory (adopt --setup, affected)
      run: |
        base=$(git merge-base "origin/${{ github.base_ref }}" HEAD)
        ci/adopter-ci-advisory.sh "$RUNNER_TEMP/corvint-release" "$base"
    - uses: actions/upload-artifact@v4
      if: always()
      with:
        name: corvint-advisory
        path: ${{ runner.temp }}/corvint-advisory/*.*
```

`ci/adopter-ci-advisory.sh` is wherever the adopter keeps its copy. On a push or merge-queue run
without a base, pass no `BASE`: the summary then records `affected: skipped, no BASE`. Use the
release tag the adopter has qualified; this page does not choose one. Limits: the recipe was run
on Linux amd64 against a locally packed archive of a source build at this lane's commit and a
two-commit repository (both advisory steps exited 0), not yet from a sibling repository's hosted
CI; `script/adopter-ci-advisory_test.sh` covers the refusals.

