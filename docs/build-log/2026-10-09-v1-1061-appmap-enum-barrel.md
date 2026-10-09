# Application map: injected string enums, one-level re-exports and the di-constant diagnostic (V1-1061)

Date: 2026-10-09. Ticket V1-1061; GitHub [#705](https://github.com/beamfall/corvint/issues/705).
Requirements: `AMAP-V0-024`..`AMAP-V0-026` (proposed, not accepted) in
`docs/specs/application-map-v0.md`. Follows V1-1027 (AMAP-V0-021..023).

## Problem

An injected registration `.constant('SectionNames', SectionTable)` did not resolve when the
registering file imported `SectionTable` from a directory (`'./routes'` -> `routes/index.ts`)
whose index only re-exported it (`export * from './routes.constants'` or `export { SectionTable }
from ...`). The build exited 0 and every screen read `non-literal-name`, with nothing saying which
step failed. A string enum was already read, but a mixed enum (one auto-numbered or computed
member) still resolved its literal members, although a registration hands the whole value to the
injector.

## Decision

- AMAP-V0-024: through injection only, an enum is a table when every member has a literal string
  initializer; one other member makes it no table (`non-literal-member`). A router's own imported
  enum keeps the AMAP-V0-016 rule, so AMAP-V0-016 maps are unchanged.
- AMAP-V0-025: through injection only, when the imported file does not declare the name, follow
  exactly one level of `export { T }`, `export { S as T }`, `export { default as T }` or
  `export * from` in that file. A named re-export shadows `export *` (ECMAScript). Two candidates
  or an unreadable item -> `ambiguous-barrel`; a candidate that could re-export again ->
  `barrel-depth-exceeded`; a candidate outside the repository index -> `out-of-scope`; none ->
  `identifier-not-found`. The re-export statement is a new anchor between the declaring line and
  the import. Re-exports for router-file imports (AMAP-V0-016 follow-up 10) stay out of scope.
- AMAP-V0-026: when the scope holds exactly one registration of the name and a read through it
  fails, the map carries `{kind: "di-constant", ref: NAME, reason, path, line}` at the
  `.constant(...)` call. This is a map unknown, not a refusal, so no error code or
  `diagnostic.Refusal` site is added (error-code ownership and diagnostic coverage unchanged).
  Maps where every injected read resolves, or with no scope, are byte-identical.

## Evidence

- New tests `TestAMAPV0024InjectedStringEnum` (8 cases + AMAP-V0-016 guard),
  `TestAMAPV0025InjectedTableThroughBarrel` (17 cases), `TestAMAPV0026DIConstantDiagnostics`
  (9 failure + 3 control cases), all synthetic fixtures.
- Fails on base `dd90cfa6` (test file copied to a base worktree): 32 subtests fail for the right
  reason: mixed enums resolve (5), barrels do not resolve (5), no `di-constant` unknown (22).
  All pass after the change; existing AMAP-V0-016 and AMAP-V0-021..023 tests pass unchanged.
- `go test ./internal/appmap ./internal/testplan ./internal/specindex ./cmd/corvint-corpus-mcp`
  and the `cmd/corvint` flows-appmap tests pass; the lane doc gates pass.

## Limits

- No diagnostic for zero or several registrations, a poisoned scope, or a router-side binding the
  reader cannot prove (owner question 16 stays open); those keep the AMAP-V0-023 unknowns only.
- `import { X } from './a'; export { X };` (local re-export list) and deeper barrels are not
  followed. No adopter-scale qualification (`NOT_RUN`); `make gate` `NOT_RUN` per lane rules.
