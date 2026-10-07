# Decision 0446 — application map, run-verified navigation and scenario planner requirements accepted

Date: 2026-10-07. Status: accepted by the owner. On 2026-10-07 the owner replied "approved" to the
orchestrator's summary of the issue 657, 658 and 660 requirements and their 28 open questions.

## Context

Issues [657](https://github.com/beamfall/corvint/issues/657),
[658](https://github.com/beamfall/corvint/issues/658) and
[660](https://github.com/beamfall/corvint/issues/660) asked for a revision-pinned application map,
navigation verified by test-run receipts, and a scenario planner over the map. The V1-0956,
V1-0957 and V1-0959 lanes proposed `AMAP-V0-001`..`015` in `docs/specs/application-map-v0.md`,
`RVN-V0-001`..`008` in `docs/specs/run-verified-navigation-v0.md` and `AMSP-V0-001`..`010` in
`docs/specs/application-map-scenario-planner-v0.md`. The batch integration reconciled the
verification overlay seam (`docs/build-log/2026-10-07-application-map-batch-integration.md`).
AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts all 33 requirements as written, including the reconciled seam: one learned fact
per bound step with source `run-verification` and the status in its kind.

The owner also keeps every fail-closed V0 default behind the specs' open questions (14 for the map,
5 for run-verified navigation, 9 for the planner). Each question stays recorded in its spec's
Owner questions section for a later answer. In particular, a step's passing receipt does not
verify its selector or reused methods (planner question 9), a newer pass does not clear an older
placed failure (run-verified question 2), and the line-level JavaScript/TypeScript reader still
needs adopter-scale qualification before promotion (map question 14).

## Limits

This decision settles intent only. Delivery stays experimental: receipts in the tests are
synthetic, `make gate` was not run, and follow-ups V1-0968 and V1-0971..V1-0975 remain open.

## Rollback

Revert this decision, restore the "proposed, pending owner acceptance" markers and proposed status
in the three specs and `docs/specs/README.md`, then regenerate `docs/specs/REQUIREMENTS.tsv`.
