# Lane 1021g: dogfood refusals name their cause and every refused row

Date: 2026-10-21. Lane: claude/dogfood-1021g (base ae3b41815). Tickets V1-2266, V1-1961 (part 1)
and V1-1724 (AC2). Requirements DCW-V0-113, DCW-V0-114 and OCM-V0-025 are accepted under the
owner's standing auto-accept of lane requirements (2026-10-11).

## Problem

- V1-2266: a lane branched from the shared base that merged a sealed predecessor binds the
  predecessor's hunks too, so `dogfood change` refuses with `too-many-obligations` (or another OCM
  aggregate bound) and gives no route.
- V1-1961 (1): `dogfood check` SUMMARY counts unlinked requirements but does not name them.
- V1-1724 (AC2): `dogfood change` stops at the first refused `cem cite` row, so a plan with several
  bad rows costs one bind attempt per row.

## Decisions

- The stacked route already works: binding, checking and sealing against the predecessor's seal
  commit as `BASE` leaves only the lane's hunks, and `batch-check` verifies each sealed change
  against its own base (`DCW-V0-042`). No bound is widened and no merged hunk is excluded from a
  shared-base bind. The aggregate-bound fix line now names the cause, up to four merged sealed
  changes by bind and seal commit, and the route; `docs/DOGFOOD.md` gains "Stacking on a sealed
  predecessor" (`DCW-V0-113`).
- `dogfood check` prints `NOTE unlinked-requirements count=N ids=...` on stderr, at most 32 IDs;
  the SUMMARY line is unchanged (`OCM-V0-025`).
- After the first row-caused `cem cite` refusal, each later cite row is cited against the same map
  into a private probe output that is removed at once; the report lists every refused row by row
  number, hunk, path, span and code, at most 32 (`DCW-V0-114`). The probe never changes the
  prepared map or staged citations and skips unknown rows. Unchanged: the step still fails on the
  first refusal, and no admitted row is cited.

## Evidence

- `TestChangeDCWV0113StackedPredecessorBindsAgainstItsSeal`, `TestOCMV0025CheckNamesUnlinkedRequirementIDs`
  and the updated `TestOCMV0019SummaryCountsRequirementsTheChangeCitesOrTraces`, and
  `TestChangeDCWV0114ReportsEveryRefusedCiteRow` pass. Each new test failed with its fix neutered.
- The #1442 stacked-lane failure was reproduced in a fixture only, not on the live lane.

## NOT_RUN and residuals

- V1-1961 parts (2) (preflight `--plan --witnesses`) and (3) (capability listing): not started.
- V1-1724 AC1 (list non-citable modified paths up front, or a plan-check verb) and AC3 (a measured
  batch with zero first-attempt citation refusals): not started; AC3 needs batch measurement.
- Tier-2 affected units and the exhaustive gate are NOT_RUN locally and are left to batch CI.

## Rollback

Revert the lane's commits. Each change only adds report lines or a fix line; no wire format,
bound, refusal code or stored state changes.
