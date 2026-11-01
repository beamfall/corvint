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
writes no Tasks store state: it reports ticket state as `Corvint-Ticket` commit trailers from
`corvint-tasks work start|deliver|block` (read-only; [TASKS-EXTERNAL-AGENTS](TASKS-EXTERNAL-AGENTS.md)
"Report lane work with commit trailers"), and the orchestrator ingests them with `work
close-merged` after the merge (TAW-V0-017). A lane never edits `.taskman/` or `.github/`, never
starts background jobs or subagents, and never touches another lane's checkout.

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
script/lane-pressure.sh --level <current level>   # both: the table below applied to one sample
```

`script/lane-pressure.sh` (ASD-V0-936, V1-1627) reads those inputs and prints
`level=L load_per_cpu=X memory=M swap=S admit=N`. From the current level it applies the release rules,
so the orchestrator only checks that two heartbeats agree before changing the cap. An unreadable
sample exits 3 and keeps the level.

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

Every heavy target takes the same queue (ASD-V0-930, V1-3626). The `Makefile` prefixes the recipes
of `build`, `tasks-build`, `doc-gates`, `batch-precheck` and `dogfood-finish` with `HEAVY_ADMIT`
(default `script/heavy.sh --admit-only`; empty under `CI`; `HEAVY_ADMIT=` opts out). So `make
dogfood-closeout` is admitted through its precheck and finish steps, and `script/codex-review.sh`
runs `codex exec` through `CODEX_REVIEW_HEAVY --admit-only` (default the `heavy.sh` beside it).
`--admit-only` takes a slot without reading the command: no policy, no rewrite and no offload, so
a review prompt that mentions `go test` is never refused and `make build` never cross-builds. A
make dry run (`make -n`) takes no slot. The holder exports `HEAVY_HELD`. A
`heavy.sh` started inside an admitted command (batch-precheck's steps, the finish's tests) takes its
holder's token `slotN.inner` or any free slot, whichever frees first. It never waits for a second
slot of its own, so nesting cannot deadlock, and it skips the load gate (ASD-V0-931). Each local run
ends with `heavy.sh: done queue=Ns (load=Ns slot=Ns) run=Ns rc=N`, so queue wait and run time are
read apart (ASD-V0-935).

Go defaults (ASD-V0-938, ASD-V0-939). A `go test` with no `-timeout` gets `-timeout 30m`
(`HEAVY_GO_TEST_TIMEOUT`, `off` adds none), so a loaded host never trips Go's 600 s default. A
`sh -c 'go test ...'` wrapper is unwrapped when its string is a plain command, and refused with
`heavy.sh: REFUSE wrapped-compound ...` when it holds an operator or an expansion (`cd x && go
test`, a pipe, `$PKGS`). Run the go command directly instead:
`script/heavy.sh env K=V go test -run X ./pkg`.

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

The other way round, `script/heavy-groups.sh [--budget 480] PACKAGE...` packs several small
packages into as few commands as fit the budget (ASD-V0-832). It reads each package's latest
completed whole-package `run=` seconds from the heavy.sh selection log and prints one group per
line, to run as one `script/heavy.sh go test ... GROUP` each. A package with no recorded cost, or
one over the budget alone, gets its own line and a stderr note; the latter needs `make test-shard`.

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
`GOFLAGS=` argument or the exported `GOFLAGS`) runs locally, because its files would stay remote; so does `HEAVY_REMOTE=0`.
On a Darwin lane host, a `go test` or `go vet` whose packages hold darwin-only files never offloads
(ASD-V0-934). It runs locally without `HEAVY_LOCAL_REASON`, after a
`heavy.sh: N darwin-only file(s), first F: running on this darwin host, not offloading` line and
`darwin-local` lines in the fallback and selection logs.
Checkout files a remote test writes, such as goldens regenerated with `-update`, are copied back
over a second ssh connection and named in `heavy.sh: copied back PATH` lines (DCW-V0-359); when a
local edit made during the run touches one of them, nothing is copied and the wrapper exits 96
asking for a local run. `script/heavy.sh --print-lock-dir` prints the slot lock directory a run
would use (DCW-V0-357).
When `corvint test --base SHA --dry-run --manifest M` selects packages whole (a dependency change
such as `internal/procgroup` selects about 200), `script/heavy.sh --plan M` prints one
`HEAVY_SELECTION=M script/heavy.sh go test ...` command per package, and `HEAVY_SELECTION=M` admits
each package M selects whole without a typed `HEAVY_WHOLE_REASON` (ASD-V0-891). A run missing both
`HEAVY_WHOLE_REASON` and `HEAVY_LOCAL_REASON` is refused once, naming both (ASD-V0-890).
Before choosing a focused `-run` pattern for a selected package, run
`corvint test --base SHA --dry-run --pins`: it prints one `pins go:PKG -run '^(...)$'` line per
selected package naming the tests whose bodies or helpers name a changed declaration or quote a
changed string literal (TSP-V0-944, V1-3808). Run those first; they are advice, so the dry-run
selection still decides what must pass before the lane finishes.

#### Several offload hosts (ASD-V0-933, V1-3626, V1-3518)

`HEAVY_REMOTE_HOSTS` lists several ssh destinations (whitespace or commas) and wins over
`HEAVY_REMOTE_HOST`. Put a user and port in the destination or in `~/.ssh/config`, never in the
script. Every other `HEAVY_REMOTE_*` setting applies to each host, and its `~` paths expand on that
host. So give each host the same layout: a clone at `HEAVY_REMOTE_CLONE` and the env file at
`HEAVY_REMOTE_ENV`. Both hosts must also take the same `HEAVY_REMOTE_SHELL`, so reach a WSL box
through its own sshd rather than through Windows. For the lane Mac with the WSL box and the Unraid
box:

```sh
HEAVY_REMOTE_HOSTS='ssh://russell@192.168.4.128:2222 ssh://agent@192.168.4.3:2222'
HEAVY_REMOTE_SHELL='bash -s'
HEAVY_REMOTE_CLONE=~/corvint
HEAVY_REMOTE_ENV=~/bench/go.env
```

Before each offload, the wrapper sends each host one probe ssh (bounded by
`HEAVY_REMOTE_PROBE_TIMEOUT`, 15 s) for its 1-minute load and CPU count. It tries the hosts once
each in order of load per CPU, unreachable ones last, and prints
`heavy.sh: remote hosts by load per CPU: H:L/C,...; trying H first`. An unavailable host passes the
run to the next. The retry and the logged local fallback below apply only when every host was
unavailable. A host short of disk is unavailable too (ASD-V0-937). The remote half refuses with
`low-disk PATH free=NG floor=MG` when its per-run scratch or its Go cache has less than
`HEAVY_MIN_FREE_GB` (3) GiB free, and the probe orders such a host after the others
(`H:lowdisk:NG`). The remote's Go cache and `GOTMPDIR` default to its disk, not a tmpfs `/tmp`
(V1-2691). A `/tmp/corvint-go-build-cache` left by older runs is not moved; trim it with
`make go-cache-trim`. An empty `HEAVY_REMOTE_HOST` in the environment keeps one run local even
with `HEAVY_REMOTE_HOSTS` configured. A build tries the first host only. With one host there is no probe. A host serves as a
target once it has a clone of origin, the pinned Go (in `~/sdk/go<version>` or through the env
file), `perl`, `git` and `timeout(1)`, and accepts the lane host's key.

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
appends `local wait=Ns run=Ns rc=N loadwait=Ns load=L queue=Ns` after it exits (`wait` is the slot
wait, `loadwait` the load-gate wait, `load` the starting 1-minute load (ASD-V0-970), `queue` both
waits (ASD-V0-935)), and a load or slot wait prints `heavy.sh: queued for ...` when it starts; a
fallback appends `fallback host=H`; a multi-host choice appends `pick hosts=... chose=H probe=Ns`. When
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
`HEAVY_LOAD_WAIT` (120 s) while the 1-minute load exceeds the threshold: `HEAVY_MAX_LOAD` when set,
else `floor(HEAVY_LOAD_PER_CPU x CPUs)` with `HEAVY_LOAD_PER_CPU` defaulting to 1.0, so 12 on a
12-core host (ASD-V0-932). The wait line names the source. The lane
cutover below keeps the existing selection and fallback log paths so the canary counts keep
reading the same files.

Declared host differences (V1-2563): the reference box's `/tmp` is a 16 GB tmpfs, but its CI
runners (`corvint-box-1..8`) set `TMPDIR=/var/tmp/corvint-ci/tmp/<runner>` (ext4), the offload env
file sets `TMPDIR` on the home disk, and offloaded commands get the per-run directory above, so
none of them writes test temporaries to tmpfs. The hosted `ubuntu-24.04` runner's temporary
filesystem has not been observed; a test that depends on filesystem type (rename atomicity,
`O_DIRECT`, inode reuse, free space) must not assume either.

Observation procedure (V1-2563 AC1). On a Linux host, `stat -f -c %T "${TMPDIR:-/tmp}"` prints the
temporary filesystem type (`tmpfs`, `ext2/ext3` for ext4) and `findmnt -T "${TMPDIR:-/tmp}"` the
mount; on macOS use `df "${TMPDIR:-/tmp}"` and `mount` (`stat -f` there means something else, and
`findmnt` is absent); a hosted runner needs one step that prints both for `$RUNNER_TEMP`, and that is a
`.github/` change for the orchestrator's own PR. NOT OBSERVED for the hosted runner as of this
entry; the box side stays the declared difference above. The box divergence is declared where lanes
and CI owners read it (AC2, second branch), so a box-only failure on a filesystem-dependent test is
first checked against the Linux command above before it is filed as a product bug.

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
# HEAVY_MAX_LOAD unset: the gate is 1.0 x CPUs (ASD-V0-932); the cutover's absolute 10 is retired
HEAVY_SELECTION_LOG=/private/tmp/cem10-build/heavy-selection.log
HEAVY_FALLBACK_LOG=/private/tmp/cem10-build/heavy-fallback.log
```

Then point the operator wrapper and the lane charter at `script/heavy.sh` and retire the prototype
and `rt.sh`. `gh` must be authenticated on the lane host; an unauthenticated `gh` fails every query
and so keeps every command local, visible as `query-failed` lines in the fallback log.

### One semaphore per host: `~/.config/corvint/heavy.env` (V1-3222)

Every host that runs lanes keeps exactly one heavy semaphore. `script/heavy.sh` reads its host
settings from `~/.config/corvint/heavy.env` (`HEAVY_CONFIG` names another file): at least
`HEAVY_SLOTS` (the host-wide slot count) and `HEAVY_LOCK_DIR` (the one directory every caller takes
its `flock` slots in), plus any other `HEAVY_*` key; the environment still wins. Any other wrapper
on the host delegates to `script/heavy.sh` rather than keeping slots of its own. On the WSL lane
host (2026-10-30) the file is

```sh
HEAVY_LOCK_DIR=/home/russell/bench/rt/.locks
HEAVY_SLOTS=4
HEAVY_SELECTION_LOG=/home/russell/lanes/heavy-selection.log
```

`~/bench/rt/.locks` is where `~/bench/rt.sh` (the remote half the Mac's offloads run) takes its four
`flock` slots, so local lanes and offloads now queue on one semaphore instead of three
(`~/lanes/heavy.sh`'s `mkdir` slots, 4; `script/heavy.sh`'s `/tmp/corvint-heavy-1000`, 8; and
`rt.sh`'s). `~/lanes/heavy.sh` now delegates to the checkout's `script/heavy.sh` and refuses outside
a checkout whose script reads `heavy.env`; the old wrapper is kept as
`~/lanes/heavy.sh.bak-20261030-v1-3222`. Change the file between batches: a running command keeps
the slot it holds, and only later commands see a new lock directory. `script/heavy.sh
--print-lock-dir` prints the directory a run would use.

Host check (V1-3605). `~/lanes/heavy.sh` delegates only when the lane's checkout carries a
`script/heavy.sh` that knows `--print-lock-dir`; for any older checkout (and outside a checkout) it
runs the old prototype, kept as `~/lanes/heavy.sh.bak-*-prototype`, which keeps its own `mkdir`
slots and writes none of the `script/heavy.sh` log lines (the host line of ASD-V0-920
work of lane scripts-w5 never prints from it). A lane on an older base therefore sees the prototype
even after the wrapper was updated. Check from the lane worktree: `~/lanes/heavy.sh --print-lock-dir`
prints the lock directory when the delegated script runs, and fails or runs a command when it is the
prototype.
On the Unraid host the delegating wrapper was installed at 2026-11-01T01:16Z (`~/lanes/heavy.sh`,
with the prototype kept as `~/lanes/heavy.sh.bak-20261101-prototype`, last changed 2026-10-31
18:36Z), so lane scripts-w5 on 2026-10-31 most likely ran the prototype itself; a checkout whose
base predates `--print-lock-dir` still gets it. Observed 2026-11-01 from the lane spectool-r
worktree: each run printed `heavy.sh: script sha256=... <worktree>/script/heavy.sh` and took a
slot of `HEAVY_LOCK_DIR=/home/agent/lanes/.flock-slots` from `~/.config/corvint/heavy.env`
(`HEAVY_SLOTS=2`), so it delegated; the host line of the unmerged ASD-V0-920 work does not print
because that work is not on this base. The
orchestrator refreshes the lane base, or the host wrapper, between batches; no lane changes a host.
A lane's focused checks include `make line-citations-selects BASE=<base>` (`DCG-V0-921`, V1-3323):
when it names a pinned citation into a file the change touched, run `make line-citations-check`
then, not only at doc-gates.

A lane host that offloads (the Mac lane host, for example) sets `HEAVY_REMOTE_HOST` in this file, not
in a lane charter or a session's environment: a worktree session started from the standard charter
then offloads with no export (V1-3518). Every run prints `heavy.sh: host=H why=W` first
(`ASD-V0-920`), so a lane whose runs stay local reads, for example, `host=local why=HEAVY_REMOTE_HOST
unset in the environment and in config /Users/x/.config/corvint/heavy.env (absent)`.

### Small packages run whole without a reason (ASD-V0-074, V1-3120)

`corvint test --dry-run` plans a whole Go package for a small tool package, which `ASD-V0-030`
would refuse without `HEAVY_WHOLE_REASON`. A whole-package `go test` whose every package is a plain
`./tools/...` or `./conformance/...` directory (no `...`) with at most `HEAVY_SMALL_TEST_BYTES`
(default 49152) bytes of `*_test.go` runs without one and is logged `whole reason=small-package:NB`;
on 2026-10-30 the six such packages each ran whole in 0.6 to 16 s. `cmd/` and `internal/` packages
still need the reason.

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
- `CITATIONS`: the citation plan file (distinct spans, anchored on BASE lines the diff does not edit;
  a `drafted proposal` row citing the hunk's own file is confirmed or replaced, `DCW-V0-962`);
- `INTENTS`: the governing spec paths, each a spec Markdown file with one `## Requirements`
  heading, or the one line `#no-intent-declared`; a `docs/build-log/*.md` entry or
  `docs/decisions/*.md` file is an intent only of a record change, one that touches nothing but
  those records and CEM sidecars (`DCW-V0-058`, `DCW-V0-817`); `VERIFY`: one shell-free command per
  line;
- `OUTCOME`: `passed`, `failed` or `blocked`.

A reseal, which binds commits a lane made after its own earlier seal, takes the previous seal
head (the last seal commit on `HEAD`'s first-parent path) as `BASE`, not the merge base: the range
then holds only the new commits, so the plan stays inside the 256-row bound, and
`dogfood batch-check` verifies each chained seal against its own base (docs/DOGFOOD.md section 4,
"Resealing after a seal").

A charter that hands the lane a requirement-ID block checks each block first with
`make requirement-ids-free PREFIX=<prefix> RANGE=<lo>..<hi>` (exit 0 and `free ...` when no tracked
file on `origin/main` names any ID in it; exit 1 with one `used ID PATH:LINE` line per taken ID;
`ASD-V0-960`). The new IDs go on their own definition lines inside the spec's `## Requirements`
section, never into the frozen header `Intent status:` or Agent digest `Status:` line, which
`make spec-requirements` refuses (`SRG-V0-980`, `SRG-V0-890`), nor after that section
(`SRG-V0-891`).
`ASD-V0-960`); with a `RANGE` the same target also checks every unmerged local branch and worktree
and names each ID one of them defines as `used ID NAME (SHA) FILE` (`ASD-V0-952`). The charter tells
the lane to run `make requirement-id-collisions` after staging a spec that adds an ID and before its
final commit, and the orchestrator runs it in the batch checkout before integrating each lane
(`ASD-V0-950`, `ASD-V0-951`).

A charter that lets a lane add divergence-register entries hands it a DR-number block the same way,
because each lane would otherwise take the next free `DR-` number from its own base. The batch
precheck's `dr-numbers` step (`make dr-numbers-check`, `script/check-dr-numbers.sh`, `ASD-V0-642`)
fails when the merged `conformance/divergence-register.md` defines one `DR-` heading number twice,
naming every defining line; renumber the later entry before the batch push.

Before any lane starts, the orchestrator screens the batch's ticket files with
`make lane-dispatch-check BASE=<sha> LANES="lane-a=tickets-a.json lane-b=tickets-b.json"`
(`script/lane-dispatch-check.sh`, `ASD-V0-645`). Each file is one lane's JSON ticket array. The
check fails (exit 1) on `EMPTY`, a ticket without `record.effects.touchPaths`; `DUPLICATE`, one
ticket in two lanes; `OVERLAP`, two lanes whose tickets name the same path or a directory key (ending
in `/`) that holds the other's path; and, with `BASE`, `ABSENT`, a path not at the lane base. `WAIVE=V1-N`
(space separated) waives `EMPTY` and `ABSENT` for a ticket that creates its paths; an overlap is
never waived: serialize the tickets or dispatch them to one lane. A malformed file or a base that is
not a commit exits 2.

A charter also tells the lane where a new requirement range goes: on each requirement's own
definition line (its leading note), in the spec's acceptance and traceability rows and in the
build-log entry. A spec's header `Intent status:` and its Agent digest `- Status:` line are frozen,
so `make spec-requirements` refuses a new ID or range named there, naming that line as `path:line`
(`SRG-V0-980`, `SRG-V0-950`).

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
Every dogfood make target requires an explicit BASE: the Makefile's `BASE ?= origin/main` default
(a moving ref, not a pinned sha) is refused, not used (`ASD-V0-019`, `ACX-V0-857`).

- Windows vet before push (ASD-V0-023, V1-1725): install the pre-push hook once in the shared lane
  repository (its hooks serve every linked worktree):
  `printf '#!/bin/sh\nexec script/pre-push-windows-vet.sh "$@"\n' > "$(git rev-parse --git-path hooks)/pre-push"`
  then `chmod +x` that file. Git runs it from the pushing checkout's top level, so each lane uses
  its own copy. It runs `make windows-vet` when the pushed commits change a `.go`, `go.mod` or
  `go.sum` file and refuses the push when it fails.
- Windows main before vet (ASD-V0-974, V1-3782): a new untagged file (for example `help.go`) in a
  package whose `main.go` carries `//go:build darwin || linux` builds on Windows with no `main`, and
  `make windows-vet` reports it only as `function main is undeclared in the main package`. Give
  the new file the same `//go:build` line, or add a windows main. `make windows-main-check` finds
  it in seconds; run it when you add a file to a build-tagged main package. It is not a step of
  `make windows-vet`, whose recipe the `AFP-V0-054` parity test pins to the CI vet commands.
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

Closeout, check and seal run in the foreground (ASD-V0-107, V1-3224). A lane that starts
`dogfood-closeout`, `dogfood-finish`, `dogfood-check` or `dogfood-seal` as a background job and
ends its turn to wait for it parks: the job's completion does not reliably resume the session, so
the lane sits idle with its seal unfinished until the orchestrator notices. Run each such command
in the foreground and, when one would pass the 600 s tool-call limit, split it at its own phase
boundary (stop after the bind commit, as the table says) instead of backgrounding it. A lane never
ends a turn with its own job still running: it finishes the job, or it reports the exact remaining
command as its handoff and ends with nothing running. `script/lane-guard.sh` enforces the first half for
every session, subagent or not: a backgrounded `make dogfood-closeout`, `-finish`, `-check` or
`-seal`, `script/dogfood-finish.sh` or `corvint dogfood finish-lane`/`finish`/`check`/`seal` is
denied with this foreground recipe.

The orchestrator's landing step is `script/pr-landing-watch.sh [--act] PR` (ASD-V0-833, V1-1735),
run once per CI or merge-queue event, never in a loop. It prints one decision: a failed head run
whose failing Go tests all match `script/known-flakes.txt` is rerun (at most 3 times per head
commit); auto-merge turned off by a bot's merge-queue removal is re-armed (at most 2 times); a
real failure, an exhausted cap or a human dequeue prints `ESCALATE` and appends one line to the
alerts file that the session's Monitor tails. Without `--act` it only reports.

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
  `script/lane-setup.sh --repo <shared> --dir <lanes-root>/<slug> --branch claude/<slug>`, which runs
  `git -C <shared> worktree add -b claude/<slug> <lanes-root>/<slug> origin/main` and installs the
  AI-trailer commit-msg hook below.
- The shared repository holds the full history (ASD-V0-072, V1-3230). A shallow clone breaks
  merge-base, `corvint affected` and every dogfood range, so `lane-setup.sh`, `corvint affected
  --base`, `dogfood-change`, `dogfood-check` (and so `dogfood-seal`), `dogfood-bind-range`,
  `batch-review-packet` and `batch-precheck --base` refuse one up front with `REFUSE
  shallow-repository` and the recovery `git -C <shared> fetch --unshallow origin` (or a reclone
  without `--depth`).
- No commit carries an AI co-author or "Generated with" trailer (ASD-V0-560, ASD-V0-831, V1-3227).
  The commit-msg hook `lane-setup.sh` installs (`script/check-ai-coauthor-trailers.sh
  --install-hook`, shared by every worktree of the clone) refuses one at commit time, and
  `make batch-precheck` refuses any commit in `BASE..HEAD` (default `origin/main..HEAD`) that
  carries one, naming the commit and the line, and prints the `--strip` rebase that removes it. The dogfood
  loop supports linked worktrees with per-worktree trace stores and locks (`DCW-V0-044`); never
  switch one checkout between lanes. A fresh plain clone remains the fallback until a live lane has
  run `dogfood-finish` and `dogfood-batch-check` in a linked worktree
  (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-18-v1-1627-lane-orchestration.md`).
- Lanes share `rr-cache`, `MERGE_RR` and its lock through the common directory (ASD-V0-965,
  V1-3651). `lane-setup.sh` sets `rerere.enabled=false` in each lane worktree's own config
  (`extensions.worktreeConfig`), so no lane records or replays another lane's resolution. A git
  command that may land commits (merge, commit, cherry-pick, rebase) can still fail on a transient
  lock after its commits landed: record HEAD before it, compare after a failure, and retry only when
  HEAD did not move. When HEAD moved, verify the landed commits (`git log --oneline BEFORE..HEAD`)
  instead of repeating the command, which would land them twice.
- Never run `git stash` in a lane. The stash is one ref (`refs/stash`) in the clone's common
  directory, shared by every linked worktree, so one lane's `git stash pop` can apply another
  lane's stash (incident, 2026-10-21: the dogfood-flow lane's `git stash pop` applied the affected-selection lane's stash `723eec1`, V1-2161). Park work in a
  patch file kept outside the checkout (`git diff > ../p.patch`, then `git apply -R ../p.patch`;
  restore with `git apply ../p.patch`) or in a named WIP commit on the lane's own branch.
  `script/lane-clean.sh` keeps any checkout that still has a stash entry.
  A lane charter template carries this line verbatim (V1-3767; a charter that omitted it let a
  lane stash in a shared-object worktree): "Never run `git stash` in a lane: the stash is shared
  by every worktree of the clone. Park work in a named WIP commit on the lane branch, a patch file
  outside the checkout, or a scratch `git worktree add` off the lane branch."
- Scratch logs go outside the checkout (untracked files make dogfood report
  `record-index-failed`), into a directory only this lane writes (ASD-V0-108, V1-3156):
  `"$(git rev-parse --absolute-git-dir)/corvint/lane-logs/"`, unique per worktree like
  `lane-inputs/` (section 3), or a `mktemp -d` directory. Never a fixed name in the shared
  temporary directory such as `/tmp/closeout.log` or `/tmp/precheck.log`: on macOS `/tmp` is
  `/private/tmp`, one directory for every lane on the host, so two lanes running the same step
  overwrite each other's log and a report can quote another lane's output.
- Helper binaries build into scratch too: `go build -o "$TMPDIR/NAME" ./cmd/NAME`, never a bare
  `go build ./cmd/NAME`, which leaves an untracked `NAME` at the worktree root that the recorder
  counts as dirty (V1-3132). `dogfood-change` names such a file as
  `NOTE untracked-build-output` (`DCW-V0-386`); delete it before the next dogfood step.
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
  (`${TMPDIR:-/tmp}/corvint-go-build-cache`, ASD-V0-914), which `script/heavy.sh` also gives a local
  run unless the caller set `GOCACHE`; on an offload host it is the remote env file's `CORVINT_GOCACHE`
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
  lane transcript; target median at most 2. On each lane host, before `lane-clean.sh` removes the
  worktrees, also run `make dogfood-change-runs SINCE=<batch start, UTC>` (ASD-V0-955): it reads the
  repository-wide ledger every `dogfood-change.sh` run appends to, which survives
  `git worktree remove`, and prints one `run-count BRANCH N` line per lane plus the median. Retain
  its output in the batch build-log entry; a host whose ledger has no row reads `NOT_OBSERVED`.
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
- relies on the precheck's `make lane-scripts-test` step (ASD-V0-640, V1-3744) to run the lane
  shell tests on the merged tree, where two lanes' fixtures first meet: it runs alone after the
  parallel steps, and under `BASE` only when a `script/`, `Makefile` or `docs/DOGFOOD.md` path
  changed (a print-only skip row says so otherwise);
- runs `go vet` through `script/heavy.sh` like any heavy Go command, from the batch worktree:
  `set -o pipefail; script/heavy.sh env GOTOOLCHAIN=local go vet ./... 2>&1 | tail -n 40` for the
  root module, and `(cd interop/cem01-go && ../../script/heavy.sh env GOTOOLCHAIN=local go vet ./...)`
  for the interop module (heavy.sh runs it in the same subdirectory of its tree);
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

Fail-without-fix runs (`ASD-V0-906`, V1-3259). Run the new test against the unfixed code with
`-timeout 2m` or less, for example `script/heavy.sh env GOTOOLCHAIN=local go test -count=1
-timeout 2m -run '^TestName$' ./internal/pkg`, never the 30-minute hang bound: unfixed code may
block instead of failing, and a blocked run then holds the tool's whole 10-minute budget. A test
that proves a file is read once through a FIFO must not block on the unfixed code's second read:
the writer's first open is a plain `O_WRONLY` open, which waits for the reader and writes the
content; every later open is `O_WRONLY|syscall.O_NONBLOCK` in a loop, closed at once, until the
test closes a stop channel. A second read by unfixed code then gets end of file and the test fails
fast (`internal/dogfoodflow/verify_quote_fix_unix_test.go` is the pattern).

Measurement: over the next 6 lanes that carry the pass, the share of reviews returning CONCERNS
against the 3 of 3 baseline and the repair rounds per lane, retained in the batch build-log entry.
It is `NOT_PRODUCED` until those 6 lanes have run.

## 10. Codex review login preflight (ASD-V0-064)

On 2026-10-30 a lane's `codex exec -m gpt-6.1-sol -s read-only` review retried "401 Not logged
in" several times before it became `NOT_PRODUCED` (V1-3219). Lane setup runs the check once:

```sh
script/codex-review.sh --preflight   # codex-review: OK logged-in, or NOT_AVAILABLE and exit 3
```

and the lane's review goes through the same script, which checks `codex login status` once more
and then runs `codex exec -m gpt-6.1-sol -s read-only` with the lane's arguments
(`CODEX_REVIEW_MODEL` and `CODEX_BIN` override the model and executable). A missing CLI or a
logged-out one prints one `codex-review: NOT_AVAILABLE` line with the fix (`codex login`, or
`codex login --with-api-key` on a headless host) and exits 3 without any review call; the lane
records the review as `NOT_RUN` with that line instead of retrying.

## 11. Friction goes up to the orchestrator (ASD-V0-076)

Lane friction (stale binaries, missing logins, requirement ID collisions, parked background jobs)
is the main input to process repairs, and most lane hosts cannot file tickets: the WSL and Unraid
lanes have no current Tasks store. A side session's relayed report is cut to its first 400
characters, so a Friction section only in the final report never reaches the orchestrator; on
2026-10-30 the owner restated this three times. The orchestrator copies the block below into
every lane charter verbatim, together with a digest (ID and title) of the open friction tickets
from `corvint-tasks ticket search --status OPEN --label agent-memory`, and puts every merge
instruction (for example, a lane to merge as the one pre-dogfood merge) in the charter before
dispatch:

```text
Friction report (docs/LANE-ORCHESTRATION.md section 11). Before closeout and before wrap_up,
SendMessage the full numbered friction list to the orchestrator session: an item already in the
charter's digest as one line `+1 V1-NNNN <context>`; a new item with its repro, host and tool
version, expected versus actual, and whether it is suspected or confirmed; "none" when there was
none. Friction found mid-task may be sent as soon as it is found. A Friction section only in the
final report does not count. Do not file tickets yourself.
```

An in-process subagent's charter (the Agent tool) requires the same Friction section in its
result instead. The orchestrator reads every list in the round it arrives, dedupes it against
`corvint-tasks ticket search --text`, files the new items (labels `agent-memory` plus the
category) or adds the `+1` context to the existing ticket, and asks a lane whose report arrived
cut off to resend its list by SendMessage.

## 12. Friction dedupe and lanes without a journal (TCL-V0-001 to TCL-V0-012)

A lane report ends with a Friction section, and before 2026-10-30 the orchestrator deduplicated
each item by hand: that day's round of 27 items held 13 duplicates of open tickets, each found by
repeated `ticket search` calls. The duplicate check in
[Corvint Tasks CLI V0](specs/corvint-tasks-cli-v0.md) replaces those searches.

Charter. The orchestrator pastes a recent digest into each charter and gives a remote lane the
full digest file:

```sh
corvint-tasks ticket digest --since 2026-10-23 --format text > digest.txt   # paste into the brief
corvint-tasks ticket digest --format text > digest-full.txt                 # every OPEN agent-memory ticket
```

The full digest of every open `agent-memory` ticket was 1,070 lines (120 KB) on 2026-10-30, too
large for a brief; a one-week `--since` window was 263 lines (33 KB). The digest header names the
intent tree digest it was printed from.

Lane. For each friction item, a lane matches the item against the digest, with no store and no
journal:

```sh
corvint-tasks ticket similar --digest digest.txt --text 'heavy.sh ignores unstaged files'
```

An item that matches an open ticket is reported as `+1 V1-NNNN <one-line context>`; only an item
with no match is written out in full. The digest holds titles only, so the offline match is weaker
than the store's (11 of 13 still-open duplicates at rank 3 or better in the 2026-10-30 round,
against 14 of 15 for the store); a lane that is unsure reports both the item and its best match.

Orchestrator. A `+1` line is recorded on the existing ticket, never as a new one:

```sh
corvint-tasks ticket recur V1-NNNN --context 'lane w4: <context>' --request-id <lane>-friction-<n>
```

Each new item takes one `ticket similar --text` call. `ticket create` runs the same check and
refuses a likely duplicate with `DUPLICATE_ID`, naming the candidates; file it anyway with
`--not-duplicate-of V1-NNNN` (repeatable) once the candidates are confirmed different.

Lanes without a journal. A lane host whose checkout has no Tasks journal (for example Unraid)
reads the queue from a snapshot the orchestrator exports on the owning host:

```sh
corvint-tasks queue snapshot --out queue-snapshot.json                      # on the owning host
CORVINT_TASKS_SNAPSHOT=queue-snapshot.json corvint-tasks ticket show V1-3223 # on the lane host
```

Under `CORVINT_TASKS_SNAPSHOT` only `ticket search`, `ticket show`, `ticket export`,
`ticket similar`, `ticket digest`, `roadmap` and `queue status` run. Every file and the tree digest
are verified first, and every result names the snapshot's host, primary worktree, `producedAt` and
journal sequence. Any other verb, every write included, is refused `UNSUPPORTED` before any read,
naming the owning store and the command to run there.

## 13. Lane flow guide and refusal recovery map (V1-3239)

One lane runs this flow, in order; each step names the command and the section that owns it.

1. Orient: `corvint index --if-stale`, then `corvint affected --base FULL_SHA` and
   `corvint context --task "..."`; retain the `query` and `impact` receipts (DOGFOOD section 1).
   A wide or UNKNOWN selection means focused tests only.
2. Build in the lane's own worktree and branch; spec first when behavior changes (AGENTS.md "Spec-driven
   development"). Check requirement IDs with `make requirement-ids-free`.
3. Focused tests through `script/heavy.sh`, one package per command, `set -o pipefail` when piped
   (section 2).
4. Doc gates before the final commit: `make doc-gates`, `make line-citations-check`, `gofmt -l`.
   Repins are mechanical: `make repin` (line citations and the analyzer-schema pin) and
   `script/repin-use-case-receipts.sh` (use-case receipts); both stage what they rewrite.
   Repin per commit, not once at the end (V1-3821): before each per-ticket commit, stage the
   ticket's files, run `make line-citations-check`, and when it names a moved citation run
   `make repin BASE=<lane-base>` and commit the staged repins with that ticket. A later
   catch-up repin commit leaves every earlier commit red for anyone who checks it out or bisects.
5. Dogfood inputs: `corvint dogfood propose-citations BASE > draft.tsv`, review it into
   `CITATIONS`, write `INTENTS` and `VERIFY`, then `make dogfood-inputs-check BASE=.. CITATIONS=..
   INTENTS=.. VERIFY=.. OUTCOME=passed` (section 3). That target binds nothing.
   After the last commit, run `script/heavy.sh make dogfood-finish LANE=1 BASE=.. CITATIONS=..
   INTENTS=.. VERIFY=.. OUTCOME=..` in the foreground (ASD-V0-015). A charter may instead hand the
   finish to the orchestrator (review-before-seal): the lane then stops at the final commit plus a
   passing `dogfood-inputs-check` and reports the input file paths.
6. Report: head SHA, one line per ticket, tests and results, then the Friction list (section 11).

Common refusals and their recovery:

| Refusal | Recovery |
|---|---|
| `heavy.sh: REFUSED local-reason-missing` | run on the offload host, or set `HEAVY_LOCAL_REASON='darwin-only: <why>'` for a command that needs this OS |
| `heavy.sh: REFUSED whole-reason-missing` | use a selective `-run '<selected>'` from `corvint test --base SHA --dry-run`, or set `HEAVY_WHOLE_REASON` with the evidence (Corvint selected the package whole, or UNKNOWN/FALLBACK); `-short` does not clear it |
| `missing-inputs CITATIONS` (`make dogfood-inputs-check`) | pass `CITATIONS=`, `INTENTS=`, `VERIFY=` and `OUTCOME=` make variables (DOGFOOD "Inputs") |
| `reused-worktree-state` | `corvint migrate-traces --dry-run`, inspect it, then `corvint migrate-traces --apply --plan-digest DIGEST` with the digest the dry run printed; rerun |
| `unsupported-object-alternates` | `git repack -a -d`, delete `.git/objects/info/alternates`, rerun (DOGFOOD "Inputs") |
| `patch-unavailable reason=<code>` | make every changed path a regular file with a textual hunk, commit, rerun `propose-citations` |
| `citation-plan-uniform` | cite each hunk with the base lines it depends on, not one span for all |
| `bootstrap-intent-not-unknown` | give the intent spec this change creates no row, or an `unknown` row |
| line-citation or use-case receipt drift after an edit | `make repin`; `script/repin-use-case-receipts.sh`; commit the repins with the edit that moved the text (step 4) |
| `sealed-cem-in-change` | drop the seal from the diff, or for a batch of sealed leaves follow DOGFOOD "Integrating sealed branches" |
| `unarchived-base-cem` | run the `fix:` line it prints (commits the inherited archive), rerun with the same BASE |
| `RESOURCE_COLLISION`, `OUT_OF_SCOPE`, `DUPLICATE_ID` (Tasks) | orchestrator-owned; report the exact line, do not work around it |

Drift. `make flow-guide-check` (a doc gate, ASD-V0-996) checks this section and the repository
skill `.claude/skills/corvint-lane-flow/SKILL.md` against the real CLI: every `corvint`,
`corvint-tasks`, `make` and `script/` span must name an existing subcommand, flag, Makefile rule or
script, and every code in the table above must occur in source. A refusal the product renames
therefore fails the gate instead of misleading a lane.

New refusals (ASD-V0-997). The same check gates the refusal sites of `script/*.sh` (`refuse "CODE
..."`) and `internal/dogfoodflow` (`.fail` and `.refuse`): a site must name its recovery (a `fix:`
line, a valid form such as `want` or `usage:`, a `failRecoveries` default) or have its code in the
table above. The pairs that did not on 2026-11-01 are listed in
`tools/flow-guide-check/unhinted-refusals.tsv`, which only shrinks; a new unhinted code fails.
`go run ./tools/flow-guide-check --audit` prints the counts (ASD-V0-998). A `dogfood check` FAIL
without its own fix line now prints the code's default recovery.

Lane notes (V1-3610). A lane starts with no seeded sibling hand-over by design: its inputs are the
charter, the open-friction digest (section 12) and `~/lane-inputs`. The orchestrator names an
in-flight sibling and the files it owns in the charter's "other lanes in flight" paragraph, which is
the only sibling summary; a lane that needs a sibling's result asks the orchestrator and never reads
another lane's checkout (Roles above).

## 14. Remote (SSH) lane hosts: inputs and closeout channel

An SSH lane host (WSL, Unraid) has no `gh` login (Unraid) or no `gh` at all (WSL), no Tasks journal
and no route to the orchestrator session. SendMessage from there failed on 2026-10-31 with `session not visible from the Unraid SSH host`
(V1-3620); on 2026-11-01 it worked (see "Closeout channel" below). So dispatch to an SSH host stages, before the lane starts:

- `~/lane-inputs/ticket-bodies-<lane>/<ID>.json`: `corvint-tasks ticket show <ID>` JSON for every
  assigned ticket, exported on the owning host. A lane host may instead read a queue snapshot
  (`CORVINT_TASKS_SNAPSHOT`, section 12); without either, `ticket show` there refuses
  `journal-absent` (V1-3262). The `open-friction-*.txt` digest cuts titles at 120 characters, so it
  is not a ticket body.
- `~/lane-inputs/gh-issues-<lane>/<N>.md`: the body of every GitHub issue a ticket cites
  (`gh issue view N --repo beamfall/corvint --json title,body`, run on the Mac) (V1-3252). The route
  is staged files; the lane never needs `gh`.
- the charter, requirement-ID block and digest files, as for a local lane.

Closeout channel. A remote lane sends its final report and full numbered Friction list (section 11
format) to the orchestrator with SendMessage, `to` set to the orchestrator's `local_...` session id
from the lane brief. Observed 2026-11-01 from the Unraid lane spectool-r: the send returned
`Forwarded to Claude Desktop's session messaging: Message queued` (message 669b8253) while the
orchestrator was mid-turn, and the orchestrator acted on it, staging the requested ticket bodies as
`~/lanes/spectool-r-tickets.txt` at 03:23Z. The tool reports queued delivery only, not a read, so
the lane also keeps the file fallback: it writes the same report and list to
`~/lane-reports/<lane>/final.md` and `~/lane-reports/<lane>/friction.md` before wrap_up and names
both paths in its last message. The orchestrator fetches them with
`ssh HOST 'cat "$HOME/lane-reports/<lane>/final.md"'` and the same for `friction.md` (single-quoted
so the remote shell expands `$HOME`; or `scp`) when no message arrived in the round the lane ends.

## 15. In-lane subagents and test-writing rules (V1-3253, V1-3259)

Shared worktree. Subagents of one lane share the lane's worktree, so a package one subagent has
half-edited fails to build for every sibling's Go test. Rule: at most one subagent writes to a Go
package at a time; subagents that must touch the same package run one after the other, or each in
its own `git worktree add` off the lane branch and the lane merges the results. Read-only subagents
share freely. A lane charter template carries this line verbatim: "In-lane subagents never edit the
same package concurrently; a sibling's build failure in a package you did not edit is a sibling's
half-edit, so wait and rerun, do not repair it."

Fail-without-fix runs. A test run against the pre-fix code to prove it fails must carry
`-timeout 2m` at most (`script/heavy.sh go test -timeout 2m -run '^TestX$' ./pkg`), because a
test that blocks on the old code otherwise hangs for the full 10-minute tool timeout. A read-once
test that races a FIFO opens the writer non-blocking so a missing reader fails fast:
`syscall.Open(path, O_WRONLY|O_NONBLOCK, 0)` in a retry loop bounded by a deadline (ENXIO means no
reader yet), never a plain `os.OpenFile(path, O_WRONLY, 0)`, which blocks forever against old code.
