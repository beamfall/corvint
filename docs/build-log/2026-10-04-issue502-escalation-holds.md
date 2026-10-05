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
- The dispatcher's native observation carries each ticket's derived hold apart from its plan
  reason. The roster never assigns a held ticket to any role, including a role that does not
  require a selected plan, so the dispatcher never launches a session for it.
- `dispatch status` lists each held ticket with its sorted request IDs from the dispatcher's own
  ledger (`seen.escalations`), even when a pool, pause or ordinary hold is the primary plan reason.

`ESCALATION_PENDING` amends TCP-00 §11's closed code set to 72, following the leases spec's A17
precedent. A reader built before this change refuses the code; only a writer-produced reference
can make it appear, and no writer exists yet.

## Owner decision

On 2026-10-05 the owner accepted the TCP-00 §11 amendment adding `ESCALATION_PENDING` (72 codes)
and its compatibility cost: Tasks readers built before the change refuse the new code. It is
recorded on native ticket V1-0699 at receipt 2749. The escalations spec records it as accepted,
and the leases spec's TCP-00 amendment list points to it. It is no longer an open decision.

## Independent review correction

The first holds commit (482452c0) claimed the native claim refusal kept the dispatcher from
bypassing the hold. Codex review showed that was false: a role that does not require a selected plan
rostered a held ticket, and the dispatcher launches a worker before any claim. Status also showed a
hold only when it was the primary plan reason. The fix carries the derived hold in the observation,
skips held tickets before role matching, and keeps the hold in the ledger apart from the plan reason.
`TestIssue502_DispatchNeverLaunchesForEscalationHold` and
`TestIssue502_DispatchStatusKeepsHoldBehindOtherBlockers` fail with the roster skip removed.

## Not delivered

- Kinds and ages in dispatch status (ESC-V0-009).
- A writer-produced reference through Core's planner and a live dispatcher. After the writer slice
  (a9df0e4a) merged, `TestIssue502_WriterReferenceHoldsAndReleases` holds and releases native direct
  claim and claim-next over a reference the real writer produced.

## Rollback

Revert the change. Core's planner then blocks such tickets as `TICKET_STATE` again, and native
claims stop honouring the hold. No native store byte format changes. The dispatcher ledger gains
the optional `seen.escalations` key; a dispatcher built before the change refuses a ledger saved
while a hold was observed, so after rollback remove that program's ledger `state.json` or wait for a
save with no hold before downgrading.
