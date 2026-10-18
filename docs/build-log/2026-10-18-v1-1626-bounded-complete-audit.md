# V1-1626: bounded complete audit on a moving journal (issue 1063)

Owner input: [issue 1063](https://github.com/beamfall/corvint/issues/1063), P1. `pool acquire`
stayed CPU-bound for 38 minutes or more on a 47,400-receipt journal (455 MB) whose head moved
every minute or two under lease renewals and heartbeats; its profile was dominated by the ModeFull
walk under `prepareLease → AuditForWriteObserved → audit → auditAttempt → walk`. After V1-1617 the
orchestrator decided (2026-10-17) to keep the CAL-V0-116 rewritten-prefix detection and fix the
speed through the root cause. PSR-V0-150..153 were added to
`docs/specs/corvint-tasks-pool-safe-reuse-v0.md` under the owner's standing auto-accept of lane
requirements (2026-10-11). No CAL-V0 ID was added. Delivery stays experimental.

## Measurement before the change (acceptance criterion 1)

Host: Apple M2 Max (Mac14,13), 12 CPUs, 64 GB, Darwin 25.6.0, Go 1.27.2, corvint 1.0.0-rc.3
(build 634). Host load 7.7 to 9.0 before and after the run (other lanes were building).

Harness: opt-in `TestPSRV0055_PoolAcquireLoad` with per-round rows (added in this change),
`_RECEIPTS=47000 _ACQUIRERS=1 _SECONDS=200 _WRITER_MS=60000 _WRITER_KIND=lease _CALL_SECONDS=150
_GRACE_SECONDS=150`: one acquirer, a dispatcher that claims and then renews or heartbeats every
60 s, no detach. Fixture build 149 s; run 4 min 16 s; final head 47,018.

| call | audit rows | rounds / resumed | wall |
| --- | --- | --- | --- |
| claim (no checkpoint yet) | FULL 34.6 s | 1 / 0 | 52.6 s |
| pool acquire 1 | COMPLETE_PREFIX 34.9 s | 2 / 1 | 37.2 s |
| pool release 1 | COMPLETE_PREFIX 56.1 s | 2 / 1 | 60.1 s |
| pool acquire 2 | COMPLETE_PREFIX 54.4 s | 2 / 1 | 57.7 s |
| pool release 2 | FULL 37.5 s | 1 / 0 | 48.3 s |
| heartbeat, renew, claim (writer route) | none | 1 / 0 | 0.1 to 0.2 s |

Report retained at `/private/tmp/cem10-build/gh-acquire-scratch/run1-before.json` (outside the
repository). The ticket's hypothesis (one failed resume-base build sets a command-wide no-resume
flag, after which every round runs a complete audit) is refuted for the plain synthetic store:
every acquire resumed in round 2 after one prefix walk, with no `fast.declined` stage, and reached
`ACQUIRED` in 37 to 58 s, inside the 150 s bound of acceptance criterion 2. The decline is
therefore a precondition of the reported run, not the rule; which decline class the user's store
hit stays NOT_OBSERVED (V1-1650 removed one such class, note receipts, the day before).

## Root cause, confirmed on the source

With `noResume` set (a prefix walk declined, or `ResumeBase` failed), every later round called
`prepareLease` with `resumable=false`:

1. its complete audit was a plain `AuditForWriteObserved`, up to four complete walks when the head
   moves under each (`audit()` retries `SNAPSHOT_MOVED` four times with no early stop; only the
   prefix-fallback attempt had `abortMoved`);
2. a round whose audit returned `SNAPSHOT_MOVED` then fell through to `leaseAuditObserved`, which
   audited again (up to four more walks) and failed on `inventory differs from audit`, because its
   key was the head the round read at its start.

Sixteen rounds of up to eight walks at the measured 35 to 60 s per walk at 47k receipts is 75 to
128 minutes at the cap, and 38 minutes at the observed rate. `TestPSRV0150_DeclinedResumeWalksOncePerRound`
forces the decline and a dispatcher write during every complete attempt; on the unmodified base it
fails with 16 rounds of 4 attempts (the fold that starts each complete attempt is counted).

## Design

- `journal.Reader.StopOnMove`: the non-fallback complete audit of `AuditForWriteObserved` becomes one
  attempt with `abortMoved` (the prefix-fallback attempt's existing early stop, at 512-receipt
  boundaries). `prepareLease` sets it for journals at or above the writer-checkpoint floor on
  platforms whose audits resume, the same condition as the prefix fallback; below it nothing changes.
- `prepareLease` ends a round whose first complete audit returned `SNAPSHOT_MOVED` instead of
  running the guarded audit against a lost head (the fallback path already did).
- `PhysicalObservation.Walks` (`journal.WalkStats`) counts complete and prefix walks, how many
  moved, and their time; `LeaseTiming` sums them with resumed walks and resume declines
  (PSR-V0-151); the detached `progress` member reports nine more decimal strings (PSR-V0-152).
  `--timing` is unchanged.
- The harness reports per-round rows (each audit's mode or code and wall time; each progress
  report, now with the split).

The verdicts are unchanged: a settled attempt is the ordinary complete audit, and PSR-V0-153 shows
a receipt rewritten during the bounded attempt, with its stamps restored, is refused by the next
round and never commits.

## Measurement after the change

Same host, harness and knobs, run while the store package's own tests ran beside it: host load
8.6 to 15.6 before and 10.4 to 14.9 after. Fixture build 176 s; run 4 min 7 s; final head 47,018;
no worker failure. Report retained at `/private/tmp/cem10-build/gh-acquire-scratch/run2-after.json`.

| call | audit rows (walk split from the progress row) | rounds / resumed | wall |
| --- | --- | --- | --- |
| claim (no checkpoint yet) | FULL 44.4 s: complete 1 (moved 0) | 2 / 0 | 56.0 s |
| pool acquire 1 | FULL 37.8 s: complete 1 (moved 0) | 2 / 1 | 41.7 s |
| pool release 1 | COMPLETE_PREFIX 48.9 s: complete 1 (moved 1, 16.7 s), prefix 1 (32.3 s) | 2 / 1 | 53.8 s |
| pool acquire 2 | COMPLETE_PREFIX 56.5 s: complete 1 (moved 1, 24.3 s), prefix 1 (32.2 s) | 2 / 1 | 59.7 s |
| pool release 2 | FULL 28.7 s: complete 1 (moved 0) | 1 / 0 | 35.2 s |
| heartbeat, renew, release, confirm-safe, claim (writer route) | none | 1 / 0 | 0.1 to 0.4 s |

Both acquires reached `ACQUIRED` inside the 150 s bound with the head moving every minute
(acceptance criterion 2), 41.7 s and 59.7 s, the same shape as before on a busier host: one
complete walk, a prefix walk when that one moved, then the resumed route. The per-walk cost is the
same as before (walk time is the residual below); what the change removes is the repeated walks of
a declined command, which the synthetic store does not reach and `TestPSRV0150` forces. Neither
run observed a decline (`declines=0` in every progress row), so the user's 38-minute shape remains
reproduced only by the forced test, not by this harness.

## Residuals

- The walk's `openat` cost: in the 47k profile, 17.4 s of a 31.5 s walk was `openat` inside
  `nativeRead.Read` (94k files, about 185 µs each), while an isolated microbenchmark of
  `safeopen.PinDir` + `InDir` over 20k receipts measured 18 µs per open. The faster prefix walk the
  issue asks for is still a non-goal of PSR; the gap is unexplained (vnode cache pressure at 94k
  files is a hypothesis) and is left for a ticket.
- `pool release` is not on the writer route (`writerRouteReady`), so each release costs a complete
  or prefix walk (56 s above). A ticket candidate, out of this lane's scope.
- A refusal is derived twice in its round (preparation audit, then the guarded audit); bounded by
  the refusal, unchanged here.

## Review repair (Codex round on PR #1089 at 34b9fcf9c)

Two P2 findings, both fixed in this lane under the same standing auto-accept (2026-10-11); the
PSR-V0-151 and PSR-V0-152 text was amended in place.

- Resumed walks were counted per resumed-route call. A route that declined before
  `AuditForResume` (writer token invalidated after the base was captured) still charged one, and a
  resumed audit's `SNAPSHOT_MOVED` retry charged one for two attempts. `AuditForResume` now
  records each attempt in `PhysicalObservation.Walks` (`Resumed`, `ResumedTime`), `observeWriter`
  hands the walks back, and `leaseWriter` charges them through `LeaseTiming.walked`. Evidence:
  `TestPSRV0151_DeclinedResumeChargesNoResumedWalk` (store) and
  `TestPSRV0151_ResumedWalksCountEachAttempt` (journal).
- The walk split had been added to `progress.json`, which the base revision reads with unknown
  members refused, so an older `--attach` lost even the PSR-V0-112 counters (`progress: null`).
  `progress.json` is back to its PSR-V0-112 shape; the full report goes to a sibling
  `progress-walks.json`, written first and used only when its PSR-V0-112 members match.
  Evidence: `TestPSRV0112_PendingReportsProgress` decodes the new child's `progress.json`
  strictly with the base revision's record struct, and reads a record without the sibling.
- The same push also gives `lease_bounded_audit_test.go` the `darwin || linux` constraint of the
  resume fixture it uses; `GOOS=windows go vet` failed on the undefined helpers in CI
  (AFP-V0-042).
