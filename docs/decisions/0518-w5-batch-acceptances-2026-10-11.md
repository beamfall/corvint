# Decision 0518 — w5 batch requirements accepted

Date: 2026-10-11. Status: accepted by the owner. Authority: owner answers in the orchestrator chat,
2026-10-11 UTC ("approved", "accept 217", "accepted" and "accept", each replying to the listed
requirement IDs), plus the owner's standing auto-accept given the same day ("auto accept. I just do
that, so you may as well") for the lane requirements named below as auto-accepted.

## Context

The w5 batch lands V1-0913, V1-1129/V1-1130 (pull request #788), the w5 docs lane (V1-1140,
V1-1142, V1-1143), the application-map lanes (appmap-a: V1-1131, V1-1136, V1-1138, V1-1144,
V1-1145; appmap-b: V1-1132..V1-1135, V1-1137) and the impact lane (V1-1141). Each lane wrote its new
requirements as "proposed, pending owner acceptance". AGENTS.md invariant 8 keeps acceptance
human-owned.

## Decision

The owner accepts these requirements as written in the spec text carried by this batch:

- `CAL-V0-217` (V1-0913; amends `CAL-V0-139` with a 15-minute idle full-read minimum) and
  `CAL-V0-216` (V1-1130, `--lock-wait`) in `docs/specs/corvint-tasks-agent-leases-v0.md`.
- `LPCVD-V0-001` through `LPCVD-V0-004` (V1-1140) in
  `docs/specs/live-proof-carrying-verification-v0.md`, with the V1-1140 discovery-guidance
  amendment in `docs/specs/mcp-test-validity-profile-v0.md`.
- `UCL1142-V0-001` through `UCL1142-V0-005` (V1-1142) in `docs/specs/upgrade-checklist-v0.md`.
  V1-1143 adds no requirement.
- `ANL-V0-001` through `ANL-V0-016` (V1-1132..V1-1135, V1-1137) in
  `docs/specs/application-navigation-ledger-v0.md`.
- `AMAP-V0-040` through `AMAP-V0-048`, `AMAP-V0-050` through `AMAP-V0-065`, `AMAP-V0-070` through
  `AMAP-V0-072` and `AMAP-V0-075` through `AMAP-V0-077` (V1-1131, V1-1136, V1-1138, V1-1144,
  V1-1145) in `docs/specs/application-map-v0.md`. The `ambiguous-barrel` reason name is kept.
- `DIRTY-CACHE-014` and `DIRTY-CACHE-015` (V1-1141) in
  `docs/specs/revision-cache-dirty-worktree-v0.md`, accepted under the owner's standing
  auto-accept (2026-10-11) after an independent review PASS.

Their status text changes from "proposed, pending owner acceptance (<ticket>)" to accepted, citing
decision 0518.

`CAL-V0-215` (V1-1129, retryable `RESOURCE_EXHAUSTED`) and its A28 amendment were not named in an
acceptance and stay proposed. Every other requirement that was proposed before this batch also
stays proposed.

## Limits

Delivery stays experimental. Live adopter qualification, Linux and the load envelopes the lane
build-log entries record remain NOT_OBSERVED. The acceptance admits no ranking, evidence or authority
input beyond what the accepted text states. The standing auto-accept covers only lane-proposed
requirements after review PASS. It never covers classifier-blocked actions, releases, tags, manual
merges, force-pushes, remote-branch deletion or installs.

## Rollback

To withdraw acceptance, revert this decision and the status text. The requirements then return to
"proposed, pending owner acceptance". The implementations are experimental and need no migration.
