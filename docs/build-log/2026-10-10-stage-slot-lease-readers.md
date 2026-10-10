# V1-0467, V1-0949: the unassigned stage slot in lease and detached-run tests

Date: 2026-10-10

## Question

V1-0467 (`TestCALV0019_RacingLeaseVerbsLeaveOneConsistentHead`, racer 2 refused
`MALFORMED: staging/a03: unassigned stage slot` at 30e306ec) and V1-0949
(`TestATRV0008_DetachedRunSurvivesItsLauncher`, one `unassigned stage slot` failure at 5d1f7167)
were filed before V1-0825 (PR #734, `CTS-V0-008`, decision 0490). This entry asks whether both are
that race: an unlocked reader that sees a live writer's descriptor-less staging slots and refuses
them at once.

## Reader paths

Only `readStage` in `internal/tasks/journal/stage_read.go` and the archive export's stage read
produce this refusal. Every journal entry point reaches `readStage` through `Reader.audit`, which
waits since #734. `AuditForWriter` is the one exception, and it hands any error back to the caller,
which then runs the complete audit. No unlocked journal reader is left without the
`CTS-V0-008` wait.

- V1-0467: the race list at 30e306ec is renew, submit, gate, complete and so on. "Racer 2" is the
  zero-based index of `gate-a`, not `submit-a` as the V1-0825 build log said. `GateRun` reads the
  attempt before taking the lock: `unlockedAttemptProof` calls `readLeaseProof`, which calls
  `leaseAudit` and then `AuditForWrite`. That is an unlocked journal audit, and it runs while the
  other eight racers publish. `submit-a` reaches the same audit through `prepareLease`.
- V1-0949: `heartbeatAfter` polls `store.AttemptRecord`, which calls `readLeaseProof`, while the
  detached supervisor process journals heartbeats every 200 ms. The test fails on any read error.

## Evidence

Every run used `-race` test binaries on darwin arm64 with go1.27.1, 12 CPUs and `GOMAXPROCS=3`.
They ran beside other lanes' test runs, with a host load average between 20 and 211. Busy-loop
burners ran in the same shell, and a trap killed their PIDs on exit.

| Source | Test | Runs (`-count` x `-cpu 1,4`) | Burners | `unassigned stage slot` | Other failures |
| --- | --- | --- | --- | --- | --- |
| main bf04e8a0 | CAL-V0-019 racing lease verbs | 200 (100 x 2) | 4 | 0 | 0 |
| pre-#734 8b6af3ee | CAL-V0-019 racing lease verbs | 100 (50 x 2) | 2 | 0 | 0 |
| main bf04e8a0 | ATR-V0-008 detached run | 100 (50 x 2) | 2 | 0 | 5 `REDO_PENDING`, 1 `SNAPSHOT_MOVED` |
| main + heartbeat retry | ATR-V0-008 detached run | 100 (50 x 2) | 2 | 0 | 3 `REDO_PENDING` |
| pre-#734 8b6af3ee | ATR-V0-008 detached run | 50 (25 x 2) | 0 | 3 | 2 `REDO_PENDING` |

The three failures before #734 were all
`attempt_detach_test.go:179: MALFORMED: staging/a00: unassigned stage slot`, in `heartbeatAfter`.
That is the V1-0949 symptom, at the reader #734 fixed.

The new test `TestCTSV0008_UnlockedLeaseReadWaitsForStageSlot` (in `internal/tasks/store`) plants
a descriptor-less slot and calls `store.AttemptRecord`. On main, the read refuses
`MALFORMED staging/a00` only after the full two-second wait (`-race -count=3` PASS). Copied onto
8b6af3ee, it fails because the read refuses after 80 ms. The test pins the lease read, which both
tickets' failures share, to the wait. It extends `CTS-V0-008` coverage to that reader and changes no
behaviour.

The original V1-0467 evidence under `/tmp/corvint-import-scaling-20260928` no longer exists, so
the ticket body is the only record of that failure.

## Disposition

- V1-0949: the stage-slot failure is the V1-0825 race. It reproduced before #734 (3 of 50 runs)
  and not after (0 of 200 runs). `heartbeatAfter` now also retries a retryable `SNAPSHOT_MOVED`,
  which a loaded host produced once in 100 runs. A separate failure remains: `run --attach` reads
  the head with a bare `snapshot.Probe`, which has no `CTS-V0-006` patience, so it refuses
  `REDO_PENDING` at once while the supervisor is between a receipt link and its head rename (3 to 5
  in 100 runs under load). That failure is a separate finding, not this symptom.
- V1-0467: not reproduced, on main or before #734 (0 of 300 runs). The cause is attributed by code
  path to the same unlocked `readLeaseProof` audit, on the evidence above. That attribution is an
  inference, not an observed reproduction.

## Limits

`make gate` and the full suite are `NOT_RUN`. The run counts bound reproduction under this host's
load only. The two-second `CTS-V0-008` budget can still be outlasted on a host loaded heavily
enough (decision 0490, Limits).
