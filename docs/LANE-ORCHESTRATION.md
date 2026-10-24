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
Every dogfood make target requires an explicit BASE: the Makefile's `BASE ?= main` default
(local `main`, which may not be `origin/main`) is refused, not used (`ASD-V0-019`).

- Windows vet before push (ASD-V0-023, V1-1725): install the pre-push hook once in the shared lane
  repository (its hooks serve every linked worktree):
  `printf '#!/bin/sh\nexec script/pre-push-windows-vet.sh "$@"\n' > "$(git rev-parse --git-path hooks)/pre-push"`
  then `chmod +x` that file. Git runs it from the pushing checkout's top level, so each lane uses
  its own copy. It runs `make windows-vet` when the pushed commits change a `.go`, `go.mod` or
  `go.sum` file and refuses the push when it fails.

## 4. No polling in lanes; long commands are bounded (ASD-V0-016)

Lanes never run `sleep`, `until`/`while` wait loops, `gh pr checks --watch`, `gh run watch` or
background jobs. A lane pushes once, opens the PR, arms auto-merge and reports; CI results,
merge-queue progress and post-merge steps are the orchestrator's, which reads PR state on demand
at its heartbeat instead of waiting on events. A tool call has a 600 s limit; these commands are
known to approach or exceed it and are bounded as listed:

| Command | Observed time and phases | Bound in a lane |
|---|---|---|
| `go test ./cmd/corvint` (whole package) | about 591 s on a quiet host (decision 0082); far more under load | never whole-package: narrow `-run`, through `heavy.sh` |
| `go test ./...`, `make gate` | tens of minutes | not run in lanes (AGENTS.md "Verify"); orchestrator-owned when policy needs it |
| `make dogfood-finish` | build tree verifier, build base verifier, prepare, bind, commit, rebind (the full change steps), check, seal; each lane reports its own phase timings in its PR body | once, through `heavy.sh`; if it would pass 600 s, stop after the bind commit and hand the exact command to the orchestrator |
| `make dogfood-batch-check` | builds the verifiers, then re-verifies every lane in the range | once per lane at the end; per batch it is the orchestrator's |
| `corvint cem verify --guarding-tests` | runs the change's tests for up to 1200 s | not run in lanes |
| CI and merge-queue waits | minutes to hours | orchestrator only |

The rule is enforced by `script/lane-guard.sh` (ASD-V0-024, V1-1953), a PreToolUse(Bash) hook the
operator installs in Claude Code settings with `LANE_GUARD_ROOTS=<lanes-root>` (and, once lanes use
the repository semaphore, `LANE_GUARD_HEAVY=script/heavy.sh`). It denies polling loops and `go test`
without `heavy.sh`, and ignores heredoc bodies, so writing a file that contains a loop is allowed.

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
  (`docs/build-log/2026-10-18-v1-1627-lane-orchestration.md`).
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
- Go build caches (ASD-V0-021, V1-0360): after each batch merge, and whenever free disk falls below
  about 15%, run `make go-cache-trim` (dry run), review the `WOULD-TRIM` and `WOULD-REMOVE` lines,
  then `make go-cache-trim APPLY=1`. It trims each Go cache to 20 GiB, oldest entries first and
  never one used in the last 2 hours, and removes `go-build*` work directories older than 24 hours
  that no running process names. A trimmed entry is only a later cache miss.
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

These are measured over the next batch run under this procedure and retained in a build-log
entry; until then they are `NOT_PRODUCED`:

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

- merges every constituent branch into the batch branch locally, runs the batch's local doc gates,
  and pushes once;
- after a CI failure, fixes every failure the run reported, then pushes once more (one push per
  failure round);
- never pushes a partial batch to "start CI early"; a constituent that arrives late joins the next
  failure-round push or the next batch.

Report, after the batch merges (count of cancelled `pull_request` runs on the batch branch):

```sh
gh run list --repo beamfall/corvint --workflow ci.yml --branch <batch-branch> --event pull_request \
  --limit 200 --json conclusion --jq '[.[] | select(.conclusion == "cancelled")] | length'
```

Retain the per-batch counts in the batch's build-log entry. Target: at most 1 cancelled run per
batch on average over 10 batches; the measurement under this procedure is `NOT_PRODUCED` until 10
batches have run.
