# Lane scripts, wave 2 remainder: V1-1630, V1-1627, V1-1628 and V1-2819 measurements — 2026-10-31

Lane `scripts` (wave 2), branch `claude/lane-scripts-w2-1031`, on the sealed head `02a632273`. The
orchestrator reopened four tickets whose acceptance criteria had no retained measurement on
origin/main. This entry records those measurements. Every Go run went through `script/heavy.sh`
with `HEAVY_REMOTE_HOST=wsl-lanes` (the WSL host: 32 cores, Go 1.27.3). The Mac host has 12 cores.

## Decisions

- V1-1630 (`ASD-V0-834`): `script/lane-transcript-metrics.py` now takes several project
  directories as one window. With `--sessions` it also reads top-level session transcripts. Wave-2
  lanes run as their own worktree sessions, and the old script could not see them. Each row names
  its kind, and the output reports `sessions` beside `subagents`.

## Evidence

- V1-2819 AC2 (met). 11 runs of
  `script/heavy.sh go test -count=1 -run TestMigrationRatchetCLI ./cmd/corvint` on `wsl-lanes`,
  from a warm slot.
  - Wall times: 5, 4, 4, 3, 4, 4, 3, 4, 3, 4 and 3 s.
  - The first 10 runs have a median of 4 s, against the 2026-10-27 baseline of 54 s. The test itself
    takes 0.1 s, so fixed overhead fell from about 54 s to about 4 s, a cut of over 90 %.
  - Phase line: the first run read `receive=0.5s prepare=0.4s run=1.8s`. Every later run read
    `receive=0.1s wait=0.0s prepare=0.1s run=1.1s`.
- V1-2819 AC3 (met: the cache stays bounded, V1-2691).
  - Every run used `worktree=heavy-slot1/reused`.
  - Before the V1-1628 runs below, the remote held one slot worktree:
    `~/.cache/corvint-heavy/wt/heavy-slot1`, 152 MB.
  - The Go build cache is one directory on the remote disk, `~/.cache/go-build`, 112 GB, never
    `/tmp`. The disk is 1007 GB with 27 % used.
  - The V1-1628 runs added slots 2 to 5. They are reused per slot and bounded by
    `HEAVY_REMOTE_SLOTS` (8).
- V1-1628 AC3 (gap recorded: 1.51x warm against the 1.5x target). Whole-package
  `go test -count=1 ./internal/liveverify/affected` on `wsl-lanes`.
  - Quiet host, 4 runs (load 0.3 before): wall times 48, 46, 48 and 46 s, median 47 s. Run phase
    44.7 to 47.1 s.
  - 5 concurrent lanes, one foreground `xargs -P 5` per round. Each lane took its own remote slot,
    and none waited for admission (`wait=0.0s`).
    - Round a, cold: 4 new slot worktrees, `prepare=1.8s`. Wall times 68, 75, 75, 75 and 76 s,
      median 75 s, which is 1.60x quiet. Load 1-minute average 7.85 at 35 s.
    - Round b, warm: wall times 69, 70, 71, 71 and 71 s, median 71 s, which is 1.51x quiet. Load
      8.01.
  - The remaining gap is about 1 % over the target. Each `go test` runs its package's tests in
    parallel, so 5 lanes compete for CPU even with free slots. The admission limit bounds how many
    commands run, not how much CPU each uses.
- V1-1627 AC3 (met). Before and after, by the same method:
  - Load: median of `load averages:` samples in lane tool output, Mac host.
  - Focused test duration: median single-package `go test` time from
    `script/lane-transcript-metrics.py`.

  | Batch | Load samples | Load median (p90) | Focused `go test` calls | Focused median |
  | --- | --- | --- | --- | --- |
  | Before: orchestrator session `03e2afac`, 2026-10-14 to 2026-10-17, no cap | 2864 | 57.6 (78.8) | 2710 | 21 s |
  | After: wave 2, 2026-10-31, cap applied | 72 | 11.4 (39.3) | 190 | 9 s |

  - The before figures reproduce the ticket's own audit: load median 59.95 over 2462 samples, 12
    cores.
  - The after window has fewer samples and a different package mix, so the comparison is indicative.
    The direction and size of the change are clear.
- V1-1630 AC3 (met on the reviewed count; the raw count sits exactly at the limit). Measured with
  the new script:
  `lane-transcript-metrics.py ~/.claude/projects/-Users-russelllewis-projects-corvint ~/.claude/projects/-Users-russelllewis-projects-corvint--claude-worktrees-* --sessions --since 2026-10-31 --until 2026-11-01`.
  - It read 46 subagents and 8 sessions, 28 of them lanes.
  - Polling: 0.223 h in 43 calls, under the 1 h target.
  - Harness timeouts: the script reports 10. That is the target's limit, which is "under 10", not
    below it.
    - One of the 10 is a false positive. The phrase "timed out at 1800s" appeared in a `cat` of a
      script, not in a harness timeout. That leaves 9.
    - One of the 9 is the orchestrator's own session. It counts as a lane because it commits.
    - Only 2 of the 9 were 600 s timeouts, both lane `go test` runs. The other 7 hit the default
      180 s limit.
  - Tool calls of 600 s or more: 2, `make test-shard` (601 s) and `go test ./cmd/corvint` (601 s).
  - AC2 (commands over 600 s): `cmd/corvint` is split by `make test-shard` (`ASD-V0-026`).
    `dogfood-closeout` records phase timings in its `timings.tsv`.
  - SSH-host lanes keep their transcripts on those hosts, so they are not counted.
- Tests: `make lane-scripts-test` passes, 37 groups, including the new `ASD-V0-834` group.

## NOT_RUN

- `make gate` and `go test ./...` were not run (owner preference for scoped work). No Go source
  changed.
- Transcripts from SSH-host lanes (wsl-lanes, unraid-lanes) were not read for V1-1630.

## Rollback

`ASD-V0-834`'s rollback is in `docs/specs/corvint-self-development-v0.md`. The measurements change
nothing.

## Friction

1. The `TIMEOUT` pattern in `script/lane-transcript-metrics.py` matches the phrase anywhere in tool
   output. A `cat` of a script that prints "timed out at 1800s" counts as a harness timeout.
2. The orchestrator's own session counts as a lane under `--sessions`, because it commits.
3. `heavy.sh` refuses a whole-package measurement run without `HEAVY_WHOLE_REASON`. The refusal
   costs one wasted round, and an AC that names a whole-package wall time always needs the reason.
4. `xargs -I{}` with an inline `sh -c` script failed on macOS with "command line cannot be
   assembled, too long". A small script file with `-n 1` worked.
5. V1-1628's 1.5x bound is about CPU contention inside each `go test`. An admission count cannot
   meet it without a per-command CPU limit (`-p` or `GOMAXPROCS` per slot).

## Outcome

- V1-1630: AC1 to AC3 met. AC3 met on the reviewed count of 9 timeouts; the raw count is 10.
  Delivers `ASD-V0-834`.
- V1-1627: AC3 met. Load median 57.6 → 11.4 and focused median 21 s → 9 s.
- V1-1628: AC3 gap recorded. Warm 1.51x and cold 1.60x, against 1.5x.
- V1-2819: AC2 met (median 4 s against 54 s); AC3 met (one reused slot worktree and a single
  on-disk cache).
