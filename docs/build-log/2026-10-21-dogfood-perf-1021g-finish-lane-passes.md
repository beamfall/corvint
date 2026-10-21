# Finish-lane passes run only what they decide; per-step timings and interrupt recovery

Lane dogfood-perf-1021g, tickets V1-1664 (V1-1616 folded in) and V1-1488. Requirements
`DCW-V0-115` and `DCW-V0-116` in `docs/specs/daily-change-evidence-workflow-v0.md`, accepted under
the owner's standing auto-accept (2026-10-11).

## Problem

`dogfood finish-lane` ran three full `dogfood change` passes. The prepare pass needs only the
prepared map, yet it also ran the coordination-time query and impact, the OCM prepare/status/
aggregate, `cem status` and the recorder, which always fails before the bind commit
(`record-index-failed`). The bind pass ran query, impact and the recorder again, all of which
the rebind recomputes and the check decides. The prepare pass's OCM map was also outdated by the
bind's citations, so every bind paid a refused `ocm prepare` plus a `--replace` retry. There
were no phase timings, and an interrupted run printed the step and rows but no recovery.

## Profile (same host, 12 cores; host load from `uptime` before and after each run)

Fixture: a docs-only change of 72 hunks (two lines appended to 72 build-log files), a 72-row
plan, one intent spec, `git diff --check` as the verify command, `make dogfood-finish`, step
binary wrapped to time each command. Before = binary and tree at `ae3b41815`; after = this
change, same fixture cherry-picked.

| Run | Load (1 min) | Wall | Commands | Command seconds |
|---|---|---|---|---|
| before, first profile | 161 to 266 | 255 s | 198 | 225 s |
| before, re-measured | 16 to 60 | 132.9 s | 198 | 115.3 s |
| after | 18 to 20 | 67.5 s | 179 | 58.9 s total |
| after, fresh clone | 56 to 34 | 73.4 s | 179 | 67.4 s |

Before (first profile, per pass): prepare about 55 s, bind about 66 s, rebind about 97 s. Dominant
commands across the run: `dogfood-record` 74 s (52.6 s in the rebind alone), `impact` 37 s,
`ocm prepare` 26 s over four runs, `cem cite` 22 s (144 runs), `query` 15 s. Re-measured: record
20.8 s, impact 17.5 s, `ocm prepare` 16.2 s (4 runs), `dogfood-ocm status` 15.2 s.

After (native TIMING, fresh clone): prepare 0.44 s (1 command), bind 14.67 s (83), commit 0.14 s,
rebind 37.70 s (89), check-seal 18.13 s (6). Dominant stage: the rebind, and within it
`dogfood-record` at 14.3 s; then `dogfood-ocm status` in check-seal at 9.3 s over 3 runs, and
`cem cite` at 14.1 s over 144 runs.

## Decisions

- An unexported pass scope in `change`: `prepare` returns after `cem prepare` (on failure it still
  renders the report and prints the failure rows and hints); `bind` skips query, impact and the
  recorder. The rebind stays the full pass, so the rebind, check and seal evidence is unchanged;
  the cmd recipe test still proves the seal tree equals the manual recipe's.
- Timing wraps the step and verifier runners at `FinishLane`, so every subprocess is recorded with
  its step; stderr gets one TIMING line per step and a total naming the private TSV. Labels keep the
  command's leading words only.
- Recovery: an interruption prints the exact restore and rerun. The existing resume-from-bind-commit
  logic is the resume path; nothing new is persisted.
- V1-1488: the reported "about 5 min per spec" did not reproduce on current main. Direct
  `ocm prepare` per spec: 1.41 s and 1.20 s on an uncited lane map (load about 290); 3.10 s and
  2.85 s on the 72-hunk cited map (load about 300); 4.73, 4.69 and 4.85 s for the three specs of the
  PR #961 stage map (56 hunks, load about 290). Inside finish it is now 1.80 s (bind) and 2.02 s
  (rebind) at load 56 to 34. The fix removes the prepare pass's OCM map and with it the bind's
  outdated-map refusal and retry: four runs per spec became two. A `verification-timeout` row now
  prints a `fix:` line naming the intent line, its stderr and the rerun.

## Evidence

- `TestFinishLaneDCWV0115PreparePassRunsOnlyCEMPrepare`, the updated
  `TestFinishLaneDCWV0067NamesTheInterruptedStep` and `...NamesAnInterruptedSeal`, and
  `TestDogfoodFinishLaneDCWV0060MatchesManualRecipe` fail with the scope map set to full and the fix
  line removed, and pass with the change.
- Live interruption: SIGINT 28 s into a finish stopped it at the rebind after the bind commit;
  the printed restore plus the rerun resumed from the bind commit and sealed in 54.1 s.

## NOT_RUN and limits

- A quiet-host measurement: the host load never fell below 16 during the lane (NOT_OBSERVED).
- A finish at load 60 or more with this change: the after runs saw 18 to 56; the before profile at
  161 to 266 has no same-load after run. Wall time stayed under 600 s in every run.
- `verification-timeout` from `ocm prepare` was not reproduced, so its hint is tested only by
  construction (NOT_OBSERVED live).
- The recorder (`dogfood-record`, about 14 s per run) and the query and impact recomputation in
  the rebind remain the dominant costs; they are outside this change.

## Rollback

Revert the change: the passes run every step again, the TIMING and recovery lines disappear and
`.git/corvint/dogfood-finish-lane.timings.tsv` becomes an unused private file. No evidence format
changes, so nothing needs migrating.
