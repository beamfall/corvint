# 2026-10-14 V1-1343: the live selection budget under a contended or race-instrumented runner

Lane V1-1343, base adf587fdb (origin/main). Intent spec: `docs/specs/live-proof-carrying-verification-v0.md`,
whose P3 gate states "warm plan publication p95 at most 100 ms" on clean machines; no spec states the
100 ms incremental-selection figure as a requirement, and no spec text changes here. No new
requirement IDs.

## Intent

`TestIncrementalSelectionMeetsTheLiveBudget` held a hard 100 ms p95 over 160 `affected.Select` calls
on whatever runner its shard landed on. Merge-queue run 38376972938 (go-product-shard 7) failed it at
p95 102.0 ms, p50 91.4 ms; the pull-request run of the same tree passed. CI shards run
`go test -race -p 1`, so the hosted figure is a race-instrumented one on a shared runner, roughly four
times this host's uninstrumented cost. A merge-queue eviction for runner speed costs a full queue
cycle. The product target stays: 100 ms p95 on the real repository graph, enforced as an absolute
figure whenever the host meets it.

## Decision: a same-process calibration probe

- `timeCalibrationProbe` is a fixed, deterministic, single-goroutine workload shaped like selection
  (string-keyed maps, witness path copies, breadth-first frontiers, sorts over a synthetic 512-node
  graph). It does not call the `affected` package; it shares only the Go runtime's allocator and
  collector with `Select`. The test times one probe before each of the 160 selections, interleaved,
  so both see the same host.
- `enforceSelectionBudget`: a selection p95 within 100 ms passes as before. A miss is judged in probe
  units: selection p95 divided by the probe p95 over the same interleaved run. Above the bound the
  test fails on any host ("selection itself is too slow"). Within it, the test calls `t.Skipf`
  naming the absolute budget as `NOT ENFORCED`, with the selection p95, the probe p95, the ratio and
  the bound. No path passes the absolute budget silently.
- The bound is 100 ms / `referenceProbeP95` (20 ms) = 5.0. The probe p95 on this host (Apple
  M-series, 12 cores, go1.27.2, no race detector) measured 17.1-19.6 ms; rounding up to 20 ms makes
  the bound stricter. On this host selection p95 is 1.2-1.3 probe p95s uninstrumented and 1.2-1.5
  under `-race`, so the bound is the 100 ms budget moved to the reference host's speed, not a widened
  constant: a ratio above 5.0 means selection would miss 100 ms on the reference host. Each caller
  passes the reference value of the probe statistic it divides by.
- p95 over p95, not p95 over the probe median: both tails come from the same interleaved samples, so
  contention that inflates selection's tail inflates the probe's alike. A median variant (bound
  100/15 = 6.67) was tried and rejected: unchanged code under natural load ~60 read 3.79-4.70, well
  over half the bound, because selection's tail grew while the probe median did not.
- `TestSelectionOnTheLiveDirtyWorktree` shares `Select` and the failure class (BUILD-LOG
  2026-09-25: both tests exceeded the budget under host load 170-570), so it uses the same
  enforcement. It used to time one selection; it now times five, interleaved with five probes, and
  judges the selection median against the probe median, with the bound 100 ms /
  `referenceProbeMedian` (15 ms, measured 13.6-14.3 ms) = 6.67. The single-sample form was tried
  first: under natural load ~60 the unchanged code failed once, one selection at 355 ms against a
  48.8 ms probe median (ratio 7.28), a spike that a slower `Select` would show in every sample
  instead. On a clean CI checkout it selects nothing and takes under a millisecond. That is the
  helper's second caller. The helper stays in the package; V1-1324 (tasks-store timing tests under
  load) is a possible later consumer, not a caller now.

## Rejected options

- Widening the constant: drops the product target. Refused by the lane design.
- Best of several interleaved batches: the hosted failure has p50 91.4 ms, persistent slowness from
  race instrumentation and runner speed rather than a transient spike, so the best batch still sits
  near 100 ms.
- Skipping under the race detector: CI only runs race shards, so the budget would never be checked
  in CI.
- Thread CPU time instead of wall time: not portable (process-wide on darwin, includes GC workers).
- Retry on failure: a retry that passes hides a real regression whenever the host happens to be fast.

## Evidence

All runs on this 12-core host, shared with other lanes (natural load averages 16-104 throughout).
"Contended" adds 12 busy loops (`yes`, one per CPU) for the run, killed by a trap on exit
(`agent-1343/contend.sh`). Numbers are selection p50/p95 and, after the change, probe p95 and the
ratio.

Incremental test (160 selections; selection p95, ratio = selection p95 / probe p95, bound 5.00):

| Run | Load | Result | Selection p50 / p95 | Probe p95 | Ratio |
| --- | --- | --- | --- | --- | --- |
| base, natural load | 53 | FAIL 3/3 | - / 110-336 ms | - | - |
| base, quieter | 17-23 | PASS 5/5 | 17-21 / 23-59 ms | - | - |
| base, `-race` | 16-35 | FAIL 3/3 | 82-94 / 157-550 ms | - | - |
| base, contended | 22-69 | FAIL 3/3 | - / 163-201 ms | - | - |
| after, quieter | ~16 | PASS 5/5 | - / 20.6-25.9 ms | 17.1-19.6 ms | 1.21-1.32 |
| after, natural load | 58-61 | SKIP 6/6 | 64-123 / 243-356 ms | 164-267 ms | 1.11-1.59 |
| after, `-race` | 16-59 | SKIP 6/6 | 283-338 / 416-1045 ms (3 runs) | 297-582 ms (3 runs) | 1.21-1.80 |
| after, contended | 22-104 | SKIP 6/6 | 74-86 / 271-361 ms (3 runs) | 215-251 ms (3 runs) | 1.07-1.44 |
| 8x CPU regression, natural load | 73-104 | FAIL | 686 / 1240 ms | 148 ms | 8.37 |
| 8x CPU regression, contended | 63-68 | FAIL | 812 / 1548 ms | 183 ms | 8.47 |
| 4x CPU regression (first trial) | 50-70 | FAIL 2/3, SKIP 1/3 | - | - | 4.38-5.81 |
| 100 ms sleep, natural load | 66 | SKIP | 133 / 179 ms | 72 ms | 2.49 |
| 100 ms sleep, contended | 43-70 | SKIP | 169 / 292 ms | 165 ms | 1.77 |

The "after" rows pool every run of the final incremental check, across the evidence passes; the
dirty-worktree changes below did not touch it. Ranges marked with a run count come from the runs
that logged those figures. The 4x injection costs about 100-110 ms on the reference host, so it
sits on the bound by construction; the 8x injection, about 200 ms there, failed by a wide margin in
both conditions.

Dirty-worktree test (this lane's worktree dirty: 49 units selected; median of five selections /
median of five probes, bound 6.67):

| Run | Load | Result | Selection median | Probe median | Ratio |
| --- | --- | --- | --- | --- | --- |
| single-sample form, natural load | 58-61 | FAIL 1/3 | 355 ms | 48.8 ms | 7.28 |
| after, natural load | 54-70 | PASS 2/8, SKIP 6/8 | 40-302 ms | 21-101 ms | 1.05-4.43 |
| after, contended | 54-104 | PASS 1/8, SKIP 7/8 | 82-244 ms | 56-136 ms | 0.95-4.34 |
| after, `-race` | 46-60 | SKIP 3/3 | 204-490 ms | 88-179 ms | 2.33-3.94 |
| 8x CPU regression, natural load | 73-104 | FAIL | 733 ms | 33 ms | 22.08 |
| 8x CPU regression, contended | 63-68 | FAIL | 1329 ms | 71 ms | 18.64 |
| 100 ms sleep, natural load | 66 | SKIP | 150 ms | 27 ms | 5.60 |
| 100 ms sleep, contended | 43-70 | SKIP | 216 ms | 54 ms | 3.98 |

(The regression rows select 84 units, because the scratch worktree has a different dirty set.)

## Failure modes and limits

- Wall-clock regressions that do not burn CPU (a sleep, a lock wait, I/O) are discounted by the
  contention factor in the relative path, because the probe slows only with host load. The injected
  100 ms sleep skipped in every run above. With selection at 1.3 probe p95s, a 100 ms sleep fails
  the relative check only when the probe p95 is under about 27 ms, a host within about 1.35 times
  the reference speed; on a host that meets the budget, it fails the absolute check as before.
  `Select` is pure in-memory computation today, so its realistic regressions are CPU regressions,
  which scale with the reference-host cost and fail (the 8x rows). A future `Select` that waits or
  does I/O needs a different guard.
- Contention that lands on selection's tail but not on the probe's can still fail the unchanged
  code in the relative path. The incremental bound leaves about 2.8 times the highest measured
  ratio (1.80, under `-race` at load ~50). The dirty-worktree bound leaves only about 1.5 times its
  highest (4.43): that selection slows under load more than the probe does, and five samples are
  few. It reaches the relative path only on a dirty developer worktree; on a clean CI checkout it
  selects nothing.
- An allocation-heavy regression in `Select` can raise the shared collector's cost and so slow
  the probes a little, masking part of itself. The injected CPU regression, which repeats the
  allocating reach computation, still failed.
- The relative path accepts any host speed: a runner ten times slower than the reference passes if
  selection is within the bound in probe units. The skip message says so with the numbers.
- The bound rests on the probe's cost ratio to selection being stable across architectures and the
  race detector. Measured here: 1.2-1.3 uninstrumented on a lightly loaded host, 1.2-1.8 under
  `-race` and load; the hosted x86 ratio is NOT_OBSERVED until the next CI run logs it.
- The probe adds about 160 x 14 ms (2.3 s) to the test uninstrumented on a quiet host, and several
  times that under `-race` (about 30 s at load ~50 here).
- Growth of the repository graph raises selection cost; the absolute and relative checks both
  follow it, so a much larger graph will fail on its own merits.

## Independent review

One Codex read-only round (verdict CONCERNS) on the first draft, which divided by the probe median:

1. Selection's tail against the probe's median can fail a contended runner. Confirmed by the median
   variant's numbers above; fixed by dividing p95 by p95.
2. A regression is skipped when contention raises the probe. Partly valid: a CPU regression scales
   with the probe and fails (8x rows); a wait or sleep is discounted, recorded under limits.
3. The dirty-worktree probes all ran after the selection. Fixed: now five interleaved pairs.
4. The probe shares the allocator and collector with `Select`. Fixed the wording; recorded as a
   limit.

The later changes (p95 over p95, median of five in the dirty test, a reference value per probe
statistic) follow from this lane's own measurements and had no second review round.

## Verification

- `corvint affected --base adf587fdb`: 52 units, scope UNKNOWN (go build-constraint variants,
  nested-module frontier). Run: the package tree `./internal/liveverify/affected/...` and the plan's
  bounded units (`internal/diagnostic`, `internal/mcp/bridge`, `internal/obscorpus`,
  `internal/semescalate`, `cmd/corvint-go-test-provider`, `conformance/go-live-test-v0`,
  `tools/compat-trial`, `tools/corvint-pr-tests`, `tools/fixture-maintenance-check`): all ok. The
  plan's 42 unbounded-reader units (`cmd/corvint` among them) are NOT_RUN: the change is one test
  file and a build-log entry, and the host is shared.
- `go test -race` on `./internal/liveverify/affected/`: ok. `go vet` on the package tree: ok.
  `gofmt -l cmd internal tools .github/cishards .github/testconfine`: empty.
- The 12 CI doc gates, staged: all exit 0.

## Rollback

Revert the commit: the test returns to the hard 100 ms p95 assertion. No data, wire contract or
spec changes.
