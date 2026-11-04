<!-- SPDX-License-Identifier: Apache-2.0 -->
# Owned selection: path-glob owners, must-run tests, fail-fast order

Date: 2026-11-03. Lane selmark. Tickets: V1-2662, V1-1184, V1-1189, V1-2277. Requirements:
AFP-V0-2480 to AFP-V0-2487 in `docs/specs/affected-plan-selection-v0.md`; AFP-V0-2486 adds one
value to the AFU-V1-019 `reasons` list of `docs/specs/application-flow-understanding-v1.md`.

## Decisions

- **One declaration, not markers in test source (AFP-V0-2480).** V1-2662 asked for a path-glob to
  owning-tests mapping and V1-1184 for must-run tests. Both are project-owned selection authority,
  so both live in one closed file, `.corvint/test-selection.json`. It follows the AFP-V0-160
  documentation declaration: bounded, strictly ordered, refused whole when invalid, and part of
  the graph digest and cache only when present. A per-test annotation would need a parser per
  language and could not be reviewed in one place.
- **Requirement IDs.** V1-1184 and V1-1189 name the application-map, navigation-ledger or
  application-flow specs. The lane's ID block is AFP-V0-2480 to AFP-V0-2519. The rules act on the
  unit plan that `corvint affected`, `corvint test`, the CI selector and the e2e-safe profile all
  read, so they are specified here. AFP-V0-2486 carries must-run into the e2e-safe profile, and
  the AFU wire list names its new reason.
- **A contradicted mapping refuses the build (AFP-V0-2483).** An owner lifts the
  `UNOWNED_DIRTY_PATH` unknown of the paths it matches. A mapping whose tests no longer exist would
  therefore turn a full-suite fallback into a selection of nothing. A full graph refuses it, naming
  the entry; a partial graph skips it, as AFP-V0-819 skips runtime edges.
- **Must-run runs after the frontier step (AFP-V0-2484).** Adding must-run units to the reached
  universe would make every declaration path count as reached (AFP-V0-443), widen frontiers, and
  enlarge the delta denominator. The step instead runs just before the selection loop. It reaches
  `plan.selected` and the CI and `corvint test` package lists, but not `ReachedUnitIDs`. A must-run
  Go package always runs whole in `corvint test`.
- **The order is opt-in and read-only (AFP-V0-2487).** `plan.selected` is already ordered by graph
  distance (AFP-V0-007). Recent local failures come from `corvint test` receipts in the Git
  directory, which are local state outside Git. The order is therefore a separate member behind
  `--fail-fast-order`, so default plans stay a function of the repository alone.
- **V1-2277 was already delivered.** The compact plan carries `tiers` (AFP-V0-162) from
  `tierAffectedSelections`, pinned by `TestAFPV0162AffectedTiersRankTheFullSelection`.

## Evidence

Recorded in the lane handback: the focused tests named in the spec's traceability rows, each with
a negative control and a fault-injection run (the test fails with the rule reverted).
