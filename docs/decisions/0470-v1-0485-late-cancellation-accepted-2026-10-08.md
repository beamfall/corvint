# Decision 0470: accept TCP-V0-064 and MCPV0-034

Date: 2026-10-08. Status: accepted (owner approval in chat, 2026-10-08: "accept TCP-V0-064 and
MCPV0-034").
Ticket: V1-0485.

## Context

V1-0485 confirmed on f33ea8ef that a `context` request cancelled after its Git reads, or before the
call when no subject was given, returned `READY`, and that `Registry.Call` returned a READY
`corvint.context` result when cancellation arrived after the compile returned. The lane proposed
two requirements and implemented them.

## Decision

1. `TCP-V0-064` is accepted: `TaskContext` checks its request context at its stage boundaries and
   once after the packet is built; a cancelled request returns the cancellation error and no packet,
   and the history and recency readers are joined on every return path. Uncancelled packets and
   abstentions are unchanged.
2. `MCPV0-034` is accepted: `Registry.Call` checks the request context again after the operation
   returns and reports `cancelled` for every tool.
3. Both specs keep their overall proposed/experimental status; only these clauses are accepted.

## Limits

Retirement is bounded by the work between two adjacent checks, not by a constant; the large
repository worst case is not measured. A cancellation after `Registry.Call`'s final check counts as
a completed call, and MCPV0-011 decides at the server whether its result is written.

## Rollback

Restore the two "proposed" markers and delete this file and its index row. The code rollback is
the one each requirement's spec section names.
