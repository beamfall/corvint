<!-- SPDX-License-Identifier: Apache-2.0 -->
# Sixteen Go CI shards from refreshed hosted costs

Owner intent (2026-11-02, “lets go 16”): reduce the slowest required Go-test shard on pull
requests, merge-queue entries and main pushes. Lane `ci16-w54`, branch
`claude/lane-ci16-w54-1102`, base `b486d3978728731c6638b50fa2210dfbb6e365ea`.

`AFP-V0-2180` records this slice under the owner's standing approval; it is not an owner
read-back. Existing `AFP-V0-041`, `AFP-V0-049`, `AFP-V0-060` and `AFP-V0-590` are amended in
place. The affected-plan Requirements section had 254 obligations and now has 255, below the
256 cap; no companion spec or extra obligation is needed. Existing `AFP-V0-022/024/040`,
`AFP-V0-048/164/170` and `AFP-V0-591` retain placement, complete-run and coverage safety.

## Inputs and implementation

The operator supplied all eight `ci-shard-costs-<i>` artifacts from successful FULL
[pull-request run 41429718453](https://github.com/beamfall/corvint/actions/runs/41429718453).
Use source revision `f216f4cb3056c6e46d69cc4fe402c6c76c1cf9f3`: the operator established that
tested merge `5524c224` merged it into its own ancestor `b5620b4d9`, so their trees are equal.
The source revision is locally present on origin/main, with tree
`8d9a1dde4c7683b37e1f37d83a3a46e82aa6f470`. The synthetic merge object is absent locally;
its tree equivalence is supplied operator evidence, not an independently repeated Git check.
`ci-costs/` remains input only and is excluded from delivery.

Mechanical refresh replaces the old 373-package, 8,887,238 ms table (148.1206 package-test
minutes, run 40122992631) with 424 packages and 9,703,394 ms (161.7232 package-test minutes).
These raw terminal-package elapsed costs exclude compilation, setup, cache restores and
queueing. They do not establish or contradict the operator's approximately 195 job-minutes
for a contemporary full CI run; the denominators differ.

Every matrix-dependent count is sixteen, including the narrowed-tree record count. The
matrix exports FULL outputs 0 through 15, and both full and narrowed record conditions
require all sixteen. The coverage guard uses its SHARDS parameter; its executable test now
checks sixteen markers and rejects an old eight-marker set. Forward-row jq already uses its
supplied count and needs no algorithm change. Frozen eight-shard hosted fixtures and historical
cost/reuse tests retain their original count, so their provenance and compatibility remain tested.
Generic partition/selection tests using eight also remain valid inputs; committed-plan admission
and bound tests now use sixteen. The unrelated eight-rerun limit, eight complete qualification
rows and 8 GiB seed cap stay unchanged. Historical decisions/build logs and the CI-narrowing
spec's description of earlier runs stay historical.

The slice helper previously equated input-log count with output-shard count. New
`--log-shards` admits a declared complete source run separately; the default remains the output
`--shards` value (now sixteen). Both generation and replay combine source slices using the
source count, preserving partial, failed and repeated-run refusals. Its help/comment examples
and focused rebalance test cover that distinction. Replay reads the original source allow-list
and slices from a temporary copy; candidate costs and slices are explicit arguments.

Generation at the default ideal share produced two cmd slices and three store slices, but
isolated conformance exposed its conflict with the existing AFP-V0-060 350-second generation
target and 400-second slice bound. That candidate was discarded. The delivered generation uses
`--target 350s`, retaining that accepted contract: cmd/corvint has three slices (largest planned
343,500 ms), tasks/store four (311,501 ms), tasks/cli two (235,588 ms); authority stays whole.
Both packages above the sixteen-shard ideal share (606,462 ms) split. No allow-list change.

## Replay predictions, not hosted measurements

Both replays place the same fresh measured package universe. The old plan uses the saved old
cost/slice files at eight shards; the candidate uses refreshed costs/slices at sixteen.
Per-shard values below are seconds rounded by the replay tool to one decimal.

| Shard | Old eight-shard prediction | New sixteen-shard prediction |
|---:|---:|---:|
| 0 | 962.2 | 606.4 |
| 1 | 1207.3 | 606.4 |
| 2 | 1183.6 | 606.4 |
| 3 | 1244.3 | 606.4 |
| 4 | 1208.6 | 606.4 |
| 5 | 1535.5 | 606.4 |
| 6 | 1223.7 | 606.4 |
| 7 | 1138.1 | 606.4 |
| 8 | — | 607.4 |
| 9 | — | 606.4 |
| 10 | — | 606.4 |
| 11 | — | 606.4 |
| 12 | — | 606.4 |
| 13 | — | 606.4 |
| 14 | — | 606.4 |
| 15 | — | 606.4 |
| Minimum | 962.2 | 606.4 |
| Maximum | 1535.5 | 607.4 |
| Total package time, rounded | 9703.4 | 9703.4 |

Summing the individually displayed old values gives 9703.3 seconds due to rounding. The model
apportions package time by observed top-level test shares, rounding slice estimates to milliseconds;
those estimates do not measure new slice-process overhead. The predicted maximum drops about
60.4% (25.59 to 10.12 minutes). The supplied run actually measured eight shard package sums
962.2, 1209.1, 1111.0, 1315.1, 1300.0, 1457.1, 1179.2 and 1169.6 seconds; these are distinct
from the old-plan replay predictions.

## Verification and limits

Exact commands, environment adjustments, local results and retained failures are in
`ci16-w54.handback.md`; raw local command outputs and input SHA-256 values are retained under
`/tmp/ci16-w54-evidence/`. The focused safety checks cover count agreement, all FULL outputs,
source/output rebalance with complete logs, sixteen-marker coverage, and isolated race-enabled
partition/slice conformance. All four focused tool packages and the isolated race-enabled cishards module passed after the
retained fixture repairs. Spec requirements, obligation/enumeration, line-citation, flow-guide,
requirement-ID collision, formatting and whitespace checks passed. Line-citation checking retained
its note that 180 citations with untracked root basenames were skipped. A spec test fixture
inherited and rewrote the disposable staged index; it was rebuilt, and the final check cleared
Git overrides for test subprocesses. The real index and worktree sources were unaffected.
Exact final results are recorded in the handback.

The Corvint context and affected routes were used against the pinned base; context returned
AFP-V0-590 and the workflow, while reporting an in-memory index build. The final affected
receipt remains UNKNOWN (unowned workflow/JSON/input paths and language frontiers). The installed
binary reports DEGRADED self-binary-stale (built from 5d1ed52b17d6; 23 Go paths differ), so
its advisory output is not current-source or selection-completeness evidence. Retrieval/profile changes,
learning and frozen retrieval evaluations are not applicable to this CI placement slice.
`make dogfood-change` failed because its linked-worktree Git evidence directory is read-only.
CEM/reviewer binding and seal are `NOT_PRODUCED`: post-commit binding cannot run under this
builder's explicit no-commit instruction. Independent review is still required from the integrating
reviewer; no codex or delegated reviewer was invoked. No repository-wide gate, hosted qualification,
publication or native ticket action was run in this builder lane.

The real Git index and object directory are outside writable roots. Staged-spec checks therefore
use a private index and object store under `/tmp`; the actual worktree remains unstaged. This is
local staged-input evidence only, requiring normal staging/checking by the integrator. The handback
retains commands and limitations instead of claiming a clean committed closure.

Hosted sixteen-shard wall latency is `NOT_OBSERVED`. About 26 jobs per run can hit Actions
concurrent-job limits. The race cache key includes job-total, so the transition creates one cold
race cache per shard; extra shards duplicate race compile minutes. These may offset the predicted
package-time improvement. Rollback is one revert of this slice, restoring all counts and FULL
outputs/guards to eight together with the old cost and slice tables. No test membership,
selection authority, narrowing thresholds or failure policy is relaxed.
