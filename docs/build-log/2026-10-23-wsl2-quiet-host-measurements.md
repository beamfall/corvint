# Quiet-host measurements on WSL2 (V1-2442)

Date: 2026-10-23. Tickets: V1-2442 (host), V1-2205 and V1-2385 (B2), V1-2203 (B1 baseline),
V1-2300 (index wall time), V1-2347 (probe reference), V1-1569 (pool acquire at 45k), V1-2399
(procgroup TERM grace on Linux). Host setup and harness settings: [BENCHMARK-HOST](../BENCHMARK-HOST.md).

**Every number here is WSL2/Linux.** None is compared with a macOS, native Windows or CI number.
Comparisons are only between binaries built and run on this host in the same session.

## Host and binaries

- **Measured sha:** origin/main `0641ad2e09a6da042e144fa5ded4861637e550a9`, in a detached worktree
  on WSL ext4.
- **Comparison binaries:** base `ef2a01db5`, seal `9080ab596`, and pre-fix `c7505ee38^1`, each
  built in its own worktree on the same host.
- **Host:** AMD Ryzen 9 5950X (32 logical CPUs visible), 31 GiB visible. Windows 10.0.26200.9168,
  WSL 2.7.13.0, kernel `6.18.33.2-microsoft-standard-WSL2`, Ubuntu 26.04 LTS. Windows power plan
  "Winhance Power Plan"; WSL2 exposes no cpufreq governor.
- **Go:** 1.27.2, with a verified official checksum.
- **One serial runner:** no other measurement and no agent-started Windows workload ran during it.
  A native Windows qualification (V1-2441) was held until the run ended.

**Idle checks.** One ran before every step and is retained as JSON in `~/bench/results/*.idle.json`
on the host.

- probe and index were quiet: load1 0.97 and 0.01, Windows CPU 0.6% and 1.6%.
- procgroup, B1, B2 and pool were marked not quiet against the check's own 1.0 bar: load1
  1.1–4.3. Windows CPU stayed between 0.4% and 1.5% throughout.
- The load was residue from the previous step on this host. It stayed far below the 16-core bound
  of decision 0549.
- The pool run's own 8 acquirers raised load to 14.5, still at or below the core count.

## Results

| Ticket | Measure | Median (spread) | Target | Verdict |
| --- | --- | --- | --- | --- |
| V1-2300 | `corvint index` CLI wall time, 10 alternating rounds, fresh snapshot store each run | base 3.41 s (3.17–3.56), seal 2.41 s (2.31–2.57), main 2.53 s (2.24–2.78); paired change vs base: seal −28.2% (−33.1 to −22.1), main −26.3% (−33.1 to −14.8) | −20% | **met** |
| V1-2203 (B1, 880 tickets, 5 reps × 3 runs) | heartbeat wall at 3.5k / 13.5k / 40k receipts | 72.4 (70.5–84.6) / 72.1 (70.4–79.9) / 76.4 (74.7–85.5) ms | ≤ 50 ms, flat to 40k | flat; **miss** on level |
| V1-2203 (B1) | renew wall at 3.5k / 13.5k / 40k | 81.6 / 82.0 / 85.0 ms | — | — |
| V1-2203 (B1) | heartbeat `lease.fsync` / `commit` phases | fsync 32.8 / 31.8 / 34.4 ms; commit 46.6 / 46.0 / 48.4 ms | — | floor |
| V1-2205 / V1-2385 (B2, 40k receipts, 15 writers, 120 s, 3 runs) | heartbeat wall p50 / p99 | 116 (115–121) / 1331 (862–1599) ms | ≤ 50 / ≤ 250 ms | **miss** |
| V1-2205 / V1-2385 (B2) | renew wall p50 / p99 | 113 (113–120) / 1837 (1091–2592) ms | ≤ 250 ms p99 | **miss** |
| V1-2347 | AHI-172 probe (`/usr/bin/true` every 100 ms), p95 per 60 s window, 5 windows | 1.03 ms (0.99–1.10); p50 0.79 ms; max 2.53 ms | reference 2.0 ms | one host |
| V1-2399 | `TestSourceViewCancellationTerminatesGitProcessGroup` ×10 | main 0.08 s each; pre-fix (`c7505ee38^1`) 5.07–5.09 s each | well under 1 s | **met** |
| V1-2399 | `TestPGOV0022*` ×5 (including the TERM-ignoring subtest) | 15/15 pass | pass | **met** |
| V1-1569 | `TestPSRV0055_PoolAcquireLoad`, 45k receipts, 8 acquirers, one run | 49/132 acquires completed; p99 233 s; failures are inotify `ENOSPC` | PSR-V0-055 bound | **invalid run** |

**B2 detail.**

- `LOCK_TIMEOUT` was 0 in every run.
- Heartbeat writer hold: p50 92–96 ms, p99 125–156 ms. Gate wait p99 for heartbeat ran 760–1457 ms
  and for renew 986–2481 ms.
- Claims held the writer 637–1119 ms at p50 and up to 1809 ms. Their whole call took 38–61 s at p50,
  because claim/widen/reap walk 40k receipts outside the gate.
- The p99 miss is therefore queueing behind claim holds, on top of the fsync floor.

## Findings

- **V1-2300 meets −20% on a quiet host.** The lane's −8% Darwin figure was measured under load 25–60.
  The quiet WSL2 run shows −28% at the seal. Main is −26%, with the alternating-pair spread
  overlapping the seal's.
- **The heartbeat floor on this host is storage.** On this WSL2 ext4 virtual disk, `lease.fsync`
  alone is 32–40 ms of a 46–48 ms commit. The 50 ms median is unreachable here at the current
  number of synced writes per heartbeat, whatever the receipt count. Heartbeat time is flat from
  3.5k to 40k receipts, so history growth is not the cost. V1-2203 stays open: its acceptance
  is ≤ 50 ms, and fewer or cheaper syncs is the lever this baseline names.
- **The V1-2205 B2 p99 is queueing, not per-write cost.** Heartbeat and renew hold the writer for
  about 95 ms, but they wait up to 2.5 s behind claim sections of 0.6–1.8 s. Meeting p99 ≤ 250 ms
  needs shorter claim holds, or lease writes that do not queue behind them, in addition to the
  fsync floor. This is retained as a miss with cause.
- **AHI-172 probe.** On this host the production sampler's p95 is stable within ±7% across five
  one-minute windows, at about half the fixed 2 ms reference. So the fixed constant overstates the
  probe cost here, and the slowdown verdict saturates at 1.0. One host is not the two the ticket
  needs, and OpenCode is not installed on the WSL2 distro, so no candidate/probe ratio was
  measured. V1-2347 stays open.
- **V1-2399's Linux repro confirms the fix on main.** A TERM-obeying group now returns in 0.08 s
  instead of the full 5 s grace. The TERM-ignoring subtest still SIGKILLs after the grace.
  Darwin's `graceHeldQuiet` is nil, so macOS behaviour is unchanged.

## V1-1569: pool acquire at 45k receipts is not qualified

**Verdict: invalid run.** The PSR-V0-055 bound at 45,000 receipts stays `NOT_RUN` on Linux.

- **First 45k run** (defaults: 8 acquirers, 600 s, 300 s grace): 49 of 132 acquires completed.
  Acquire wall p99 was 233 s and the maximum 247 s. Most failures came back as `MALFORMED`.
- **Diagnostic 45k rerun.** The harness keeps only the error code (`pool_acquire_load_test.go:386`),
  so this run used a scratch build that also kept the error text. A native Windows workload was
  active during it, so its timings are not results.
  - Every `MALFORMED` was `no space left on device`: acquire 62, claim 53, pool-release 7.
  - The disk was 2% used and the inodes 1%.
- **10k diagnostic run, 90 s:** no errors.
- **Cause (from reading the code).**
  - `authority.WatchChanges` adds one inotify watch for every path in the store
    (`change_guard_linux.go`). So every live guard holds O(receipts) watches.
  - `inotify_add_watch` returns `ENOSPC` when the per-user total is reached. On this host
    `fs.inotify.max_user_watches` is 524288.
  - About ten concurrent guards over a store of roughly 50k paths exceed that limit.
  - The guard's descriptor is closed, so this is not a leak. V1-0906 and V1-1225 bounded descriptors,
    not watch count.
- **Classification.** `wire.CodeOf` maps `ENOSPC` to `MALFORMED`, where it maps `EMFILE` and
  `ENFILE` to `RESOURCE_EXHAUSTED`.
- **Filed:** V1-2482 (bounded watch count on Linux) and V1-2483 (`ENOSPC` classification, and the
  harness dropping error detail).
- **Not done:** the host's inotify limit was not raised. Darwin's kqueue path is not affected by this
  limit.

## Reproduce

Follow [BENCHMARK-HOST](../BENCHMARK-HOST.md). The runner `~/bench/run-all.sh` and the raw results
`~/bench/results/` stay on the host. The probe calibration uses a scratch in-package test that is not
committed. It calls `sampleBaseline` for N windows of S seconds and prints each window's quantiles.
