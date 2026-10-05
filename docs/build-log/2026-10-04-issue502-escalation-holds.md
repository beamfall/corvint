# Typed escalation admission holds: issue 502

Human-owned intent: GitHub issue 502 (native ticket V1-0699), contract
`docs/specs/corvint-tasks-escalations-v0.md` ESC-V0-006, Gate A decisions accepted by decision 0428.
This slice is stacked on the record-key slice (`claude/502-escalations-record-key`, 874444cb).

## Decision

The `ESCALATION_PENDING` hold is derived state. It is computed only from the ticket record's
`escalations` reference: OPEN entries of kind decision, scope or blocked whose source acceptance
revision equals the record's current acceptance revision. Stale, infrastructure, answered and
superseded questions never hold. Nothing is written to place or lift the hold, and acceptance and
supervision semantics are unchanged (ESC-V0-003).

One predicate, `wire.EscalationPending`, decides it, so the native Tasks reader and Core's read-only
planner agree through `internal/tasks/wire` only (decision 0397). It returns the held request IDs
sorted, deduplicated and bounded to `wire.EscalationMaxCurrentOpen` (16), with the unbounded count.

- Native eligibility (`ticket.Inventory.View`) adds the blocker, so direct claim, claim-next,
  recorded claimability and the priority plan all refuse through their existing blocker path.
  A refused claim names the sorted IDs.
- Core's planner blocks such a ticket as `ESCALATION_PENDING` rather than `TICKET_STATE`, through
  the same predicate. An agreement test reads the fixture the Tasks codec writes.
- `dispatch status` lists the tickets its own last plan observation held, without reading the
  native store.

`ESCALATION_PENDING` amends TCP-00 §11's closed code set to 72, following the leases spec's A17
precedent. A reader built before this change refuses the code; only a writer-produced reference
can make it appear, and no writer exists yet.

## Not delivered

- The dispatcher's own roster hold for roles that do not require a selected plan. The native claim
  still refuses, so the hold is not bypassed.
- Request IDs, kinds and ages in dispatch status (ESC-V0-009).
- End-to-end evidence with a writer-produced reference.

## Rollback

Revert the change. Core's planner then blocks such tickets as `TICKET_STATE` again, and native
claims stop honouring the hold. No stored byte format changes.
