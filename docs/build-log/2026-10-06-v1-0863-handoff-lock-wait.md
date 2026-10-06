# 2026-10-06: V1-0863 HANDOFF release lock wait

## Intent

Ticket V1-0863, GitHub [beamfall/corvint#637](https://github.com/beamfall/corvint/issues/637). With 6 to
10 concurrent writers, `corvint-tasks release --reason HANDOFF --evidence ...` returned the retryable
`LOCK_TIMEOUT` on `taskman.prepare.lock` after the fixed 30-second wait. The issue asks for a
caller-chosen wait and safe same-request resubmission. It also offers a third option: treat a plain
release as a HANDOFF when evidence exists.

## Change

- Spec `docs/specs/corvint-tasks-agent-leases-v0.md` adds CAL-V0-109..111 in a V1-0863 subsection,
  with non-goals, failure modes, acceptance evidence and rollback. It also adds a slices row,
  traceability rows, the issue-637 input, a CAL-V0-109 exception to the issue-494 30-second wait
  sentence, and the delivery status (mirrored in `docs/specs/README.md` and `INDEX.json`).
  `docs/TASKS-EXTERNAL-AGENTS.md` documents the flag under `LOCK_TIMEOUT`.
- `authority.LockOptions.CallerWait` is an explicit caller bound clamped to `MaxCallerLockWait`
  (300 s). Zero keeps `EffectiveWait`, so the frozen 30-second §1 default and maximum still apply to
  every other caller. `AcquirePreparation` and `AcquireLock` spend the chosen bound across the same
  phases, without restarting.
- `store.WithLeaseLockWait` carries the bound in the context, like `--timing`. `leaseWrite` passes it
  to the one preparation admission and to the orphan-stage cleanup lock, and `commitLease` passes
  it to each writer-lock round. It is not
  in the request, its digest or any receipt.
- `release` and `attempt heartbeat` accept `--lock-wait SECONDS`: canonical whole seconds 1..300.
  Any other value, a repeated flag, or the flag on another lease verb refuses MALFORMED before any
  store access. The usage and help text are updated in `command_help.go`.

## Decisions

- **Same-request replay already held, so CAL-V0-110 adds tests, not code.** Both lock refusals
  happen before any write, and the wait is outside the request digest. So a same-ID resubmission
  commits once, and later ones replay the committed receipt sequence with no write and no retry
  charge. The CLI replay envelope reports `replayed: true` with an empty `receipt` (existing
  behaviour). The store test pins the replayed `ReceiptSeq` to the committed one.
- **Option 3 is rejected (CAL-V0-111).** Turning a plain release into a HANDOFF would silently change
  the retry accounting the caller asked for, based on a heuristic. A maintained test shows that a
  plain release after a timed-out HANDOFF records no evidence and is charged.
- **One bound for both locks.** Preparation admission alone would still leave the writer-lock rounds
  at 30 s under the same contention. The ceiling of 300 s keeps a slot holder bounded; values above
  it refuse rather than clamp silently.
- **Independent review (Codex, read-only) of `2dc319db` found one defect, fixed:**
  - `clearLeaseOrphans` (`internal/tasks/store/lease_recovery.go`) took the writer lock with default
    options whenever `staging/` was not empty. So `--lock-wait 1` could still wait 30 s, and
    `--lock-wait 60` could fail at 30 s. It now spends the caller's bound.
    `TestCALV0109_OrphanCleanupSpendsCallerWait` covers it: with the fix reverted, it failed with
    "lock acquisition exceeded 30s after 30.0s".
  - Codex found no replay or concurrency defect. Its sandbox could not run the Go tests.
  - Checked and left unchanged: `settleLease` and `probeLeaseRecovery` take default locks but serve
    the gate and pool commands, not `store.Lease`. Those verbs do not take `--lock-wait`.

## Evidence

All runs were on Darwin with `GOMAXPROCS=2 GOTOOLCHAIN=local go test -p 1 -timeout 30m`.

- `-count=1` PASSED:
  - `TestCALV0109_CallerWaitBound` and `TestCALV0109_CallerWaitOutlastsDefault` (authority). In the
    second, both locks are held for 31 s; caller waiters of 40 s acquire after the default, while
    default waiters beside them refuse LOCK_TIMEOUT.
  - `TestCALV0110_HandoffReleaseReplaysAfterLockTimeout` and
    `TestCALV0109_OrphanCleanupSpendsCallerWait` (store; the second was added after review).
  - `TestCALV0109_LockWaitFlag`, `TestCALV0109_LockWaitBoundsContendedRelease`,
    `TestCALV0110_SameRequestHandoffReplayAfterLockTimeout` and
    `TestCALV0111_PlainReleaseAfterTimedOutHandoffIsCharged` (cli).
- `-count=3 -run 'TestCALV0109_|TestCALV0110_|TestCALV0111_'` over authority, store and cli PASSED,
  and the store package again after the review fix.
- `-count=1 -run 'Help|Usage|CALV00(26|69|78|81|95)|CALV0044|Lease|Release|Heartbeat|Preparation|Lock|GH494'`
  over the same three packages PASSED.
- Mutation check, reverted afterwards: dropping `CallerWait` from `lease_write.go` failed both CLI
  contention tests, with "LOCK_TIMEOUT after 30.0s with --lock-wait 1s". The store test now bounds its
  elapsed time the same way.
- `go vet` PASSED on the three packages for GOOS darwin, linux and windows, and `gofmt -l` was clean.
- These checks PASSED: `make spec-requirements-check requirement-definitions-check
  traceability-tests-check decision-numbers-check line-citations-check error-code-ownership-check
  unbounded-readers-check use-case-receipts-check diagnostic-coverage-check`.

## Not run

- NOT_RUN: live fleet qualification under 6 to 10 concurrent writers, Linux and Windows execution
  (vet only), `make gate`, the full package suites, CEM dogfood binding, and native completion of
  V1-0863.

## Rollback

Revert the commit. No stored state, request digest or receipt shape changes. Callers that pass
`--lock-wait` then refuse MALFORMED as an unknown flag.
