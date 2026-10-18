# V1-1835: ticketBudget overrides without a default limit (issue 1149)

Ticket V1-1835, GitHub issue 1149. Amends ATR-V0-085 in place in
[corvint-tasks-attempt-runner-v0](../specs/corvint-tasks-attempt-runner-v0.md) and the CAL-V0-155
text in [corvint-tasks-agent-leases-v0](../specs/corvint-tasks-agent-leases-v0.md); no new
requirement IDs, as the leases spec is over its bound. Accepted under the owner's standing
auto-accept of lane requirements (2026-10-11); this is not a new owner decision.

## Decisions

- **Omit, not `unlimited`.** The issue allows either. Omitting both default limits needs no new
  wire value: a `ticketBudget` with at least one override may leave out `sessionsPerDay` and
  `tokensPerDay`. A `ticketBudget` with neither a default nor an override, and a role budget with
  no limit, are still refused.
- **No limit means no ticket scope.** `TicketLimit` returns no budget for a ticket that no override
  selects when there is no default, so the gate never holds it and the spend report omits it.
  The spend history still records its sessions, so adding a default later counts them at once.
- **A recorded hold stays visible.** A ticket whose recorded hold can no longer be resolved (for
  example, an unobserved ticket once held by a label override) is still listed, with `selector`
  `NONE`, until the next tick drops the hold.

## Evidence

`TestATRV0085_OverridesWithoutDefaultValidate`, `TestATRV0085_UnmatchedTicketsNeverHeld`
(`internal/tasks/dispatch`) and `TestATRV0085_DispatchStatusListsOnlyOverrideScopes`
(`internal/tasks/cli`), with the existing ATR-V0-085..089 and CAL-V0-155..160 tests. Live
dispatcher qualification is NOT_RUN.
