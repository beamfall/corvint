# Application map: state names read through constant tables (V1-0981)

Date: 2026-10-07. Ticket V1-0981; GitHub [#669](https://github.com/beamfall/corvint/issues/669)
part 1. Requirement: `AMAP-V0-016` (proposed) in `docs/specs/application-map-v0.md`.

## Problem

A pilot of `corvint flows appmap build` on a large AngularJS + UI-Router app read 153 of 176
`.state(...)` calls as `non-literal-name`, because most names are written
`.state(Names.SOME_SCREEN, {...})` with `Names` an object literal or string enum imported from
another file.

## Decision

- Resolve `X.Y` (positional name, `name:` and `parent:`) only through a table the map reads whole:
  a top-level `const X = {...}` (`as const`, `Object.freeze`) or string `enum`, in the router file
  or the tracked file a static ES import resolves to. Import resolution reuses the contextindex
  `WebImportResolver` (relative and tsconfig/jsconfig `paths`, GPK-V0-077..081), so the map
  follows the same resolution as impact analysis.
- Fail closed: any use of `X` other than an unassigned member read, `typeof`, or an export makes
  the table unprovable; a duplicate, spread, computed key or non-string member does too.
  Mutation from a third module is out of reach of a static map and is a recorded limit.
- Evidence: `name_from` / `parent_from` anchors (path, line, blob, span digest at the map
  revision) on the screen; they join lineage freshness, so editing the constant reads the screen's
  lineage `STALE`. Both are `omitempty`, so literal-only maps (the committed fixture) are
  byte-identical and keep their digest.
- Cost: the revision index is now built before the routers (it was always built); each declaring
  file is lexed once per build, only when a router references it.

## Alternatives rejected

- Evaluating the module or adopting a TypeScript parser: outside the token-level reader contract
  (owner question 14) and the single-binary boundary.
- Matching the member name textually without reading the declaration: a guess.

## Evidence

`TestAMAPV0016ConstantStateNames` and `TestAMAPV0016UnprovableConstantsStayUnknown` (19
fail-closed cases plus a control) in `internal/appmap/stateconst_test.go`; the full
`internal/appmap` package passes unchanged. No adopter-scale re-measurement of the 153/176 figure
was run (the reporter's app is not available here).
