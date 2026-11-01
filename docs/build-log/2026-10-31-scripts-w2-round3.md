# Lane scripts, wave 2 round 3: V1-1628, V1-3549 and V1-3550 — 2026-10-31

Lane `scripts` (wave 2), branch `claude/lane-scripts-w2-1031`, on the sealed head `ad407b26c`. Every
Go run went through `script/heavy.sh` with `HEAVY_REMOTE_HOST=wsl-lanes` (32 cores, 8 slots, Go
1.27.3).

## Decisions

- V1-1628 (`ASD-V0-835`): a command `heavy.sh` offloads now runs with `GOMAXPROCS` bounded per slot.
  The default is the remote `-p`, CPUs divided by slots (4 on `wsl-lanes`).
  `HEAVY_REMOTE_GOMAXPROCS=N` overrides it, and `0` sets no bound. A `GOMAXPROCS` the caller already
  exported wins.
- V1-3549 (`ASD-V0-836`): `script/lane-transcript-metrics.py` counts a harness timeout only when a
  tool result begins with the harness's own text. Two forms were seen in real transcripts:
  - "Command timed out after 3m 0s", optionally after an "Exit code N" line.
  - "Command did not complete within its 600s timeout".

  A session that starts other sessions, or one named by `--orchestrator`, is reported under
  `orchestratorSessions` and left out of every total.
- V1-3550 (`ASD-V0-837`): `dogfood-closeout` runs each batch-drift step once, on the sealed head.
  - The precheck before the bind runs with `BATCH_PRECHECK_SKIP_DRIFT=1`. It still runs every
    documentation gate, including the OCM bound that motivated `DCW-V0-900`, and writes no receipt.
  - The precheck after the seal runs every step and writes the receipt.
- V1-3550: reusing the receipt across the seal was rejected as unsound. The ticket assumes an
  unchanged tree, but the seal adds `.corvint/changes/<sha>.cem.json`. The batch-drift steps carry
  no declared salt, so the gate ledger keys them on the whole tree.
  - Planner evidence: `corvint affected --base 13ec5fbe4` at `ad407b26c` covers a delta that only
    adds the sealed CEM. It reported `scope=UNKNOWN selected=55 unknown=1`, with the unknown being
    `LANGUAGE_FRONTIER` (`shell:foreign-invocation-unmodelled`).
  - The selected units include `cmd/corvint` (`PATH_LITERAL_READER`) and `internal/companionrelease`
    (`UNBOUNDED_READER`, through the literal `".."` in `companionrelease_test.go`).
  - So the CEM is an input of those tests. A result from before the seal does not bind the sealed
    tree.

## Evidence

- V1-1628 AC3 (met, narrowly: 1.49x against 1.5x). Whole-package
  `go test -count=1 ./internal/liveverify/affected` on `wsl-lanes`, bound `-p=4 GOMAXPROCS=4`, same
  method as `2026-10-31-scripts-w2-remainder.md`.
  - Quiet host: 49, 49 and 50 s, median 49 s.
  - 5 concurrent lanes (`xargs -P 5`), each on its own warm slot, `wait=0.0s`:
    - Round a: 72 to 73 s, load 4.82.
    - Round b: 73 s in all 5, load 6.43.
  - 73 / 49 = 1.49x.
  - The bound did not shorten the concurrent runs: before it they took 71 to 75 s. The ratio
    improved mostly because quiet runs went from 47 s to 49 s.
  - Load fell from about 8 to 5 to 6.4.
  - Floor and cause: `TestIncrementalSelectionMeetsTheLiveBudget` takes 31.15 s of the package's
    46.7 s. It is a single-threaded whole-repository graph build plus 32 `Select` rounds. With the
    CPU unsaturated, the inferred bottleneck is memory bandwidth or clock, which no per-slot CPU
    bound can remove.
  - Cost of the bound on a package run alone:
    - `internal/cem/verify`: run phase went from 5.8–6.4 s to 7.8–8.0 s, about +30 %.
    - `internal/cem/workflow`: unchanged.

    `HEAVY_REMOTE_GOMAXPROCS=0` restores the unbounded run.
- V1-3549 AC (met). Window: `--sessions --since 2026-10-31 --until 2026-11-01` over the main and
  worktree project directories, old script against new.

  | Measure | Old | New |
  | --- | --- | --- |
  | Sessions | 8 | 7 |
  | Lanes | 29 | 28 |
  | All harness timeouts | 18 | 12 |
  | 600 s timeouts | 5 | 3 |

  - The orchestrator session `ba315b73` is now reported apart.
  - Quoted false positives that the new pattern drops include "subagents agent-aef0 did not complete
    within its 180s timeout", from a `cat` of a summary.
  - Fixture tests: `make lane-scripts-test` passes 38 groups. They include the new `ASD-V0-836`
    group, which covers quoted phrases, the anchored 3m and 600 s forms, the auto-detected
    orchestrator and `--orchestrator`.
- V1-3550 AC (met as each drift step running once per closeout). Both runs are script/doc-only
  closeouts.
  - Before, at `ad407b26c`: closeout took 453 s.
    - Each batch-drift step took about 166 to 185 s, and ran twice, once in each precheck.
    - Bind, check and seal took 42 s.
  - After, measured on this branch at `a07494555`: the pre-bind precheck with
    `BATCH_PRECHECK_SKIP_DRIFT=1` passed 33 steps in 48 s wall (169 s summed). It printed
    "no receipt".
  - The post-seal precheck is unchanged.
  - The whole-closeout wall time of this lane's seal is in the lane report. It is expected to be
    about 453 − 180 s.
  - `make batch-precheck-test` passes. The new case checks that a skip-drift run calls no drift
    `go test`, prints the skipped row and writes no receipt.

## NOT_RUN

- `make gate` and `go test ./...` were not run (owner preference for scoped work). No Go source
  changed.
- The solo timing of `internal/tasks/store` under the bound. It runs past the 590 s tool limit, was
  auto-backgrounded and was killed. It was not rerun.

## Rollback

The rollbacks for `ASD-V0-835`, `ASD-V0-836` and `ASD-V0-837` are in
`docs/specs/corvint-self-development-v0.md`. `HEAVY_REMOTE_GOMAXPROCS=0` disables the bound without
a code change.

## Friction

1. A whole-package `go test` measurement of `internal/tasks/store` passes the 590 s tool limit and is
   auto-backgrounded, so solo timings of the largest packages cannot run in the foreground.
2. My first cut refused an empty `HEAVY_REMOTE_GOMAXPROCS`. Empty now means the default.
3. V1-1628's 1.5x bound is framed as CPU contention. The dominant test is single-threaded and bound
   by memory or clock, so a CPU bound barely moves the concurrent time.
4. The per-slot bound costs about 30 % on a parallel package that runs alone.
5. V1-3550's "unchanged tree" premise is false. The seal adds a CEM that the planner names as a
   drift-test input. The planner itself leaves an unknown (`LANGUAGE_FRONTIER`,
   `shell:foreign-invocation-unmodelled`), so `--base` narrowing selects everything.
6. The harness timeout phrase is quoted in other forms besides "timed out at": "did not complete
   within its Ns timeout" appears inside `cat` output of summaries.
7. zsh fails `grep --include=*.jsonl` with "no matches found". `find | xargs` works.

## Outcome

- V1-1628: AC3 met at 1.49x, against the 1.5x target. Floor cause recorded. Delivers `ASD-V0-835`.
- V1-3549: met, with fixture tests for both cases. Delivers `ASD-V0-836`.
- V1-3550: met as one drift run per closeout. The pre-bind precheck went from drift-bound (about
  185 s) to 48 s. Delivers `ASD-V0-837`.

## Review fixes

One independent review round over the lane (`script/pr-landing-watch.sh`, `script/heavy.sh`). The
spec amendment to `ASD-V0-833` and `ASD-V0-835` is its own commit, before the fixes. Each new
`make lane-scripts-test` case failed before the fix and passes after it.

- P2 concurrent supervisor passes exceed retry caps: FIXED. Each pass now holds a per-PR lock file,
  `pr-PR.lock`. The file is hard-linked into place with its PID already written. A dead holder's lock
  is taken over by renaming it away. A held lock, or a count that cannot be recorded, exits 2 without
  acting. In the test, 6 racing `--act` passes on one PR give distinct attempts, with reruns equal to
  the attempts. Before the fix, two passes both printed `attempt=1/3`.
- P2 invalid flake patterns classify real failures as flakes: FIXED. The flake list is compiled with
  `grep -E` before any `gh` call. Exit 2 refuses it. The match itself also checks grep's status. A
  `[` pattern now exits 2 and makes no call; before the fix it reached `gh`.
- P2 mixed failures rerun as flakes: FIXED. A log with `[build failed]`, `[setup failed]` or
  `panic: test timed out` escalates `real-failure`. So does a failed `JOB<TAB>STEP` that names no
  `--- FAIL:` test. Both tests printed `RERUN` before the fix.
- P2 exported caller GOMAXPROCS lost during offload: FIXED. `offload` carries `GOMAXPROCS` as
  `CALLER_GMP` in `meta.env`, and `serve` applies it over the env file, `HEAVY_REMOTE_GOMAXPROCS`
  and the default. The ssh stub now drops `GOMAXPROCS` as real ssh does. Before the fix the remote saw
  4 (default) and 3 (override) instead of the caller's 1.
- Found while fixing: without `--repo`, under bash 3.2 (macOS `/usr/bin/env bash`), the empty
  `repo_args` array aborted on `set -u` (`repo_args[@]: unbound variable`). FIXED with the `${a[@]+...}`
  form, and a pass without `--repo` is now tested.

Batch-1101c note: the in-place amendment of `ASD-V0-833` and `ASD-V0-835` was not carried. Their accepted text stays, and the added rules are the new refining requirements `ASD-V0-838` (V1-1735) and `ASD-V0-839` (V1-1628).
