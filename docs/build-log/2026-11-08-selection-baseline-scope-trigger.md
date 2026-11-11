# Selection-baseline scope trigger and CI cache finding — 2026-11-08

Owner-owned intent: lane sbcache, V1-5162; base
`b76d89e80d7df5f0deb4dd7d134d9358106d5dbc`. The ticket reported that the CI doc-gates job hit
its 30-minute limit on batch PR 145 (run 42498373123, job 128785597706) because
`selection-baseline-check` replays the whole change set whenever a batch edits
`.corvint/test-read-scopes.json`, and that the result cache was off in CI. This entry records the
evidence, the design choice and the measurements. It does not record ticket completion.

## What the replay reads

`tools/selection-baseline` clones the repository with `--shared` and, for each change, checks out
the change's immutable head in that clone and runs the planner (`corvint affected`, `corvint test
--dry-run`) and the selector there. Both read that clone's own `.corvint/test-read-scopes.json`,
never this checkout's copy. In the current checkout the replay reads only
`benchmarks/selection-baseline/changes.json`, `.github/cishards/package-costs.json` and `go.mod`.
These are the driver's declared read scope.

So this checkout's declaration matters only to the input decision, through the driver's own entry:
that entry bounds which current-tree paths count as driver runtime inputs (`ASD-V0-944`). Corvint's
reader index binds the declaration file to every declared package (`inReadScope`), so the old
decision ran on any edit, including one confined to other packages' entries. 22 of the 90
first-parent merges to main since 2026-11-01 edited it.

## Decision

`ASC-V0-9850` (amends `ASD-V0-944`) compares the declaration's profile and the
`tools/selection-baseline` entry between the base blob and the worktree. Both sides are decoded as
Corvint decodes them (json/v2, unknown members and duplicate names refused). If they are equal, the
declaration path is dropped from the explicit inputs and the reader bounds. A changed or removed
entry runs, and so does any doubt: absent at base, deleted, not a regular file, or undecodable.
`.corvint/unbounded-readers.json`, `Makefile` and the module files stay whole-file inputs.

`ASC-V0-9851` keys cache records without the change entry's `recorded` scope and verdict, which
`measure` never reads. `ASC-V0-9852` prints a cold-cache line.

## CI cache finding

The `result cache off (SELECTION_BASELINE_CACHE=off or CORVINT_GATE_LEDGER=off)` line in that
job did not come from `selection-baseline-check`. The job's environment sets neither variable.
The line came from PR 145's own new `selection-contract-check` doc gate
(`script/selection-contract-check.sh` on `claude/batch-1108a`), which runs
`SELECTION_CONTRACT_CHECK=1 SELECTION_BASELINE_CACHE=off SELECTION_BASELINE_JOBS=1 selection-baseline`
and so replays all 58 changes a second time, serially, alongside the target on the same 4-CPU
runner.

`selection-baseline-check` itself printed no REUSE line. Its cache was on but cold, because a
hosted runner starts with an empty user cache directory and the workflow persists none. It ran
16:10:57 to 16:38:43 (1684 s, exit 0), and the job was cancelled at 16:39:08 while the contract
replay was still at `pr-38`.

All three batch PRs named in the ticket also change `cmd/corvint` (the new decision prints
`matching input Makefile`, `.corvint/unbounded-readers.json` and `cmd/corvint/affected.go` for
PRs 145, 146 and 147). The planner changed in each, so their replay is required, and no cache
keyed on planner bytes can reuse it.

## Measurements

WSL lane host, 26 CPUs, 8 replay workers, `TMPDIR` on disk. The first two rows use a fresh
`SELECTION_BASELINE_CACHE_DIR` (cold, as on a CI runner). The edit is
`tools/str-parity: [] -> ["go.mod"]` in `.corvint/test-read-scopes.json`, uncommitted, with
explicit `BASE` set to the commit holding the code under test.

| Case | Before (base code) | After |
|---|---|---|
| (a) scope-only edit to an unrelated entry, cold cache | runs: `matching input .corvint/test-read-scopes.json`, 532 s | skips: `diff is disjoint`, 1.35 s |
| (b) identical rerun of (a) | REUSE 58 of 58, 8 s | skips, 1.36 s |
| driver-entry edit (`tools/selection-baseline` gains `internal/cem/`) | runs | runs: `matching input .corvint/test-read-scopes.json: its tools/selection-baseline entry changed` |
| full replay with the changed tool, cold then warm | n/a | 511 s, then REUSE 58 of 58 in 7.4 s, identical results (`narrow=33/43`, `misses=0`) |

## Not done here

No workflow edit (this lane leaves `.github` alone). Reuse across CI runs needs the doc-gates job
to restore and save `SELECTION_BASELINE_CACHE_DIR`. Records are content-keyed, so restoring any
earlier directory can only add hits. It helps only a rerun whose planner, selector and tool bytes
are unchanged. The larger lever for PR-145-shaped timeouts is not running a second full serial
replay in the same job.
