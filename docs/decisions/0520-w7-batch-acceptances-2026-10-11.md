# Decision 0520 — w7 batch requirements accepted

Date: 2026-10-11. Status: accepted under the owner's standing auto-accept (2026-10-11). Authority:
the owner's standing auto-accept of lane-proposed requirements after an independent review PASS,
given in the orchestrator chat on 2026-10-11 and recorded in decision 0518.

## Context

The w7 batch lands five lane branches on `origin/main`:

- corpus-input-path (V1-1226, GitHub #796).
- dogfood-friction (V1-1219, V1-1198, V1-1209, V1-1207).
- runs (V1-1229, V1-1230, GitHub #797 and #798).
- appmap-rest (V1-1220, V1-1164, V1-1168, GitHub #773, #776, #777 and #778).
- gh800 (V1-1231, GitHub #800).

Each lane wrote its new or amended requirements as "proposed, pending owner acceptance". Each lane
reached a final Codex read-only review verdict of PASS before acceptance. AGENTS.md invariant 8
keeps acceptance human-owned; the standing auto-accept is that human acceptance.

## Decision

These requirements are accepted under the owner's standing auto-accept (2026-10-11), as written in
the spec text carried by this batch:

- `DCP-V1-054` (V1-1226) and `DCP-V1-055` (V1-1231) in `docs/specs/documentation-corpus-v1.md`.
- `DCW-V0-042` (V1-1219), with its amendments to `DCW-V0-039` (a) and (c), `DCW-V0-043` (V1-1198),
  `DCW-V0-044` (V1-1209) and `DCW-V0-045` (V1-1207) in
  `docs/specs/daily-change-evidence-workflow-v0.md`.
- `PWR-V0-001` through `PWR-V0-010` (V1-1229, V1-1230) in
  `docs/specs/playwright-run-evidence-v0.md`.
- `AMAP-V0-079` (V1-1220), with its imported-array clause in `AMAP-V0-078`, and `AMAP-V0-080`
  through `AMAP-V0-082` (V1-1168) in `docs/specs/application-map-v0.md`.
- `ANL-V0-035` and `ANL-V0-036` (V1-1164), with the derived-effect clause in `ANL-V0-010`, in
  `docs/specs/application-navigation-ledger-v0.md`.

Their status text changes from "proposed, pending owner acceptance (<ticket>)" to accepted, citing
this decision. Every other requirement that was proposed before this batch stays proposed.

## Limits

Delivery stays experimental. Live adopter qualification and Linux remain NOT_OBSERVED. GitHub #777
is delivered only in part: the static step effects of `ANL-V0-035`/`036`. Its other parts stay open.
The follow-up gaps that each lane's build-log entry records stay unaccepted and are filed as
tickets. The acceptance admits no ranking, evidence or authority input beyond what the accepted
text states. The standing auto-accept covers only lane-proposed requirements after review. It never
covers classifier-blocked actions, releases, tags, manual merges, force-pushes, remote-branch
deletion or installs.

## Rollback

To withdraw acceptance, revert this decision and the status text. The requirements then return to
"proposed, pending owner acceptance". The implementations are experimental and need no migration.
