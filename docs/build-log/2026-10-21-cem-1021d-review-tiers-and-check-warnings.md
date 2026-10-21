# Lane cem-1021d: stable review tiers, dogfood-check tier warnings, aggregate narrowing route

Date: 2026-10-21. Lane `cem-1021d`, base `origin/main` 0106d06cd. Requirements below are accepted under
the owner's standing auto-accept of lane requirements (2026-10-11).

## Problem

- V1-1339: the `cem report` stable rows showed evidence id, relation and drift only, so a map whose
  445 hunks were merely cited from their own files (03a8ca10) read as fully supported.
- V1-1760: `dogfood check` passed silently when hunks sat below the `anchored` tier.
- V1-1068: the aggregate bound refusal already names per-scope counts and the bound (38e4e2858) but not
  a narrowing route.

## Decisions

- `CEM-V1-368`: the stable review projection copies the verifier's own citation tier, anchors and
  tier counts, adds each evidence's drift path and target byte span, flags self-only citations and
  orders the weakest support first. It stays a pure function of the result (`CEM-V1-034`), so line
  numbers and a quoted span are not produced; they need a repository read and a contract decision
  (residual).
- `DCW-V0-112`: `dogfood check` prints to stderr one non-failing `WARN tier-below-anchored` line per cited or
  unknown hunk with the `CEM-V1-081` fix, capped at 50.
- `OCM-V0-024`: every aggregate bound refusal ends with a fixed, pathless narrowing route; the worst
  case stays under finish's 512-byte detail.
- Dropped after the coordinator's check: V1-2133 (in flight in #1421), V1-1329 (#1410), V1-1929
  (#1419). V1-0953 stays refused: the same design (trusting the stored report-set digest and target
  ancestry) was reverted on main in 1649aa2c4 because invalidated evidence could be archived as a
  completed stage; it needs a binding design.

## Evidence

- `TestCEMV1368StableRowsCarryTierAnchorsAndLocation`, `TestDCWV0112CheckWarnsOnHunksBelowAnchored` and
  `TestOCMV0024BoundRefusalNamesNarrowingRoute` fail without their change and pass with it.
- `TestDogfoodDailyPathRunsFromBinaryInForeignRepository` (built binary, real bind) shows the
  warning on stderr; it went to stdout first and broke the `DCW-V0-033` two-line stdout contract, so
  the warning moved to stderr.
- `./internal/cem/workflow` passes in full; the stable report tests in `./internal/cem/cli` and
  `./cmd/corvint` pass.

## NOT_RUN

`make gate` (owner preference for scoped work).

## Rollback

Revert the lane commit. No stored state, wire byte or packet byte depends on it; stable review
record-set digests return to their prior values.
