# Decision 0478 — pending UNPAUSE recovery (PUR-V0-001..005) accepted

Date: 2026-10-09. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-09
("allow all pending", given for the V1-0309 owner decisions listed below, and later
"PUR-V0-001..005 accepted"), covering native ticket V1-0309.

## Context

V1-0309 (panel F2, audit addendum STO-01) found that nothing settled a pending UNPAUSE receipt
whose post deletes the fixture barrier: `redoPosts` refused the deletion as `UNSUPPORTED`, and
`Barrier` never ran redo, so retrying the unpause returned `REDO_PENDING`. The fix amends the TCP-00
settled-barrier clause ("Pending receipts refuse without redo") and is recorded as
`PUR-V0-001`..`005` in `docs/specs/corvint-tasks-pending-unpause-recovery-v0.md`. The build log
`docs/build-log/2026-10-09-v1-0309-pending-unpause-recovery.md` listed three owner-pending items.
AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `PUR-V0-001` through `PUR-V0-005` as written, and the three owner-pending items of
the build log:

1. Recover verb: no new top-level `recover` verb ("the closed command set and help profile would
   need a contract amendment"); "Existing writers are the supported recovery path."
2. `WRITER_ACTIVE`: the reader code is not added; issue 433 (`CTS-V0-006`) covers the reader
   symptom with its bounded wait.
3. Behaviour change: `Barrier` runs §5.2 redo under its own writer guards before the request
   lookup, so "`Barrier` no longer refuses an unrelated pending receipt; it settles it first."

Adding a `recover` verb or a `WRITER_ACTIVE` code later needs its own owner decision
(`PUR-V0-005`). The spec's intent status, digest, README row and INDEX entry record the acceptance;
delivery stays experimental.

## Limits

This decision settles intent. The evidence is focused tests in
`internal/tasks/store/barrier_redo_test.go`, recorded with their base failures in the build log.
The full `internal/tasks/store` package run is `NOT_PASSED (timeout)`; live, process-kill and
hostile-editor qualification is `NOT_RUN`; `make gate` is `NOT_RUN`. The matching wording edit to
the external TCP-00 barrier clause in `beamfall/corvint-tasks` is not made and remains open.
V1-0309 is not completed by this decision.

## Rollback

Revert this decision and the V1-0309 change: restore the `Barrier` path in
`internal/tasks/store/barrier.go` without redo, remove the barrier-deletion redo, the
`redoBarrierObserved` hook and the narrowed branch guard from `internal/tasks/store/redo.go`, the
`PendingBarrierRemoval` audit mode from `internal/tasks/journal`, the V1-0309 tests and the
`PUR-V0` spec with its README and INDEX entries, then regenerate `docs/specs/REQUIREMENTS.tsv`.
Stores are unchanged until a pending receipt is settled; a settled receipt is an ordinary head.
