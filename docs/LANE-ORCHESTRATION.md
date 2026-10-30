# Lane orchestration

This is the maintained procedure for running several agent lanes on one host to develop Corvint:
one orchestrator session dispatches builder lanes (one branch and one checkout each), and the lanes
share the host's CPU, memory, disk and CI capacity. The governing requirements are `ASD-V0-013` to
`ASD-V0-018` in [Corvint Self-Development V0](specs/corvint-self-development-v0.md). The per-change
loop each lane runs stays in [DOGFOOD](DOGFOOD.md); ticket authority stays with the orchestrator
and the Tasks store (AGENTS.md "Agent routing"). A lane charter generalises this page for one batch;
it may narrow the rules here but never loosens them.

Roles. The **orchestrator** owns the Tasks store (claims, heartbeats, gates, completion), lane
admission, CI and merge-queue waits, the idle watchdog and checkout cleanup. A **builder lane** owns
one branch: spec, implementation, focused checks, one dogfood finish, one push and one PR. A lane
never runs `corvint-tasks`, never edits `.taskman/` or `.github/`, never starts background jobs or
subagents, and never touches another lane's checkout.

## 1. Admission: cap heavy lanes by host pressure (ASD-V0-013)

A lane is **heavy** when it will run `go test`, `go build`, `go vet` or a make target that builds
Go; a docs/script-only lane is **light**. Light lanes are not counted against the cap.

The cap reuses the host-pressure signal of the dispatcher throttle (`CAL-V0-068`, V1-0694,
`internal/tasks/dispatch/pressure*.go`): one-minute load average per online CPU, plus the kernel
memory-pressure level on Darwin (`kern.memorystatus_vm_pressure_level`: 1 normal, 2 warn,
4 critical) or used/total swap on Linux. The dispatcher applies it only to workers that
`corvint-tasks dispatch` launches. Agent lanes are started by the orchestrator session itself, so
that admission hook never sees them; the orchestrator samples the same inputs at each heartbeat and
applies the same classification and hysteresis by hand:

```sh
sysctl -n vm.loadavg hw.ncpu kern.memorystatus_vm_pressure_level   # Darwin
cat /proc/loadavg; getconf _NPROCESSORS_ONLN; grep -E '^Swap(Total|Free):' /proc/meminfo  # Linux
```

| Level | Entered when (any signal) | Released when (all signals) | Heavy lanes admitted |
|---|---|---|---|
| 0 calm | - | load/CPU < 1.0 and memory normal (swap < 0.5) | 4 |
| 1 high | load/CPU >= 1.5, memory warn, or swap >= 0.7 | as level 0 | 3 |
| 2 critical | load/CPU >= 3.0, memory critical, or swap >= 0.85 | load/CPU < 1.5 and memory not critical | 1 |

A level changes only after two consecutive heartbeat samples agree (`ticksToChange: 2`). An
unreadable sample keeps the current level (it never counts as calm). As in `CAL-V0-068`, pressure
holds only **new** heavy launches: running lanes are never stopped for pressure. These thresholds
are lane defaults for a 12-core host whose lanes each run `-p 4` tests; the dispatcher's own
`pressure` configuration stays separate. On the incident host (12 CPUs, load average 60, that is
5 per CPU) the table gives level 2 and admits one heavy lane, where seven were running.

## 2. Every heavy Go command goes through `script/heavy.sh` (ASD-V0-014)

```sh
script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run '^TestName$' ./internal/pkg
```

Lane test recipe (ASD-V0-060, V1-3213): when the output is piped, start the step with
`set -o pipefail;` so a refusing `heavy.sh` fails the step and a following `&& git commit` never runs:

```sh
set -o pipefail; script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -run '^TestName$' ./internal/pkg 2>&1 | tail -n 40 && git commit ...
```

Without `pipefail` the pipeline status is `tail`'s, so a refusal (exit 2) is lost and the commit goes in
with no test run. A refusal prints `heavy.sh: REFUSED <reason> (exit 2)` on stdout too, so the tail shows it.
Other docs cite this paragraph instead of repeating it.

`script/heavy.sh` is a host-wide counting semaphore: `HEAVY_SLOTS` commands run at once (default
CPUs/4, so 3 on 12 CPUs), the rest wait quietly and print `heavy.sh: slot N/M after Ss` when
admitted. Each admitted command gets `GOFLAGS=-p=HEAVY_P` (default CPUs/slots) unless `GOFLAGS`
already sets `-p`; an explicit `-p` argument still wins. Slots are `flock(2)` locks in
`HEAVY_LOCK_DIR` (default `/tmp/corvint-heavy-<uid>`). The command inherits the locked descriptor
and runs in its own process group, so the slot stays held while the workload runs even if the
wrapper is killed, INT/TERM/HUP reach the whole group, and the kernel releases the slot when the
last holder exits, so a killed lane never strands a slot. Every lane on a host must use the same lock
directory and slot count. Rules: one package per command, a narrow `-run` for `./cmd/corvint`, no
`go test ./...` and no `make gate` in a lane (AGENTS.md "Verify"), and `make dogfood-finish` /
`make dogfood-batch-check` (which build two verifiers) also run through the wrapper.

A package slower than the 600 s foreground limit runs as shards (ASD-V0-026):

```sh
make test-shard PKG=./internal/tasks/store SHARDS=8 SHARD=0   # then SHARD=1..7
make test-shard PKG=./internal/tasks/cli SHARDS=4 SHARD=0     # then SHARD=1..3
```

`tools/test-shards` lists the package's top-level tests and places each by a hash of its name;
the last shard `-skip`s the others' names, so together the shards run every test exactly once,
including one added after the listing. Run one shard per command. Placement is not cost-balanced:
on 2026-10-21 (load 16-170) the slowest store shard took 447 s and the slowest cli shard 154 s. A
shard that nears the limit needs a larger `SHARDS`. `TEST_SHARD_FLAGS=-json` adds `go test` flags.

The light-weight prototype this replaces (`mkdir` slots with a PID file and dead-owner reclaim)
could remove a live owner's slot when two waiters reclaimed the same dead slot at once; the
`flock` form has no reclaim step. Cut over all lanes of a host together, because the two forms do
not see each other's slots. `make lane-scripts-test` runs the serialisation check (two concurrent
commands with `HEAVY_SLOTS=1` never overlap), the killed-wrapper, dead-owner and group-signal
checks.

From any clone, scratch or plain, the entry point is that clone's own `script/heavy.sh`; there is
no other copy to find. Put the host's settings (`HEAVY_SLOTS`, `HEAVY_LOCK_DIR`, `HEAVY_REMOTE_HOST`
and the rest) in `HEAVY_CONFIG` (default `~/.config/corvint/heavy.env`) rather than a lane's
environment, so every clone on the host shares one semaphore and one remote. A plain `git clone` of a
lane checkout offloads as that checkout's origin at its `origin/main` (ASD-V0-056). A checkout that
cannot offload prints `heavy.sh: offload unavailable from this checkout: REASON` and runs locally.

### Offload to a Linux host (ASD-V0-028)

With `HEAVY_REMOTE_HOST` set, `script/heavy.sh` runs `go test`, `go vet` and `make windows-vet`
on that ssh host instead of the lane host, so lanes on a laptop stop competing for its CPUs. Put
the settings in `~/.config/corvint/heavy.env` (`KEY=VALUE` lines for `HEAVY_*` keys only; the file
is read, never executed, and the environment wins; a `#` starts a comment only at the start of a
line), for example for a Windows host with WSL:

```sh
HEAVY_REMOTE_HOST=user@host
HEAVY_REMOTE_SHELL='wsl -d Ubuntu -- bash -s; exit $LASTEXITCODE'
# a plain clone of the same origin, fetched on demand
HEAVY_REMOTE_CLONE=~/corvint
# puts the pinned Go first on the remote PATH
HEAVY_REMOTE_ENV=~/bench/go.env
# remote concurrent commands; leave room for co-tenants
HEAVY_REMOTE_SLOTS=4
HEAVY_SLOTS=3
```

The remote reproduces the checkout's exact tree (unpushed commits, the uncommitted diff,
untracked files under 50 MB, whenever `git ls-files -o --exclude-standard` lists any) in a
reused per-slot worktree, checks that its tree equals the local one (else the run counts as
unavailable and falls back), prints
`heavy.sh: running on <host> <os>/<arch> <go> at <commit>[ +wip] tree=<tree> dirty=<n>`, queues on its own flock
semaphore and returns the command's status. Only the argv travels, so test environment goes in
`env K=V` inside the command; a local `GOCACHE`/`GOMODCACHE` assignment is dropped. A command with
a file-writing flag (`-o`, `-c`, `-coverprofile`, profiles, `-trace`, `-test.*`, also inside a
`GOFLAGS=` argument or the exported `GOFLAGS`) runs locally, because its files would stay remote; so does `HEAVY_REMOTE=0` (use it for darwin-specific tests).
Checkout files a remote test writes, such as goldens regenerated with `-update`, are copied back
over a second ssh connection and named in `heavy.sh: copied back PATH` lines (DCW-V0-359); when a
local edit made during the run touches one of them, nothing is copied and the wrapper exits 96
asking for a local run. `script/heavy.sh --print-lock-dir` prints the slot lock directory a run
would use (DCW-V0-357).
When the host is unreachable the offload is retried once after `HEAVY_REMOTE_RETRY` seconds
(default 15), then the command runs locally and says so. Each local run of an offloadable command
appends `UTC pwd=... reason=offload-unavailable|HEAVY_REMOTE=0|writes-files cmd=...` to
`HEAVY_FALLBACK_LOG` (default `/tmp/corvint-heavy-<uid>/fallback.log`); the orchestrator reads it at
each heartbeat, and a run of `offload-unavailable` lines means the host is down, not that lanes may
run more local tests. A remote host shared with CI runners (the reference box hosts 8) needs a
`HEAVY_REMOTE_SLOTS` that leaves them CPU, and its timings are not quiet-host timings.
`HEAVY_REMOTE_TIMEOUT` (default 40m, enforced with the remote's `timeout(1)`) bounds the command
once it holds a remote slot; the slot wait is a queue and is not bounded. A TERM, INT or HUP to the
wrapper stops its ssh at once, but the remote command keeps running until it ends or that timeout
stops it, so a lane that kills an offloaded run should not immediately start another.
`tree=` is the `git write-tree` of the checkout including uncommitted and untracked files (computed
from a scratch index) and `dirty=` counts paths that differ from `HEAD`, so a result names the exact
content it tested; a local run prints `heavy.sh: running locally at HEAD tree=<tree> dirty=<n>`.
The remote command's `TMPDIR` is a fresh per-run directory under
`${HEAVY_REMOTE_TMPDIR:-~/.cache/corvint-heavy/tmp}` on the remote disk, removed afterwards.
The worktree is not temporary (ASD-V0-031, V1-2819): each remote slot owns
`~/.cache/corvint-heavy/wt/heavy-slot<i>`, reset to the tested tree on every run, because Go keys
compiled packages by directory and a fresh path per run rebuilt the whole module (a filtered
`./cmd/corvint` run fell from about 23 s to about 6 s). Keep `~/.cache/corvint-heavy/wt` out of any
remote sweep. Each run prints `heavy.sh: phase receive= wait= prepare= run= worktree=heavy-slot<i>/reused|created`,
and every offload appends `offload host= pack= remote= rc=` to `HEAVY_SELECTION_LOG`.

Run telemetry (ASD-V0-037, V1-2905): `script/heavy.sh` is the one runner. Its remote half is the
same file, shipped over ssh on every offload, so no copy on the box is maintained by hand. Every
run prints `heavy.sh: script sha256=<hash> <path>`, and every selection-log line ends with
`script=<first 12 hex>`, so a log row names the runner revision that wrote it. The offload line
carries the remote's phase seconds (`rc=N receive= wait= prepare= run= worktree=`); a local run
appends `local wait=Ns run=Ns rc=N` after it exits; a fallback appends `fallback host=H`. When
`HEAVY_FALLBACK_ALERT` (default 3, `0` disables) fallbacks for one host follow each other with no
offload in between that the host completed (rc other than 97 or 255), the wrapper prints
`heavy.sh: ALERT remote-unavailable streak=N host=H` and logs an `alert` line; check the host with
`script/wsl-keepalive-check.sh`. `HEAVY_REMOTE_BUSY_PROCESS=Runner.Worker` makes an offload
unavailable (status 97, so the run falls back) while that process runs on the remote, the CI
guard the box prototype carried; the CI guard below also covers queued runs. Goldens a remote test
regenerates already come back through copy-back (DCW-V0-359); no fetch flag is needed. The
cutover from the box prototype is in "Cutover from the lane prototype" below.

Lane policy (ASD-V0-030, the lane prototype's rules): a `go test` without `-run` is refused unless
`HEAVY_WHOLE_REASON` says why the whole package runs (Corvint selected it whole, or its
UNKNOWN/FALLBACK reason; pick units with `corvint test --base <sha> --dry-run`), and every `go test`
is recorded in `HEAVY_SELECTION_LOG` as `selected`, `whole reason=...` or `refused`. `HEAVY_REMOTE=0`
needs `HEAVY_LOCAL_REASON` for an offloadable command. A local run first waits up to
`HEAVY_LOAD_WAIT` (120 s) while the 1-minute load exceeds `HEAVY_MAX_LOAD` (10). The lane
cutover below keeps the existing selection and fallback log paths so the canary counts keep
reading the same files.

Declared host differences (V1-2563): the reference box's `/tmp` is a 16 GB tmpfs, but its CI
runners (`corvint-box-1..8`) set `TMPDIR=/var/tmp/corvint-ci/tmp/<runner>` (ext4), the offload env
file sets `TMPDIR` on the home disk, and offloaded commands get the per-run directory above, so
none of them writes test temporaries to tmpfs. The hosted `ubuntu-24.04` runner's temporary
filesystem has not been observed; a test that depends on filesystem type (rename atomicity,
`O_DIRECT`, inode reuse, free space) must not assume either.

### CI guard and build offload (ASD-V0-040, ASD-V0-041)

The offload host's spare CPUs belong to the CI runners it hosts, so the owner rule of 2026-10-28
is that no lane work runs there while CI is running (ASD-V0-040, V1-2983). Before any offload
(test, vet or build), `script/heavy.sh` runs
`gh run list --repo HEAVY_CI_REPO --status S --limit 5 --json databaseId --jq length` for each
status in `HEAVY_CI_STATUSES` (default `queued in_progress`), each bounded by `HEAVY_CI_TIMEOUT`
seconds (default 20). Any counted run, a failed query, a timeout or a non-numeric answer keeps the
command local: it prints `heavy.sh: CI is running or unknown (repo=R runs=N|status=S query-failed);
not using HOST, running locally`, appends a `ci-running` line to `HEAVY_FALLBACK_LOG` and a
`ci-busy` line to `HEAVY_SELECTION_LOG`, and needs no `HEAVY_LOCAL_REASON`. It is not a host
fallback, so it does not feed the `ALERT remote-unavailable` streak. `HEAVY_CI_REPO` defaults to the
checkout's origin when that is on github.com; `HEAVY_CI_REPO=off` (or a non-GitHub origin) skips
the query. `HEAVY_CI_GH` names the `gh` command, which the tests replace with a stub, so `make
lane-scripts-test` exercises the guard without network.

CI moved to GitHub-hosted runners on 2026-10-28 (repository variable `CORVINT_SELF_HOSTED=off`), so
CI no longer uses the box and the guard would only push work back onto the Mac. Under
`HEAVY_CI_GUARD=auto` (the default) `script/heavy.sh` first runs `gh variable get CORVINT_SELF_HOSTED
--repo HEAVY_CI_REPO`, bounded by `HEAVY_CI_TIMEOUT`: the answer `off` skips the run query and the
command offloads. Any other answer, a failed query or a timeout keeps the guard in force (fail
closed). `HEAVY_CI_GUARD=on` always runs the run query without asking for the variable, and
`HEAVY_CI_GUARD=off` never guards; another value exits 2 (`invalid-ci-guard`). If CI moves back to
the box runners, setting the variable to `on` restores the guard with no local change.

Builds offload too (ASD-V0-041, superseding ASD-V0-037's "not ported"): `go build`, `make ... build`
and `make ... tasks-build` cross-compile on the remote for this host's `GOOS`/`GOARCH` in one try,
only when no CI runs and the remote's 1-minute load is at most `HEAVY_REMOTE_BUILD_MAX_LOAD`
(default 16, the prototype's threshold). The cross-build uses Go's default `CGO_ENABLED=0`; a
build that needs cgo sets `HEAVY_BUILD_OFFLOAD=0`. An absolute output (`-o P`, `-o=P`,
`NAME_BIN=P`, `P` ending in `/` for a directory) is built under the remote worktree and installed at
`P`; a relative one lands at its checkout path. Every file the build wrote is copied back and named
in a `heavy.sh: copied back` line. A busy or unreachable remote, a failed copy-back, an argument with
a `..` component or a file-writing `GOFLAGS` builds locally with a `build-local` fallback-log line;
`HEAVY_BUILD_OFFLOAD=0` keeps every build local. A failing remote build returns its own status and
is not rebuilt locally.

### Cutover from the lane prototype

The orchestrator performs the cutover between batches, never under running lanes, because the
prototype's `mkdir` slots and `script/heavy.sh`'s `flock` slots do not see each other. The
prototype (`/private/tmp/cem10-build/heavy.sh`, `heavy-offload.sh`, `~/bench/rt.sh` on the box)
carries the CI guard, so the cutover must keep it: `HEAVY_CI_REPO=beamfall/corvint` is set
explicitly (not left to the origin default), and before pointing lanes at `script/heavy.sh` the
orchestrator runs one offloadable command with `HEAVY_CI_GUARD=on` while a CI run is queued or in
progress and sees the `CI is running or unknown` line. Set in `~/.config/corvint/heavy.env`:

```sh
HEAVY_REMOTE_HOST=russe@192.168.4.128
HEAVY_REMOTE_SHELL='wsl -d Ubuntu -- bash -s; exit $LASTEXITCODE'
HEAVY_REMOTE_CLONE=/home/russell/corvint
HEAVY_REMOTE_ENV=/home/russell/bench/go.env
HEAVY_REMOTE_BUSY_PROCESS=Runner.Worker
HEAVY_CI_REPO=beamfall/corvint
HEAVY_CI_STATUSES="queued in_progress"
HEAVY_CI_GUARD=auto
HEAVY_REMOTE_BUILD_MAX_LOAD=16
HEAVY_SLOTS=3
HEAVY_MAX_LOAD=10
HEAVY_SELECTION_LOG=/private/tmp/cem10-build/heavy-selection.log
HEAVY_FALLBACK_LOG=/private/tmp/cem10-build/heavy-fallback.log
```

Then point the operator wrapper and the lane charter at `script/heavy.sh` and retire the prototype
and `rt.sh`. `gh` must be authenticated on the lane host; an unauthenticated `gh` fails every query
and so keeps every command local, visible as `query-failed` lines in the fallback log.

### WSL keepalive (ASD-V0-039)

A WSL distro stops a few seconds after its last client exits, so an offload host reached over ssh
loses its slots, caches and running tests between sessions unless a Windows-side process holds the
distro open. `script/wsl-keepalive.ps1` is that holder: it runs
`wsl.exe -d Ubuntu --exec /bin/sleep infinity`, restarts it 30 s after any exit, keeps one copy per
distro (a named mutex) and logs each start and exit under `%LOCALAPPDATA%\corvint\`. It is meant to
run as a boot-triggered scheduled task with an S4U principal, so it needs no interactive logon and
survives reboots and ssh exits. `script/wsl-keepalive-check.sh [--host U@H] [--distro D] [--task T]`
runs one read-only query over ssh and prints `holder pid= parent=` per holder and the task's state;
it exits 1 with `FAIL no-holder` when the distro has no Windows-side holder, warns
`task-not-running` when holders exist but none would come back after a reboot, and exits 3 when the
host cannot be queried. Installing or changing the task is the owner's step, with the commands in
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-27-host-tooling-telemetry.md`; the repository never installs it.

Go filename pitfall: a file name ending in a GOOS or GOARCH element (`_js.go`, `_wasm.go`,
`_android.go`, `_386.go`) is a build constraint, so `validity_js.go` silently builds only for
GOOS=js. Name JavaScript/TypeScript analyser files `_jsts.go` or similar; `make
goos-filenames-check` (a `make gate` step, `ASD-V0-020`) refuses such names outside
darwin/linux/windows/amd64/arm64 unless allowlisted.

## 3. Dogfood inputs up front, one finish after the last edit (ASD-V0-015)

The charter or the lane's first step fixes every dogfood input before work starts:

- `BASE`: the full 40-hex `origin/main` commit the branch starts from (a merge commit after the one
  allowed main merge);
- `CITATIONS`: the citation plan file (distinct spans, citing only files the diff does not modify);
- `INTENTS`: the governing spec paths; `VERIFY`: one shell-free command per line;
- `OUTCOME`: `passed`, `failed` or `blocked`.

Write the `CITATIONS`, `INTENTS` and `VERIFY` files into a directory only this lane writes, outside
the tree: `"$(git rev-parse --absolute-git-dir)/corvint/lane-inputs/"` is unique per worktree. Build a
lane-local `corvint` there too (`go build -o "$(git rev-parse --absolute-git-dir)/corvint/bin/corvint"
./cmd/corvint`), never into `./bin/`: that path is not ignored, so finish-lane refuses the
untracked binary as `dirty-worktree` (`DCW-V0-397` now names it). A
shared path such as `/tmp/cites.tsv` lets another lane overwrite the plan between prepare and bind;
finish-lane hashes every supplied input file before prepare and refuses
`REFUSE input-changed-during-run step=<step>` naming each changed file (`DCW-V0-378`), instead of the
`intent-scope-drift` or `citation-plan-map-mismatch` the rewritten file would otherwise surface as.

A lane runs `make dogfood-change BASE=<sha>` once at the start (pre-change context, rc retained) and
then, after its last edit and commit, exactly one
`script/heavy.sh make dogfood-finish LANE=1 BASE=.. CITATIONS=.. INTENTS=.. VERIFY=.. OUTCOME=..`
(prepare, bind, commit, rebind, check and seal, `DCW-V0-060`). `LANE=1` refuses before any build,
naming every missing or unusable input in one line
(`dogfood-finish-lane: REFUSE missing-inputs CITATIONS BASE:not-full-sha VERIFY ...`). A change
that touches `docs/specs/` without committing regenerated spec indexes also refuses before any build
(`REFUSE spec-requirements-stale`, `DCW-V0-090`): run `make spec-requirements` and commit. At most two
finish attempts are allowed; a second failure stops the lane with the exact command and output in
its report. Then one `make dogfood-batch-check BASE=<origin/main tip>` from a clean checkout.
Rerunning `dogfood-change` to "see where it stands" between edits is not part of the loop.
Every dogfood make target requires an explicit BASE: the Makefile's `BASE ?= main` default
(local `main`, which may not be `origin/main`) is refused, not used (`ASD-V0-019`).

- Windows vet before push (ASD-V0-023, V1-1725): install the pre-push hook once in the shared lane
  repository (its hooks serve every linked worktree):
  `printf '#!/bin/sh\nexec script/pre-push-windows-vet.sh "$@"\n' > "$(git rev-parse --git-path hooks)/pre-push"`
  then `chmod +x` that file. Git runs it from the pushing checkout's top level, so each lane uses
  its own copy. It runs `make windows-vet` when the pushed commits change a `.go`, `go.mod` or
  `go.sum` file and refuses the push when it fails.
- Sealed evidence before push (DCW-V0-348, V1-2805, V1-1773): the batch-head checkout installs
  `script/pre-push-dogfood-seal.sh` the same way. It runs `script/dogfood-batch-check.sh` against
  the pushed branch's merge base with `origin/main` and refuses `dogfood-seal-missing` while any
  commit, including one made after the seal, is unbound; it also refuses (`dogfood-seal-not-checked`)
  a pushed branch that is not the checked-out HEAD or a tree with uncommitted tracked changes. Git's
  hook stdin can be read once, so a hook running both scripts reads it first:
  `refs=$(cat); printf '%s\n' "$refs" | script/pre-push-windows-vet.sh "$@" || exit 1;
  printf '%s\n' "$refs" | exec script/pre-push-dogfood-seal.sh "$@"`. `git push --no-verify` bypasses
  it and stays an explicit, reported choice. Lanes that only commit locally never run it.

## 4. No polling in lanes; long commands are bounded (ASD-V0-016)

Lanes never run `sleep`, `until`/`while` wait loops, `gh pr checks --watch`, `gh run watch` or
background jobs. A lane pushes once, opens the PR, arms auto-merge and reports; CI results,
merge-queue progress and post-merge steps are the orchestrator's, which reads PR state on demand
at its heartbeat instead of waiting on events. A tool call has a 600 s limit; these commands are
known to approach or exceed it and are bounded as listed:

| Command | Observed time and phases | Bound in a lane |
|---|---|---|
| `go test ./cmd/corvint` (whole package) | about 591 s on a quiet host (decision 0082); far more under load (6 lane calls of 600 s or more on 2026-10-23..25, max 1,307 s) | never whole-package: narrow `-run`, through `heavy.sh` |
| `go test ./...`, `make gate` | tens of minutes | not run in lanes (AGENTS.md "Verify"); orchestrator-owned when policy needs it |
| `make dogfood-finish` | build tree verifier, build base verifier, prepare, bind, commit, rebind (the full change steps), check, seal; each lane reports its own phase timings in its PR body | once, through `heavy.sh`; if it would pass 600 s, stop after the bind commit and hand the exact command to the orchestrator |
| `make dogfood-batch-check` | builds the verifiers, then re-verifies every lane in the range | once per lane at the end; per batch it is the orchestrator's |
| `corvint cem verify --guarding-tests` | runs the change's tests for up to 1200 s | not run in lanes |
| CI and merge-queue waits | minutes to hours | orchestrator only |
| `go test ./internal/tasks/store` (whole package) | 10 lane calls of 600 s or more on 2026-10-23..25 (median 602 s, max 702 s); CI 2739 s | `make test-shard SHARDS=8`, one shard per command (section 2) |
| `go test ./internal/tasks/cli` (whole package) | CI 545 s; lane median 175 s on 2026-10-23..25 (42 s on 2026-10-14..17) | `make test-shard SHARDS=4` |
| `go test ./internal/doccorpus` | 7 lane calls of 600 s or more (load-bound); 52 s on WSL2 at load 33 | offload (section 2) and one package per command |
| `go test ./internal/contextindex`, `./conformance/mcp-2026-07-28`, `./internal/mcp/corpusbridge` | 604-707 s in a lane under load; CI 59 s, 27 s and 5.5 s | offload, narrow `-run` |

The 2026-10-23..25 counts come from `script/lane-transcript-metrics.py`
(`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-25-lane-infra-p0-evidence.md`): 48 tool calls ran 600 s or longer, 15 of
them unclassified Bash commands; the timeout target in section 7 was not yet met.

The rule is enforced by `script/lane-guard.sh` (ASD-V0-024, V1-1953), a PreToolUse(Bash) hook the
operator installs in Claude Code settings with `LANE_GUARD_ROOTS=<lanes-root>` (and, once lanes use
the repository semaphore, `LANE_GUARD_HEAVY=script/heavy.sh`, the default). It denies polling loops and `go test`
without `heavy.sh`, and ignores heredoc bodies, so writing a file that contains a loop is allowed.
For a subagent (the hook input has an `agent_id`; install it for `Bash|Monitor`) it also denies
background work: `run_in_background`, `Monitor`, `nohup`/`setsid`/`disown` and a shell-level `&`,
while `&&`, redirections such as `2>&1` and `&>`, `|&`, quoted strings, comments and heredoc
bodies are allowed, and so is balanced arithmetic such as `$((a & b))`.

## 5. Idle-lane watchdog (ASD-V0-017)

At every heartbeat (at most 10 minutes apart) the orchestrator runs

```sh
script/lane-idle-check.sh --idle-minutes 30 <each running lane's transcript or output file>
```

which prints `LANE ACTIVE|IDLE idle=<m>m last=<UTC> <path>` from the newest transcript
RFC 3339 `"timestamp"` (or the modification time of a file with no timestamp key) and exits 1 when
any lane is idle. A transcript it cannot read or parse prints `LANE UNKNOWN ... reason=...` and is
flagged like an idle lane. It is read-only.
A lane idle for more than 30 minutes is messaged once with what it should do next. If it is still
idle at the next heartbeat after that message, the orchestrator stops it, releases its claim with
the reason (`idle <m> min since <UTC>, messaged at <UTC>`), retains the reason on the ticket and
redispatches from the lane's pushed branch or from scratch. `--gaps` replays transcripts and prints
every gap above the threshold with the time the watchdog would have flagged it.

## 6. Checkout lifecycle and cleanup (ASD-V0-018)

- One shared lane repository per host: a plain `git clone` of origin, never `--shared` or
  `--reference` (dogfood refuses borrowed objects, `unsupported-object-alternates`). The
  orchestrator fetches it.
- One checkout per lane, created once and reused for every check of that lane:
  `git -C <shared> worktree add -b claude/<slug> <lanes-root>/<slug> origin/main`. The dogfood
  loop supports linked worktrees with per-worktree trace stores and locks (`DCW-V0-044`); never
  switch one checkout between lanes. A fresh plain clone remains the fallback until a live lane has
  run `dogfood-finish` and `dogfood-batch-check` in a linked worktree
  (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-18-v1-1627-lane-orchestration.md`).
- Never run `git stash` in a lane. The stash is one ref (`refs/stash`) in the clone's common
  directory, shared by every linked worktree, so one lane's `git stash pop` can apply another
  lane's stash (incident, 2026-10-21: the dogfood-flow lane's `git stash pop` applied the affected-selection lane's stash `723eec1`, V1-2161). Park work in a
  patch file kept outside the checkout (`git diff > ../p.patch`, then `git apply -R ../p.patch`;
  restore with `git apply ../p.patch`) or in a named WIP commit on the lane's own branch.
  `script/lane-clean.sh` keeps any checkout that still has a stash entry.
- Scratch logs go outside the checkout (untracked files make dogfood report
  `record-index-failed`).
- After a lane's PR merges, the orchestrator fetches the shared repository and runs
  `script/lane-clean.sh --root <lanes-root> --repo <shared> --active <dir>...` (dry run), reviews
  the `WOULD-REMOVE` lines, then reruns with `--apply`. It removes only checkouts whose `HEAD` and
  local branches are ancestors of `origin/main`, with a clean status, no stash and not listed as
  active, and, for a clone, with no registered linked worktree; any failing Git inspection keeps
the checkout, and a root that does not resolve (or is `/`) is refused. Worktrees are removed with
`git worktree remove`. Its `SUMMARY` line carries the counts
  and free disk before and after.
- Go build caches (ASD-V0-021, ASD-V0-038, V1-0360, V1-2904): each host has one Go build cache for
  lanes and heavy runs. On a lane host it is the Makefile's `CORVINT_GOCACHE`
  (`/tmp/corvint-go-build-cache`), which `script/heavy.sh` also gives a local run unless the caller
  set `GOCACHE`; on an offload host it is the remote env file's `CORVINT_GOCACHE`
  (`~/.cache/corvint-go-build-cache` on the reference box), which the remote half exports as
  `GOCACHE`. After each batch merge, and whenever free disk falls below about 20%, run
  `make go-cache-trim` (dry run), review the `WOULD-TRIM` and `WOULD-REMOVE` lines, then
  `make go-cache-trim APPLY=1`. It trims each Go cache to 20 GiB, oldest entries first and never
  one used in the last 12 hours, and removes `go-build*` work directories older than 24 hours that
  no running process names. A trimmed entry is only a later cache miss. When the free share of
  `/tmp` stays under 20% after the trim it prints `WARN low-disk free-pct=N min-free-pct=20`: the
  caches are not what fills the disk, so look at checkouts and scratch next.
- Closed-session scratch (ASD-V0-022, V1-1978): after each batch merge, run
  `make scratch-sweep SCRATCH_SWEEP_ARGS='--root <session-scratch-root> --root <lanes-root>
  --repo <shared> --keep <active-lane-or-session>... --archive <archive-dir>'` (dry run), review
  it, then rerun with `APPLY=1`. It prunes stale worktree registrations and removes directories idle
  for 24 hours; unfinished Git work is archived (patch, untracked tarball, status, bundle) before
  removal and kept when no archive directory is given. Owner-approved toolchain directories go on
  the `--keep` list.
- Local binaries (ASD-V0-025, V1-1979): install a rebuilt tool with
  `script/install-local-bin.sh <built> ~/.local/bin/<tool>` instead of an ad-hoc sibling copy. It
  renames a sibling copy over the target, keeps one `<tool>.corvint-prev-<n>` rollback (more with
  `--keep N`), prunes older ones, and `--rollback` restores it. It never removes other backup names;
  existing ad-hoc `.bak*` files are the operator's to review and delete.
- CEM verifier pair (TCB-V0-046, V1-1761): rebuild and install `corvint` and `cem01-go` together
  from one clean checkout with `script/install-cem-verifiers.sh` (default `~/.local/bin`), so the
  cem-verdict gate's two legs share their source; `--check` reads the installed pair without
  building and names a stale consumer `MISMATCH`.

### Lane build-log entry shape

One entry per lane, `docs/build-log/YYYY-MM-DD-<lane>-<slug>.md`, with these sections in order:
problem, decisions, evidence, `NOT_RUN`, rollback, and **Outcome** (V1-2234). Outcome states the
final disposition per ticket (delivered, already delivered with evidence, skipped with reason), the
dogfood result and every `NOT_PRODUCED` reason. Fill it before the final commit; text in earlier
sections must not say "see the outcome below" unless the entry has that section.

## 7. Measuring the procedure

These are measured over each batch run under this procedure and retained in a build-log entry.
`script/lane-transcript-metrics.py <transcript-dir> --since D --until D` prints the dogfood,
polling, timeout, long-call and focused-test counts as JSON (counts and durations only, no
transcript text). The first before/after measurement (2026-10-14..17 against 2026-10-23..25) is
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-25-lane-infra-p0-evidence.md`.

- Pressure (V1-1627): sample `vm.loadavg` at each heartbeat; report median one-minute load per CPU
  and the median wall time of one representative focused test (for example
  `go test -run '^TestDogfoodFinishLaneDCWV0060RefusesBeforeAnyStep$' ./cmd/corvint`) quiet vs in
  the batch.
- Semaphore (V1-1628): the same test with 5 concurrent lanes, target within 1.5x of quiet; the
  `heavy.sh: slot ... after Ss` lines give the queueing share.
- Dogfood runs (V1-1629): per lane, count `dogfood-change` and `dogfood-finish` invocations in the
  lane transcript; target median at most 2.
- Polling and timeouts (V1-1630): sum tool time spent in sleep/poll commands and count tool calls
  that hit the 600 s limit across lane transcripts; targets under 1 h and under 10.
- Idle lanes (V1-1631): `script/lane-idle-check.sh --gaps` over the batch's lane transcripts.
- Disk (V1-1632): the `lane-clean.sh --apply` `SUMMARY` line of the one-off cleanup.

## 8. Batch pull request pushes (ASD-V0-027)

Every push to a pull request cancels that pull request's in-flight `ci.yml` run (AFP-V0-082
`concurrency`, `cancel-in-progress`), so a batch pushed after each constituent merge pays for runs
that never finish (V1-2253: 12 cancelled runs, about 250 runner-minutes, on 2026-10-21). The batch
integrator therefore:

- merges every constituent branch into the batch branch locally, runs `make batch-regen`
  (ASD-V0-033: regenerates every derived file in one commit, a no-op when nothing is stale), then
  `make batch-precheck` (ASD-V0-029, ASD-V0-034..036: every doc-gates step derived from `ci.yml`
  after the mechanical repins, gofmt, the capability catalog and the cross-lane drift tests as one
  offloaded `go test`, in parallel, about 90 s), and pushes once only when it passes, as
  `make batch-push-check && git push` so a push of a commit the precheck did not pass is refused.
  A lane runs it as `make batch-precheck BASE=<lane base sha>` (ASD-V0-032): a failure the base
  already shows prints as a base-present `WARN` and does not fail the lane, but it still fails
  batch CI, so the lane names it in its report for the integrator to fix or file;
- after a CI failure, fixes every failure the run reported, then pushes once more (one push per
  failure round);
- never pushes a partial batch to "start CI early"; a constituent that arrives late joins the next
  failure-round push or the next batch.

Ledger reuse (ASD-V0-042, V1-2985): each `batch-precheck` step runs through `tools/gate-ledger`
keyed by its name plus a salt of its command and the `GO*`, `CGO_*` and `CORVINT_*` environment, so
a step whose command, environment and declared input scope are unchanged since a recorded pass
prints `batch-precheck: REUSE NAME`, shows `REUSE` in the table, and the run ends with `N of M steps
reused a ledger pass on identical inputs`. A step with no declared scope keys on the whole tree, so
any change reruns it. Reuse is off, and every step runs, when `CORVINT_GATE_LEDGER=off`, when the
index differs from the working tree, when an ignored file a Go test may read is present, or when the
ledger cannot be built. No step is dropped: the batch's final precheck still runs every step, and a
reused step stands only for a pass on byte-identical inputs. `dogfood-closeout` in each lane
therefore pays for a step only when that lane changed its inputs. The lane charter
(orchestrator-owned) drops its explicit doc-check invocations (`spec-requirements-check`,
`traceability-tests-check`, `line-citations-check`, `test-read-scopes-check`,
`unbounded-readers-check`) that closeout's precheck already runs; a lane runs one of them by hand
only to iterate on a failure.

Drift and catalog selection (ASD-V0-050, ASD-V0-051; V1-3134, V1-3136): with `BASE=<sha>`, the
precheck asks `corvint affected --base <sha>` for the batch diff's plan and runs only the drift
tests the diff can reach (by package, or by an input the `DRIFT_TESTS` entry declares), printing
`batch-drift runs K of N drift tests` and each skipped entry with its reason; it skips
`capability-catalog-check` with a printed reason when no catalog input changed. Any doubt (no base,
a failed or UNKNOWN plan beyond the repository's three frontier details, a path that needs
quoting, an ignored test input) runs every drift test and the catalog check and prints why. The
make-run steps share the precheck's Go build cache. Batch CI still runs every test.

Batch review packet (ASD-V0-043, V1-2986): the batch reviewer's prompt takes the output of

```sh
make batch-review-packet BASE=<origin/main sha>   # HEAD=<rev> must name the checkout
```

run on the merged batch branch. It is read-only and bounded: the reviewed revision (base, head,
tree, commit and first-parent merge counts, corvint version), lane overlap (every path changed by
more than one first-parent merge, with those merges), `corvint impact --base BASE`, and `corvint
dogfood batch-check BASE` (each constituent CEM report and unresolved obligation, or the refusal
naming a missing seal). Each section keeps its first `BATCH_REVIEW_LINES` lines (default 200) and
counts the rest, and each corvint command runs under `BATCH_REVIEW_TIMEOUT` seconds (default 300)
with its exit status printed, so a refusal, timeout or missing corvint is visible instead of an
empty section. Impact and batch-check need a clean worktree; on a dirty one they print their
refusal. The reviewer reviews the named head; a later push needs a new packet.

Report, after the batch merges (count of cancelled `pull_request` runs on the batch branch):

```sh
gh run list --repo beamfall/corvint --workflow ci.yml --branch <batch-branch> --event pull_request \
  --limit 200 --json conclusion --jq '[.[] | select(.conclusion == "cancelled")] | length'
```

Retain the per-batch counts in the batch's build-log entry. Target: at most 1 cancelled run per
batch on average over 10 batches; the measurement under this procedure is `NOT_PRODUCED` until 10
batches have run.

## 9. Adversarial refusal pass before review (ASD-V0-052)

On 2026-10-29, 3 of 3 Codex reviews of lane work found false admissions: a refusal or uncertainty
case that the lane's tests never exercised (V1-3135). Each repair cost about 25 to 30 minutes of
rebind and reseal on top of the lane's first pass. A lane that adds or changes an admission, edge
or refusal rule (a parser, a reachability or selection rule, an evidence filter, a guard that
narrows work) therefore runs this pass before closeout. The orchestrator copies the block below
into every such charter verbatim:

```text
Adversarial refusal pass (docs/LANE-ORCHESTRATION.md section 9). Before closeout, for each new or
changed admission/edge/refusal rule, write a case list in the lane build-log entry with one row
per category: malformed input; aliasing (a second name for the same value, path or ref);
spread/variadic arguments; quoting and escaping; historical (base/commit) reads versus HEAD or
worktree reads; unobserved state (absent, unreadable or unindexed input must yield UNKNOWN or
refusal, never CLEAN); concurrency overlap (two lanes, runs or writers on the same input). Each
row names a test that fails without the fix, or says why the category cannot apply. Closeout
step: cite the case list (entry path and section) in the lane report; a rule change with no case
list is not ready for review.
```

The reviewer prompt carries one clause for such a lane: "For each rule the change adds, check the
lane's adversarial case list (docs/LANE-ORCHESTRATION.md section 9): is every category covered by
a test that fails without the fix or justified as not applicable? Report each missing or
untested category as a finding." A case list does not replace review; it moves the cheap refusal
cases ahead of it.

Measurement: over the next 6 lanes that carry the pass, the share of reviews returning CONCERNS
against the 3 of 3 baseline and the repair rounds per lane, retained in the batch build-log entry.
It is `NOT_PRODUCED` until those 6 lanes have run.
