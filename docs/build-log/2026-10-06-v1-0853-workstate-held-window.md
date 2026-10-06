# 2026-10-06: V1-0853 work-state-held tickets outside the dispatcher's selection window

## Intent

Owner request [issue 624](https://github.com/beamfall/corvint/issues/624), native ticket V1-0853.
The program's work state held 21 tickets (`hold-lane`, `hold-dep`). They still took slots in the
`maxActiveAttempts` window of 24, so 142 actionable tickets stayed DEFERRED `LIMIT_EXCEEDED` and no
role could launch them. The owner asked that held tickets leave the window, with a distinct reason,
that UNKNOWN keep today's behaviour, and that the result be deterministic. The preferred approach
was a held set that the dispatcher derives and passes into planning, rather than one it stores.

## Change

- `internal/tasks/transaction/plan.go`: `PlanInput.WorkStateHeld` (ticket ID set) and
  `PlanReasonWorkStateHeld = "WORK_STATE_HELD"`. In `PriorityFirst`, a held ticket that would
  otherwise reach `choose` is DEFERRED `WORK_STATE_HELD`. It consumes no window slot and no
  resource. A held OPEN pool ticket still counts as waiting for the CAL-V0-101 priority yield.
- `internal/tasks/dispatch/roster.go`:
  - adds `Observation.Replan` and `PlanView`;
  - adds `workStateHeld`. A ticket is held when its known state (not NONE or UNKNOWN) is admitted
    by the state predicate of no ticket role;
  - extracts `stateMatches` from `matches`.
- `internal/tasks/dispatch/loop.go`: `observe` replans once when the held set is non-empty.
- `internal/tasks/cli/dispatch.go`: the native observation supplies `Replan`, which re-runs
  `PriorityFirst` on the same snapshot with the held set. It makes no store read or write.
- Spec: new `CAL-V0-105` (V1-0853 section), plus the status, inputs, slice-table and traceability
  rows. `docs/TASKS-SUPERVISION.md` gains one paragraph.

## Decisions

- Held is derived from the existing role state predicates, so there is no new config key and no
  stored state.
- `NONE`, `UNKNOWN` and a missing reader fail closed to today's window.
- Lane roles are ignored, because they do not launch per ticket.
- `WORK_STATE_HELD` is dispatcher output only. `plan preview` and `claim-next` have no work-state
  observation and are unchanged; the spec records this as a non-goal and a failure mode.
- BLOCKED decisions win over held.

## Evidence

All tests ran as `GOMAXPROCS=2 GOTOOLCHAIN=local go test -p 1 -count=1 -timeout 30m`, and all passed:

- transaction, `-run 'TestCALV0105|TestCALV0097|TestCALV0101'`.
  `TestCALV0105_WorkStateHeldTicketsLeaveWindow` first reproduces the starvation without the held
  set, then shows the fix.
- dispatch, `-run 'TestCALV0105|TestCALV0054|TestCALV0053'`.
- cli, `-run 'TestCALV0105|TestCALV0097'`. `TestCALV0105_DispatchObservationReplansHeldTickets`
  uses a real store and checks that the replan is pure and deterministic.

## Not run

The following were not run: `make gate`, `go test ./...`, the full package suites, and live
dispatcher qualification against the reporting program's store.

## Rollback

Revert the commit. No config, ledger, store or wire bytes change. As an operational rollback, drop
the `workState` reader, or let a role's `states` admit the hold values.
