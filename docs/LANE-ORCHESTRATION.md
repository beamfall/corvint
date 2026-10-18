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
`HEAVY_LOCK_DIR` (default `/tmp/corvint-heavy-<uid>`), released by the kernel when the holder exits
for any reason, so a killed lane never strands a slot. Every lane on a host must use the same lock
directory and slot count. Rules: one package per command, a narrow `-run` for `./cmd/corvint`, no
`go test ./...` and no `make gate` in a lane (AGENTS.md "Verify"), and `make dogfood-finish` /
`make dogfood-batch-check` (which build two verifiers) also run through the wrapper.

The light-weight prototype this replaces (`mkdir` slots with a PID file and dead-owner reclaim)
could remove a live owner's slot when two waiters reclaimed the same dead slot at once; the
`flock` form has no reclaim step. Cut over all lanes of a host together, because the two forms do
not see each other's slots. `make lane-scripts-test` runs the serialisation check (two concurrent
commands with `HEAVY_SLOTS=1` never overlap) and the dead-owner check.

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
(`dogfood-finish-lane: REFUSE missing-inputs CITATIONS BASE:not-full-sha VERIFY ...`). At most two
finish attempts are allowed; a second failure stops the lane with the exact command and output in
its report. Then one `make dogfood-batch-check BASE=<origin/main tip>` from a clean checkout.
Rerunning `dogfood-change` to "see where it stands" between edits is not part of the loop.

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

## 5. Idle-lane watchdog (ASD-V0-017)

At every heartbeat (at most 10 minutes apart) the orchestrator runs

```sh
script/lane-idle-check.sh --idle-minutes 30 <each running lane's transcript or output file>
```

which prints `LANE ACTIVE|IDLE idle=<m>m last=<UTC> <path>` from the newest transcript
`"timestamp"` (or the file's modification time) and exits 1 when any lane is idle. It is read-only.
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
- Scratch logs go outside the checkout (untracked files make dogfood report
  `record-index-failed`).
- After a lane's PR merges, the orchestrator fetches the shared repository and runs
  `script/lane-clean.sh --root <lanes-root> --repo <shared> --active <dir>...` (dry run), reviews
  the `WOULD-REMOVE` lines, then reruns with `--apply`. It removes only checkouts whose `HEAD` and
  local branches are ancestors of `origin/main`, with a clean status, no stash and not listed as
  active; worktrees are removed with `git worktree remove`. Its `SUMMARY` line carries the counts
  and free disk before and after. Go build-cache trimming is separate (V1-0360).

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
