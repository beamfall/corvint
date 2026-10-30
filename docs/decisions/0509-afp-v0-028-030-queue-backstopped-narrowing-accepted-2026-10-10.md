# Decision 0509 — Queue-backstopped PR narrowing amendments (AFP-V0-028..030) accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-10
("acceptance is given").

## Context

Decision 0431 (2026-10-05) accepted the intent that a `main` merge queue backstops pull-request test
selection. V1-0810 (PR #754) wrote that intent into `docs/specs/affected-plan-v0.md` as `AFP-V0-028`
(queue-backstopped PR narrowing), `AFP-V0-029` (forward rows and shadow admission) and `AFP-V0-030`
(kill switch and reviewed epoch re-arm), and amended `AFP-V0-013/014/016/017/022/026` and the bounded
documentation CI exception. The spec text itself carried no owner acceptance of its wording.
AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `AFP-V0-028`, `AFP-V0-029` and `AFP-V0-030` as written in the merged spec, together
with the V1-0810 amendments to the other requirements named above. The status lines change from the
decision 0431 intent to "accepted, decision 0509". `AFP-V0-027` is unchanged.

## Limits

This decision settles spec wording only. Delivery stays experimental: the queue behavior, shadow
admission, kill switch, cost and timing evidence are `NOT_OBSERVED`, and no workflow change is made
here. `make gate` is `NOT_RUN`.

## Rollback

Revert this decision and return the spec, README and INDEX status text to "accepted (decision 0431)".
No code or stored state changes.
