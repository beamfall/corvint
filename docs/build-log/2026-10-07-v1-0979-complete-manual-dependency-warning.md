# 2026-10-07: complete-manual dependency warning (V1-0979)

## Intent

Ticket V1-0979 (owner decision 2026-10-07, V1-0978) records that `complete-manual` completed
V1-0015 while its COMPLETED-obligation dependency V1-0014 was OPEN, and the result carried only
the actor-binding warning. The owner chose to warn and still commit. The change adds proposed
CAL-V0-186 to `docs/specs/corvint-tasks-agent-leases-v0.md`, pending owner acceptance.

## Decisions

- **Envelope warning only.** The warning is added by the CLI after a fresh COMPLETE_MANUAL commit;
  the mutation model, receipt, journal entry and result item are untouched, so no wire or replay
  contract changes and no store or intent package is edited.
- **Pre-write read.** Dependency statuses come from the intent store the command already loaded
  before the write, so the check costs no extra store read. A dependency completed concurrently
  can be reported stale; the warning is advisory.
- **Satisfaction test reused.** COMPLETED or ARCHIVED-from-COMPLETED satisfies, as in the §3.1
  eligibility test. A dependency missing from the queue is reported as `not found`. GATE_PASSED
  dependencies are not evaluated (no journal read in this path).
- **Order and replay.** Warnings are in ticketId byte order, one per dependency. An exact replay
  does not repeat them, matching the claim-warning precedent (CAL-V0-107).

## Evidence

`TestCALV0186_CompleteManualWarnsOnUnmetCompletedDependency` (`internal/tasks/cli`) and
`TestCALV0186_UnmetCompletedDependencies` (`internal/tasks/ticket`).
