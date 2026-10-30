# Decision 0460 — AngularJS injected state-name constants (AMAP-V0-021..023) accepted

Date: 2026-10-08. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-08
("accept the specs for batch H too"), covering native ticket V1-1027 (GitHub #685).

## Context

V1-1027 reads state-name tables that a router file reaches through AngularJS dependency
injection. The proposed requirements are `AMAP-V0-021..023` in
`docs/specs/application-map-v0.md`: an optional manifest `di_constants` scope, resolution of
injected `.constant(...)` names and parents inside that scope, and `UNKNOWN` for anything the
injection path cannot prove. AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `AMAP-V0-021..023` as written. Their status lines change from
`proposed (V1-1027; GitHub #685)` to `accepted (decision 0460; V1-1027; GitHub #685)`, and the
spec header, digest and requirements preamble record the acceptance. `AMAP-V0-016` and
`AMAP-V0-017..020` stay `proposed`.

## Limits

This decision settles intent only. Delivery stays `experimental`. No adopter-scale qualification
is implied, and V1-1027 is not completed by this decision.

## Rollback

Revert this decision, restore the `proposed (V1-1027; GitHub #685)` text on the three
requirements and in the spec header, digest and preamble, then regenerate
`docs/specs/REQUIREMENTS.tsv`.
