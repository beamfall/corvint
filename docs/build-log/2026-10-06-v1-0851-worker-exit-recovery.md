# 2026-10-06: V1-0851 dispatcher worker-exit release or reap

## Intent

Owner request [issue 622](https://github.com/beamfall/corvint/issues/622), native ticket V1-0851.
When a worker that the dispatcher launched exits and leaves its attempt RUNNING, the dispatcher
made one `HANDOFF` release attempt. If that release was refused, it reported `needs-owner` and left
the attempt and its pool member held, even after the lease had expired. The program `flowproof`
saw six such cases in about 24 hours. The owner asked for three things:

- a bounded release retry with backoff;
- a reap once the lease has expired;
- `needs-owner` only when both fail.

All of it sits behind a dispatcher switch that is on by default.

## Change

- `internal/tasks/dispatch/config.go`: adds the optional `heal.exitRecovery` (default true) and
  `Heal.ExitRecoveryOn`, which applies only with `heal.handoff`.
- `internal/tasks/dispatch/loop.go`: when a hand-off is refused, `heal` now records an in-memory
  `exitRecovery` keyed by attempt ID. That record pins the generation and holder it observed.
  `recoverExits` runs first in each later heal pass and handles each case:
  - An attempt that is absent or no longer live is resolved as `ALREADY_ENDED`.
  - An attempt whose generation or holder changed is resolved as `SUPERSEDED` and never touched.
  - An expired lease is reaped, at most 3 times.
  - Otherwise the release is retried, at most 3 times including the first.
  Writes back off at `tickSeconds` doubled per write, capped at 5 minutes, using `d.Now`. Each retry
  has a distinct deterministic request ID. A single `needs-owner` is emitted only when the reaps are
  exhausted, or when the releases are exhausted and the attempt has no lease. A lease that is already
  expired when the worker ends is reaped in the same tick.
- Spec `docs/specs/corvint-tasks-agent-leases-v0.md`:
  - new `CAL-V0-104` (V1-0851 section);
  - CAL-V0-056's refused-handoff sentence now defers to it;
  - status, inputs, slice table and traceability rows updated.
- `docs/TASKS-SUPERVISION.md` gains one paragraph.

## Decisions

- No new event kinds; the closed CAL-V0-058 vocabulary is unchanged. Resolution reuses `handoff`
  and `reaped`, with a `recovery` detail of `RELEASED`, `REAPED`, `ALREADY_ENDED` or `SUPERSEDED`.
  Retries reuse `handoff-refused`, and reap retries reuse `alert`.
- Recovery state is not persisted, so the ledger schema is unchanged. After a restart, `heal.reap`
  still reaps expired leases of holders that are not running workers. Release retries are lost on
  restart, and that limit is recorded as a non-goal.
- A reap that reports "the mutation changed nothing" returns OK from the CLI queue. It is treated as
  resolved, which matches the issue's FAILED/FENCED case.
- Fencing: only attempts held by a worker this dispatcher launched or adopted are recovered, and
  only at the observed generation and holder.

## Evidence

`GOMAXPROCS=2 GOTOOLCHAIN=local go test -p 1 -count=1 -timeout 30m -run 'TestCALV0104|TestCALV0056|TestCALV0052|TestCALV0058' ./internal/tasks/dispatch/`
passed. It covers these tests:

- `TestCALV0104_ExitRecoveryRetriesHandoffOnLiveLease`
- `TestCALV0104_ExitRecoveryReapsExpiredLease` (expired at exit; expiry after bounded releases)
- `TestCALV0104_ExitRecoveryResolvesFencedAttempt` (already ended; generation changed)
- `TestCALV0104_ExitRecoveryNeedsOwnerOnlyWhenBothFail`
- `TestCALV0104_ExitRecoverySwitchOff`
- `TestCALV0104_ExitRecoveryConfig`
- the existing CAL-V0-056 handoff and cancellation tests

Vet and the spec checks are listed in the commit's lane report.

## Not run

The following were not run: `make gate`, `go test ./...`, the full `cmd/corvint` and package suites,
and live dispatcher qualification against a native store with a real host. The native
release/reap paths are exercised only through a fake queue here.

## Rollback

Set `heal.exitRecovery: false` to restore CAL-V0-056's single hand-off followed by `needs-owner`, or
revert the commit. No store, ledger or wire bytes change. An older binary refuses a config naming
`exitRecovery`, so remove the member before downgrading.

## Review repair

An independent review found that a worker re-reported as ended restarted its recovery. This happens
when the observation after heal fails, so `finish` never runs and the worker is reported again on
the next tick. The hand-off loop then released again and overwrote the recovery with `releases:1`,
which reset the tries and backoff and reused refused request IDs. Exhaustion and `needs-owner`
could therefore never be reached. The finding was confirmed: the new test fails without the fix,
with unbounded releases.

The fix has two parts:

- The hand-off loop skips any attempt that recovery handled this pass or that has a recovery
  recorded.
- An exhausted recovery is now kept, marked and writing nothing, until the attempt ends or is
  superseded, so it is never restarted or reported twice.

CAL-V0-104 gains one sentence to say this, and `TestCALV0104_ExitRecoverySurvivesFailedPostHealObservation`
is the witness for it.
