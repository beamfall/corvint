# Decision 0459 — ticket milestones in queue policy (CAL-V0-195..196) accepted

Date: 2026-10-08. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-08
("accept the specs for batch H too"), covering native ticket V1-1026.

## Context

V1-1026 lets a queue opt in to requiring a milestone on new and refined tickets and makes the
unmilestoned count visible. The proposed requirements are `CAL-V0-195..196` in
`docs/specs/corvint-tasks-agent-leases-v0.md`: an optional policy key `milestones.required`
that refuses `CREATE` without a milestone and a `REFINE` clearing one with `MILESTONE_REQUIRED`,
and `queue status.openWithoutMilestone` plus a roadmap warning that count OPEN tickets with no
milestone. AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `CAL-V0-195..196` as written. Their status lines change from
`proposed; V1-1026` to `accepted (decision 0459); V1-1026`, and the V1-1026 section status,
authoritative-inputs line and requirement table row record the acceptance. Other proposed
`CAL-V0` ranges stay `proposed`.

## Limits

This decision settles intent only. Delivery stays `experimental`. No other proposed range in the
spec is accepted, and V1-1026 is not completed by this decision.

## Rollback

Revert this decision, restore the `proposed (V1-1026)` text on the V1-1026 section, the two
requirements, the authoritative-inputs line and the table row, then regenerate
`docs/specs/REQUIREMENTS.tsv`.
