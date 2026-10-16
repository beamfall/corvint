# V1-1460: Tasks gate timeoutSeconds bound raised to 14400 seconds

Date: 2026-10-16
Ticket: V1-1460 (bug)
Requirement: CAL-V0-255 (`docs/specs/corvint-tasks-agent-leases-v0.md`), new and accepted under the
owner's standing auto-accept of lane requirements (2026-10-11). No accepted requirement was amended.

## Problem

`make gate` (`full-gate`, `timeoutSeconds` 7200) ended `FAILED`/`TIMEOUT` twice for V1-0252 at load
average about 64. The owner pre-approved a longer full-gate timeout on 2026-10-15. Policy decoding
bounded every gate `timeoutSeconds` by `wire.MaxGateTimeoutSecs = 120*60`, so a store policy could not
express the change.

## Constraint survey

The survey covered every limit that could depend on the gate timeout, at base `b1ee552b3`:

- Gate run deadline: `executeArgv` (`internal/tasks/store/lease_gate.go`) arms a timer for the declared
  value and kills the process group. It does not depend on the bound.
- Lease: `MaxLeaseMinutes` is 1440. `gate run` holds no writer lock and writes no heartbeat while the
  command runs. It records only while the lease is live (`runnable` before the run, the fence in
  `planGateRun` at record time). 14400 s is 240 min, so one claim or renewal can cover a run.
  Shorter leases need renewal before expiry, the same as with 7200 s. A heartbeat records liveness
  only and does not extend `ExpiresAt` (`planHeartbeat` compared with `planRenew`).
- Detached runs: `attempt run --timeout` is bounded by `maxRunTimeoutSeconds =
  MaxLeaseMinutes*60-130` (86270 s), and `attempt_detach.go` re-checks it on read. This is separate
  from the gate bound, and it is still larger.
- Supervised workflow: `program_run.go` and `Workflow.stage` apply a `wallSeconds`/lane-wall deadline
  to the agent stage only. `RunRole` calls `GateRun` for review-stage gates with the caller's
  context once that stage returns, so `MaxLaneWallMinutes` (240 min) does not bound a gate run. Only
  the gate's `timeoutSeconds` does.
- Dispatcher: role `idleSeconds` 30..86400 (from last activity) and `wallSeconds` 60..604800 (from
  worker start) come from operator configuration. A worker that runs a gate needs both to allow the
  gate timeout.
- Record validators: `GATE_RESULT` records carry no `timeoutSeconds`. A definition is bound by its
  digest, and no receipt or evidence check re-derives the bound. The journal audit
  (`journal/records.go`) re-decodes every posted policy through `intent.DecodePolicy`, so it follows
  the new bound.
- Specs: no spec stated the 7200 value. The only reference was an older test (`intent_test.go`)
  that expected 7201 to be refused.

No constraint prevents raising the bound to 14400. The bound is now `240 * 60`. No other limit
changed.

## Decision

The only code change is `MaxGateTimeoutSecs = 240 * 60`. CAL-V0-255 records the bound and how it
relates to the lease, heartbeat, detach, wall and dispatcher limits. It also records the
compatibility limit: an older binary refuses a policy above 7200 `LIMIT_EXCEEDED`, and its
`receipt audit` keeps refusing the journal after such a policy is posted, even when a later policy
lowers the value. The store policy change is left to the orchestrator, who applies it after a
rebuilt `corvint-tasks` is installed.

Recommended store policy change (v11 to v12): in the `full-gate` gate object only, set
`"timeoutSeconds":"14400"` (from `"7200"`) and `"policyVersion":"12"`. Leave every other byte of the
canonical policy unchanged.

## Independent review

One Codex round (read-only, `b1ee552b3..520afe636`) found no runtime dependency on the old bound and
raised two P2 documentation defects, both fixed in CAL-V0-255 and in this entry:

1. The text said a holder may "renew or heartbeat" to outlast a run, but a heartbeat does not extend
   the lease.
2. The text said supervised gates are cancelled by the stage wall. In fact the wall deadline wraps
   only the agent stage.

The review also noted that no test covers historical rejection by an older binary. That limit is
recorded rather than tested, because an older binary is not built in this change.

## Evidence

- `TestCALV0255_GateTimeoutBoundIs14400` (`internal/tasks/intent`): 1, 7200 and 14400 decode, and
  14401 and 0 refuse `LIMIT_EXCEEDED`. It also asserts that the bound stays at or below
  `MaxLeaseMinutes*60`.
- `TestCALV0255_AuditAcceptsPostedGateTimeoutBound` (`internal/tasks/journal`): a posted policy with
  14400 passes the full audit, and one with 14401 refuses `LIMIT_EXCEEDED`.
- The existing over-maximum case in `intent_test.go` now uses 14401.
- Mutation 1: restoring `120 * 60` fails both new tests (the intent test reports "bound 7200, want
  14400"; the journal test reports "value 14400 outside 1..7200").
- Mutation 2: decoding with `MaxGateTimeoutSecs+1` fails both tests on the 14401 case (got nil,
  want `LIMIT_EXCEEDED`).
- Focused runs: `corvint affected --base b1ee552b3` selected 100 packages, because `wire` is imported
  widely. That full set was not run (NOT_RUN, host load). Instead, `-race -count=1` passed on the
  changed packages `wire`, `intent` and `journal` (load 70 to 74). `transaction` and
  `criterionbinding` passed. `store` and `cli` were filtered by `-run 'Gate|Policy|CALV0016|CALV024'`
  (load 63 to 70). In that run `TestCALV0072_MultiRepositoryGatesFailClosed` failed once and passed
  on rerun. `TestTOLV0027_PreflightDeepStatusPastTheGateTimeoutFails` fails at base `b1ee552b3` as
  well (load 53 to 66): the test's 2-second gate budget expires before the clean filter starts. It
  is a pre-existing load sensitivity, unrelated to the bound, and is reported for filing.
