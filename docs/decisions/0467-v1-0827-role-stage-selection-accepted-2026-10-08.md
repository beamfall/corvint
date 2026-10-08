# Decision 0467 — role selection of answered waits by stage (CAL-V0-197) accepted

Date: 2026-10-08. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-08
("accept CAL-V0-197 and the TCN-V0-012 interpretations"), covering native ticket V1-0827.

## Context

V1-0827 found that `run --role implementer` selected every answered `WAITING` attempt whatever
its stage, so an implementer run opened an answered review- or integrate-stage wait and was then
refused `MALFORMED` by the native `DISPATCH`, while reviewer and integrator runs selected no
answered wait at all. The proposed requirement is `CAL-V0-197` in
`docs/specs/corvint-tasks-agent-leases-v0.md`: `run --role` and `admit --role` select an answered
wait only when its recorded stage is the stage of the role, the `STOPPED` reselection predicate
(CAL-V0-078) uses the same rule, and a supervisor transition into `WAITING` with a new question
records it unanswered. AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `CAL-V0-197` as written. Its status changes from `proposed; V1-0827` to
`accepted (decision 0467; V1-0827)`, and the V1-0827 section status, authoritative-inputs line and
requirement table row record the acceptance. Other proposed `CAL-V0` ranges stay `proposed`.

## Limits

This decision settles intent only. Delivery stays `experimental`. No other proposed range in the
spec is accepted, and V1-0827 is not completed by this decision.

## Rollback

Revert this decision, restore the `proposed (V1-0827)` text on the V1-0827 section, the
requirement, the authoritative-inputs line and the table row, then regenerate
`docs/specs/REQUIREMENTS.tsv`.
