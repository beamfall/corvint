# 2026-10-23 Tasks Linux qualification: B3 harness, padded journal, host sleep, inotify limit

Tickets: V1-2202, V1-2204, V1-2278, V1-2137, V1-0911, V1-1569, V1-1634. Requirements: JSG-V0-020..021,
CAL-V0-360..364 (PSR-V0-215..219 unused). Base `7b3a8e55f` (origin/main). Go 1.27.2 fetched with
`GOTOOLCHAIN=go1.27.2` (the image has go1.24.7).

## Host

Linux 6.18.44-fc-v153 x86_64, a Firecracker VM, 4 vCPU (Intel Xeon @ 2.80 GHz), 16 GiB, ext4 on
/dev/vda, `fs.inotify.max_user_watches` 130016, `max_user_instances` 128. `/sys/power/state` is empty
and there is no RTC: a real suspend is impossible (NOT_RUN). The B3 runs were pinned to CPUs 0-2
(`taskset`, GOMAXPROCS 3), so the report says `cpuCount` 3 and labels against load 3; development ran
niced on CPU 3. Loads are the 1/5/15-minute `/proc/loadavg` values before and after each series.

## V1-2202 (JSG-V0-011): already delivered by S6a; before and after

S6a (commit 1521b238, ancestor of origin/main: `internal/tasks/journal/resume.go` lines 157-175 and
304-315, tests `TestJSGV0011_SealedStoreWidenAndReapResume`, `TestJSGV0011_ResumeDeclinesRewrittenSegment`
in `internal/tasks/store/lease_resume_sealed_test.go`) already walks and resumes segmented stores. No new
code was written for this ticket. B3 `moved` series (a dispatcher write after every complete preparation
audit), 10,000 receipts, Linux, S6a reverted (`git revert 1521b238` source files) against main:

| code | widen | reap |
|---|---|---|
| S6a reverted (4 calls each) | 4/4 `SNAPSHOT_MOVED`, p50 97.5 s, 16 audits per call | 4/4 `SNAPSHOT_MOVED`, p50 96.4 s, 16 audits per call |
| main (20 calls each) | 20/20 ok, p50 6.9 s, 1 audit and 1 resumed write per call | 20/20 ok, p50 7.0 s |

B2 (`TestCALV0263_LeaseGateLoad`, 40,000 receipts, 15 writers, 300 s, 3 s think, main): 399 calls, no error
code (0 `SNAPSHOT_MOVED`, 0 `LOCK_TIMEOUT`); widen 4/4 ok p50 66 s, reap 4/4 ok p50 88 s, heartbeat p50 428
ms p99 22.7 s. UNQUALIFIED: load 14.4 on 4 CPUs. T2's p99 targets are not met on this host.

Open (S6b, not attempted): T3 is 2 s; single-writer widen at 40,000 receipts takes 36 s (below). S6b needs a
design decision (widen and reap are off the writer-checkpoint route, `writerLeaseVerb`).

## V1-2204, V1-2278 (JSG-V0-020, JSG-V0-021): B3 harness

`TestJSGV0020_SingleWriterHarness` (opt-in, `CORVINT_TASKS_B3`), 20 calls per verb, main, ms, nearest rank.
Run: `CORVINT_TASKS_B3=out.json CORVINT_TASKS_B3_STORE=dir go test -run TestJSGV0020_SingleWriterHarness`.
The 10,000 and 40,000 `quiet` and `moved` series all ran at load below 3 (label QUALIFIED).

| receipts | series | verb | n | errors | min | p50 | p95 | p99 | max | complete audits | resumed | load before | load after | label |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 10000 | quiet | heartbeat | 20 | 0 | 47 | 58 | 77 | 91 | 91 | 0 | 0 | 2.16 1.42 1.26 | 1.75 1.58 1.38 | QUALIFIED |
| 10000 | quiet | renew | 20 | 0 | 61 | 76 | 94 | 104 | 104 | 0 | 0 | 2.16 1.42 1.26 | 1.75 1.58 1.38 | QUALIFIED |
| 10000 | quiet | widen | 20 | 0 | 9356 | 9708 | 9937 | 10008 | 10008 | 20 | 0 | 2.16 1.42 1.26 | 1.75 1.58 1.38 | QUALIFIED |
| 10000 | quiet | reap | 20 | 0 | 8795 | 9302 | 9686 | 9922 | 9922 | 20 | 0 | 2.16 1.42 1.26 | 1.75 1.58 1.38 | QUALIFIED |
| 10000 | moved | heartbeat | 20 | 0 | 49 | 55 | 66 | 66 | 66 | 0 | 0 | 1.75 1.58 1.38 | 1.44 1.53 1.42 | QUALIFIED |
| 10000 | moved | renew | 20 | 0 | 62 | 67 | 78 | 82 | 82 | 0 | 0 | 1.75 1.58 1.38 | 1.44 1.53 1.42 | QUALIFIED |
| 10000 | moved | widen | 20 | 0 | 6545 | 6906 | 7422 | 7812 | 7812 | 20 | 20 | 1.75 1.58 1.38 | 1.44 1.53 1.42 | QUALIFIED |
| 10000 | moved | reap | 20 | 0 | 6527 | 6963 | 7316 | 7337 | 7337 | 20 | 20 | 1.75 1.58 1.38 | 1.44 1.53 1.42 | QUALIFIED |
| 40000 | quiet | heartbeat | 20 | 0 | 57 | 79 | 97 | 97 | 97 | 0 | 0 | 1.64 1.60 1.47 | 1.20 1.28 1.35 | QUALIFIED |
| 40000 | quiet | renew | 20 | 0 | 68 | 84 | 106 | 113 | 113 | 0 | 0 | 1.64 1.60 1.47 | 1.20 1.28 1.35 | QUALIFIED |
| 40000 | quiet | widen | 20 | 0 | 34922 | 36179 | 37436 | 38700 | 38700 | 20 | 0 | 1.64 1.60 1.47 | 1.20 1.28 1.35 | QUALIFIED |
| 40000 | quiet | reap | 20 | 0 | 33726 | 34596 | 36854 | 37256 | 37256 | 20 | 0 | 1.64 1.60 1.47 | 1.20 1.28 1.35 | QUALIFIED |
| 40000 | moved | heartbeat | 20 | 0 | 59 | 72 | 86 | 88 | 88 | 0 | 0 | 1.18 1.27 1.34 | 1.56 1.41 1.34 | QUALIFIED |
| 40000 | moved | renew | 20 | 0 | 74 | 84 | 98 | 99 | 99 | 0 | 0 | 1.18 1.27 1.34 | 1.56 1.41 1.34 | QUALIFIED |
| 40000 | moved | widen | 20 | 0 | 24459 | 25411 | 26249 | 27103 | 27103 | 20 | 20 | 1.18 1.27 1.34 | 1.56 1.41 1.34 | QUALIFIED |
| 40000 | moved | reap | 20 | 0 | 24053 | 24924 | 25747 | 25780 | 25780 | 20 | 20 | 1.18 1.27 1.34 | 1.56 1.41 1.34 | QUALIFIED |

Median phases (ms) of widen and reap:

| receipts | series | verb | snapshotRead | validation | lockHold | journalWrite | fsync |
|---|---|---|---|---|---|---|---|
| 10000 | quiet | widen | 7577 | 1890 | 198 | 23 | 11 |
| 10000 | quiet | reap | 7268 | 1817 | 206 | 20 | 10 |
| 10000 | moved | widen | 6581 | 14 | 377 | 24 | 9 |
| 10000 | moved | reap | 6626 | 20 | 390 | 22 | 9 |
| 40000 | quiet | widen | 27491 | 7894 | 690 | 19 | 11 |
| 40000 | quiet | reap | 26531 | 7447 | 665 | 18 | 10 |
| 40000 | moved | widen | 24551 | 13 | 1154 | 19 | 9 |
| 40000 | moved | reap | 24079 | 18 | 1136 | 18 | 8 |

Reading: at 40,000 receipts a quiet widen is 27.5 s of complete audit, 7.9 s of model and capacity
measurement (`transaction.CheckCapacity`, 20 s of a 99 s CPU profile) and 0.7 s under the lock. The resumed
write skips the 7.9 s. The audit alone misses T3 by 12x. An earlier run (before the harness clock fix) showed
one `OUTCOME_BLOCKED` reap per series from a virtual clock behind the wall clock; fixed in the harness.

## V1-2137 (CAL-V0-360): padded-journal builder

`fixture.PadLiveness` appends N synthetic heartbeat receipts to a store with live attempts. A first version
spaced whole receipts by attempt-own count and stopped the doctor scan at 24 receipts (`no retained record
within 4096 receipts`); compaction spaces by journal sequence (`c.seq-whole >= maxRun`), now matched, with a
mutation check in `TestCALV0360_PaddedLivenessStoreAuditsAndWalks`. At 30,000 padded receipts (24 attempts,
29,280 elided, head 30,073; Linux, the store took 54.7 s to build including 24 CLI claims; `receipt audit`
41.0 s) the doctor scans its 4,096-receipt bound; best of five interleaved runs, host load about 2-3 from the
B3 run on other CPUs: unshared walks 38.7 s wall / 38.2 s CPU, shared walks (TQD-V0-045) 25.8 s / 25.5 s.
About 24 s of each is the fixed snapshot read.

## V1-1569 (PSR-V0-055) and V1-0911 (CAL-V0-363, CAL-V0-364): Linux findings

`TestPSRV0055_PoolAcquireLoad` at 45,000 unsealed receipts, 8 acquirers, a writer every 3 s, 600 s:

| run | result |
|---|---|
| main | 154 claims completed, 520 `MALFORMED`, 144 acquire `MALFORMED`; detail `no space left on device` |
| after the ENOSPC fallback only | 82 of 223 claims completed; 141 `UNSUPPORTED_FILESYSTEM` naming the inotify limit, down to `/tmp` itself |
| after the per-guard ceiling (CAL-V0-364) | 20 of 20 claim, acquire, pool-release, confirm-safe and release cycles completed, no error code, acquire p50 81.7 s and max 86.9 s (under the 2-minute bound of PSR-V0-055); 762 s; load 1.72 before and 10.85 after on 4 CPUs, UNQUALIFIED |

Cause: on Linux the change guard put one inotify watch on every file (about 95,000 for this store). The
allowance is per user (130,016 here, 8,192 on older kernels), so concurrent writers spend it all. Disk and
inode use were checked and were not the cause (`df`, `df -i`). Darwin avoids this with a 16-descriptor
budget. Fix (CAL-V0-364): directories first, then at most min(4096, allowance/32) files per guard with
inotify, the rest tracked by stat and swept, and an ENOSPC file falls back the same way. Tests fail on the old
code (`TestCALV0364_ChangeGuardUnderTheRealInotifyLimit` at a limit of 26 watches: `registration ...: no
space left on device`) and pass on the new; mutation checks for the directories-first order. 

Other Linux results: `TestCALV0136_*`, `TestCALV0056_*`, `TestV11091*`, `TestSERVICE500_Linux*`,
`TestCALV0125/0351` and `TestCALV0220` pass on real Linux; new `TestCALV0363_ProcRowsAgreeWithPsOnLinux`
agrees with `ps` and fails on two parser mutations.

## V1-1634 (CAL-V0-361, CAL-V0-362, ATR-V0-064)

New tests: lease-keeper idle check across a three-hour gap (fails with the `slept` term removed), exit
recovery at a wake, unowned expired lease reaped at a wake, store claim and `reap` after a wake (pinned as a
limit, CAL-V0-361). `TestCALV0362_GoMonotonicClockIsNotBoottime` runs in a time namespace where CLOCK_BOOTTIME
is 3 h ahead: CLOCK_MONOTONIC 4952401678949 ns, CLOCK_BOOTTIME 15752401680433 ns, `runtime.nanotime`
4952401680750 ns. The opt-in wall-clock step test compiles and was NOT run (it moves the host clock, which
would have corrupted a concurrent measurement).

## NOT_RUN

Real suspend (no suspend state); macOS; the wall-clock step test; idle CPU per minute on stores A and D and
the supervisor host-wait run (`TestCALV0137_HostWaitIdleCPU` and `tools`-free commands exist, 3 s smoke only:
0.23 CPU s/min normal, 1.7 detached, including startup); `ReadDir` d_type benchmark and strace counts;
`corvint affected`, `dogfood-*`, `heavy.sh`, `corvint-tasks` writes. Budget ran out.
