# 2026-10-24: a Pi writer dispatch gets its record bound before KILL; REDO_PENDING is a pending operation (V1-2573, PWV-V0-018, PWV-V0-019)

## Decision

The Pi adapter's process helper gave every interrupted command 100 ms between TERM and KILL. A native
Tasks writer that is interrupted records its result under its own 30-second write bound (CAL-V0-319),
so a KILL 100 ms after TERM could land after the receipt commit and before the head apply, the last
§5.2 effect. The store was then `REDO_PENDING`. Reads wait that state out and refuse; they never redo
(TM-V0-008, invariant 4). Every reconciliation read then refused, and the wrapper reported
`reconciliation-unavailable`. That is the V1-2506 item 3 residue: 5 of 12 Pi runs under load.

- PWV-V0-018 (proposed): `createCommandRunner.run` takes a per-call `graceMs`, an integer from 100 to
  60,000 (default 100). The Tasks writer dispatch passes 40,000. That covers the native gate's 5 s
  `gateWaitDelay`, the CAL-V0-319 30 s record bound, and a 5 s margin for the post-run observations,
  which are not bounded. Leader exit ends the grace, and the process group is swept 100 ms later.
  Reads keep 100 ms.
- PWV-V0-019 (proposed): when every failing reconciliation read refuses with native code
  `REDO_PENDING`, the service returns `pending-operation` (mutation `not-attempted`, receipts retained,
  no dispatch). The next native writer redoes the receipt. Other refusals stay `reconciliation-unavailable`.

The IDs are PWV rather than CTS or CAL because both changes are in the Pi adapter
(`integrations/pi`), which `pi-workflow-v0.md` governs. No open PR or `origin/main` uses
PWV-V0-018 or PWV-V0-019. Rejected: a native read-through redo, which would make a read mutate and
needs an owner decision on invariant 4.

## Evidence (darwin/arm64, node, native `corvint-tasks` built from this tree with go1.27.3)

- Failing first: `process.test.mjs` PWV-V0-018 (expected `recorded`, got empty stdout) and
  `tasks.test.mjs` PWV-V0-018/019 (expected `graceMs` 30000). Both then pass.
- Codex review (one round) found two P2 issues, and both are fixed with failing-first tests:
  - A leader exit that arrived after a graced sweep scheduled a second SIGKILL to the released group.
    `process.test.mjs` now checks for "late leader exit".
  - A 30 s grace did not cover the native cleanup that runs before the 30 s record bound starts. The
    grace is now 40,000 ms, and `tasks.test.mjs` expects 40000.
  - The five-run result below used the 30,000 ms build. The 40,000 ms change only lengthens the grace.
- Failing first with the real native binary: `tasks-host.test.mjs` "PWV-V0-019 native gate run killed
  after its receipt commit". The test runs a real gate run, then restores the head it replaced, which
  leaves the state a SIGKILL leaves just before the head rename. Native `receipt audit` refuses
  `REDO_PENDING`. Without the tasks.js branch the result is `reconciliation-unavailable`. With it, the
  result is `pending-operation`, with no writer dispatched. A native `renew` then redoes the receipt
  (exit 0, `OK`), and audit and ticket reads succeed.
- V1-2506 item 3: the actual-native interruption test and the PWV-V0-019 test passed 5 of 5
  consecutive runs (2/2 each, 38 to 44 s). Host 1-minute load was 14.06 before, then 32.98, 19.78,
  16.12, 150.67 and 93.66 after each run (12 cores). MET.
- `make host-adapter-test`: 47/47 and 129/129 pass. `tasks-host.test.mjs` in full: 5/5.
  `pi-protected`: 19 pass and 2 fail, the same as before the change (`build/node` absent: the SEA image
  is not built here).

## Host qualifications left NEEDS-HOST by #1704 (one bounded foreground run each)

- V1-2449, darwin/arm64, 12 cores, go1.27.3, `nice -n 15`:
  - Ran `TestCALV0019_RacingLeaseVerbsLeaveOneConsistentHead -count=50`: 50/50 pass, 0 `SNAPSHOT_MOVED`.
  - Per-run p50 1.95 s, p99 2.44 s (n=50).
  - Load 39.70 before, 12.63 after.
  - MET for the host run. The fail-without-fix half rests on #1551/#1673, as recorded in #1704.
- V1-2482, linux/amd64 WSL2 6.18.33.2, 32 threads, go1.27.3, `max_user_watches` 524288:
  - Ran `TestPSRV0055_PoolAcquireLoad` with 45,000 receipts, 8 acquirers, 240 s and 60 s grace.
  - It passed with 0 ENOSPC and no error codes; every verb completed 39/39 and dispatch 62/62.
  - Acquire wall p50 24.7 s, p99 28.8 s (n=39).
  - Load 7.50 before, 17.26 after.
  - The peak watch count was NOT_OBSERVED: the lane forbids the sampling loop that would watch it.
  - Derived from `TestPSRV0222_ChangeGuardWatchCountIsBoundedByDirectories` on the same host: 9 watches
    per guard over 11 files and 9 over 5,001 files. The peak is therefore about 9 per concurrent guard
    (an inference).
  - NOT MET as a qualification: the load is above the quiet bound of 2, and the peak was not retained.
- V1-2202, same WSL2 host:
  - B2 (`TestCALV0263_LeaseGateLoad`, 40,000 receipts, 15 writers, 240 s, 3 s think): 1,581 calls,
    0 error codes, `LOCK_TIMEOUT=0`, 0 `SNAPSHOT_MOVED`. Widen 26/26 and reap 13/13 completed, which
    meets acceptance criterion 1.
  - T2 is NOT MET. Heartbeat p99 is 261 ms and renew p99 283 ms, against a 250 ms target. The
    widen/reap gate-hold p99 is 151/125 ms, against 50 ms.
  - The box's load after the run was 6.01, above the quiet bound of 2.
  - T3 used B3 (`TestJSGV0020_SingleWriterHarness`, 40,000 receipts, quiet series, 3 calls per verb):
    widen p50 17.1 s and max 17.4 s; reap p50 17.7 s and max 19.6 s; heartbeat and renew p50 31/30 ms.
    The harness labels it QUALIFIED (load 8.52 at or below 12). T3 (at most 2 s) is NOT MET; S6b
    remains open.

## Rollback

Drop the `graceMs` argument from the writer dispatch and the `REDO_PENDING` branch in reconciliation.
Neither persists state. A killed writer then returns to `reconciliation-unavailable` until the next
native writer redoes its receipt.
