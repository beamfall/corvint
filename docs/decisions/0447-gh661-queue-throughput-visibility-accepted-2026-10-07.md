# Decision 0447 — queue throughput visibility requirements accepted

Date: 2026-10-07. Status: accepted by the owner. On 2026-10-07 the owner replied "accept" to the
orchestrator's summary of the issue 661 requirements and the lane's fail-closed defaults.

## Context

Issue [661](https://github.com/beamfall/corvint/issues/661) asked how a supervisor can tell whether a
long-running dispatcher's queue is still moving. The V1-0966 lane proposed `CAL-V0-181`..`185` in
`docs/specs/corvint-tasks-agent-leases-v0.md` and Codex reviewed it for three rounds until it
reported no finding (`docs/build-log/2026-10-07-v1-0966-queue-throughput-visibility.md`).
AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `CAL-V0-181`..`185` as written: the `ticket list --status` filter, record-derived
transition times, `lastAttemptEndedAt`, the `queue status` last completion and completion windows,
and the dispatcher's advisory stall count.

The owner also keeps the lane's fail-closed defaults, each recorded in the spec and build log:

- `statusChangedAt` stays `UNKNOWN` where the record cannot prove it; an exact time for every record
  needs a new record field and stays deferred.
- Completion windows count current records only; a completion that a later REOPEN removed is not
  counted.
- The stall count is keyed on native status only, skips sessions whose work state is unknown, and
  the `stalled` event is advisory: it never holds, parks or skips a ticket.
- The dispatcher ledger moves to `taskman-dispatch-state/3`; an earlier build refuses it with
  `UNSUPPORTED_VERSION`.
- An audit failure degrades only `lastCompletion.receipt` to `UNKNOWN`.

## Limits

This decision settles intent only. Live dispatcher qualification was not run and `make gate` was not
run.

## Rollback

Revert this decision and restore the "proposed, pending owner acceptance" markers in the spec,
`docs/specs/README.md` and `docs/specs/INDEX.json`, then regenerate `docs/specs/REQUIREMENTS.tsv`.
A dispatcher rollback first drains, then removes the ledger's `stall` member or its `state.json`.
